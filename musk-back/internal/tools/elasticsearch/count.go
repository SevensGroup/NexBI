package elasticsearch

import (
	"context"

	"autonomous-agent/internal/logger"
	"autonomous-agent/internal/requestid"
	"autonomous-agent/internal/tools"
	"autonomous-agent/internal/tools/mcp"
)

// CountTool is the read-only Elasticsearch count tool backed by MCP.
type CountTool struct {
	client  mcp.Client
	mcpName string
	logger  logger.Logger
}

var _ tools.Tool = (*CountTool)(nil)

// NewCountTool constructs the count tool.
func NewCountTool(client mcp.Client, mcpName string, log logger.Logger) *CountTool {
	if mcpName == "" {
		mcpName = MCPCount
	}
	return &CountTool{client: client, mcpName: mcpName, logger: log}
}

func (t *CountTool) Name() string { return NameCount }

func (t *CountTool) Description() string {
	return "Read-only Elasticsearch count via MCP. Optional Query DSL filter. Scripts and writes are rejected."
}

func (t *CountTool) InputSchema() any {
	return map[string]any{
		"type":     "object",
		"required": []string{"index"},
		"properties": map[string]any{
			"index": map[string]any{"type": "string", "description": "Target index"},
			"query": map[string]any{"type": "object", "description": "Optional Query DSL filter"},
		},
	}
}

func (t *CountTool) Execute(ctx context.Context, input map[string]any) (tools.ToolResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	traceID := requestid.From(ctx)
	if input == nil {
		input = map[string]any{}
	}
	index := inputString(input, "index")
	if err := validateIndex(index); err != nil {
		return rejectErr(NameCount, err.Error(), traceID)
	}

	args := map[string]any{"index": index}
	hashSource := map[string]any{"index": index}
	if _, exists := input["query"]; exists && input["query"] != nil {
		query, err := parseQuery(input["query"])
		if err != nil {
			return rejectErr(NameCount, err.Error(), traceID)
		}
		if err := validateQueryObject(query); err != nil {
			return rejectErr(NameCount, err.Error(), traceID)
		}
		args["query"] = query
		hashSource["query"] = query
	}

	hash := queryHash(hashSource)
	rt := runtime{client: t.client, mcpName: t.mcpName, logger: t.logger, toolName: NameCount}
	return rt.call(ctx, args, map[string]any{
		"query_hash": hash,
		"index":      index,
	})
}
