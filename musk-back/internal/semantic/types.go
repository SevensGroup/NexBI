package semantic

import "time"

// Metric is a business measure. Leaf metrics map to a field; derived metrics use Formula.
type Metric struct {
	Key         string
	Name        string
	Description string
	Formula     *Formula
	Field       string
	Aggregation string
	Aliases     []string
}

// Dimension is a business breakdown or filter key mapped to a field.
type Dimension struct {
	Key         string
	Name        string
	Description string
	Field       string
	Type        string
	Entity      string
	Aliases     []string
	Values      map[string][]string
}

// Entity is a real-world advertising object such as a campaign or creative.
type Entity struct {
	Key         string
	Name        string
	Description string
	KeyField    string
	Dimensions  []string
}

// DataSource is the Elasticsearch index (and date field) behind the catalog.
type DataSource struct {
	Key         string
	Name        string
	Description string
	Index       string
	DateField   string
	Default     bool
}

// Filter is a dimension predicate on a SemanticQuery.
type Filter struct {
	Dimension string `json:"dimension"`
	Op        string `json:"op"`
	Value     any    `json:"value"`
}

// TimeRange is an inclusive-start / exclusive-end window.
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
	Label string    `json:"label,omitempty"`
}

// OrderBy sorts result rows by a metric or dimension.
type OrderBy struct {
	Field     string `json:"field"`
	Direction string `json:"direction,omitempty"`
}

// Query is the business-level SemanticQuery. Agents emit this, never ES DSL.
type Query struct {
	Metrics    []string  `json:"metrics"`
	Dimensions []string  `json:"dimensions"`
	Filters    []Filter  `json:"filters"`
	TimeRange  TimeRange `json:"time_range"`
	OrderBy    []OrderBy `json:"order_by"`
	Limit      int       `json:"limit"`
	DataSource string    `json:"data_source,omitempty"`
}

// SemanticQuery is the spec name for Query.
type SemanticQuery = Query

// CompiledQuery is Elasticsearch Query DSL produced by the compiler.
type CompiledQuery struct {
	Index        string
	Query        map[string]any
	Aggregations map[string]any
	Size         int
	Formulas     []*Formula
	Metrics      []string
	Dimensions   []string
	LeafMetrics  []string
}

// Limits cap SemanticQuery validation.
type Limits struct {
	MaxTimeRange  time.Duration
	MaxResultSize int
}

func (m Metric) IsDerived() bool {
	return m.Formula != nil && m.Formula.Expression != ""
}

func (t TimeRange) Duration() time.Duration {
	if t.Start.IsZero() || t.End.IsZero() {
		return 0
	}
	if t.End.Before(t.Start) {
		return 0
	}
	return t.End.Sub(t.Start)
}

func (t TimeRange) IsZero() bool {
	return t.Start.IsZero() && t.End.IsZero() && t.Label == ""
}
