package analyst

import (
	"fmt"
	"strings"
)

// FormCauses builds root causes only from observed evidence. Uncited claims are dropped.
func FormCauses(s *State) []RootCause {
	if s == nil {
		return []RootCause{}
	}
	cur, okC := s.Periods[periodCurrent]
	prev, okP := s.Periods[periodPrevDay]
	if !okC || !cur.Observed {
		return []RootCause{}
	}

	idCurrent := evidenceIDFor(s, "", periodCurrent)
	idPrev := evidenceIDFor(s, "", periodPrevDay)
	idSeven := evidenceIDFor(s, "", periodSevenDay)
	periodIDs := compactIDs(idCurrent, idPrev, idSeven)

	var out []RootCause
	add := func(statement string, ids ...string) {
		ids = existingIDs(s, ids)
		if len(ids) == 0 {
			return
		}
		out = append(out, RootCause{Statement: statement, EvidenceIDs: ids})
	}

	metric := s.Intent.Metric
	if metric == "" {
		metric = metricROAS
	}

	if okP && prev.Observed {
		if cv, ok := cur.metric(metric); ok {
			if pv, ok := prev.metric(metric); ok && worse(metric, cv, pv) {
				add(fmt.Sprintf("%s declined from %.4g (previous day) to %.4g (current)", metric, pv, cv), periodIDs...)
			}
		}
		if cur.Revenue < prev.Revenue {
			add(fmt.Sprintf("Revenue declined from %.4g to %.4g", prev.Revenue, cur.Revenue), periodIDs...)
		}
		if cur.Spend > prev.Spend {
			add(fmt.Sprintf("Spend rose from %.4g to %.4g", prev.Spend, cur.Spend), periodIDs...)
		}
		if cur.HasCTR && prev.HasCTR && cur.CTR < prev.CTR {
			add(fmt.Sprintf("CTR declined from %.4g to %.4g", prev.CTR, cur.CTR), periodIDs...)
		}
		if cur.HasCPC && prev.HasCPC && cur.CPC > prev.CPC {
			add(fmt.Sprintf("CPC rose from %.4g to %.4g", prev.CPC, cur.CPC), periodIDs...)
		}
		if cur.HasCVR && prev.HasCVR && cur.CVR < prev.CVR {
			add(fmt.Sprintf("CVR declined from %.4g to %.4g", prev.CVR, cur.CVR), periodIDs...)
		}
	}

	if seven, ok := s.Periods[periodSevenDay]; ok && seven.Observed {
		if cv, ok := cur.metric(metric); ok {
			if sv, ok := seven.metric(metric); ok && worse(metric, cv, sv) {
				add(fmt.Sprintf("%s is below the 7 day average (%.4g vs %.4g)", metric, cv, sv), compactIDs(idCurrent, idSeven)...)
			}
		}
	}

	for _, a := range s.Anomalies {
		ids := existingIDs(s, []string{a.EvidenceID})
		if len(ids) == 0 {
			continue
		}
		add(fmt.Sprintf("%s %s %s %s (%.4g vs baseline %.4g, %+.1f%%)",
			a.Entity, a.Name, a.Metric, a.Direction, a.Current, a.Baseline, a.ChangePct*100), ids...)
	}

	return dedupeCauses(out)
}

func FormRecommendations(s *State, causes []RootCause) []Recommendation {
	if s == nil {
		return []Recommendation{}
	}
	var out []Recommendation
	add := func(action string, ids ...string) {
		ids = existingIDs(s, ids)
		if len(ids) == 0 {
			return
		}
		out = append(out, Recommendation{Action: action, EvidenceIDs: ids})
	}

	seenEntity := map[string]struct{}{}
	for _, a := range s.Anomalies {
		key := a.Entity + ":" + a.Name
		if _, ok := seenEntity[key]; ok {
			continue
		}
		seenEntity[key] = struct{}{}
		add(fmt.Sprintf("Investigate %s %s — %s moved %s versus the prior period", a.Entity, a.Name, a.Metric, a.Direction), a.EvidenceID)
	}

	cur, okC := s.Periods[periodCurrent]
	prev, okP := s.Periods[periodPrevDay]
	if okC && okP && cur.Observed && prev.Observed {
		ids := existingIDs(s, []string{evidenceIDFor(s, "", periodCurrent), evidenceIDFor(s, "", periodPrevDay)})
		if cur.Spend > prev.Spend && cur.Revenue <= prev.Revenue {
			add("Review budget increases that did not produce matching revenue", ids...)
		}
		if cur.HasCTR && prev.HasCTR && cur.CTR < prev.CTR {
			add("Review creative and audience delivery that may be reducing CTR", ids...)
		}
	}
	if len(out) == 0 && len(causes) > 0 {
		ids := causes[0].EvidenceIDs
		add("Monitor the cited metrics and re-run the analysis after the next delivery day", ids...)
	}
	return out
}

func existingIDs(s *State, ids []string) []string {
	out := make([]string, 0, len(ids))
	seen := map[string]struct{}{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		if _, ok := s.evidenceByID(id); !ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func compactIDs(ids ...string) []string {
	out := make([]string, 0, len(ids))
	seen := map[string]struct{}{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func dedupeCauses(in []RootCause) []RootCause {
	seen := map[string]struct{}{}
	out := make([]RootCause, 0, len(in))
	for _, c := range in {
		key := c.Statement + "|" + strings.Join(c.EvidenceIDs, ",")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, c)
	}
	return out
}
