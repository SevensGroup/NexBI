package elasticsearch

import (
	"context"
	"errors"
	"io"

	"autonomous-agent/internal/logger"
	"autonomous-agent/internal/requestid"
	"autonomous-agent/internal/tools"
	"autonomous-agent/internal/tools/mcp"
)

type runtime struct {
	client   mcp.Client
	mcpName  string
	limits   limits
	logger   logger.Logger
	toolName string
}

func (r runtime) call(ctx context.Context, args map[string]any, meta map[string]any) (tools.ToolResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	traceID := requestid.From(ctx)
	if meta == nil {
		meta = map[string]any{}
	}
	meta["tool"] = r.toolName
	meta["mcp_tool"] = r.mcpName
	if traceID != "" {
		meta["trace_id"] = traceID
	}
	if r.client == nil {
		te := mcp.Normalize(r.toolName, traceID, mcp.ErrUnavailable)
		return failResult(te, meta), te
	}

	hash, _ := meta["query_hash"].(string)
	r.logSafe(ctx, "elasticsearch tool started", hash, meta)

	res, err := r.client.Call(ctx, r.mcpName, args)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			te := mcp.Normalize(r.toolName, traceID, err)
			return failResult(te, meta), te
		}
		te := mcp.Normalize(r.toolName, traceID, err)
		r.logSafe(ctx, "elasticsearch tool failed", hash, meta)
		return failResult(te, meta), te
	}

	out := tools.ToolResult{
		Success:  true,
		Data:     mcp.DataFromCall(res),
		Metadata: meta,
	}
	r.logSafe(ctx, "elasticsearch tool finished", hash, meta)
	return out, nil
}

func (r runtime) logSafe(ctx context.Context, msg, hash string, meta map[string]any) {
	if r.logger == nil {
		r.logger = logger.New("info", "json", io.Discard)
	}
	args := []any{
		"tool", r.toolName,
		"mcp_tool", r.mcpName,
		"trace_id", requestid.From(ctx),
		"request_id", requestid.From(ctx),
	}
	if hash != "" {
		args = append(args, "query_hash", hash)
	}
	if index, ok := meta["index"]; ok {
		args = append(args, "index", index)
	}
	if size, ok := meta["size"]; ok {
		args = append(args, "size", size)
	}
	r.logger.InfoContext(ctx, msg, args...)
}
