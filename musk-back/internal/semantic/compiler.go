package semantic

import (
	"fmt"
	"strings"
	"time"
)

const aggPrefix = "by_"

// Compiler translates a SemanticQuery into Elasticsearch Query DSL.
type Compiler struct {
	catalog *Registry
}

// NewCompiler constructs a Semantic Query Compiler.
func NewCompiler(catalog *Registry) *Compiler {
	return &Compiler{catalog: catalog}
}

// Compile maps SemanticQuery → Elasticsearch Query DSL.
// The agent must never produce this DSL itself.
func (c *Compiler) Compile(q *Query) (*CompiledQuery, error) {
	if c == nil || c.catalog == nil {
		return nil, fmt.Errorf("%w: catalog is required", ErrCompile)
	}
	if q == nil {
		return nil, fmt.Errorf("%w: query is required", ErrCompile)
	}

	src, err := c.resolveSource(q)
	if err != nil {
		return nil, err
	}

	leaves, formulas, err := c.collectMetrics(q.Metrics)
	if err != nil {
		return nil, err
	}

	filters, err := c.compileFilters(q.Filters, q.TimeRange, src)
	if err != nil {
		return nil, err
	}

	metricAggs, err := c.metricAggregations(leaves)
	if err != nil {
		return nil, err
	}

	aggs := metricAggs
	if len(q.Dimensions) > 0 {
		aggs, err = c.dimensionAggregations(q.Dimensions, metricAggs, q.OrderBy, q.Limit, src)
		if err != nil {
			return nil, err
		}
	}

	query := map[string]any{"match_all": map[string]any{}}
	if len(filters) > 0 {
		query = map[string]any{
			"bool": map[string]any{
				"filter": filters,
			},
		}
	}

	leafNames := make([]string, 0, len(leaves))
	for _, m := range leaves {
		leafNames = append(leafNames, m.Name)
	}

	return &CompiledQuery{
		Index:        src.Index,
		Query:        query,
		Aggregations: aggs,
		Size:         0,
		Formulas:     formulas,
		Metrics:      append([]string(nil), q.Metrics...),
		Dimensions:   append([]string(nil), q.Dimensions...),
		LeafMetrics:  leafNames,
	}, nil
}

func (c *Compiler) resolveSource(q *Query) (DataSource, error) {
	if q.DataSource != "" {
		src, ok := c.catalog.GetDataSource(q.DataSource)
		if !ok {
			return DataSource{}, fmt.Errorf("%w: unknown data source %s", ErrCompile, q.DataSource)
		}
		return src, nil
	}
	src, ok := c.catalog.DefaultDataSource()
	if !ok {
		return DataSource{}, fmt.Errorf("%w: no data source is configured", ErrCompile)
	}
	return src, nil
}

func (c *Compiler) collectMetrics(names []string) ([]Metric, []*Formula, error) {
	leaves := map[string]Metric{}
	var formulas []*Formula
	var walk func(string) error
	walk = func(name string) error {
		m, ok := c.catalog.GetMetric(name)
		if !ok {
			return fmt.Errorf("%w: %s", ErrUnknownMetric, name)
		}
		if m.IsDerived() {
			formulas = append(formulas, m.Formula)
			for _, dep := range m.Formula.Dependencies() {
				if err := walk(dep); err != nil {
					return fmt.Errorf("metric %s: %w", m.Name, err)
				}
			}
			return nil
		}
		leaves[normalizeKey(m.Name)] = m
		return nil
	}
	for _, name := range names {
		if err := walk(name); err != nil {
			return nil, nil, err
		}
	}
	out := make([]Metric, 0, len(leaves))
	for _, m := range leaves {
		out = append(out, m)
	}
	return out, formulas, nil
}

func (c *Compiler) compileFilters(filters []Filter, window TimeRange, src DataSource) ([]any, error) {
	out := make([]any, 0, len(filters)+1)
	if !window.IsZero() {
		rangeQ := map[string]any{
			"gte": formatTime(window.Start),
			"lt":  formatTime(window.End),
		}
		out = append(out, map[string]any{
			"range": map[string]any{
				src.DateField: rangeQ,
			},
		})
	}
	for _, f := range filters {
		d, ok := c.catalog.GetDimension(f.Dimension)
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrUnknownFilter, f.Dimension)
		}
		clause, err := filterClause(d.Field, f)
		if err != nil {
			return nil, err
		}
		out = append(out, clause)
	}
	return out, nil
}

func filterClause(field string, f Filter) (map[string]any, error) {
	op := strings.ToLower(strings.TrimSpace(f.Op))
	if op == "" {
		op = "eq"
	}
	switch op {
	case "eq", "=", "term":
		return map[string]any{"term": map[string]any{field: f.Value}}, nil
	case "in":
		values, err := asValues(f.Value)
		if err != nil {
			return nil, err
		}
		return map[string]any{"terms": map[string]any{field: values}}, nil
	case "neq", "ne", "!=":
		return map[string]any{
			"bool": map[string]any{
				"must_not": []any{
					map[string]any{"term": map[string]any{field: f.Value}},
				},
			},
		}, nil
	default:
		return nil, fmt.Errorf("%w: filter op %q is not allowed", ErrCompile, f.Op)
	}
}

func asValues(v any) ([]any, error) {
	switch typed := v.(type) {
	case []any:
		return typed, nil
	case []string:
		out := make([]any, len(typed))
		for i, s := range typed {
			out[i] = s
		}
		return out, nil
	default:
		return []any{v}, nil
	}
}

func (c *Compiler) metricAggregations(leaves []Metric) (map[string]any, error) {
	aggs := map[string]any{}
	for _, m := range leaves {
		aggType := strings.ToLower(strings.TrimSpace(m.Aggregation))
		if aggType == "" {
			aggType = "sum"
		}
		switch aggType {
		case "sum", "avg", "min", "max", "value_count", "cardinality":
			aggs[aggName(m.Name)] = map[string]any{
				aggType: map[string]any{"field": m.Field},
			}
		default:
			return nil, fmt.Errorf("%w: unsupported aggregation %s for metric %s", ErrCompile, aggType, m.Name)
		}
	}
	return aggs, nil
}

func (c *Compiler) dimensionAggregations(dimensions []string, metricAggs map[string]any, order []OrderBy, limit int, src DataSource) (map[string]any, error) {
	if limit < 1 {
		limit = defaultLimit
	}
	inner := metricAggs
	for i := len(dimensions) - 1; i >= 0; i-- {
		d, ok := c.catalog.GetDimension(dimensions[i])
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrUnknownDimension, dimensions[i])
		}
		bucket := map[string]any{}
		if strings.EqualFold(d.Type, "date") || strings.EqualFold(d.Name, "Date") {
			field := d.Field
			if field == "" {
				field = src.DateField
			}
			dateAgg := map[string]any{
				"field":             field,
				"calendar_interval": "day",
				"min_doc_count":     1,
			}
			if dir := termsOrder(c.catalog, order, inner); dir != nil {
				dateAgg["order"] = dir
			}
			bucket["date_histogram"] = dateAgg
		} else {
			terms := map[string]any{
				"field": d.Field,
				"size":  limit,
			}
			if dir := termsOrder(c.catalog, order, inner); dir != nil {
				terms["order"] = dir
			}
			bucket["terms"] = terms
		}
		if len(inner) > 0 {
			bucket["aggs"] = inner
		}
		inner = map[string]any{
			aggBucketName(d.Name): bucket,
		}
	}
	return inner, nil
}

func termsOrder(catalog *Registry, order []OrderBy, metricAggs map[string]any) map[string]any {
	if len(order) == 0 {
		return nil
	}
	o := order[0]
	dir := strings.ToLower(strings.TrimSpace(o.Direction))
	if dir != "asc" {
		dir = "desc"
	}
	field := strings.TrimSpace(o.Field)
	if _, ok := metricAggs[aggName(field)]; ok {
		return map[string]any{aggName(field): dir}
	}
	if m, ok := catalog.GetMetric(field); ok {
		if m.IsDerived() {
			deps := m.Formula.Dependencies()
			if len(deps) > 0 {
				if _, exists := metricAggs[aggName(deps[0])]; exists {
					return map[string]any{aggName(deps[0]): dir}
				}
				if dep, ok := catalog.GetMetric(deps[0]); ok {
					if _, exists := metricAggs[aggName(dep.Name)]; exists {
						return map[string]any{aggName(dep.Name): dir}
					}
				}
			}
		}
		if _, exists := metricAggs[aggName(m.Name)]; exists {
			return map[string]any{aggName(m.Name): dir}
		}
	}
	return nil
}

func aggName(metric string) string {
	return "m_" + normalizeKey(metric)
}

func aggBucketName(dimension string) string {
	return aggPrefix + normalizeKey(dimension)
}

func formatTime(t time.Time) string {
	if t.Location() == time.UTC || t.Location() == nil {
		return t.UTC().Format(time.RFC3339)
	}
	return t.UTC().Format(time.RFC3339)
}
