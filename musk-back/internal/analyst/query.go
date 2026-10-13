package analyst

import (
	"strings"
	"time"

	"autonomous-agent/internal/semantic"
)

const (
	stepCurrentTotals = "current_totals"
	stepPrevTotals    = "previous_day_totals"
	stepSevenTotals   = "seven_day_totals"
	stepCampaign      = "campaign"
	stepAdGroup       = "adgroup"
	stepCreative      = "creative"
	stepCountry       = "country"
)

// Step is one autonomous follow-up query the analyst may run.
type Step struct {
	Name      string
	Period    string
	Dimension string
	TimeRange semantic.TimeRange
}

func periodWindows(current semantic.TimeRange) (prev, seven semantic.TimeRange) {
	if current.IsZero() {
		return semantic.TimeRange{}, semantic.TimeRange{}
	}
	span := current.End.Sub(current.Start)
	if span <= 0 {
		span = 24 * time.Hour
	}
	prev = semantic.TimeRange{
		Start: current.Start.Add(-span),
		End:   current.Start,
		Label: periodPrevDay,
	}
	seven = semantic.TimeRange{
		Start: current.Start.AddDate(0, 0, -7),
		End:   current.Start,
		Label: periodSevenDay,
	}
	if !seven.End.After(seven.Start) {
		seven.End = seven.Start.Add(7 * 24 * time.Hour)
	}
	return prev, seven
}

func timeRangeInput(tr semantic.TimeRange) map[string]any {
	out := map[string]any{}
	if !tr.Start.IsZero() {
		out["start"] = tr.Start.UTC().Format(time.RFC3339)
	}
	if !tr.End.IsZero() {
		out["end"] = tr.End.UTC().Format(time.RFC3339)
	}
	if tr.Label != "" {
		out["label"] = tr.Label
	}
	return out
}

func filterInput(filters []semantic.Filter) []any {
	out := make([]any, 0, len(filters))
	for _, f := range filters {
		out = append(out, map[string]any{
			"dimension": f.Dimension,
			"op":        firstNonEmpty(f.Op, "eq"),
			"value":     f.Value,
		})
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
