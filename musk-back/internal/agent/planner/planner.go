package planner

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
	planToolName = "submit_plan"
	planToolDesc = "Submit a structured task graph for the user goal. Output only the plan fields. Do not include chain-of-thought."
)

// llmTask is the structured task the model must emit. Status is assigned by the scheduler.
type llmTask struct {
	ID             string   `json:"id" jsonschema:"required" jsonschema_description:"Stable task id used by dependencies"`
	Title          string   `json:"title" jsonschema:"required" jsonschema_description:"Short task title"`
	Description    string   `json:"description,omitempty" jsonschema_description:"What this task does"`
	Dependencies   []string `json:"dependencies,omitempty" jsonschema_description:"Task ids that must complete first"`
	Priority       int      `json:"priority,omitempty" jsonschema_description:"Higher values run first among ready tasks"`
	ExpectedOutput string   `json:"expected_output" jsonschema:"required" jsonschema_description:"Concrete, checkable output of this task"`
}

// llmPlan is the structured planner output constrained by JSON Schema.
type llmPlan struct {
	Goal  string    `json:"goal" jsonschema:"required" jsonschema_description:"User goal being decomposed"`
	Tasks []llmTask `json:"tasks" jsonschema:"required" jsonschema_description:"DAG nodes. Independent tasks may run in parallel."`
}

// Planner decomposes a goal into a validated task DAG.
type Planner struct {
	model       einomodel.BaseChatModel
	logger      logger.Logger
	instruction string
	maxTasks    int
	planTool    *schema.ToolInfo
	schemaJSON  string
}

// New builds a Planner that forces structured Plan output.
func New(model einomodel.BaseChatModel, instruction string, maxTasks int, log logger.Logger) (*Planner, error) {
	if model == nil {
		return nil, errors.New("chat model is required")
	}
	if log == nil {
		return nil, errors.New("logger is required")
	}
	if maxTasks < 1 {
		maxTasks = defaultMaxTasks
	}

	toolInfo, err := utils.GoStruct2ToolInfo[llmPlan](planToolName, planToolDesc)
	if err != nil {
		return nil, fmt.Errorf("build plan json schema: %w", err)
	}

	schemaJSON := "{}"
	if toolInfo.ParamsOneOf != nil {
		js, err := toolInfo.ParamsOneOf.ToJSONSchema()
		if err != nil {
			return nil, fmt.Errorf("plan json schema: %w", err)
		}
		raw, err := json.Marshal(js)
		if err != nil {
			return nil, fmt.Errorf("marshal plan json schema: %w", err)
		}
		schemaJSON = string(raw)
	}

	return &Planner{
		model:       model,
		logger:      log,
		instruction: instruction,
		maxTasks:    maxTasks,
		planTool:    toolInfo,
		schemaJSON:  schemaJSON,
	}, nil
}

// MaxTasks returns the configured MAX_TASKS cap.
func (p *Planner) MaxTasks() int {
	if p == nil || p.maxTasks < 1 {
		return defaultMaxTasks
	}
	return p.maxTasks
}

// Plan decomposes goal into a DAG using the current agent state as context.
func (p *Planner) Plan(ctx context.Context, goal string, state decision.AgentState) (*Plan, error) {
	if p == nil {
		return nil, errors.New("planner is required")
	}

	goal = strings.TrimSpace(goal)
	if goal == "" {
		goal = strings.TrimSpace(state.Goal)
	}
	if goal == "" {
		return nil, ErrEmptyGoal
	}

	raw, err := p.generate(ctx, goal, state)
	if err != nil {
		return nil, err
	}

	plan, err := decodePlan(raw)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(plan.Goal) == "" {
		plan.Goal = goal
	}
	if err := ValidatePlan(plan, p.MaxTasks()); err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(plan.Tasks))
	for _, t := range plan.Tasks {
		ids = append(ids, t.ID)
	}
	p.logger.InfoContext(ctx, "agent plan created",
		"request_id", requestid.From(ctx),
		"goal_len", len(plan.Goal),
		"task_count", len(plan.Tasks),
		"task_ids", strings.Join(ids, ","),
		"max_tasks", p.MaxTasks(),
	)
	return plan, nil
}

func (p *Planner) generate(ctx context.Context, goal string, state decision.AgentState) ([]byte, error) {
	msgs := []*schema.Message{
		schema.SystemMessage(p.systemPrompt()),
		schema.UserMessage(p.statePrompt(goal, state)),
	}

	cm := p.model
	opts := []einomodel.Option{}
	if tm, ok := p.model.(einomodel.ToolCallingChatModel); ok && p.planTool != nil {
		bound, err := tm.WithTools([]*schema.ToolInfo{p.planTool})
		if err != nil {
			return nil, fmt.Errorf("bind plan schema: %w", err)
		}
		cm = bound
		opts = append(opts, einomodel.WithToolChoice(schema.ToolChoiceForced, planToolName))
	}

	msg, err := cm.Generate(ctx, msgs, opts...)
	if err != nil {
		return nil, fmt.Errorf("generate plan: %w", err)
	}
	return extractPlanPayload(msg)
}

func (p *Planner) systemPrompt() string {
	var b strings.Builder
	if inst := strings.TrimSpace(p.instruction); inst != "" {
		b.WriteString(inst)
		b.WriteString("\n\n")
	}
	b.WriteString("You are an autonomous planner.\n")
	b.WriteString("Decompose the user goal into a directed acyclic task graph (DAG).\n")
	b.WriteString("Each task must have a clear, concrete expected_output.\n")
	b.WriteString("Identify dependencies by task id. Tasks with no mutual dependencies may run in parallel.\n")
	b.WriteString(fmt.Sprintf("Do not split excessively. Return at most %d tasks. A simple goal is exactly one task.\n", p.MaxTasks()))
	b.WriteString("Reply in English only.\n")
	b.WriteString("If the goal is to create or update a BI dashboard, return two or three ordered tasks: query the ads rows with bi_dashboard, decide which metrics and slices those numbers support, then create the board from that observation. The last task's expected_output is the dashboard url. Do not collapse this into a single create, and do not plan placeholder metrics.\n")
	b.WriteString("Output a single structured Plan that matches the JSON Schema.\n")
	b.WriteString("Do not output chain-of-thought, hidden reasoning, or step-by-step scratch work.\n")
	b.WriteString("JSON Schema:\n")
	b.WriteString(p.schemaJSON)
	return b.String()
}

func (p *Planner) statePrompt(goal string, state decision.AgentState) string {
	obs := make([]map[string]any, 0, len(state.Observations))
	for _, o := range state.Observations {
		obs = append(obs, map[string]any{
			"summary":    o.Summary,
			"reason":     o.Reason,
			"evidence":   o.Evidence,
			"confidence": o.Confidence,
		})
	}
	view := map[string]any{
		"goal":              goal,
		"current_objective": state.CurrentObjective,
		"completed_tasks":   state.CompletedTasks,
		"failed_tasks":      state.FailedTasks,
		"observations":      obs,
		"current_step":      state.CurrentStep,
		"max_steps":         state.MaxSteps,
		"max_tasks":         p.MaxTasks(),
	}
	raw, err := json.Marshal(view)
	if err != nil {
		return "Decompose this goal into a structured task graph. Return only the Plan.\n" + goal
	}
	return "Decompose this goal into a structured task graph. Return only the Plan.\n" + string(raw)
}

func extractPlanPayload(msg *schema.Message) ([]byte, error) {
	if msg == nil {
		return nil, fmt.Errorf("%w: empty model response", ErrInvalidPlan)
	}
	for _, call := range msg.ToolCalls {
		if strings.TrimSpace(call.Function.Arguments) == "" {
			continue
		}
		return []byte(strings.TrimSpace(call.Function.Arguments)), nil
	}
	payload := extractJSON(msg.Content)
	if payload == "" {
		return nil, fmt.Errorf("%w: model did not return structured plan json", ErrInvalidPlan)
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

func decodePlan(payload []byte) (*Plan, error) {
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPlan, err)
	}
	delete(raw, "chain_of_thought")
	delete(raw, "chainOfThought")
	delete(raw, "thinking")
	delete(raw, "reasoning")
	delete(raw, "reasoning_content")
	delete(raw, "cot")

	normalized, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPlan, err)
	}

	var parsed llmPlan
	if err := json.Unmarshal(normalized, &parsed); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPlan, err)
	}

	plan := &Plan{
		Goal:  strings.TrimSpace(parsed.Goal),
		Tasks: make([]Task, 0, len(parsed.Tasks)),
	}
	for _, t := range parsed.Tasks {
		plan.Tasks = append(plan.Tasks, Task{
			ID:             t.ID,
			Title:          t.Title,
			Description:    t.Description,
			Dependencies:   t.Dependencies,
			Priority:       t.Priority,
			Status:         StatusPending,
			ExpectedOutput: t.ExpectedOutput,
		})
	}
	return plan, nil
}
