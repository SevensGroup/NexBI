package planner

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"autonomous-agent/internal/agent/decision"
	"autonomous-agent/internal/logger"
)

type stubChatModel struct {
	reply string
	err   error
}

func (s *stubChatModel) Generate(_ context.Context, _ []*schema.Message, _ ...einomodel.Option) (*schema.Message, error) {
	if s.err != nil {
		return nil, s.err
	}
	return schema.AssistantMessage(s.reply, nil), nil
}

func (s *stubChatModel) Stream(context.Context, []*schema.Message, ...einomodel.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("stream not implemented")
}

func testPlanner(t *testing.T, model einomodel.BaseChatModel, maxTasks int) *Planner {
	t.Helper()
	p, err := New(model, "You are a test planner.", maxTasks, logger.New("error", "json", io.Discard))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return p
}

func mustPlanJSON(t *testing.T, plan llmPlan) string {
	t.Helper()
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(raw)
}

func TestSingleTask(t *testing.T) {
	reply := mustPlanJSON(t, llmPlan{
		Goal: "say hello",
		Tasks: []llmTask{{
			ID:             "t1",
			Title:          "greet",
			Description:    "reply to the user",
			ExpectedOutput: "a greeting",
			Priority:       1,
		}},
	})
	p := testPlanner(t, &stubChatModel{reply: reply}, 8)

	plan, err := p.Plan(context.Background(), "say hello", *decision.NewState("say hello", 8))
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(plan.Tasks) != 1 {
		t.Fatalf("tasks = %d", len(plan.Tasks))
	}
	if plan.Tasks[0].ExpectedOutput != "a greeting" {
		t.Fatalf("expected_output = %q", plan.Tasks[0].ExpectedOutput)
	}

	sched := NewTaskScheduler(plan)
	if len(sched.Ready()) != 1 {
		t.Fatalf("ready = %d", len(sched.Ready()))
	}
	if err := ExecuteMock(context.Background(), sched, NewMockTool()); err != nil {
		t.Fatalf("ExecuteMock: %v", err)
	}
	if !sched.AllComplete() {
		t.Fatal("single task should complete")
	}
	if len(sched.Completed()) != 1 || len(sched.Failed()) != 0 {
		t.Fatalf("completed=%d failed=%d", len(sched.Completed()), len(sched.Failed()))
	}
}

func TestMultiTask(t *testing.T) {
	reply := mustPlanJSON(t, llmPlan{
		Goal: "research then write",
		Tasks: []llmTask{
			{ID: "t1", Title: "research", ExpectedOutput: "notes", Priority: 2},
			{ID: "t2", Title: "outline", Dependencies: []string{"t1"}, ExpectedOutput: "outline"},
			{ID: "t3", Title: "draft", Dependencies: []string{"t2"}, ExpectedOutput: "draft"},
		},
	})
	p := testPlanner(t, &stubChatModel{reply: reply}, 8)

	plan, err := p.Plan(context.Background(), "research then write", decision.AgentState{Goal: "research then write"})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(plan.Tasks) != 3 {
		t.Fatalf("tasks = %d", len(plan.Tasks))
	}

	sched := NewTaskScheduler(plan)
	if err := ExecuteMock(context.Background(), sched, NewMockTool()); err != nil {
		t.Fatalf("ExecuteMock: %v", err)
	}
	if !sched.AllComplete() {
		t.Fatal("multi task graph should complete")
	}
}

func TestDependency(t *testing.T) {
	plan := &Plan{
		Goal: "ordered work",
		Tasks: []Task{
			{ID: "t1", Title: "first", ExpectedOutput: "step 1", Status: StatusPending},
			{ID: "t2", Title: "second", Dependencies: []string{"t1"}, ExpectedOutput: "step 2", Status: StatusPending},
		},
	}
	if err := ValidatePlan(plan, 8); err != nil {
		t.Fatalf("ValidatePlan: %v", err)
	}

	sched := NewTaskScheduler(plan)
	ready := sched.Ready()
	if len(ready) != 1 || ready[0].ID != "t1" {
		t.Fatalf("ready before t1 = %+v", ready)
	}

	if err := sched.Start("t1"); err != nil {
		t.Fatalf("Start t1: %v", err)
	}
	if got := sched.Ready(); len(got) != 0 {
		t.Fatalf("t2 must wait until t1 completes, ready=%+v", got)
	}
	if err := sched.Complete("t1"); err != nil {
		t.Fatalf("Complete t1: %v", err)
	}

	ready = sched.Ready()
	if len(ready) != 1 || ready[0].ID != "t2" {
		t.Fatalf("ready after t1 = %+v", ready)
	}
}

func TestParallelTasks(t *testing.T) {
	plan := &Plan{
		Goal: "independent work",
		Tasks: []Task{
			{ID: "a", Title: "alpha", ExpectedOutput: "A", Priority: 1, Status: StatusPending},
			{ID: "b", Title: "beta", ExpectedOutput: "B", Priority: 1, Status: StatusPending},
			{ID: "c", Title: "combine", Dependencies: []string{"a", "b"}, ExpectedOutput: "C", Status: StatusPending},
		},
	}
	if err := ValidatePlan(plan, 8); err != nil {
		t.Fatalf("ValidatePlan: %v", err)
	}

	if got := ReadyTasks(plan.Tasks, map[string]bool{}); len(got) != 2 {
		t.Fatalf("ReadyTasks independent = %d, want 2", len(got))
	}

	sched := NewTaskScheduler(plan)
	ready := sched.Ready()
	if len(ready) != 2 {
		t.Fatalf("independent tasks should be ready together, got %d", len(ready))
	}
	ids := map[string]bool{}
	for _, tsk := range ready {
		ids[tsk.ID] = true
	}
	if !ids["a"] || !ids["b"] {
		t.Fatalf("ready ids = %v", ids)
	}

	if err := ExecuteMock(context.Background(), sched, NewMockTool()); err != nil {
		t.Fatalf("ExecuteMock: %v", err)
	}
	if !sched.AllComplete() {
		t.Fatal("parallel graph should complete")
	}
	if len(sched.Completed()) != 3 {
		t.Fatalf("completed = %d", len(sched.Completed()))
	}
}

func TestCircularDependency(t *testing.T) {
	plan := &Plan{
		Goal: "cycle",
		Tasks: []Task{
			{ID: "a", Title: "A", Dependencies: []string{"b"}, ExpectedOutput: "A"},
			{ID: "b", Title: "B", Dependencies: []string{"a"}, ExpectedOutput: "B"},
		},
	}
	err := ValidatePlan(plan, 8)
	if !errors.Is(err, ErrCyclicDependency) {
		t.Fatalf("err = %v, want ErrCyclicDependency", err)
	}

	self := &Plan{
		Goal: "self",
		Tasks: []Task{
			{ID: "a", Title: "A", Dependencies: []string{"a"}, ExpectedOutput: "A"},
		},
	}
	if err := ValidateDAG(self.Tasks); !errors.Is(err, ErrCyclicDependency) {
		t.Fatalf("self-cycle err = %v", err)
	}
}

func TestIllegalDAGUnknownDependency(t *testing.T) {
	plan := &Plan{
		Goal: "dangling",
		Tasks: []Task{
			{ID: "a", Title: "A", Dependencies: []string{"missing"}, ExpectedOutput: "A"},
			{ID: "b", Title: "B", Dependencies: []string{"a", "task_1"}, ExpectedOutput: "B"},
		},
	}
	if err := ValidatePlan(plan, 8); err != nil {
		t.Fatalf("unknown deps should be dropped, got %v", err)
	}
	if len(plan.Tasks[0].Dependencies) != 0 {
		t.Fatalf("dangling dep on a = %v", plan.Tasks[0].Dependencies)
	}
	if len(plan.Tasks[1].Dependencies) != 1 || plan.Tasks[1].Dependencies[0] != "a" {
		t.Fatalf("kept deps = %v", plan.Tasks[1].Dependencies)
	}
}

func TestMaxTasks(t *testing.T) {
	reply := mustPlanJSON(t, llmPlan{
		Goal: "too many",
		Tasks: []llmTask{
			{ID: "t1", Title: "one", ExpectedOutput: "1"},
			{ID: "t2", Title: "two", ExpectedOutput: "2"},
			{ID: "t3", Title: "three", ExpectedOutput: "3"},
		},
	})
	p := testPlanner(t, &stubChatModel{reply: reply}, 2)
	_, err := p.Plan(context.Background(), "too many", decision.AgentState{Goal: "too many"})
	if !errors.Is(err, ErrTooManyTasks) {
		t.Fatalf("err = %v, want ErrTooManyTasks", err)
	}
}

func TestTaskRequiresExpectedOutput(t *testing.T) {
	plan := &Plan{
		Goal: "g",
		Tasks: []Task{
			{ID: "t1", Title: "do it", Description: "do it"},
		},
	}
	err := ValidatePlan(plan, 8)
	if !errors.Is(err, ErrInvalidPlan) || !strings.Contains(err.Error(), "expected_output") {
		t.Fatalf("err = %v", err)
	}
}

func TestPlannerRejectsEmptyGoal(t *testing.T) {
	p := testPlanner(t, &stubChatModel{reply: "{}"}, 8)
	_, err := p.Plan(context.Background(), "  ", decision.AgentState{})
	if !errors.Is(err, ErrEmptyGoal) {
		t.Fatalf("err = %v, want ErrEmptyGoal", err)
	}
}

func TestGoalPlanGraphMockComplete(t *testing.T) {
	reply := mustPlanJSON(t, llmPlan{
		Goal: "build a report",
		Tasks: []llmTask{
			{ID: "gather", Title: "gather facts", ExpectedOutput: "fact list", Priority: 2},
			{ID: "analyze", Title: "analyze", Dependencies: []string{"gather"}, ExpectedOutput: "insights"},
			{ID: "extra", Title: "side check", ExpectedOutput: "check result", Priority: 2},
		},
	})
	p := testPlanner(t, &stubChatModel{reply: reply}, 8)

	plan, err := p.Plan(context.Background(), "build a report", *decision.NewState("build a report", 8))
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if err := ValidateDAG(plan.Tasks); err != nil {
		t.Fatalf("ValidateDAG: %v", err)
	}

	sched := NewTaskScheduler(plan)
	if got := len(sched.Ready()); got != 2 {
		t.Fatalf("parallel ready = %d, want 2", got)
	}
	if err := ExecuteMock(context.Background(), sched, NewMockTool()); err != nil {
		t.Fatalf("ExecuteMock: %v", err)
	}
	if !sched.AllComplete() || len(sched.Running()) != 0 || len(sched.Failed()) != 0 {
		t.Fatalf("completed=%d running=%d failed=%d", len(sched.Completed()), len(sched.Running()), len(sched.Failed()))
	}
}
