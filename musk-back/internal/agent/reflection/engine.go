package reflection

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"

	"autonomous-agent/internal/agent/decision"
	"autonomous-agent/internal/logger"
	"autonomous-agent/internal/requestid"
)

const (
	reflectionToolName = "submit_reflection"
	reflectionToolDesc = "Submit the structured reflection after a tool observation. Output only the structured fields. Do not include chain-of-thought."
)

// Engine judges whether a tool observation is enough to continue, replan, finish, or ask the user.
type Engine struct {
	model          einomodel.BaseChatModel
	validator      *Validator
	logger         logger.Logger
	instruction    string
	reflectionTool *schema.ToolInfo
	schemaJSON     string
}

var _ Reflector = (*Engine)(nil)

// NewEngine builds a ReflectionEngine that forces structured Reflection output.
func NewEngine(model einomodel.BaseChatModel, instruction string, log logger.Logger) (*Engine, error) {
	if log == nil {
		return nil, errors.New("logger is required")
	}

	eng := &Engine{
		model:       model,
		validator:   NewValidator(),
		logger:      log,
		instruction: instruction,
		schemaJSON:  "{}",
	}

	if model == nil {
		return eng, nil
	}

	toolInfo, err := utils.GoStruct2ToolInfo[Reflection](reflectionToolName, reflectionToolDesc)
	if err != nil {
		return nil, fmt.Errorf("build reflection json schema: %w", err)
	}
	eng.reflectionTool = toolInfo
	if toolInfo.ParamsOneOf != nil {
		js, err := toolInfo.ParamsOneOf.ToJSONSchema()
		if err != nil {
			return nil, fmt.Errorf("reflection json schema: %w", err)
		}
		raw, err := json.Marshal(js)
		if err != nil {
			return nil, fmt.Errorf("marshal reflection json schema: %w", err)
		}
		eng.schemaJSON = string(raw)
	}
	return eng, nil
}

// NewHeuristic builds a rule-only ReflectionEngine (no LLM).
func NewHeuristic(log logger.Logger) *Engine {
	if log == nil {
		log = logger.New("info", "json", nil)
	}
	eng, _ := NewEngine(nil, "", log)
	return eng
}

// Reflect answers: is the goal done, is evidence enough, is data complete,
// are there anomalies, should we continue, and what is the next best action.
func (e *Engine) Reflect(ctx context.Context, state *decision.AgentState, observation decision.Observation) (*Reflection, error) {
	if e == nil {
		return Analyze(state, observation), nil
	}

	base := Analyze(state, observation)
	if DataBackendUnavailable(state, observation) {
		e.logReflection(ctx, state, base)
		return base, nil
	}
	if LoopDetected(state) {
		e.logReflection(ctx, state, base)
		return base, nil
	}

	if e.model == nil {
		e.logReflection(ctx, state, base)
		return base, nil
	}

	raw, err := e.generate(ctx, state, observation)
	if err != nil {
		e.logger.WarnContext(ctx, "agent reflection llm failed, using heuristic",
			"request_id", requestid.From(ctx),
			"error", err,
		)
		e.logReflection(ctx, state, base)
		return base, nil
	}

	llm, err := decodeReflection(raw)
	if err != nil {
		e.logger.WarnContext(ctx, "agent reflection decode failed, using heuristic",
			"request_id", requestid.From(ctx),
			"error", err,
		)
		e.logReflection(ctx, state, base)
		return base, nil
	}
	if err := e.validator.Validate(llm); err != nil {
		e.logger.WarnContext(ctx, "agent reflection invalid, using heuristic",
			"request_id", requestid.From(ctx),
			"error", err,
		)
		e.logReflection(ctx, state, base)
		return base, nil
	}

	out := merge(base, llm, state)
	if err := e.validator.Validate(out); err != nil {
		e.logReflection(ctx, state, base)
		return base, nil
	}
	e.logReflection(ctx, state, out)
	return out, nil
}

func merge(base, llm *Reflection, state *decision.AgentState) *Reflection {
	if llm == nil {
		return normalize(base)
	}
	out := *llm
	normalize(&out)
	if base != nil {
		out.Errors = appendUnique(out.Errors, base.Errors...)
		out.MissingInformation = appendUnique(out.MissingInformation, base.MissingInformation...)
		if !base.ResultCorrect {
			out.ResultCorrect = false
		}
		if !base.EvidenceSufficient {
			out.EvidenceSufficient = false
		}
		if len(base.Errors) > 0 && out.NextAction == NextFinish {
			out.NextAction = NextContinue
			out.GoalSatisfied = false
		}
		if base.NextAction == NextAskUser {
			out.NextAction = NextAskUser
			out.GoalSatisfied = false
		}
	}
	if DataBackendUnavailable(state, decision.Observation{}) {
		out.NextAction = NextAskUser
		out.GoalSatisfied = false
		out.EvidenceSufficient = false
	}
	if LoopDetected(state) && out.NextAction != NextAskUser {
		out.NextAction = NextReplan
		out.GoalSatisfied = false
		out.Errors = appendUnique(out.Errors, "same decision, tool, and arguments repeated three times")
	}
	return &out
}

func (e *Engine) generate(ctx context.Context, state *decision.AgentState, observation decision.Observation) ([]byte, error) {
	msgs := []*schema.Message{
		schema.SystemMessage(e.systemPrompt()),
		schema.UserMessage(e.statePrompt(state, observation)),
	}

	cm := e.model
	opts := []einomodel.Option{}
	if tm, ok := e.model.(einomodel.ToolCallingChatModel); ok && e.reflectionTool != nil {
		bound, err := tm.WithTools([]*schema.ToolInfo{e.reflectionTool})
		if err != nil {
			return nil, fmt.Errorf("bind reflection schema: %w", err)
		}
		cm = bound
		opts = append(opts, einomodel.WithToolChoice(schema.ToolChoiceForced, reflectionToolName))
	}

	msg, err := cm.Generate(ctx, msgs, opts...)
	if err != nil {
		return nil, fmt.Errorf("generate reflection: %w", err)
	}
	return extractReflectionPayload(msg)
}

func (e *Engine) systemPrompt() string {
	var b strings.Builder
	if inst := strings.TrimSpace(e.instruction); inst != "" {
		b.WriteString(inst)
		b.WriteString("\n\n")
	}
	b.WriteString("You are a reflection engine for an autonomous agent.\n")
	b.WriteString("After a tool observation, judge whether the result is enough. Do not fabricate tool results.\n")
	b.WriteString("Answer: (1) is the goal complete, (2) is evidence sufficient, (3) is the data complete and correct,\n")
	b.WriteString("(4) are there anomalies or errors, (5) should the loop continue, (6) what is the next best action.\n")
	b.WriteString("next_action must be one of CONTINUE, REPLAN, FINISH, ASK_USER.\n")
	b.WriteString("CONTINUE if more work is needed. REPLAN if the current plan is stuck or wrong.\n")
	b.WriteString("FINISH only when the goal is satisfied and evidence is sufficient. ASK_USER when a required data backend is unavailable (for example Elasticsearch MCP is not registered) or only the user can supply missing information. Do not REPLAN to retry a tool that is not registered.\n")
	b.WriteString("Do not output chain-of-thought.\n")
	b.WriteString("JSON Schema:\n")
	b.WriteString(e.schemaJSON)
	return b.String()
}

func (e *Engine) statePrompt(state *decision.AgentState, observation decision.Observation) string {
	view := map[string]any{
		"observation": observation,
	}
	if state != nil {
		view["goal"] = state.Goal
		view["current_objective"] = state.CurrentObjective
		view["plan"] = state.Plan
		view["completed_tasks"] = state.CompletedTasks
		view["failed_tasks"] = state.FailedTasks
		view["observations"] = state.Observations
		view["tool_calls"] = state.ToolCalls
		view["current_step"] = state.CurrentStep
		view["max_steps"] = state.MaxSteps
	}
	raw, err := json.Marshal(view)
	if err != nil {
		return "Reflect on the latest observation. Return only the structured Reflection."
	}
	return "Reflect on this agent state and observation. Return only the structured Reflection.\n" + string(raw)
}

func (e *Engine) logReflection(ctx context.Context, state *decision.AgentState, r *Reflection) {
	if e == nil || e.logger == nil || r == nil {
		return
	}
	step := 0
	if state != nil {
		step = state.CurrentStep
	}
	e.logger.InfoContext(ctx, "agent reflection",
		"request_id", requestid.From(ctx),
		"step", step,
		"goal_satisfied", r.GoalSatisfied,
		"evidence_sufficient", r.EvidenceSufficient,
		"result_correct", r.ResultCorrect,
		"missing_information", r.MissingInformation,
		"errors", r.Errors,
		"next_action", r.NextAction,
		"confidence", r.Confidence,
	)
}

func extractReflectionPayload(msg *schema.Message) ([]byte, error) {
	if msg == nil {
		return nil, fmt.Errorf("%w: empty model response", ErrInvalidReflection)
	}

	for _, call := range msg.ToolCalls {
		if strings.TrimSpace(call.Function.Arguments) == "" {
			continue
		}
		return []byte(strings.TrimSpace(call.Function.Arguments)), nil
	}

	payload := extractJSON(msg.Content)
	if payload == "" {
		return nil, fmt.Errorf("%w: model did not return structured reflection json", ErrInvalidReflection)
	}
	return []byte(payload), nil
}
