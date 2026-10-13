package multi

import (
	"encoding/json"
	"fmt"
	"strings"

	"autonomous-agent/internal/analyst"
)

// DelegateRequest is the Coordinator DELEGATE_AGENT payload.
// Example: {"agent":"analyst","task":"Analyze why Meta ROAS declined yesterday"}
type DelegateRequest struct {
	Agent string         `json:"agent"`
	Task  string         `json:"task"`
	Extra map[string]any `json:"-"`
}

// ReviewResult is the Reviewer output schema.
// Example: {"approved": false, "reason": "insufficient evidence"}
type ReviewResult struct {
	Approved bool     `json:"approved"`
	Reason   string   `json:"reason"`
	Errors   []string `json:"errors,omitempty"`
}

// AgentOutput is a specialist's structured result for the Coordinator.
type AgentOutput struct {
	Agent      string        `json:"agent"`
	Task       string        `json:"task"`
	Conclusion string        `json:"conclusion,omitempty"`
	Evidence   string        `json:"evidence,omitempty"`
	Confidence float64       `json:"confidence,omitempty"`
	Review     *ReviewResult `json:"review,omitempty"`
}

// Rejected reports a Reviewer decision of approved=false.
func (o *AgentOutput) Rejected() bool {
	return o != nil && o.Review != nil && !o.Review.Approved
}

func (r DelegateRequest) input() map[string]any {
	in := map[string]any{}
	for k, v := range r.Extra {
		in[k] = v
	}
	if strings.TrimSpace(r.Agent) != "" {
		in["agent"] = NormalizeRole(r.Agent)
	}
	if strings.TrimSpace(r.Task) != "" {
		in["task"] = strings.TrimSpace(r.Task)
	}
	return in
}

func stringArg(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	s := strings.TrimSpace(fmt.Sprint(v))
	if s == "" || s == "<nil>" {
		return ""
	}
	return s
}

func formatTaskMessage(spec Spec, input map[string]any) string {
	task := stringArg(input, "task")
	if task == "" {
		task = stringArg(input, "objective")
	}
	var b strings.Builder
	b.WriteString("You are agent ")
	b.WriteString(spec.Name)
	b.WriteString(".\n")
	b.WriteString("Task: ")
	b.WriteString(task)
	b.WriteString("\n")
	if goal := stringArg(input, "goal"); goal != "" {
		b.WriteString("Goal: ")
		b.WriteString(goal)
		b.WriteString("\n")
	}
	if ctx := stringArg(input, "context"); ctx != "" {
		b.WriteString("Context: ")
		b.WriteString(ctx)
		b.WriteString("\n")
	}
	if c := stringArg(input, "conclusion"); c != "" {
		b.WriteString("Conclusion: ")
		b.WriteString(c)
		b.WriteString("\n")
	}
	if e := stringArg(input, "evidence"); e != "" {
		b.WriteString("Evidence: ")
		b.WriteString(e)
		b.WriteString("\n")
	}
	if src := stringArg(input, "source_agent"); src != "" {
		b.WriteString("Source agent: ")
		b.WriteString(src)
		b.WriteString("\n")
	}
	if Role(spec.Name) == RoleReviewer {
		b.WriteString("Return a ReviewResult JSON in the FINISH objective: {\"approved\":true|false,\"reason\":\"...\"}.\n")
		b.WriteString("Reject with {\"approved\":false,\"reason\":\"insufficient evidence\"} when evidence is insufficient.\n")
	} else {
		b.WriteString("FINISH with a concise conclusion. Do not fabricate tool results.\n")
	}
	return b.String()
}

// ParseReview extracts ReviewResult from a FINISH objective or JSON blob.
func ParseReview(raw string) (*ReviewResult, bool) {
	payload := extractJSONObject(raw)
	if payload == "" {
		return nil, false
	}
	var rawMap map[string]any
	if err := json.Unmarshal([]byte(payload), &rawMap); err != nil {
		return nil, false
	}
	if _, ok := rawMap["approved"]; !ok {
		return nil, false
	}
	var rev ReviewResult
	if err := json.Unmarshal([]byte(payload), &rev); err != nil {
		return nil, false
	}
	rev.Reason = strings.TrimSpace(rev.Reason)
	if rev.Reason == "" && !rev.Approved {
		rev.Reason = "insufficient evidence"
	}
	return &rev, true
}

// ParseAgentOutput turns a specialist answer into Coordinator-facing output.
func ParseAgentOutput(agent, task, raw string) *AgentOutput {
	out := &AgentOutput{
		Agent:      NormalizeRole(agent),
		Task:       strings.TrimSpace(task),
		Conclusion: strings.TrimSpace(raw),
		Evidence:   strings.TrimSpace(raw),
	}
	if payload := extractJSONObject(raw); payload != "" {
		if report, ok := parseEmbeddedReport(payload); ok {
			if out.Conclusion == "" || out.Conclusion == strings.TrimSpace(raw) {
				out.Conclusion = strings.TrimSpace(report.Summary)
			}
			if out.Evidence == strings.TrimSpace(raw) && len(report.Evidence) > 0 {
				out.Evidence = payload
			}
			if report.Confidence > 0 {
				out.Confidence = report.Confidence
			}
		}
		var parsed AgentOutput
		if err := json.Unmarshal([]byte(payload), &parsed); err == nil && (parsed.Conclusion != "" || parsed.Review != nil || parsed.Evidence != "") {
			if parsed.Agent != "" {
				out.Agent = NormalizeRole(parsed.Agent)
			}
			if parsed.Task != "" {
				out.Task = strings.TrimSpace(parsed.Task)
			}
			if parsed.Conclusion != "" {
				out.Conclusion = strings.TrimSpace(parsed.Conclusion)
			}
			if parsed.Evidence != "" {
				out.Evidence = strings.TrimSpace(parsed.Evidence)
			}
			out.Confidence = parsed.Confidence
			out.Review = parsed.Review
		}
		if rev, ok := ParseReview(payload); ok {
			out.Review = rev
			if out.Conclusion == "" || out.Conclusion == strings.TrimSpace(raw) {
				out.Conclusion = rev.Reason
			}
			if !rev.Approved && out.Evidence == strings.TrimSpace(raw) {
				out.Evidence = rev.Reason
			}
		}
	}
	if Role(out.Agent) == RoleReviewer && out.Review == nil {
		out.Review = &ReviewResult{Approved: false, Reason: "insufficient evidence"}
		if out.Conclusion == "" {
			out.Conclusion = out.Review.Reason
		}
	}
	return out
}

func parseEmbeddedReport(payload string) (*analyst.Report, bool) {
	return analyst.ParseReport(payload)
}

func extractJSONObject(s string) string {
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
