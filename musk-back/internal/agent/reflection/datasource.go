package reflection

import (
	"strings"

	"autonomous-agent/internal/agent/decision"
)

const (
	// DataBackendMissingAsk is the ASK_USER objective when advertising data
	// cannot be queried because Elasticsearch MCP is not registered.
	DataBackendMissingAsk = "Elasticsearch MCP is not configured (set ELASTICSEARCH_MCP_URL and restart). Advertising metrics cannot be queried, so this run cannot continue."
)

// DataBackendUnavailable reports whether the latest observation (or any
// observation / tool result on state) shows a missing advertising data backend.
// This is not recoverable by REPLAN: the process must be restarted with a real MCP URL.
func DataBackendUnavailable(state *decision.AgentState, obs decision.Observation) bool {
	if blobLooksLikeMissingDataBackend(obs.Summary, obs.Evidence) {
		return true
	}
	if state == nil {
		return false
	}
	for _, o := range state.Observations {
		if blobLooksLikeMissingDataBackend(o.Summary, o.Evidence) {
			return true
		}
	}
	for _, call := range state.ToolCalls {
		if blobLooksLikeMissingDataBackend(call.Tool, call.Result) {
			return true
		}
	}
	return false
}

func blobLooksLikeMissingDataBackend(parts ...string) bool {
	blob := strings.ToLower(strings.Join(parts, " "))
	if blob == "" {
		return false
	}
	if strings.Contains(blob, "elasticsearch aggregate tool is not registered") {
		return true
	}
	if strings.Contains(blob, "advertising data source is not configured") {
		return true
	}
	if strings.Contains(blob, "elasticsearch mcp is not configured") {
		return true
	}
	if strings.Contains(blob, "set elasticsearch_mcp_url") {
		return true
	}
	return strings.Contains(blob, "tool_not_found") && strings.Contains(blob, "elasticsearch")
}

func dataBackendAskUserReflection() *Reflection {
	return &Reflection{
		GoalSatisfied:      false,
		EvidenceSufficient: false,
		ResultCorrect:      false,
		MissingInformation: []string{"ELASTICSEARCH_MCP_URL pointing at a reachable Elasticsearch MCP"},
		Errors:             []string{"elasticsearch aggregate tool is not registered"},
		NextAction:         NextAskUser,
		Confidence:         1,
	}
}
