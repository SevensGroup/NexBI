package runtime

import (
	"autonomous-agent/internal/agent/multi"
	"autonomous-agent/internal/semantic"
	"autonomous-agent/internal/tools"
)

// ListAgents returns Coordinator plus Analyst / Researcher / Reviewer.
func (r *Runtime) ListAgents() []AgentInfo {
	if r == nil {
		return nil
	}
	out := []AgentInfo{{
		Name:        firstNonEmpty(r.agentName, string(multi.RoleCoordinator)),
		Description: "Coordinator: plans, delegates, and finishes the user goal",
		Role:        string(multi.RoleCoordinator),
	}}
	specs := multi.DefaultSpecs(r.maxSteps)
	byName := map[string]multi.Spec{}
	for _, spec := range specs {
		byName[spec.Name] = spec
	}
	if r.team != nil {
		for _, listing := range r.team.Listings() {
			info := AgentInfo{
				Name:         listing.Name,
				Description:  listing.Description,
				Role:         listing.Name,
				InputSchema:  listing.InputSchema,
				OutputSchema: listing.OutputSchema,
			}
			if spec, ok := byName[listing.Name]; ok {
				info.Tools = append([]string(nil), spec.Tools...)
			}
			out = append(out, info)
		}
		return out
	}
	for _, spec := range specs {
		out = append(out, AgentInfo{
			Name:         spec.Name,
			Description:  spec.Description,
			Role:         spec.Name,
			Tools:        append([]string(nil), spec.Tools...),
			InputSchema:  spec.InputSchema,
			OutputSchema: spec.OutputSchema,
		})
	}
	return out
}

// ListTools returns the unified Tool Registry catalog.
func (r *Runtime) ListTools() []tools.Info {
	if r == nil || r.registry == nil {
		return nil
	}
	return r.registry.List()
}

// ListSemanticMetrics returns advertising metrics from the Semantic Layer catalog.
func (r *Runtime) ListSemanticMetrics() []semantic.Metric {
	if r == nil || r.catalog == nil {
		return nil
	}
	return r.catalog.ListMetrics()
}

// ListSemanticDimensions returns advertising dimensions from the Semantic Layer catalog.
func (r *Runtime) ListSemanticDimensions() []semantic.Dimension {
	if r == nil || r.catalog == nil {
		return nil
	}
	return r.catalog.ListDimensions()
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
