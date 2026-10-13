package memory

import (
	"context"
	"strings"
)

type runIDKey struct{}
type agentKey struct{}

const metadataAgent = "agent"

// WithRunID stores the agent-run identifier on ctx for MemoryTool defaults.
func WithRunID(ctx context.Context, runID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return ctx
	}
	return context.WithValue(ctx, runIDKey{}, runID)
}

// RunIDFrom returns the run id stored on ctx, or empty string.
func RunIDFrom(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(runIDKey{}).(string)
	return strings.TrimSpace(id)
}

// WithAgent stores the specialist (or coordinator) name on ctx so memory
// reads and writes stay inside that agent's scope.
func WithAgent(ctx context.Context, agent string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	agent = normalizeAgent(agent)
	if agent == "" {
		return ctx
	}
	return context.WithValue(ctx, agentKey{}, agent)
}

// AgentFrom returns the agent name stored on ctx, or empty string.
func AgentFrom(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	name, _ := ctx.Value(agentKey{}).(string)
	return normalizeAgent(name)
}

// ScopeRunID namespaces a parent run id with an agent name (parent:agent).
func ScopeRunID(runID, agent string) string {
	agent = normalizeAgent(agent)
	runID = strings.TrimSpace(runID)
	if agent == "" {
		return runID
	}
	suffix := ":" + agent
	if strings.HasSuffix(runID, suffix) {
		return runID
	}
	if runID == "" {
		return agent
	}
	return runID + suffix
}

func normalizeAgent(agent string) string {
	return strings.ToLower(strings.TrimSpace(agent))
}
