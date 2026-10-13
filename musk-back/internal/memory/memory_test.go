package memory

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"autonomous-agent/internal/config"
	"autonomous-agent/internal/logger"
)

func testStore(t *testing.T, ttl time.Duration) MemoryStore {
	t.Helper()
	st, err := Open(context.Background(), config.MemoryConfig{
		ShortTermTTL: config.Duration(ttl),
		TaskTTL:      config.Duration(7 * 24 * time.Hour),
		SearchLimit:  20,
	}, logger.New("error", "json", io.Discard))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestSaveGetDelete(t *testing.T) {
	st := testStore(t, time.Hour)
	ctx := context.Background()

	rec := Record{
		ID:    "mem-1",
		RunID: "run-1",
		Layer: LayerShortTerm,
		Kind:  KindGoal,
		Content: map[string]any{
			"goal": "analyze Meta ROAS",
		},
		Metadata: map[string]any{"source": "test"},
	}
	if err := st.Save(ctx, rec); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := st.Get(ctx, "mem-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.RunID != "run-1" || got.Kind != KindGoal || got.Layer != LayerShortTerm {
		t.Fatalf("record = %+v", got)
	}
	content, _ := got.Content.(map[string]any)
	if content["goal"] != "analyze Meta ROAS" {
		t.Fatalf("content = %#v", got.Content)
	}
	if got.Metadata["source"] != "test" {
		t.Fatalf("metadata = %#v", got.Metadata)
	}

	if err := st.Delete(ctx, "mem-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := st.Get(ctx, "mem-1"); err != ErrNotFound {
		t.Fatalf("Get after delete = %v, want ErrNotFound", err)
	}
}

func TestSearchPaginationAndRunID(t *testing.T) {
	st := testStore(t, time.Hour)
	ctx := context.Background()

	for i, goal := range []string{"alpha", "beta", "gamma", "delta"} {
		if err := st.Save(ctx, Record{
			ID:      "g" + string(rune('1'+i)),
			RunID:   "run-a",
			Layer:   LayerShortTerm,
			Kind:    KindGoal,
			Content: map[string]any{"goal": goal},
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.Save(ctx, Record{
		ID:      "other",
		RunID:   "run-b",
		Layer:   LayerShortTerm,
		Kind:    KindGoal,
		Content: map[string]any{"goal": "other run"},
	}); err != nil {
		t.Fatal(err)
	}

	page1, err := st.Search(ctx, Query{RunID: "run-a", Layer: LayerShortTerm, Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("Search page1: %v", err)
	}
	if page1.Total != 4 {
		t.Fatalf("total = %d, want 4", page1.Total)
	}
	if len(page1.Records) != 2 {
		t.Fatalf("page1 len = %d", len(page1.Records))
	}
	page2, err := st.Search(ctx, Query{RunID: "run-a", Layer: LayerShortTerm, Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("Search page2: %v", err)
	}
	if len(page2.Records) != 2 {
		t.Fatalf("page2 len = %d", len(page2.Records))
	}
	ids := map[string]bool{}
	for _, rec := range append(page1.Records, page2.Records...) {
		ids[rec.ID] = true
		if rec.RunID != "run-a" {
			t.Fatalf("run_id leaked: %+v", rec)
		}
	}
	if len(ids) != 4 {
		t.Fatalf("unique ids = %d, want 4", len(ids))
	}

	text, err := st.Search(ctx, Query{Text: "beta", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if text.Total != 1 {
		t.Fatalf("text total = %d", text.Total)
	}
}

func TestTTLExpiresRecord(t *testing.T) {
	st := testStore(t, time.Hour)
	ctx := context.Background()
	expired := time.Now().UTC().Add(-time.Minute)
	if err := st.Save(ctx, Record{
		ID:        "old",
		RunID:     "run-ttl",
		Layer:     LayerShortTerm,
		Kind:      KindObservation,
		Content:   map[string]any{"summary": "stale"},
		ExpiresAt: &expired,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Get(ctx, "old"); err != ErrNotFound {
		t.Fatalf("expired Get = %v, want ErrNotFound", err)
	}
	res, err := st.Search(ctx, Query{RunID: "run-ttl", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 0 {
		t.Fatalf("expired search total = %d", res.Total)
	}
}

func TestSanitizeRejectsAPIKeysAndCredentials(t *testing.T) {
	st := testStore(t, time.Hour)
	ctx := context.Background()
	if err := st.Save(ctx, Record{
		ID:    "secret",
		RunID: "run-sec",
		Layer: LayerLongTerm,
		Kind:  KindPreference,
		Content: map[string]any{
			"note":     "prefer daily reports, key sk-abcdefghijklmnopqrstuvwxyz",
			"api_key":  "sk-live-should-not-persist",
			"password": "hunter2",
			"nested": map[string]any{
				"authorization": "Bearer abcdefghijklmnop",
			},
		},
		Metadata: map[string]any{
			"access_token": "tok-secret",
			"channel":      "Meta",
		},
	}); err != nil {
		t.Fatal(err)
	}
	got, err := st.Get(ctx, "secret")
	if err != nil {
		t.Fatal(err)
	}
	raw := stringify(got.Content) + stringify(got.Metadata)
	for _, banned := range []string{"sk-live-should-not-persist", "hunter2", "tok-secret", "sk-abcdefghijklmnopqrstuvwxyz", "Bearer abcdefghijklmnop"} {
		if strings.Contains(raw, banned) {
			t.Fatalf("secret %q leaked: %s", banned, raw)
		}
	}
	if !strings.Contains(raw, redacted) {
		t.Fatalf("expected redaction, got %s", raw)
	}
	content, _ := got.Content.(map[string]any)
	if content["api_key"] != redacted || content["password"] != redacted {
		t.Fatalf("keys not redacted: %#v", content)
	}
	if got.Metadata["channel"] != "Meta" {
		t.Fatalf("non-secret metadata dropped: %#v", got.Metadata)
	}
}

func TestKindMustMatchLayer(t *testing.T) {
	st := testStore(t, time.Hour)
	err := st.Save(context.Background(), Record{
		RunID:   "run-1",
		Layer:   LayerShortTerm,
		Kind:    KindPreference,
		Content: "nope",
	})
	if err == nil || !strings.Contains(err.Error(), "invalid memory kind") {
		t.Fatalf("err = %v", err)
	}
}

func TestTaskAndLongTermLayers(t *testing.T) {
	st := testStore(t, time.Hour)
	ctx := context.Background()
	if err := st.Save(ctx, Record{
		ID:     "task-1",
		RunID:  "run-t",
		TaskID: "t1",
		Layer:  LayerTask,
		Kind:   KindEvidence,
		Content: map[string]any{
			"evidence": "ROAS dropped 12%",
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.Save(ctx, Record{
		ID:    "lt-1",
		RunID: "run-t",
		Layer: LayerLongTerm,
		Kind:  KindQueryPattern,
		Content: map[string]any{
			"pattern": "Meta ROAS yesterday",
		},
		Metadata: map[string]any{"metric": "ROAS"},
	}); err != nil {
		t.Fatal(err)
	}

	res, err := st.Search(ctx, Query{Layer: LayerTask, TaskID: "t1", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 || res.Records[0].Kind != KindEvidence {
		t.Fatalf("task search = %+v", res)
	}
	res, err = st.Search(ctx, Query{Layer: LayerLongTerm, Metadata: map[string]any{"metric": "ROAS"}, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 {
		t.Fatalf("metadata search = %+v", res)
	}
}

type failingStore struct{}

func (failingStore) Save(context.Context, Record) error { return ErrClosed }
func (failingStore) Get(context.Context, string) (*Record, error) {
	return nil, ErrClosed
}
func (failingStore) Search(context.Context, Query) (SearchResult, error) {
	return SearchResult{}, ErrClosed
}
func (failingStore) Delete(context.Context, string) error { return ErrClosed }
func (failingStore) Close() error                         { return nil }

func TestRecorderWriteFailureDoesNotPanic(t *testing.T) {
	rec := NewRecorder(failingStore{}, logger.New("error", "json", io.Discard))
	ctx := context.Background()
	rec.SaveGoal(ctx, "run-x", "do not crash")
	rec.SavePlan(ctx, "run-x", []string{"a"})
	rec.SaveDecision(ctx, "run-x", map[string]any{"type": "FINISH"})
	rec.SaveToolCall(ctx, "run-x", "calculator", map[string]any{"op": "sum"}, "5")
	rec.SaveObservation(ctx, "run-x", "sum", "need numbers", "5", 0.9)
	rec.SaveTask(ctx, "run-x", "t1", "sum", "add", "5", "pending")
	rec.SaveEvidence(ctx, "run-x", "t1", "5")
	rec.SaveFailure(ctx, "run-x", "t1", "boom")
	rec.SaveSuccess(ctx, "run-x", "t1", "5")
	rec.SaveConclusion(ctx, "run-x", "FINISH", "done")
	if got := rec.Recall(ctx, "run-x", "anything", 5); got == nil {
		t.Fatal("Recall should return empty slice, not panic")
	}
}

func TestPostgresStoreOptional(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("MEMORY_POSTGRES_URL"))
	if dsn == "" {
		t.Skip("MEMORY_POSTGRES_URL not set")
	}
	st, err := Open(context.Background(), config.MemoryConfig{
		PostgresURL:  dsn,
		ShortTermTTL: config.Duration(time.Hour),
		SearchLimit:  20,
	}, logger.New("error", "json", io.Discard))
	if err != nil {
		t.Fatalf("Open postgres: %v", err)
	}
	defer st.Close()

	ctx := context.Background()
	id := "pg-" + time.Now().UTC().Format("150405.000")
	if err := st.Save(ctx, Record{
		ID:      id,
		RunID:   "pg-run",
		Layer:   LayerShortTerm,
		Kind:    KindGoal,
		Content: map[string]any{"goal": "postgres roundtrip"},
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := st.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.RunID != "pg-run" {
		t.Fatalf("run_id = %q", got.RunID)
	}
	if err := st.Delete(ctx, id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}
