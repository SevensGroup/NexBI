package reflection

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
	reply      string
	err        error
	called     int
	generateFn func(ctx context.Context, input []*schema.Message, opts ...einomodel.Option) (*schema.Message, error)
}

func (s *stubChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...einomodel.Option) (*schema.Message, error) {
	s.called++
	if s.generateFn != nil {
		return s.generateFn(ctx, input, opts...)
	}
	if s.err != nil {
		return nil, s.err
	}
	return schema.AssistantMessage(s.reply, nil), nil
}

func (s *stubChatModel) Stream(context.Context, []*schema.Message, ...einomodel.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("stream not implemented")
}

func testEngine(t *testing.T, model einomodel.BaseChatModel) *Engine {
	t.Helper()
	eng, err := NewEngine(model, "You are a test agent.", logger.New("error", "json", io.Discard))
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	return eng
}

func mustJSON(t *testing.T, r Reflection) string {
	t.Helper()
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(raw)
}

func successObs() decision.Observation {
	return decision.Observation{
		Summary:    "calculator result",
		Reason:     "need sum",
		Evidence:   `{"value":5}`,
		Confidence: 0.8,
	}
}

func TestReflectContinue(t *testing.T) {
	reply := mustJSON(t, Reflection{
		GoalSatisfied:      false,
		EvidenceSufficient: false,
		ResultCorrect:      true,
		MissingInformation: []string{"remaining planned tasks"},
		NextAction:         NextContinue,
		Confidence:         0.6,
	})
	eng := testEngine(t, &stubChatModel{reply: reply})
	state := decision.NewState("what is 2 + 3?", 8)
	state.Plan = []string{"sum", "report"}

	r, err := eng.Reflect(context.Background(), state, successObs())
	if err != nil {
		t.Fatalf("Reflect: %v", err)
	}
	if r.NextAction != NextContinue {
		t.Fatalf("next_action = %s", r.NextAction)
	}
	if r.GoalSatisfied {
		t.Fatal("goal should not be satisfied while plan remains")
	}
}

func TestReflectFinish(t *testing.T) {
	reply := mustJSON(t, Reflection{
		GoalSatisfied:      true,
		EvidenceSufficient: true,
		ResultCorrect:      true,
		NextAction:         NextFinish,
		Confidence:         0.95,
	})
	eng := testEngine(t, &stubChatModel{reply: reply})
	state := decision.NewState("what is 2 + 3?", 8)
	state.Plan = []string{"sum"}
	state.CompletedTasks = []string{"sum"}

	r, err := eng.Reflect(context.Background(), state, successObs())
	if err != nil {
		t.Fatalf("Reflect: %v", err)
	}
	if r.NextAction != NextFinish {
		t.Fatalf("next_action = %s", r.NextAction)
	}
	if !r.GoalSatisfied || !r.EvidenceSufficient || !r.ResultCorrect {
		t.Fatalf("reflection = %+v", r)
	}
}

func TestReflectAskUser(t *testing.T) {
	reply := mustJSON(t, Reflection{
		GoalSatisfied:      false,
		EvidenceSufficient: false,
		ResultCorrect:      false,
		MissingInformation: []string{"which date range?"},
		NextAction:         NextAskUser,
		Confidence:         0.7,
	})
	eng := testEngine(t, &stubChatModel{reply: reply})

	r, err := eng.Reflect(context.Background(), decision.NewState("analyze ROAS", 8), decision.Observation{
		Summary: "semantic_query result",
		Reason:  "need a time window",
	})
	if err != nil {
		t.Fatalf("Reflect: %v", err)
	}
	if r.NextAction != NextAskUser {
		t.Fatalf("next_action = %s", r.NextAction)
	}
	if len(r.MissingInformation) == 0 {
		t.Fatal("expected missing information")
	}
}

func TestReflectReplan(t *testing.T) {
	reply := mustJSON(t, Reflection{
		GoalSatisfied:      false,
		EvidenceSufficient: false,
		ResultCorrect:      false,
		Errors:             []string{"plan cannot produce the expected output"},
		NextAction:         NextReplan,
		Confidence:         0.8,
	})
	eng := testEngine(t, &stubChatModel{reply: reply})

	r, err := eng.Reflect(context.Background(), decision.NewState("analyze ROAS", 8), decision.Observation{
		Summary:  "semantic_query error",
		Evidence: "unsupported metric",
	})
	if err != nil {
		t.Fatalf("Reflect: %v", err)
	}
	if r.NextAction != NextReplan {
		t.Fatalf("next_action = %s", r.NextAction)
	}
}

func TestInvalidReflectionFallsBackToHeuristic(t *testing.T) {
	eng := testEngine(t, &stubChatModel{reply: `{"next_action":"DANCE","confidence":0.2}`})
	r, err := eng.Reflect(context.Background(), decision.NewState("goal", 8), successObs())
	if err != nil {
		t.Fatalf("Reflect: %v", err)
	}
	if r.NextAction != NextContinue {
		t.Fatalf("next_action = %s, want heuristic CONTINUE", r.NextAction)
	}
}

func TestValidatorRejectsBadConfidenceAndAction(t *testing.T) {
	v := NewValidator()
	if err := v.Validate(&Reflection{NextAction: NextFinish, Confidence: 1.5}); !errors.Is(err, ErrInvalidReflection) {
		t.Fatalf("err = %v", err)
	}
	if err := v.Validate(&Reflection{NextAction: "DANCE", Confidence: 0.2}); !errors.Is(err, ErrInvalidReflection) {
		t.Fatalf("err = %v", err)
	}
}

func TestAnalyzeAnswersSixQuestions(t *testing.T) {
	state := decision.NewState("what is 2 + 3?", 8)
	state.Plan = []string{"sum", "report"}
	obs := successObs()

	r := Analyze(state, obs)
	if r.NextAction != NextContinue {
		t.Fatalf("next_action = %s", r.NextAction)
	}
	if !r.EvidenceSufficient {
		t.Fatal("successful evidence should be sufficient")
	}
	if !r.ResultCorrect {
		t.Fatal("successful result should be marked correct")
	}
	if r.GoalSatisfied {
		t.Fatal("incomplete plan should not satisfy the goal")
	}
	if len(r.MissingInformation) == 0 {
		t.Fatal("expected missing information for remaining tasks")
	}

	state.CompletedTasks = []string{"sum", "report"}
	r = Analyze(state, obs)
	if !r.GoalSatisfied {
		t.Fatal("complete plan with evidence should satisfy the goal")
	}
}

func TestAnalyzeDataBackendUnavailableAsksUser(t *testing.T) {
	state := decision.NewState("Analyze why Meta ad ROI declined yesterday", 8)
	r := Analyze(state, decision.Observation{
		Summary:  "semantic_query error",
		Evidence: `TOOL_NOT_FOUND: tool "semantic_query": elasticsearch aggregate tool is not registered`,
	})
	if r.NextAction != NextAskUser {
		t.Fatalf("next_action = %s, want ASK_USER", r.NextAction)
	}
	if r.EvidenceSufficient || r.GoalSatisfied {
		t.Fatalf("reflection = %+v", r)
	}
}

func TestReflectSkipsLLMWhenDataBackendMissing(t *testing.T) {
	stub := &stubChatModel{reply: mustJSON(t, Reflection{
		GoalSatisfied:      false,
		EvidenceSufficient: false,
		ResultCorrect:      false,
		NextAction:         NextReplan,
		Confidence:         0.2,
	})}
	eng := testEngine(t, stub)
	r, err := eng.Reflect(context.Background(), decision.NewState("Analyze yesterday Meta ROI", 8), decision.Observation{
		Summary:  "analyze_advertising error",
		Evidence: "elasticsearch aggregate tool is not registered",
	})
	if err != nil {
		t.Fatalf("Reflect: %v", err)
	}
	if r.NextAction != NextAskUser {
		t.Fatalf("next_action = %s, want ASK_USER", r.NextAction)
	}
	if stub.called != 0 {
		t.Fatalf("llm called %d times, want 0", stub.called)
	}
}

func TestAnalyzeToolError(t *testing.T) {
	r := Analyze(decision.NewState("goal", 8), decision.Observation{
		Summary:  "calculator error",
		Evidence: "division by zero",
	})
	if r.ResultCorrect || r.EvidenceSufficient || r.GoalSatisfied {
		t.Fatalf("error observation = %+v", r)
	}
	if len(r.Errors) == 0 {
		t.Fatal("expected errors")
	}
	if r.NextAction != NextContinue {
		t.Fatalf("next_action = %s", r.NextAction)
	}
}

func TestLoopForcesReplanWithoutLLM(t *testing.T) {
	stub := &stubChatModel{reply: mustJSON(t, Reflection{
		GoalSatisfied:      true,
		EvidenceSufficient: true,
		ResultCorrect:      true,
		NextAction:         NextFinish,
		Confidence:         1,
	})}
	eng := testEngine(t, stub)

	state := decision.NewState("find eino docs", 8)
	p := decision.NewPolicy()
	same := &decision.Decision{
		Type:       decision.DecisionCallTool,
		Reason:     "search once",
		Tool:       "search",
		Arguments:  map[string]any{"q": "eino"},
		Confidence: 0.7,
	}
	p.Apply(state, same)
	p.Apply(state, same)
	p.Apply(state, same)

	r, err := eng.Reflect(context.Background(), state, successObs())
	if err != nil {
		t.Fatalf("Reflect: %v", err)
	}
	if r.NextAction != NextReplan {
		t.Fatalf("next_action = %s, want REPLAN", r.NextAction)
	}
	if !strings.Contains(strings.Join(r.Errors, " "), "three times") {
		t.Fatalf("errors = %v", r.Errors)
	}
	if stub.called != 0 {
		t.Fatalf("llm called %d times, want 0", stub.called)
	}
}

func TestLoopDetectedFromToolCalls(t *testing.T) {
	state := decision.NewState("goal", 8)
	call := decision.ToolCallRecord{Tool: "calculator", Arguments: map[string]any{"op": "sum", "a": 1, "b": 2}}
	state.ToolCalls = []decision.ToolCallRecord{call, call, call}
	if !LoopDetected(state) {
		t.Fatal("three identical tool calls should detect a loop")
	}
}

func TestHeuristicEngineNoModel(t *testing.T) {
	eng := NewHeuristic(logger.New("error", "json", io.Discard))
	r, err := eng.Reflect(context.Background(), decision.NewState("goal", 8), successObs())
	if err != nil {
		t.Fatalf("Reflect: %v", err)
	}
	if r.NextAction != NextContinue {
		t.Fatalf("next_action = %s", r.NextAction)
	}
	if !r.EvidenceSufficient {
		t.Fatal("expected sufficient evidence")
	}
}

func TestFinishRejectedWhenToolErrored(t *testing.T) {
	reply := mustJSON(t, Reflection{
		GoalSatisfied:      true,
		EvidenceSufficient: true,
		ResultCorrect:      true,
		NextAction:         NextFinish,
		Confidence:         0.9,
	})
	eng := testEngine(t, &stubChatModel{reply: reply})
	r, err := eng.Reflect(context.Background(), decision.NewState("goal", 8), decision.Observation{
		Summary:  "calculator error",
		Evidence: "division by zero",
	})
	if err != nil {
		t.Fatalf("Reflect: %v", err)
	}
	if r.NextAction != NextContinue {
		t.Fatalf("next_action = %s, want CONTINUE after tool error", r.NextAction)
	}
	if r.GoalSatisfied {
		t.Fatal("should not mark goal satisfied after a tool error")
	}
}
