package guardrail

import (
	"context"
	"strings"
	"sync"
	"time"

	"autonomous-agent/internal/requestid"
	"autonomous-agent/internal/tools"
)

// ApprovalStatus is the Human-in-the-Loop decision on a high-risk action.
type ApprovalStatus string

const (
	StatusPending  ApprovalStatus = "PENDING"
	StatusApproved ApprovalStatus = "APPROVED"
	StatusRejected ApprovalStatus = "REJECTED"
	StatusExpired  ApprovalStatus = "EXPIRED"
)

const defaultApprovalTTL = 15 * time.Minute

// ApprovalRequest is created when the agent interrupts on HIGH / CRITICAL (or configured MEDIUM).
type ApprovalRequest struct {
	ID        string         `json:"id"`
	RunID     string         `json:"run_id"`
	Tool      string         `json:"tool"`
	Arguments map[string]any `json:"arguments,omitempty"`
	Risk      RiskLevel      `json:"risk"`
	Reason    string         `json:"reason,omitempty"`
	Status    ApprovalStatus `json:"status"`
	CreatedAt time.Time      `json:"created_at"`
	ExpiresAt time.Time      `json:"expires_at"`
	DecidedAt *time.Time     `json:"decided_at,omitempty"`
	Note      string         `json:"note,omitempty"`
}

// ApprovalManager stores pending Human-in-the-Loop decisions.
type ApprovalManager interface {
	CreateApproval(ctx context.Context, req ApprovalRequest) (*ApprovalRequest, error)
	Approve(ctx context.Context, id string) (*ApprovalRequest, error)
	Reject(ctx context.Context, id, reason string) (*ApprovalRequest, error)
	Expire(ctx context.Context, id string) (*ApprovalRequest, error)
	Get(ctx context.Context, id string) (*ApprovalRequest, error)
}

type memoryApprovals struct {
	mu    sync.Mutex
	items map[string]*ApprovalRequest
	ttl   time.Duration
	now   func() time.Time
	idFn  func() string
}

// NewApprovalManager constructs an in-process ApprovalManager.
func NewApprovalManager(ttl time.Duration) ApprovalManager {
	if ttl <= 0 {
		ttl = defaultApprovalTTL
	}
	return &memoryApprovals{
		items: map[string]*ApprovalRequest{},
		ttl:   ttl,
		now:   time.Now,
		idFn:  requestid.New,
	}
}

func (m *memoryApprovals) CreateApproval(_ context.Context, req ApprovalRequest) (*ApprovalRequest, error) {
	if m == nil {
		return nil, ErrInvalidApproval
	}
	tool := strings.TrimSpace(req.Tool)
	if tool == "" {
		return nil, ErrInvalidApproval
	}
	risk := req.Risk
	if _, err := tools.ParseRiskLevel(string(risk)); err != nil {
		risk = RiskHigh
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	id := strings.TrimSpace(req.ID)
	if id == "" {
		id = m.idFn()
	}
	item := &ApprovalRequest{
		ID:        id,
		RunID:     strings.TrimSpace(req.RunID),
		Tool:      tool,
		Arguments: cloneArgs(req.Arguments),
		Risk:      risk,
		Reason:    strings.TrimSpace(req.Reason),
		Status:    StatusPending,
		CreatedAt: now,
		ExpiresAt: now.Add(m.ttl),
	}
	m.items[id] = item
	return cloneApproval(item), nil
}

func (m *memoryApprovals) Approve(ctx context.Context, id string) (*ApprovalRequest, error) {
	return m.decide(ctx, id, StatusApproved, "")
}

func (m *memoryApprovals) Reject(ctx context.Context, id, reason string) (*ApprovalRequest, error) {
	return m.decide(ctx, id, StatusRejected, reason)
}

func (m *memoryApprovals) Expire(ctx context.Context, id string) (*ApprovalRequest, error) {
	return m.decide(ctx, id, StatusExpired, "expired")
}

func (m *memoryApprovals) Get(_ context.Context, id string) (*ApprovalRequest, error) {
	if m == nil {
		return nil, ErrNotFound
	}
	id = strings.TrimSpace(id)
	m.mu.Lock()
	defer m.mu.Unlock()
	item, ok := m.items[id]
	if !ok {
		return nil, ErrNotFound
	}
	m.expireIfNeededLocked(item)
	return cloneApproval(item), nil
}

func (m *memoryApprovals) decide(_ context.Context, id string, status ApprovalStatus, note string) (*ApprovalRequest, error) {
	if m == nil {
		return nil, ErrNotFound
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, ErrInvalidApproval
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	item, ok := m.items[id]
	if !ok {
		return nil, ErrNotFound
	}
	m.expireIfNeededLocked(item)
	if item.Status != StatusPending {
		if item.Status == StatusExpired {
			return cloneApproval(item), ErrExpired
		}
		return cloneApproval(item), ErrNotPending
	}
	now := m.now()
	item.Status = status
	item.DecidedAt = &now
	item.Note = strings.TrimSpace(note)
	return cloneApproval(item), nil
}

func (m *memoryApprovals) expireIfNeededLocked(item *ApprovalRequest) {
	if item == nil || item.Status != StatusPending {
		return
	}
	if m.now().Before(item.ExpiresAt) {
		return
	}
	now := m.now()
	item.Status = StatusExpired
	item.DecidedAt = &now
	item.Note = "expired"
}

func cloneApproval(in *ApprovalRequest) *ApprovalRequest {
	if in == nil {
		return nil
	}
	out := *in
	out.Arguments = cloneArgs(in.Arguments)
	if in.DecidedAt != nil {
		t := *in.DecidedAt
		out.DecidedAt = &t
	}
	return &out
}

func cloneArgs(args map[string]any) map[string]any {
	if args == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(args))
	for k, v := range args {
		out[k] = v
	}
	return out
}
