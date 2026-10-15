package decision

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"

	"autonomous-agent/internal/logger"
	"autonomous-agent/internal/requestid"
)

const (
	decisionToolName = "submit_decision"
	decisionToolDesc = "Submit the next autonomous agent decision. Output only the structured fields. Do not include chain-of-thought or any tool result."

	// maxDecideAttempts is the first generate plus a bounded re-ask when the
	// model returns an invalid structured Decision (unmarshal or validation).
	maxDecideAttempts = 2
)

// Engine is the autonomous decision engine: Understand → Decide → (caller Acts / Observes / Evaluates).
type Engine struct {
	model        einomodel.BaseChatModel
	validator    *Validator
	policy       *Policy
	logger       logger.Logger
	instruction  string
	decisionTool *schema.ToolInfo
	schemaJSON   string
}

// NewEngine builds a DecisionEngine that forces structured Decision output.
func NewEngine(model einomodel.BaseChatModel, instruction string, log logger.Logger) (*Engine, error) {
	if model == nil {
		return nil, errors.New("chat model is required")
	}
	if log == nil {
		return nil, errors.New("logger is required")
	}

	toolInfo, err := utils.GoStruct2ToolInfo[Decision](decisionToolName, decisionToolDesc)
	if err != nil {
		return nil, fmt.Errorf("build decision json schema: %w", err)
	}

	schemaJSON := "{}"
	if toolInfo.ParamsOneOf != nil {
		js, err := toolInfo.ParamsOneOf.ToJSONSchema()
		if err != nil {
			return nil, fmt.Errorf("decision json schema: %w", err)
		}
		raw, err := json.Marshal(js)
		if err != nil {
			return nil, fmt.Errorf("marshal decision json schema: %w", err)
		}
		schemaJSON = string(raw)
	}

	return &Engine{
		model:        model,
		validator:    NewValidator(),
		policy:       NewPolicy(),
		logger:       log,
		instruction:  instruction,
		decisionTool: toolInfo,
		schemaJSON:   schemaJSON,
	}, nil
}

// Decide produces the next structured Decision for state.
func (e *Engine) Decide(ctx context.Context, state *AgentState) (*Decision, error) {
	if state == nil {
		return nil, ErrEmptyState
	}
	Understand(state)

	if e.policy.MaxStepsReached(state) {
		d := abortDecision("maximum steps reached")
		e.logDecision(ctx, state, d)
		return d, nil
	}

	var lastErr error
	for attempt := 1; attempt <= maxDecideAttempts; attempt++ {
		raw, err := e.generate(ctx, state, lastErr)
		if err != nil {
			if retryInvalidDecision(err, attempt) {
				e.logInvalidDecisionRetry(ctx, state, attempt, err)
				lastErr = err
				continue
			}
			return nil, err
		}

		d, err := decodeDecision(raw)
		if err != nil {
			if retryInvalidDecision(err, attempt) {
				e.logInvalidDecisionRetry(ctx, state, attempt, err)
				lastErr = err
				continue
			}
			return nil, err
		}
		if err := e.validator.Validate(d); err != nil {
			if retryInvalidDecision(err, attempt) {
				e.logInvalidDecisionRetry(ctx, state, attempt, err)
				lastErr = err
				continue
			}
			return nil, err
		}
		if err := e.policy.CheckDuplicate(state, d); err != nil {
			return nil, err
		}

		e.logDecision(ctx, state, d)
		return d, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("%w: empty decision", ErrInvalidDecision)
}

func retryInvalidDecision(err error, attempt int) bool {
	return attempt < maxDecideAttempts && errors.Is(err, ErrInvalidDecision)
}

// Apply records a validated decision into state and returns CONTINUE / FINISH / ASK_USER / ABORT.
func (e *Engine) Apply(state *AgentState, d *Decision) Outcome {
	return e.policy.Apply(state, d)
}

func (e *Engine) generate(ctx context.Context, state *AgentState, previousErr error) ([]byte, error) {
	msgs := []*schema.Message{
		schema.SystemMessage(e.systemPrompt()),
		schema.UserMessage(e.statePrompt(state, previousErr)),
	}

	cm := e.model
	opts := []einomodel.Option{}
	if tm, ok := e.model.(einomodel.ToolCallingChatModel); ok && e.decisionTool != nil {
		bound, err := tm.WithTools([]*schema.ToolInfo{e.decisionTool})
		if err != nil {
			return nil, fmt.Errorf("bind decision schema: %w", err)
		}
		cm = bound
		opts = append(opts, einomodel.WithToolChoice(schema.ToolChoiceForced, decisionToolName))
	}

	msg, err := cm.Generate(ctx, msgs, opts...)
	if err != nil {
		return nil, fmt.Errorf("generate decision: %w", err)
	}
	return extractDecisionPayload(msg)
}

func (e *Engine) systemPrompt() string {
	var b strings.Builder
	if inst := strings.TrimSpace(e.instruction); inst != "" {
		b.WriteString(inst)
		b.WriteString("\n\n")
	}
	b.WriteString("You are an autonomous decision engine.\n")
	b.WriteString("Loop: Goal → Understand → Decide → Act → Observe → Reflection → Decide / Finish.\n")
	b.WriteString("After a tool observation, a Reflection Engine judges whether the goal is met, evidence is enough, and what to do next (CONTINUE, REPLAN, FINISH, ASK_USER). Use the latest reflection when deciding.\n")
	b.WriteString("Output a single structured Decision that matches the JSON Schema.\n")
	b.WriteString("Default reply language is Chinese. Write thought, reason, objective, and any dashboard title or summary in Chinese. Copy table column names and cell values exactly from the query; those stored fields stay English. Do not translate campaign names, channel names, or column keys.\n")
	b.WriteString("thought is 2 to 4 first-person sentences about what you noticed and why the next action follows. reason is one short Chinese line. Do not add a hidden scratchpad outside thought, and do not put a tool result in thought.\n")
	b.WriteString("Only fill type, thought, reason, objective, tool, arguments, expected_result, confidence, and risk.\n")
	b.WriteString("Never fabricate a tool result. CALL_TOOL means a tool should be executed later; expected_result is a prediction, not an observation.\n")
	b.WriteString("CALL_TOOL only for tools listed in available_tools. arguments must be a JSON object (for example {\"nl\":\"...\"}), never a stringified JSON string, and may be omitted or null when unused.\n")
	b.WriteString("If available_agents is non-empty, you are the Coordinator. Assign work with DELEGATE_AGENT and arguments {\"agent\":\"analyst|researcher|reviewer\",\"task\":\"...\"}. Example: {\"agent\":\"analyst\",\"task\":\"Analyze why Meta ROAS declined yesterday\"}. Do not fabricate specialist results.\n")
	b.WriteString("After Analyst or Researcher returns, DELEGATE_AGENT to reviewer to verify conclusions and evidence. If the reviewer returns {\"approved\":false,\"reason\":\"insufficient evidence\"} (or any approved=false), do not FINISH that conclusion; the runtime will REPLAN and you must re-execute.\n")
	b.WriteString("If available_agents is empty, never use DELEGATE_AGENT.\n")
	b.WriteString("Use search_memory to recall user preferences, business knowledge, historical insights, and query patterns. Use save_memory to persist them. Never store API keys or credentials.\n")
	b.WriteString("When the user asks to build, create, or update a BI dashboard or 看板, do not create it on the first step and do not delegate it away. First CALL_TOOL bi_dashboard with action query and read the ads totals and campaign rows. After that observation, decide yourself which KPIs, channels, and campaigns the numbers support, and what conclusion they justify. Then CALL_TOOL bi_dashboard with action create. Copy metric values and table rows from the query observation, keeping English column keys and English campaign values. summary is a Chinese conclusion naming a channel or campaign and what stands out. Never send placeholder values such as \"-\" or an empty rows array. Do not invent a dashboard id; the tool returns the url.\n")
	b.WriteString("JSON Schema:\n")
	b.WriteString(e.schemaJSON)
	return b.String()
}

func (e *Engine) statePrompt(state *AgentState, previousErr error) string {
	view := map[string]any{
		"goal":              state.Goal,
		"current_objective": state.CurrentObjective,
		"plan":              state.Plan,
		"completed_tasks":   state.CompletedTasks,
		"failed_tasks":      state.FailedTasks,
		"observations":      state.Observations,
		"tool_calls":        state.ToolCalls,
		"available_tools":   state.AvailableTools,
		"available_agents":  state.AvailableAgents,
		"delegations":       state.Delegations,
		"memories":          state.Memories,
		"reflections":       state.Reflections,
		"run_id":            state.RunID,
		"current_step":      state.CurrentStep,
		"max_steps":         state.MaxSteps,
		"confidence":        state.Confidence,
		"finished":          state.Finished,
	}
	raw, err := json.Marshal(view)
	if err != nil {
		return state.Goal
	}
	prompt := "Decide the next action for this agent state. Return only the structured Decision.\n" + string(raw)
	if previousErr != nil {
		prompt += "\n\nPrevious structured Decision was invalid: " + previousErr.Error() +
			"\nFix the JSON. arguments must be a JSON object, not a string. Example: {\"nl\":\"...\"}."
	}
	return prompt
}

func (e *Engine) logInvalidDecisionRetry(ctx context.Context, state *AgentState, attempt int, err error) {
	step := 0
	if state != nil {
		step = state.CurrentStep
	}
	e.logger.WarnContext(ctx, "invalid decision, retrying",
		"request_id", requestid.From(ctx),
		"step", step,
		"attempt", attempt,
		"error", err,
	)
}

func (e *Engine) logDecision(ctx context.Context, state *AgentState, d *Decision) {
	e.logger.InfoContext(ctx, "agent decision",
		"request_id", requestid.From(ctx),
		"step", state.CurrentStep,
		"type", d.Type,
		"summary", d.summary(),
		"reason", d.Reason,
		"evidence", d.evidence(),
		"confidence", d.Confidence,
		"risk", d.Risk,
	)
}

func extractDecisionPayload(msg *schema.Message) ([]byte, error) {
	if msg == nil {
		return nil, fmt.Errorf("%w: empty model response", ErrInvalidDecision)
	}

	for _, call := range msg.ToolCalls {
		if strings.TrimSpace(call.Function.Arguments) == "" {
			continue
		}
		return []byte(strings.TrimSpace(call.Function.Arguments)), nil
	}

	payload := extractJSON(msg.Content)
	if payload == "" {
		return nil, fmt.Errorf("%w: model did not return structured decision json", ErrInvalidDecision)
	}
	return []byte(payload), nil
}

func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```JSON")
		s = strings.TrimPrefix(s, "```")
		if i := strings.LastIndex(s, "```"); i >= 0 {
			s = s[:i]
		}
		s = strings.TrimSpace(s)
	}
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return ""
}
