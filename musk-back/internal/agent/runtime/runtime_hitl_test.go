package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"

	"autonomous-agent/internal/agent/decision"
	"autonomous-agent/internal/guardrail"
	"autonomous-agent/internal/requestid"
	"autonomous-agent/internal/tools"
)

type countingTool struct {
	name  string
	calls atomic.Int32
}

func (t *countingTool) Name() string { return t.name }
func (t *countingTool) Description() string {
	return "Mutate advertising campaign configuration (HIGH risk)."
}
func (t *countingTool) InputSchema() any {
	return map[string]any{
		"type":     "object",
		"required": []string{"campaign_id"},
		"properties": map[string]any{
			"campaign_id": map[string]any{"type": "string"},
		},
	}
}
func (t *countingTool) Execute(_ context.Context, input map[string]any) (tools.ToolResult, error) {
	t.calls.Add(1)
	return tools.ToolResult{
		Success: true,
		Data:    map[string]any{"updated": input["campaign_id"], "ok": true},
	}, nil
}

func callHighJSON() string {
	raw, _ := json.Marshal(decision.Decision{
		Type:           decision.DecisionCallTool,
		Reason:         "need to change campaign budget",
		Objective:      "update campaign",
		Tool:           "mutate_campaign",
		Arguments:      map[string]any{"campaign_id": "c-1"},
		ExpectedResult: "campaign updated",
		Confidence:     0.7,
		Risk:           "high",
	})
	return string(raw)
}

func TestRuntimeHighRiskInterruptApproveResumeExecuteFinish(t *testing.T) {
	tool := &countingTool{name: "mutate_campaign"}
	rt := newTestRuntime(t, callHighJSON(), finishJSON("campaign updated"))
	if err := rt.registry.Register(tool, tools.RiskHigh); err != nil {
		t.Fatalf("register: %v", err)
	}

	ctx := requestid.With(context.Background(), "run-hitl-high")
	_, err := rt.Run(ctx, "raise campaign c-1 budget")
	var waiting *guardrail.WaitingError
	if !errors.As(err, &waiting) {
		t.Fatalf("Run err = %v, want WaitingError", err)
	}
	if waiting.ApprovalID == "" || waiting.RunID != "run-hitl-high" {
		t.Fatalf("waiting = %+v", waiting)
	}
	if waiting.Risk != guardrail.RiskHigh {
		t.Fatalf("risk = %s", waiting.Risk)
	}
	if tool.calls.Load() != 0 {
		t.Fatal("HIGH action must not execute before approval")
	}

	if err := rt.Approve(ctx, waiting.ApprovalID); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	got, err := rt.Resume(ctx, waiting.RunID)
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if got != "campaign updated" {
		t.Fatalf("answer = %q", got)
	}
	if tool.calls.Load() != 1 {
		t.Fatalf("execute calls = %d, want 1 after resume", tool.calls.Load())
	}
}

func TestRuntimeHighRiskRejectDoesNotExecute(t *testing.T) {
	tool := &countingTool{name: "mutate_campaign"}
	rt := newTestRuntime(t, callHighJSON(), finishJSON("did not mutate"))
	if err := rt.registry.Register(tool, tools.RiskHigh); err != nil {
		t.Fatalf("register: %v", err)
	}

	ctx := requestid.With(context.Background(), "run-hitl-reject")
	_, err := rt.Run(ctx, "raise campaign c-1 budget")
	var waiting *guardrail.WaitingError
	if !errors.As(err, &waiting) {
		t.Fatalf("Run err = %v, want WaitingError", err)
	}

	if err := rt.Reject(ctx, waiting.ApprovalID, "too risky"); err != nil {
		t.Fatalf("Reject: %v", err)
	}

	got, err := rt.Resume(ctx, waiting.RunID)
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if got != "did not mutate" {
		t.Fatalf("answer = %q", got)
	}
	if tool.calls.Load() != 0 {
		t.Fatal("rejected HIGH action must not execute")
	}
}
