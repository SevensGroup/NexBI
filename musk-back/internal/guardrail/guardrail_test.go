package guardrail

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestClassifyRules(t *testing.T) {
	cases := []struct {
		name   string
		action Action
		want   RiskLevel
	}{
		{name: "calculator low", action: Action{Tool: "calculator", RegisteredRisk: RiskLow}, want: RiskLow},
		{name: "search low", action: Action{Tool: "search_memory", Arguments: map[string]any{"q": "roas"}}, want: RiskLow},
		{name: "large query medium", action: Action{Tool: "elasticsearch_search", Arguments: map[string]any{"size": 5000}, RegisteredRisk: RiskLow}, want: RiskMedium},
		{name: "registered medium wins over infer low", action: Action{Tool: "semantic_query", RegisteredRisk: RiskMedium}, want: RiskMedium},
		{name: "mutate high", action: Action{Tool: "mutate_campaign", Arguments: map[string]any{"campaign_id": "c1"}}, want: RiskHigh},
		{name: "update ads high", action: Action{Tool: "update_ad_config", RegisteredRisk: RiskLow}, want: RiskHigh},
		{name: "external api high", action: Action{Tool: "call_api", Arguments: map[string]any{"url": "https://ads.example/v1"}}, want: RiskHigh},
		{name: "delete critical", action: Action{Tool: "delete_campaign"}, want: RiskCritical},
		{name: "funds critical", action: Action{Tool: "transfer_funds", Arguments: map[string]any{"amount": 100}}, want: RiskCritical},
		{name: "registered critical wins", action: Action{Tool: "calculator", RegisteredRisk: RiskCritical}, want: RiskCritical},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(tc.action)
			if got != tc.want {
				t.Fatalf("Classify = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestEngineLOWAutoMEDIUMConfigHIGHApproval(t *testing.T) {
	auto := NewEngine(Policy{MediumAuto: true})
	gated := NewEngine(Policy{MediumAuto: false})

	low, err := auto.Evaluate(Action{Tool: "calculator", RegisteredRisk: RiskLow})
	if err != nil {
		t.Fatal(err)
	}
	if low.Level != RiskLow || low.Verdict != VerdictAllow {
		t.Fatalf("LOW = %+v", low)
	}

	medAuto, err := auto.Evaluate(Action{Tool: "semantic_query", RegisteredRisk: RiskMedium})
	if err != nil {
		t.Fatal(err)
	}
	if medAuto.Verdict != VerdictAllow {
		t.Fatalf("MEDIUM auto = %+v", medAuto)
	}

	medGate, err := gated.Evaluate(Action{Tool: "semantic_query", RegisteredRisk: RiskMedium})
	if err != nil {
		t.Fatal(err)
	}
	if medGate.Verdict != VerdictApprove {
		t.Fatalf("MEDIUM gated = %+v", medGate)
	}

	high, err := auto.Evaluate(Action{Tool: "mutate_campaign", RegisteredRisk: RiskHigh})
	if err != nil {
		t.Fatal(err)
	}
	if high.Level != RiskHigh || !high.RequiresApproval() {
		t.Fatalf("HIGH = %+v", high)
	}

	crit, err := auto.Evaluate(Action{Tool: "delete_index", RegisteredRisk: RiskCritical})
	if err != nil {
		t.Fatal(err)
	}
	if crit.Level != RiskCritical || !crit.RequiresApproval() {
		t.Fatalf("CRITICAL = %+v", crit)
	}
}

func TestApprovalManagerCreateApproveRejectExpire(t *testing.T) {
	mgr := NewApprovalManager(time.Hour)
	ctx := context.Background()

	created, err := mgr.CreateApproval(ctx, ApprovalRequest{
		RunID:     "run-1",
		Tool:      "mutate_campaign",
		Arguments: map[string]any{"id": "c1"},
		Risk:      RiskHigh,
		Reason:    "modify advertising config",
	})
	if err != nil {
		t.Fatalf("CreateApproval: %v", err)
	}
	if created.ID == "" || created.Status != StatusPending {
		t.Fatalf("created = %+v", created)
	}

	got, err := mgr.Get(ctx, created.ID)
	if err != nil || got.Status != StatusPending {
		t.Fatalf("Get = %+v err=%v", got, err)
	}

	approved, err := mgr.Approve(ctx, created.ID)
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if approved.Status != StatusApproved || approved.DecidedAt == nil {
		t.Fatalf("approved = %+v", approved)
	}
	if _, err := mgr.Approve(ctx, created.ID); !errors.Is(err, ErrNotPending) {
		t.Fatalf("second Approve err = %v", err)
	}

	other, err := mgr.CreateApproval(ctx, ApprovalRequest{Tool: "mutate_campaign", Risk: RiskHigh})
	if err != nil {
		t.Fatal(err)
	}
	rejected, err := mgr.Reject(ctx, other.ID, "no")
	if err != nil {
		t.Fatal(err)
	}
	if rejected.Status != StatusRejected || rejected.Note != "no" {
		t.Fatalf("rejected = %+v", rejected)
	}

	third, err := mgr.CreateApproval(ctx, ApprovalRequest{Tool: "delete_campaign", Risk: RiskCritical})
	if err != nil {
		t.Fatal(err)
	}
	expired, err := mgr.Expire(ctx, third.ID)
	if err != nil {
		t.Fatal(err)
	}
	if expired.Status != StatusExpired {
		t.Fatalf("expired = %+v", expired)
	}
}

func TestApprovalTTLExpiresOnGet(t *testing.T) {
	mgr := NewApprovalManager(time.Millisecond)
	mem, ok := mgr.(*memoryApprovals)
	if !ok {
		t.Fatal("expected memoryApprovals")
	}
	start := time.Now()
	mem.now = func() time.Time { return start }

	created, err := mgr.CreateApproval(context.Background(), ApprovalRequest{Tool: "mutate_campaign", Risk: RiskHigh})
	if err != nil {
		t.Fatal(err)
	}

	mem.now = func() time.Time { return start.Add(time.Second) }
	got, err := mgr.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusExpired {
		t.Fatalf("status = %s, want EXPIRED", got.Status)
	}
	if _, err := mgr.Approve(context.Background(), created.ID); !errors.Is(err, ErrExpired) {
		t.Fatalf("Approve after TTL err = %v", err)
	}
}

func TestCheckpointSaveLoad(t *testing.T) {
	store := NewCheckpointer()
	if err := store.Save(Checkpoint{RunID: "run-1", ApprovalID: "appr-1"}); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load("run-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ApprovalID != "appr-1" {
		t.Fatalf("checkpoint = %+v", got)
	}
	if _, err := store.Load("missing"); !errors.Is(err, ErrNoCheckpoint) {
		t.Fatalf("missing err = %v", err)
	}
	_ = store.Delete("run-1")
	if _, err := store.Load("run-1"); !errors.Is(err, ErrNoCheckpoint) {
		t.Fatalf("deleted err = %v", err)
	}
}

func TestWaitingErrorUnwrap(t *testing.T) {
	err := &WaitingError{RunID: "r", ApprovalID: "a", Tool: "mutate_campaign", Risk: RiskHigh}
	if !errors.Is(err, ErrWaitingApproval) {
		t.Fatal("expected ErrWaitingApproval")
	}
	if !strings.Contains(err.Error(), "mutate_campaign") {
		t.Fatalf("error = %q", err.Error())
	}
}
