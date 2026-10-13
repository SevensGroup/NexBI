package observability

import (
	"sync"
	"time"
)

// TokenUsage is prompt/completion token counts for cost tracking.
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Add merges other into u.
func (u *TokenUsage) Add(other TokenUsage) {
	if u == nil {
		return
	}
	u.PromptTokens += other.PromptTokens
	u.CompletionTokens += other.CompletionTokens
	u.TotalTokens += other.TotalTokens
	if u.TotalTokens == 0 {
		u.TotalTokens = u.PromptTokens + u.CompletionTokens
	}
}

// CostModel converts tokens to USD.
type CostModel struct {
	InputUSDPer1K  float64
	OutputUSDPer1K float64
}

// DefaultCostModel is a conservative OpenAI-compatible default (gpt-4o-mini-class).
func DefaultCostModel() CostModel {
	return CostModel{InputUSDPer1K: 0.00015, OutputUSDPer1K: 0.0006}
}

// CostUSD returns estimated USD for the given usage.
func (m CostModel) CostUSD(u TokenUsage) float64 {
	in := m.InputUSDPer1K
	out := m.OutputUSDPer1K
	if in <= 0 {
		in = DefaultCostModel().InputUSDPer1K
	}
	if out <= 0 {
		out = DefaultCostModel().OutputUSDPer1K
	}
	return (float64(u.PromptTokens)/1000)*in + (float64(u.CompletionTokens)/1000)*out
}

// Snapshot is a point-in-time view of one run's telemetry.
type Snapshot struct {
	RunID      string        `json:"run_id"`
	TokenUsage TokenUsage    `json:"token_usage"`
	CostUSD    float64       `json:"cost_usd"`
	Latency    time.Duration `json:"latency"`
	LLMCalls   int           `json:"llm_calls"`
	ToolCalls  int           `json:"tool_calls"`
	Errors     int           `json:"errors"`
	LastError  string        `json:"last_error,omitempty"`
}

type runStats struct {
	tokens    TokenUsage
	cost      float64
	started   time.Time
	llmCalls  int
	toolCalls int
	errors    int
	lastErr   string
}

// Tracker accumulates token usage, cost, latency, and errors per run_id.
type Tracker struct {
	cost CostModel

	mu    sync.Mutex
	stats map[string]*runStats
}

// NewTracker constructs a per-run telemetry accumulator.
func NewTracker(cost CostModel) *Tracker {
	if cost.InputUSDPer1K <= 0 && cost.OutputUSDPer1K <= 0 {
		cost = DefaultCostModel()
	}
	return &Tracker{cost: cost, stats: map[string]*runStats{}}
}

func (t *Tracker) get(runID string) *runStats {
	if t == nil {
		return &runStats{started: time.Now()}
	}
	s, ok := t.stats[runID]
	if !ok {
		s = &runStats{started: time.Now()}
		t.stats[runID] = s
	}
	return s
}

// StartRun marks the start of an agent run for latency.
func (t *Tracker) StartRun(runID string) {
	if t == nil || runID == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.get(runID).started = time.Now()
}

// RecordTokens adds LLM token usage and cost for a run.
func (t *Tracker) RecordTokens(runID string, usage TokenUsage) float64 {
	if t == nil {
		return 0
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	delta := t.cost.CostUSD(usage)
	t.mu.Lock()
	defer t.mu.Unlock()
	s := t.get(runID)
	s.tokens.Add(usage)
	s.cost += delta
	s.llmCalls++
	return delta
}

// RecordToolCall increments the tool-call counter.
func (t *Tracker) RecordToolCall(runID string) {
	if t == nil || runID == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.get(runID).toolCalls++
}

// RecordError tracks an error for a run.
func (t *Tracker) RecordError(runID, msg string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	s := t.get(runID)
	s.errors++
	s.lastErr = msg
}

// Snapshot returns current stats for runID.
func (t *Tracker) Snapshot(runID string) Snapshot {
	if t == nil {
		return Snapshot{RunID: runID}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	s := t.stats[runID]
	if s == nil {
		return Snapshot{RunID: runID}
	}
	started := s.started
	if started.IsZero() {
		started = time.Now()
	}
	return Snapshot{
		RunID:      runID,
		TokenUsage: s.tokens,
		CostUSD:    s.cost,
		Latency:    time.Since(started),
		LLMCalls:   s.llmCalls,
		ToolCalls:  s.toolCalls,
		Errors:     s.errors,
		LastError:  s.lastErr,
	}
}

// Cost returns accumulated LLM cost for the run.
func (t *Tracker) Cost(runID string) float64 {
	return t.Snapshot(runID).CostUSD
}

// ToolCalls returns how many tools the run has executed.
func (t *Tracker) ToolCalls(runID string) int {
	return t.Snapshot(runID).ToolCalls
}
