package memory

import "errors"

var (
	// ErrNotFound is returned when Get/Delete cannot locate a record.
	ErrNotFound = errors.New("memory not found")
	// ErrInvalidRecord is returned when a record is missing required fields.
	ErrInvalidRecord = errors.New("invalid memory record")
	// ErrInvalidLayer is returned when the layer is not short_term/task/long_term.
	ErrInvalidLayer = errors.New("invalid memory layer")
	// ErrInvalidKind is returned when the kind is unknown or not valid for the layer.
	ErrInvalidKind = errors.New("invalid memory kind")
	// ErrClosed is returned when the store has been closed.
	ErrClosed = errors.New("memory store closed")
)
