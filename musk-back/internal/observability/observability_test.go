package observability

import (
	"context"
	"testing"
)

func TestIDsRoundTrip(t *testing.T) {
	ctx := context.Background()
	run := NewID()
	step := NewID()
	tool := NewID()
	ctx = WithRunID(ctx, run)
	ctx = WithStepID(ctx, step)
	ctx = WithToolCallID(ctx, tool)
	if RunID(ctx) != run || StepID(ctx) != step || ToolCallID(ctx) != tool {
		t.Fatalf("ids = %s %s %s", RunID(ctx), StepID(ctx), ToolCallID(ctx))
	}
	attrs := Attrs(ctx)
	if len(attrs) != 6 {
		t.Fatalf("attrs len = %d", len(attrs))
	}
}

func TestTrackerTokensCostAndErrors(t *testing.T) {
	tr := NewTracker(CostModel{InputUSDPer1K: 1, OutputUSDPer1K: 2})
	tr.StartRun("r1")
	cost := tr.RecordTokens("r1", TokenUsage{PromptTokens: 1000, CompletionTokens: 500, TotalTokens: 1500})
	if cost != 1+1 {
		t.Fatalf("cost = %v, want 2", cost)
	}
	tr.RecordToolCall("r1")
	tr.RecordToolCall("r1")
	tr.RecordError("r1", "boom")
	snap := tr.Snapshot("r1")
	if snap.TokenUsage.TotalTokens != 1500 {
		t.Fatalf("tokens = %+v", snap.TokenUsage)
	}
	if snap.ToolCalls != 2 {
		t.Fatalf("tool calls = %d", snap.ToolCalls)
	}
	if snap.Errors != 1 || snap.LastError != "boom" {
		t.Fatalf("errors = %+v", snap)
	}
	if snap.CostUSD != 2 {
		t.Fatalf("cost usd = %v", snap.CostUSD)
	}
	if snap.Latency <= 0 {
		t.Fatal("expected latency")
	}
}

func TestSetupInProcess(t *testing.T) {
	tel, err := Setup(context.Background(), Config{ServiceName: "test-agent"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tel.Shutdown(context.Background())
	ctx, span := tel.StartSpan(WithRunID(context.Background(), "run-1"), "agent.run")
	span.End()
	tel.RecordLLM(ctx, TokenUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}, "stub")
	tel.RecordTool(ctx, "calculator", true, 0)
	if tel.Tracker.Snapshot("run-1").LLMCalls != 1 {
		t.Fatalf("llm calls = %+v", tel.Tracker.Snapshot("run-1"))
	}
}
