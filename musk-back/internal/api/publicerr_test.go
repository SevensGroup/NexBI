package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"autonomous-agent/internal/agent/decision"
)

func TestPublicRunErrorLLMAuth(t *testing.T) {
	status, msg := publicRunError(errors.New("generate plan: error, status code: 401, status: 401 Unauthorized, message: Incorrect API key provided: sk-live-secret123. You can find your API key at https://platform.openai.com/account/api-keys."))
	if status != http.StatusBadGateway {
		t.Fatalf("status = %d", status)
	}
	if msg != "llm error" {
		t.Fatalf("msg = %q", msg)
	}
	if strings.Contains(msg, "sk-live-secret123") || strings.Contains(msg, "sk-your-key") {
		t.Fatalf("leaked key in %q", msg)
	}
}

func TestPublicRunErrorRedactsSecrets(t *testing.T) {
	status, msg := publicRunError(errors.New("planner crashed with sk-abcDEF123456 and Bearer tok_secret"))
	if status != http.StatusInternalServerError {
		t.Fatalf("status = %d", status)
	}
	if !strings.HasPrefix(msg, "agent run failed:") {
		t.Fatalf("msg = %q", msg)
	}
	if strings.Contains(msg, "sk-abcDEF123456") || strings.Contains(msg, "tok_secret") {
		t.Fatalf("leaked secret in %q", msg)
	}
	if !strings.Contains(msg, redacted) {
		t.Fatalf("expected redaction in %q", msg)
	}
}

func TestPublicRunErrorInvalidDecision(t *testing.T) {
	status, msg := publicRunError(fmt.Errorf("%w: arguments: JSON string must contain an object, got string", decision.ErrInvalidDecision))
	if status != http.StatusBadGateway {
		t.Fatalf("status = %d", status)
	}
	if !strings.Contains(msg, "invalid decision") {
		t.Fatalf("msg = %q", msg)
	}
}

func TestPublicRunErrorTimeoutAndUnreachable(t *testing.T) {
	status, msg := publicRunError(errors.New("context deadline exceeded"))
	if status != http.StatusGatewayTimeout {
		t.Fatalf("timeout status = %d msg = %q", status, msg)
	}
	status, msg = publicRunError(errors.New("agent budget exceeded: max runtime: context deadline exceeded"))
	if status != http.StatusGatewayTimeout || !strings.Contains(msg, "max runtime") {
		t.Fatalf("budget timeout status = %d msg = %q", status, msg)
	}
	status, msg = publicRunError(errors.New("dial tcp: connection refused"))
	if status != http.StatusBadGateway || !strings.Contains(msg, "LLM_BASE_URL") {
		t.Fatalf("unreachable status = %d msg = %q", status, msg)
	}
}
