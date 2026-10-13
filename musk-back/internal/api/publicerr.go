package api

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"autonomous-agent/internal/agent/decision"
)

const (
	maxPublicErrorRunes = 280
	redacted            = "[REDACTED]"
)

var (
	secretValueRe = regexp.MustCompile(`(?i)(sk-[A-Za-z0-9_\-]{4,}|Bearer\s+\S+)`)
	apiKeyMsgRe   = regexp.MustCompile(`(?i)incorrect api key provided:\s*\S+`)
)

func publicRunError(err error) (status int, msg string) {
	if err == nil {
		return http.StatusInternalServerError, "agent run failed"
	}

	raw := strings.TrimSpace(err.Error())
	lower := strings.ToLower(raw)

	switch {
	case strings.Contains(lower, "budget exceeded") || strings.Contains(lower, "max runtime") || strings.Contains(lower, "handler timeout"):
		return http.StatusGatewayTimeout, "agent run exceeded max runtime"
	case errors.Is(err, decision.ErrInvalidDecision):
		return http.StatusBadGateway, truncateRunes(redactSecrets(raw), maxPublicErrorRunes)
	case isLLMAuthError(lower):
		return http.StatusBadGateway, "LLM authentication failed (401): set LLM_API_KEY to a real key and restart. Placeholder values from .env.example are rejected by the provider."
	case strings.Contains(lower, "status code: 429") || strings.Contains(lower, "rate limit") || strings.Contains(lower, "too many requests"):
		return http.StatusServiceUnavailable, "LLM rate limited: retry later or check provider quota"
	case strings.Contains(lower, "deadline exceeded") || strings.Contains(lower, "context deadline") || strings.Contains(lower, "timeout"):
		return http.StatusGatewayTimeout, "LLM request timed out: check LLM_BASE_URL and network"
	case strings.Contains(lower, "connection refused") || strings.Contains(lower, "no such host") || strings.Contains(lower, "network is unreachable"):
		return http.StatusBadGateway, "LLM endpoint unreachable: check LLM_BASE_URL"
	default:
		return http.StatusInternalServerError, "agent run failed: " + truncateRunes(redactSecrets(raw), maxPublicErrorRunes)
	}
}

func isLLMAuthError(lower string) bool {
	if strings.Contains(lower, "incorrect api key") || strings.Contains(lower, "invalid_api_key") || strings.Contains(lower, "invalid api key") {
		return true
	}
	if strings.Contains(lower, "status code: 401") || strings.Contains(lower, "401 unauthorized") {
		return true
	}
	return strings.Contains(lower, "status code: 403") || strings.Contains(lower, "403 forbidden")
}

func redactSecrets(s string) string {
	s = apiKeyMsgRe.ReplaceAllString(s, "incorrect API key provided: "+redacted)
	return secretValueRe.ReplaceAllString(s, redacted)
}

func truncateRunes(s string, max int) string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max]) + "..."
}
