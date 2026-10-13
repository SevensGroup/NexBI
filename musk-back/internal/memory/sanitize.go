package memory

import (
	"regexp"
	"strings"
)

const redacted = "[REDACTED]"

var (
	secretValueRe = regexp.MustCompile(`(?i)(sk-[A-Za-z0-9_\-]{8,}|Bearer\s+[A-Za-z0-9\-._~+/]+=*)`)
)

func isSensitiveKey(key string) bool {
	n := normalizeKey(key)
	switch n {
	case "api_key", "apikey", "llm_api_key",
		"password", "passwd", "pwd",
		"secret", "secret_key", "client_secret",
		"token", "access_token", "refresh_token", "id_token", "auth_token",
		"authorization", "auth",
		"credential", "credentials",
		"private_key", "privatekey",
		"access_key", "accesskey",
		"aws_secret_access_key", "aws_access_key_id":
		return true
	}
	if strings.HasSuffix(n, "_api_key") || strings.HasSuffix(n, "_apikey") {
		return true
	}
	if strings.HasSuffix(n, "_password") || strings.HasSuffix(n, "_secret") || strings.HasSuffix(n, "_token") {
		return true
	}
	if strings.HasSuffix(n, "_credential") || strings.HasSuffix(n, "_credentials") || strings.HasSuffix(n, "_private_key") {
		return true
	}
	return false
}

func normalizeKey(key string) string {
	n := strings.ToLower(strings.TrimSpace(key))
	n = strings.ReplaceAll(n, "-", "_")
	n = strings.ReplaceAll(n, " ", "_")
	return n
}

// Sanitize strips API keys and credentials from a value before persistence.
func Sanitize(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for k, val := range typed {
			if isSensitiveKey(k) {
				out[k] = redacted
				continue
			}
			out[k] = Sanitize(val)
		}
		return out
	case map[string]string:
		out := make(map[string]any, len(typed))
		for k, val := range typed {
			if isSensitiveKey(k) {
				out[k] = redacted
				continue
			}
			out[k] = Sanitize(val)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = Sanitize(item)
		}
		return out
	case []string:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = redactString(item)
		}
		return out
	case string:
		return redactString(typed)
	default:
		return v
	}
}

func SanitizeMetadata(meta map[string]any) map[string]any {
	if meta == nil {
		return map[string]any{}
	}
	cleaned, _ := Sanitize(meta).(map[string]any)
	if cleaned == nil {
		return map[string]any{}
	}
	return cleaned
}

func redactString(s string) string {
	if s == "" {
		return s
	}
	return secretValueRe.ReplaceAllString(s, redacted)
}
