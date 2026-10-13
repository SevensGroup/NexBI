package elasticsearch

import (
	"context"

	"autonomous-agent/internal/logger"
	"autonomous-agent/internal/requestid"
	"autonomous-agent/internal/tools"
	"autonomous-agent/internal/tools/mcp"
)

// GetTool is the read-only Elasticsearch get-by-id tool backed by MCP.
type GetTool struct {
	client  mcp.Client
	mcpName string
	logger  logger.Logger
}

var _ tools.Tool = (*GetTool)(nil)

// NewGetTool constructs the get tool.
func NewGetTool(client mcp.Client, mcpName string, log logger.Logger) *GetTool {
	if mcpName == "" {
		mcpName = MCPGet
	}
	return &GetTool{client: client, mcpName: mcpName, logger: log}
}

func (t *GetTool) Name() string { return NameGet }

func (t *GetTool) Description() string {
	return "Read-only Elasticsearch get via MCP. Requires index and document id."
}

func (t *GetTool) InputSchema() any {
	return map[string]any{
		"type":     "object",
		"required": []string{"index", "id"},
		"properties": map[string]any{
			"index": map[string]any{"type": "string", "description": "Target index"},
			"id":    map[string]any{"type": "string", "description": "Document id"},
		},
	}
}

func (t *GetTool) Execute(ctx context.Context, input map[string]any) (tools.ToolResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	traceID := requestid.From(ctx)
	if input == nil {
		input = map[string]any{}
	}
	index := inputString(input, "index")
	if err := validateIndex(index); err != nil {
		return rejectErr(NameGet, err.Error(), traceID)
	}
	id := inputString(input, "id")
	if err := validateDocID(id); err != nil {
		return rejectErr(NameGet, err.Error(), traceID)
	}
	args := map[string]any{"index": index, "id": id}
	hash := queryHash(args)
	rt := runtime{client: t.client, mcpName: t.mcpName, logger: t.logger, toolName: NameGet}
	return rt.call(ctx, args, map[string]any{
		"query_hash": hash,
		"index":      index,
	})
}
