package multi

import (
	"strings"

	"autonomous-agent/internal/agent/decision"
	"autonomous-agent/internal/analyst"
	"autonomous-agent/internal/memory"
	"autonomous-agent/internal/semantic"
	"autonomous-agent/internal/tools/elasticsearch"
)

// Role is a named specialist (or the coordinator) in the multi-agent team.
type Role string

const (
	RoleCoordinator Role = "coordinator"
	RoleAnalyst     Role = "analyst"
	RoleResearcher  Role = "researcher"
	RoleReviewer    Role = "reviewer"

	defaultSpecialistMaxSteps = 6
)

// Spec is one agent's identity: prompt, tool scope, memory scope, and I/O schemas.
type Spec struct {
	Name         string
	Description  string
	Instruction  string
	Tools        []string
	MaxSteps     int
	InputSchema  any
	OutputSchema any
}

// NormalizeRole lowercases and trims a role name.
func NormalizeRole(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// KnownRole reports whether name is analyst, researcher, or reviewer.
func KnownRole(name string) bool {
	switch Role(NormalizeRole(name)) {
	case RoleAnalyst, RoleResearcher, RoleReviewer:
		return true
	default:
		return false
	}
}

// DefaultSpecs returns Analyst, Researcher, and Reviewer.
// Coordinator is the parent runtime, not a delegated specialist.
func DefaultSpecs(maxSteps int) []Spec {
	if maxSteps < 1 {
		maxSteps = defaultSpecialistMaxSteps
	}
	if maxSteps > defaultSpecialistMaxSteps {
		maxSteps = defaultSpecialistMaxSteps
	}
	return []Spec{
		analystSpec(maxSteps),
		researcherSpec(maxSteps),
		reviewerSpec(maxSteps),
	}
}

func analystSpec(maxSteps int) Spec {
	return Spec{
		Name:        string(RoleAnalyst),
		Description: "data analysis, metric analysis, and anomaly detection",
		Instruction: analystInstruction,
		Tools: []string{
			"calculator",
			analyst.ToolName,
			semantic.ToolName,
			memory.ToolSearch,
			memory.ToolSave,
			elasticsearch.NameSearch,
			elasticsearch.NameGet,
			elasticsearch.NameCount,
			elasticsearch.NameAggregate,
		},
		MaxSteps:     maxSteps,
		InputSchema:  specialistInputSchema(),
		OutputSchema: analystOutputSchema(),
	}
}

func researcherSpec(maxSteps int) Spec {
	return Spec{
		Name:         string(RoleResearcher),
		Description:  "external search, industry context, and background material",
		Instruction:  researcherInstruction,
		Tools:        []string{memory.ToolSearch, memory.ToolSave, "calculator"},
		MaxSteps:     maxSteps,
		InputSchema:  specialistInputSchema(),
		OutputSchema: specialistOutputSchema(),
	}
}

func reviewerSpec(maxSteps int) Spec {
	return Spec{
		Name:         string(RoleReviewer),
		Description:  "verify conclusions, verify evidence, and find logical errors",
		Instruction:  reviewerInstruction,
		Tools:        []string{memory.ToolSearch},
		MaxSteps:     maxSteps,
		InputSchema:  reviewerInputSchema(),
		OutputSchema: reviewOutputSchema(),
	}
}

func specialistInputSchema() any {
	return map[string]any{
		"type":     "object",
		"required": []string{"task"},
		"properties": map[string]any{
			"agent": map[string]any{
				"type":        "string",
				"description": "Target agent name",
			},
			"task": map[string]any{
				"type":        "string",
				"description": "Task for this agent to complete",
			},
			"goal": map[string]any{
				"type":        "string",
				"description": "Parent user goal",
			},
			"context": map[string]any{
				"type":        "string",
				"description": "Optional extra context from the coordinator",
			},
		},
	}
}

func specialistOutputSchema() any {
	return map[string]any{
		"type":     "object",
		"required": []string{"agent", "task", "conclusion"},
		"properties": map[string]any{
			"agent":      map[string]any{"type": "string"},
			"task":       map[string]any{"type": "string"},
			"conclusion": map[string]any{"type": "string"},
			"evidence":   map[string]any{"type": "string"},
			"confidence": map[string]any{"type": "number"},
		},
	}
}

func analystOutputSchema() any {
	return map[string]any{
		"type":     "object",
		"required": []string{"summary", "metrics", "anomalies", "root_causes", "evidence", "recommendations", "confidence"},
		"properties": map[string]any{
			"summary":         map[string]any{"type": "string"},
			"metrics":         map[string]any{"type": "object"},
			"anomalies":       map[string]any{"type": "array"},
			"root_causes":     map[string]any{"type": "array"},
			"evidence":        map[string]any{"type": "array"},
			"recommendations": map[string]any{"type": "array"},
			"confidence":      map[string]any{"type": "number"},
			"agent":           map[string]any{"type": "string"},
			"task":            map[string]any{"type": "string"},
			"conclusion":      map[string]any{"type": "string"},
		},
	}
}

func reviewerInputSchema() any {
	return map[string]any{
		"type":     "object",
		"required": []string{"task"},
		"properties": map[string]any{
			"agent": map[string]any{
				"type": "string",
				"enum": []string{string(RoleReviewer)},
			},
			"task": map[string]any{
				"type":        "string",
				"description": "What to review",
			},
			"goal": map[string]any{
				"type": "string",
			},
			"conclusion": map[string]any{
				"type":        "string",
				"description": "Claim under review",
			},
			"evidence": map[string]any{
				"type":        "string",
				"description": "Supporting evidence",
			},
			"source_agent": map[string]any{
				"type":        "string",
				"description": "Which specialist produced the conclusion",
			},
		},
	}
}

func reviewOutputSchema() any {
	return map[string]any{
		"type":     "object",
		"required": []string{"approved", "reason"},
		"properties": map[string]any{
			"approved": map[string]any{
				"type":        "boolean",
				"description": "Whether the conclusion and evidence are acceptable",
			},
			"reason": map[string]any{
				"type":        "string",
				"description": "Why the review passed or failed, e.g. insufficient evidence",
			},
			"errors": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
		},
	}
}

// DefaultListings is the coordinator-facing catalog of specialists.
func DefaultListings(maxSteps int) []decision.AgentListing {
	specs := DefaultSpecs(maxSteps)
	out := make([]decision.AgentListing, 0, len(specs))
	for _, spec := range specs {
		out = append(out, spec.Listing())
	}
	return out
}

// Listing is the decision-prompt description of this agent.
func (s Spec) Listing() decision.AgentListing {
	return decision.AgentListing{
		Name:         s.Name,
		Description:  s.Description,
		InputSchema:  s.InputSchema,
		OutputSchema: s.OutputSchema,
	}
}

func cloneStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	return append([]string(nil), in...)
}
