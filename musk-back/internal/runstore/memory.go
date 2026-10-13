package runstore

import (
	"context"
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

func (s *memoryStore) Close() error { return nil }
