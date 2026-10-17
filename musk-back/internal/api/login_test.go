package api

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"autonomous-agent/internal/config"
	"autonomous-agent/internal/logger"
	"autonomous-agent/internal/loginlog"
)

func TestLoginRecordsAttemptWithoutPassword(t *testing.T) {
	t.Setenv("BI_DATABASE_DSN", "")
	logs, err := loginlog.Open(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	s := NewServer(config.ServerConfig{Addr: ":0"}, stubRunner{}, logger.New("info", "json", &buf), nil, logs)
	ts := httptest.NewServer(s.httpServer.Handler)
	defer ts.Close()

	denied := postLogin(t, ts.URL, `{"account":"demo","password":"nope"}`, map[string]string{
		"User-Agent":      "musk-test",
		"X-Forwarded-For": "198.51.100.4, 10.0.0.1",
	})
	if denied != http.StatusUnauthorized {
		t.Fatalf("denied status = %d", denied)
	}

	ok := postLogin(t, ts.URL, `{"account":" demo ","password":"musk888"}`, map[string]string{
		"User-Agent":       "musk-test",
		"CF-Connecting-IP": "203.0.113.9",
	})
	if ok != http.StatusOK {
		t.Fatalf("ok status = %d", ok)
	}

	entries, err := logs.Entries(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %+v", entries)
	}
	if entries[0].Success || entries[0].IP != "198.51.100.4" || entries[0].Account != "demo" {
		t.Fatalf("failed entry = %+v", entries[0])
	}
	if !entries[1].Success || entries[1].IP != "203.0.113.9" || entries[1].UserAgent != "musk-test" {
		t.Fatalf("success entry = %+v", entries[1])
	}
	logged := buf.String()
	if strings.Contains(logged, "musk888") || strings.Contains(logged, "nope") {
		t.Fatalf("password leaked into log: %s", logged)
	}
	if !strings.Contains(logged, `"msg":"user login"`) {
		t.Fatalf("missing login log line: %s", logged)
	}
}

func postLogin(t *testing.T, base, body string, headers map[string]string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/auth/login", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}
