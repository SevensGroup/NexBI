package semantic

import (
	"fmt"
	"strings"
	"time"
)

const (
	defaultMaxTimeRange  = 90 * 24 * time.Hour
	defaultMaxResultSize = 100
	defaultLimit         = 10
)

// Validator enforces allowed metrics, dimensions, filters, time range, and result size.
type Validator struct {
	catalog *Registry
	limits  Limits
}

// NewValidator constructs a Query Validator.
func NewValidator(catalog *Registry, limits Limits) *Validator {
	if limits.MaxTimeRange <= 0 {
		limits.MaxTimeRange = defaultMaxTimeRange
	}
	if limits.MaxResultSize < 1 {
		limits.MaxResultSize = defaultMaxResultSize
	}
	return &Validator{catalog: catalog, limits: limits}
}

// Validate checks a SemanticQuery against the catalog and configured limits.
func (v *Validator) Validate(q *Query) error {
	if v == nil || v.catalog == nil {
		return fmt.Errorf("%w: catalog is required", ErrInvalidQuery)
	}
	if q == nil {
		return fmt.Errorf("%w: query is required", ErrInvalidQuery)
	}
	if len(q.Metrics) == 0 {
		return fmt.Errorf("%w: at least one metric is required", ErrInvalidQuery)
	}

	for _, name := range q.Metrics {
		if _, ok := v.catalog.GetMetric(name); !ok {
			return fmt.Errorf("%w: %s", ErrUnknownMetric, name)
		}
	}
	for _, name := range q.Dimensions {
		if _, ok := v.catalog.GetDimension(name); !ok {
			return fmt.Errorf("%w: %s", ErrUnknownDimension, name)
		}
	}
	for _, f := range q.Filters {
		if strings.TrimSpace(f.Dimension) == "" {
			return fmt.Errorf("%w: filter dimension is required", ErrUnknownFilter)
		}
		if _, ok := v.catalog.GetDimension(f.Dimension); !ok {
			return fmt.Errorf("%w: %s", ErrUnknownFilter, f.Dimension)
		}
		if !allowedFilterOp(f.Op) {
			return fmt.Errorf("%w: filter op %q is not allowed", ErrInvalidQuery, f.Op)
		}
		if f.Value == nil || strings.TrimSpace(fmt.Sprint(f.Value)) == "" {
			return fmt.Errorf("%w: filter value is required", ErrInvalidQuery)
		}
	}
	if !q.TimeRange.IsZero() {
		if q.TimeRange.Start.IsZero() || q.TimeRange.End.IsZero() {
			return fmt.Errorf("%w: time_range start and end are required", ErrInvalidQuery)
		}
		if !q.TimeRange.End.After(q.TimeRange.Start) {
			return fmt.Errorf("%w: time_range end must be after start", ErrInvalidQuery)
		}
		if q.TimeRange.Duration() > v.limits.MaxTimeRange {
			return fmt.Errorf("%w: time range %s exceeds max %s", ErrInvalidQuery, q.TimeRange.Duration(), v.limits.MaxTimeRange)
		}
	}
	if q.Limit < 0 {
		return fmt.Errorf("%w: limit must be >= 0", ErrInvalidQuery)
	}
	if q.Limit > v.limits.MaxResultSize {
		return fmt.Errorf("%w: limit %d exceeds max result size %d", ErrInvalidQuery, q.Limit, v.limits.MaxResultSize)
	}
	if q.DataSource != "" {
		if _, ok := v.catalog.GetDataSource(q.DataSource); !ok {
			return fmt.Errorf("%w: unknown data source %s", ErrInvalidQuery, q.DataSource)
		}
	}
	for _, o := range q.OrderBy {
		if strings.TrimSpace(o.Field) == "" {
			return fmt.Errorf("%w: order_by field is required", ErrInvalidQuery)
		}
		_, metricOK := v.catalog.GetMetric(o.Field)
		_, dimOK := v.catalog.GetDimension(o.Field)
		if !metricOK && !dimOK {
			return fmt.Errorf("%w: order_by field %s is not an allowed metric or dimension", ErrInvalidQuery, o.Field)
		}
		dir := strings.ToLower(strings.TrimSpace(o.Direction))
		if dir != "" && dir != "asc" && dir != "desc" {
			return fmt.Errorf("%w: order_by direction %q is not allowed", ErrInvalidQuery, o.Direction)
		}
	}
	return nil
}

func allowedFilterOp(op string) bool {
	switch strings.ToLower(strings.TrimSpace(op)) {
	case "", "eq", "=", "term", "in", "neq", "ne", "!=":
		return true
	default:
		return false
	}
}

func applyQueryDefaults(q *Query, limits Limits) {
	if q == nil {
		return
	}
	if q.Limit == 0 {
		q.Limit = defaultLimit
		if limits.MaxResultSize > 0 && q.Limit > limits.MaxResultSize {
			q.Limit = limits.MaxResultSize
		}
	}
}
