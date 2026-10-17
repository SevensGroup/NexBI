package decision

import (
	"encoding/json"
	"strings"
)

// DecisionType is the next action the agent intends to take.
type DecisionType string

const (
	DecisionCallTool      DecisionType = "CALL_TOOL"
	DecisionDelegateAgent DecisionType = "DELEGATE_AGENT"
	DecisionReplan        DecisionType = "REPLAN"
	DecisionReflect       DecisionType = "REFLECT"
	DecisionAskUser       DecisionType = "ASK_USER"
	DecisionFinish        DecisionType = "FINISH"
	DecisionAbort         DecisionType = "ABORT"
)

// Outcome is the loop control signal after a validated decision is applied.
type Outcome string

const (
	OutcomeContinue        Outcome = "CONTINUE"
	OutcomeFinish          Outcome = "FINISH"
	OutcomeAskUser         Outcome = "ASK_USER"
	OutcomeAbort           Outcome = "ABORT"
	OutcomeWaitingApproval Outcome = "WAITING_APPROVAL"
	OutcomeResume          Outcome = "RESUME"
)

// Agent run status for Human-in-the-Loop interrupt / resume.
const (
	StatusRunning         = "RUNNING"
	StatusWaitingApproval = "WAITING_APPROVAL"
	StatusResume          = "RESUME"
	StatusFinished        = "FINISHED"
)

// Decision is the structured next-step produced by the LLM.
// It must not include chain-of-thought or a fabricated tool result.
type Decision struct {
	Type           DecisionType `json:"type" jsonschema:"required,enum=CALL_TOOL,enum=DELEGATE_AGENT,enum=REPLAN,enum=REFLECT,enum=ASK_USER,enum=FINISH,enum=ABORT" jsonschema_description:"Next action type"`
	Thought        string       `json:"thought,omitempty" jsonschema_description:"Visible thinking in English only, 2 to 4 first-person sentences. What you noticed and why the next action follows. Not a tool result."`
	Reason         string       `json:"reason" jsonschema:"required" jsonschema_description:"One-line summary of the decision. No hidden scratchpad."`
	Objective      string       `json:"objective,omitempty" jsonschema_description:"Current objective or user-facing conclusion/question"`
	Tool           string       `json:"tool,omitempty" jsonschema_description:"Tool name when type is CALL_TOOL"`
	Arguments      FlexibleMap  `json:"arguments,omitempty" jsonschema_description:"Tool or action arguments as a JSON object (not a string). Must not contain a tool result."`
	ExpectedResult string       `json:"expected_result,omitempty" jsonschema_description:"What a successful action should produce. Not an actual tool result."`
	Confidence     float64      `json:"confidence" jsonschema:"required,minimum=0,maximum=1" jsonschema_description:"Confidence in [0, 1]"`
	Risk           string       `json:"risk,omitempty" jsonschema_description:"Risk note for this action"`
}

// Observation is what the engine persists after a decision.
// It stores summary/reason/evidence/confidence only — never chain-of-thought.
type Observation struct {
	Summary    string  `json:"summary"`
	Reason     string  `json:"reason"`
	Evidence   string  `json:"evidence"`
	Confidence float64 `json:"confidence"`
}

// ToolCallRecord is a decided tool invocation.
// Result is filled only by real tool execution, never by the LLM.
type ToolCallRecord struct {
	ID        string         `json:"id,omitempty"`
	Tool      string         `json:"tool"`
	Arguments map[string]any `json:"arguments,omitempty"`
	Result    string         `json:"result,omitempty"`
	Success   bool           `json:"success,omitempty"`
}

// ToolListing is a registry entry exposed to the decision engine.
// The engine must CALL_TOOL by name; it never executes a concrete tool.
type ToolListing struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"input_schema,omitempty"`
	Risk        string `json:"risk,omitempty"`
}

// AgentListing is a specialist the Coordinator may DELEGATE_AGENT to.
type AgentListing struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	InputSchema  any    `json:"input_schema,omitempty"`
	OutputSchema any    `json:"output_schema,omitempty"`
}

// DelegationRecord is a completed DELEGATE_AGENT observation.
type DelegationRecord struct {
	Agent    string `json:"agent"`
	Task     string `json:"task"`
	Result   string `json:"result,omitempty"`
	Approved *bool  `json:"approved,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// AgentState is the working memory of one agent run.
type AgentState struct {
	Goal              string             `json:"goal"`
	CurrentObjective  string             `json:"current_objective"`
	Plan              []string           `json:"plan,omitempty"`
	CompletedTasks    []string           `json:"completed_tasks,omitempty"`
	FailedTasks       []string           `json:"failed_tasks,omitempty"`
	Observations      []Observation      `json:"observations,omitempty"`
	ToolCalls         []ToolCallRecord   `json:"tool_calls,omitempty"`
	AvailableTools    []ToolListing      `json:"available_tools,omitempty"`
	AvailableAgents   []AgentListing     `json:"available_agents,omitempty"`
	Delegations       []DelegationRecord `json:"delegations,omitempty"`
	Memories          []MemoryRecall     `json:"memories,omitempty"`
	Reflections       []ReflectionView   `json:"reflections,omitempty"`
	RunID             string             `json:"run_id,omitempty"`
	CurrentStep       int                `json:"current_step"`
	CurrentStepID     string             `json:"current_step_id,omitempty"`
	MaxSteps          int                `json:"max_steps"`
	Confidence        float64            `json:"confidence"`
	Finished          bool               `json:"finished"`
	Status            string             `json:"status,omitempty"`
	PendingApprovalID string             `json:"pending_approval_id,omitempty"`

	executed []string
}

// ReflectionView is the decision-loop copy of a Reflection Engine result.
// The engine type lives in internal/agent/reflection to avoid an import cycle.
type ReflectionView struct {
	GoalSatisfied      bool     `json:"goal_satisfied"`
	EvidenceSufficient bool     `json:"evidence_sufficient"`
	ResultCorrect      bool     `json:"result_correct"`
	MissingInformation []string `json:"missing_information,omitempty"`
	Errors             []string `json:"errors,omitempty"`
	NextAction         string   `json:"next_action"`
	Confidence         float64  `json:"confidence"`
}

// MemoryRecall is a snippet retrieved from persistent memory for the decision prompt.
type MemoryRecall struct {
	ID       string         `json:"id,omitempty"`
	Layer    string         `json:"layer"`
	Kind     string         `json:"kind"`
	Content  any            `json:"content"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// NewState starts a run from a user goal.
func NewState(goal string, maxSteps int) *AgentState {
	goal = strings.TrimSpace(goal)
	return &AgentState{
		Goal:             goal,
		CurrentObjective: goal,
		Plan:             []string{},
		CompletedTasks:   []string{},
		FailedTasks:      []string{},
		Observations:     []Observation{},
		ToolCalls:        []ToolCallRecord{},
		AvailableTools:   []ToolListing{},
		AvailableAgents:  []AgentListing{},
		Delegations:      []DelegationRecord{},
		Reflections:      []ReflectionView{},
		CurrentStep:      0,
		MaxSteps:         maxSteps,
		Status:           StatusRunning,
	}
}

// Clone is a deep-enough copy for interrupt checkpoints (including action fingerprints).
func (s *AgentState) Clone() *AgentState {
	if s == nil {
		return nil
	}
	out := *s
	out.Plan = append([]string(nil), s.Plan...)
	out.CompletedTasks = append([]string(nil), s.CompletedTasks...)
	out.FailedTasks = append([]string(nil), s.FailedTasks...)
	out.Observations = append([]Observation(nil), s.Observations...)
	out.ToolCalls = make([]ToolCallRecord, len(s.ToolCalls))
	for i, call := range s.ToolCalls {
		out.ToolCalls[i] = ToolCallRecord{
			ID:        call.ID,
			Tool:      call.Tool,
			Arguments: cloneMap(call.Arguments),
			Result:    call.Result,
		}
	}
	out.AvailableTools = append([]ToolListing(nil), s.AvailableTools...)
	out.AvailableAgents = append([]AgentListing(nil), s.AvailableAgents...)
	out.Delegations = append([]DelegationRecord(nil), s.Delegations...)
	out.Memories = append([]MemoryRecall(nil), s.Memories...)
	out.Reflections = append([]ReflectionView(nil), s.Reflections...)
	out.executed = append([]string(nil), s.executed...)
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

// Understand sets the current objective from the goal when it is empty.
func Understand(state *AgentState) {
	if state == nil {
		return
	}
	if strings.TrimSpace(state.CurrentObjective) == "" {
		state.CurrentObjective = strings.TrimSpace(state.Goal)
	}
}

func (d *Decision) summary() string {
	if d == nil {
		return ""
	}
	parts := []string{string(d.Type)}
	if obj := strings.TrimSpace(d.Objective); obj != "" {
		parts = append(parts, obj)
	} else if tool := strings.TrimSpace(d.Tool); tool != "" {
		parts = append(parts, tool)
	} else if reason := strings.TrimSpace(d.Reason); reason != "" {
		parts = append(parts, reason)
	}
	return strings.Join(parts, ": ")
}

func (d *Decision) evidence() string {
	if d == nil {
		return ""
	}
	return strings.TrimSpace(d.ExpectedResult)
}

func (d *Decision) fingerprint() string {
	args := d.Arguments
	if args == nil {
		args = map[string]any{}
	}
	raw, _ := json.Marshal(args)
	return string(d.Type) + "\x00" + strings.TrimSpace(d.Tool) + "\x00" + string(raw)
}

// Forget drops one recorded fingerprint so a failed tool call can be retried.
func (s *AgentState) Forget(d *Decision) {
	if s == nil || d == nil || len(s.executed) == 0 {
		return
	}
	fp := d.fingerprint()
	for i := len(s.executed) - 1; i >= 0; i-- {
		if s.executed[i] != fp {
			continue
		}
		s.executed = append(s.executed[:i], s.executed[i+1:]...)
		return
	}
}

func (s *AgentState) remember(d *Decision) {
	if s == nil || d == nil {
		return
	}
	s.executed = append(s.executed, d.fingerprint())
}

// ConsecutiveActionRepeats reports whether the last n applied decisions
// share the same type, tool, and arguments.
func (s *AgentState) ConsecutiveActionRepeats(n int) bool {
	if s == nil || n < 1 || len(s.executed) < n {
		return false
	}
	last := s.executed[len(s.executed)-1]
	for i := 1; i < n; i++ {
		if s.executed[len(s.executed)-1-i] != last {
			return false
		}
	}
	return true
}

// ConsecutiveToolRepeats reports whether the last n tool calls share the same
// tool name and arguments.
func (s *AgentState) ConsecutiveToolRepeats(n int) bool {
	if s == nil || n < 1 || len(s.ToolCalls) < n {
		return false
	}
	last := toolFingerprint(s.ToolCalls[len(s.ToolCalls)-1])
	for i := 1; i < n; i++ {
		if toolFingerprint(s.ToolCalls[len(s.ToolCalls)-1-i]) != last {
			return false
		}
	}
	return true
}

// RecordReflection stores a Reflection Engine result on working memory.
func (s *AgentState) RecordReflection(v ReflectionView) {
	if s == nil {
		return
	}
	s.Reflections = append(s.Reflections, v)
	s.Confidence = v.Confidence
}

// ClearActionHistory drops duplicate-action fingerprints so a REPLAN can re-execute
// the same DELEGATE_AGENT that the previous plan already applied.
func (s *AgentState) ClearActionHistory() {
	if s == nil {
		return
	}
	s.executed = nil
}

// DelegateTarget reads DELEGATE_AGENT arguments {"agent","task"} with objective fallback.
func DelegateTarget(d *Decision) (agent, task string) {
	if d == nil {
		return "", ""
	}
	if d.Arguments != nil {
		agent = argString(d.Arguments["agent"])
		task = argString(d.Arguments["task"])
	}
	if agent == "" {
		agent = strings.TrimSpace(d.Tool)
	}
	if task == "" {
		task = strings.TrimSpace(d.Objective)
	}
	return strings.ToLower(agent), task
}

func argString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	s := strings.Trim(string(raw), `"`)
	if s == "null" {
		return ""
	}
	return strings.TrimSpace(s)
}

func toolFingerprint(call ToolCallRecord) string {
	d := &Decision{Type: DecisionCallTool, Tool: call.Tool, Arguments: call.Arguments}
	return d.fingerprint()
}

func (s *AgentState) hasAction(d *Decision) bool {
	if s == nil || d == nil {
		return false
	}
	fp := d.fingerprint()
	for _, existing := range s.executed {
		if existing == fp {
			return true
		}
	}
	for _, call := range s.ToolCalls {
		if !call.Success {
			continue
		}
		prev := &Decision{Type: DecisionCallTool, Tool: call.Tool, Arguments: call.Arguments}
		if prev.fingerprint() == fp {
			return true
		}
	}
	return false
}

// TerminalAnswer is the user-facing text for a terminal decision.
func TerminalAnswer(d *Decision) string {
	if d == nil {
		return ""
	}
	if obj := strings.TrimSpace(d.Objective); obj != "" {
		return obj
	}
	return strings.TrimSpace(d.Reason)
}
