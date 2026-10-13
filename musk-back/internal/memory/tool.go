package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"autonomous-agent/internal/requestid"
	"autonomous-agent/internal/tools"
)

const (
	ToolSearch = "search_memory"
	ToolSave   = "save_memory"
)

// SearchTool lets the agent retrieve memories by text / layer / kind / run_id.
type SearchTool struct {
	store MemoryStore
}

// SaveTool lets the agent persist a typed memory record.
type SaveTool struct {
	store MemoryStore
}

var (
	_ tools.Tool = (*SearchTool)(nil)
	_ tools.Tool = (*SaveTool)(nil)
)

func NewSearchTool(store MemoryStore) *SearchTool { return &SearchTool{store: store} }
func NewSaveTool(store MemoryStore) *SaveTool     { return &SaveTool{store: store} }

func (t *SearchTool) Name() string { return ToolSearch }

func (t *SearchTool) Description() string {
	return "Search Agent Memory. Layers: short_term (Goal, Plan, Decision, ToolCall, Observation), " +
		"task (Task, Conclusion, Evidence, Failures, SuccessfulActions), " +
		"long_term (UserPreference, BusinessKnowledge, HistoricalInsight, QueryPattern). " +
		"Supports pagination via limit and offset."
}

func (t *SearchTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Free-text search over content, metadata, and kind",
			},
			"layer": map[string]any{
				"type":        "string",
				"description": "short_term, task, or long_term",
				"enum":        []string{"short_term", "task", "long_term"},
			},
			"kind": map[string]any{
				"type":        "string",
				"description": "Memory kind such as Goal, UserPreference, QueryPattern",
			},
			"run_id": map[string]any{
				"type":        "string",
				"description": "Limit to a single agent run",
			},
			"task_id": map[string]any{
				"type": "string",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Page size (default 20, max 100)",
			},
			"offset": map[string]any{
				"type":        "integer",
				"description": "Pagination offset",
			},
		},
	}
}

func (t *SearchTool) Execute(ctx context.Context, input map[string]any) (tools.ToolResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if t == nil || t.store == nil {
		return memoryFail(ctx, ToolSearch, tools.CodeExecutionFailed, "memory store is not configured", tools.ErrExecutionFailed)
	}
	if input == nil {
		input = map[string]any{}
	}

	q := Query{
		Text:   stringField(input, "query"),
		RunID:  firstNonEmpty(stringField(input, "run_id"), RunIDFrom(ctx)),
		TaskID: stringField(input, "task_id"),
	}
	if raw := stringField(input, "layer"); raw != "" {
		layer, err := ParseLayer(raw)
		if err != nil {
			return memoryFail(ctx, ToolSearch, tools.CodeInvalidInput, err.Error(), err)
		}
		q.Layer = layer
	}
	if agent := AgentFrom(ctx); agent != "" {
		q = scopeQuery(q, agent)
	}
	if raw := stringField(input, "kind"); raw != "" {
		kind, err := ParseKind(raw)
		if err != nil {
			return memoryFail(ctx, ToolSearch, tools.CodeInvalidInput, err.Error(), err)
		}
		q.Kind = kind
	}
	if _, ok := input["limit"]; ok {
		n, ok := asInt(input["limit"])
		if !ok {
			return memoryFail(ctx, ToolSearch, tools.CodeInvalidInput, "limit must be an integer", tools.ErrInvalidInput)
		}
		q.Limit = n
	}
	if _, ok := input["offset"]; ok {
		n, ok := asInt(input["offset"])
		if !ok {
			return memoryFail(ctx, ToolSearch, tools.CodeInvalidInput, "offset must be an integer", tools.ErrInvalidInput)
		}
		q.Offset = n
	}

	res, err := t.store.Search(ctx, q)
	if err != nil {
		return memoryFail(ctx, ToolSearch, tools.CodeExecutionFailed, err.Error(), err)
	}

	items := make([]map[string]any, 0, len(res.Records))
	for _, rec := range res.Records {
		items = append(items, recordView(rec))
	}
	return tools.ToolResult{
		Success: true,
		Data: map[string]any{
			"records": items,
			"total":   res.Total,
			"limit":   res.Limit,
			"offset":  res.Offset,
		},
		Metadata: map[string]any{
			"tool":     ToolSearch,
			"trace_id": requestid.From(ctx),
			"total":    res.Total,
		},
	}, nil
}

func (t *SaveTool) Name() string { return ToolSave }

func (t *SaveTool) Description() string {
	return "Save a record into Agent Memory. Choose layer and kind. " +
		"short_term: Goal, Plan, Decision, ToolCall, Observation. " +
		"task: Task, Conclusion, Evidence, Failures, SuccessfulActions. " +
		"long_term: UserPreference, BusinessKnowledge, HistoricalInsight, QueryPattern. " +
		"API keys and credentials are stripped. Optional ttl (Go duration, e.g. 24h)."
}

func (t *SaveTool) InputSchema() any {
	return map[string]any{
		"type":     "object",
		"required": []string{"layer", "kind", "content"},
		"properties": map[string]any{
			"layer": map[string]any{
				"type": "string",
				"enum": []string{"short_term", "task", "long_term"},
			},
			"kind": map[string]any{
				"type":        "string",
				"description": "Canonical kind such as UserPreference or Conclusion",
			},
			"content": map[string]any{
				"description": "Payload to store. Objects and strings are accepted. Secrets are redacted.",
			},
			"metadata": map[string]any{
				"type": "object",
			},
			"run_id": map[string]any{
				"type": "string",
			},
			"task_id": map[string]any{
				"type": "string",
			},
			"ttl": map[string]any{
				"type":        "string",
				"description": "Optional TTL such as 24h, 7d is not parsed here — use Go durations (h, m, s)",
			},
		},
	}
}

func (t *SaveTool) Execute(ctx context.Context, input map[string]any) (tools.ToolResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if t == nil || t.store == nil {
		return memoryFail(ctx, ToolSave, tools.CodeExecutionFailed, "memory store is not configured", tools.ErrExecutionFailed)
	}
	if input == nil {
		input = map[string]any{}
	}

	layer, err := ParseLayer(stringField(input, "layer"))
	if err != nil {
		return memoryFail(ctx, ToolSave, tools.CodeInvalidInput, "layer is required and must be short_term, task, or long_term", err)
	}
	kind, err := ParseKind(stringField(input, "kind"))
	if err != nil {
		return memoryFail(ctx, ToolSave, tools.CodeInvalidInput, "kind is invalid", err)
	}
	content, ok := input["content"]
	if !ok || content == nil {
		return memoryFail(ctx, ToolSave, tools.CodeInvalidInput, "content is required", tools.ErrInvalidInput)
	}

	rec := Record{
		RunID:    firstNonEmpty(stringField(input, "run_id"), RunIDFrom(ctx)),
		TaskID:   stringField(input, "task_id"),
		Layer:    layer,
		Kind:     kind,
		Content:  content,
		Metadata: map[string]any{},
	}
	if raw, ok := input["metadata"]; ok && raw != nil {
		if meta, ok := raw.(map[string]any); ok {
			rec.Metadata = meta
		} else {
			return memoryFail(ctx, ToolSave, tools.CodeInvalidInput, "metadata must be an object", tools.ErrInvalidInput)
		}
	}
	if agent := AgentFrom(ctx); agent != "" {
		rec = applyAgentScope(rec, agent)
	}
	if ttl := stringField(input, "ttl"); ttl != "" {
		d, err := parseTTL(ttl)
		if err != nil {
			return memoryFail(ctx, ToolSave, tools.CodeInvalidInput, "ttl must be a duration such as 24h or 7d", tools.ErrInvalidInput)
		}
		if d > 0 {
			exp := time.Now().UTC().Add(d)
			rec.ExpiresAt = &exp
		}
	}

	if err := t.store.Save(ctx, rec); err != nil {
		code := tools.CodeExecutionFailed
		if errors.Is(err, ErrInvalidKind) || errors.Is(err, ErrInvalidLayer) || errors.Is(err, ErrInvalidRecord) {
			code = tools.CodeInvalidInput
		}
		return memoryFail(ctx, ToolSave, code, err.Error(), err)
	}

	return tools.ToolResult{
		Success: true,
		Data: map[string]any{
			"saved":  true,
			"layer":  string(layer),
			"kind":   string(kind),
			"run_id": rec.RunID,
		},
		Metadata: map[string]any{
			"tool":     ToolSave,
			"trace_id": requestid.From(ctx),
			"layer":    string(layer),
			"kind":     string(kind),
		},
	}, nil
}

func recordView(rec Record) map[string]any {
	view := map[string]any{
		"id":         rec.ID,
		"run_id":     rec.RunID,
		"layer":      string(rec.Layer),
		"kind":       string(rec.Kind),
		"content":    rec.Content,
		"metadata":   rec.Metadata,
		"created_at": rec.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
	if rec.TaskID != "" {
		view["task_id"] = rec.TaskID
	}
	if rec.ExpiresAt != nil {
		view["expires_at"] = rec.ExpiresAt.UTC().Format(time.RFC3339Nano)
	}
	return view
}

func memoryFail(ctx context.Context, tool, code, msg string, sentinel error) (tools.ToolResult, error) {
	traceID := requestid.From(ctx)
	te := &tools.ToolError{
		Code:    code,
		Tool:    tool,
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
			"tool":       tool,
		},
	}, te
}

func stringField(input map[string]any, key string) string {
	v, ok := input[key]
	if !ok || v == nil {
		return ""
	}
	s := strings.TrimSpace(fmt.Sprint(v))
	if s == "<nil>" {
		return ""
	}
	return s
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func parseTTL(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("empty duration")
	}
	if strings.HasSuffix(raw, "d") {
		n, err := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(raw, "d")))
		if err != nil {
			return 0, err
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(raw)
}

func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case float64:
		if n != float64(int(n)) {
			return 0, false
		}
		return int(n), true
	case json.Number:
		i, err := n.Int64()
		return int(i), err == nil
	default:
		return 0, false
	}
}
