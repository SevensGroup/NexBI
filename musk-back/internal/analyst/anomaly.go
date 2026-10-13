package analyst

import (
	"fmt"
	"math"
	"strings"
)

func detectAnomalies(s *State) []Anomaly {
	if s == nil {
		return []Anomaly{}
	}
	out := []Anomaly{}
	metric := s.Intent.Metric
	if metric == "" {
		metric = metricROAS
	}
	for _, dim := range breakdownDims {
		currentRows := s.rows(dim, periodCurrent)
		prevRows := indexRows(s.rows(dim, periodPrevDay), dim)
		for _, row := range currentRows {
			name := rowDimValue(row, dim)
			if name == "" {
				continue
			}
			curVal, ok := rowMetric(row, metric)
			if !ok {
				continue
			}
			baseVal, hasPrev := rowMetric(prevRows[name], metric)
			if !hasPrev {
				if overall, ok := s.Periods[periodCurrent].metric(metric); ok {
					baseVal, hasPrev = overall, true
				}
			}
			if !hasPrev {
				continue
			}
			chg := changePct(curVal, baseVal)
			if math.Abs(chg) < s.Threshold {
				continue
			}
			if !worse(metric, curVal, baseVal) {
				continue
			}
			out = append(out, Anomaly{
				Entity:     dim,
				Name:       name,
				Metric:     metric,
				Current:    curVal,
				Baseline:   baseVal,
				ChangePct:  round4(chg),
				Direction:  direction(metric, chg),
				EvidenceID: evidenceIDFor(s, dim, periodCurrent),
			})
		}
	}
	return out
}

func (s *State) rows(dim, period string) []map[string]any {
	if s == nil {
		return nil
	}
	byPeriod, ok := s.Breakdowns[dim]
	if !ok {
		return nil
	}
	return byPeriod[period]
}

func indexRows(rows []map[string]any, dim string) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, row := range rows {
		name := rowDimValue(row, dim)
		if name == "" {
			continue
		}
		out[name] = row
	}
	return out
}

func rowMetric(row map[string]any, metric string) (float64, bool) {
	if row == nil {
		return 0, false
	}
	if v, ok := asFloat(lookupMetric(row, metric)); ok {
		return v, true
	}
	snap := snapshotFromMetrics("", row)
	return snap.metric(metric)
}

func rowDimValue(row map[string]any, dim string) string {
	if row == nil {
		return ""
	}
	v, ok := row[dim]
	if !ok || v == nil {
		return ""
	}
	s := strings.TrimSpace(fmt.Sprint(v))
	if s == "" || s == "<nil>" {
		return ""
	}
	return s
}

func evidenceIDFor(s *State, dim, period string) string {
	if s == nil {
		return ""
	}
	for i := len(s.Evidence) - 1; i >= 0; i-- {
		e := s.Evidence[i]
		if strings.EqualFold(e.Dimension, dim) && e.Period == period {
			return e.ID
		}
	}
	if len(s.Evidence) > 0 {
		return s.Evidence[len(s.Evidence)-1].ID
	}
	return ""
}

func direction(_ string, change float64) string {
	if change < 0 {
		return "down"
	}
	if change > 0 {
		return "up"
	}
	return "flat"
}

func round4(v float64) float64 {
	return math.Round(v*10000) / 10000
}
