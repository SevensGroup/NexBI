package multi

import (
	"context"
	"fmt"
	"strings"

	"autonomous-agent/internal/agent/decision"
	"autonomous-agent/internal/tools"
)

// Delegator is the Coordinator-facing multi-agent API.
type Delegator interface {
	Delegate(ctx context.Context, req DelegateRequest) (*AgentOutput, error)
	Listings() []decision.AgentListing
}

// Team is Coordinator → Analyst / Researcher / Reviewer, wired as Agent-as-Tool.
type Team struct {
	specs map[string]Spec
	tools map[string]*AgentTool
}

var _ Delegator = (*Team)(nil)

// NewTeam registers each spec as an Agent-as-Tool. spawn builds the specialist runner.
func NewTeam(specs []Spec, spawn SpawnFunc) *Team {
	t := &Team{
		specs: map[string]Spec{},
		tools: map[string]*AgentTool{},
	}
	for _, spec := range specs {
		name := NormalizeRole(spec.Name)
		if name == "" || name == string(RoleCoordinator) {
			continue
		}
		spec.Name = name
		spec.Tools = cloneStrings(spec.Tools)
		t.specs[name] = spec
		t.tools[name] = NewAgentTool(spec, spawn)
	}
	return t
}

// Spec returns a copy of the named specialist spec.
func (t *Team) Spec(name string) (Spec, bool) {
	if t == nil {
		return Spec{}, false
	}
	spec, ok := t.specs[NormalizeRole(name)]
	if !ok {
		return Spec{}, false
	}
	spec.Tools = cloneStrings(spec.Tools)
	return spec, true
}

// Tool returns the Agent-as-Tool for name.
func (t *Team) Tool(name string) (*AgentTool, bool) {
	if t == nil {
		return nil, false
	}
	tool, ok := t.tools[NormalizeRole(name)]
	return tool, ok
}

// Listings is the coordinator catalog (independent I/O schemas).
func (t *Team) Listings() []decision.AgentListing {
	if t == nil {
		return nil
	}
	order := []string{string(RoleAnalyst), string(RoleResearcher), string(RoleReviewer)}
	out := make([]decision.AgentListing, 0, len(t.specs))
	seen := map[string]struct{}{}
	for _, name := range order {
		if spec, ok := t.specs[name]; ok {
			out = append(out, spec.Listing())
			seen[name] = struct{}{}
		}
	}
	for name, spec := range t.specs {
		if _, ok := seen[name]; ok {
			continue
		}
		out = append(out, spec.Listing())
	}
	return out
}

// Delegate runs the named specialist through Agent-as-Tool.
func (t *Team) Delegate(ctx context.Context, req DelegateRequest) (*AgentOutput, error) {
	if t == nil {
		return nil, ErrUnknownAgent
	}
	req.Agent = NormalizeRole(req.Agent)
	req.Task = strings.TrimSpace(req.Task)
	if req.Agent == "" || req.Task == "" {
		return nil, fmt.Errorf("%w: agent and task are required", ErrInvalidDelegate)
	}
	if !KnownRole(req.Agent) {
		return nil, fmt.Errorf("%w: %s", ErrUnknownAgent, req.Agent)
	}
	tool, ok := t.tools[req.Agent]
	if !ok || tool == nil {
		return nil, fmt.Errorf("%w: %s", ErrUnknownAgent, req.Agent)
	}

	result, err := tool.Execute(ctx, req.input())
	if err != nil && result.Data == nil {
		return nil, err
	}
	out := outputFromResult(req, result)
	if out == nil {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%w: empty specialist result", ErrInvalidDelegate)
	}
	if result.Success {
		return out, nil
	}
	return out, err
}

func outputFromResult(req DelegateRequest, result tools.ToolResult) *AgentOutput {
	if out, ok := result.Data.(*AgentOutput); ok && out != nil {
		if out.Agent == "" {
			out.Agent = req.Agent
		}
		if out.Task == "" {
			out.Task = req.Task
		}
		return out
	}
	if out, ok := result.Data.(AgentOutput); ok {
		if out.Agent == "" {
			out.Agent = req.Agent
		}
		if out.Task == "" {
			out.Task = req.Task
		}
		return &out
	}
	if result.Error != "" {
		out := ParseAgentOutput(req.Agent, req.Task, result.Error)
		if Role(req.Agent) == RoleReviewer && out.Review == nil {
			out.Review = &ReviewResult{Approved: false, Reason: "insufficient evidence"}
		}
		return out
	}
	if result.Data != nil {
		return ParseAgentOutput(req.Agent, req.Task, fmt.Sprint(result.Data))
	}
	return nil
}
