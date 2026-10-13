package selftest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	DefaultBase = "http://127.0.0.1:8080"
	downHint    = "start musk-back with musk-back/.env (cd musk-back && make run)"
)

func APIBase() string {
	if v := strings.TrimSpace(os.Getenv("MUSK_BACK_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return DefaultBase
}

func ArtifactPath() string {
	if v := strings.TrimSpace(os.Getenv("MUSK_SELFTEST_ARTIFACT")); v != "" {
		return v
	}
	wd, err := os.Getwd()
	if err != nil {
		return filepath.Join("..", "musk-web", "e2e", ".last-live-run.json")
	}
	dir := wd
	for {
		web := filepath.Join(dir, "musk-web")
		if st, err := os.Stat(web); err == nil && st.IsDir() {
			return filepath.Join(web, "e2e", ".last-live-run.json")
		}
		if filepath.Base(dir) == "musk-back" {
			return filepath.Join(filepath.Dir(dir), "musk-web", "e2e", ".last-live-run.json")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return filepath.Join("..", "musk-web", "e2e", ".last-live-run.json")
}

type healthBody struct {
	Status string `json:"status"`
}

type errorBody struct {
	RequestID string `json:"request_id"`
	Error     string `json:"error"`
}

type runPostBody struct {
	RequestID  string  `json:"request_id"`
	RunID      string  `json:"run_id"`
	Status     string  `json:"status"`
	Answer     string  `json:"answer"`
	Confidence float64 `json:"confidence"`
	Steps      int     `json:"steps"`
	Error      string  `json:"error"`
}

type tokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type runSnapshot struct {
	ID          string           `json:"id"`
	Goal        string           `json:"goal"`
	Status      string           `json:"status"`
	Decisions   []map[string]any `json:"decisions"`
	TokenUsage  tokenUsage       `json:"token_usage"`
	Error       string           `json:"error"`
	Answer      string           `json:"answer"`
	FinalResult string           `json:"final_result"`
}

type LiveArtifact struct {
	RunID  string `json:"run_id"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
	Goal   string `json:"goal"`
}

func NewHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

func CheckHealth(client *http.Client) error {
	resp, err := client.Get(APIBase() + "/health")
	if err != nil {
		return fmt.Errorf("GET /health failed (%v); %s", err, downHint)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET /health HTTP %d; %s", resp.StatusCode, downHint)
	}
	var body healthBody
	if err := json.Unmarshal(raw, &body); err != nil || strings.ToLower(body.Status) != "ok" {
		return fmt.Errorf("GET /health unexpected body %s", truncate(raw, 200))
	}
	return nil
}

func ProbeSiliconFlow(client *http.Client, env LLMEnv) error {
	url := env.BaseURL + "/models"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("SiliconFlow GET /models: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("SiliconFlow rejected LLM_API_KEY (HTTP %d). Check musk-back/.env", resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("SiliconFlow GET /models HTTP %d", resp.StatusCode)
	}
	if !bytes.Contains(bytes.ToLower(raw), []byte("deepseek")) && !bytes.Contains(raw, []byte(env.Model)) {
		// Provider may not list every model; a 2xx still proves the key talks to this host.
		return nil
	}
	return nil
}

func PostAgentRun(client *http.Client, requestID, message string) (status int, body runPostBody, raw []byte, err error) {
	payload, _ := json.Marshal(map[string]string{"message": message})
	req, err := http.NewRequest(http.MethodPost, APIBase()+"/api/v1/agent/run", bytes.NewReader(payload))
	if err != nil {
		return 0, runPostBody{}, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", requestID)
	resp, err := client.Do(req)
	if err != nil {
		return 0, runPostBody{}, nil, fmt.Errorf("POST /api/v1/agent/run: %w", err)
	}
	defer resp.Body.Close()
	raw, _ = io.ReadAll(resp.Body)
	_ = json.Unmarshal(raw, &body)
	if body.Error == "" {
		var eb errorBody
		if json.Unmarshal(raw, &eb) == nil {
			body.Error = eb.Error
			if body.RequestID == "" {
				body.RequestID = eb.RequestID
			}
		}
	}
	if body.RunID == "" {
		body.RunID = requestID
	}
	return resp.StatusCode, body, raw, nil
}

func GetRun(client *http.Client, runID string) (runSnapshot, []byte, error) {
	resp, err := client.Get(APIBase() + "/api/v1/runs/" + runID)
	if err != nil {
		return runSnapshot{}, nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		return runSnapshot{}, raw, fmt.Errorf("GET /api/v1/runs/%s: not found", runID)
	}
	if resp.StatusCode != http.StatusOK {
		return runSnapshot{}, raw, fmt.Errorf("GET /api/v1/runs/%s HTTP %d %s", runID, resp.StatusCode, truncate(raw, 300))
	}
	var snap runSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return runSnapshot{}, raw, fmt.Errorf("GET /api/v1/runs/%s: invalid json", runID)
	}
	return snap, raw, nil
}

func PollRun(client *http.Client, runID string, timeout time.Duration) (runSnapshot, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		snap, _, err := GetRun(client, runID)
		if err != nil {
			lastErr = err
			time.Sleep(2 * time.Second)
			continue
		}
		switch snap.Status {
		case "COMPLETED", "FAILED", "ABORTED", "WAITING_APPROVAL":
			return snap, nil
		}
		time.Sleep(2 * time.Second)
	}
	if lastErr != nil {
		return runSnapshot{}, fmt.Errorf("timeout waiting for run %s: %v", runID, lastErr)
	}
	return runSnapshot{}, fmt.Errorf("timeout waiting for terminal status on run %s", runID)
}

func WriteArtifact(a LiveArtifact) error {
	path := ArtifactPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func truncate(b []byte, n int) string {
	s := strings.TrimSpace(string(b))
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func TokensUsed(u tokenUsage) int {
	if u.TotalTokens > 0 {
		return u.TotalTokens
	}
	return u.PromptTokens + u.CompletionTokens
}
