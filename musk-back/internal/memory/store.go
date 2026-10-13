package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"autonomous-agent/internal/config"
	"autonomous-agent/internal/logger"
)

const (
	defaultSearchLimit = 20
	maxSearchLimit     = 100
)

// store applies validation, TTL defaults, and sanitization around a backend.
type store struct {
	inner MemoryStore
	cfg   config.MemoryConfig
	now   func() time.Time
}

// Open returns a PostgreSQL store when memory.postgres_url is set, otherwise an in-memory store.
func Open(ctx context.Context, cfg config.MemoryConfig, log logger.Logger) (MemoryStore, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if log == nil {
		log = logger.New("info", "json", nil)
	}

	var inner MemoryStore
	var err error
	if dsn := strings.TrimSpace(cfg.PostgresURL); dsn != "" {
		inner, err = openPostgres(ctx, dsn, log)
		if err != nil {
			return nil, err
		}
	} else {
		inner = newMemStore()
	}
	return wrap(inner, cfg), nil
}

func wrap(inner MemoryStore, cfg config.MemoryConfig) MemoryStore {
	return &store{
		inner: inner,
		cfg:   cfg,
		now:   func() time.Time { return time.Now().UTC() },
	}
}

func (s *store) Save(ctx context.Context, rec Record) error {
	prepared, err := s.prepare(rec)
	if err != nil {
		return err
	}
	return s.inner.Save(ctx, prepared)
}

func (s *store) Get(ctx context.Context, id string) (*Record, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("%w: id is required", ErrInvalidRecord)
	}
	rec, err := s.inner.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if rec == nil || rec.expired(s.now()) {
		return nil, ErrNotFound
	}
	return rec, nil
}

func (s *store) Search(ctx context.Context, q Query) (SearchResult, error) {
	q = s.normalizeQuery(q)
	return s.inner.Search(ctx, q)
}

func (s *store) Delete(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("%w: id is required", ErrInvalidRecord)
	}
	return s.inner.Delete(ctx, id)
}

func (s *store) Close() error {
	if s == nil || s.inner == nil {
		return nil
	}
	return s.inner.Close()
}

func (s *store) prepare(rec Record) (Record, error) {
	rec.ID = strings.TrimSpace(rec.ID)
	if rec.ID == "" {
		rec.ID = uuid.NewString()
	}
	rec.RunID = strings.TrimSpace(rec.RunID)
	rec.TaskID = strings.TrimSpace(rec.TaskID)

	if rec.Layer == "" {
		return Record{}, fmt.Errorf("%w: layer is required", ErrInvalidLayer)
	}
	layer, err := ParseLayer(string(rec.Layer))
	if err != nil {
		return Record{}, err
	}
	rec.Layer = layer

	kind, err := ParseKind(string(rec.Kind))
	if err != nil {
		return Record{}, err
	}
	if !kind.ValidFor(layer) {
		return Record{}, fmt.Errorf("%w: %s is not valid for %s", ErrInvalidKind, kind, layer)
	}
	rec.Kind = kind

	if rec.Layer != LayerLongTerm && rec.RunID == "" {
		return Record{}, fmt.Errorf("%w: run_id is required", ErrInvalidRecord)
	}

	if rec.Content == nil {
		return Record{}, fmt.Errorf("%w: content is required", ErrInvalidRecord)
	}
	rec.Content = Sanitize(cloneJSON(rec.Content))
	rec.Metadata = SanitizeMetadata(rec.Metadata)

	now := s.now()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	} else {
		rec.CreatedAt = rec.CreatedAt.UTC()
	}
	if rec.ExpiresAt == nil {
		if ttl := s.ttlFor(layer); ttl > 0 {
			exp := now.Add(ttl)
			rec.ExpiresAt = &exp
		}
	} else {
		exp := rec.ExpiresAt.UTC()
		rec.ExpiresAt = &exp
	}
	return rec, nil
}

func (s *store) ttlFor(layer Layer) time.Duration {
	switch layer {
	case LayerShortTerm:
		return s.cfg.ShortTermTTL.Duration()
	case LayerTask:
		return s.cfg.TaskTTL.Duration()
	case LayerLongTerm:
		return s.cfg.LongTermTTL.Duration()
	default:
		return 0
	}
}

func (s *store) normalizeQuery(q Query) Query {
	if q.Layer != "" {
		if layer, err := ParseLayer(string(q.Layer)); err == nil {
			q.Layer = layer
		}
	}
	if q.Kind != "" {
		if kind, err := ParseKind(string(q.Kind)); err == nil {
			q.Kind = kind
		}
	}
	q.RunID = strings.TrimSpace(q.RunID)
	q.TaskID = strings.TrimSpace(q.TaskID)
	q.Text = strings.TrimSpace(q.Text)
	if q.Offset < 0 {
		q.Offset = 0
	}
	if q.Limit <= 0 {
		q.Limit = s.cfg.SearchLimit
	}
	if q.Limit <= 0 {
		q.Limit = defaultSearchLimit
	}
	if q.Limit > maxSearchLimit {
		q.Limit = maxSearchLimit
	}
	return q
}

func cloneJSON(v any) any {
	if v == nil {
		return nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return v
	}
	return out
}

func cloneRecord(rec Record) Record {
	rec.Content = cloneJSON(rec.Content)
	meta := cloneJSON(rec.Metadata)
	if m, ok := meta.(map[string]any); ok && m != nil {
		rec.Metadata = m
	} else {
		rec.Metadata = map[string]any{}
	}
	if rec.ExpiresAt != nil {
		exp := rec.ExpiresAt.UTC()
		rec.ExpiresAt = &exp
	}
	return rec
}

func metadataMatches(rec Metadata, want map[string]any) bool {
	if len(want) == 0 {
		return true
	}
	if rec == nil {
		return false
	}
	for k, v := range want {
		got, ok := rec[k]
		if !ok {
			return false
		}
		if fmt.Sprint(got) != fmt.Sprint(v) {
			return false
		}
	}
	return true
}

// Metadata is an alias so tests can construct filters without importing maps everywhere.
type Metadata = map[string]any
