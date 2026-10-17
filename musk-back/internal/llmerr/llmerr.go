package llmerr

import "strings"

// Public is the only LLM failure text shown to users.
const Public = "llm error"

// Is reports whether err came from the model provider and must not be shown raw.
func Is(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	switch {
	case strings.Contains(text, "failed to create chat completion"):
		return true
	case strings.Contains(text, "status code: 429"), strings.Contains(text, "status code: 401"), strings.Contains(text, "status code: 403"):
		return true
	case strings.Contains(text, "status code: 500"), strings.Contains(text, "status code: 502"), strings.Contains(text, "status code: 503"), strings.Contains(text, "status code: 504"):
		return true
	case strings.Contains(text, "quota exceeded"), strings.Contains(text, "rate limit"), strings.Contains(text, "too many requests"):
		return true
	case strings.Contains(text, "resource_exhausted"):
		return true
	case strings.Contains(text, "incorrect api key"), strings.Contains(text, "invalid api key"), strings.Contains(text, "invalid_api_key"):
		return true
	case strings.Contains(text, "errorresponse"):
		return true
	default:
		return false
	}
}
