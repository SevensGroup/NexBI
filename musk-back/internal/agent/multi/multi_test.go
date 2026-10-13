package multi

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"autonomous-agent/internal/config"
	"autonomous-agent/internal/memory"
	"autonomous-agent/internal/tools"
)

type stubRunner struct {
	answer string
	err    error
	got    string
	spec   Spec
}

func (s *stubRunner) Run(_ context.Context, message string) (string, error) {
	s.got = message
	if s.err != nil {
		return "", s.err
	}
	return s.answer, nil
}

func TestDefaultSpecsIndependent(t *testing.T) {
	specs := DefaultSpecs(4)
	if len(specs) != 3 {
		t.Fatalf("specs = %d, want 3", len(specs))
	}
	seen := map[string]Spec{}
	for _, spec := range specs {
		seen[spec.Name] = spec
		if strings.TrimSpace(spec.Instruction) == "" {
			t.Fatalf("%s missing instruction", spec.Name)
		}
		if spec.InputSchema == nil || spec.OutputSchema == nil {
			t.Fatalf("%s missing I/O schema", spec.Name)
		}
		if spec.MaxSteps != 4 {
			t.Fatalf("%s max steps = %d", spec.Name, spec.MaxSteps)
		}
	}
	if seen[string(RoleAnalyst)].Instruction == seen[string(RoleReviewer)].Instruction {
		t.Fatal("analyst and reviewer must have independent system prompts")
	}
	if strings.Join(seen[string(RoleAnalyst)].Tools, ",") == strings.Join(seen[string(RoleReviewer)].Tools, ",") {
		t.Fatal("analyst and reviewer must have independent tool scopes")
	}
	reviewerTools := strings.Join(seen[string(RoleReviewer)].Tools, ",")
	if strings.Contains(reviewerTools, "calculator") || strings.Contains(reviewerTools, "semantic_query") {
		t.Fatalf("reviewer tool scope too broad: %s", reviewerTools)
	}
	if !strings.Contains(seen[string(RoleAnalyst)].Instruction, "data analysis") {
		t.Fatal("analyst prompt missing data analysis")
	}
	if !strings.Contains(seen[string(RoleResearcher)].Instruction, "external search") {
		t.Fatal("researcher prompt missing external search")
	}
	if !strings.Contains(seen[string(RoleReviewer)].Instruction, "verify conclusions") {
		t.Fatal("reviewer prompt missing verify conclusions")
	}
}

func TestTeamDelegateAnalyst(t *testing.T) {
	runner := &stubRunner{answer: `{"agent":"analyst","task":"Analyze why Meta ROAS declined yesterday","conclusion":"ROAS dropped on Meta","evidence":"spend up, revenue down","confidence":0.8}`}
	team := NewTeam(DefaultSpecs(4), func(spec Spec) (Runner, error) {
		if spec.Name != string(RoleAnalyst) {
			t.Fatalf("spawned %s", spec.Name)
		}
		runner.spec = spec
		return runner, nil
	})

	out, err := team.Delegate(context.Background(), DelegateRequest{
		Agent: "analyst",
		Task:  "Analyze why Meta ROAS declined yesterday",
	})
	if err != nil {
		t.Fatalf("Delegate: %v", err)
	}
	if out.Rejected() {
		t.Fatal("analyst should not look like a reviewer rejection")
	}
	if out.Conclusion != "ROAS dropped on Meta" {
		t.Fatalf("conclusion = %q", out.Conclusion)
	}
	if !strings.Contains(runner.got, "Analyze why Meta ROAS declined yesterday") {
		t.Fatalf("task not passed to specialist: %q", runner.got)
	}
	if !strings.Contains(runner.spec.Instruction, "Analyst") {
		t.Fatalf("independent prompt not used: %q", runner.spec.Instruction)
	}
}

func TestReviewerReject(t *testing.T) {
	team := NewTeam(DefaultSpecs(4), func(spec Spec) (Runner, error) {
		if spec.Name != string(RoleReviewer) {
			t.Fatalf("spawned %s", spec.Name)
		}
		return &stubRunner{answer: `{"approved": false, "reason": "insufficient evidence"}`}, nil
	})

	out, err := team.Delegate(context.Background(), DelegateRequest{
		Agent: "reviewer",
		Task:  "verify Meta ROAS drop",
		Extra: map[string]any{"conclusion": "it dropped", "evidence": ""},
	})
	if err != nil {
		t.Fatalf("Delegate: %v", err)
	}
	if !out.Rejected() {
		t.Fatalf("want rejected, got %#v", out)
	}
	if out.Review.Reason != "insufficient evidence" {
		t.Fatalf("reason = %q", out.Review.Reason)
	}
}

func TestReviewerApprove(t *testing.T) {
	team := NewTeam(DefaultSpecs(4), func(Spec) (Runner, error) {
		return &stubRunner{answer: `{"approved": true, "reason": "conclusion matches evidence"}`}, nil
	})
	out, err := team.Delegate(context.Background(), DelegateRequest{Agent: "reviewer", Task: "verify"})
	if err != nil {
		t.Fatalf("Delegate: %v", err)
	}
	if out.Rejected() || out.Review == nil || !out.Review.Approved {
		t.Fatalf("want approved, got %#v", out)
	}
}

func TestReviewerInvalidOutputRejects(t *testing.T) {
	team := NewTeam(DefaultSpecs(4), func(Spec) (Runner, error) {
		return &stubRunner{answer: "looks fine to me"}, nil
	})
	out, err := team.Delegate(context.Background(), DelegateRequest{Agent: "reviewer", Task: "verify"})
	if err != nil {
		t.Fatalf("Delegate: %v", err)
	}
	if !out.Rejected() {
		t.Fatal("unstructured reviewer output must fail closed")
	}
}

func TestUnknownAgent(t *testing.T) {
	team := NewTeam(DefaultSpecs(4), func(Spec) (Runner, error) {
		t.Fatal("should not spawn")
		return nil, nil
	})
	_, err := team.Delegate(context.Background(), DelegateRequest{Agent: "writer", Task: "draft"})
	if !errors.Is(err, ErrUnknownAgent) {
		t.Fatalf("err = %v, want ErrUnknownAgent", err)
	}
}

func TestDelegateRequiresTask(t *testing.T) {
	team := NewTeam(DefaultSpecs(4), func(Spec) (Runner, error) {
		return &stubRunner{answer: "x"}, nil
	})
	_, err := team.Delegate(context.Background(), DelegateRequest{Agent: "analyst"})
	if !errors.Is(err, ErrInvalidDelegate) {
		t.Fatalf("err = %v", err)
	}
}

func TestFilterRegistryToolScope(t *testing.T) {
	inner := tools.NewRegistry(nil, 0)
	if err := tools.RegisterBuiltins(inner); err != nil {
		t.Fatal(err)
	}
	reviewer := FilterRegistry(inner, []string{memory.ToolSearch})
	if _, ok := reviewer.Get("calculator"); ok {
		t.Fatal("reviewer must not see calculator")
	}
	if _, err := reviewer.Execute(context.Background(), "calculator", map[string]any{"op": "sum", "a": 1, "b": 2}); !errors.Is(err, tools.ErrNotFound) {
		t.Fatalf("execute calculator err = %v, want not in scope", err)
	}
	names := map[string]bool{}
	for _, info := range reviewer.List() {
		names[info.Name] = true
	}
	if names["calculator"] {
		t.Fatal("calculator listed for reviewer")
	}

	analyst := FilterRegistry(inner, []string{"calculator", memory.ToolSearch})
	res, err := analyst.Execute(context.Background(), "calculator", map[string]any{"op": tools.OpSum, "a": 2.0, "b": 3.0})
	if err != nil {
		t.Fatalf("analyst calculator: %v", err)
	}
	if !res.Success {
		t.Fatalf("analyst calculator failed: %+v", res)
	}
}

func TestIndependentMemoryScope(t *testing.T) {
	ctx := context.Background()
	root, err := memory.Open(ctx, memoryConfig(), nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = root.Close() })

	analyst := memory.Scope(root, string(RoleAnalyst))
	researcher := memory.Scope(root, string(RoleResearcher))

	if err := analyst.Save(ctx, memory.Record{
		RunID:   "run-1",
		Layer:   memory.LayerShortTerm,
		Kind:    memory.KindObservation,
		Content: map[string]any{"note": "analyst-only"},
	}); err != nil {
		t.Fatalf("analyst save: %v", err)
	}
	if err := researcher.Save(ctx, memory.Record{
		RunID:   "run-1",
		Layer:   memory.LayerShortTerm,
		Kind:    memory.KindObservation,
		Content: map[string]any{"note": "researcher-only"},
	}); err != nil {
		t.Fatalf("researcher save: %v", err)
	}

	aRes, err := analyst.Search(ctx, memory.Query{RunID: "run-1", Layer: memory.LayerShortTerm, Limit: 10})
	if err != nil {
		t.Fatalf("analyst search: %v", err)
	}
	rRes, err := researcher.Search(ctx, memory.Query{RunID: "run-1", Layer: memory.LayerShortTerm, Limit: 10})
	if err != nil {
		t.Fatalf("researcher search: %v", err)
	}
	if aRes.Total != 1 || rRes.Total != 1 {
		t.Fatalf("totals analyst=%d researcher=%d", aRes.Total, rRes.Total)
	}
	if fmt.Sprint(aRes.Records[0].Content) == fmt.Sprint(rRes.Records[0].Content) {
		t.Fatal("agents must not share the same short-term record")
	}
	if aRes.Records[0].RunID == rRes.Records[0].RunID {
		t.Fatalf("run ids should be scoped, got %q and %q", aRes.Records[0].RunID, rRes.Records[0].RunID)
	}

	if _, err := researcher.Get(ctx, aRes.Records[0].ID); !errors.Is(err, memory.ErrNotFound) {
		t.Fatalf("researcher Get analyst record: %v, want not found", err)
	}
}

func memoryConfig() config.MemoryConfig {
	return config.MemoryConfig{SearchLimit: 20, ShortTermTTL: config.Duration(time.Hour)}
}

func TestAgentToolMissingRunner(t *testing.T) {
	tool := NewAgentTool(analystSpec(4), nil)
	_, err := tool.Execute(context.Background(), map[string]any{"task": "x"})
	if !errors.Is(err, ErrNoRunner) {
		t.Fatalf("err = %v, want ErrNoRunner", err)
	}
}

func TestParseReview(t *testing.T) {
	rev, ok := ParseReview("```json\n{\"approved\": false, \"reason\": \"insufficient evidence\"}\n```")
	if !ok || rev.Approved || rev.Reason != "insufficient evidence" {
		t.Fatalf("parse = %#v ok=%v", rev, ok)
	}
}

func TestListingsHaveSchemas(t *testing.T) {
	team := NewTeam(DefaultSpecs(3), func(Spec) (Runner, error) { return &stubRunner{answer: "x"}, nil })
	list := team.Listings()
	if len(list) != 3 {
		t.Fatalf("listings = %d", len(list))
	}
	if list[0].Name != string(RoleAnalyst) || list[2].Name != string(RoleReviewer) {
		t.Fatalf("order = %#v", list)
	}
	for _, item := range list {
		if item.InputSchema == nil || item.OutputSchema == nil {
			t.Fatalf("%s missing schema", item.Name)
		}
	}
}
