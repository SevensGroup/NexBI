package reflection

import (
	"strings"

	"autonomous-agent/internal/agent/decision"
)

// Analyze answers the six reflection questions with deterministic rules.
// NextAction is CONTINUE unless a three-in-a-row loop forces REPLAN.
// FINISH / ASK_USER are left to the LLM path (or a caller-injected Reflector).
func Analyze(state *decision.AgentState, observation decision.Observation) *Reflection {
	obs := effectiveObservation(state, observation)
	r := &Reflection{
		GoalSatisfied:      false,
		EvidenceSufficient: false,
		ResultCorrect:      false,
		MissingInformation: []string{},
		Errors:             []string{},
		NextAction:         NextContinue,
		Confidence:         obs.Confidence,
	}
	if r.Confidence <= 0 {
		r.Confidence = 0.5
	}

	if DataBackendUnavailable(state, obs) {
		return dataBackendAskUserReflection()
	}

	if LoopDetected(state) {
		return loopReflection()
	}

	evidence := strings.TrimSpace(obs.Evidence)
	summary := strings.TrimSpace(obs.Summary)
	failed := observationFailed(obs)
	empty := evidence == ""

	if failed {
		r.ResultCorrect = false
		r.EvidenceSufficient = false
		r.GoalSatisfied = false
		errMsg := firstNonEmpty(evidence, summary, "tool execution failed")
		r.Errors = append(r.Errors, errMsg)
		r.MissingInformation = append(r.MissingInformation, "a successful tool result")
		return r
	}

	r.ResultCorrect = !empty
	r.EvidenceSufficient = !empty
	if empty {
		r.MissingInformation = append(r.MissingInformation, "tool returned no evidence")
	}

	if state != nil {
		if len(state.FailedTasks) > 0 {
			r.GoalSatisfied = false
			r.Errors = append(r.Errors, "failed tasks: "+strings.Join(state.FailedTasks, ", "))
		}
		if planIncomplete(state) {
			r.GoalSatisfied = false
			r.MissingInformation = appendUnique(r.MissingInformation, "remaining planned tasks")
		} else if r.EvidenceSufficient && len(r.Errors) == 0 {
			r.GoalSatisfied = true
		}
	}

	return r
}

func effectiveObservation(state *decision.AgentState, obs decision.Observation) decision.Observation {
	if strings.TrimSpace(obs.Summary) != "" || strings.TrimSpace(obs.Evidence) != "" {
		return obs
	}
	if state != nil && len(state.Observations) > 0 {
		return state.Observations[len(state.Observations)-1]
	}
	return obs
}

func observationFailed(obs decision.Observation) bool {
	summary := strings.ToLower(strings.TrimSpace(obs.Summary))
	if strings.HasSuffix(summary, " error") || strings.Contains(summary, " error") {
		return true
	}
	evidence := strings.ToLower(strings.TrimSpace(obs.Evidence))
	if strings.HasPrefix(evidence, "error:") || strings.Contains(evidence, "\"error\"") {
		return true
	}
	return false
}

func planIncomplete(state *decision.AgentState) bool {
	if state == nil || len(state.Plan) == 0 {
		return false
	}
	return len(state.CompletedTasks) < len(state.Plan)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

func appendUnique(dst []string, items ...string) []string {
	seen := make(map[string]struct{}, len(dst))
	for _, existing := range dst {
		seen[existing] = struct{}{}
	}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		dst = append(dst, item)
	}
	return dst
}
