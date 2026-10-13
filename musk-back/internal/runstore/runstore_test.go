package runstore

import (
	"context"
	"testing"
	"time"
)

func TestMemoryStoreLifecycle(t *testing.T) {
	s := NewMemoryStore()
	run := &Run{ID: "r1", Goal: "analyze roi", Status: StatusCreated}
	if err := s.Create(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(context.Background(), "r1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Goal != "analyze roi" || got.Status != StatusCreated {
		t.Fatalf("got = %+v", got)
	}

	updated, err := s.Update(context.Background(), "r1", func(r *Run) {
		r.Status = StatusRunning
		r.Steps = append(r.Steps, Step{ID: "s1", Index: 0, StartedAt: time.Now()})
		r.Decisions = append(r.Decisions, DecisionRecord{Type: "CALL_TOOL", Reason: "need data", Confidence: 0.7})
		r.ToolCalls = append(r.ToolCalls, ToolCallRecord{ID: "tc1", Tool: "semantic_query", Success: true})
		r.Observations = append(r.Observations, ObservationRecord{Summary: "semantic_query result", Evidence: "roas=1.2"})
		r.Reflections = append(r.Reflections, ReflectionRecord{NextAction: "CONTINUE", EvidenceSufficient: false})
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != StatusRunning || len(updated.Steps) != 1 || len(updated.Decisions) != 1 {
		t.Fatalf("updated = %+v", updated)
	}

	_, err = s.Update(context.Background(), "r1", func(r *Run) {
		r.Status = StatusCompleted
		r.FinalResult = "roi dropped because CPC rose"
		r.Answer = "roi dropped because CPC rose"
		r.Confidence = 0.93
		r.Evidence = []any{"campaign A CPC +40%"}
	})
	if err != nil {
		t.Fatal(err)
	}
	done, err := s.Get(context.Background(), "r1")
	if err != nil {
		t.Fatal(err)
	}
	if done.Status != StatusCompleted || done.FinalResult == "" || done.Confidence != 0.93 {
		t.Fatalf("done = %+v", done)
	}

	if _, err := s.Get(context.Background(), "missing"); err != ErrNotFound {
		t.Fatalf("err = %v", err)
	}
}
