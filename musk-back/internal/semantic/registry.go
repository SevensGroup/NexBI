package semantic

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// SemanticRegistry is the spec name for Registry.
type SemanticRegistry = Registry

// Registry holds advertising Metrics, Dimensions, Entities, and DataSources.
type Registry struct {
	mu         sync.RWMutex
	metrics    map[string]Metric
	metricIdx  map[string]string
	dimensions map[string]Dimension
	dimIdx     map[string]string
	entities   map[string]Entity
	sources    map[string]DataSource
}

// NewRegistry constructs an empty SemanticRegistry.
func NewRegistry() *Registry {
	return &Registry{
		metrics:    map[string]Metric{},
		metricIdx:  map[string]string{},
		dimensions: map[string]Dimension{},
		dimIdx:     map[string]string{},
		entities:   map[string]Entity{},
		sources:    map[string]DataSource{},
	}
}

// RegisterMetric adds a metric. Name and aliases are unique.
func (r *Registry) RegisterMetric(m Metric) error {
	if r == nil {
		return fmt.Errorf("%w: registry is required", ErrInvalidQuery)
	}
	m.Key = firstNonEmpty(m.Key, m.Name)
	m.Name = firstNonEmpty(m.Name, m.Key)
	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("%w: metric name is required", ErrInvalidQuery)
	}
	expr := formulaExpr(m)
	if expr != "" {
		f, err := ParseFormula(expr)
		if err != nil {
			return fmt.Errorf("metric %s: %w", m.Name, err)
		}
		m.Formula = f
	} else if strings.TrimSpace(m.Field) == "" {
		return fmt.Errorf("%w: metric %s needs a field or formula", ErrInvalidQuery, m.Name)
	}
	if strings.TrimSpace(m.Aggregation) == "" && !m.IsDerived() {
		m.Aggregation = "sum"
	}

	canon := m.Name
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.metrics[normalizeKey(canon)]; ok {
		return fmt.Errorf("%w: metric %s already registered", ErrInvalidQuery, canon)
	}
	keys := append([]string{m.Key, m.Name}, m.Aliases...)
	for _, k := range keys {
		nk := normalizeKey(k)
		if nk == "" {
			continue
		}
		if existing, ok := r.metricIdx[nk]; ok && !strings.EqualFold(existing, canon) {
			return fmt.Errorf("%w: metric alias %q already bound to %s", ErrInvalidQuery, k, existing)
		}
	}
	r.metrics[normalizeKey(canon)] = m
	for _, k := range keys {
		nk := normalizeKey(k)
		if nk == "" {
			continue
		}
		r.metricIdx[nk] = canon
	}
	return nil
}

// GetMetric looks up a metric by name, key, or alias.
func (r *Registry) GetMetric(name string) (Metric, bool) {
	if r == nil {
		return Metric{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	canon, ok := r.metricIdx[normalizeKey(name)]
	if !ok {
		return Metric{}, false
	}
	m, ok := r.metrics[normalizeKey(canon)]
	return m, ok
}

// ListMetrics returns registered metrics sorted by name.
func (r *Registry) ListMetrics() []Metric {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Metric, 0, len(r.metrics))
	for _, m := range r.metrics {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

// RegisterDimension adds a dimension. Name and aliases are unique.
func (r *Registry) RegisterDimension(d Dimension) error {
	if r == nil {
		return fmt.Errorf("%w: registry is required", ErrInvalidQuery)
	}
	d.Key = firstNonEmpty(d.Key, d.Name)
	d.Name = firstNonEmpty(d.Name, d.Key)
	if strings.TrimSpace(d.Name) == "" {
		return fmt.Errorf("%w: dimension name is required", ErrInvalidQuery)
	}
	if strings.TrimSpace(d.Field) == "" {
		return fmt.Errorf("%w: dimension %s field is required", ErrInvalidQuery, d.Name)
	}
	if strings.TrimSpace(d.Type) == "" {
		d.Type = "keyword"
	}

	canon := d.Name
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.dimensions[normalizeKey(canon)]; ok {
		return fmt.Errorf("%w: dimension %s already registered", ErrInvalidQuery, canon)
	}
	keys := append([]string{d.Key, d.Name}, d.Aliases...)
	for _, k := range keys {
		nk := normalizeKey(k)
		if nk == "" {
			continue
		}
		if existing, ok := r.dimIdx[nk]; ok && !strings.EqualFold(existing, canon) {
			return fmt.Errorf("%w: dimension alias %q already bound to %s", ErrInvalidQuery, k, existing)
		}
	}
	r.dimensions[normalizeKey(canon)] = d
	for _, k := range keys {
		nk := normalizeKey(k)
		if nk == "" {
			continue
		}
		r.dimIdx[nk] = canon
	}
	for canonVal, aliases := range d.Values {
		for _, a := range append([]string{canonVal}, aliases...) {
			nk := normalizeKey(a)
			if nk == "" {
				continue
			}
			if existing, ok := r.dimIdx["value:"+nk]; ok && existing != d.Name+"="+canonVal {
				continue
			}
			r.dimIdx["value:"+nk] = d.Name + "=" + canonVal
		}
	}
	return nil
}

// GetDimension looks up a dimension by name, key, or alias.
func (r *Registry) GetDimension(name string) (Dimension, bool) {
	if r == nil {
		return Dimension{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	canon, ok := r.dimIdx[normalizeKey(name)]
	if !ok {
		return Dimension{}, false
	}
	d, ok := r.dimensions[normalizeKey(canon)]
	return d, ok
}

// ListDimensions returns registered dimensions sorted by name.
func (r *Registry) ListDimensions() []Dimension {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Dimension, 0, len(r.dimensions))
	for _, d := range r.dimensions {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

// RegisterEntity adds an entity definition.
func (r *Registry) RegisterEntity(e Entity) error {
	if r == nil {
		return fmt.Errorf("%w: registry is required", ErrInvalidQuery)
	}
	e.Key = firstNonEmpty(e.Key, e.Name)
	e.Name = firstNonEmpty(e.Name, e.Key)
	if strings.TrimSpace(e.Name) == "" {
		return fmt.Errorf("%w: entity name is required", ErrInvalidQuery)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.entities[normalizeKey(e.Name)]; ok {
		return fmt.Errorf("%w: entity %s already registered", ErrInvalidQuery, e.Name)
	}
	r.entities[normalizeKey(e.Name)] = e
	if k := normalizeKey(e.Key); k != "" {
		if _, ok := r.entities[k]; !ok {
			r.entities[k] = e
		}
	}
	return nil
}

// GetEntity looks up an entity by name or key.
func (r *Registry) GetEntity(name string) (Entity, bool) {
	if r == nil {
		return Entity{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.entities[normalizeKey(name)]
	return e, ok
}

// ListEntities returns registered entities (canonical names only).
func (r *Registry) ListEntities() []Entity {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	seen := map[string]struct{}{}
	out := make([]Entity, 0, len(r.entities))
	for _, e := range r.entities {
		if _, ok := seen[normalizeKey(e.Name)]; ok {
			continue
		}
		seen[normalizeKey(e.Name)] = struct{}{}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

// RegisterDataSource adds an Elasticsearch-backed source.
func (r *Registry) RegisterDataSource(s DataSource) error {
	if r == nil {
		return fmt.Errorf("%w: registry is required", ErrInvalidQuery)
	}
	s.Key = firstNonEmpty(s.Key, s.Name)
	s.Name = firstNonEmpty(s.Name, s.Key)
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("%w: data source name is required", ErrInvalidQuery)
	}
	if strings.TrimSpace(s.Index) == "" {
		return fmt.Errorf("%w: data source %s index is required", ErrInvalidQuery, s.Name)
	}
	if strings.TrimSpace(s.DateField) == "" {
		s.DateField = "date"
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sources[normalizeKey(s.Name)] = s
	if k := normalizeKey(s.Key); k != "" {
		r.sources[k] = s
	}
	return nil
}

// GetDataSource looks up a data source by name or key.
func (r *Registry) GetDataSource(name string) (DataSource, bool) {
	if r == nil {
		return DataSource{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sources[normalizeKey(name)]
	return s, ok
}

// ListDataSources returns unique data sources.
func (r *Registry) ListDataSources() []DataSource {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	seen := map[string]struct{}{}
	out := make([]DataSource, 0, len(r.sources))
	for _, s := range r.sources {
		if _, ok := seen[normalizeKey(s.Name)]; ok {
			continue
		}
		seen[normalizeKey(s.Name)] = struct{}{}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

// DefaultDataSource returns the default source, or the only source.
func (r *Registry) DefaultDataSource() (DataSource, bool) {
	sources := r.ListDataSources()
	for _, s := range sources {
		if s.Default {
			return s, true
		}
	}
	if len(sources) == 1 {
		return sources[0], true
	}
	return DataSource{}, false
}

// LookupDimensionValue maps a raw token such as "Meta" onto (Channel, Meta).
func (r *Registry) LookupDimensionValue(token string) (Dimension, string, bool) {
	if r == nil {
		return Dimension{}, "", false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	binding, ok := r.dimIdx["value:"+normalizeKey(token)]
	if !ok {
		return Dimension{}, "", false
	}
	dimName, value, ok := strings.Cut(binding, "=")
	if !ok {
		return Dimension{}, "", false
	}
	d, found := r.dimensions[normalizeKey(dimName)]
	return d, value, found
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

func formulaExpr(m Metric) string {
	if m.Formula != nil {
		return strings.TrimSpace(m.Formula.Expression)
	}
	return ""
}
