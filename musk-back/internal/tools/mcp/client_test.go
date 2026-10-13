package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"autonomous-agent/internal/logger"
	"autonomous-agent/internal/tools"
)

func testClient(t *testing.T, url string, retry RetryPolicy, timeout time.Duration) *HTTPClient {
	t.Helper()
	if timeout <= 0 {
		timeout = time.Second
	}
	if retry.MaxAttempts == 0 {
		retry.MaxAttempts = 2
	}
	if retry.Backoff == 0 {
		retry.Backoff = time.Millisecond
	}
	c, err := NewHTTPClient(url, timeout, retry, logger.New("error", "json", io.Discard))
	if err != nil {
		t.Fatalf("NewHTTPClient: %v", err)
	}
	return c
}

func TestHTTPClientDiscoverAndCall(t *testing.T) {
	srv := StartMockServer()
	defer srv.Close()

	c := testClient(t, srv.URL, RetryPolicy{MaxAttempts: 2, Backoff: time.Millisecond}, time.Second)
	tools, err := c.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools) < 4 {
		t.Fatalf("discovered %d tools", len(tools))
	}

	res, err := c.Call(context.Background(), "search", map[string]any{
		"index": "logs",
		"query": map[string]any{"match_all": map[string]any{}},
	})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if res.IsError {
		t.Fatal("search should succeed")
	}
	data := DataFromCall(res)
	raw, _ := json.Marshal(data)
	if !strings.Contains(strings.ToLower(string(raw)), "hits") {
		t.Fatalf("data = %#v", data)
	}
}

func TestHTTPClientRetry(t *testing.T) {
	srv := StartMockServer()
	defer srv.Close()
	srv.FailTimes = 2

	c := testClient(t, srv.URL, RetryPolicy{MaxAttempts: 4, Backoff: time.Millisecond}, time.Second)
	if _, err := c.ListTools(context.Background()); err != nil {
		t.Fatalf("ListTools after retry: %v", err)
	}
}

func TestHTTPClientTimeout(t *testing.T) {
	srv := StartMockServer()
	defer srv.Close()
	srv.Delay = 200 * time.Millisecond

	c := testClient(t, srv.URL, RetryPolicy{MaxAttempts: 1, Backoff: time.Millisecond}, 30*time.Millisecond)
	_, err := c.ListTools(context.Background())
	if err == nil {
		t.Fatal("expected timeout")
	}
}

func TestHTTPClientSSE(t *testing.T) {
	srv := StartMockServer()
	defer srv.Close()
	srv.SSE = true

	c := testClient(t, srv.URL, RetryPolicy{MaxAttempts: 1, Backoff: time.Millisecond}, time.Second)
	res, err := c.Call(context.Background(), "count", map[string]any{"index": "logs"})
	if err != nil {
		t.Fatalf("Call sse: %v", err)
	}
	if DataFromCall(res) == nil {
		t.Fatal("expected count data")
	}
}

func TestHTTPClientNormalizesWriteError(t *testing.T) {
	srv := StartMockServer()
	defer srv.Close()

	c := testClient(t, srv.URL, RetryPolicy{MaxAttempts: 1}, time.Second)
	_, err := c.Call(context.Background(), "delete", map[string]any{"index": "logs", "id": "1"})
	if !errors.Is(err, ErrToolFailed) {
		t.Fatalf("err = %v, want ErrToolFailed", err)
	}
	te := Normalize("delete", "tid", err)
	if te.Code != tools.CodeExecutionFailed {
		t.Fatalf("code = %s", te.Code)
	}
}

func TestAdapterExecute(t *testing.T) {
	srv := StartMockServer()
	defer srv.Close()
	c := testClient(t, srv.URL, RetryPolicy{MaxAttempts: 1}, time.Second)

	listed, err := c.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	var search ToolInfo
	for _, info := range listed {
		if info.Name == "search" {
			search = info
			break
		}
	}
	ad := NewAdapter(c, search)
	if ad.Name() != "search" {
		t.Fatalf("name = %s", ad.Name())
	}
	res, err := ad.Execute(context.Background(), map[string]any{"index": "logs", "query": map[string]any{"match_all": map[string]any{}}})
	if err != nil || !res.Success {
		t.Fatalf("Execute: res=%+v err=%v", res, err)
	}
}

func TestNewHTTPClientRejectsBadURL(t *testing.T) {
	if _, err := NewHTTPClient("not-a-url", time.Second, RetryPolicy{}, logger.New("error", "json", io.Discard)); !errors.Is(err, ErrInvalidURL) {
		t.Fatalf("err = %v", err)
	}
}
