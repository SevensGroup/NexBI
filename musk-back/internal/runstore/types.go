package runstore

import (
	"time"

	"autonomous-agent/internal/observability"
)

// Status is an Agent Run lifecycle state.
type Status string

const (
	StatusCreated         Status = "CREATED"
	StatusRunning         Status = "RUNNING"
	StatusWaitingApproval Status = "WAITING_APPROVAL"
	StatusCompleted       Status = "COMPLETED"
	StatusFailed          Status = "FAILED"
	StatusAborted         Status = "ABORTED"
)

// Run is a persisted Agent Run: goal, status, steps, decisions, tools, observations, reflection, result.
type Run struct {
	ID           string                   `json:"id"`
	Goal         string                   `json:"goal"`
	Status       Status                   `json:"status"`
	Steps        []Step                   `json:"steps"`
	Decisions    []DecisionRecord         `json:"decisions"`
	ToolCalls    []ToolCallRecord         `json:"tool_calls"`
	Observations []ObservationRecord      `json:"observations"`
	Reflections  []ReflectionRecord       `json:"reflections"`
	FinalResult  string                   `json:"final_result,omitempty"`
	Answer       string                   `json:"answer,omitempty"`
	Confidence   float64                  `json:"confidence,omitempty"`
	Evidence     []any                    `json:"evidence,omitempty"`
	TokenUsage   observability.TokenUsage `json:"token_usage"`
	CostUSD      float64                  `json:"cost_usd,omitempty"`
	LatencyMS    int64                    `json:"latency_ms,omitempty"`
	Error        string                   `json:"error,omitempty"`
	ApprovalID   string                   `json:"approval_id,omitempty"`
	CreatedAt    time.Time                `json:"created_at"`
	UpdatedAt    time.Time                `json:"updated_at"`
}

// Step is one decision-loop iteration.
type Step struct {
	ID        string    `json:"id"`
	Index     int       `json:"index"`
	StartedAt time.Time `json:"started_at"`
}

// DecisionRecord is a structured Decision persisted for a run.
type DecisionRecord struct {
	StepID         string         `json:"step_id,omitempty"`
	Type           string         `json:"type"`
	Thought        string         `json:"thought,omitempty"`
	Reason         string         `json:"reason"`
	Objective      string         `json:"objective,omitempty"`
	Tool           string         `json:"tool,omitempty"`
	Arguments      map[string]any `json:"arguments,omitempty"`
	ExpectedResult string         `json:"expected_result,omitempty"`
	Confidence     float64        `json:"confidence"`
	Risk           string         `json:"risk,omitempty"`
	At             time.Time      `json:"at"`
}

// ToolCallRecord is a real tool invocation (never LLM-fabricated).
type ToolCallRecord struct {
	ID        string         `json:"id"`
	StepID    string         `json:"step_id,omitempty"`
	Tool      string         `json:"tool"`
	Arguments map[string]any `json:"arguments,omitempty"`
	Result    string         `json:"result,omitempty"`
	Success   bool           `json:"success"`
	LatencyMS int64          `json:"latency_ms,omitempty"`
	At        time.Time      `json:"at"`
}

// ObservationRecord is summary/reason/evidence/confidence after a decision or tool.
type ObservationRecord struct {
	StepID     string    `json:"step_id,omitempty"`
	Summary    string    `json:"summary"`
	Reason     string    `json:"reason"`
	Evidence   string    `json:"evidence"`
	Confidence float64   `json:"confidence"`
	At         time.Time `json:"at"`
}

// ReflectionRecord is a Reflection Engine judgment.
type ReflectionRecord struct {
	StepID             string    `json:"step_id,omitempty"`
	GoalSatisfied      bool      `json:"goal_satisfied"`
	EvidenceSufficient bool      `json:"evidence_sufficient"`
	ResultCorrect      bool      `json:"result_correct"`
	MissingInformation []string  `json:"missing_information,omitempty"`
	Errors             []string  `json:"errors,omitempty"`
	NextAction         string    `json:"next_action"`
	Confidence         float64   `json:"confidence"`
	At                 time.Time `json:"at"`
}

// Clone returns a deep-enough copy for concurrent readers.
func (r *Run) Clone() *Run {
	if r == nil {
		return nil
	}
	out := *r
	out.Steps = append([]Step(nil), r.Steps...)
	out.Decisions = append([]DecisionRecord(nil), r.Decisions...)
	out.ToolCalls = make([]ToolCallRecord, len(r.ToolCalls))
	for i, c := range r.ToolCalls {
		out.ToolCalls[i] = c
		if c.Arguments != nil {
			out.ToolCalls[i].Arguments = cloneMap(c.Arguments)
		}
	}
	out.Observations = append([]ObservationRecord(nil), r.Observations...)
	out.Reflections = append([]ReflectionRecord(nil), r.Reflections...)
	out.Evidence = append([]any(nil), r.Evidence...)
	return &out
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
