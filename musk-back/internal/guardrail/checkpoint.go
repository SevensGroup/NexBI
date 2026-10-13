package guardrail

import (
	"strings"
	"sync"
	"time"

	"autonomous-agent/internal/agent/decision"
	"autonomous-agent/internal/agent/planner"
)

// Checkpoint is the interrupted run, restorable by Resume.
type Checkpoint struct {
	RunID           string
	ApprovalID      string
	State           *decision.AgentState
	Plan            *planner.Plan
	PendingDecision *decision.Decision
	SavedAt         time.Time
}

// Checkpointer persists interrupt state so a run can Resume after approval.
type Checkpointer interface {
	Save(cp Checkpoint) error
	Load(runID string) (*Checkpoint, error)
	Delete(runID string) error
}

type memoryCheckpoints struct {
	mu    sync.Mutex
	items map[string]Checkpoint
}

// NewCheckpointer constructs an in-process checkpoint store.
func NewCheckpointer() Checkpointer {
	return &memoryCheckpoints{items: map[string]Checkpoint{}}
}

func (s *memoryCheckpoints) Save(cp Checkpoint) error {
	if s == nil {
		return ErrNoCheckpoint
	}
	runID := strings.TrimSpace(cp.RunID)
	if runID == "" {
		return ErrNoCheckpoint
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cp.RunID = runID
	if cp.SavedAt.IsZero() {
		cp.SavedAt = time.Now()
	}
	s.items[runID] = cp
	return nil
}

func (s *memoryCheckpoints) Load(runID string) (*Checkpoint, error) {
	if s == nil {
		return nil, ErrNoCheckpoint
	}
	runID = strings.TrimSpace(runID)
	s.mu.Lock()
	defer s.mu.Unlock()
	cp, ok := s.items[runID]
	if !ok {
		return nil, ErrNoCheckpoint
	}
	out := cp
	return &out, nil
}

func (s *memoryCheckpoints) Delete(runID string) error {
	if s == nil {
		return nil
	}
	runID = strings.TrimSpace(runID)
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, runID)
	return nil
}

// WaitingError is returned by the agent loop after Interrupt, before Resume.
type WaitingError struct {
	RunID      string
	ApprovalID string
	Tool       string
	Risk       RiskLevel
	Reason     string
}

func (e *WaitingError) Error() string {
	if e == nil {
		return ErrWaitingApproval.Error()
	}
	if e.Tool != "" {
		return ErrWaitingApproval.Error() + ": " + e.Tool
	}
	return ErrWaitingApproval.Error()
}

func (e *WaitingError) Unwrap() error {
	return ErrWaitingApproval
}
