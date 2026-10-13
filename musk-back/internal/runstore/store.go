package runstore

import (
	"context"
	"errors"
)

// ErrNotFound is returned when a run id is unknown.
var ErrNotFound = errors.New("agent run not found")

// Store persists Agent Runs.
type Store interface {
	Create(ctx context.Context, run *Run) error
	Get(ctx context.Context, id string) (*Run, error)
	Save(ctx context.Context, run *Run) error
	Update(ctx context.Context, id string, mut func(*Run)) (*Run, error)
	Close() error
}
