package analyst

import (
	"fmt"
	"strings"

	"autonomous-agent/internal/semantic"
)

const (
	defaultMaxQueries       = 16
	defaultAnomalyThreshold = 0.15
	defaultBreakdownLimit   = 10
)

// State is the working memory of one autonomous investigation.
type State struct {
	Intent     Intent
	Periods    map[string]Snapshot
	Breakdowns map[string]map[string][]map[string]any // dimension → period → rows
	Evidence   []EvidenceItem
	Queries    int
	MaxQueries int
	Threshold  float64
	Anomalies  []Anomaly
	Attempted  map[string]struct{}
}

func newState(intent Intent) *State {
	return &State{
		Intent:     intent,
		Periods:    map[string]Snapshot{},
		Breakdowns: map[string]map[string][]map[string]any{},
		Evidence:   []EvidenceItem{},
		MaxQueries: defaultMaxQueries,
		Threshold:  defaultAnomalyThreshold,
		Attempted:  map[string]struct{}{},
	}
}

func stepKey(step *Step) string {
	if step == nil {
		return ""
	}
	return step.Name + "|" + step.Period + "|" + step.Dimension
}

func (s *State) markAttempted(step *Step) {
	if s == nil || step == nil {
		return
	}
	if s.Attempted == nil {
		s.Attempted = map[string]struct{}{}
	}
	s.Attempted[stepKey(step)] = struct{}{}
}

func (s *State) alreadyAttempted(step *Step) bool {
	if s == nil || step == nil || s.Attempted == nil {
		return false
	}
	_, ok := s.Attempted[stepKey(step)]
	return ok
}

func (s *State) addEvidence(period, dimension, description string, metrics map[string]any) EvidenceItem {
	if s == nil {
		return EvidenceItem{}
	}
	item := EvidenceItem{
		ID:          fmt.Sprintf("e%d", len(s.Evidence)+1),
		Description: description,
		Period:      period,
		Dimension:   dimension,
		Metrics:     cloneAnyMap(metrics),
	}
	s.Evidence = append(s.Evidence, item)
	s.Queries++
	return item
}

func (s *State) evidenceByID(id string) (EvidenceItem, bool) {
	if s == nil {
		return EvidenceItem{}, false
	}
	for _, e := range s.Evidence {
		if e.ID == id {
			return e, true
		}
	}
	return EvidenceItem{}, false
}

func (s *State) hasPeriod(label string) bool {
	if s == nil {
		return false
	}
	snap, ok := s.Periods[label]
	return ok && snap.Observed
}

func (s *State) hasBreakdown(dim, period string) bool {
	if s == nil {
		return false
	}
	byPeriod, ok := s.Breakdowns[dim]
	if !ok {
		return false
	}
	_, ok = byPeriod[period]
	return ok
}

func (s *State) targetDeclined() bool {
	if s == nil {
		return false
	}
	cur, okC := s.Periods[periodCurrent]
	if !okC || !cur.Observed {
		return false
	}
	metric := s.Intent.Metric
	cv, ok := cur.metric(metric)
	if !ok {
		return false
	}
	if prev, ok := s.Periods[periodPrevDay]; ok && prev.Observed {
		if pv, ok := prev.metric(metric); ok && worse(metric, cv, pv) {
			return true
		}
	}
	if seven, ok := s.Periods[periodSevenDay]; ok && seven.Observed {
		if sv, ok := seven.metric(metric); ok && worse(metric, cv, sv) {
			return true
		}
	}
	return false
}

func worse(metric string, current, baseline float64) bool {
	if baseline == 0 && current == 0 {
		return false
	}
	switch strings.ToUpper(strings.TrimSpace(metric)) {
	case "SPEND", "CPC", "CPA", "CPM":
		return current > baseline
	default:
		return current < baseline
	}
}

// Check is the EvidenceSufficient verdict plus what is still missing.
type Check struct {
	Sufficient bool
	Missing    []string
}

// Sufficient reports whether the investigation may finish.
func (s *State) Sufficient() Check {
	if s == nil {
		return Check{Missing: []string{"investigation state"}}
	}
	var missing []string
	if strings.TrimSpace(s.Intent.Metric) == "" {
		missing = append(missing, "target metric")
	}
	if !s.hasPeriod(periodCurrent) {
		missing = append(missing, "current period metrics (spend, revenue, impressions, clicks, conversions)")
	}
	if !s.hasPeriod(periodPrevDay) {
		missing = append(missing, "previous day comparison")
	}
	if !s.hasPeriod(periodSevenDay) {
		missing = append(missing, "7 day average comparison")
	}
	if s.hasPeriod(periodCurrent) && s.hasPeriod(periodPrevDay) && s.targetDeclined() {
		if !s.hasBreakdown("Campaign", periodCurrent) {
			missing = append(missing, "Campaign breakdown")
		}
		if !s.hasBreakdown("Creative", periodCurrent) {
			missing = append(missing, "Creative breakdown")
		}
	}
	if len(s.Evidence) == 0 {
		missing = append(missing, "at least one query observation")
	}
	return Check{Sufficient: len(missing) == 0, Missing: missing}
}

// Next returns the next follow-up query, or nil when EvidenceSufficient.
// Finding a ROAS drop does not finish: continue with revenue/spend/CTR/CPC/CVR and entity breakdowns.
func (s *State) Next() *Step {
	if s == nil {
		return nil
	}
	if s.MaxQueries > 0 && s.Queries >= s.MaxQueries {
		return nil
	}
	current := s.Intent.TimeRange
	prev, seven := periodWindows(current)

	candidates := []*Step{
		{Name: stepCurrentTotals, Period: periodCurrent, TimeRange: labeled(current, periodCurrent)},
	}
	if !prev.IsZero() {
		candidates = append(candidates, &Step{Name: stepPrevTotals, Period: periodPrevDay, TimeRange: prev})
	}
	if !seven.IsZero() {
		candidates = append(candidates, &Step{Name: stepSevenTotals, Period: periodSevenDay, TimeRange: seven})
	}
	if s.hasPeriod(periodCurrent) {
		for _, dim := range breakdownDims {
			candidates = append(candidates, &Step{
				Name: strings.ToLower(dim), Period: periodCurrent, Dimension: dim, TimeRange: labeled(current, periodCurrent),
			})
			if !prev.IsZero() {
				candidates = append(candidates, &Step{
					Name: strings.ToLower(dim) + "_previous", Period: periodPrevDay, Dimension: dim, TimeRange: prev,
				})
			}
		}
	}
	for _, step := range candidates {
		if s.alreadyAttempted(step) {
			continue
		}
		if step.Dimension == "" {
			if s.hasPeriod(step.Period) {
				continue
			}
			return step
		}
		if s.hasBreakdown(step.Dimension, step.Period) {
			continue
		}
		return step
	}

	if check := s.Sufficient(); check.Sufficient {
		return nil
	}
	return nil
}

func labeled(tr semantic.TimeRange, label string) semantic.TimeRange {
	tr.Label = label
	return tr
}

func cloneAnyMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
