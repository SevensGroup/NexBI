package analyst

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"autonomous-agent/internal/config"
	"autonomous-agent/internal/logger"
	"autonomous-agent/internal/requestid"
	"autonomous-agent/internal/semantic"
	"autonomous-agent/internal/tools"
)

const (
	// ToolName is the registry name of the advertising analyst tool.
	ToolName = "analyze_advertising"
)

// Tool runs the autonomous analyst loop as a single CALL_TOOL.
type Tool struct {
	engine *Engine
	logger logger.Logger
}

var _ tools.Tool = (*Tool)(nil)

// NewTool wraps Engine as a Tool Registry entry.
func NewTool(engine *Engine, log logger.Logger) *Tool {
	if log == nil {
		log = logger.New("info", "json", os.Stdout)
	}
	return &Tool{engine: engine, logger: log}
}

func (t *Tool) Name() string { return ToolName }

func (t *Tool) Description() string {
	return "Run an Advertising Autonomous Analyst investigation for a goal such as " +
		"\"Why did Meta ad ROI decline yesterday\". Identifies channel/metric/time, fetches spend/revenue/" +
		"impressions/clicks/conversions, calculates ROAS/CTR/CPC/CVR/CPA, compares yesterday / " +
		"previous day / 7 day average, finds Campaign/AdGroup/Creative/Country anomalies, and " +
		"continues querying until evidence is sufficient. Returns a structured report. " +
		"Do not generate Elasticsearch Query DSL."
}

func (t *Tool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Advertising analysis goal, e.g. Why did Meta ad ROI decline yesterday",
			},
			"goal": map[string]any{
				"type":        "string",
				"description": "Alias of query",
			},
		},
	}
}

// SetNow pins the investigation clock (tests).
func (t *Tool) SetNow(now func() time.Time) {
	if t == nil || t.engine == nil || now == nil {
		return
	}
	t.engine.now = now
}

func (t *Tool) Execute(ctx context.Context, input map[string]any) (tools.ToolResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	traceID := requestid.From(ctx)
	if t == nil || t.engine == nil {
		return t.fail(traceID, tools.CodeExecutionFailed, "analyst engine is not configured", tools.ErrExecutionFailed)
	}
	if input == nil {
		input = map[string]any{}
	}
	goal := strings.TrimSpace(fmt.Sprint(input["query"]))
	if goal == "" || goal == "<nil>" {
		goal = strings.TrimSpace(fmt.Sprint(input["goal"]))
	}
	if goal == "" || goal == "<nil>" {
		for _, k := range []string{"nl", "question", "objective", "task", "message"} {
			goal = strings.TrimSpace(fmt.Sprint(input[k]))
			if goal != "" && goal != "<nil>" {
				break
			}
		}
	}
	if goal == "" || goal == "<nil>" {
		goal = "Analyze why Meta ad ROI declined yesterday"
	}

	report, state, err := t.engine.Analyze(ctx, goal)
	if err != nil && (errors.Is(err, ErrNoDataSource) || errors.Is(err, ErrNoSemanticTool)) {
		code := tools.CodeNotFound
		if errors.Is(err, ErrNoSemanticTool) {
			code = tools.CodeExecutionFailed
		}
		return t.fail(traceID, code, err.Error(), err)
	}
	if err != nil && len(report.Evidence) == 0 {
		return t.fail(traceID, tools.CodeExecutionFailed, err.Error(), err)
	}

	meta := map[string]any{
		"trace_id":            traceID,
		"tool":                ToolName,
		"queries":             0,
		"evidence_sufficient": false,
		"review_approved":     Review(report).Approved,
	}
	if state != nil {
		meta["queries"] = state.Queries
		check := state.Sufficient()
		meta["evidence_sufficient"] = check.Sufficient
		meta["channel"] = state.Intent.Channel
		meta["metric"] = state.Intent.Metric
		meta["time"] = state.Intent.TimeLabel
	}

	return tools.ToolResult{
		Success:  true,
		Data:     report,
		Metadata: meta,
	}, nil
}

func (t *Tool) fail(traceID, code, msg string, sentinel error) (tools.ToolResult, error) {
	te := &tools.ToolError{
		Code:    code,
		Tool:    ToolName,
		Message: msg,
		TraceID: traceID,
		Err:     sentinel,
	}
	return tools.ToolResult{
		Success: false,
		Error:   te.Error(),
		Metadata: map[string]any{
			"error_code": code,
			"trace_id":   traceID,
			"tool":       ToolName,
		},
	}, te
}

// RegisterFromConfig loads the semantic catalog and registers analyze_advertising.
func RegisterFromConfig(_ context.Context, reg tools.Registry, cfg config.SemanticConfig, log logger.Logger) error {
	if reg == nil {
		return fmt.Errorf("tool registry is required")
	}
	if _, ok := reg.Get(semantic.ToolName); !ok {
		return nil
	}
	dir := strings.TrimSpace(cfg.Dir)
	if dir == "" {
		return nil
	}
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("semantic dir %s: %w", dir, err)
	}
	catalog, err := semantic.Load(dir)
	if err != nil {
		return err
	}
	if log == nil {
		log = logger.New("info", "json", os.Stdout)
	}
	return reg.Register(NewTool(NewEngine(reg, catalog, log), log), tools.RiskMedium)
}

// MarshalReport is a stable JSON encoding of Report for HTTP / FINISH answers.
func MarshalReport(r Report) string {
	raw, err := json.Marshal(r)
	if err != nil {
		return ""
	}
	return string(raw)
}
