package semantic

import (
	"fmt"
	"sort"
	"strings"
)

// Result is a business-level answer: metrics, not Elasticsearch DSL.
type Result struct {
	Metrics    map[string]any   `json:"metrics,omitempty"`
	Rows       []map[string]any `json:"rows,omitempty"`
	TimeRange  TimeRange        `json:"time_range"`
	Filters    []Filter         `json:"filters,omitempty"`
	Dimensions []string         `json:"dimensions,omitempty"`
}

func applyFormulas(compiled *CompiledQuery, catalog *Registry, q *Query, esData any) Result {
	out := Result{
		TimeRange:  q.TimeRange,
		Filters:    append([]Filter(nil), q.Filters...),
		Dimensions: append([]string(nil), q.Dimensions...),
		Metrics:    map[string]any{},
	}
	aggs := extractAggregations(esData)
	if len(q.Dimensions) == 0 {
		vals := readMetricValues(aggs, compiled)
		out.Metrics = projectMetrics(q.Metrics, catalog, vals)
		return out
	}
	rows := walkBuckets(aggs, q.Dimensions, compiled, catalog, nil)
	rows = sortRows(rows, q.OrderBy, catalog)
	if q.Limit > 0 && len(rows) > q.Limit {
		rows = rows[:q.Limit]
	}
	out.Rows = rows
	if len(rows) == 1 {
		out.Metrics = projectMetrics(q.Metrics, catalog, rowValues(rows[0], q.Metrics))
	}
	return out
}

func extractAggregations(data any) map[string]any {
	obj, ok := asMap(data)
	if !ok {
		return map[string]any{}
	}
	if inner, ok := asMap(obj["aggregations"]); ok {
		return inner
	}
	if inner, ok := asMap(obj["aggs"]); ok {
		return inner
	}
	return obj
}

func walkBuckets(aggs map[string]any, dimensions []string, compiled *CompiledQuery, catalog *Registry, prefix map[string]any) []map[string]any {
	if len(dimensions) == 0 {
		row := cloneRow(prefix)
		vals := readMetricValues(aggs, compiled)
		metrics := projectMetrics(compiled.Metrics, catalog, vals)
		for k, v := range metrics {
			row[k] = v
		}
		return []map[string]any{row}
	}
	name := dimensions[0]
	bucketAgg, _ := asMap(aggs[aggBucketName(name)])
	rawBuckets := bucketAgg["buckets"]
	buckets, _ := asSlice(rawBuckets)
	if len(buckets) == 0 {
		vals := readMetricValues(aggs, compiled)
		row := cloneRow(prefix)
		for k, v := range projectMetrics(compiled.Metrics, catalog, vals) {
			row[k] = v
		}
		return []map[string]any{row}
	}
	var rows []map[string]any
	for _, raw := range buckets {
		bucket, ok := asMap(raw)
		if !ok {
			continue
		}
		next := cloneRow(prefix)
		next[name] = bucketKey(bucket)
		nested := walkBuckets(bucket, dimensions[1:], compiled, catalog, next)
		rows = append(rows, nested...)
	}
	return rows
}

func readMetricValues(aggs map[string]any, compiled *CompiledQuery) map[string]float64 {
	vals := map[string]float64{}
	if compiled == nil {
		return vals
	}
	for _, name := range compiled.LeafMetrics {
		if agg, ok := asMap(aggs[aggName(name)]); ok {
			if v, ok := asFloat(agg["value"]); ok {
				vals[normalizeKey(name)] = v
			}
		}
	}
	return vals
}

func projectMetrics(names []string, catalog *Registry, leaves map[string]float64) map[string]any {
	out := map[string]any{}
	vals := map[string]float64{}
	for k, v := range leaves {
		vals[normalizeKey(k)] = v
	}
	for _, name := range names {
		m, ok := catalog.GetMetric(name)
		if !ok {
			continue
		}
		if m.IsDerived() {
			for _, dep := range m.Formula.Dependencies() {
				if dm, ok := catalog.GetMetric(dep); ok {
					if v, exists := leaves[normalizeKey(dm.Name)]; exists {
						vals[normalizeKey(dep)] = v
						vals[normalizeKey(dm.Key)] = v
						vals[normalizeKey(dm.Name)] = v
					}
				}
			}
			if v, ok := m.Formula.Eval(vals); ok {
				out[m.Name] = v
			} else {
				out[m.Name] = nil
			}
			continue
		}
		if v, ok := leaves[normalizeKey(m.Name)]; ok {
			out[m.Name] = v
		}
	}
	return out
}

func rowValues(row map[string]any, metrics []string) map[string]float64 {
	out := map[string]float64{}
	for _, name := range metrics {
		if v, ok := asFloat(row[name]); ok {
			out[normalizeKey(name)] = v
		}
	}
	return out
}

func sortRows(rows []map[string]any, order []OrderBy, catalog *Registry) []map[string]any {
	if len(order) == 0 || len(rows) < 2 {
		return rows
	}
	o := order[0]
	field := o.Field
	if m, ok := catalog.GetMetric(field); ok {
		field = m.Name
	}
	if d, ok := catalog.GetDimension(field); ok {
		field = d.Name
	}
	desc := strings.ToLower(strings.TrimSpace(o.Direction)) != "asc"
	sort.SliceStable(rows, func(i, j int) bool {
		vi, _ := asFloat(rows[i][field])
		vj, _ := asFloat(rows[j][field])
		if desc {
			return vi > vj
		}
		return vi < vj
	})
	return rows
}

func bucketKey(bucket map[string]any) any {
	if v, ok := bucket["key_as_string"]; ok {
		return v
	}
	return bucket["key"]
}

func cloneRow(in map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func asMap(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	return m, ok
}

func asSlice(v any) ([]any, bool) {
	s, ok := v.([]any)
	return s, ok
}

func asFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	case nil:
		return 0, false
	default:
		var f float64
		_, err := fmt.Sscan(strings.TrimSpace(fmt.Sprint(n)), &f)
		return f, err == nil
	}
}
