package elasticsearch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"autonomous-agent/internal/tools"
)

var (
	indexNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._*-]*$`)
	docIDRe     = regexp.MustCompile(`^[A-Za-z0-9._\-:@]{1,512}$`)
)

var forbiddenQueryKeys = map[string]struct{}{
	"script":           {},
	"stored_script":    {},
	"script_fields":    {},
	"runtime_mappings": {},
	"delete":           {},
	"update":           {},
	"upsert":           {},
	"scripted_upsert":  {},
	"bulk":             {},
}

type limits struct {
	MaxSize               int
	MaxAggregationBuckets int
}

func queryHash(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		sum := sha256.Sum256([]byte(fmt.Sprint(v)))
		return hex.EncodeToString(sum[:])
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func validateIndex(index string) error {
	index = strings.TrimSpace(index)
	if index == "" {
		return fmt.Errorf("index is required")
	}
	if strings.Contains(index, "..") || strings.ContainsAny(index, `/\: ?<>|`) {
		return fmt.Errorf("index contains illegal characters")
	}
	parts := strings.Split(index, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || p == "." || p == ".." {
			return fmt.Errorf("index contains an illegal name")
		}
		if !indexNameRe.MatchString(p) {
			return fmt.Errorf("index %q is not a valid identifier", p)
		}
	}
	return nil
}

func validateDocID(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("id is required")
	}
	if strings.Contains(id, "..") || strings.ContainsAny(id, `/\\`) {
		return fmt.Errorf("id contains illegal characters")
	}
	if !docIDRe.MatchString(id) {
		return fmt.Errorf("id is not a valid document identifier")
	}
	return nil
}

func asObject(v any) (map[string]any, bool) {
	switch typed := v.(type) {
	case map[string]any:
		return typed, true
	default:
		return nil, false
	}
}

func parseQuery(raw any) (map[string]any, error) {
	if raw == nil {
		return nil, fmt.Errorf("query is required")
	}
	if obj, ok := asObject(raw); ok {
		return obj, nil
	}
	s, ok := raw.(string)
	if !ok {
		return nil, fmt.Errorf("query must be a JSON object")
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("query is required")
	}
	// Query must be Query DSL JSON, never a concatenated Lucene string.
	if !strings.HasPrefix(s, "{") {
		return nil, fmt.Errorf("query must be a JSON object")
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(s), &obj); err != nil {
		return nil, fmt.Errorf("query is not valid JSON")
	}
	return obj, nil
}

func rejectInjection(v any, path string) error {
	switch typed := v.(type) {
	case map[string]any:
		for k, child := range typed {
			lk := strings.ToLower(strings.TrimSpace(k))
			if _, bad := forbiddenQueryKeys[lk]; bad {
				return fmt.Errorf("query contains forbidden field %s", joinPath(path, k))
			}
			if err := rejectInjection(child, joinPath(path, k)); err != nil {
				return err
			}
		}
	case []any:
		for i, child := range typed {
			if err := rejectInjection(child, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	case string:
		if looksLikeJSONInjection(typed) {
			return fmt.Errorf("query contains a possible injection payload at %s", path)
		}
	}
	return nil
}

func looksLikeJSONInjection(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	// Reject values that try to break out of a JSON string into extra clauses.
	if strings.Contains(s, `"script"`) || strings.Contains(s, `"delete"`) {
		return true
	}
	if strings.Contains(s, "}\n") && strings.Contains(s, "{") {
		return true
	}
	return false
}

func joinPath(path, key string) string {
	if path == "" || path == "$" {
		return key
	}
	return path + "." + key
}

func validateQueryObject(query map[string]any) error {
	if query == nil {
		return fmt.Errorf("query is required")
	}
	return rejectInjection(query, "query")
}

func maxRequestedSize(size, from int) int {
	if size < 0 {
		size = 0
	}
	if from < 0 {
		from = 0
	}
	return from + size
}

func validateSize(size, from, maxSize int) error {
	if maxSize < 1 {
		maxSize = 100
	}
	if size < 0 {
		return fmt.Errorf("size must be >= 0")
	}
	if from < 0 {
		return fmt.Errorf("from must be >= 0")
	}
	if size > maxSize {
		return fmt.Errorf("size %d exceeds max_size %d", size, maxSize)
	}
	if from > maxSize {
		return fmt.Errorf("from %d exceeds max_size %d", from, maxSize)
	}
	if maxRequestedSize(size, from) > maxSize && size > 0 {
		return fmt.Errorf("from+size %d exceeds max_size %d", from+size, maxSize)
	}
	return nil
}

func maxAggregationBuckets(v any) int {
	max := 0
	var walk func(any)
	walk = func(node any) {
		switch typed := node.(type) {
		case map[string]any:
			for k, child := range typed {
				lk := strings.ToLower(k)
				if lk == "size" || lk == "shard_size" {
					if n, ok := asInt(child); ok && n > max {
						max = n
					}
				}
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(v)
	return max
}

func validateAggregations(aggs any, maxBuckets int) error {
	if aggs == nil {
		return fmt.Errorf("aggregations are required")
	}
	obj, ok := asObject(aggs)
	if !ok {
		return fmt.Errorf("aggregations must be a JSON object")
	}
	if err := rejectInjection(obj, "aggregations"); err != nil {
		return err
	}
	if maxBuckets < 1 {
		maxBuckets = 100
	}
	if n := maxAggregationBuckets(obj); n > maxBuckets {
		return fmt.Errorf("aggregation buckets %d exceed max_aggregation_buckets %d", n, maxBuckets)
	}
	return nil
}

func asInt(v any) (int, bool) {
	f, ok := asFloat(v)
	if !ok || f != float64(int(f)) {
		return 0, false
	}
	return int(f), true
}

func asFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case string:
		var f float64
		if _, err := fmt.Sscan(strings.TrimSpace(n), &f); err == nil {
			return f, true
		}
	}
	return 0, false
}

func inputString(input map[string]any, key string) string {
	if input == nil {
		return ""
	}
	v, ok := input[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func inputInt(input map[string]any, key string, fallback int) (int, error) {
	if input == nil {
		return fallback, nil
	}
	v, ok := input[key]
	if !ok || v == nil {
		return fallback, nil
	}
	n, ok := asInt(v)
	if !ok {
		return 0, fmt.Errorf("%s must be an integer", key)
	}
	return n, nil
}

func rejectErr(tool, msg string, traceID string) (tools.ToolResult, error) {
	te := &tools.ToolError{
		Code:    tools.CodeInvalidInput,
		Tool:    tool,
		Message: msg,
		TraceID: traceID,
		Err:     tools.ErrInvalidInput,
	}
	return failResult(te, map[string]any{"error_code": tools.CodeInvalidInput}), te
}

func failResult(err *tools.ToolError, extra map[string]any) tools.ToolResult {
	meta := map[string]any{}
	for k, v := range extra {
		meta[k] = v
	}
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
