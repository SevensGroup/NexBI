package guardrail

import (
	"strings"

	"autonomous-agent/internal/tools"
)

// RiskLevel is the blast-radius classification used by the Risk Engine.
type RiskLevel = tools.RiskLevel

const (
	RiskLow      = tools.RiskLow
	RiskMedium   = tools.RiskMedium
	RiskHigh     = tools.RiskHigh
	RiskCritical = tools.RiskCritical
)

// Verdict is whether an action may run immediately or must wait for a human.
type Verdict string

const (
	// VerdictAllow means the action is executed without interrupt.
	VerdictAllow Verdict = "ALLOW"
	// VerdictApprove means the agent must interrupt for Human-in-the-Loop.
	VerdictApprove Verdict = "REQUIRE_APPROVAL"
)

// Action is a proposed tool (or equivalent) invocation to classify.
type Action struct {
	Tool           string
	Arguments      map[string]any
	RegisteredRisk RiskLevel
}

// Assessment is the Risk Engine result for one action.
type Assessment struct {
	Level   RiskLevel
	Verdict Verdict
	Reason  string
}

// RequiresApproval reports whether the agent must CreateApproval and interrupt.
func (a Assessment) RequiresApproval() bool {
	return a.Verdict == VerdictApprove
}

// Policy controls MEDIUM auto-execution. HIGH and CRITICAL always require approval.
type Policy struct {
	// MediumAuto is LOW-style auto execute for MEDIUM when true.
	// When false, MEDIUM also requires approval.
	MediumAuto bool
}

// Engine is the Risk Engine: classify, then allow or require approval.
type Engine struct {
	policy Policy
}

// NewEngine constructs a Risk Engine. Pass Policy{MediumAuto: true} to auto-run MEDIUM actions.
func NewEngine(policy Policy) *Engine {
	return &Engine{policy: policy}
}

// Evaluate classifies an action and returns ALLOW or REQUIRE_APPROVAL.
// LOW is always auto. MEDIUM follows policy. HIGH / CRITICAL always require approval.
func (e *Engine) Evaluate(action Action) (Assessment, error) {
	if strings.TrimSpace(action.Tool) == "" && action.RegisteredRisk == "" {
		return Assessment{}, ErrInvalidAction
	}

	level := Classify(action)
	assess := Assessment{
		Level:   level,
		Verdict: VerdictAllow,
		Reason:  reasonFor(level),
	}

	switch level {
	case RiskLow:
		assess.Verdict = VerdictAllow
		assess.Reason = "LOW: query, calculate, or search — auto execute"
	case RiskMedium:
		if e != nil && !e.policy.MediumAuto {
			assess.Verdict = VerdictApprove
			assess.Reason = "MEDIUM: large query or long-running task — approval required by config"
		} else {
			assess.Verdict = VerdictAllow
			assess.Reason = "MEDIUM: large query or long-running task — auto execute by config"
		}
	case RiskHigh:
		assess.Verdict = VerdictApprove
		assess.Reason = "HIGH: modify business data, advertising config, or call an external API — approval required"
	case RiskCritical:
		assess.Verdict = VerdictApprove
		assess.Reason = "CRITICAL: delete, funds, or irreversible production operation — approval required"
	default:
		assess.Level = RiskHigh
		assess.Verdict = VerdictApprove
		assess.Reason = "unknown risk — approval required"
	}
	return assess, nil
}

func reasonFor(level RiskLevel) string {
	switch level {
	case RiskLow:
		return "LOW"
	case RiskMedium:
		return "MEDIUM"
	case RiskHigh:
		return "HIGH"
	case RiskCritical:
		return "CRITICAL"
	default:
		return string(level)
	}
}
