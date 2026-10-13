package analyst

import (
	"encoding/json"
	"fmt"
	"strings"

	"autonomous-agent/internal/agent/decision"
	"autonomous-agent/internal/agent/reflection"
	"autonomous-agent/internal/semantic"
)

// EvaluateAgentState decides whether an advertising run may FINISH.
// A single ROAS observation is not enough: comparisons and follow-up queries are required.
func EvaluateAgentState(state *decision.AgentState) Check {
	if state == nil {
		return Check{Missing: []string{"agent state"}}
	}
	if report, ok := reportFromState(state); ok {
		rev := Review(*report)
		if rev.Approved && len(report.Evidence) > 0 {
			if _, ok := report.Metrics[periodPrevDay]; ok {
				if _, ok := report.Metrics[periodSevenDay]; ok {
					return Check{Sufficient: true}
				}
			}
			if hasComparisonMetrics(report.Metrics) {
				return Check{Sufficient: true}
			}
		}
		if !rev.Approved {
			return Check{Missing: append([]string{"reviewer-approved evidence-backed report"}, rev.Errors...)}
		}
	}

	covered := coveredQueries(state)
	var missing []string
	if !covered.current {
		missing = append(missing, "current period metrics")
	}
	if !covered.prev {
		missing = append(missing, "previous day comparison")
	}
	if !covered.seven {
		missing = append(missing, "7 day average comparison")
	}
	if covered.current && (!covered.campaign || !covered.creative) {
		missing = append(missing, "Campaign and Creative follow-up queries")
	}
	return Check{Sufficient: len(missing) == 0 && covered.current, Missing: missing}
}

// GateReflection forces CONTINUE when an advertising goal does not yet have sufficient evidence.
// Missing Elasticsearch MCP is not recoverable by more queries: ASK_USER instead of looping.
func GateReflection(state *decision.AgentState, r *reflection.Reflection) {
	if r == nil || state == nil {
		return
	}
	if reflection.DataBackendUnavailable(state, lastObservation(state)) {
		r.EvidenceSufficient = false
		r.GoalSatisfied = false
		r.ResultCorrect = false
		r.NextAction = reflection.NextAskUser
		r.MissingInformation = appendUnique(r.MissingInformation, "ELASTICSEARCH_MCP_URL pointing at a reachable Elasticsearch MCP")
		r.Errors = appendUnique(r.Errors, "elasticsearch aggregate tool is not registered")
		return
	}
	if strings.EqualFold(strings.TrimSpace(r.NextAction), reflection.NextAskUser) {
		r.EvidenceSufficient = false
		r.GoalSatisfied = false
		return
	}
	if !IsAdvertisingGoal(state.Goal) && !IsAdvertisingGoal(state.CurrentObjective) {
		return
	}
	eval := EvaluateAgentState(state)
	if eval.Sufficient {
		r.EvidenceSufficient = true
		return
	}
	r.EvidenceSufficient = false
	r.GoalSatisfied = false
	if strings.EqualFold(strings.TrimSpace(r.NextAction), reflection.NextFinish) {
		r.NextAction = reflection.NextContinue
	}
	r.MissingInformation = appendUnique(r.MissingInformation, eval.Missing...)
}

func lastObservation(state *decision.AgentState) decision.Observation {
	if state == nil || len(state.Observations) == 0 {
		return decision.Observation{}
	}
	return state.Observations[len(state.Observations)-1]
}

func reportFromState(state *decision.AgentState) (*Report, bool) {
	if state == nil {
		return nil, false
	}
	for i := len(state.Observations) - 1; i >= 0; i-- {
		if r, ok := ParseReport(state.Observations[i].Evidence); ok {
			return r, true
		}
		if r, ok := ParseReport(state.Observations[i].Summary); ok {
			return r, true
		}
	}
	for i := len(state.ToolCalls) - 1; i >= 0; i-- {
		if r, ok := ParseReport(state.ToolCalls[i].Result); ok {
			return r, true
		}
	}
	return nil, false
}

type coverage struct {
	current, prev, seven, campaign, creative bool
}

func coveredQueries(state *decision.AgentState) coverage {
	var c coverage
	if state == nil {
		return c
	}
	for _, call := range state.ToolCalls {
		tool := strings.ToLower(strings.TrimSpace(call.Tool))
		if tool == ToolName && strings.TrimSpace(call.Result) != "" {
			if r, ok := ParseReport(call.Result); ok && len(r.Evidence) > 0 {
				c.current, c.prev, c.seven, c.campaign, c.creative = true, true, true, true, true
				return c
			}
		}
		if tool != semantic.ToolName {
			continue
		}
		args := call.Arguments
		period := periodFromArgs(args)
		dims := dimsFromArgs(args)
		switch period {
		case periodPrevDay:
			c.prev = true
		case periodSevenDay:
			c.seven = true
		default:
			c.current = true
		}
		for _, d := range dims {
			switch strings.ToLower(d) {
			case "campaign":
				c.campaign = true
			case "creative":
				c.creative = true
			}
		}
		blob := strings.ToLower(call.Result + " " + formatArgs(args))
		if strings.Contains(blob, "previous") || strings.Contains(blob, periodPrevDay) {
			c.prev = true
		}
		if strings.Contains(blob, "seven") || strings.Contains(blob, "7 day") || strings.Contains(blob, periodSevenDay) {
			c.seven = true
		}
		if strings.Contains(blob, "campaign") {
			c.campaign = true
		}
		if strings.Contains(blob, "creative") {
			c.creative = true
		}
	}
	return c
}

func periodFromArgs(args map[string]any) string {
	if args == nil {
		return ""
	}
	if tr, ok := args["time_range"].(map[string]any); ok {
		label := strings.ToLower(strings.TrimSpace(fmtSprintArg(tr["label"])))
		switch {
		case strings.Contains(label, "previous"):
			return periodPrevDay
		case strings.Contains(label, "seven") || strings.Contains(label, "7"):
			return periodSevenDay
		case label != "":
			return periodCurrent
		}
	}
	return ""
}

func dimsFromArgs(args map[string]any) []string {
	if args == nil {
		return nil
	}
	switch v := args["dimensions"].(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s := strings.TrimSpace(fmtSprintArg(item))
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func hasComparisonMetrics(metrics map[string]any) bool {
	if metrics == nil {
		return false
	}
	_, prev := metrics[periodPrevDay]
	_, seven := metrics[periodSevenDay]
	return prev && seven
}

func formatArgs(args map[string]any) string {
	if args == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(fmtSprintArg(args)))
}

func fmtSprintArg(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return strings.TrimSpace(strings.ReplaceAll(stringifyMap(v), "<nil>", ""))
}

func stringifyMap(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(raw)
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
