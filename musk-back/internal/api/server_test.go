package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"autonomous-agent/internal/agent/runtime"
	"autonomous-agent/internal/config"
	"autonomous-agent/internal/guardrail"
	"autonomous-agent/internal/logger"
	"autonomous-agent/internal/runstore"
	"autonomous-agent/internal/semantic"
	"autonomous-agent/internal/tools"
)

type stubRunner struct {
	answer     string
	err        error
	resume     string
	resumeErr  error
	approveErr error
	rejectErr  error
	run        *runstore.Run
}

func (s stubRunner) Run(_ context.Context, message string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	if message == "" {
		return "", runtime.ErrEmptyMessage
	}
	return s.answer, nil
}

func (s stubRunner) Resume(_ context.Context, _ string) (string, error) {
	if s.resumeErr != nil {
		return "", s.resumeErr
	}
	if s.resume != "" {
		return s.resume, nil
	}
	return s.answer, nil
}

func (s stubRunner) Approve(_ context.Context, _ string) error {
	return s.approveErr
}

func (s stubRunner) Reject(_ context.Context, _, _ string) error {
	return s.rejectErr
}

func (s stubRunner) GetRun(_ context.Context, id string) (*runstore.Run, error) {
	if s.run != nil {
		return s.run, nil
	}
	if s.answer != "" {
		return &runstore.Run{
			ID:         id,
			Status:     runstore.StatusCompleted,
			Answer:     s.answer,
			FinalResult: s.answer,
			Confidence: 0.9,
			Evidence:   []any{},
			Steps:      []runstore.Step{{ID: "s1", Index: 0}},
		}, nil
	}
	return nil, runstore.ErrNotFound
}

func (s stubRunner) ListAgents() []runtime.AgentInfo { return nil }
func (s stubRunner) ListTools() []tools.Info         { return nil }
func (s stubRunner) ListSemanticMetrics() []semantic.Metric {
	return nil
}
func (s stubRunner) ListSemanticDimensions() []semantic.Dimension {
	return nil
}

type blockingRunner struct{ stubRunner }

func (b blockingRunner) Run(ctx context.Context, _ string) (string, error) {
	<-ctx.Done()
	return "", ctx.Err()
}

func TestAgentRunHandlerTimeout(t *testing.T) {
	s := NewServer(config.ServerConfig{
		Addr:           ":0",
		HandlerTimeout: config.Duration(80 * time.Millisecond),
		WriteTimeout:   config.Duration(time.Second),
	}, blockingRunner{}, logger.New("error", "json", io.Discard))
	ts := httptest.NewServer(s.httpServer.Handler)
	defer ts.Close()

	start := time.Now()
	resp, err := http.Post(ts.URL+"/api/v1/agent/run", "application/json", strings.NewReader(`{"message":"hello"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if time.Since(start) > time.Second {
		t.Fatalf("handler timeout took %s", time.Since(start))
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 from TimeoutHandler", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "handler timeout") {
		t.Fatalf("body = %s", body)
	}
}

func newTestServer(t *testing.T, runner runtime.Runner) *httptest.Server {
	t.Helper()
	s := NewServer(config.ServerConfig{Addr: ":0"}, runner, logger.New("error", "json", io.Discard))
	return httptest.NewServer(s.httpServer.Handler)
}

func TestHealth(t *testing.T) {
	ts := newTestServer(t, stubRunner{answer: "ok"})
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	var body healthResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "ok" {
		t.Fatalf("status body = %+v", body)
	}
}

func TestAgentRun(t *testing.T) {
	ts := newTestServer(t, stubRunner{answer: "world"})
	defer ts.Close()

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/agent/run", bytes.NewBufferString(`{"message":"hello"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", "req-1")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	var body runResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Answer != "world" {
		t.Fatalf("answer = %q", body.Answer)
	}
	if body.RequestID != "req-1" {
		t.Fatalf("request_id = %q", body.RequestID)
	}
	if body.Status != "COMPLETED" {
		t.Fatalf("status = %q", body.Status)
	}
	if body.RunID == "" {
		t.Fatal("run_id is required")
	}
}

func TestAgentRunSurfacesSanitizedLLMError(t *testing.T) {
	ts := newTestServer(t, stubRunner{err: errors.New("generate plan: error, status code: 401, status: 401 Unauthorized, message: Incorrect API key provided: sk-live-secret123")})
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/v1/agent/run", "application/json", bytes.NewBufferString(`{"message":"Analyze why Meta ad ROI declined yesterday"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	var body errorResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body.Error, "LLM authentication failed") {
		t.Fatalf("error = %q", body.Error)
	}
	if strings.Contains(body.Error, "sk-live-secret123") {
		t.Fatalf("leaked key in %q", body.Error)
	}
	if body.RequestID == "" {
		t.Fatal("request_id is required")
	}
}

func TestAgentRunRejectsEmptyMessage(t *testing.T) {
	ts := newTestServer(t, stubRunner{})
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/v1/agent/run", "application/json", bytes.NewBufferString(`{"message":""}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestAgentRunWaitingApprovalAndResume(t *testing.T) {
	ts := newTestServer(t, stubRunner{
		err: &guardrail.WaitingError{
			RunID:      "run-1",
			ApprovalID: "appr-1",
			Tool:       "mutate_campaign",
			Risk:       guardrail.RiskHigh,
		},
		resume: "done after approval",
	})
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/v1/agent/run", "application/json", bytes.NewBufferString(`{"message":"mutate"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var waiting runResponse
	if err := json.NewDecoder(resp.Body).Decode(&waiting); err != nil {
		t.Fatal(err)
	}
	if waiting.Status != "WAITING_APPROVAL" || waiting.ApprovalID != "appr-1" {
		t.Fatalf("waiting = %+v", waiting)
	}

	approve, err := http.Post(ts.URL+"/api/v1/agent/approvals/appr-1/approve", "application/json", bytes.NewBufferString(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer approve.Body.Close()
	if approve.StatusCode != http.StatusOK {
		t.Fatalf("approve status = %d", approve.StatusCode)
	}

	resumeReq, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/agent/resume", bytes.NewBufferString(`{"run_id":"run-1"}`))
	if err != nil {
		t.Fatal(err)
	}
	resumeReq.Header.Set("Content-Type", "application/json")
	resume, err := http.DefaultClient.Do(resumeReq)
	if err != nil {
		t.Fatal(err)
	}
	defer resume.Body.Close()
	if resume.StatusCode != http.StatusOK {
		t.Fatalf("resume status = %d", resume.StatusCode)
	}
	var body runResponse
	if err := json.NewDecoder(resume.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Answer != "done after approval" {
		t.Fatalf("answer = %q", body.Answer)
	}
}

func TestGetRunAndCatalogs(t *testing.T) {
	ts := newTestServer(t, stubRunner{
		answer: "ok",
		run: &runstore.Run{
			ID:         "run-99",
			Goal:       "Analyze why Meta ad ROI declined yesterday",
			Status:     runstore.StatusCompleted,
			Answer:     "roi dropped",
			FinalResult: "roi dropped",
			Confidence: 0.93,
			Evidence:   []any{"cpc up"},
			Steps:      []runstore.Step{{ID: "s1", Index: 0}},
			Decisions:  []runstore.DecisionRecord{{Type: "FINISH", Reason: "done"}},
		},
	})
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/runs/run-99")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var run runstore.Run
	if err := json.NewDecoder(resp.Body).Decode(&run); err != nil {
		t.Fatal(err)
	}
	if run.Goal == "" || run.Status != runstore.StatusCompleted || run.FinalResult == "" {
		t.Fatalf("run = %+v", run)
	}

	for _, path := range []string{"/api/v1/agents", "/api/v1/tools", "/api/v1/semantic/metrics", "/api/v1/semantic/dimensions"} {
		r, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		if r.StatusCode != http.StatusOK {
			t.Fatalf("%s status = %d", path, r.StatusCode)
		}
		_ = r.Body.Close()
	}
}
