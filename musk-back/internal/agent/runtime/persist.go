package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"autonomous-agent/internal/agent/decision"
	"autonomous-agent/internal/agent/reflection"
	"autonomous-agent/internal/analyst"
	"autonomous-agent/internal/guardrail"
	"autonomous-agent/internal/observability"
	"autonomous-agent/internal/runstore"
	"autonomous-agent/internal/tools"
)

var (
	// ErrBudgetToolCalls is returned when AGENT_MAX_TOOL_CALLS / MAX_TOOL_CALLS is exceeded.
	ErrBudgetToolCalls = errors.New("agent budget exceeded: max tool calls")
	// ErrBudgetRuntime is returned when AGENT_MAX_RUNTIME / MAX_RUNTIME is exceeded.
	ErrBudgetRuntime = errors.New("agent budget exceeded: max runtime")
	// ErrBudgetCost is returned when AGENT_MAX_LLM_COST / MAX_LLM_COST is exceeded.
	ErrBudgetCost = errors.New("agent budget exceeded: max llm cost")
)

func (r *Runtime) startSpan(ctx context.Context, name string) (context.Context, func()) {
	if r == nil || r.telemetry == nil {
		return ctx, func() {}
	}
	ctx, span := r.telemetry.StartSpan(ctx, name)
	return ctx, func() { span.End() }
}

func (r *Runtime) checkBudget(ctx context.Context, state *decision.AgentState) error {
	if ctx != nil && ctx.Err() != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("%w: %v", ErrBudgetRuntime, ctx.Err())
		}
		return ctx.Err()
	}
	if r == nil {
		return nil
	}
	runID := observability.RunID(ctx)
	if runID == "" && state != nil {
		runID = state.RunID
	}
	if r.budget.MaxLLMCost > 0 && r.telemetry != nil && r.telemetry.Tracker != nil {
		if cost := r.telemetry.Tracker.Cost(runID); cost > r.budget.MaxLLMCost {
			return fmt.Errorf("%w: cost=%.4f max=%.4f", ErrBudgetCost, cost, r.budget.MaxLLMCost)
		}
	}
	return nil
}

func (r *Runtime) checkToolBudget(ctx context.Context, state *decision.AgentState) error {
	if err := r.checkBudget(ctx, state); err != nil {
		return err
	}
	if r == nil {
		return nil
	}
	runID := observability.RunID(ctx)
	if runID == "" && state != nil {
		runID = state.RunID
	}
	used := 0
	if r.telemetry != nil && r.telemetry.Tracker != nil {
		used = r.telemetry.Tracker.ToolCalls(runID)
	} else if state != nil {
		used = len(state.ToolCalls)
	}
	if r.budget.MaxToolCalls > 0 && used >= r.budget.MaxToolCalls {
		return fmt.Errorf("%w: used=%d max=%d", ErrBudgetToolCalls, used, r.budget.MaxToolCalls)
	}
	return nil
}

func (r *Runtime) createRun(ctx context.Context, runID, goal string) {
	if r == nil || r.runs == nil || strings.TrimSpace(runID) == "" {
		return
	}
	now := time.Now().UTC()
	_ = r.runs.Create(ctx, &runstore.Run{
		ID:        runID,
		Goal:      goal,
		Status:    runstore.StatusCreated,
		CreatedAt: now,
		UpdatedAt: now,
	})
	_, _ = r.runs.Update(ctx, runID, func(run *runstore.Run) {
		run.Status = runstore.StatusRunning
	})
}

func (r *Runtime) recordStep(ctx context.Context, runID, stepID string, index int) {
	r.patchRun(ctx, runID, func(run *runstore.Run) {
		run.Steps = append(run.Steps, runstore.Step{ID: stepID, Index: index, StartedAt: time.Now().UTC()})
		run.Status = runstore.StatusRunning
	})
}

func (r *Runtime) recordDecision(ctx context.Context, runID, stepID string, d *decision.Decision) {
	if d == nil {
		return
	}
	r.patchRun(ctx, runID, func(run *runstore.Run) {
		run.Decisions = append(run.Decisions, runstore.DecisionRecord{
			StepID:         stepID,
			Type:           string(d.Type),
			Thought:        d.Thought,
			Reason:         d.Reason,
			Objective:      d.Objective,
			Tool:           d.Tool,
			Arguments:      d.Arguments,
			ExpectedResult: d.ExpectedResult,
			Confidence:     d.Confidence,
			Risk:           d.Risk,
			At:             time.Now().UTC(),
		})
		run.Confidence = d.Confidence
	})
}

func (r *Runtime) recordToolCall(ctx context.Context, runID, toolCallID, stepID string, d *decision.Decision, result tools.ToolResult) {
	if d == nil {
		return
	}
	evidence := result.Error
	success := result.Success
	if success {
		evidence = formatToolData(result.Data)
	}
	r.patchRun(ctx, runID, func(run *runstore.Run) {
		run.ToolCalls = append(run.ToolCalls, runstore.ToolCallRecord{
			ID:        toolCallID,
			StepID:    stepID,
			Tool:      d.Tool,
			Arguments: d.Arguments,
			Result:    evidence,
			Success:   success,
			LatencyMS: result.Duration.Milliseconds(),
			At:        time.Now().UTC(),
		})
	})
}

func (r *Runtime) recordObservation(ctx context.Context, runID, stepID string, obs decision.Observation) {
	r.patchRun(ctx, runID, func(run *runstore.Run) {
		run.Observations = append(run.Observations, runstore.ObservationRecord{
			StepID:     stepID,
			Summary:    obs.Summary,
			Reason:     obs.Reason,
			Evidence:   obs.Evidence,
			Confidence: obs.Confidence,
			At:         time.Now().UTC(),
		})
	})
}

func (r *Runtime) recordReflection(ctx context.Context, runID, stepID string, ref *reflection.Reflection) {
	if ref == nil {
		return
	}
	r.patchRun(ctx, runID, func(run *runstore.Run) {
		run.Reflections = append(run.Reflections, runstore.ReflectionRecord{
			StepID:             stepID,
			GoalSatisfied:      ref.GoalSatisfied,
			EvidenceSufficient: ref.EvidenceSufficient,
			ResultCorrect:      ref.ResultCorrect,
			MissingInformation: append([]string(nil), ref.MissingInformation...),
			Errors:             append([]string(nil), ref.Errors...),
			NextAction:         ref.NextAction,
			Confidence:         ref.Confidence,
			At:                 time.Now().UTC(),
		})
	})
}

func (r *Runtime) markWaiting(ctx context.Context, runID, approvalID string) {
	r.patchRun(ctx, runID, func(run *runstore.Run) {
		run.Status = runstore.StatusWaitingApproval
		run.ApprovalID = approvalID
	})
}

func (r *Runtime) failRun(ctx context.Context, runID string, status runstore.Status, err error) {
	if err == nil {
		return
	}
	if r.telemetry != nil {
		r.telemetry.RecordError(ctx, err)
	}
	r.patchRun(ctx, runID, func(run *runstore.Run) {
		run.Status = status
		run.Error = err.Error()
		r.applyTelemetry(run)
	})
}

func (r *Runtime) finalizeRun(ctx context.Context, state *decision.AgentState, answer string, err error) {
	if r == nil || state == nil {
		return
	}
	runID := state.RunID
	if err != nil {
		var waiting *guardrail.WaitingError
		if errors.As(err, &waiting) {
			r.markWaiting(ctx, runID, waiting.ApprovalID)
			return
		}
		status := runstore.StatusFailed
		if errors.Is(err, ErrBudgetToolCalls) || errors.Is(err, ErrBudgetRuntime) || errors.Is(err, ErrBudgetCost) {
			status = runstore.StatusAborted
		}
		r.failRun(ctx, runID, status, err)
		return
	}

	confidence := state.Confidence
	evidence := evidenceFromAnswer(answer, state)
	r.patchRun(ctx, runID, func(run *runstore.Run) {
		if strings.EqualFold(state.Status, decision.StatusWaitingApproval) {
			run.Status = runstore.StatusWaitingApproval
		} else {
			run.Status = runstore.StatusCompleted
		}
		run.Answer = answer
		run.FinalResult = answer
		run.Confidence = confidence
		run.Evidence = evidence
		r.applyTelemetry(run)
	})
	if r.telemetry != nil {
		r.telemetry.RecordRunLatency(ctx, r.telemetry.Tracker.Snapshot(runID).Latency, string(runstore.StatusCompleted))
	}
}

func (r *Runtime) applyTelemetry(run *runstore.Run) {
	if r == nil || r.telemetry == nil || r.telemetry.Tracker == nil || run == nil {
		return
	}
	snap := r.telemetry.Tracker.Snapshot(run.ID)
	if snap.LLMCalls > 0 || snap.TokenUsage.PromptTokens > 0 || snap.TokenUsage.CompletionTokens > 0 || snap.TokenUsage.TotalTokens > 0 {
		run.TokenUsage = snap.TokenUsage
		run.CostUSD = snap.CostUSD
	}
	if snap.Latency > 0 {
		run.LatencyMS = snap.Latency.Milliseconds()
	}
}

func (r *Runtime) patchRun(ctx context.Context, runID string, mut func(*runstore.Run)) {
	if r == nil || r.runs == nil || strings.TrimSpace(runID) == "" || mut == nil {
		return
	}
	_, _ = r.runs.Update(ctx, runID, func(run *runstore.Run) {
		mut(run)
		r.applyTelemetry(run)
	})
}

func evidenceFromAnswer(answer string, state *decision.AgentState) []any {
	if report, ok := analyst.ParseReport(answer); ok && report != nil {
		out := make([]any, 0, len(report.Evidence))
		for _, e := range report.Evidence {
			out = append(out, e)
		}
		if len(out) > 0 {
			return out
		}
	}
	if state == nil {
		return []any{}
	}
	out := make([]any, 0)
	for _, o := range state.Observations {
		if strings.HasPrefix(o.Summary, "reflection:") {
			continue
		}
		if ev := strings.TrimSpace(o.Evidence); ev != "" {
			out = append(out, ev)
		}
	}
	return out
}

// GetRun returns a persisted Agent Run with live token_usage / cost from the tracker.
func (r *Runtime) GetRun(ctx context.Context, id string) (*runstore.Run, error) {
	if r == nil || r.runs == nil {
		return nil, runstore.ErrNotFound
	}
	run, err := r.runs.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	r.applyTelemetry(run)
	return run, nil
}
