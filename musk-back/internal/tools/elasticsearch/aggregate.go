package elasticsearch

import (
	"context"

	"autonomous-agent/internal/logger"
	"autonomous-agent/internal/requestid"
	"autonomous-agent/internal/tools"
	"autonomous-agent/internal/tools/mcp"
)

// AggregateTool is the read-only Elasticsearch aggregation tool backed by MCP.
type AggregateTool struct {
	client  mcp.Client
	mcpName string
	limits  limits
	logger  logger.Logger
}

// ElasticsearchAggregateTool is the spec name for AggregateTool.
type ElasticsearchAggregateTool = AggregateTool

var _ tools.Tool = (*AggregateTool)(nil)

// NewAggregateTool constructs ElasticsearchAggregateTool.
func NewAggregateTool(client mcp.Client, mcpName string, maxSize, maxBuckets int, log logger.Logger) *AggregateTool {
	if mcpName == "" {
		mcpName = MCPAggregate
	}
	if maxSize < 1 {
		maxSize = 100
	}
	if maxBuckets < 1 {
		maxBuckets = 100
	}
	return &AggregateTool{
		client:  client,
		mcpName: mcpName,
		limits:  limits{MaxSize: maxSize, MaxAggregationBuckets: maxBuckets},
		logger:  log,
	}
}

func (t *AggregateTool) Name() string { return NameAggregate }

func (t *AggregateTool) Description() string {
	return "Read-only Elasticsearch aggregations via MCP. Requires index and aggregations. Scripts, writes, and oversized buckets are rejected."
}

func (t *AggregateTool) InputSchema() any {
	return map[string]any{
		"type":     "object",
		"required": []string{"index", "aggregations"},
		"properties": map[string]any{
			"index": map[string]any{
				"type":        "string",
				"description": "Target index or comma-separated index names",
			},
			"query": map[string]any{
				"type":        "object",
				"description": "Optional Query DSL filter",
			},
			"aggregations": map[string]any{
				"type":        "object",
				"description": "Elasticsearch aggregations object",
			},
			"size": map[string]any{
				"type":        "integer",
				"description": "Hit size; typically 0 for aggregations-only",
			},
		},
	}
}

func (t *AggregateTool) Execute(ctx context.Context, input map[string]any) (tools.ToolResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	traceID := requestid.From(ctx)
	if input == nil {
		input = map[string]any{}
	}

	index := inputString(input, "index")
	if err := validateIndex(index); err != nil {
		return rejectErr(NameAggregate, err.Error(), traceID)
	}
	if err := validateAggregations(input["aggregations"], t.limits.MaxAggregationBuckets); err != nil {
		return rejectErr(NameAggregate, err.Error(), traceID)
	}
	aggs, _ := asObject(input["aggregations"])

	var query map[string]any
	if _, exists := input["query"]; exists && input["query"] != nil {
		parsed, err := parseQuery(input["query"])
		if err != nil {
			return rejectErr(NameAggregate, err.Error(), traceID)
		}
		if err := validateQueryObject(parsed); err != nil {
			return rejectErr(NameAggregate, err.Error(), traceID)
		}
		query = parsed
	}

	size, err := inputInt(input, "size", 0)
	if err != nil {
		return rejectErr(NameAggregate, err.Error(), traceID)
	}
	if err := validateSize(size, 0, t.limits.MaxSize); err != nil {
		return rejectErr(NameAggregate, err.Error(), traceID)
	}

	hashSource := map[string]any{"aggregations": aggs}
	if query != nil {
		hashSource["query"] = query
	}
	hash := queryHash(hashSource)

	args := map[string]any{
		"index":        index,
		"aggregations": aggs,
		"aggs":         aggs,
		"size":         size,
	}
	if query != nil {
		args["query"] = query
	}

	rt := runtime{client: t.client, mcpName: t.mcpName, limits: t.limits, logger: t.logger, toolName: NameAggregate}
	return rt.call(ctx, args, map[string]any{
		"query_hash": hash,
		"index":      index,
		"size":       size,
	})
}
