package elasticsearch

import (
	"context"

	"autonomous-agent/internal/logger"
	"autonomous-agent/internal/requestid"
	"autonomous-agent/internal/tools"
	"autonomous-agent/internal/tools/mcp"
)

// SearchTool is the read-only Elasticsearch search tool backed by MCP.
type SearchTool struct {
	client  mcp.Client
	mcpName string
	limits  limits
	logger  logger.Logger
}

// ElasticsearchSearchTool is the spec name for SearchTool.
type ElasticsearchSearchTool = SearchTool

var _ tools.Tool = (*SearchTool)(nil)

// NewSearchTool constructs ElasticsearchSearchTool.
func NewSearchTool(client mcp.Client, mcpName string, maxSize int, log logger.Logger) *SearchTool {
	if mcpName == "" {
		mcpName = MCPSearch
	}
	if maxSize < 1 {
		maxSize = 100
	}
	return &SearchTool{client: client, mcpName: mcpName, limits: limits{MaxSize: maxSize}, logger: log}
}

func (t *SearchTool) Name() string { return NameSearch }

func (t *SearchTool) Description() string {
	return "Read-only Elasticsearch search via MCP. Requires index and a Query DSL object. Writes, scripts, and oversized result windows are rejected."
}

func (t *SearchTool) InputSchema() any {
	return map[string]any{
		"type":     "object",
		"required": []string{"index", "query"},
		"properties": map[string]any{
			"index": map[string]any{
				"type":        "string",
				"description": "Target index or comma-separated index names",
			},
			"query": map[string]any{
				"type":        "object",
				"description": "Elasticsearch Query DSL object",
			},
			"size": map[string]any{
				"type":        "integer",
				"description": "Maximum hits to return",
			},
			"from": map[string]any{
				"type":        "integer",
				"description": "Hit offset",
			},
		},
	}
}

func (t *SearchTool) Execute(ctx context.Context, input map[string]any) (tools.ToolResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	traceID := requestid.From(ctx)
	if input == nil {
		input = map[string]any{}
	}

	index := inputString(input, "index")
	if err := validateIndex(index); err != nil {
		return rejectErr(NameSearch, err.Error(), traceID)
	}
	query, err := parseQuery(input["query"])
	if err != nil {
		return rejectErr(NameSearch, err.Error(), traceID)
	}
	if err := validateQueryObject(query); err != nil {
		return rejectErr(NameSearch, err.Error(), traceID)
	}

	size, err := inputInt(input, "size", 10)
	if err != nil {
		return rejectErr(NameSearch, err.Error(), traceID)
	}
	from, err := inputInt(input, "from", 0)
	if err != nil {
		return rejectErr(NameSearch, err.Error(), traceID)
	}
	if size == 0 && input["size"] == nil {
		size = 10
	}
	if err := validateSize(size, from, t.limits.MaxSize); err != nil {
		return rejectErr(NameSearch, err.Error(), traceID)
	}

	hash := queryHash(query)
	args := map[string]any{
		"index": index,
		"query": query,
		"size":  size,
		"from":  from,
	}
	rt := runtime{client: t.client, mcpName: t.mcpName, limits: t.limits, logger: t.logger, toolName: NameSearch}
	return rt.call(ctx, args, map[string]any{
		"query_hash": hash,
		"index":      index,
		"size":       size,
		"from":       from,
	})
}
