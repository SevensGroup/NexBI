package analyst

import (
	"fmt"
	"strings"
)

// ReviewResult is the deterministic Reviewer check: every conclusion must cite evidence.
type ReviewResult struct {
	Approved bool     `json:"approved"`
	Reason   string   `json:"reason"`
	Errors   []string `json:"errors,omitempty"`
}

// Review rejects reports whose root causes or recommendations lack evidence.
func Review(report Report) ReviewResult {
	ids := map[string]struct{}{}
	for _, e := range report.Evidence {
		id := strings.TrimSpace(e.ID)
		if id != "" {
			ids[id] = struct{}{}
		}
	}

	var errs []string
	if len(report.Evidence) == 0 {
		errs = append(errs, "no evidence")
	}
	for i, cause := range report.RootCauses {
		if len(cause.EvidenceIDs) == 0 {
			errs = append(errs, fmt.Sprintf("root_causes[%d] has no evidence", i))
			continue
		}
		for _, id := range cause.EvidenceIDs {
			if _, ok := ids[strings.TrimSpace(id)]; !ok {
				errs = append(errs, fmt.Sprintf("root_causes[%d] cites unknown evidence %q", i, id))
			}
		}
	}
	for i, rec := range report.Recommendations {
		if len(rec.EvidenceIDs) == 0 {
			errs = append(errs, fmt.Sprintf("recommendations[%d] has no evidence", i))
			continue
		}
		for _, id := range rec.EvidenceIDs {
			if _, ok := ids[strings.TrimSpace(id)]; !ok {
				errs = append(errs, fmt.Sprintf("recommendations[%d] cites unknown evidence %q", i, id))
			}
		}
	}
	for i, a := range report.Anomalies {
		id := strings.TrimSpace(a.EvidenceID)
		if id == "" {
			errs = append(errs, fmt.Sprintf("anomalies[%d] has no evidence", i))
			continue
		}
		if _, ok := ids[id]; !ok {
			errs = append(errs, fmt.Sprintf("anomalies[%d] cites unknown evidence %q", i, id))
		}
	}

	if len(errs) > 0 {
		return ReviewResult{Approved: false, Reason: "insufficient evidence", Errors: errs}
	}
	if strings.TrimSpace(report.Summary) == "" {
		return ReviewResult{Approved: false, Reason: "insufficient evidence", Errors: []string{"summary is empty"}}
	}
	return ReviewResult{Approved: true, Reason: "conclusion matches evidence"}
}

// DropUncited removes conclusions the Reviewer would reject, so the report never
// ships a cause without evidence.
func DropUncited(report *Report) {
	if report == nil {
		return
	}
	ids := map[string]struct{}{}
	for _, e := range report.Evidence {
		id := strings.TrimSpace(e.ID)
		if id != "" {
			ids[id] = struct{}{}
		}
	}
	causes := make([]RootCause, 0, len(report.RootCauses))
	for _, c := range report.RootCauses {
		if cited(ids, c.EvidenceIDs) {
			causes = append(causes, c)
		}
	}
	report.RootCauses = causes

	recs := make([]Recommendation, 0, len(report.Recommendations))
	for _, r := range report.Recommendations {
		if cited(ids, r.EvidenceIDs) {
			recs = append(recs, r)
		}
	}
	report.Recommendations = recs

	anoms := make([]Anomaly, 0, len(report.Anomalies))
	for _, a := range report.Anomalies {
		if _, ok := ids[strings.TrimSpace(a.EvidenceID)]; ok {
			anoms = append(anoms, a)
		}
	}
	report.Anomalies = anoms
}

func cited(ids map[string]struct{}, refs []string) bool {
	if len(refs) == 0 {
		return false
	}
	for _, id := range refs {
		if _, ok := ids[strings.TrimSpace(id)]; !ok {
			return false
		}
	}
	return true
}
