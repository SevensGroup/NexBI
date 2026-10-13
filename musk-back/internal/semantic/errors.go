package semantic

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrUnknownMetric is returned when a metric is not in the catalog.
	ErrUnknownMetric = errors.New("unknown metric")
	// ErrUnknownDimension is returned when a dimension is not in the catalog.
	ErrUnknownDimension = errors.New("unknown dimension")
	// ErrUnknownFilter is returned when a filter dimension is not allowed.
	ErrUnknownFilter = errors.New("unknown filter")
	// ErrInvalidQuery is returned when a SemanticQuery fails validation.
	ErrInvalidQuery = errors.New("invalid semantic query")
	// ErrParse is returned when natural language cannot be mapped to a SemanticQuery.
	ErrParse = errors.New("semantic parse failed")
	// ErrCompile is returned when a SemanticQuery cannot be compiled to ES DSL.
	ErrCompile = errors.New("semantic compile failed")
)

func wrap(err error, msg string) error {
	if err == nil {
		return errors.New(msg)
	}
	return fmt.Errorf("%w: %s", err, msg)
}

func normalizeKey(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
