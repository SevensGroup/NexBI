package analyst

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"autonomous-agent/internal/logger"
	"autonomous-agent/internal/requestid"
	"autonomous-agent/internal/semantic"
	"autonomous-agent/internal/tools"
	"autonomous-agent/internal/tools/elasticsearch"
)

// Engine runs the autonomous advertising investigation loop.
// It queries through the Tool Registry (semantic_query), never Elasticsearch DSL.
type Engine struct {
	registry tools.Registry
	catalog  *semantic.Registry
	now      func() time.Time
	logger   logger.Logger
}

// NewEngine constructs an Advertising Autonomous Analyst.
func NewEngine(registry tools.Registry, catalog *semantic.Registry, log logger.Logger) *Engine {
	if log == nil {
		log = logger.New("info", "json", os.Stdout)
	}
	return &Engine{
		registry: registry,
		catalog:  catalog,
		now:      func() time.Time { return time.Now().UTC() },
		logger:   log,
	}
}

// Analyze understands the goal, runs follow-up queries until EvidenceSufficient, then
// forms root causes (evidence-only), runs the Reviewer check, and returns the report.
func (e *Engine) Analyze(ctx context.Context, goal string) (Report, *State, error) {
	if e == nil {
		return emptyReport(), nil, ErrNoSemanticTool
	}
	goal = strings.TrimSpace(goal)
	if goal == "" {
		return emptyReport(), nil, ErrEmptyGoal
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if e.registry == nil {
		return emptyReport(), nil, ErrNoSemanticTool
	}
	if _, ok := e.registry.Get(semantic.ToolName); !ok {
		return emptyReport(), nil, ErrNoSemanticTool
	}

	now := time.Now().UTC()
	if e.now != nil {
		now = e.now()
	}

	if _, ok := e.registry.Get(elasticsearch.NameAggregate); !ok {
		intent, perr := ParseIntent(goal, e.catalog, now)
		if perr != nil {
			intent = Intent{}
		}
		st := newState(intent)
		st.addEvidence("", "", ErrNoDataSource.Error(), map[string]any{"error": ErrNoDataSource.Error()})
		report := e.buildReport(st)
		report.Summary = "Cannot analyze advertising metrics: Elasticsearch MCP is not configured. Set ELASTICSEARCH_MCP_URL and restart."
		report.Confidence = 0
		e.log(ctx, "analyst investigation aborted", "error", ErrNoDataSource.Error())
		return report, st, ErrNoDataSource
	}

	intent, err := ParseIntent(goal, e.catalog, now)
	if err != nil {
		return emptyReport(), nil, err
	}

	state := newState(intent)
	e.log(ctx, "analyst investigation started",
		"goal", goal,
		"channel", intent.Channel,
		"metric", intent.Metric,
		"time", intent.TimeLabel,
	)

	for {
		next := state.Next()
		if next == nil {
			break
		}
		state.markAttempted(next)
		if err := e.runStep(ctx, state, next); err != nil {
			e.log(ctx, "analyst step failed", "step", next.Name, "error", err.Error())
			state.addEvidence(next.Period, next.Dimension, "query error: "+err.Error(), map[string]any{"error": err.Error()})
			continue
		}
	}

	state.Anomalies = detectAnomalies(state)
	report := e.buildReport(state)
	DropUncited(&report)
	rev := Review(report)
	if !rev.Approved {
		DropUncited(&report)
		report.Summary = strings.TrimSpace(report.Summary + " Reviewer: " + rev.Reason)
		e.log(ctx, "analyst reviewer rejected", "reason", rev.Reason, "errors", strings.Join(rev.Errors, "; "))
	}

	check := state.Sufficient()
	if !check.Sufficient && len(report.Evidence) == 0 {
		return report, state, ErrInsufficientEvidence
	}
	e.log(ctx, "analyst investigation finished",
		"queries", state.Queries,
		"evidence_sufficient", check.Sufficient,
		"anomalies", len(report.Anomalies),
		"root_causes", len(report.RootCauses),
		"review_approved", rev.Approved,
	)
	return report, state, nil
}

func (e *Engine) runStep(ctx context.Context, state *State, step *Step) error {
	input := map[string]any{
		"metrics":    queryMetrics,
		"filters":    filterInput(state.Intent.Filters),
		"time_range": timeRangeInput(step.TimeRange),
		"limit":      defaultBreakdownLimit,
	}
	if step.Dimension != "" {
		input["dimensions"] = []any{step.Dimension}
	}

	res, err := e.registry.Execute(ctx, semantic.ToolName, input)
	if err != nil && !res.Success {
		return err
	}
	if !res.Success {
		return fmt.Errorf("%s", res.Error)
	}

	result := resultFromData(res.Data)
	metrics := cloneAnyMap(result.Metrics)
	if len(metrics) == 0 && len(result.Rows) == 1 {
		metrics = cloneAnyMap(result.Rows[0])
	}

	desc := step.Name
	if state.Intent.Channel != "" {
		desc = step.Name + " " + state.Intent.Channel
	}
	ev := state.addEvidence(step.Period, step.Dimension, desc, metricsFromResult(result, step.Dimension))

	if step.Dimension == "" {
		snap := snapshotFromMetrics(step.Period, metrics)
		if step.Period == periodSevenDay {
			days := step.TimeRange.Duration().Hours() / 24
			if days < 1 {
				days = 7
			}
			snap = snap.averageByDays(days)
			snap.Label = periodSevenDay
		}
		state.Periods[step.Period] = snap
	} else {
		if state.Breakdowns[step.Dimension] == nil {
			state.Breakdowns[step.Dimension] = map[string][]map[string]any{}
		}
		state.Breakdowns[step.Dimension][step.Period] = result.Rows
	}

	e.log(ctx, "analyst step observed",
		"step", step.Name,
		"period", step.Period,
		"dimension", step.Dimension,
		"evidence_id", ev.ID,
		"query_hash", metadataString(res.Metadata, "query_hash"),
	)
	return nil
}

func (e *Engine) buildReport(state *State) Report {
	report := emptyReport()
	if state == nil {
		return report
	}
	report.Evidence = append([]EvidenceItem(nil), state.Evidence...)
	report.Anomalies = append([]Anomaly(nil), state.Anomalies...)
	report.RootCauses = FormCauses(state)
	report.Recommendations = FormRecommendations(state, report.RootCauses)

	metrics := map[string]any{
		"channel": state.Intent.Channel,
		"metric":  state.Intent.Metric,
		"time":    state.Intent.TimeLabel,
	}
	if snap, ok := state.Periods[periodCurrent]; ok && snap.Observed {
		metrics[periodCurrent] = snap.asMap()
	}
	if snap, ok := state.Periods[periodPrevDay]; ok && snap.Observed {
		metrics[periodPrevDay] = snap.asMap()
	}
	if snap, ok := state.Periods[periodSevenDay]; ok && snap.Observed {
		metrics[periodSevenDay] = snap.asMap()
	}
	report.Metrics = metrics
	report.Summary = summarize(state, report)
	report.Confidence = confidence(state)
	return report
}

func summarize(state *State, report Report) string {
	if state == nil {
		return ""
	}
	metric := state.Intent.Metric
	channel := state.Intent.Channel
	when := state.Intent.TimeLabel
	cur, okC := state.Periods[periodCurrent]
	prev, okP := state.Periods[periodPrevDay]
	if !okC || !cur.Observed {
		return "Insufficient observations to summarize " + strings.TrimSpace(channel+" "+metric)
	}
	cv, _ := cur.metric(metric)
	parts := []string{fmt.Sprintf("%s %s %s is %.4g", firstNonEmpty(when, "current"), firstNonEmpty(channel, "channel"), metric, cv)}
	if okP && prev.Observed {
		if pv, ok := prev.metric(metric); ok {
			chg := changePct(cv, pv) * 100
			parts = append(parts, fmt.Sprintf("previous day %.4g (%+.1f%%)", pv, chg))
		}
	}
	if seven, ok := state.Periods[periodSevenDay]; ok && seven.Observed {
		if sv, ok := seven.metric(metric); ok {
			parts = append(parts, fmt.Sprintf("7 day average %.4g", sv))
		}
	}
	if len(report.RootCauses) > 0 {
		parts = append(parts, "Root cause: "+report.RootCauses[0].Statement)
	} else if !state.targetDeclined() {
		parts = append(parts, "No evidenced decline versus the comparison windows.")
	}
	return strings.Join(parts, ". ")
}

func confidence(state *State) float64 {
	if state == nil {
		return 0
	}
	c := 0.35
	if state.hasPeriod(periodCurrent) {
		c += 0.15
	}
	if state.hasPeriod(periodPrevDay) {
		c += 0.15
	}
	if state.hasPeriod(periodSevenDay) {
		c += 0.1
	}
	if state.hasBreakdown("Campaign", periodCurrent) {
		c += 0.1
	}
	if state.hasBreakdown("Creative", periodCurrent) {
		c += 0.1
	}
	if check := state.Sufficient(); check.Sufficient {
		c += 0.05
	}
	if c > 0.95 {
		c = 0.95
	}
	return round4(c)
}

func resultFromData(data any) semantic.Result {
	switch t := data.(type) {
	case semantic.Result:
		return t
	case *semantic.Result:
		if t == nil {
			return semantic.Result{}
		}
		return *t
	case map[string]any:
		raw, _ := json.Marshal(t)
		var r semantic.Result
		_ = json.Unmarshal(raw, &r)
		return r
	default:
		raw, err := json.Marshal(data)
		if err != nil {
			return semantic.Result{}
		}
		var r semantic.Result
		_ = json.Unmarshal(raw, &r)
		return r
	}
}

func metricsFromResult(result semantic.Result, dimension string) map[string]any {
	if dimension == "" {
		if len(result.Metrics) > 0 {
			return cloneAnyMap(result.Metrics)
		}
		if len(result.Rows) == 1 {
			return cloneAnyMap(result.Rows[0])
		}
		return map[string]any{"row_count": len(result.Rows)}
	}
	out := map[string]any{"row_count": len(result.Rows)}
	if len(result.Rows) > 0 {
		out["sample"] = cloneAnyMap(result.Rows[0])
	}
	return out
}

func metadataString(meta map[string]any, key string) string {
	if meta == nil {
		return ""
	}
	v, ok := meta[key]
	if !ok || v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func (e *Engine) log(ctx context.Context, msg string, kv ...any) {
	if e == nil || e.logger == nil {
		return
	}
	args := append([]any{"request_id", requestid.From(ctx)}, kv...)
	e.logger.InfoContext(ctx, msg, args...)
}
