package mcp

import (
	"context"
	"strings"

	"autonomous-agent/internal/requestid"
	"autonomous-agent/internal/tools"
)

// Adapter converts a discovered MCP tool into the unified Tool interface.
type Adapter struct {
	client Client
	info   ToolInfo
}

var _ tools.Tool = (*Adapter)(nil)

// NewAdapter wraps an MCP tool as tools.Tool.
func NewAdapter(client Client, info ToolInfo) *Adapter {
	info.Name = strings.ToLower(strings.TrimSpace(info.Name))
	if strings.TrimSpace(info.Description) == "" {
		info.Description = "MCP tool " + info.Name
	}
	if info.InputSchema == nil {
		info.InputSchema = map[string]any{"type": "object"}
	}
	return &Adapter{client: client, info: info}
}

func (a *Adapter) Name() string { return a.info.Name }

func (a *Adapter) Description() string { return a.info.Description }

func (a *Adapter) InputSchema() any { return a.info.InputSchema }

func (a *Adapter) Execute(ctx context.Context, input map[string]any) (tools.ToolResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	traceID := requestid.From(ctx)
	if a == nil || a.client == nil {
		te := Normalize("", traceID, ErrUnavailable)
		return failed(te), te
	}
	if input == nil {
		input = map[string]any{}
	}
	res, err := a.client.Call(ctx, a.info.Name, input)
	if err != nil {
		te := Normalize(a.info.Name, traceID, err)
		out := failed(te)
		if res.IsError {
			out.Metadata["mcp_error"] = true
		}
		return out, te
	}
	return tools.ToolResult{
		Success: true,
		Data:    DataFromCall(res),
		Metadata: map[string]any{
			"tool":     a.info.Name,
			"trace_id": traceID,
			"mcp":      true,
		},
	}, nil
}

func failed(err *tools.ToolError) tools.ToolResult {
	meta := map[string]any{}
	msg := ""
	if err != nil {
		msg = err.Error()
		if err.Tool != "" {
			meta["tool"] = err.Tool
		}
		if err.TraceID != "" {
			meta["trace_id"] = err.TraceID
		}
		if err.Code != "" {
			meta["error_code"] = err.Code
		}
	}
	return tools.ToolResult{Success: false, Error: msg, Metadata: meta}
}
