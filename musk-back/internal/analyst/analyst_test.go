package analyst

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"autonomous-agent/internal/agent/decision"
	"autonomous-agent/internal/agent/reflection"
	"autonomous-agent/internal/config"
	"autonomous-agent/internal/logger"
	"autonomous-agent/internal/requestid"
	"autonomous-agent/internal/semantic"
	"autonomous-agent/internal/tools"
	"autonomous-agent/internal/tools/elasticsearch"
	"autonomous-agent/internal/tools/mcp"
)

func testCatalog(t *testing.T) *semantic.Registry {
	t.Helper()
	reg, err := semantic.Load(filepath.Join("..", "..", "configs", "semantic"))
	if err != nil {
		t.Fatalf("Load catalog: %v", err)
	}
	return reg
}

func TestParseIntentYesterdayMetaROI(t *testing.T) {
	now := time.Date(2026, 8, 25, 15, 0, 0, 0, time.UTC)
	in, err := ParseIntent("Why did Meta ad ROI decline yesterday", testCatalog(t), now)
	if err != nil {
		t.Fatalf("ParseIntent: %v", err)
	}
	if in.Channel != "Meta" {
		t.Fatalf("channel = %q", in.Channel)
	}
	if in.Metric != "ROAS" {
		t.Fatalf("metric = %q", in.Metric)
	}
	if in.TimeLabel != "yesterday" {
		t.Fatalf("time = %q", in.TimeLabel)
	}
}

func TestGateReflectionAsksUserWhenDataBackendMissing(t *testing.T) {
	state := decision.NewState("Why did Meta ad ROI decline yesterday", 8)
	state.Observations = []decision.Observation{{
		Summary:  "semantic_query error",
		Evidence: `TOOL_NOT_FOUND: tool "semantic_query": elasticsearch aggregate tool is not registered`,
	}}
	r := &reflection.Reflection{
		GoalSatisfied:      false,
		EvidenceSufficient: false,
		ResultCorrect:      false,
		NextAction:         reflection.NextReplan,
		Confidence:         0.2,
	}
	GateReflection(state, r)
	if r.NextAction != reflection.NextAskUser {
		t.Fatalf("next = %s, want ASK_USER", r.NextAction)
	}
}

func TestGateReflectionBlocksEarlyFinish(t *testing.T) {
	state := decision.NewState("Why did Meta ad ROI decline yesterday", 8)
	state.Observations = []decision.Observation{{
		Summary:  "semantic_query result",
		Evidence: `{"metrics":{"ROAS":1.1}}`,
	}}
	r := &reflection.Reflection{
		GoalSatisfied:      true,
		EvidenceSufficient: true,
		ResultCorrect:      true,
		NextAction:         reflection.NextFinish,
		Confidence:         0.9,
	}
	GateReflection(state, r)
	if r.EvidenceSufficient {
		t.Fatal("single ROAS observation is not sufficient")
	}
	if r.NextAction != reflection.NextContinue {
		t.Fatalf("next = %s, want CONTINUE", r.NextAction)
	}
}

func TestAnalyzeFailsFastWithoutElasticsearch(t *testing.T) {
	log := logger.New("error", "json", io.Discard)
	reg := tools.NewRegistry(log, time.Second)
	catalog := testCatalog(t)
	if err := reg.Register(semantic.NewQueryTool(catalog, reg, semantic.Limits{
		MaxTimeRange:  90 * 24 * time.Hour,
		MaxResultSize: 100,
	}, log), tools.RiskMedium); err != nil {
		t.Fatal(err)
	}
	eng := NewEngine(reg, catalog, log)
	eng.now = func() time.Time { return time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC) }

	start := time.Now()
	report, _, err := eng.Analyze(context.Background(), "Why did Meta ad ROI decline yesterday")
	if !errors.Is(err, ErrNoDataSource) {
		t.Fatalf("err = %v, want ErrNoDataSource", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("Analyze hung for %s", time.Since(start))
	}
	if !strings.Contains(report.Summary, "Elasticsearch") {
		t.Fatalf("summary = %q", report.Summary)
	}

	if err := reg.Register(NewTool(eng, log), tools.RiskMedium); err != nil {
		t.Fatal(err)
	}
	res, execErr := reg.Execute(context.Background(), ToolName, map[string]any{"query": "Why did Meta ad ROI decline yesterday"})
	if execErr == nil || res.Success {
		t.Fatalf("tool should fail without ES: %+v err=%v", res, execErr)
	}
	if !strings.Contains(res.Error, "elasticsearch") && !strings.Contains(res.Error, "ELASTICSEARCH_MCP_URL") {
		t.Fatalf("tool error = %q", res.Error)
	}
}

func TestNextDoesNotFinishAfterCurrentROAS(t *testing.T) {
	now := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	in, err := ParseIntent("Why did Meta ad ROI decline yesterday", testCatalog(t), now)
	if err != nil {
		t.Fatal(err)
	}
	st := newState(in)
	st.Periods[periodCurrent] = Snapshot{Label: periodCurrent, Spend: 200, Revenue: 180, Observed: true, ROAS: 0.9, HasROAS: true}
	st.Evidence = []EvidenceItem{{ID: "e1", Description: "current totals", Period: periodCurrent, Metrics: map[string]any{"ROAS": 0.9}}}
	st.Queries = 1
	next := st.Next()
	if next == nil {
		t.Fatal("must continue querying after a single ROAS observation")
	}
	if check := st.Sufficient(); check.Sufficient {
		t.Fatalf("evidence must not be sufficient yet: %+v", check)
	}
}

func TestUncitedRootCauseRejected(t *testing.T) {
	report := emptyReport()
	report.Summary = "creative fatigue"
	report.RootCauses = []RootCause{{Statement: "creative fatigue", EvidenceIDs: []string{}}}
	rev := Review(report)
	if rev.Approved {
		t.Fatal("reviewer must reject uncited causes")
	}
	if rev.Reason != "insufficient evidence" {
		t.Fatalf("reason = %q", rev.Reason)
	}
	DropUncited(&report)
	if len(report.RootCauses) != 0 {
		t.Fatalf("uncited causes must be dropped: %+v", report.RootCauses)
	}
}

func TestFormCausesRequireEvidence(t *testing.T) {
	st := newState(Intent{Metric: "ROAS", Channel: "Meta"})
	st.Periods[periodCurrent] = Snapshot{Label: periodCurrent, Spend: 200, Revenue: 180, ROAS: 0.9, HasROAS: true, Observed: true}
	st.Periods[periodPrevDay] = Snapshot{Label: periodPrevDay, Spend: 100, Revenue: 250, ROAS: 2.5, HasROAS: true, Observed: true}
	if got := FormCauses(st); len(got) != 0 {
		t.Fatalf("causes without evidence ids: %+v", got)
	}
	st.Evidence = []EvidenceItem{
		{ID: "e1", Period: periodCurrent, Description: "current"},
		{ID: "e2", Period: periodPrevDay, Description: "prev"},
	}
	got := FormCauses(st)
	if len(got) == 0 {
		t.Fatal("expected evidenced causes")
	}
	for _, c := range got {
		if len(c.EvidenceIDs) == 0 {
			t.Fatalf("cause missing evidence: %+v", c)
		}
	}
}

func TestEndToEndAdvertisingAnalyst(t *testing.T) {
	srv := mcp.StartMockServer()
	defer srv.Close()
	srv.Handler = advertisingFixtureHandler(t)

	log := logger.New("error", "json", io.Discard)
	reg := tools.NewRegistry(log, 5*time.Second)
	if err := elasticsearch.Register(context.Background(), reg, mustMCP(t, srv.URL), esCfg(srv.URL), log); err != nil {
		t.Fatalf("register es: %v", err)
	}
	catalog := testCatalog(t)
	if err := reg.Register(semantic.NewQueryTool(catalog, reg, semantic.Limits{}, log), tools.RiskMedium); err != nil {
		t.Fatalf("register semantic: %v", err)
	}
	eng := NewEngine(reg, catalog, log)
	eng.now = func() time.Time { return time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC) }
	if err := reg.Register(NewTool(eng, log), tools.RiskMedium); err != nil {
		t.Fatalf("register analyst: %v", err)
	}

	ctx := requestid.With(context.Background(), "trace-analyst-e2e")
	report, state, err := eng.Analyze(ctx, "Why did Meta ad ROI decline yesterday")
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if state == nil {
		t.Fatal("missing investigation state")
	}
	if state.Intent.Channel != "Meta" || state.Intent.Metric != "ROAS" || state.Intent.TimeLabel != "yesterday" {
		t.Fatalf("intent = %+v", state.Intent)
	}
	if state.Queries < 5 {
		t.Fatalf("queries = %d, agent must continue after the first ROAS observation", state.Queries)
	}
	if srv.CallCount("aggregate") < 5 {
		t.Fatalf("mcp aggregate calls = %d, want follow-up queries", srv.CallCount("aggregate"))
	}

	cur, ok := state.Periods[periodCurrent]
	if !ok || !cur.Observed {
		t.Fatal("missing current metrics")
	}
	for _, name := range []string{"Spend", "Revenue", "Impressions", "Clicks", "Conversions"} {
		if _, ok := cur.metric(name); !ok && name != "" {
			t.Fatalf("missing leaf %s", name)
		}
	}
	if cur.Spend == 0 || cur.Revenue == 0 {
		t.Fatalf("current snapshot = %+v", cur)
	}
	if !cur.HasROAS || !cur.HasCTR || !cur.HasCPC || !cur.HasCVR || !cur.HasCPA {
		t.Fatalf("derived metrics missing: %+v", cur)
	}
	if _, ok := state.Periods[periodPrevDay]; !ok {
		t.Fatal("missing previous day comparison")
	}
	if _, ok := state.Periods[periodSevenDay]; !ok {
		t.Fatal("missing 7 day average")
	}

	found := map[string]bool{}
	for _, a := range report.Anomalies {
		found[a.Entity] = true
	}
	for _, dim := range []string{"Campaign", "AdGroup", "Creative", "Country"} {
		if !state.hasBreakdown(dim, periodCurrent) {
			t.Fatalf("missing %s breakdown", dim)
		}
	}

	check := state.Sufficient()
	if !check.Sufficient {
		t.Fatalf("EvidenceSufficient=false missing=%v", check.Missing)
	}

	if strings.TrimSpace(report.Summary) == "" {
		t.Fatal("summary is required")
	}
	if report.Metrics == nil {
		t.Fatal("metrics is required")
	}
	if report.Anomalies == nil || report.RootCauses == nil || report.Evidence == nil || report.Recommendations == nil {
		t.Fatal("report slices must be present")
	}
	if len(report.Evidence) == 0 {
		t.Fatal("evidence is required")
	}
	if len(report.RootCauses) == 0 {
		t.Fatal("expected evidenced root causes")
	}
	for _, c := range report.RootCauses {
		if len(c.EvidenceIDs) == 0 {
			t.Fatalf("uncited cause: %+v", c)
		}
	}
	rev := Review(report)
	if !rev.Approved {
		t.Fatalf("reviewer rejected a cited report: %+v", rev)
	}
	if report.Confidence <= 0 {
		t.Fatalf("confidence = %v", report.Confidence)
	}

	raw := MarshalReport(report)
	parsed, ok := ParseReport(raw)
	if !ok || parsed.Summary != report.Summary {
		t.Fatalf("round-trip report failed: ok=%v", ok)
	}

	res, err := reg.Execute(ctx, ToolName, map[string]any{"query": "Why did Meta ad ROI decline yesterday"})
	if err != nil || !res.Success {
		t.Fatalf("analyze_advertising: %+v err=%v", res, err)
	}
	toolReport, ok := res.Data.(Report)
	if !ok {
		t.Fatalf("tool data type %T", res.Data)
	}
	if len(toolReport.Evidence) == 0 {
		t.Fatal("tool report missing evidence")
	}
}

func TestReviewerRejectsFabricatedCause(t *testing.T) {
	report := Report{
		Summary:   "maybe weather",
		Metrics:   map[string]any{"ROAS": 1.0},
		Anomalies: []Anomaly{},
		RootCauses: []RootCause{{
			Statement:   "it rained on the ads",
			EvidenceIDs: []string{"missing"},
		}},
		Evidence:        []EvidenceItem{{ID: "e1", Description: "ROAS 1.0"}},
		Recommendations: []Recommendation{},
	}
	rev := Review(report)
	if rev.Approved {
		t.Fatal("must reject fabricated / uncited cause")
	}
}

func advertisingFixtureHandler(t *testing.T) func(name string, arguments map[string]any) (mcp.CallResult, error) {
	t.Helper()
	return func(name string, arguments map[string]any) (mcp.CallResult, error) {
		if name != "aggregate" {
			t.Fatalf("mcp tool = %s, want aggregate", name)
		}
		day := inspectDay(arguments)
		dim := inspectDimension(arguments)
		if dim == "" {
			return mcp.CallResult{StructuredContent: map[string]any{"aggregations": totalsFor(day)}}, nil
		}
		return mcp.CallResult{StructuredContent: map[string]any{
			"aggregations": map[string]any{
				"by_" + strings.ToLower(dim): map[string]any{"buckets": entityBuckets(day, dim)},
			},
		}}, nil
	}
}

func inspectDay(args map[string]any) string {
	gte := findGTE(args["query"])
	switch {
	case strings.HasPrefix(gte, "2026-08-24"):
		return "yesterday"
	case strings.HasPrefix(gte, "2026-08-23"):
		return "previous"
	case strings.HasPrefix(gte, "2026-08-17"):
		return "seven"
	default:
		return "yesterday"
	}
}

func inspectDimension(args map[string]any) string {
	aggs, _ := args["aggregations"].(map[string]any)
	for k := range aggs {
		switch strings.ToLower(k) {
		case "by_campaign":
			return "campaign"
		case "by_adgroup":
			return "adgroup"
		case "by_creative":
			return "creative"
		case "by_country":
			return "country"
		}
	}
	return ""
}

func findGTE(v any) string {
	switch t := v.(type) {
	case map[string]any:
		if g, ok := t["gte"]; ok {
			return strings.TrimSpace(strings.ReplaceAll(fmtStringish(g), " ", "T"))
		}
		for _, child := range t {
			if s := findGTE(child); s != "" {
				return s
			}
		}
	case []any:
		for _, child := range t {
			if s := findGTE(child); s != "" {
				return s
			}
		}
	}
	return ""
}

func fmtStringish(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return strings.TrimSpace(stringish(v))
}

func stringish(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return strings.Trim(string(raw), `"`)
}

func totalsFor(day string) map[string]any {
	switch day {
	case "previous":
		return metricAggs(100, 250, 10000, 200, 20)
	case "seven":
		return metricAggs(700, 1750, 70000, 1400, 140)
	default:
		return metricAggs(200, 180, 10000, 80, 8)
	}
}

func metricAggs(spend, revenue, impressions, clicks, conversions float64) map[string]any {
	return map[string]any{
		"m_spend":       map[string]any{"value": spend},
		"m_revenue":     map[string]any{"value": revenue},
		"m_impressions": map[string]any{"value": impressions},
		"m_clicks":      map[string]any{"value": clicks},
		"m_conversions": map[string]any{"value": conversions},
	}
}

func entityBuckets(day, dim string) []any {
	type row struct {
		key                                string
		spend, revenue, impr, clicks, conv float64
	}
	var rows []row
	bad := day == "yesterday" || day == ""
	switch strings.ToLower(dim) {
	case "campaign":
		if bad {
			rows = []row{
				{key: "camp_good", spend: 50, revenue: 150, impr: 4000, clicks: 40, conv: 6},
				{key: "camp_bad", spend: 150, revenue: 30, impr: 6000, clicks: 40, conv: 2},
			}
		} else {
			rows = []row{
				{key: "camp_good", spend: 50, revenue: 140, impr: 4000, clicks: 100, conv: 10},
				{key: "camp_bad", spend: 50, revenue: 110, impr: 6000, clicks: 100, conv: 10},
			}
		}
	case "adgroup":
		if bad {
			rows = []row{
				{key: "ag_ok", spend: 50, revenue: 140, impr: 4000, clicks: 40, conv: 6},
				{key: "ag_bad", spend: 150, revenue: 40, impr: 6000, clicks: 40, conv: 2},
			}
		} else {
			rows = []row{
				{key: "ag_ok", spend: 50, revenue: 130, impr: 4000, clicks: 100, conv: 10},
				{key: "ag_bad", spend: 50, revenue: 120, impr: 6000, clicks: 100, conv: 10},
			}
		}
	case "creative":
		if bad {
			rows = []row{
				{key: "cr_good", spend: 50, revenue: 150, impr: 4000, clicks: 40, conv: 6},
				{key: "cr_old", spend: 150, revenue: 30, impr: 6000, clicks: 40, conv: 2},
			}
		} else {
			rows = []row{
				{key: "cr_good", spend: 50, revenue: 140, impr: 4000, clicks: 100, conv: 10},
				{key: "cr_old", spend: 50, revenue: 110, impr: 6000, clicks: 100, conv: 10},
			}
		}
	default:
		if bad {
			rows = []row{
				{key: "US", spend: 80, revenue: 140, impr: 5000, clicks: 50, conv: 6},
				{key: "BR", spend: 120, revenue: 40, impr: 5000, clicks: 30, conv: 2},
			}
		} else {
			rows = []row{
				{key: "US", spend: 50, revenue: 130, impr: 5000, clicks: 100, conv: 10},
				{key: "BR", spend: 50, revenue: 120, impr: 5000, clicks: 100, conv: 10},
			}
		}
	}
	out := make([]any, 0, len(rows))
	for _, r := range rows {
		b := metricAggs(r.spend, r.revenue, r.impr, r.clicks, r.conv)
		b["key"] = r.key
		out = append(out, b)
	}
	return out
}

func mustMCP(t *testing.T, url string) mcp.Client {
	t.Helper()
	c, err := mcp.NewHTTPClient(url, time.Second, mcp.RetryPolicy{MaxAttempts: 3, Backoff: time.Millisecond}, logger.New("error", "json", io.Discard))
	if err != nil {
		t.Fatalf("mcp client: %v", err)
	}
	return c
}

func esCfg(url string) config.ElasticsearchConfig {
	readOnly := true
	return config.ElasticsearchConfig{
		MCPURL:                url,
		Timeout:               config.Duration(time.Second),
		Retry:                 config.RetryConfig{MaxAttempts: 3, Backoff: config.Duration(time.Millisecond)},
		MaxSize:               20,
		MaxAggregationBuckets: 20,
		AllowedTools:          []string{"search", "get", "count", "aggregate"},
		DeniedTools:           []string{"delete", "update", "index", "bulk"},
		ReadOnly:              &readOnly,
	}
}
