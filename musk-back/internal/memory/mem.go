package memory

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"
)

type memStore struct {
	mu      sync.RWMutex
	records map[string]Record
	closed  bool
	now     func() time.Time
}

func newMemStore() *memStore {
	return &memStore{
		records: map[string]Record{},
		now:     func() time.Time { return time.Now().UTC() },
	}
}

func (s *memStore) Save(_ context.Context, rec Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	s.records[rec.ID] = cloneRecord(rec)
	return nil
}

func (s *memStore) Get(_ context.Context, id string) (*Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, ErrClosed
	}
	rec, ok := s.records[id]
	if !ok || rec.expired(s.now()) {
		return nil, ErrNotFound
	}
	cloned := cloneRecord(rec)
	return &cloned, nil
}

func (s *memStore) Search(_ context.Context, q Query) (SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return SearchResult{}, ErrClosed
	}

	now := s.now()
	matched := make([]Record, 0)
	for _, rec := range s.records {
		if rec.expired(now) {
			continue
		}
		if q.RunID != "" && rec.RunID != q.RunID {
			continue
		}
		if q.Layer != "" && rec.Layer != q.Layer {
			continue
		}
		if q.Kind != "" && rec.Kind != q.Kind {
			continue
		}
		if q.TaskID != "" && rec.TaskID != q.TaskID {
			continue
		}
		if !metadataMatches(rec.Metadata, q.Metadata) {
			continue
		}
		if q.Text != "" && !textMatches(rec, q.Text) {
			continue
		}
		matched = append(matched, cloneRecord(rec))
	}

	sortRecords(matched)
	total := len(matched)
	start := q.Offset
	if start > total {
		start = total
	}
	end := start + q.Limit
	if end > total {
		end = total
	}
	page := matched[start:end]
	if page == nil {
		page = []Record{}
	}
	return SearchResult{Records: page, Total: total, Limit: q.Limit, Offset: q.Offset}, nil
}

func (s *memStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	if _, ok := s.records[id]; !ok {
		return ErrNotFound
	}
	delete(s.records, id)
	return nil
}

func (s *memStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.records = map[string]Record{}
	return nil
}

func textMatches(rec Record, q string) bool {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return true
	}
	hay := strings.ToLower(rec.Kind.String() + " " + string(rec.Layer) + " " + rec.RunID + " " + rec.TaskID + " " + stringify(rec.Content) + " " + stringify(rec.Metadata))
	return strings.Contains(hay, q)
}

func (k Kind) String() string { return string(k) }

func stringify(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(raw)
}

func sortRecords(recs []Record) {
	for i := 0; i < len(recs); i++ {
		for j := i + 1; j < len(recs); j++ {
			if recs[j].CreatedAt.After(recs[i].CreatedAt) || (recs[j].CreatedAt.Equal(recs[i].CreatedAt) && recs[j].ID < recs[i].ID) {
				recs[i], recs[j] = recs[j], recs[i]
			}
		}
	}
}
