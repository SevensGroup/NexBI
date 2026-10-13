package reflection

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrInvalidReflection is returned when a structured reflection fails validation.
	ErrInvalidReflection = errors.New("invalid reflection")
)

var validNextActions = map[string]struct{}{
	NextContinue: {},
	NextReplan:   {},
	NextFinish:   {},
	NextAskUser:  {},
}

var droppedCoTKeys = map[string]struct{}{
	"chain_of_thought":  {},
	"chainOfThought":    {},
	"thinking":          {},
	"reasoning":         {},
	"reasoning_content": {},
	"cot":               {},
}

// Validator rejects illegal structured reflections.
type Validator struct{}

// NewValidator constructs a Reflection validator.
func NewValidator() *Validator {
	return &Validator{}
}

// Validate checks next_action, confidence, and required fields.
func (v *Validator) Validate(r *Reflection) error {
	if r == nil {
		return fmt.Errorf("%w: reflection is nil", ErrInvalidReflection)
	}
	normalize(r)
	if _, ok := validNextActions[r.NextAction]; !ok {
		return fmt.Errorf("%w: unknown next_action %q", ErrInvalidReflection, r.NextAction)
	}
	if r.Confidence < 0 || r.Confidence > 1 {
		return fmt.Errorf("%w: confidence must be between 0 and 1", ErrInvalidReflection)
	}
	return nil
}

func decodeReflection(payload []byte) (*Reflection, error) {
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidReflection, err)
	}
	for key := range droppedCoTKeys {
		delete(raw, key)
	}

	normalized, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidReflection, err)
	}

	var r Reflection
	if err := json.Unmarshal(normalized, &r); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidReflection, err)
	}
	return &r, nil
}

func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```JSON")
		s = strings.TrimPrefix(s, "```")
		if i := strings.LastIndex(s, "```"); i >= 0 {
			s = s[:i]
		}
		s = strings.TrimSpace(s)
	}
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return ""
}
