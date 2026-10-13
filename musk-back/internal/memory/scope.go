package memory

import (
	"context"
	"fmt"
	"strings"
)

// scopedStore isolates one agent's memory: tagged metadata and namespaced run ids.
type scopedStore struct {
	inner MemoryStore
	agent string
}

// Scope wraps store so Save/Get/Search/Delete only see records for agent.
// Close is a no-op so a specialist cannot shut down the parent store.
func Scope(store MemoryStore, agent string) MemoryStore {
	agent = normalizeAgent(agent)
	if store == nil || agent == "" {
		return store
	}
	return &scopedStore{inner: store, agent: agent}
}

func (s *scopedStore) Save(ctx context.Context, rec Record) error {
	if s == nil || s.inner == nil {
		return fmt.Errorf("%w: store is required", ErrInvalidRecord)
	}
	rec = applyAgentScope(rec, s.agent)
	return s.inner.Save(ctx, rec)
}

func (s *scopedStore) Get(ctx context.Context, id string) (*Record, error) {
	if s == nil || s.inner == nil {
		return nil, ErrNotFound
	}
	rec, err := s.inner.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if rec == nil || !belongsToAgent(*rec, s.agent) {
		return nil, ErrNotFound
	}
	return rec, nil
}

func (s *scopedStore) Search(ctx context.Context, q Query) (SearchResult, error) {
	if s == nil || s.inner == nil {
		return SearchResult{}, nil
	}
	q = scopeQuery(q, s.agent)
	return s.inner.Search(ctx, q)
}

func (s *scopedStore) Delete(ctx context.Context, id string) error {
	if s == nil || s.inner == nil {
		return ErrNotFound
	}
	rec, err := s.inner.Get(ctx, id)
	if err != nil {
		return err
	}
	if rec == nil || !belongsToAgent(*rec, s.agent) {
		return ErrNotFound
	}
	return s.inner.Delete(ctx, id)
}

func (s *scopedStore) Close() error {
	return nil
}

func applyAgentScope(rec Record, agent string) Record {
	agent = normalizeAgent(agent)
	if agent == "" {
		return rec
	}
	if rec.Metadata == nil {
		rec.Metadata = map[string]any{}
	}
	rec.Metadata[metadataAgent] = agent
	if rec.Layer != LayerLongTerm {
		rec.RunID = ScopeRunID(rec.RunID, agent)
	}
	return rec
}

func scopeQuery(q Query, agent string) Query {
	agent = normalizeAgent(agent)
	if agent == "" {
		return q
	}
	if q.Metadata == nil {
		q.Metadata = map[string]any{}
	} else {
		cloned := make(map[string]any, len(q.Metadata)+1)
		for k, v := range q.Metadata {
			cloned[k] = v
		}
		q.Metadata = cloned
	}
	q.Metadata[metadataAgent] = agent
	if q.Layer != LayerLongTerm && strings.TrimSpace(q.RunID) != "" {
		q.RunID = ScopeRunID(q.RunID, agent)
	}
	if q.Layer == LayerLongTerm {
		q.RunID = ""
	}
	return q
}

func belongsToAgent(rec Record, agent string) bool {
	agent = normalizeAgent(agent)
	if agent == "" {
		return true
	}
	if rec.Metadata != nil {
		if got := normalizeAgent(fmt.Sprint(rec.Metadata[metadataAgent])); got == agent {
			return true
		}
	}
	return strings.HasSuffix(strings.TrimSpace(rec.RunID), ":"+agent)
}
