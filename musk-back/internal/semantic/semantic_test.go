package semantic

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"autonomous-agent/internal/config"
	"autonomous-agent/internal/logger"
	"autonomous-agent/internal/requestid"
	"autonomous-agent/internal/tools"
	"autonomous-agent/internal/tools/elasticsearch"
	"autonomous-agent/internal/tools/mcp"
)

func productionCatalog(t *testing.T) *Registry {
	t.Helper()
	dir := filepath.Join("..", "..", "configs", "semantic")
	reg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return reg
}

func TestLoadMetricsDimensions(t *testing.T) {
	reg := productionCatalog(t)
	if _, ok := reg.GetMetric("ROAS"); !ok {
		t.Fatal("expected ROAS")
	}
	if _, ok := reg.GetMetric("ROI"); !ok {
		t.Fatal("ROI should alias ROAS")
	}
	if _, ok := reg.GetDimension("Campaign"); !ok {
		t.Fatal("expected Campaign")
	}
	if _, ok := reg.GetDimension("Creative"); !ok {
		t.Fatal("expected Creative")
	}
	if len(reg.ListMetrics()) < 9 {
		t.Fatalf("metrics = %d", len(reg.ListMetrics()))
	}
	if len(reg.ListDimensions()) < 7 {
		t.Fatalf("dimensions = %d", len(reg.ListDimensions()))
	}
}

func TestParseYesterdayMetaROI(t *testing.T) {
	now := time.Date(2026, 8, 25, 15, 0, 0, 0, time.UTC)
	q, err := Parse("Why did Meta ROI decline yesterday", productionCatalog(t), now)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(q.Metrics) != 1 || q.Metrics[0] != "ROAS" {
		t.Fatalf("metrics = %v, want [ROAS]", q.Metrics)
	}
	if q.TimeRange.Label != "yesterday" {
		t.Fatalf("date label = %q, want yesterday", q.TimeRange.Label)
	}
	wantStart := time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	if !q.TimeRange.Start.Equal(wantStart) || !q.TimeRange.End.Equal(wantEnd) {
		t.Fatalf("time_range = %s .. %s", q.TimeRange.Start, q.TimeRange.End)
	}
	if len(q.Filters) != 1 || q.Filters[0].Dimension != "Channel" || q.Filters[0].Value != "Meta" {
		t.Fatalf("filters = %+v, want Channel=Meta", q.Filters)
	}
}

func TestROAS(t *testing.T) {
	catalog := productionCatalog(t)
	now := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	q := &Query{
		Metrics:   []string{"ROAS"},
		Filters:   []Filter{{Dimension: "Channel", Op: "eq", Value: "Meta"}},
		TimeRange: TimeRange{Start: now.AddDate(0, 0, -1), End: now, Label: "yesterday"},
	}
	if err := NewValidator(catalog, Limits{}).Validate(q); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	compiled, err := NewCompiler(catalog).Compile(q)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	assertNoScript(t, compiled)
	if !hasAggField(compiled.Aggregations, "spend") || !hasAggField(compiled.Aggregations, "revenue") {
		t.Fatalf("ROAS aggs = %#v", compiled.Aggregations)
	}
	if len(compiled.Formulas) != 1 || compiled.Formulas[0].Expression != "revenue / spend" {
		t.Fatalf("formulas = %#v", compiled.Formulas)
	}

	result := applyFormulas(compiled, catalog, q, map[string]any{
		"aggregations": map[string]any{
			"m_spend":   map[string]any{"value": 100.0},
			"m_revenue": map[string]any{"value": 250.0},
		},
	})
	v, ok := result.Metrics["ROAS"].(float64)
	if !ok || v != 2.5 {
		t.Fatalf("ROAS = %v", result.Metrics["ROAS"])
	}
}

func TestCTR(t *testing.T) {
	catalog := productionCatalog(t)
	q := &Query{Metrics: []string{"CTR"}}
	compiled, err := NewCompiler(catalog).Compile(q)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	assertNoScript(t, compiled)
	if !hasAggField(compiled.Aggregations, "clicks") || !hasAggField(compiled.Aggregations, "impressions") {
		t.Fatalf("CTR aggs = %#v", compiled.Aggregations)
	}
	result := applyFormulas(compiled, catalog, q, map[string]any{
		"aggregations": map[string]any{
			"m_clicks":      map[string]any{"value": 20.0},
			"m_impressions": map[string]any{"value": 1000.0},
		},
	})
	v, ok := result.Metrics["CTR"].(float64)
	if !ok || v != 0.02 {
		t.Fatalf("CTR = %v", result.Metrics["CTR"])
	}
}

func TestCampaign(t *testing.T) {
	catalog := productionCatalog(t)
	q := &Query{Metrics: []string{"ROAS"}, Dimensions: []string{"Campaign"}, Limit: 10}
	compiled, err := NewCompiler(catalog).Compile(q)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	bucket, ok := compiled.Aggregations["by_campaign"].(map[string]any)
	if !ok {
		t.Fatalf("missing by_campaign: %#v", compiled.Aggregations)
	}
	terms, _ := bucket["terms"].(map[string]any)
	if terms["field"] != "campaign_id" {
		t.Fatalf("campaign field = %v", terms["field"])
	}
	result := applyFormulas(compiled, catalog, q, map[string]any{
		"aggregations": map[string]any{
			"by_campaign": map[string]any{
				"buckets": []any{
					map[string]any{
						"key":       "cmp_1",
						"m_spend":   map[string]any{"value": 50.0},
						"m_revenue": map[string]any{"value": 150.0},
					},
				},
			},
		},
	})
	if len(result.Rows) != 1 || result.Rows[0]["Campaign"] != "cmp_1" {
		t.Fatalf("rows = %#v", result.Rows)
	}
	if result.Rows[0]["ROAS"] != 3.0 {
		t.Fatalf("campaign ROAS = %v", result.Rows[0]["ROAS"])
	}
}

func TestCreative(t *testing.T) {
	catalog := productionCatalog(t)
	q := &Query{Metrics: []string{"CTR"}, Dimensions: []string{"Creative"}, Limit: 5}
	compiled, err := NewCompiler(catalog).Compile(q)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	bucket, ok := compiled.Aggregations["by_creative"].(map[string]any)
	if !ok {
		t.Fatalf("missing by_creative: %#v", compiled.Aggregations)
	}
	terms, _ := bucket["terms"].(map[string]any)
	if terms["field"] != "creative_id" {
		t.Fatalf("creative field = %v", terms["field"])
	}
}

func TestDateRange(t *testing.T) {
	catalog := productionCatalog(t)
	start := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	q := &Query{
		Metrics:   []string{"Spend"},
		TimeRange: TimeRange{Start: start, End: end, Label: "last_7_days"},
	}
	if err := NewValidator(catalog, Limits{MaxTimeRange: 90 * 24 * time.Hour}).Validate(q); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	compiled, err := NewCompiler(catalog).Compile(q)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	filters := compiled.Query["bool"].(map[string]any)["filter"].([]any)
	found := false
	for _, f := range filters {
		obj := f.(map[string]any)
		if rng, ok := obj["range"].(map[string]any); ok {
			date := rng["date"].(map[string]any)
			if date["gte"] != start.Format(time.RFC3339) || date["lt"] != end.Format(time.RFC3339) {
				t.Fatalf("range = %#v", date)
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("date range missing: %#v", compiled.Query)
	}

	tooWide := &Query{
		Metrics:   []string{"Spend"},
		TimeRange: TimeRange{Start: start.AddDate(0, 0, -100), End: end},
	}
	if err := NewValidator(catalog, Limits{MaxTimeRange: 90 * 24 * time.Hour}).Validate(tooWide); err == nil {
		t.Fatal("expected max time range rejection")
	}
}

func TestFilter(t *testing.T) {
	catalog := productionCatalog(t)
	q := &Query{
		Metrics: []string{"Spend"},
		Filters: []Filter{{Dimension: "Channel", Op: "eq", Value: "Meta"}},
	}
	compiled, err := NewCompiler(catalog).Compile(q)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	filters := compiled.Query["bool"].(map[string]any)["filter"].([]any)
	found := false
	for _, f := range filters {
		obj := f.(map[string]any)
		if term, ok := obj["term"].(map[string]any); ok && term["channel"] == "Meta" {
			found = true
		}
	}
	if !found {
		t.Fatalf("channel filter missing: %#v", compiled.Query)
	}

	bad := &Query{Metrics: []string{"Spend"}, Filters: []Filter{{Dimension: "UnknownDim", Op: "eq", Value: "x"}}}
	if err := NewValidator(catalog, Limits{}).Validate(bad); !errors.Is(err, ErrUnknownFilter) {
		t.Fatalf("err = %v, want ErrUnknownFilter", err)
	}
}

func TestAggregation(t *testing.T) {
	catalog := productionCatalog(t)
	q := &Query{
		Metrics:    []string{"Spend", "Revenue", "ROAS"},
		Dimensions: []string{"Campaign", "Creative"},
		OrderBy:    []OrderBy{{Field: "ROAS", Direction: "desc"}},
		Limit:      8,
	}
	compiled, err := NewCompiler(catalog).Compile(q)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	assertNoScript(t, compiled)
	campaign, ok := compiled.Aggregations["by_campaign"].(map[string]any)
	if !ok {
		t.Fatalf("aggs = %#v", compiled.Aggregations)
	}
	inner, _ := campaign["aggs"].(map[string]any)
	creative, ok := inner["by_creative"].(map[string]any)
	if !ok {
		t.Fatalf("nested aggs = %#v", inner)
	}
	metrics, _ := creative["aggs"].(map[string]any)
	if !hasAggField(metrics, "spend") || !hasAggField(metrics, "revenue") {
		t.Fatalf("metric aggs = %#v", metrics)
	}
}

func TestValidatorRejectsUnknownMetricAndOversize(t *testing.T) {
	catalog := productionCatalog(t)
	v := NewValidator(catalog, Limits{MaxResultSize: 5, MaxTimeRange: 24 * time.Hour})
	if err := v.Validate(&Query{Metrics: []string{"NotAMetric"}}); !errors.Is(err, ErrUnknownMetric) {
		t.Fatalf("err = %v", err)
	}
	if err := v.Validate(&Query{Metrics: []string{"ROAS"}, Dimensions: []string{"NotADim"}}); !errors.Is(err, ErrUnknownDimension) {
		t.Fatalf("err = %v", err)
	}
	if err := v.Validate(&Query{Metrics: []string{"ROAS"}, Limit: 50}); err == nil {
		t.Fatal("expected max result size rejection")
	}
}

func TestQueryToolActUsesCompilerNotRawDSL(t *testing.T) {
	srv := mcp.StartMockServer()
	defer srv.Close()
	srv.Handler = func(name string, arguments map[string]any) (mcp.CallResult, error) {
		if name != "aggregate" {
			t.Fatalf("mcp tool = %s, want aggregate", name)
		}
		if _, ok := arguments["query"].(map[string]any); !ok {
			t.Fatalf("compiled query missing: %#v", arguments)
		}
		if _, ok := arguments["aggregations"].(map[string]any); !ok {
			t.Fatalf("compiled aggregations missing: %#v", arguments)
		}
		return mcp.CallResult{StructuredContent: map[string]any{
			"aggregations": map[string]any{
				"m_spend":   map[string]any{"value": 80.0},
				"m_revenue": map[string]any{"value": 200.0},
			},
		}}, nil
	}

	toolsReg := tools.NewRegistry(logger.New("error", "json", io.Discard), time.Second)
	if err := elasticsearch.Register(context.Background(), toolsReg, mustMCP(t, srv.URL), esCfg(srv.URL), logger.New("error", "json", io.Discard)); err != nil {
		t.Fatalf("register es: %v", err)
	}
	catalog := productionCatalog(t)
	tool := NewQueryTool(catalog, toolsReg, Limits{}, logger.New("error", "json", io.Discard))
	tool.now = func() time.Time { return time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC) }
	if err := toolsReg.Register(tool, tools.RiskMedium); err != nil {
		t.Fatalf("register semantic: %v", err)
	}

	res, err := toolsReg.Execute(requestid.With(context.Background(), "trace-semantic"), ToolName, map[string]any{
		"query": "Why did Meta ROI decline yesterday",
	})
	if err != nil || !res.Success {
		t.Fatalf("Execute: res=%+v err=%v", res, err)
	}
	if _, ok := res.Metadata["query_hash"]; !ok {
		t.Fatal("query_hash is required")
	}
	if _, ok := res.Metadata["query"]; ok {
		t.Fatal("query body must not be stored in metadata")
	}
	data, ok := res.Data.(Result)
	if !ok {
		t.Fatalf("data type = %T", res.Data)
	}
	v, _ := data.Metrics["ROAS"].(float64)
	if v != 2.5 {
		t.Fatalf("ROAS = %v data=%#v", data.Metrics["ROAS"], res.Data)
	}
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
		MaxAggregationBuckets: 10,
		AllowedTools:          []string{"search", "get", "count", "aggregate"},
		DeniedTools:           []string{"delete", "update", "index", "bulk"},
		ReadOnly:              &readOnly,
	}
}

func hasAggField(aggs map[string]any, field string) bool {
	for _, v := range aggs {
		obj, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if sum, ok := obj["sum"].(map[string]any); ok && sum["field"] == field {
			return true
		}
		if inner, ok := obj["aggs"].(map[string]any); ok && hasAggField(inner, field) {
			return true
		}
	}
	return false
}

func assertNoScript(t *testing.T, compiled *CompiledQuery) {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"query": compiled.Query, "aggregations": compiled.Aggregations})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(string(raw)), `"script"`) {
		t.Fatalf("compiled DSL must not contain script: %s", raw)
	}
}
