package memory

import (
	"context"

	"autonomous-agent/internal/logger"
)

// Recorder writes loop events to MemoryStore. Failures are logged and never returned.
type Recorder struct {
	store MemoryStore
	log   logger.Logger
}

// NewRecorder wraps store so agent-loop writes cannot crash the run.
func NewRecorder(store MemoryStore, log logger.Logger) *Recorder {
	if log == nil {
		log = logger.New("info", "json", nil)
	}
	return &Recorder{store: store, log: log}
}

func (r *Recorder) save(ctx context.Context, rec Record) {
	if r == nil || r.store == nil {
		return
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			r.log.WarnContext(ctx, "memory save panic",
				"panic", recovered,
				"run_id", rec.RunID,
				"kind", rec.Kind,
				"layer", rec.Layer,
			)
		}
	}()
	if err := r.store.Save(ctx, rec); err != nil {
		r.log.WarnContext(ctx, "memory save failed",
			"error", err,
			"run_id", rec.RunID,
			"kind", rec.Kind,
			"layer", rec.Layer,
		)
	}
}

func (r *Recorder) SaveGoal(ctx context.Context, runID, goal string) {
	r.save(ctx, Record{
		RunID: runID,
		Layer: LayerShortTerm,
		Kind:  KindGoal,
		Content: map[string]any{
			"goal": goal,
		},
	})
}

func (r *Recorder) SavePlan(ctx context.Context, runID string, plan []string) {
	r.save(ctx, Record{
		RunID: runID,
		Layer: LayerShortTerm,
		Kind:  KindPlan,
		Content: map[string]any{
			"plan": plan,
		},
	})
}

func (r *Recorder) SaveDecision(ctx context.Context, runID string, content map[string]any) {
	r.save(ctx, Record{
		RunID:   runID,
		Layer:   LayerShortTerm,
		Kind:    KindDecision,
		Content: content,
	})
}

func (r *Recorder) SaveToolCall(ctx context.Context, runID, tool string, arguments map[string]any, result string) {
	r.save(ctx, Record{
		RunID: runID,
		Layer: LayerShortTerm,
		Kind:  KindToolCall,
		Content: map[string]any{
			"tool":      tool,
			"arguments": arguments,
			"result":    result,
		},
		Metadata: map[string]any{"tool": tool},
	})
}

func (r *Recorder) SaveObservation(ctx context.Context, runID, summary, reason, evidence string, confidence float64) {
	r.save(ctx, Record{
		RunID: runID,
		Layer: LayerShortTerm,
		Kind:  KindObservation,
		Content: map[string]any{
			"summary":    summary,
			"reason":     reason,
			"evidence":   evidence,
			"confidence": confidence,
		},
	})
}

func (r *Recorder) SaveTask(ctx context.Context, runID, taskID, title, description, expected, status string) {
	r.save(ctx, Record{
		RunID:  runID,
		TaskID: taskID,
		Layer:  LayerTask,
		Kind:   KindTask,
		Content: map[string]any{
			"id":              taskID,
			"title":           title,
			"description":     description,
			"expected_output": expected,
			"status":          status,
		},
	})
}

func (r *Recorder) SaveEvidence(ctx context.Context, runID, taskID, evidence string) {
	r.save(ctx, Record{
		RunID:  runID,
		TaskID: taskID,
		Layer:  LayerTask,
		Kind:   KindEvidence,
		Content: map[string]any{
			"evidence": evidence,
		},
	})
}

func (r *Recorder) SaveFailure(ctx context.Context, runID, taskID, errMsg string) {
	r.save(ctx, Record{
		RunID:  runID,
		TaskID: taskID,
		Layer:  LayerTask,
		Kind:   KindFailures,
		Content: map[string]any{
			"task_id": taskID,
			"error":   errMsg,
		},
	})
}

func (r *Recorder) SaveSuccess(ctx context.Context, runID, taskID, result string) {
	r.save(ctx, Record{
		RunID:  runID,
		TaskID: taskID,
		Layer:  LayerTask,
		Kind:   KindSuccess,
		Content: map[string]any{
			"task_id": taskID,
			"result":  result,
		},
	})
}

func (r *Recorder) SaveConclusion(ctx context.Context, runID, outcome, answer string) {
	r.save(ctx, Record{
		RunID: runID,
		Layer: LayerTask,
		Kind:  KindConclusion,
		Content: map[string]any{
			"outcome": outcome,
			"answer":  answer,
		},
	})
}

// Recall loads long-term memories and prior task conclusions relevant to query.
// Search failures yield an empty slice.
func (r *Recorder) Recall(ctx context.Context, runID, query string, limit int) []Record {
	if r == nil || r.store == nil {
		return nil
	}
	if limit <= 0 {
		limit = 8
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			r.log.WarnContext(ctx, "memory search panic", "panic", recovered, "run_id", runID)
		}
	}()

	out := make([]Record, 0, limit)
	seen := map[string]struct{}{}
	appendPage := func(q Query) {
		res, err := r.store.Search(ctx, q)
		if err != nil {
			r.log.WarnContext(ctx, "memory search failed", "error", err, "run_id", runID, "layer", q.Layer)
			return
		}
		for _, rec := range res.Records {
			if _, ok := seen[rec.ID]; ok {
				continue
			}
			seen[rec.ID] = struct{}{}
			out = append(out, rec)
			if len(out) >= limit {
				return
			}
		}
	}

	per := limit / 2
	if per < 1 {
		per = 1
	}
	appendPage(Query{Layer: LayerLongTerm, Text: query, Limit: per})
	appendPage(Query{Layer: LayerTask, Kind: KindConclusion, Text: query, Limit: per})
	if runID != "" {
		appendPage(Query{RunID: runID, Layer: LayerShortTerm, Kind: KindGoal, Limit: 2})
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}
