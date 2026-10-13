package analyst

import (
	"encoding/json"
	"strings"
)

// Report is the Advertising Autonomous Analyst final output.
// Every root cause and recommendation must cite Evidence.
type Report struct {
	Summary         string           `json:"summary"`
	Metrics         map[string]any   `json:"metrics"`
	Anomalies       []Anomaly        `json:"anomalies"`
	RootCauses      []RootCause      `json:"root_causes"`
	Evidence        []EvidenceItem   `json:"evidence"`
	Recommendations []Recommendation `json:"recommendations"`
	Confidence      float64          `json:"confidence"`
}

// Anomaly is a dimension value whose metric moved beyond the investigation threshold.
type Anomaly struct {
	Entity     string  `json:"entity"`
	Name       string  `json:"name"`
	Metric     string  `json:"metric"`
	Current    float64 `json:"current"`
	Baseline   float64 `json:"baseline"`
	ChangePct  float64 `json:"change_pct"`
	Direction  string  `json:"direction,omitempty"`
	EvidenceID string  `json:"evidence_id"`
}

// RootCause is a conclusion that is only emitted when evidence_ids resolve.
type RootCause struct {
	Statement   string   `json:"statement"`
	EvidenceIDs []string `json:"evidence_ids"`
}

// EvidenceItem is one real query observation. IDs are cited by causes and recommendations.
type EvidenceItem struct {
	ID          string         `json:"id"`
	Description string         `json:"description"`
	Period      string         `json:"period,omitempty"`
	Dimension   string         `json:"dimension,omitempty"`
	Metrics     map[string]any `json:"metrics,omitempty"`
}

// Recommendation is an action grounded in evidence.
type Recommendation struct {
	Action      string   `json:"action"`
	EvidenceIDs []string `json:"evidence_ids"`
}

// emptyReport returns a report whose array fields serialize as [] not null.
func emptyReport() Report {
	return Report{
		Metrics:         map[string]any{},
		Anomalies:       []Anomaly{},
		RootCauses:      []RootCause{},
		Evidence:        []EvidenceItem{},
		Recommendations: []Recommendation{},
	}
}

// ParseReport extracts a Report from a FINISH answer or tool observation.
func ParseReport(raw string) (*Report, bool) {
	payload := extractJSONObject(raw)
	if payload == "" {
		return nil, false
	}
	var probe map[string]any
	if err := json.Unmarshal([]byte(payload), &probe); err != nil {
		return nil, false
	}
	if _, ok := probe["summary"]; !ok {
		return nil, false
	}
	if _, ok := probe["metrics"]; !ok {
		return nil, false
	}
	if _, ok := probe["root_causes"]; !ok && probe["anomalies"] == nil && probe["evidence"] == nil {
		return nil, false
	}
	var r Report
	if err := json.Unmarshal([]byte(payload), &r); err != nil {
		return nil, false
	}
	if r.Metrics == nil {
		r.Metrics = map[string]any{}
	}
	if r.Anomalies == nil {
		r.Anomalies = []Anomaly{}
	}
	if r.RootCauses == nil {
		r.RootCauses = []RootCause{}
	}
	if r.Evidence == nil {
		r.Evidence = []EvidenceItem{}
	}
	if r.Recommendations == nil {
		r.Recommendations = []Recommendation{}
	}
	return &r, true
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
