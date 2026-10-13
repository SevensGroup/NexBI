package decision

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrInvalidDecision is returned when a structured decision fails validation.
	ErrInvalidDecision = errors.New("invalid decision")
	// ErrDuplicateAction is returned when the same action would be executed again.
	ErrDuplicateAction = errors.New("duplicate action")
	// ErrEmptyState is returned when Decide is called without agent state.
	ErrEmptyState = errors.New("agent state is required")
)

var validDecisionTypes = map[DecisionType]struct{}{
	DecisionCallTool:      {},
	DecisionDelegateAgent: {},
	DecisionReplan:        {},
	DecisionReflect:       {},
	DecisionAskUser:       {},
	DecisionFinish:        {},
	DecisionAbort:         {},
}

var forgedResultKeys = map[string]struct{}{
	"tool_result":      {},
	"toolResult":       {},
	"tool_output":      {},
	"toolOutput":       {},
	"forged_result":    {},
	"simulated_result": {},
	"fake_result":      {},
}

var droppedCoTKeys = map[string]struct{}{
	"chain_of_thought":  {},
	"chainOfThought":    {},
	"thinking":          {},
	"reasoning":         {},
	"reasoning_content": {},
	"cot":               {},
}

// Validator rejects illegal structured decisions.
type Validator struct{}

// NewValidator constructs a Decision validator.
func NewValidator() *Validator {
	return &Validator{}
}

// Validate checks type, confidence, required fields, and forged tool results.
func (v *Validator) Validate(d *Decision) error {
	if d == nil {
		return fmt.Errorf("%w: decision is nil", ErrInvalidDecision)
	}

	d.Type = DecisionType(strings.ToUpper(strings.TrimSpace(string(d.Type))))
	d.Reason = strings.TrimSpace(d.Reason)
	d.Objective = strings.TrimSpace(d.Objective)
	d.Tool = strings.TrimSpace(d.Tool)
	d.ExpectedResult = strings.TrimSpace(d.ExpectedResult)
	d.Risk = strings.TrimSpace(d.Risk)

	if _, ok := validDecisionTypes[d.Type]; !ok {
		return fmt.Errorf("%w: unknown type %q", ErrInvalidDecision, d.Type)
	}
	if d.Reason == "" {
		return fmt.Errorf("%w: reason is required", ErrInvalidDecision)
	}
	if d.Confidence < 0 || d.Confidence > 1 {
		return fmt.Errorf("%w: confidence must be between 0 and 1", ErrInvalidDecision)
	}
	if d.Type == DecisionCallTool && d.Tool == "" {
		return fmt.Errorf("%w: tool is required for CALL_TOOL", ErrInvalidDecision)
	}
	if d.Type == DecisionDelegateAgent {
		agent, task := DelegateTarget(d)
		if agent == "" && task != "" {
			agent = "analyst"
		}
		if agent == "" {
			return fmt.Errorf("%w: agent is required for DELEGATE_AGENT", ErrInvalidDecision)
		}
		if task == "" {
			return fmt.Errorf("%w: task is required for DELEGATE_AGENT", ErrInvalidDecision)
		}
		if d.Arguments == nil {
			d.Arguments = map[string]any{}
		}
		d.Arguments["agent"] = agent
		d.Arguments["task"] = task
		if d.Objective == "" {
			d.Objective = task
		}
	}
	if err := rejectForgedToolResult(d); err != nil {
		return err
	}
	return nil
}

func rejectForgedToolResult(d *Decision) error {
	if _, ok := forgedResultKeys[d.Tool]; ok {
		return fmt.Errorf("%w: forging tool results is not allowed", ErrInvalidDecision)
	}
	for key := range d.Arguments {
		if _, ok := forgedResultKeys[key]; ok {
			return fmt.Errorf("%w: forging tool results is not allowed", ErrInvalidDecision)
		}
	}
	return nil
}

func stripCoT(raw map[string]any) {
	for key := range droppedCoTKeys {
		delete(raw, key)
	}
	for key := range forgedResultKeys {
		delete(raw, key)
	}
}

func decodeDecision(payload []byte) (*Decision, error) {
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidDecision, err)
	}
	stripCoT(raw)

	normalized, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidDecision, err)
	}

	var d Decision
	if err := json.Unmarshal(normalized, &d); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidDecision, err)
	}
	return &d, nil
}
