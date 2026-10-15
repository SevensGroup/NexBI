package bi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"autonomous-agent/internal/tools"
)

const toolName = "bi_dashboard"

// Config points the tool at the BI server agent API.
type Config struct {
	BaseURL string
	Token   string
}

// ConfigFromEnv reads BI_API_BASE_URL and BI_AGENT_TOKEN, with local defaults.
func ConfigFromEnv() Config {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("BI_API_BASE_URL")), "/")
	if base == "" {
		base = "http://127.0.0.1:8081"
	}
	token := strings.TrimSpace(os.Getenv("BI_AGENT_TOKEN"))
	if token == "" {
		token = "dev-bi-agent"
	}
	return Config{BaseURL: base, Token: token}
}

// Tool creates and lists dashboards in the Sevens BI app.
type Tool struct {
	cfg    Config
	client *http.Client
}

func New(cfg Config) *Tool {
	return &Tool{
		cfg:    cfg,
		client: &http.Client{Timeout: 20 * time.Second},
	}
}

func Register(reg tools.Registry, cfg Config) error {
	return reg.Register(New(cfg), tools.RiskLow)
}

func (t *Tool) Name() string { return toolName }

func (t *Tool) Description() string {
	return "Read advertising rows and publish a dashboard in the Sevens BI app. " +
		"action=query returns live totals and campaign rows from the ads table. Call it before creating a board. " +
		"action=create writes the board. name is the title, summary is your own conclusion from the query, metrics are KPI cards, columns and rows are the table. " +
		"Copy numbers from the query result. Do not send placeholder values or an empty table. Do not invent the id; use the id and url returned by this tool."
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
	if t == nil {
		return fail("bi dashboard tool is not configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	action := strings.ToLower(strings.TrimSpace(fmt.Sprint(input["action"])))
	switch action {
	case "list":
		return t.get(ctx, "/api/agent/dashboards")
	case "query":
		return t.queryAds(ctx)
	case "create":
		name := strings.TrimSpace(fmt.Sprint(input["name"]))
		if name == "" || name == "<nil>" {
			return fail("name is required")
		}
		if err := validateCreate(input); err != nil {
			return fail(err.Error())
		}
		body := map[string]any{
			"name":    name,
			"summary": stringField(input, "summary"),
		}
		if v, ok := input["metrics"]; ok && v != nil {
			body["metrics"] = v
		}
		if v, ok := input["columns"]; ok && v != nil {
			body["columns"] = v
		}
		if v, ok := input["rows"]; ok && v != nil {
			body["rows"] = v
		}
		return t.post(ctx, "/api/agent/dashboards", body)
	default:
		return fail(fmt.Sprintf("unsupported action %q", action))
	}
}

func (t *Tool) queryAds(ctx context.Context) (tools.ToolResult, error) {
	result, err := t.get(ctx, "/api/agent/ads")
	if err != nil || !result.Success {
		return result, err
	}
	return tools.ToolResult{
		Success: true,
		Data: map[string]any{
			"source": "sevens_bi.ads",
			"board":  result.Data,
			"next":   "KPI cards must copy the totals object, including clicks and impressions. Do not re-sum the rows. Keep column keys and campaign values in English exactly as returned. Write the dashboard title and summary in Chinese, then call action=create.",
		},
		Metadata: map[string]any{"tool": toolName},
	}, nil
}

func (t *Tool) get(ctx context.Context, path string) (tools.ToolResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.cfg.BaseURL+path, nil)
	if err != nil {
		return fail(err.Error())
	}
	return t.do(req)
}

func (t *Tool) post(ctx context.Context, path string, body any) (tools.ToolResult, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return fail(err.Error())
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.cfg.BaseURL+path, bytes.NewReader(raw))
	if err != nil {
		return fail(err.Error())
	}
	req.Header.Set("Content-Type", "application/json")
	return t.do(req)
}

func (t *Tool) do(req *http.Request) (tools.ToolResult, error) {
	req.Header.Set("X-BI-Agent-Token", t.cfg.Token)
	resp, err := t.client.Do(req)
	if err != nil {
		return fail(err.Error())
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fail(err.Error())
	}
	var env struct {
		Code    int             `json:"code"`
		Msg     string          `json:"msg"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(payload, &env); err != nil {
		return fail(fmt.Sprintf("bi api status %d: %s", resp.StatusCode, strings.TrimSpace(string(payload))))
	}
	if resp.StatusCode >= 300 || env.Code != 0 {
		msg := env.Msg
		if msg == "" {
			msg = env.Message
		}
		if msg == "" {
			msg = strings.TrimSpace(string(payload))
		}
		return fail(msg)
	}
	var data any
	if len(env.Data) > 0 {
		_ = json.Unmarshal(env.Data, &data)
	}
	return tools.ToolResult{
		Success: true,
		Data:    data,
		Metadata: map[string]any{
			"tool": toolName,
		},
	}, nil
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
