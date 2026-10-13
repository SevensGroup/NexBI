package decision

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// FlexibleMap is a JSON object that also accepts a JSON-encoded object string.
// SiliconFlow/DeepSeek sometimes emit arguments as "{\"nl\":\"...\"}" instead of {"nl":"..."}.
type FlexibleMap map[string]any

// UnmarshalJSON accepts a JSON object, a JSON string containing an object,
// null, or an empty string. Invalid payloads return a clear error and never panic.
func (m *FlexibleMap) UnmarshalJSON(data []byte) error {
	if m == nil {
		return fmt.Errorf("arguments: UnmarshalJSON on nil pointer")
	}

	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*m = nil
		return nil
	}

	switch data[0] {
	case '{':
		var obj map[string]any
		if err := json.Unmarshal(data, &obj); err != nil {
			return fmt.Errorf("arguments: invalid JSON object: %w", err)
		}
		*m = obj
		return nil
	case '"':
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return fmt.Errorf("arguments: invalid JSON string: %w", err)
		}
		return m.unmarshalObjectString(s)
	default:
		return fmt.Errorf("arguments: expected object, JSON string, or null, got %s", jsonKind(data))
	}
}

func (m *FlexibleMap) unmarshalObjectString(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		*m = FlexibleMap{}
		return nil
	}

	inner := []byte(s)
	inner = bytes.TrimSpace(inner)
	if len(inner) == 0 || bytes.Equal(inner, []byte("null")) {
		*m = FlexibleMap{}
		return nil
	}
	if inner[0] != '{' {
		return fmt.Errorf("arguments: JSON string must contain an object, got %s", jsonKind(inner))
	}

	var obj map[string]any
	if err := json.Unmarshal(inner, &obj); err != nil {
		return fmt.Errorf("arguments: JSON string is not a valid object: %w", err)
	}
	*m = obj
	return nil
}

func jsonKind(data []byte) string {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return "empty"
	}
	switch data[0] {
	case '{':
		return "object"
	case '[':
		return "array"
	case '"':
		return "string"
	case 't':
		if bytes.Equal(data, []byte("true")) {
			return "boolean"
		}
	case 'f':
		if bytes.Equal(data, []byte("false")) {
			return "boolean"
		}
	case 'n':
		if bytes.Equal(data, []byte("null")) {
			return "null"
		}
	}
	if data[0] == '-' || (data[0] >= '0' && data[0] <= '9') {
		return "number"
	}
	return "other"
}
