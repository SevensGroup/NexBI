package analyst

import (
	"fmt"
	"strings"
	"time"

	"autonomous-agent/internal/semantic"
)

// Intent is the understood advertising analysis goal.
// Example: "Why did Meta ad ROI decline yesterday" → Channel=Meta, Metric=ROAS, Time=yesterday.
type Intent struct {
	Goal      string
	Channel   string
	Metric    string
	TimeLabel string
	TimeRange semantic.TimeRange
	Filters   []semantic.Filter
}

const (
	metricROAS = "ROAS"
	dimChannel = "Channel"
)

// ParseIntent maps a natural-language advertising question onto Channel / Metric / Time.
func ParseIntent(goal string, catalog *semantic.Registry, now time.Time) (Intent, error) {
	goal = strings.TrimSpace(goal)
	if goal == "" {
		return Intent{}, ErrEmptyGoal
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	in := Intent{Goal: goal, Metric: metricROAS}
	if catalog != nil {
		if q, err := semantic.Parse(goal, catalog, now); err == nil && q != nil {
			if len(q.Metrics) > 0 {
				in.Metric = q.Metrics[0]
			}
			in.TimeRange = q.TimeRange
			in.TimeLabel = q.TimeRange.Label
			in.Filters = append([]semantic.Filter(nil), q.Filters...)
			for _, f := range q.Filters {
				if strings.EqualFold(f.Dimension, dimChannel) {
					in.Channel = strings.TrimSpace(fmt.Sprint(f.Value))
				}
			}
		}
	}

	if in.Channel == "" {
		in.Channel = detectChannel(goal)
	}
	if strings.TrimSpace(in.Metric) == "" {
		in.Metric = detectMetric(goal)
	}
	if in.TimeRange.IsZero() {
		in.TimeRange, in.TimeLabel = detectTime(goal, now)
	}
	if in.TimeLabel == "" && !in.TimeRange.IsZero() {
		in.TimeLabel = in.TimeRange.Label
	}
	if in.Channel != "" && !hasChannelFilter(in.Filters, in.Channel) {
		in.Filters = append(in.Filters, semantic.Filter{Dimension: dimChannel, Op: "eq", Value: in.Channel})
	}
	if strings.TrimSpace(in.Metric) == "" {
		in.Metric = metricROAS
	}
	return in, nil
}

// IsAdvertisingGoal reports whether the user asked for advertising metric analysis.
func IsAdvertisingGoal(goal string) bool {
	g := strings.ToLower(strings.TrimSpace(goal))
	if g == "" {
		return false
	}
	hasTopic := containsAny(g, "roi", "roas", "ctr", "cpc", "cvr", "cpa", "cpm", "meta", "facebook", "google ads", "tiktok", "spend", "campaign", "adgroup", "creative", "advertising")
	hasAsk := containsAny(g, "why", "drop", "declin", "anomal", "root cause", "analyze", "analysis")
	return hasTopic && hasAsk
}

func detectChannel(goal string) string {
	g := strings.ToLower(goal)
	switch {
	case strings.Contains(goal, "Meta") || strings.Contains(g, "meta") || strings.Contains(g, "facebook"):
		return "Meta"
	case strings.Contains(g, "google"):
		return "Google"
	case strings.Contains(g, "tiktok"):
		return "TikTok"
	default:
		return ""
	}
}

func detectMetric(goal string) string {
	g := strings.ToLower(goal)
	switch {
	case strings.Contains(g, "roas") || strings.Contains(g, "roi"):
		return metricROAS
	case strings.Contains(g, "ctr"):
		return "CTR"
	case strings.Contains(g, "cpc"):
		return "CPC"
	case strings.Contains(g, "cvr"):
		return "CVR"
	case strings.Contains(g, "cpa"):
		return "CPA"
	case strings.Contains(g, "cpm"):
		return "CPM"
	case strings.Contains(g, "spend"):
		return "Spend"
	case strings.Contains(g, "revenue"):
		return "Revenue"
	default:
		return metricROAS
	}
}

func detectTime(goal string, now time.Time) (semantic.TimeRange, string) {
	now = now.In(time.UTC)
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	g := strings.ToLower(goal)
	switch {
	case strings.Contains(g, "yesterday"):
		return semantic.TimeRange{Start: day.AddDate(0, 0, -1), End: day, Label: "yesterday"}, "yesterday"
	case strings.Contains(g, "today"):
		return semantic.TimeRange{Start: day, End: day.AddDate(0, 0, 1), Label: "today"}, "today"
	default:
		return semantic.TimeRange{Start: day.AddDate(0, 0, -1), End: day, Label: "yesterday"}, "yesterday"
	}
}

func hasChannelFilter(filters []semantic.Filter, channel string) bool {
	for _, f := range filters {
		if strings.EqualFold(f.Dimension, dimChannel) && strings.EqualFold(strings.TrimSpace(fmt.Sprint(f.Value)), channel) {
			return true
		}
	}
	return false
}

func containsAny(haystack string, needles ...string) bool {
	for _, n := range needles {
		if n != "" && strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}
