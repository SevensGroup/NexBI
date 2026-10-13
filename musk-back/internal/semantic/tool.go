package semantic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"autonomous-agent/internal/config"
	"autonomous-agent/internal/logger"
	"autonomous-agent/internal/requestid"
	"autonomous-agent/internal/tools"
)

const (
	// ToolName is the registry name of the semantic query tool.
	ToolName = "semantic_query"
	// esAggregateTool is the registry name used to execute compiled ES DSL.
	esAggregateTool = "elasticsearch_aggregate"
)

// QueryTool turns a SemanticQuery (or NL) into ES DSL via the compiler, then executes through the Tool Registry.
type QueryTool struct {
	catalog  *Registry
	registry tools.Registry
	limits   Limits
	now      func() time.Time
	logger   logger.Logger
}

var _ tools.Tool = (*QueryTool)(nil)

// NewQueryTool constructs the semantic_query tool. Execution goes through registry, never an ES SDK.
func NewQueryTool(catalog *Registry, registry tools.Registry, limits Limits, log logger.Logger) *QueryTool {
	if limits.MaxTimeRange <= 0 {
		limits.MaxTimeRange = defaultMaxTimeRange
	}
	if limits.MaxResultSize < 1 {
		limits.MaxResultSize = defaultMaxResultSize
	}
	if log == nil {
		log = logger.New("info", "json", os.Stdout)
	}
	return &QueryTool{
		catalog:  catalog,
		registry: registry,
		limits:   limits,
		now:      func() time.Time { return time.Now().UTC() },
		logger:   log,
	}
}

func (t *QueryTool) Name() string { return ToolName }

func (t *QueryTool) Description() string {
	metrics := make([]string, 0)
	dims := make([]string, 0)
	if t != nil && t.catalog != nil {
		for _, m := range t.catalog.ListMetrics() {
			metrics = append(metrics, m.Name)
		}
		for _, d := range t.catalog.ListDimensions() {
			dims = append(dims, d.Name)
		}
	}
	return "Query advertising data using business concepts (metrics, dimensions, filters, time_range). " +
		"Do not pass Elasticsearch Query DSL. " +
		"Available metrics: " + strings.Join(metrics, ", ") + ". " +
		"Available dimensions: " + strings.Join(dims, ", ") + "."
}

func (t *QueryTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Natural-language business question, e.g. Why did Meta ROI decline yesterday",
			},
			"metrics": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Business metrics such as ROAS, CTR, Spend",
			},
			"dimensions": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Breakdowns such as Campaign, Creative, Channel",
			},
			"filters": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"dimension": map[string]any{"type": "string"},
						"op":        map[string]any{"type": "string"},
						"value":     map[string]any{},
					},
				},
			},
			"time_range": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"start": map[string]any{"type": "string"},
					"end":   map[string]any{"type": "string"},
					"label": map[string]any{"type": "string"},
				},
			},
			"order_by": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"field":     map[string]any{"type": "string"},
						"direction": map[string]any{"type": "string"},
					},
				},
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Max rows / aggregation buckets",
			},
		},
	}
}

func (t *QueryTool) Execute(ctx context.Context, input map[string]any) (tools.ToolResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	traceID := requestid.From(ctx)
	if input == nil {
		input = map[string]any{}
	}

	q, err := t.buildQuery(input)
	if err != nil {
		return t.fail(traceID, tools.CodeInvalidInput, err.Error(), tools.ErrInvalidInput)
	}
	applyQueryDefaults(q, t.limits)

	if err := NewValidator(t.catalog, t.limits).Validate(q); err != nil {
		return t.fail(traceID, tools.CodeInvalidInput, err.Error(), tools.ErrInvalidInput)
	}

	compiled, err := NewCompiler(t.catalog).Compile(q)
	if err != nil {
		return t.fail(traceID, tools.CodeInvalidInput, err.Error(), tools.ErrInvalidInput)
	}

	hash := compiledHash(compiled)
	t.logger.InfoContext(ctx, "semantic query compiled",
		"trace_id", traceID,
		"request_id", traceID,
		"tool", ToolName,
		"query_hash", hash,
		"metrics", strings.Join(q.Metrics, ","),
		"dimensions", strings.Join(q.Dimensions, ","),
	)

	esInput := map[string]any{
		"index":        compiled.Index,
		"query":        compiled.Query,
		"aggregations": compiled.Aggregations,
		"size":         compiled.Size,
	}

	if t.registry == nil {
		return t.fail(traceID, tools.CodeExecutionFailed, "tool registry is required", tools.ErrExecutionFailed)
	}
	if _, ok := t.registry.Get(esAggregateTool); !ok {
		return t.fail(traceID, tools.CodeNotFound, "elasticsearch aggregate tool is not registered", tools.ErrNotFound)
	}

	esRes, esErr := t.registry.Execute(ctx, esAggregateTool, esInput)
	if esErr != nil && !esRes.Success {
		meta := map[string]any{"query_hash": hash, "trace_id": traceID, "tool": ToolName}
		if esRes.Metadata != nil {
			if h, ok := esRes.Metadata["query_hash"]; ok {
				meta["es_query_hash"] = h
			}
		}
		return tools.ToolResult{Success: false, Error: esRes.Error, Metadata: meta}, esErr
	}

	result := applyFormulas(compiled, t.catalog, q, esRes.Data)
	meta := map[string]any{
		"query_hash": hash,
		"trace_id":   traceID,
		"tool":       ToolName,
		"index":      compiled.Index,
	}
	if esRes.Metadata != nil {
		if h, ok := esRes.Metadata["query_hash"]; ok {
			meta["es_query_hash"] = h
		}
	}
	return tools.ToolResult{
		Success:  true,
		Data:     result,
		Metadata: meta,
	}, nil
}

func (t *QueryTool) buildQuery(input map[string]any) (*Query, error) {
	now := time.Now().UTC()
	if t.now != nil {
		now = t.now()
	}

	var q *Query
	if raw := strings.TrimSpace(fmt.Sprint(input["query"])); raw != "" && raw != "<nil>" {
		parsed, err := Parse(raw, t.catalog, now)
		if err != nil {
			return nil, err
		}
		q = parsed
	} else {
		q = &Query{}
	}

	if metrics, ok := asStringSlice(input["metrics"]); ok && len(metrics) > 0 {
		q.Metrics = metrics
	}
	if dims, ok := asStringSlice(input["dimensions"]); ok && len(dims) > 0 {
		q.Dimensions = dims
	}
	if filters, err := parseFilters(input["filters"]); err != nil {
		return nil, err
	} else if len(filters) > 0 {
		q.Filters = filters
	}
	if tr, err := parseTimeRange(input["time_range"], now); err != nil {
		return nil, err
	} else if !tr.IsZero() {
		q.TimeRange = tr
	}
	if order, err := parseOrderBy(input["order_by"]); err != nil {
		return nil, err
	} else if len(order) > 0 {
		q.OrderBy = order
	}
	if _, ok := input["limit"]; ok {
		n, ok := asInt(input["limit"])
		if !ok {
			return nil, fmt.Errorf("%w: limit must be an integer", ErrInvalidQuery)
		}
		q.Limit = n
	}
	if src := strings.TrimSpace(fmt.Sprint(input["data_source"])); src != "" && src != "<nil>" {
		q.DataSource = src
	}
	return q, nil
}

func (t *QueryTool) fail(traceID, code, msg string, sentinel error) (tools.ToolResult, error) {
	te := &tools.ToolError{
		Code:    code,
		Tool:    ToolName,
		Message: msg,
		TraceID: traceID,
		Err:     sentinel,
	}
	return tools.ToolResult{
		Success: false,
		Error:   te.Error(),
		Metadata: map[string]any{
			"error_code": code,
			"trace_id":   traceID,
			"tool":       ToolName,
		},
	}, te
}

func parseFilters(raw any) ([]Filter, error) {
	if raw == nil {
		return nil, nil
	}
	items, ok := asSlice(raw)
	if !ok {
		return nil, fmt.Errorf("%w: filters must be an array", ErrInvalidQuery)
	}
	out := make([]Filter, 0, len(items))
	for _, item := range items {
		obj, ok := asMap(item)
		if !ok {
			return nil, fmt.Errorf("%w: filter must be an object", ErrInvalidQuery)
		}
		op := strings.TrimSpace(fmt.Sprint(obj["op"]))
		if op == "<nil>" {
			op = "eq"
		}
		out = append(out, Filter{
			Dimension: strings.TrimSpace(fmt.Sprint(obj["dimension"])),
			Op:        firstNonEmpty(op, "eq"),
			Value:     obj["value"],
		})
	}
	return out, nil
}

func parseTimeRange(raw any, now time.Time) (TimeRange, error) {
	if raw == nil {
		return TimeRange{}, nil
	}
	obj, ok := asMap(raw)
	if !ok {
		return TimeRange{}, fmt.Errorf("%w: time_range must be an object", ErrInvalidQuery)
	}
	label := strings.TrimSpace(fmt.Sprint(obj["label"]))
	if label == "<nil>" {
		label = ""
	}
	tr := TimeRange{Label: label}
	if v, ok := obj["start"]; ok && v != nil {
		t, err := parseTimeValue(v)
		if err != nil {
			return TimeRange{}, err
		}
		tr.Start = t
	}
	if v, ok := obj["end"]; ok && v != nil {
		t, err := parseTimeValue(v)
		if err != nil {
			return TimeRange{}, err
		}
		tr.End = t
	}
	if tr.Label != "" && tr.Start.IsZero() && tr.End.IsZero() {
		if phrase, ok := matchDatePhrase(tr.Label, now); ok {
			tr.Start, tr.End, tr.Label = phrase.start, phrase.end, phrase.label
		}
	}
	return tr, nil
}

func parseTimeValue(v any) (time.Time, error) {
	switch typed := v.(type) {
	case time.Time:
		return typed.UTC(), nil
	case string:
		s := strings.TrimSpace(typed)
		if s == "" {
			return time.Time{}, nil
		}
		for _, layout := range []string{time.RFC3339, "2006-01-02", "2006-01-02T15:04:05", "2006-01-02 15:04:05"} {
			if t, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
				return t, nil
			}
		}
		return time.Time{}, fmt.Errorf("%w: invalid time %q", ErrInvalidQuery, s)
	default:
		return time.Time{}, fmt.Errorf("%w: invalid time value", ErrInvalidQuery)
	}
}

func parseOrderBy(raw any) ([]OrderBy, error) {
	if raw == nil {
		return nil, nil
	}
	items, ok := asSlice(raw)
	if !ok {
		return nil, fmt.Errorf("%w: order_by must be an array", ErrInvalidQuery)
	}
	out := make([]OrderBy, 0, len(items))
	for _, item := range items {
		obj, ok := asMap(item)
		if !ok {
			return nil, fmt.Errorf("%w: order_by item must be an object", ErrInvalidQuery)
		}
		out = append(out, OrderBy{
			Field:     strings.TrimSpace(fmt.Sprint(obj["field"])),
			Direction: strings.TrimSpace(fmt.Sprint(obj["direction"])),
		})
	}
	return out, nil
}

func asStringSlice(v any) ([]string, bool) {
	switch typed := v.(type) {
	case []string:
		return typed, true
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			s := strings.TrimSpace(fmt.Sprint(item))
			if s == "" || s == "<nil>" {
				continue
			}
			out = append(out, s)
		}
		return out, true
	default:
		return nil, false
	}
}

func asInt(v any) (int, bool) {
	f, ok := asFloat(v)
	if !ok || f != float64(int(f)) {
		return 0, false
	}
	return int(f), true
}

func compiledHash(c *CompiledQuery) string {
	raw, err := json.Marshal(map[string]any{
		"index":        c.Index,
		"query":        c.Query,
		"aggregations": c.Aggregations,
		"size":         c.Size,
	})
	if err != nil {
		sum := sha256.Sum256([]byte(fmt.Sprint(c)))
		return hex.EncodeToString(sum[:])
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// RegisterFromConfig loads configs/semantic YAML and registers semantic_query on the Tool Registry.
func RegisterFromConfig(_ context.Context, reg tools.Registry, cfg config.SemanticConfig, log logger.Logger) (*Registry, error) {
	dir := strings.TrimSpace(cfg.Dir)
	if dir == "" {
		return nil, nil
	}
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("semantic dir %s: %w", dir, err)
	}
	catalog, err := Load(dir)
	if err != nil {
		return nil, err
	}
	limits := Limits{
		MaxTimeRange:  cfg.MaxTimeRange.Duration(),
		MaxResultSize: cfg.MaxResultSize,
	}
	if err := reg.Register(NewQueryTool(catalog, reg, limits, log), tools.RiskMedium); err != nil {
		return nil, err
	}
	return catalog, nil
}

// IsRawSearchTool reports Elasticsearch tools the agent must not call when the semantic layer is present.
func IsRawSearchTool(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	return strings.HasPrefix(n, "elasticsearch_")
}
