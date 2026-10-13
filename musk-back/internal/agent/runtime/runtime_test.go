package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"autonomous-agent/internal/agent/decision"
	"autonomous-agent/internal/agent/multi"
	"autonomous-agent/internal/agent/planner"
	"autonomous-agent/internal/agent/reflection"
	"autonomous-agent/internal/analyst"
	"autonomous-agent/internal/config"
	"autonomous-agent/internal/logger"
	"autonomous-agent/internal/memory"
	"autonomous-agent/internal/observability"
	"autonomous-agent/internal/requestid"
	"autonomous-agent/internal/runstore"
	"autonomous-agent/internal/semantic"
	"autonomous-agent/internal/tools"
	"autonomous-agent/internal/tools/elasticsearch"
	"autonomous-agent/internal/tools/mcp"
)

type stubProvider struct {
	model einomodel.BaseChatModel
	err   error
}

func (p stubProvider) NewChatModel(context.Context) (einomodel.BaseChatModel, error) {
	return p.model, p.err
}

type stubChatModel struct {
	replies []string
	i       int
}

func (s *stubChatModel) Generate(_ context.Context, _ []*schema.Message, _ ...einomodel.Option) (*schema.Message, error) {
	if s.i >= len(s.replies) {
		return schema.AssistantMessage(s.replies[len(s.replies)-1], nil), nil
	}
	reply := s.replies[s.i]
	s.i++
	return schema.AssistantMessage(reply, nil), nil
}

func (s *stubChatModel) Stream(_ context.Context, _ []*schema.Message, _ ...einomodel.Option) (*schema.StreamReader[*schema.Message], error) {
	reply := s.replies[0]
	sr, sw := schema.Pipe[*schema.Message](1)
	sw.Send(schema.AssistantMessage(reply, nil), nil)
	sw.Close()
	return sr, nil
}

func finishJSON(answer string) string {
	raw, _ := json.Marshal(decision.Decision{
		Type:       decision.DecisionFinish,
		Reason:     "done",
		Objective:  answer,
		Confidence: 0.9,
		Risk:       "low",
	})
	return string(raw)
}

func callToolJSON() string {
	return callCalculatorJSON(tools.OpSum, 2, 3)
}

func callCalculatorJSON(op string, a, b float64) string {
	raw, _ := json.Marshal(decision.Decision{
		Type:           decision.DecisionCallTool,
		Reason:         "need a calculation",
		Objective:      "compute " + op,
		Tool:           "calculator",
		Arguments:      map[string]any{"op": op, "a": a, "b": b},
		ExpectedResult: "a numeric value",
		Confidence:     0.6,
		Risk:           "low",
	})
	return string(raw)
}

func callToolJSONNamed(tool, q string) string {
	raw, _ := json.Marshal(decision.Decision{
		Type:           decision.DecisionCallTool,
		Reason:         "need a lookup",
		Objective:      "look up " + q,
		Tool:           tool,
		Arguments:      map[string]any{"q": q},
		ExpectedResult: "a greeting",
		Confidence:     0.6,
		Risk:           "low",
	})
	return string(raw)
}

type staticPlanner struct {
	tasks []planner.Task
	err   error
	n     int
}

func (s *staticPlanner) Plan(_ context.Context, goal string, _ decision.AgentState) (*planner.Plan, error) {
	s.n++
	if s.err != nil {
		return nil, s.err
	}
	tasks := make([]planner.Task, len(s.tasks))
	for i, t := range s.tasks {
		t.Dependencies = append([]string(nil), t.Dependencies...)
		t.Status = planner.StatusPending
		tasks[i] = t
	}
	if len(tasks) == 0 {
		tasks = []planner.Task{{
			ID:             "t1",
			Title:          "respond",
			Description:    "respond to the user",
			ExpectedOutput: "the answer",
			Priority:       1,
		}}
	}
	return &planner.Plan{Goal: goal, Tasks: tasks}, nil
}

func replanJSON() string {
	raw, _ := json.Marshal(decision.Decision{
		Type:       decision.DecisionReplan,
		Reason:     "need a better split",
		Objective:  "replan the goal",
		Confidence: 0.7,
		Risk:       "low",
	})
	return string(raw)
}

func newTestRuntime(t *testing.T, replies ...string) *Runtime {
	t.Helper()
	if len(replies) == 0 {
		replies = []string{finishJSON("hello back")}
	}
	rt, err := New(context.Background(), stubProvider{model: &stubChatModel{replies: replies}}, config.AgentConfig{
		Name:        "test-agent",
		Description: "test",
		Instruction: "You are a test agent.",
		MaxSteps:    8,
		MaxTasks:    8,
	}, config.ToolsConfig{Timeout: config.Duration(time.Second)}, config.SemanticConfig{}, config.MemoryConfig{SearchLimit: 8, ShortTermTTL: config.Duration(time.Hour)}, config.GuardrailConfig{}, logger.New("error", "json", io.Discard))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	rt.planner = &staticPlanner{}
	rt.reflector = reflection.NewHeuristic(logger.New("error", "json", io.Discard))
	t.Cleanup(func() { _ = rt.Close() })
	return rt
}

func TestRuntimeRun(t *testing.T) {
	got, err := newTestRuntime(t, finishJSON("hello back")).Run(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(got, "hello back") {
		t.Fatalf("answer = %q", got)
	}
}

func TestRuntimeContinueThenFinish(t *testing.T) {
	got, err := newTestRuntime(t, callToolJSON(), finishJSON("hello back")).Run(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != "hello back" {
		t.Fatalf("answer = %q", got)
	}
}

func TestRuntimeStringifiedToolArguments(t *testing.T) {
	call := `{
		"type":"CALL_TOOL",
		"reason":"need a calculation",
		"objective":"compute sum",
		"tool":"calculator",
		"arguments":"{\"op\":\"sum\",\"a\":2,\"b\":3}",
		"expected_result":"a numeric value",
		"confidence":0.6
	}`
	got, err := newTestRuntime(t, call, finishJSON("5")).Run(context.Background(), "what is 2 plus 3?")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != "5" {
		t.Fatalf("answer = %q", got)
	}
}

func TestRuntimeCalculatorActObserve(t *testing.T) {
	rt := newTestRuntime(t, finishJSON("unused"))
	state := decision.NewState("what is 2 + 3?", 8)
	d := &decision.Decision{
		Type:           decision.DecisionCallTool,
		Reason:         "need sum",
		Objective:      "add numbers",
		Tool:           "calculator",
		Arguments:      map[string]any{"op": tools.OpSum, "a": 2.0, "b": 3.0},
		ExpectedResult: "5",
		Confidence:     0.8,
	}
	_ = rt.engine.Apply(state, d)

	sched := planner.NewTaskScheduler(&planner.Plan{
		Goal: state.Goal,
		Tasks: []planner.Task{
			{ID: "t1", Title: "sum", Description: "add 2 and 3", ExpectedOutput: "5", Status: planner.StatusPending},
		},
	})
	ctx := requestid.With(context.Background(), "trace-runtime-calc")
	if err := rt.act(ctx, state, d, sched); err != nil {
		t.Fatalf("act: %v", err)
	}
	if !sched.AllComplete() {
		t.Fatal("successful calculator call should complete the ready task")
	}
	if len(state.ToolCalls) == 0 || !strings.Contains(state.ToolCalls[0].Result, "5") {
		t.Fatalf("tool result = %#v", state.ToolCalls)
	}
	last := state.Observations[len(state.Observations)-1]
	if last.Summary != "calculator result" || !strings.Contains(last.Evidence, "5") {
		t.Fatalf("observation = %+v", last)
	}
}

func TestRuntimeCalculatorLoop(t *testing.T) {
	rt := newTestRuntime(t, callCalculatorJSON(tools.OpSum, 2, 3), finishJSON("5"))
	got, err := rt.Run(context.Background(), "what is 2 plus 3?")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != "5" {
		t.Fatalf("answer = %q", got)
	}
}

func TestRuntimeAskUserWhenElasticsearchMissing(t *testing.T) {
	rt := newTestRuntime(t, callToolJSONNamed("semantic_query", "Analyze why Meta ad ROI declined yesterday"))
	if err := rt.registry.Register(missingESTool{}, tools.RiskMedium); err != nil {
		t.Fatalf("register: %v", err)
	}
	start := time.Now()
	got, err := rt.Run(context.Background(), "Analyze why Meta ad ROI declined yesterday")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("Run hung for %s", time.Since(start))
	}
	if !strings.Contains(got, "ELASTICSEARCH_MCP_URL") && !strings.Contains(got, "Elasticsearch MCP") {
		t.Fatalf("answer = %q", got)
	}
}

type missingESTool struct{}

func (missingESTool) Name() string { return "semantic_query" }
func (missingESTool) Description() string {
	return "stub advertising query"
}
func (missingESTool) InputSchema() any { return map[string]any{"type": "object"} }
func (missingESTool) Execute(_ context.Context, _ map[string]any) (tools.ToolResult, error) {
	msg := `TOOL_NOT_FOUND: tool "semantic_query": elasticsearch aggregate tool is not registered`
	return tools.ToolResult{Success: false, Error: msg}, errors.New(msg)
}

func TestRuntimeUnknownToolObserved(t *testing.T) {
	rt := newTestRuntime(t, callToolJSONNamed("nope", "x"), finishJSON("cannot compute"))
	got, err := rt.Run(context.Background(), "lookup x")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != "cannot compute" {
		t.Fatalf("answer = %q", got)
	}
}

func TestRuntimeRejectsEmptyMessage(t *testing.T) {
	rt := newTestRuntime(t, finishJSON("x"))
	if _, err := rt.Run(context.Background(), "   "); err != ErrEmptyMessage {
		t.Fatalf("err = %v, want ErrEmptyMessage", err)
	}
}

func TestRuntimePlanCalculatorComplete(t *testing.T) {
	rt := newTestRuntime(t, callCalculatorJSON(tools.OpSum, 1, 1), callCalculatorJSON(tools.OpMultiply, 2, 3), finishJSON("report ready"))
	rt.planner = &staticPlanner{tasks: []planner.Task{
		{ID: "a", Title: "sum", Description: "add 1 and 1", ExpectedOutput: "2", Priority: 2},
		{ID: "b", Title: "multiply", Description: "multiply 2 and 3", Dependencies: []string{"a"}, ExpectedOutput: "6"},
	}}

	got, err := rt.Run(context.Background(), "compute then finish")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != "report ready" {
		t.Fatalf("answer = %q", got)
	}
}

func TestRuntimeCircularPlan(t *testing.T) {
	rt := newTestRuntime(t, finishJSON("should not run"))
	rt.planner = &staticPlanner{tasks: []planner.Task{
		{ID: "a", Title: "A", Dependencies: []string{"b"}, ExpectedOutput: "A"},
		{ID: "b", Title: "B", Dependencies: []string{"a"}, ExpectedOutput: "B"},
	}}
	_, err := rt.Run(context.Background(), "cycle")
	if !errors.Is(err, planner.ErrCyclicDependency) {
		t.Fatalf("err = %v, want ErrCyclicDependency", err)
	}
}

func TestRuntimeReplan(t *testing.T) {
	rt := newTestRuntime(t, replanJSON(), finishJSON("replanned answer"))
	p := &staticPlanner{tasks: []planner.Task{
		{ID: "t1", Title: "respond", ExpectedOutput: "the answer"},
	}}
	rt.planner = p

	got, err := rt.Run(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != "replanned answer" {
		t.Fatalf("answer = %q", got)
	}
	if p.n != 2 {
		t.Fatalf("planner called %d times, want 2", p.n)
	}
}

func callESSearchJSON() string {
	raw, _ := json.Marshal(decision.Decision{
		Type:      decision.DecisionCallTool,
		Reason:    "need elasticsearch hits",
		Objective: "search logs",
		Tool:      elasticsearch.NameSearch,
		Arguments: map[string]any{
			"index": "logs",
			"query": map[string]any{"match_all": map[string]any{}},
			"size":  5,
		},
		ExpectedResult: "matching hits",
		Confidence:     0.7,
		Risk:           "medium",
	})
	return string(raw)
}

func reflectJSON() string {
	raw, _ := json.Marshal(decision.Decision{
		Type:       decision.DecisionReflect,
		Reason:     "hits are sufficient",
		Objective:  "summarize elasticsearch hits",
		Confidence: 0.8,
		Risk:       "low",
	})
	return string(raw)
}

func esTestConfig(mcpURL string) config.ElasticsearchConfig {
	readOnly := true
	return config.ElasticsearchConfig{
		MCPURL:                mcpURL,
		Timeout:               config.Duration(time.Second),
		Retry:                 config.RetryConfig{MaxAttempts: 3, Backoff: config.Duration(time.Millisecond)},
		MaxSize:               20,
		MaxAggregationBuckets: 10,
		AllowedTools:          []string{"search", "get", "count", "aggregate"},
		DeniedTools:           []string{"delete", "update", "index", "bulk"},
		ReadOnly:              &readOnly,
	}
}

func newTestRuntimeES(t *testing.T, mcpURL string, replies ...string) *Runtime {
	t.Helper()
	if len(replies) == 0 {
		replies = []string{finishJSON("hello back")}
	}
	rt, err := New(context.Background(), stubProvider{model: &stubChatModel{replies: replies}}, config.AgentConfig{
		Name:        "test-agent",
		Description: "test",
		Instruction: "You are a test agent.",
		MaxSteps:    8,
		MaxTasks:    8,
	}, config.ToolsConfig{
		Timeout:       config.Duration(time.Second),
		Elasticsearch: esTestConfig(mcpURL),
	}, config.SemanticConfig{}, config.MemoryConfig{SearchLimit: 8, ShortTermTTL: config.Duration(time.Hour)}, config.GuardrailConfig{}, logger.New("error", "json", io.Discard))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	rt.planner = &staticPlanner{}
	rt.reflector = reflection.NewHeuristic(logger.New("error", "json", io.Discard))
	t.Cleanup(func() { _ = rt.Close() })
	return rt
}

func TestRuntimeElasticsearchLoop(t *testing.T) {
	srv := mcp.StartMockServer()
	defer srv.Close()

	rt := newTestRuntimeES(t, srv.URL, callESSearchJSON(), reflectJSON(), finishJSON("one matching log"))
	got, err := rt.Run(context.Background(), "search logs for hello")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != "one matching log" {
		t.Fatalf("answer = %q", got)
	}
	if srv.CallCount("search") < 1 {
		t.Fatalf("mcp search calls = %v", srv.Calls)
	}
}

func TestRuntimeElasticsearchActObserve(t *testing.T) {
	srv := mcp.StartMockServer()
	defer srv.Close()

	rt := newTestRuntimeES(t, srv.URL, finishJSON("unused"))
	state := decision.NewState("search logs", 8)
	d := &decision.Decision{
		Type:      decision.DecisionCallTool,
		Reason:    "need hits",
		Objective: "search logs",
		Tool:      elasticsearch.NameSearch,
		Arguments: map[string]any{
			"index": "logs",
			"query": map[string]any{"match_all": map[string]any{}},
			"size":  5,
		},
		ExpectedResult: "hits",
		Confidence:     0.8,
	}
	_ = rt.engine.Apply(state, d)

	sched := planner.NewTaskScheduler(&planner.Plan{
		Goal: state.Goal,
		Tasks: []planner.Task{
			{ID: "t1", Title: "search", Description: "search logs", ExpectedOutput: "hits", Status: planner.StatusPending},
		},
	})
	ctx := requestid.With(context.Background(), "trace-runtime-es")
	if err := rt.act(ctx, state, d, sched); err != nil {
		t.Fatalf("act: %v", err)
	}
	if !sched.AllComplete() {
		t.Fatal("successful elasticsearch call should complete the ready task")
	}
	if len(state.Observations) == 0 {
		t.Fatal("expected observation")
	}
	last := state.Observations[len(state.Observations)-1]
	if last.Summary != elasticsearch.NameSearch+" result" {
		t.Fatalf("observation = %+v", last)
	}
	if !strings.Contains(last.Evidence, "hits") {
		t.Fatalf("evidence = %q", last.Evidence)
	}
}

func semanticTestDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("..", "..", "..", "configs", "semantic")
	return dir
}

func callAnalyzeJSON() string {
	raw, _ := json.Marshal(decision.Decision{
		Type:      decision.DecisionCallTool,
		Reason:    "need a full advertising investigation",
		Objective: "analyze Meta ROAS decline",
		Tool:      analyst.ToolName,
		Arguments: map[string]any{
			"query": "Why did Meta ad ROI decline yesterday",
		},
		ExpectedResult: "structured advertising report",
		Confidence:     0.8,
		Risk:           "medium",
	})
	return string(raw)
}

func TestRuntimeAdvertisingAnalystReport(t *testing.T) {
	srv := mcp.StartMockServer()
	defer srv.Close()
	srv.Handler = func(name string, arguments map[string]any) (mcp.CallResult, error) {
		if name != "aggregate" {
			return mcp.CallResult{}, fmt.Errorf("unexpected mcp tool %s", name)
		}
		gte := findRangeGTE(arguments["query"])
		aggs, _ := arguments["aggregations"].(map[string]any)
		dim := ""
		for k := range aggs {
			if strings.HasPrefix(strings.ToLower(k), "by_") {
				dim = strings.TrimPrefix(strings.ToLower(k), "by_")
				break
			}
		}
		day := "yesterday"
		switch {
		case strings.HasPrefix(gte, "2026-08-23"):
			day = "previous"
		case strings.HasPrefix(gte, "2026-08-17"):
			day = "seven"
		}
		if dim == "" {
			return mcp.CallResult{StructuredContent: map[string]any{"aggregations": runtimeMetricAggs(day)}}, nil
		}
		return mcp.CallResult{StructuredContent: map[string]any{
			"aggregations": map[string]any{
				"by_" + dim: map[string]any{"buckets": runtimeBuckets(day, dim)},
			},
		}}, nil
	}

	rt := newTestRuntimeSemantic(t, srv.URL, callAnalyzeJSON(), finishJSON("unused"))
	if tool, ok := rt.registry.Get(analyst.ToolName); ok {
		if at, ok := tool.(*analyst.Tool); ok {
			at.SetNow(func() time.Time { return time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC) })
		}
	} else {
		t.Fatal("analyze_advertising must be registered")
	}

	got, err := rt.Run(context.Background(), "Why did Meta ad ROI decline yesterday")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	report, ok := analyst.ParseReport(got)
	if !ok {
		t.Fatalf("answer is not a report: %s", got)
	}
	if strings.TrimSpace(report.Summary) == "" || len(report.Evidence) == 0 {
		t.Fatalf("report = %+v", report)
	}
	if fmt.Sprint(report.Metrics["channel"]) != "Meta" {
		t.Fatalf("metrics = %#v", report.Metrics)
	}
	if srv.CallCount("aggregate") < 5 {
		t.Fatalf("follow-up queries not run: %d", srv.CallCount("aggregate"))
	}
}

func findRangeGTE(v any) string {
	switch t := v.(type) {
	case map[string]any:
		if g, ok := t["gte"]; ok {
			return strings.TrimSpace(fmt.Sprint(g))
		}
		for _, child := range t {
			if s := findRangeGTE(child); s != "" {
				return s
			}
		}
	case []any:
		for _, child := range t {
			if s := findRangeGTE(child); s != "" {
				return s
			}
		}
	}
	return ""
}

func runtimeMetricAggs(day string) map[string]any {
	switch day {
	case "previous":
		return map[string]any{
			"m_spend": map[string]any{"value": 100.0}, "m_revenue": map[string]any{"value": 250.0},
			"m_impressions": map[string]any{"value": 10000.0}, "m_clicks": map[string]any{"value": 200.0}, "m_conversions": map[string]any{"value": 20.0},
		}
	case "seven":
		return map[string]any{
			"m_spend": map[string]any{"value": 700.0}, "m_revenue": map[string]any{"value": 1750.0},
			"m_impressions": map[string]any{"value": 70000.0}, "m_clicks": map[string]any{"value": 1400.0}, "m_conversions": map[string]any{"value": 140.0},
		}
	default:
		return map[string]any{
			"m_spend": map[string]any{"value": 200.0}, "m_revenue": map[string]any{"value": 180.0},
			"m_impressions": map[string]any{"value": 10000.0}, "m_clicks": map[string]any{"value": 80.0}, "m_conversions": map[string]any{"value": 8.0},
		}
	}
}

func runtimeBuckets(day, dim string) []any {
	bad := day == "yesterday" || day == ""
	type row struct {
		key                                string
		spend, revenue, impr, clicks, conv float64
	}
	var rows []row
	if bad {
		rows = []row{{key: dim + "_good", spend: 50, revenue: 150, impr: 4000, clicks: 40, conv: 6}, {key: dim + "_bad", spend: 150, revenue: 30, impr: 6000, clicks: 40, conv: 2}}
	} else {
		rows = []row{{key: dim + "_good", spend: 50, revenue: 140, impr: 4000, clicks: 100, conv: 10}, {key: dim + "_bad", spend: 50, revenue: 110, impr: 6000, clicks: 100, conv: 10}}
	}
	out := make([]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"key": r.key, "m_spend": map[string]any{"value": r.spend}, "m_revenue": map[string]any{"value": r.revenue},
			"m_impressions": map[string]any{"value": r.impr}, "m_clicks": map[string]any{"value": r.clicks}, "m_conversions": map[string]any{"value": r.conv},
		})
	}
	return out
}

func callSemanticJSON() string {
	raw, _ := json.Marshal(decision.Decision{
		Type:      decision.DecisionCallTool,
		Reason:    "need advertising metrics",
		Objective: "analyze Meta ROAS",
		Tool:      semantic.ToolName,
		Arguments: map[string]any{
			"query": "Why did Meta ROI decline yesterday",
		},
		ExpectedResult: "ROAS for Meta yesterday",
		Confidence:     0.7,
		Risk:           "medium",
	})
	return string(raw)
}

func newTestRuntimeSemantic(t *testing.T, mcpURL string, replies ...string) *Runtime {
	t.Helper()
	if len(replies) == 0 {
		replies = []string{finishJSON("hello back")}
	}
	rt, err := New(context.Background(), stubProvider{model: &stubChatModel{replies: replies}}, config.AgentConfig{
		Name:        "test-agent",
		Description: "test",
		Instruction: "You are a test agent.",
		MaxSteps:    8,
		MaxTasks:    8,
	}, config.ToolsConfig{
		Timeout:       config.Duration(15 * time.Second),
		Elasticsearch: esTestConfig(mcpURL),
	}, config.SemanticConfig{
		Dir:           semanticTestDir(t),
		MaxTimeRange:  config.Duration(90 * 24 * time.Hour),
		MaxResultSize: 100,
	}, config.MemoryConfig{SearchLimit: 8, ShortTermTTL: config.Duration(time.Hour)}, config.GuardrailConfig{}, logger.New("error", "json", io.Discard))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	rt.planner = &staticPlanner{}
	rt.reflector = reflection.NewHeuristic(logger.New("error", "json", io.Discard))
	t.Cleanup(func() { _ = rt.Close() })
	return rt
}

func TestRuntimeSemanticQueryLoop(t *testing.T) {
	srv := mcp.StartMockServer()
	defer srv.Close()

	rt := newTestRuntimeSemantic(t, srv.URL, callSemanticJSON(), reflectJSON(), finishJSON("Meta ROAS dropped"))
	got, err := rt.Run(context.Background(), "Why did Meta ROI decline yesterday")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != "Meta ROAS dropped" {
		t.Fatalf("answer = %q", got)
	}
	if srv.CallCount("aggregate") < 1 {
		t.Fatalf("mcp aggregate calls = %v", srv.Calls)
	}
}

func TestRuntimeSemanticHidesRawESTools(t *testing.T) {
	srv := mcp.StartMockServer()
	defer srv.Close()

	rt := newTestRuntimeSemantic(t, srv.URL, finishJSON("unused"))
	state := decision.NewState("Why did Meta ROI decline yesterday", 8)
	rt.attachTools(state)
	names := map[string]bool{}
	for _, info := range state.AvailableTools {
		names[info.Name] = true
		if semantic.IsRawSearchTool(info.Name) {
			t.Fatalf("raw elasticsearch tool exposed to agent: %s", info.Name)
		}
	}
	if !names[semantic.ToolName] {
		t.Fatalf("semantic_query missing, have %v", names)
	}
}

func TestRuntimeSemanticActObserve(t *testing.T) {
	srv := mcp.StartMockServer()
	defer srv.Close()

	rt := newTestRuntimeSemantic(t, srv.URL, finishJSON("unused"))
	state := decision.NewState("Why did Meta ROI decline yesterday", 8)
	d := &decision.Decision{
		Type:      decision.DecisionCallTool,
		Reason:    "need ROAS",
		Objective: "analyze Meta ROAS",
		Tool:      semantic.ToolName,
		Arguments: map[string]any{
			"query": "Why did Meta ROI decline yesterday",
		},
		ExpectedResult: "ROAS",
		Confidence:     0.8,
	}
	_ = rt.engine.Apply(state, d)
	sched := planner.NewTaskScheduler(&planner.Plan{
		Goal: state.Goal,
		Tasks: []planner.Task{
			{ID: "t1", Title: "query", Description: "semantic query", ExpectedOutput: "metrics", Status: planner.StatusPending},
		},
	})
	ctx := requestid.With(context.Background(), "trace-runtime-semantic")
	if err := rt.act(ctx, state, d, sched); err != nil {
		t.Fatalf("act: %v", err)
	}
	if !sched.AllComplete() {
		t.Fatal("successful semantic_query should complete the ready task")
	}
	last := state.Observations[len(state.Observations)-1]
	if last.Summary != semantic.ToolName+" result" {
		t.Fatalf("observation = %+v", last)
	}
}

func TestRuntimePersistsShortTermAndTaskMemory(t *testing.T) {
	rt := newTestRuntime(t, callCalculatorJSON(tools.OpSum, 2, 3), finishJSON("5"))
	ctx := requestid.With(context.Background(), "run-mem-1")
	got, err := rt.Run(ctx, "what is 2 plus 3?")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != "5" {
		t.Fatalf("answer = %q", got)
	}

	goals, err := rt.memory.Search(ctx, memory.Query{RunID: "run-mem-1", Layer: memory.LayerShortTerm, Kind: memory.KindGoal, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if goals.Total < 1 {
		t.Fatal("expected Goal in short-term memory")
	}
	plans, err := rt.memory.Search(ctx, memory.Query{RunID: "run-mem-1", Layer: memory.LayerShortTerm, Kind: memory.KindPlan, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if plans.Total < 1 {
		t.Fatal("expected Plan in short-term memory")
	}
	decisions, err := rt.memory.Search(ctx, memory.Query{RunID: "run-mem-1", Layer: memory.LayerShortTerm, Kind: memory.KindDecision, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if decisions.Total < 1 {
		t.Fatal("expected Decision in short-term memory")
	}
	calls, err := rt.memory.Search(ctx, memory.Query{RunID: "run-mem-1", Layer: memory.LayerShortTerm, Kind: memory.KindToolCall, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if calls.Total < 1 {
		t.Fatal("expected ToolCall in short-term memory")
	}
	obs, err := rt.memory.Search(ctx, memory.Query{RunID: "run-mem-1", Layer: memory.LayerShortTerm, Kind: memory.KindObservation, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if obs.Total < 1 {
		t.Fatal("expected Observation in short-term memory")
	}
	tasks, err := rt.memory.Search(ctx, memory.Query{RunID: "run-mem-1", Layer: memory.LayerTask, Kind: memory.KindTask, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if tasks.Total < 1 {
		t.Fatal("expected Task in task memory")
	}
	success, err := rt.memory.Search(ctx, memory.Query{RunID: "run-mem-1", Layer: memory.LayerTask, Kind: memory.KindSuccess, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if success.Total < 1 {
		t.Fatal("expected SuccessfulActions in task memory")
	}
	conclusions, err := rt.memory.Search(ctx, memory.Query{RunID: "run-mem-1", Layer: memory.LayerTask, Kind: memory.KindConclusion, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if conclusions.Total < 1 {
		t.Fatal("expected Conclusion in task memory")
	}
}

type failingMemory struct{}

func (failingMemory) Save(context.Context, memory.Record) error { return memory.ErrClosed }
func (failingMemory) Get(context.Context, string) (*memory.Record, error) {
	return nil, memory.ErrClosed
}
func (failingMemory) Search(context.Context, memory.Query) (memory.SearchResult, error) {
	return memory.SearchResult{}, memory.ErrClosed
}
func (failingMemory) Delete(context.Context, string) error { return memory.ErrClosed }
func (failingMemory) Close() error                         { return nil }

func TestRuntimeMemoryWriteFailureDoesNotAbort(t *testing.T) {
	rt := newTestRuntime(t, finishJSON("still works"))
	rt.memory = failingMemory{}
	rt.remember = memory.NewRecorder(failingMemory{}, logger.New("error", "json", io.Discard))

	got, err := rt.Run(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != "still works" {
		t.Fatalf("answer = %q", got)
	}
}

func TestRuntimeExposesMemoryTools(t *testing.T) {
	rt := newTestRuntime(t, finishJSON("unused"))
	state := decision.NewState("hello", 8)
	rt.attachTools(state)
	names := map[string]bool{}
	for _, info := range state.AvailableTools {
		names[info.Name] = true
	}
	if !names[memory.ToolSearch] || !names[memory.ToolSave] {
		t.Fatalf("memory tools missing: %v", names)
	}
}

type countingReflector struct {
	inner reflection.Reflector
	n     int
	last  *reflection.Reflection
}

func (c *countingReflector) Reflect(ctx context.Context, state *decision.AgentState, obs decision.Observation) (*reflection.Reflection, error) {
	c.n++
	r, err := c.inner.Reflect(ctx, state, obs)
	c.last = r
	return r, err
}

type fixedReflector struct {
	r *reflection.Reflection
}

func (f fixedReflector) Reflect(context.Context, *decision.AgentState, decision.Observation) (*reflection.Reflection, error) {
	return f.r, nil
}

type scriptedDecider struct {
	decisions []*decision.Decision
	i         int
	policy    *decision.Policy
}

func (s *scriptedDecider) Decide(_ context.Context, state *decision.AgentState) (*decision.Decision, error) {
	if s.policy == nil {
		s.policy = decision.NewPolicy()
	}
	if s.policy.MaxStepsReached(state) {
		return &decision.Decision{
			Type:       decision.DecisionAbort,
			Reason:     "maximum steps reached",
			Objective:  "maximum steps reached",
			Confidence: 1,
			Risk:       "high",
		}, nil
	}
	if len(s.decisions) == 0 {
		return &decision.Decision{Type: decision.DecisionFinish, Reason: "empty", Objective: "empty", Confidence: 1}, nil
	}
	if s.i >= len(s.decisions) {
		return s.decisions[len(s.decisions)-1], nil
	}
	d := s.decisions[s.i]
	s.i++
	return d, nil
}

func (s *scriptedDecider) Apply(state *decision.AgentState, d *decision.Decision) decision.Outcome {
	if s.policy == nil {
		s.policy = decision.NewPolicy()
	}
	return s.policy.Apply(state, d)
}

func TestRuntimeReflectsAfterTool(t *testing.T) {
	rt := newTestRuntime(t, callCalculatorJSON(tools.OpSum, 2, 3), finishJSON("5"))
	count := &countingReflector{inner: reflection.NewHeuristic(logger.New("error", "json", io.Discard))}
	rt.reflector = count

	got, err := rt.Run(context.Background(), "what is 2 plus 3?")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != "5" {
		t.Fatalf("answer = %q", got)
	}
	if count.n < 1 {
		t.Fatal("expected reflection after tool observation")
	}
	if count.last == nil || count.last.NextAction != reflection.NextContinue {
		t.Fatalf("reflection = %+v", count.last)
	}
}

func TestRuntimeReflectionFinish(t *testing.T) {
	rt := newTestRuntime(t, callCalculatorJSON(tools.OpSum, 2, 3), finishJSON("should not reach"))
	rt.reflector = fixedReflector{r: &reflection.Reflection{
		GoalSatisfied:      true,
		EvidenceSufficient: true,
		ResultCorrect:      true,
		MissingInformation: []string{},
		Errors:             []string{},
		NextAction:         reflection.NextFinish,
		Confidence:         0.9,
	}}

	got, err := rt.Run(context.Background(), "what is 2 plus 3?")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(got, "5") {
		t.Fatalf("answer = %q, want calculator evidence", got)
	}
}

func TestRuntimeReflectionReplan(t *testing.T) {
	rt := newTestRuntime(t, callCalculatorJSON(tools.OpSum, 2, 3), finishJSON("replanned after reflection"))
	p := &staticPlanner{tasks: []planner.Task{
		{ID: "t1", Title: "sum", Description: "add numbers", ExpectedOutput: "5", Priority: 1},
	}}
	rt.planner = p
	rt.reflector = fixedReflector{r: &reflection.Reflection{
		GoalSatisfied:      false,
		EvidenceSufficient: false,
		ResultCorrect:      false,
		Errors:             []string{"plan cannot produce the expected output"},
		NextAction:         reflection.NextReplan,
		Confidence:         0.8,
	}}

	got, err := rt.Run(context.Background(), "what is 2 plus 3?")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != "replanned after reflection" {
		t.Fatalf("answer = %q", got)
	}
	if p.n < 2 {
		t.Fatalf("planner called %d times, want at least 2 (initial + reflection REPLAN)", p.n)
	}
}

func TestRuntimeReflectionAskUser(t *testing.T) {
	rt := newTestRuntime(t, callCalculatorJSON(tools.OpSum, 2, 3), finishJSON("should not reach"))
	rt.reflector = fixedReflector{r: &reflection.Reflection{
		GoalSatisfied:      false,
		EvidenceSufficient: false,
		ResultCorrect:      false,
		MissingInformation: []string{"which date range?"},
		NextAction:         reflection.NextAskUser,
		Confidence:         0.7,
	}}

	got, err := rt.Run(context.Background(), "what is 2 plus 3?")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != "which date range?" {
		t.Fatalf("answer = %q", got)
	}
}

func TestRuntimeReflectionLoopReplan(t *testing.T) {
	same := &decision.Decision{
		Type:           decision.DecisionCallTool,
		Reason:         "need a calculation",
		Objective:      "compute sum",
		Tool:           "calculator",
		Arguments:      map[string]any{"op": tools.OpSum, "a": 1.0, "b": 1.0},
		ExpectedResult: "a numeric value",
		Confidence:     0.6,
		Risk:           "low",
	}
	finish := &decision.Decision{
		Type:       decision.DecisionFinish,
		Reason:     "escaped the loop",
		Objective:  "escaped loop",
		Confidence: 0.9,
		Risk:       "low",
	}

	rt := newTestRuntime(t, finishJSON("unused"))
	p := &staticPlanner{tasks: []planner.Task{
		{ID: "t1", Title: "sum", Description: "add numbers", ExpectedOutput: "2", Priority: 1},
	}}
	rt.planner = p
	rt.engine = &scriptedDecider{decisions: []*decision.Decision{same, same, same, finish}}
	rt.reflector = reflection.NewHeuristic(logger.New("error", "json", io.Discard))

	got, err := rt.Run(context.Background(), "sum then finish")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != "escaped loop" {
		t.Fatalf("answer = %q", got)
	}
	if p.n < 2 {
		t.Fatalf("planner called %d times, want at least 2 (initial + forced REPLAN)", p.n)
	}
}

func TestRuntimePersistsReflectionObservation(t *testing.T) {
	rt := newTestRuntime(t, callCalculatorJSON(tools.OpSum, 2, 3), finishJSON("5"))
	ctx := requestid.With(context.Background(), "run-reflect-1")
	if _, err := rt.Run(ctx, "what is 2 plus 3?"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	obs, err := rt.memory.Search(ctx, memory.Query{RunID: "run-reflect-1", Layer: memory.LayerShortTerm, Kind: memory.KindObservation, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, rec := range obs.Records {
		raw, _ := json.Marshal(rec.Content)
		if strings.Contains(string(raw), "reflection:") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected reflection observation in short-term memory")
	}
}

func delegateJSON(agent, task string) string {
	raw, _ := json.Marshal(decision.Decision{
		Type:       decision.DecisionDelegateAgent,
		Reason:     "assign specialist",
		Objective:  task,
		Arguments:  map[string]any{"agent": agent, "task": task},
		Confidence: 0.7,
		Risk:       "low",
	})
	return string(raw)
}

type stubDelegator struct {
	outs  []*multi.AgentOutput
	i     int
	calls []multi.DelegateRequest
}

func (s *stubDelegator) Delegate(_ context.Context, req multi.DelegateRequest) (*multi.AgentOutput, error) {
	s.calls = append(s.calls, req)
	if s.i >= len(s.outs) {
		return nil, fmt.Errorf("unexpected delegate %s", req.Agent)
	}
	out := s.outs[s.i]
	s.i++
	return out, nil
}

func (s *stubDelegator) Listings() []decision.AgentListing {
	return multi.DefaultListings(6)
}

func TestRuntimeDelegateAnalystThenFinish(t *testing.T) {
	rt := newTestRuntime(t, delegateJSON("analyst", "Analyze why Meta ROAS declined yesterday"), finishJSON("Meta ROAS dropped because spend rose"))
	team := &stubDelegator{outs: []*multi.AgentOutput{{
		Agent:      "analyst",
		Task:       "Analyze why Meta ROAS declined yesterday",
		Conclusion: "spend rose, revenue flat",
		Evidence:   "semantic_query buckets",
		Confidence: 0.8,
	}}}
	rt.team = team
	p := &staticPlanner{tasks: []planner.Task{{ID: "t1", Title: "analyze", ExpectedOutput: "cause"}}}
	rt.planner = p

	got, err := rt.Run(context.Background(), "Why did Meta ROI decline yesterday")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != "Meta ROAS dropped because spend rose" {
		t.Fatalf("answer = %q", got)
	}
	if len(team.calls) != 1 || team.calls[0].Agent != "analyst" {
		t.Fatalf("calls = %#v", team.calls)
	}
}

func TestRuntimeReviewerRejectForcesReplan(t *testing.T) {
	rt := newTestRuntime(t,
		delegateJSON("analyst", "Analyze why Meta ROAS declined yesterday"),
		delegateJSON("reviewer", "verify conclusions"),
		finishJSON("replanned after rejection"),
	)
	team := &stubDelegator{outs: []*multi.AgentOutput{
		{
			Agent:      "analyst",
			Task:       "Analyze why Meta ROAS declined yesterday",
			Conclusion: "maybe creative fatigue",
			Evidence:   "",
			Confidence: 0.4,
		},
		{
			Agent:  "reviewer",
			Task:   "verify conclusions",
			Review: &multi.ReviewResult{Approved: false, Reason: "insufficient evidence"},
		},
	}}
	rt.team = team
	p := &staticPlanner{tasks: []planner.Task{{ID: "t1", Title: "analyze", ExpectedOutput: "cause"}}}
	rt.planner = p

	got, err := rt.Run(context.Background(), "Why did Meta ROI decline yesterday")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != "replanned after rejection" {
		t.Fatalf("answer = %q", got)
	}
	if p.n != 2 {
		t.Fatalf("planner called %d times, want 2 (initial + reviewer REPLAN)", p.n)
	}
	if len(team.calls) != 2 {
		t.Fatalf("delegations = %d, want 2", len(team.calls))
	}
	if team.calls[1].Agent != "reviewer" {
		t.Fatalf("second call = %#v", team.calls[1])
	}
}

func TestRuntimeUnknownAgentContinues(t *testing.T) {
	rt := newTestRuntime(t, delegateJSON("writer", "draft copy"), finishJSON("cannot delegate writer"))
	p := &staticPlanner{}
	rt.planner = p

	got, err := rt.Run(context.Background(), "write an ad")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != "cannot delegate writer" {
		t.Fatalf("answer = %q", got)
	}
}

func TestRuntimeAttachAgents(t *testing.T) {
	rt := newTestRuntime(t, finishJSON("unused"))
	state := decision.NewState("goal", 8)
	rt.attachAgents(state)
	if len(state.AvailableAgents) != 3 {
		t.Fatalf("available agents = %#v", state.AvailableAgents)
	}
	rt.role = string(multi.RoleAnalyst)
	rt.attachAgents(state)
	if len(state.AvailableAgents) != 0 {
		t.Fatal("specialists must not see available_agents")
	}
}

func TestRuntimePersistsRunAndListsCatalog(t *testing.T) {
	rt := newTestRuntime(t, finishJSON("hello back"))
	ctx := requestid.With(context.Background(), "run-persist-1")
	got, err := rt.Run(ctx, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello back" {
		t.Fatalf("answer = %q", got)
	}
	run, err := rt.GetRun(ctx, "run-persist-1")
	if err != nil {
		t.Fatal(err)
	}
	if run.Goal != "hello" || run.Status != runstore.StatusCompleted || run.FinalResult == "" {
		t.Fatalf("run = %+v", run)
	}
	if len(run.Steps) == 0 || len(run.Decisions) == 0 {
		t.Fatalf("expected steps and decisions, got %+v", run)
	}
	if len(rt.ListAgents()) < 4 {
		t.Fatalf("agents = %#v", rt.ListAgents())
	}
	if len(rt.ListTools()) == 0 {
		t.Fatal("expected tools")
	}
}

func TestGetRunReturnsRecordedTokenUsage(t *testing.T) {
	rt := newTestRuntime(t, finishJSON("hello back"))
	ctx := context.Background()
	runID := "run-token-usage-1"
	rt.createRun(ctx, runID, "count tokens")

	usage := observability.TokenUsage{PromptTokens: 3885, CompletionTokens: 353, TotalTokens: 4238}
	rt.telemetry.RecordLLM(observability.WithRunID(ctx, runID), usage, "stub-model")

	run, err := rt.GetRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if run.TokenUsage.PromptTokens != 3885 || run.TokenUsage.CompletionTokens != 353 || run.TokenUsage.TotalTokens != 4238 {
		t.Fatalf("token_usage = %+v, want prompt=3885 completion=353 total=4238", run.TokenUsage)
	}
	if run.CostUSD <= 0 {
		t.Fatalf("cost_usd = %v, want > 0", run.CostUSD)
	}
}
