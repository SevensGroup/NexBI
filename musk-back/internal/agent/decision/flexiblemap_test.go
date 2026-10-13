package decision

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestFlexibleMapUnmarshal(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		raw     string
		want    map[string]any
		wantNil bool
		wantErr string
	}{
		{
			name: "object",
			raw:  `{"nl":"Analyze yesterday Meta ad ROI"}`,
			want: map[string]any{"nl": "Analyze yesterday Meta ad ROI"},
		},
		{
			name: "stringified json object",
			raw:  `"{\"nl\":\"Analyze yesterday Meta ad ROI\"}"`,
			want: map[string]any{"nl": "Analyze yesterday Meta ad ROI"},
		},
		{
			name: "stringified json with surrounding whitespace",
			raw:  `"  {\"city\":\"Paris\"}  "`,
			want: map[string]any{"city": "Paris"},
		},
		{
			name: "empty string",
			raw:  `""`,
			want: map[string]any{},
		},
		{
			name: "whitespace string",
			raw:  `"   "`,
			want: map[string]any{},
		},
		{
			name:    "null",
			raw:     `null`,
			wantNil: true,
		},
		{
			name:    "invalid json string",
			raw:     `"not-an-object"`,
			wantErr: "JSON string must contain an object",
		},
		{
			name:    "invalid inner json",
			raw:     `"{\"nl\":"`,
			wantErr: "JSON string is not a valid object",
		},
		{
			name:    "array",
			raw:     `["nl"]`,
			wantErr: "expected object, JSON string, or null",
		},
		{
			name:    "number",
			raw:     `1`,
			wantErr: "expected object, JSON string, or null",
		},
		{
			name:    "stringified array",
			raw:     `"[]"`,
			wantErr: "JSON string must contain an object",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var got FlexibleMap
			err := json.Unmarshal([]byte(tc.raw), &got)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("err = nil, want %q", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want substring %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if tc.wantNil {
				if got != nil {
					t.Fatalf("got %#v, want nil", got)
				}
				return
			}
			if len(got) != len(tc.want) {
				t.Fatalf("len = %d, want %d (%#v)", len(got), len(tc.want), got)
			}
			for k, v := range tc.want {
				if got[k] != v {
					t.Fatalf("got[%q] = %#v, want %#v", k, got[k], v)
				}
			}
		})
	}
}

func TestDecodeDecisionArgumentsShapes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		payload string
		wantNL  string
		wantNil bool
		wantErr string
	}{
		{
			name:    "object arguments",
			payload: `{"type":"CALL_TOOL","reason":"query roi","tool":"semantic_query","arguments":{"nl":"Analyze yesterday Meta ad ROI"},"confidence":0.8}`,
			wantNL:  "Analyze yesterday Meta ad ROI",
		},
		{
			name:    "stringified arguments",
			payload: `{"type":"CALL_TOOL","reason":"query roi","tool":"semantic_query","arguments":"{\"nl\":\"Analyze yesterday Meta ad ROI\"}","confidence":0.8}`,
			wantNL:  "Analyze yesterday Meta ad ROI",
		},
		{
			name:    "empty string arguments",
			payload: `{"type":"FINISH","reason":"done","arguments":"","confidence":1}`,
			wantNil: false,
		},
		{
			name:    "null arguments",
			payload: `{"type":"FINISH","reason":"done","arguments":null,"confidence":1}`,
			wantNil: true,
		},
		{
			name:    "missing arguments",
			payload: `{"type":"FINISH","reason":"done","confidence":1}`,
			wantNil: true,
		},
		{
			name:    "invalid string arguments",
			payload: `{"type":"CALL_TOOL","reason":"query roi","tool":"semantic_query","arguments":"not-json","confidence":0.8}`,
			wantErr: "arguments",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d, err := decodeDecision([]byte(tc.payload))
			if tc.wantErr != "" {
				if err == nil {
					t.Fatal("err = nil")
				}
				if !errors.Is(err, ErrInvalidDecision) {
					t.Fatalf("err = %v, want ErrInvalidDecision", err)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want substring %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("decodeDecision: %v", err)
			}
			if tc.wantNL != "" {
				got, _ := d.Arguments["nl"].(string)
				if got != tc.wantNL {
					t.Fatalf("nl = %q, want %q", got, tc.wantNL)
				}
				return
			}
			if tc.wantNil {
				if d.Arguments != nil {
					t.Fatalf("arguments = %#v, want nil", d.Arguments)
				}
				return
			}
			if d.Arguments == nil {
				t.Fatal("arguments = nil, want empty map")
			}
		})
	}
}

func TestFlexibleMapNilReceiver(t *testing.T) {
	t.Parallel()
	var m *FlexibleMap
	err := m.UnmarshalJSON([]byte(`{}`))
	if err == nil || !strings.Contains(err.Error(), "nil pointer") {
		t.Fatalf("err = %v, want nil pointer", err)
	}
}
