package decision

import (
	"fmt"
	"strings"
)

// Policy enforces loop limits, duplicate-action prevention, and CONTINUE/FINISH/ASK_USER/ABORT.
type Policy struct{}

// NewPolicy constructs the decision-loop policy.
func NewPolicy() *Policy {
	return &Policy{}
}

// MaxStepsReached reports whether the engine must stop before another LLM decision.
func (p *Policy) MaxStepsReached(state *AgentState) bool {
	if state == nil {
		return true
	}
	if state.MaxSteps < 1 {
		return true
	}
	return state.CurrentStep >= state.MaxSteps
}

// CheckDuplicate rejects a fully identical action that was already applied.
func (p *Policy) CheckDuplicate(state *AgentState, d *Decision) error {
	if state == nil || d == nil {
		return nil
	}
	switch d.Type {
	case DecisionFinish, DecisionAskUser, DecisionAbort:
		return nil
	}
	if state.hasAction(d) {
		return fmt.Errorf("%w: %s", ErrDuplicateAction, d.summary())
	}
	return nil
}

// Evaluate maps a validated decision onto loop control.
func (p *Policy) Evaluate(d *Decision) Outcome {
	if d == nil {
		return OutcomeAbort
	}
	switch d.Type {
	case DecisionFinish:
		return OutcomeFinish
	case DecisionAskUser:
		return OutcomeAskUser
	case DecisionAbort:
		return OutcomeAbort
	default:
		return OutcomeContinue
	}
}

// Apply records the decision into state without fabricating tool results.
func (p *Policy) Apply(state *AgentState, d *Decision) Outcome {
	if state == nil || d == nil {
		return OutcomeAbort
	}

	state.Confidence = d.Confidence
	state.Observations = append(state.Observations, Observation{
		Summary:    d.summary(),
		Reason:     d.Reason,
		Evidence:   d.evidence(),
		Confidence: d.Confidence,
	})

	switch d.Type {
	case DecisionCallTool:
		state.ToolCalls = append(state.ToolCalls, ToolCallRecord{
			Tool:      d.Tool,
			Arguments: cloneArgs(d.Arguments),
		})
		if d.Objective != "" {
			state.CurrentObjective = d.Objective
		}
	case DecisionDelegateAgent:
		if d.Objective != "" {
			state.CurrentObjective = d.Objective
		}
	case DecisionReplan:
		if d.Objective != "" {
			state.CurrentObjective = d.Objective
		}
		if plan := planFromArgs(d.Arguments); len(plan) > 0 {
			state.Plan = plan
		} else if d.Objective != "" {
			state.Plan = []string{d.Objective}
		}
	case DecisionReflect:
		// Observation already recorded. Reflection must not invent tool results.
	case DecisionFinish:
		state.Finished = true
		if d.Objective != "" {
			state.CurrentObjective = d.Objective
		}
		if d.Objective != "" {
			state.CompletedTasks = append(state.CompletedTasks, d.Objective)
		}
	case DecisionAskUser:
		state.Finished = true
	case DecisionAbort:
		state.Finished = true
		if d.Reason != "" {
			state.FailedTasks = append(state.FailedTasks, d.Reason)
		}
	}

	state.remember(d)
	return p.Evaluate(d)
}

func abortDecision(reason string) *Decision {
	return &Decision{
		Type:       DecisionAbort,
		Reason:     reason,
		Objective:  reason,
		Confidence: 1,
		Risk:       "high",
	}
}

func cloneArgs(args map[string]any) map[string]any {
	if args == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(args))
	for k, v := range args {
		out[k] = v
	}
	return out
}

func planFromArgs(args map[string]any) []string {
	if args == nil {
		return nil
	}
	raw, ok := args["plan"]
	if !ok {
		return nil
	}
	switch typed := raw.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		plan := make([]string, 0, len(typed))
		for _, item := range typed {
			s := strings.TrimSpace(fmt.Sprint(item))
			if s != "" {
				plan = append(plan, s)
			}
		}
		return plan
	case string:
		if s := strings.TrimSpace(typed); s != "" {
			return []string{s}
		}
	}
	return nil
}
