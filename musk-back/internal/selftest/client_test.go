package selftest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCheckHealthOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Fatalf("path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	t.Cleanup(ts.Close)
	t.Setenv("MUSK_BACK_URL", ts.URL)
	if err := CheckHealth(NewHTTPClient(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
}

func TestCheckHealthDownMessage(t *testing.T) {
	t.Setenv("MUSK_BACK_URL", "http://127.0.0.1:1")
	err := CheckHealth(NewHTTPClient(500 * time.Millisecond))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "start musk-back with musk-back/.env") {
		t.Fatalf("hint missing: %v", err)
	}
}
