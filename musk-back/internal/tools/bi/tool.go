package bi

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"autonomous-agent/internal/board"
	"autonomous-agent/internal/tools"
)

const toolName = "bi_dashboard"

// Tool publishes dashboards in this process. The console opens them at /bi/<code>.
type Tool struct {
	boards *board.Store
}

func New(boards *board.Store) *Tool {
	return &Tool{boards: boards}
}

func Register(reg tools.Registry, boards *board.Store) error {
	if boards == nil {
		boards = board.New("")
	}
	return reg.Register(New(boards), tools.RiskLow)
}

func (t *Tool) Name() string { return toolName }

func (t *Tool) Description() string {
	return "Read advertising rows and publish a dashboard in this app. " +
		"action=query returns totals and campaign rows. Call it before creating a board. " +
		"action=create writes the board. name is the title, summary is your own conclusion from the query, metrics are KPI cards, columns and rows are the table. " +
		"Copy numbers from the query result. Do not send placeholder values or an empty table. Do not invent the code; the tool returns url like http://localhost:5173/bi/<code>."
}

func (t *Tool) InputSchema() any {
	return map[string]any{
		"type":     "object",
		"required": []string{"action"},
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": "query reads live ads rows; create writes a board from those rows; list returns existing boards",
				"enum":        []string{"query", "create", "list"},
			},
			"name": map[string]any{
				"type":        "string",
				"description": "Dashboard title shown in the BI app",
			},
			"summary": map[string]any{
				"type":        "string",
				"description": "One-line description rendered on the board",
			},
			"metrics": map[string]any{
				"type":        "array",
				"description": "KPI cards. Each item has label and value",
				"items":       map[string]any{"type": "object"},
			},
			"columns": map[string]any{
				"type":        "array",
				"description": "Table column names",
				"items":       map[string]any{"type": "string"},
			},
			"rows": map[string]any{
				"type":        "array",
				"description": "Table rows. Keys should match columns",
				"items":       map[string]any{"type": "object"},
			},
		},
	}
}

func (t *Tool) Execute(ctx context.Context, input map[string]any) (tools.ToolResult, error) {
	if t == nil || t.boards == nil {
		return fail("bi dashboard tool is not configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	action := strings.ToLower(strings.TrimSpace(fmt.Sprint(input["action"])))
	switch action {
	case "list":
		return ok(t.boards.List()), nil
	case "query":
		return ok(map[string]any{
			"source": "ads",
			"board":  t.boards.Ads(),
			"next":   "KPI cards must copy the totals object, including clicks and impressions. Do not re-sum the rows. Keep column keys and campaign values in English exactly as returned. Write the dashboard title and summary in English, then call action=create.",
		}), nil
	case "create":
		name := strings.TrimSpace(fmt.Sprint(input["name"]))
		if name == "" || name == "<nil>" {
			return fail("name is required")
		}
		if err := validateCreate(input); err != nil {
			return fail(err.Error())
		}
		metrics, columns, rows := board.Normalize(metricsFrom(input["metrics"]), stringsFrom(input["columns"]), rowsFrom(input["rows"]))
		created, err := t.boards.Create(name, stringField(input, "summary"), metrics, columns, rows)
		if err != nil {
			return fail(err.Error())
		}
		return ok(created), nil
	default:
		return fail(fmt.Sprintf("unsupported action %q", action))
	}
}

func ok(data any) tools.ToolResult {
	return tools.ToolResult{Success: true, Data: data, Metadata: map[string]any{"tool": toolName}}
}

func metricsFrom(raw any) []board.Metric {
	items, _ := raw.([]any)
	out := make([]board.Metric, 0, len(items))
	for _, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		label := strings.TrimSpace(fmt.Sprint(row["label"]))
		if label == "" || label == "<nil>" {
			continue
		}
		value := ""
		if row["value"] != nil {
			value = strings.TrimSpace(fmt.Sprint(row["value"]))
		}
		out = append(out, board.Metric{Label: label, Value: value})
	}
	return out
}

func stringsFrom(raw any) []string {
	switch items := raw.(type) {
	case []any:
		out := make([]string, 0, len(items))
		for _, item := range items {
			text := strings.TrimSpace(fmt.Sprint(item))
			if text != "" && text != "<nil>" {
				out = append(out, text)
			}
		}
		return out
	case []string:
		return items
	default:
		return nil
	}
}

func rowsFrom(raw any) []map[string]any {
	items, _ := raw.([]any)
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		row, ok := item.(map[string]any)
		if ok {
			out = append(out, row)
		}
	}
	return out
}

type createPayload struct {
	Summary string `json:"summary"`
	Metrics []struct {
		Label string `json:"label"`
		Value any    `json:"value"`
	} `json:"metrics"`
	Rows []map[string]any `json:"rows"`
}

func validateCreate(input map[string]any) error {
	raw, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("metrics and rows must be JSON")
	}
	var body createPayload
	if err := json.Unmarshal(raw, &body); err != nil {
		return fmt.Errorf("metrics and rows must be JSON")
	}
	if !concreteSummary(body.Summary) {
		return fmt.Errorf("summary must be your conclusion from the query, naming a channel or campaign and what the numbers show. Query the ads table first")
	}
	real := 0
	for _, item := range body.Metrics {
		if strings.TrimSpace(item.Label) == "" || placeholder(item.Value) {
			continue
		}
		real++
	}
	if real < 3 {
		return fmt.Errorf("metrics must contain at least 3 figures copied from the ads query. Placeholder values are rejected. Call action=query first")
	}
	if len(body.Rows) == 0 {
		return fmt.Errorf("rows must be copied from the ads query. An empty table is rejected. Call action=query first")
	}
	return nil
}

func concreteSummary(summary string) bool {
	text := strings.TrimSpace(summary)
	if len([]rune(text)) < 12 {
		return false
	}
	lower := strings.ToLower(text)
	hasSubject := strings.Contains(lower, "meta") || strings.Contains(lower, "tiktok") || strings.Contains(lower, "google") || strings.Contains(text, "渠道") || strings.Contains(text, "计划")
	hasJudgment := strings.ContainsAny(text, "0123456789") || strings.Contains(text, "最高") || strings.Contains(text, "最低") || strings.Contains(text, "高于") || strings.Contains(text, "低于") || strings.Contains(text, "领先") || strings.Contains(text, "下降") || strings.Contains(text, "上升")
	return hasSubject && hasJudgment
}

func placeholder(v any) bool {
	if v == nil {
		return true
	}
	s := strings.TrimSpace(strings.ToLower(fmt.Sprint(v)))
	switch s {
	case "", "<nil>", "-", "—", "--", "n/a", "na", "null", "none", "undefined", "待定", "暂无":
		return true
	default:
		return !strings.ContainsAny(s, "0123456789")
	}
}

func stringField(input map[string]any, key string) string {
	v, ok := input[key]
	if !ok || v == nil {
		return ""
	}
	s := strings.TrimSpace(fmt.Sprint(v))
	if s == "<nil>" {
		return ""
	}
	return s
}

func fail(msg string) (tools.ToolResult, error) {
	err := fmt.Errorf("%s", msg)
	return tools.ToolResult{Success: false, Error: msg, Metadata: map[string]any{"tool": toolName}}, err
}
