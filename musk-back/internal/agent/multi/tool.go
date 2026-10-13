package multi

import (
	"context"
	"strings"

	"autonomous-agent/internal/requestid"
	"autonomous-agent/internal/tools"
)

// Runner is the specialist execution contract. Runtime implements this with a
// scoped child loop so Agent-as-Tool does not bypass Decision / Plan / Memory.
type Runner interface {
	Run(ctx context.Context, message string) (string, error)
}

// SpawnFunc constructs a specialist runner for spec (independent prompt, tools, memory).
type SpawnFunc func(spec Spec) (Runner, error)

// AgentTool wraps a specialist as a Tool — the project's Agent-as-Tool adapter.
// Specialists keep the Phase 2–8 loop instead of eino/adk.NewAgentTool, which
// would replace Decision, Reflection, and Memory with a generic ChatModelAgent.
type AgentTool struct {
	spec  Spec
	spawn SpawnFunc
}

var _ tools.Tool = (*AgentTool)(nil)

// NewAgentTool builds an Agent-as-Tool for spec.
func NewAgentTool(spec Spec, spawn SpawnFunc) *AgentTool {
	spec.Name = NormalizeRole(spec.Name)
	if spec.InputSchema == nil {
		spec.InputSchema = specialistInputSchema()
	}
	if spec.OutputSchema == nil {
		spec.OutputSchema = specialistOutputSchema()
	}
	return &AgentTool{spec: spec, spawn: spawn}
}

func (t *AgentTool) Name() string {
	if t == nil {
		return ""
	}
	return t.spec.Name
}

func (t *AgentTool) Description() string {
	if t == nil {
		return ""
	}
	desc := strings.TrimSpace(t.spec.Description)
	if desc == "" {
		desc = "Specialist agent"
	}
	return desc + ". Invoke via DELEGATE_AGENT with {\"agent\":\"" + t.spec.Name + "\",\"task\":\"...\"}."
}

func (t *AgentTool) InputSchema() any {
	if t == nil || t.spec.InputSchema == nil {
		return specialistInputSchema()
	}
	return t.spec.InputSchema
}

func (t *AgentTool) Execute(ctx context.Context, input map[string]any) (tools.ToolResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if t == nil {
		return delegateFail(ctx, "", "agent tool is nil", ErrNoRunner)
	}
	if err := ctx.Err(); err != nil {
		return delegateFail(ctx, t.spec.Name, err.Error(), err)
	}
	if t.spawn == nil {
		return delegateFail(ctx, t.spec.Name, "agent runner is not configured", ErrNoRunner)
	}
	if input == nil {
		input = map[string]any{}
	}

	task := stringArg(input, "task")
	if task == "" {
		return delegateFail(ctx, t.spec.Name, "task is required", ErrInvalidDelegate)
	}

	runner, err := t.spawn(t.spec)
	if err != nil {
		return delegateFail(ctx, t.spec.Name, err.Error(), err)
	}
	if runner == nil {
		return delegateFail(ctx, t.spec.Name, "agent runner is not configured", ErrNoRunner)
	}

	answer, err := runner.Run(ctx, formatTaskMessage(t.spec, input))
	if err != nil {
		return delegateFail(ctx, t.spec.Name, err.Error(), err)
	}

	out := ParseAgentOutput(t.spec.Name, task, answer)
	return tools.ToolResult{
		Success: true,
		Data:    out,
		Metadata: map[string]any{
			"tool":     t.spec.Name,
			"agent":    t.spec.Name,
			"trace_id": requestid.From(ctx),
			"task":     task,
			"rejected": out.Rejected(),
			"review":   out.Review != nil,
			"risk":     string(tools.RiskMedium),
		},
	}, nil
}

func delegateFail(ctx context.Context, agent, msg string, sentinel error) (tools.ToolResult, error) {
	if sentinel == nil {
		sentinel = ErrNoRunner
	}
	te := &tools.ToolError{
		Code:    tools.CodeExecutionFailed,
		Tool:    agent,
		Message: msg,
		TraceID: requestid.From(ctx),
		Err:     sentinel,
	}
	return tools.ToolResult{
		Success: false,
		Error:   te.Error(),
		Metadata: map[string]any{
			"tool":       agent,
			"agent":      agent,
			"trace_id":   te.TraceID,
			"error_code": te.Code,
		},
	}, te
}
