package decision

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

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

func mustJSON(t *testing.T, d Decision) string {
	t.Helper()
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(raw)
}

func TestValidDecision(t *testing.T) {
	reply := mustJSON(t, Decision{
		Type:           DecisionCallTool,
		Reason:         "need current weather",
		Objective:      "get weather",
		Tool:           "weather",
		Arguments:      map[string]any{"city": "Paris"},
		ExpectedResult: "temperature in C",
		Confidence:     0.8,
		Risk:           "low",
	})
	eng := testEngine(t, &stubChatModel{reply: reply})

	d, err := eng.Decide(context.Background(), NewState("what is the weather in Paris?", 8))
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if d.Type != DecisionCallTool {
		t.Fatalf("type = %s", d.Type)
	}
	if d.Tool != "weather" {
		t.Fatalf("tool = %q", d.Tool)
	}
	if d.Confidence != 0.8 {
		t.Fatalf("confidence = %v", d.Confidence)
	}
	if outcome := eng.Apply(NewState("g", 8), d); outcome != OutcomeContinue {
		t.Fatalf("outcome = %s", outcome)
	}
}

func TestInvalidDecision(t *testing.T) {
	t.Run("bad confidence", func(t *testing.T) {
		eng := testEngine(t, &stubChatModel{reply: mustJSON(t, Decision{
			Type:       DecisionFinish,
			Reason:     "done",
			Confidence: 1.5,
		})})
		if _, err := eng.Decide(context.Background(), NewState("goal", 8)); !errors.Is(err, ErrInvalidDecision) {
			t.Fatalf("err = %v, want ErrInvalidDecision", err)
		}
	})

	t.Run("unknown type", func(t *testing.T) {
		eng := testEngine(t, &stubChatModel{reply: `{"type":"DANCE","reason":"nope","confidence":0.2}`})
		if _, err := eng.Decide(context.Background(), NewState("goal", 8)); !errors.Is(err, ErrInvalidDecision) {
			t.Fatalf("err = %v, want ErrInvalidDecision", err)
		}
	})

	t.Run("call tool without tool name", func(t *testing.T) {
		v := NewValidator()
		err := v.Validate(&Decision{Type: DecisionCallTool, Reason: "go", Confidence: 0.5})
		if !errors.Is(err, ErrInvalidDecision) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("forged tool result", func(t *testing.T) {
		v := NewValidator()
		err := v.Validate(&Decision{
			Type:       DecisionCallTool,
			Reason:     "already ran it",
			Tool:       "search",
			Arguments:  map[string]any{"query": "x", "tool_result": "I made this up"},
			Confidence: 0.9,
		})
		if !errors.Is(err, ErrInvalidDecision) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("not json", func(t *testing.T) {
		eng := testEngine(t, &stubChatModel{reply: "sure, I will help"})
		if _, err := eng.Decide(context.Background(), NewState("goal", 8)); !errors.Is(err, ErrInvalidDecision) {
			t.Fatalf("err = %v, want ErrInvalidDecision", err)
		}
	})
}

func TestMaxSteps(t *testing.T) {
	stub := &stubChatModel{reply: mustJSON(t, Decision{Type: DecisionFinish, Reason: "should not be used", Confidence: 1})}
	eng := testEngine(t, stub)

	state := NewState("goal", 3)
	state.CurrentStep = 3

	d, err := eng.Decide(context.Background(), state)
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if d.Type != DecisionAbort {
		t.Fatalf("type = %s, want ABORT", d.Type)
	}
	if stub.called != 0 {
		t.Fatalf("llm called %d times, want 0", stub.called)
	}
	if !strings.Contains(d.Reason, "maximum steps") {
		t.Fatalf("reason = %q", d.Reason)
	}
	if outcome := eng.Apply(state, d); outcome != OutcomeAbort {
		t.Fatalf("outcome = %s", outcome)
	}
}

func TestDuplicateAction(t *testing.T) {
	first := Decision{
		Type:       DecisionCallTool,
		Reason:     "search once",
		Tool:       "search",
		Arguments:  map[string]any{"q": "eino"},
		Confidence: 0.7,
	}
	eng := testEngine(t, &stubChatModel{reply: mustJSON(t, first)})

	state := NewState("find eino docs", 8)
	d, err := eng.Decide(context.Background(), state)
	if err != nil {
		t.Fatalf("first Decide: %v", err)
	}
	if eng.Apply(state, d) != OutcomeContinue {
		t.Fatal("first action should continue")
	}
	state.CurrentStep++

	_, err = eng.Decide(context.Background(), state)
	if !errors.Is(err, ErrDuplicateAction) {
		t.Fatalf("err = %v, want ErrDuplicateAction", err)
	}
}

func TestFinishDecision(t *testing.T) {
	eng := testEngine(t, &stubChatModel{reply: mustJSON(t, Decision{
		Type:       DecisionFinish,
		Reason:     "goal complete",
		Objective:  "the answer is 42",
		Confidence: 0.95,
		Risk:       "low",
	})})

	state := NewState("meaning of life", 8)
	d, err := eng.Decide(context.Background(), state)
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if d.Type != DecisionFinish {
		t.Fatalf("type = %s", d.Type)
	}
	if outcome := eng.Apply(state, d); outcome != OutcomeFinish {
		t.Fatalf("outcome = %s", outcome)
	}
	if !state.Finished {
		t.Fatal("state should be finished")
	}
	if got := TerminalAnswer(d); got != "the answer is 42" {
		t.Fatalf("answer = %q", got)
	}
}

func TestAbortDecision(t *testing.T) {
	eng := testEngine(t, &stubChatModel{reply: mustJSON(t, Decision{
		Type:       DecisionAbort,
		Reason:     "unsafe request",
		Objective:  "cannot proceed",
		Confidence: 1,
		Risk:       "high",
	})})

	state := NewState("do something unsafe", 8)
	d, err := eng.Decide(context.Background(), state)
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if d.Type != DecisionAbort {
		t.Fatalf("type = %s", d.Type)
	}
	if outcome := eng.Apply(state, d); outcome != OutcomeAbort {
		t.Fatalf("outcome = %s", outcome)
	}
	if !state.Finished {
		t.Fatal("state should be finished")
	}
}

func TestPolicyOutcomes(t *testing.T) {
	p := NewPolicy()
	cases := []struct {
		typ  DecisionType
		want Outcome
	}{
		{DecisionCallTool, OutcomeContinue},
		{DecisionDelegateAgent, OutcomeContinue},
		{DecisionReplan, OutcomeContinue},
		{DecisionReflect, OutcomeContinue},
		{DecisionFinish, OutcomeFinish},
		{DecisionAskUser, OutcomeAskUser},
		{DecisionAbort, OutcomeAbort},
	}
	for _, tc := range cases {
		if got := p.Evaluate(&Decision{Type: tc.typ}); got != tc.want {
			t.Fatalf("%s: outcome = %s, want %s", tc.typ, got, tc.want)
		}
	}
}

func TestConsecutiveActionRepeats(t *testing.T) {
	state := NewState("goal", 8)
	p := NewPolicy()
	same := &Decision{
		Type:       DecisionCallTool,
		Reason:     "search",
		Tool:       "search",
		Arguments:  map[string]any{"q": "eino"},
		Confidence: 0.5,
	}
	if state.ConsecutiveActionRepeats(3) || state.ConsecutiveToolRepeats(3) {
		t.Fatal("empty state should not look like a loop")
	}
	p.Apply(state, same)
	p.Apply(state, same)
	if state.ConsecutiveActionRepeats(3) {
		t.Fatal("two repeats should not trip the three-in-a-row detector")
	}
	p.Apply(state, same)
	if !state.ConsecutiveActionRepeats(3) {
		t.Fatal("three identical decisions should trip ConsecutiveActionRepeats")
	}
	if !state.ConsecutiveToolRepeats(3) {
		t.Fatal("three identical tool calls should trip ConsecutiveToolRepeats")
	}
}

func TestApplyDoesNotForgeToolResult(t *testing.T) {
	state := NewState("goal", 8)
	p := NewPolicy()
	p.Apply(state, &Decision{
		Type:           DecisionCallTool,
		Reason:         "call search",
		Tool:           "search",
		Arguments:      map[string]any{"q": "x"},
		ExpectedResult: "a list of hits",
		Confidence:     0.6,
	})
	if len(state.ToolCalls) != 1 {
		t.Fatalf("tool calls = %d", len(state.ToolCalls))
	}
	if state.ToolCalls[0].Result != "" {
		t.Fatalf("forged result = %q", state.ToolCalls[0].Result)
	}
}

func TestDelegateAgentValidation(t *testing.T) {
	v := NewValidator()
	err := v.Validate(&Decision{Type: DecisionDelegateAgent, Reason: "hand off", Confidence: 0.5})
	if !errors.Is(err, ErrInvalidDecision) {
		t.Fatalf("missing agent/task err = %v", err)
	}

	fallback := &Decision{
		Type:       DecisionDelegateAgent,
		Reason:     "need analysis",
		Objective:  "Analyze why Meta ROAS declined yesterday",
		Confidence: 0.7,
	}
	if err := v.Validate(fallback); err != nil {
		t.Fatalf("DELEGATE_AGENT with objective should default agent: %v", err)
	}
	fallbackAgent, fallbackTask := DelegateTarget(fallback)
	if fallbackAgent != "analyst" || fallbackTask != "Analyze why Meta ROAS declined yesterday" {
		t.Fatalf("defaulted target = %s %s", fallbackAgent, fallbackTask)
	}

	d := &Decision{
		Type:       DecisionDelegateAgent,
		Reason:     "need analysis",
		Arguments:  map[string]any{"agent": "analyst", "task": "Analyze why Meta ROAS declined yesterday"},
		Confidence: 0.7,
	}
	if err := v.Validate(d); err != nil {
		t.Fatalf("valid DELEGATE_AGENT: %v", err)
	}
	if d.Objective != "Analyze why Meta ROAS declined yesterday" {
		t.Fatalf("objective fallback = %q", d.Objective)
	}
	agent, task := DelegateTarget(d)
	if agent != "analyst" || task != "Analyze why Meta ROAS declined yesterday" {
		t.Fatalf("target = %s %s", agent, task)
	}
}

func TestDecideStringifiedArguments(t *testing.T) {
	reply := `{
		"type":"CALL_TOOL",
		"reason":"query roi drop",
		"objective":"analyze Meta ROAS",
		"tool":"semantic_query",
		"arguments":"{\"nl\":\"Why did Meta ad ROI decline yesterday\"}",
		"expected_result":"metric evidence",
		"confidence":0.8
	}`
	stub := &stubChatModel{reply: reply}
	eng := testEngine(t, stub)

	d, err := eng.Decide(context.Background(), NewState("Analyze why Meta ad ROI declined yesterday", 8))
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if d.Type != DecisionCallTool || d.Tool != "semantic_query" {
		t.Fatalf("decision = %s %s", d.Type, d.Tool)
	}
	if got, _ := d.Arguments["nl"].(string); got != "Why did Meta ad ROI decline yesterday" {
		t.Fatalf("nl = %q", got)
	}
	if stub.called != 1 {
		t.Fatalf("called = %d, want 1 (coerce, no retry)", stub.called)
	}
}

func TestDecideRetriesInvalidThenSucceeds(t *testing.T) {
	n := 0
	stub := &stubChatModel{generateFn: func(ctx context.Context, input []*schema.Message, opts ...einomodel.Option) (*schema.Message, error) {
		n++
		if n == 1 {
			return schema.AssistantMessage("sure, I will help", nil), nil
		}
		return schema.AssistantMessage(mustJSON(t, Decision{
			Type:       DecisionFinish,
			Reason:     "goal complete",
			Objective:  "done",
			Confidence: 1,
		}), nil), nil
	}}
	eng := testEngine(t, stub)

	d, err := eng.Decide(context.Background(), NewState("goal", 8))
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if d.Type != DecisionFinish {
		t.Fatalf("type = %s", d.Type)
	}
	if n != 2 {
		t.Fatalf("generate called %d times, want 2", n)
	}
}

func TestDecideInvalidAfterRetry(t *testing.T) {
	stub := &stubChatModel{reply: "sure, I will help"}
	eng := testEngine(t, stub)

	_, err := eng.Decide(context.Background(), NewState("goal", 8))
	if !errors.Is(err, ErrInvalidDecision) {
		t.Fatalf("err = %v, want ErrInvalidDecision", err)
	}
	if stub.called != maxDecideAttempts {
		t.Fatalf("called = %d, want %d", stub.called, maxDecideAttempts)
	}
}

func TestClearActionHistoryAllowsRedelegate(t *testing.T) {
	state := NewState("goal", 8)
	p := NewPolicy()
	d := &Decision{
		Type:       DecisionDelegateAgent,
		Reason:     "need analysis",
		Arguments:  map[string]any{"agent": "analyst", "task": "Analyze why Meta ROAS declined yesterday"},
		Objective:  "Analyze why Meta ROAS declined yesterday",
		Confidence: 0.6,
	}
	p.Apply(state, d)
	if err := p.CheckDuplicate(state, d); !errors.Is(err, ErrDuplicateAction) {
		t.Fatalf("err = %v, want duplicate", err)
	}
	state.ClearActionHistory()
	if err := p.CheckDuplicate(state, d); err != nil {
		t.Fatalf("after REPLAN re-delegate should be allowed: %v", err)
	}
}
