package memory

import (
	"context"
	"strings"
	"time"
)

// Layer is one of the three memory tiers.
type Layer string

const (
	LayerShortTerm Layer = "short_term"
	LayerTask      Layer = "task"
	LayerLongTerm  Layer = "long_term"
)

// Kind is the payload type stored in a layer.
type Kind string

const (
	KindGoal         Kind = "Goal"
	KindPlan         Kind = "Plan"
	KindDecision     Kind = "Decision"
	KindToolCall     Kind = "ToolCall"
	KindObservation  Kind = "Observation"
	KindTask         Kind = "Task"
	KindConclusion   Kind = "Conclusion"
	KindEvidence     Kind = "Evidence"
	KindFailures     Kind = "Failures"
	KindSuccess      Kind = "SuccessfulActions"
	KindPreference   Kind = "UserPreference"
	KindKnowledge    Kind = "BusinessKnowledge"
	KindInsight      Kind = "HistoricalInsight"
	KindQueryPattern Kind = "QueryPattern"
)

// Record is one persisted memory item.
type Record struct {
	ID        string         `json:"id"`
	RunID     string         `json:"run_id"`
	TaskID    string         `json:"task_id,omitempty"`
	Layer     Layer          `json:"layer"`
	Kind      Kind           `json:"kind"`
	Content   any            `json:"content"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	ExpiresAt *time.Time     `json:"expires_at,omitempty"`
}

// Query is a paginated, filterable memory search.
type Query struct {
	RunID    string
	Layer    Layer
	Kind     Kind
	TaskID   string
	Text     string
	Metadata map[string]any
	Limit    int
	Offset   int
}

// SearchResult is a page of matching records plus totals.
type SearchResult struct {
	Records []Record `json:"records"`
	Total   int      `json:"total"`
	Limit   int      `json:"limit"`
	Offset  int      `json:"offset"`
}

// MemoryStore is the persistence contract for all three layers.
type MemoryStore interface {
	Save(ctx context.Context, rec Record) error
	Get(ctx context.Context, id string) (*Record, error)
	Search(ctx context.Context, q Query) (SearchResult, error)
	Delete(ctx context.Context, id string) error
	Close() error
}

// ParseLayer accepts short_term / task / long_term (any case, spaces or hyphens).
func ParseLayer(s string) (Layer, error) {
	n := strings.ToLower(strings.TrimSpace(s))
	n = strings.ReplaceAll(n, "-", "_")
	n = strings.ReplaceAll(n, " ", "_")
	switch Layer(n) {
	case LayerShortTerm, "shortterm", "working":
		return LayerShortTerm, nil
	case LayerTask, "task_memory", "episodic":
		return LayerTask, nil
	case LayerLongTerm, "longterm":
		return LayerLongTerm, nil
	default:
		return "", ErrInvalidLayer
	}
}

// ParseKind accepts the canonical PascalCase kinds (any case).
func ParseKind(s string) (Kind, error) {
	n := strings.ToLower(strings.TrimSpace(s))
	n = strings.ReplaceAll(n, "-", "")
	n = strings.ReplaceAll(n, "_", "")
	n = strings.ReplaceAll(n, " ", "")
	switch n {
	case "goal":
		return KindGoal, nil
	case "plan":
		return KindPlan, nil
	case "decision":
		return KindDecision, nil
	case "toolcall":
		return KindToolCall, nil
	case "observation":
		return KindObservation, nil
	case "task":
		return KindTask, nil
	case "conclusion":
		return KindConclusion, nil
	case "evidence":
		return KindEvidence, nil
	case "failures", "failure":
		return KindFailures, nil
	case "successfulactions", "successfulaction", "success":
		return KindSuccess, nil
	case "userpreference", "preference":
		return KindPreference, nil
	case "businessknowledge", "knowledge":
		return KindKnowledge, nil
	case "historicalinsight", "insight":
		return KindInsight, nil
	case "querypattern":
		return KindQueryPattern, nil
	default:
		return "", ErrInvalidKind
	}
}

// ValidFor reports whether kind may be stored on layer.
func (k Kind) ValidFor(layer Layer) bool {
	switch layer {
	case LayerShortTerm:
		switch k {
		case KindGoal, KindPlan, KindDecision, KindToolCall, KindObservation:
			return true
		}
	case LayerTask:
		switch k {
		case KindTask, KindConclusion, KindEvidence, KindFailures, KindSuccess:
			return true
		}
	case LayerLongTerm:
		switch k {
		case KindPreference, KindKnowledge, KindInsight, KindQueryPattern:
			return true
		}
	}
	return false
}

func (r Record) expired(now time.Time) bool {
	return r.ExpiresAt != nil && !r.ExpiresAt.IsZero() && !r.ExpiresAt.After(now)
}
