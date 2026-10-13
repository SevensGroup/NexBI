package tools

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ValidateInput checks input against a JSON-Schema-like object.
// Supported keywords: type, properties, required, enum, items.
func ValidateInput(schema any, input map[string]any) error {
	if schema == nil {
		return newToolError(CodeInvalidTool, "", "", "input schema is required", ErrInvalidTool)
	}
	if input == nil {
		input = map[string]any{}
	}
	return validateNode(schema, input, "$")
}

func validateNode(schema any, value any, path string) error {
	spec, ok := asObject(schema)
	if !ok {
		return newToolError(CodeInvalidInput, "", "", fmt.Sprintf("%s: schema must be an object", path), ErrInvalidInput)
	}

	if enums, has := spec["enum"]; has {
		if !enumContains(enums, value) {
			return newToolError(CodeInvalidInput, "", "", fmt.Sprintf("%s: value %v is not in enum", path, value), ErrInvalidInput)
		}
	}

	typ, hasType := spec["type"]
	if hasType {
		if err := checkType(fmt.Sprint(typ), value, path, spec); err != nil {
			return err
		}
	}

	if fmt.Sprint(typ) == "object" || spec["properties"] != nil || spec["required"] != nil {
		obj, ok := asObject(value)
		if !ok {
			if value == nil {
				obj = map[string]any{}
			} else {
				return newToolError(CodeInvalidInput, "", "", fmt.Sprintf("%s: expected object", path), ErrInvalidInput)
			}
		}
		if err := checkRequired(spec["required"], obj, path); err != nil {
			return err
		}
		props, _ := asObject(spec["properties"])
		for name, propSchema := range props {
			child, exists := obj[name]
			if !exists {
				continue
			}
			childPath := path + "." + name
			if path == "$" {
				childPath = name
			}
			if err := validateNode(propSchema, child, childPath); err != nil {
				return err
			}
		}
	}

	if fmt.Sprint(typ) == "array" {
		arr, ok := asArray(value)
		if !ok {
			return newToolError(CodeInvalidInput, "", "", fmt.Sprintf("%s: expected array", path), ErrInvalidInput)
		}
		if items, has := spec["items"]; has {
			for i, item := range arr {
				if err := validateNode(items, item, fmt.Sprintf("%s[%d]", path, i)); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func checkRequired(raw any, obj map[string]any, path string) error {
	if raw == nil {
		return nil
	}
	required, ok := asStringSlice(raw)
	if !ok {
		return newToolError(CodeInvalidInput, "", "", fmt.Sprintf("%s: schema required must be an array of strings", path), ErrInvalidInput)
	}
	for _, field := range required {
		if _, ok := obj[field]; !ok {
			loc := field
			if path != "$" {
				loc = path + "." + field
			}
			return newToolError(CodeInvalidInput, "", "", fmt.Sprintf("%s is required", loc), ErrInvalidInput)
		}
	}
	return nil
}

func checkType(typ string, value any, path string, spec map[string]any) error {
	switch strings.ToLower(strings.TrimSpace(typ)) {
	case "object":
		if _, ok := asObject(value); !ok && value != nil {
			return newToolError(CodeInvalidInput, "", "", fmt.Sprintf("%s: expected object", path), ErrInvalidInput)
		}
	case "string":
		if _, ok := value.(string); !ok {
			return newToolError(CodeInvalidInput, "", "", fmt.Sprintf("%s: expected string", path), ErrInvalidInput)
		}
	case "number":
		if _, ok := asFloat(value); !ok {
			return newToolError(CodeInvalidInput, "", "", fmt.Sprintf("%s: expected number", path), ErrInvalidInput)
		}
	case "integer":
		f, ok := asFloat(value)
		if !ok || f != float64(int64(f)) {
			return newToolError(CodeInvalidInput, "", "", fmt.Sprintf("%s: expected integer", path), ErrInvalidInput)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return newToolError(CodeInvalidInput, "", "", fmt.Sprintf("%s: expected boolean", path), ErrInvalidInput)
		}
	case "array":
		if _, ok := asArray(value); !ok {
			return newToolError(CodeInvalidInput, "", "", fmt.Sprintf("%s: expected array", path), ErrInvalidInput)
		}
	case "null":
		if value != nil {
			return newToolError(CodeInvalidInput, "", "", fmt.Sprintf("%s: expected null", path), ErrInvalidInput)
		}
	default:
		_ = spec
	}
	return nil
}

func enumContains(raw any, value any) bool {
	items, ok := asArray(raw)
	if !ok {
		return false
	}
	want := stringify(value)
	for _, item := range items {
		if stringify(item) == want {
			return true
		}
	}
	return false
}

func asObject(v any) (map[string]any, bool) {
	switch typed := v.(type) {
	case map[string]any:
		return typed, true
	case map[string]string:
		out := make(map[string]any, len(typed))
		for k, val := range typed {
			out[k] = val
		}
		return out, true
	default:
		return nil, false
	}
}

func asArray(v any) ([]any, bool) {
	switch typed := v.(type) {
	case []any:
		return typed, true
	case []string:
		out := make([]any, len(typed))
		for i, s := range typed {
			out[i] = s
		}
		return out, true
	default:
		return nil, false
	}
}

func asStringSlice(v any) ([]string, bool) {
	switch typed := v.(type) {
	case []string:
		return typed, true
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			out = append(out, s)
		}
		return out, true
	default:
		return nil, false
	}
}

func asFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func stringify(v any) string {
	switch typed := v.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		if f, ok := asFloat(v); ok && f == float64(int64(f)) {
			return strconv.FormatInt(int64(f), 10)
		}
		return fmt.Sprint(v)
	}
}
