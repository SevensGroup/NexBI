package runstore

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

type memoryStore struct {
	mu   sync.RWMutex
	runs map[string]*Run
}

// NewMemoryStore is the in-process Agent Run store (tests / local).
func NewMemoryStore() Store {
	return &memoryStore{runs: map[string]*Run{}}
}

func (s *memoryStore) Create(_ context.Context, run *Run) error {
	if s == nil || run == nil {
		return ErrNotFound
	}
	id := strings.TrimSpace(run.ID)
	if id == "" {
		return ErrNotFound
	}
	now := time.Now().UTC()
	if run.CreatedAt.IsZero() {
		run.CreatedAt = now
	}
	run.UpdatedAt = now
	if run.Status == "" {
		run.Status = StatusCreated
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[id] = run.Clone()
	return nil
}

func (s *memoryStore) Get(_ context.Context, id string) (*Run, error) {
	id = strings.TrimSpace(id)
	if s == nil || id == "" {
		return nil, ErrNotFound
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	run, ok := s.runs[id]
	if !ok {
		return nil, ErrNotFound
	}
	return run.Clone(), nil
}

func (s *memoryStore) Save(_ context.Context, run *Run) error {
	if s == nil || run == nil {
		return ErrNotFound
	}
	id := strings.TrimSpace(run.ID)
	if id == "" {
		return ErrNotFound
	}
	run.UpdatedAt = time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[id] = run.Clone()
	return nil
}

func (s *memoryStore) Update(_ context.Context, id string, mut func(*Run)) (*Run, error) {
	id = strings.TrimSpace(id)
	if s == nil || id == "" {
		return nil, ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.runs[id]
	if !ok {
		return nil, ErrNotFound
	}
	next := run.Clone()
	if mut != nil {
		mut(next)
	}
	next.ID = id
	next.UpdatedAt = time.Now().UTC()
	s.runs[id] = next
	return next.Clone(), nil
}

func (s *memoryStore) List(_ context.Context, offset, limit int) (Page, error) {
	if s == nil {
		return Page{}, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Summary, 0, len(s.runs))
	for _, run := range s.runs {
		out = append(out, Summarize(run))
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].UpdatedAt.After(out[j].UpdatedAt)
		}
		return out[i].ID > out[j].ID
	})
	return slicePage(out, offset, limit), nil
}

func slicePage(out []Summary, offset, limit int) Page {
	offset, limit = listWindow(offset, limit)
	total := len(out)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	runs := append([]Summary(nil), out[offset:end]...)
	if runs == nil {
		runs = []Summary{}
	}
	return Page{Runs: runs, Total: total}
}

func (s *memoryStore) Close() error { return nil }
