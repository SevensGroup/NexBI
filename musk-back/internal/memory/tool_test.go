package memory

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"autonomous-agent/internal/config"
	"autonomous-agent/internal/logger"
	"autonomous-agent/internal/requestid"
	"autonomous-agent/internal/tools"
)

func testMemoryRegistry(t *testing.T) (tools.Registry, MemoryStore) {
	t.Helper()
	st := testStore(t, time.Hour)
	reg := tools.NewRegistry(logger.New("error", "json", io.Discard), time.Second)
	if err := RegisterTools(reg, st); err != nil {
		t.Fatalf("RegisterTools: %v", err)
	}
	return reg, st
}

func TestMemoryToolsRegisterAndSaveSearch(t *testing.T) {
	reg, _ := testMemoryRegistry(t)
	ctx := memoryCtx("run-tool-1")

	res, err := reg.Execute(ctx, ToolSave, map[string]any{
		"layer": "long_term",
		"kind":  "UserPreference",
		"content": map[string]any{
			"prefer":  "daily Meta ROAS digest",
			"api_key": "sk-should-redact-this-value",
		},
		"metadata": map[string]any{"channel": "Meta"},
	})
	if err != nil {
		t.Fatalf("save_memory: %v", err)
	}
	if !res.Success {
		t.Fatalf("save failed: %+v", res)
	}

	found, err := reg.Execute(ctx, ToolSearch, map[string]any{
		"query":  "daily Meta ROAS digest",
		"layer":  "long_term",
		"kind":   "UserPreference",
		"run_id": "run-tool-1",
		"limit":  10,
		"offset": 0,
	})
	if err != nil {
		t.Fatalf("search_memory: %v", err)
	}
	data, _ := found.Data.(map[string]any)
	if data == nil {
		t.Fatalf("search data = %#v", found.Data)
	}
	total, _ := asInt(data["total"])
	if total < 1 {
		t.Fatalf("search total = %#v", data["total"])
	}
	raw, _ := json.Marshal(data)
	if strings.Contains(string(raw), "sk-should-redact-this-value") {
		t.Fatalf("api key leaked in search: %s", raw)
	}
}

func TestSaveMemoryRejectsWrongKind(t *testing.T) {
	reg, _ := testMemoryRegistry(t)
	ctx := memoryCtx("run-tool-2")
	res, err := reg.Execute(ctx, ToolSave, map[string]any{
		"layer":   "short_term",
		"kind":    "UserPreference",
		"content": "nope",
	})
	if err == nil {
		t.Fatal("expected invalid kind")
	}
	if res.Success {
		t.Fatal("expected unsuccessful result")
	}
}

func TestSaveMemoryUsesRunIDFromContext(t *testing.T) {
	reg, st := testMemoryRegistry(t)
	ctx := memoryCtx("ctx-run")
	if _, err := reg.Execute(ctx, ToolSave, map[string]any{
		"layer":   "task",
		"kind":    "Conclusion",
		"content": map[string]any{"answer": "done"},
	}); err != nil {
		t.Fatal(err)
	}
	res, err := st.Search(ctx, Query{RunID: "ctx-run", Layer: LayerTask, Kind: KindConclusion, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 {
		t.Fatalf("expected context run_id association, got %+v", res)
	}
}

func TestOpenUsesConfigWithoutPostgres(t *testing.T) {
	st, err := Open(context.Background(), config.MemoryConfig{SearchLimit: 5}, logger.New("error", "json", io.Discard))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.Save(context.Background(), Record{
		RunID:   "r",
		Layer:   LayerShortTerm,
		Kind:    KindPlan,
		Content: map[string]any{"plan": []string{"a"}},
	}); err != nil {
		t.Fatal(err)
	}
}

func memoryCtx(runID string) context.Context {
	ctx := requestid.With(context.Background(), runID)
	return WithRunID(ctx, runID)
}
