package analyst

import (
	"fmt"
	"math"
	"strings"
)

const (
	periodCurrent  = "current"
	periodPrevDay  = "previous_day"
	periodSevenDay = "seven_day_average"
)

var (
	baseMetrics    = []string{"Spend", "Revenue", "Impressions", "Clicks", "Conversions"}
	derivedMetrics = []string{"ROAS", "CTR", "CPC", "CVR", "CPA"}
	queryMetrics   = append(append([]string{}, baseMetrics...), derivedMetrics...)
	breakdownDims  = []string{"Campaign", "AdGroup", "Creative", "Country"}
)

// Snapshot is one period's leaf + derived advertising metrics.
type Snapshot struct {
	Label       string  `json:"label"`
	Spend       float64 `json:"Spend"`
	Revenue     float64 `json:"Revenue"`
	Impressions float64 `json:"Impressions"`
	Clicks      float64 `json:"Clicks"`
	Conversions float64 `json:"Conversions"`
	ROAS        float64 `json:"ROAS"`
	CTR         float64 `json:"CTR"`
	CPC         float64 `json:"CPC"`
	CVR         float64 `json:"CVR"`
	CPA         float64 `json:"CPA"`
	Days        float64 `json:"days,omitempty"`
	HasROAS     bool    `json:"-"`
	HasCTR      bool    `json:"-"`
	HasCPC      bool    `json:"-"`
	HasCVR      bool    `json:"-"`
	HasCPA      bool    `json:"-"`
	Observed    bool    `json:"-"`
}

func snapshotFromMetrics(label string, metrics map[string]any) Snapshot {
	s := Snapshot{Label: label, Observed: true}
	s.Spend, _ = asFloat(lookupMetric(metrics, "Spend"))
	s.Revenue, _ = asFloat(lookupMetric(metrics, "Revenue"))
	s.Impressions, _ = asFloat(lookupMetric(metrics, "Impressions"))
	s.Clicks, _ = asFloat(lookupMetric(metrics, "Clicks"))
	s.Conversions, _ = asFloat(lookupMetric(metrics, "Conversions"))
	s.ROAS, s.HasROAS = asFloat(lookupMetric(metrics, "ROAS"))
	s.CTR, s.HasCTR = asFloat(lookupMetric(metrics, "CTR"))
	s.CPC, s.HasCPC = asFloat(lookupMetric(metrics, "CPC"))
	s.CVR, s.HasCVR = asFloat(lookupMetric(metrics, "CVR"))
	s.CPA, s.HasCPA = asFloat(lookupMetric(metrics, "CPA"))
	s.computeDerived()
	return s
}

func (s *Snapshot) computeDerived() {
	if s == nil {
		return
	}
	if !s.HasROAS {
		if v, ok := ratio(s.Revenue, s.Spend); ok {
			s.ROAS, s.HasROAS = v, true
		}
	}
	if !s.HasCTR {
		if v, ok := ratio(s.Clicks, s.Impressions); ok {
			s.CTR, s.HasCTR = v, true
		}
	}
	if !s.HasCPC {
		if v, ok := ratio(s.Spend, s.Clicks); ok {
			s.CPC, s.HasCPC = v, true
		}
	}
	if !s.HasCVR {
		if v, ok := ratio(s.Conversions, s.Clicks); ok {
			s.CVR, s.HasCVR = v, true
		}
	}
	if !s.HasCPA {
		if v, ok := ratio(s.Spend, s.Conversions); ok {
			s.CPA, s.HasCPA = v, true
		}
	}
}

func (s Snapshot) averageByDays(days float64) Snapshot {
	if days <= 0 {
		days = 1
	}
	out := s
	out.Days = days
	out.Spend /= days
	out.Revenue /= days
	out.Impressions /= days
	out.Clicks /= days
	out.Conversions /= days
	out.HasROAS, out.HasCTR, out.HasCPC, out.HasCVR, out.HasCPA = false, false, false, false, false
	out.computeDerived()
	return out
}

func (s Snapshot) asMap() map[string]any {
	out := map[string]any{
		"Spend":       s.Spend,
		"Revenue":     s.Revenue,
		"Impressions": s.Impressions,
		"Clicks":      s.Clicks,
		"Conversions": s.Conversions,
	}
	if s.HasROAS {
		out["ROAS"] = s.ROAS
	}
	if s.HasCTR {
		out["CTR"] = s.CTR
	}
	if s.HasCPC {
		out["CPC"] = s.CPC
	}
	if s.HasCVR {
		out["CVR"] = s.CVR
	}
	if s.HasCPA {
		out["CPA"] = s.CPA
	}
	if s.Days > 0 {
		out["days"] = s.Days
	}
	return out
}

func (s Snapshot) metric(name string) (float64, bool) {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "ROAS", "ROI":
		return s.ROAS, s.HasROAS
	case "CTR":
		return s.CTR, s.HasCTR
	case "CPC":
		return s.CPC, s.HasCPC
	case "CVR":
		return s.CVR, s.HasCVR
	case "CPA":
		return s.CPA, s.HasCPA
	case "SPEND":
		return s.Spend, true
	case "REVENUE":
		return s.Revenue, true
	case "IMPRESSIONS":
		return s.Impressions, true
	case "CLICKS":
		return s.Clicks, true
	case "CONVERSIONS":
		return s.Conversions, true
	default:
		return 0, false
	}
}

func ratio(num, den float64) (float64, bool) {
	if den == 0 || math.IsNaN(den) || math.IsInf(den, 0) {
		return 0, false
	}
	return num / den, true
}

func changePct(current, baseline float64) float64 {
	if baseline == 0 {
		if current == 0 {
			return 0
		}
		return 1
	}
	return (current - baseline) / math.Abs(baseline)
}

func lookupMetric(metrics map[string]any, name string) any {
	if metrics == nil {
		return nil
	}
	if v, ok := metrics[name]; ok {
		return v
	}
	lower := strings.ToLower(name)
	for k, v := range metrics {
		if strings.EqualFold(k, name) || strings.ToLower(k) == lower {
			return v
		}
	}
	return nil
}

func asFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	case nil:
		return 0, false
	default:
		var f float64
		_, err := fmt.Sscan(strings.TrimSpace(fmt.Sprint(n)), &f)
		return f, err == nil
	}
}
