package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	einomodel "github.com/cloudwego/eino/components/model"

	"autonomous-agent/internal/agent/decision"
	"autonomous-agent/internal/agent/multi"
	"autonomous-agent/internal/agent/planner"
	"autonomous-agent/internal/agent/reflection"
	"autonomous-agent/internal/analyst"
	"autonomous-agent/internal/config"
	"autonomous-agent/internal/guardrail"
	"autonomous-agent/internal/logger"
	"autonomous-agent/internal/memory"
	"autonomous-agent/internal/model"
	"autonomous-agent/internal/observability"
	"autonomous-agent/internal/requestid"
	"autonomous-agent/internal/runstore"
	"autonomous-agent/internal/semantic"
	"autonomous-agent/internal/tools"
	"autonomous-agent/internal/tools/elasticsearch"
)

// Runner is the Agent Runtime contract used by the HTTP API.
// The API layer depends on this interface, not on Eino or HTTP types.
type Runner interface {
	Run(ctx context.Context, message string) (string, error)
	Resume(ctx context.Context, runID string) (string, error)
	Approve(ctx context.Context, approvalID string) error
	Reject(ctx context.Context, approvalID, reason string) error
	GetRun(ctx context.Context, id string) (*runstore.Run, error)
	ListAgents() []AgentInfo
	ListTools() []tools.Info
	ListSemanticMetrics() []semantic.Metric
	ListSemanticDimensions() []semantic.Dimension
}

var _ Runner = (*Runtime)(nil)

// AgentInfo is a Coordinator or specialist listing for GET /api/v1/agents.
type AgentInfo struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Role         string   `json:"role,omitempty"`
	Tools        []string `json:"tools,omitempty"`
	InputSchema  any      `json:"input_schema,omitempty"`
	OutputSchema any      `json:"output_schema,omitempty"`
}

// Deps is optional production wiring (telemetry, run store). Tests omit it.
type Deps struct {
	Telemetry *observability.Telemetry
	Runs      runstore.Store
}

// Decider is the decision-engine contract used by the runtime loop.
type Decider interface {
	Decide(ctx context.Context, state *decision.AgentState) (*decision.Decision, error)
	Apply(state *decision.AgentState, d *decision.Decision) decision.Outcome
}

// Planner is the task-graph contract used by the runtime loop.
type Planner interface {
	Plan(ctx context.Context, goal string, state decision.AgentState) (*planner.Plan, error)
}

// Runtime executes: Goal → Plan → Understand → Decide → Act → Observe → Reflection → Decide / Finish.
// Act goes through the Tool Registry, never a concrete tool type.
// DELEGATE_AGENT runs a specialist (Analyst / Researcher / Reviewer) as Agent-as-Tool.
// Short-term / task / long-term memory is persisted through MemoryStore; write failures are logged only.
type Runtime struct {
	engine      Decider
	planner     Planner
	reflector   reflection.Reflector
	registry    tools.Registry
	memory      memory.MemoryStore
	remember    *memory.Recorder
	model       einomodel.BaseChatModel
	team        multi.Delegator
	role        string
	ownsStore   bool
	maxSteps    int
	maxTasks    int
	recallLimit int
	risk        *guardrail.Engine
	approvals   guardrail.ApprovalManager
	checkpoints guardrail.Checkpointer
	logger      logger.Logger
	telemetry   *observability.Telemetry
	runs        runstore.Store
	catalog     *semantic.Registry
	budget      config.BudgetConfig
	agentName   string
	ownsRuns    bool
	ownsTel     bool
}

// New constructs an Eino-backed decision runtime from a model provider.
func New(ctx context.Context, provider model.Provider, agentCfg config.AgentConfig, toolsCfg config.ToolsConfig, semanticCfg config.SemanticConfig, memoryCfg config.MemoryConfig, guardCfg config.GuardrailConfig, log logger.Logger, extras ...Deps) (*Runtime, error) {
	if provider == nil {
		return nil, errors.New("model provider is required")
	}
	if log == nil {
		return nil, errors.New("logger is required")
	}

	chatModel, err := provider.NewChatModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("create chat model: %w", err)
	}

	engine, err := decision.NewEngine(chatModel, agentCfg.Instruction, log)
	if err != nil {
		return nil, fmt.Errorf("create decision engine: %w", err)
	}

	p, err := planner.New(chatModel, agentCfg.Instruction, agentCfg.MaxTasks, log)
	if err != nil {
		return nil, fmt.Errorf("create planner: %w", err)
	}

	refl, err := reflection.NewEngine(chatModel, agentCfg.Instruction, log)
	if err != nil {
		return nil, fmt.Errorf("create reflection engine: %w", err)
	}

	maxTasks := agentCfg.MaxTasks
	if maxTasks < 1 {
		maxTasks = p.MaxTasks()
	}

	timeout := toolsCfg.Timeout.Duration()
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	reg := tools.NewRegistry(log, timeout)
	if err := tools.RegisterBuiltins(reg); err != nil {
		return nil, fmt.Errorf("register tools: %w", err)
	}
	if err := elasticsearch.RegisterFromConfig(ctx, reg, toolsCfg.Elasticsearch, log); err != nil {
		return nil, fmt.Errorf("register elasticsearch tools: %w", err)
	}
	catalog, err := semantic.RegisterFromConfig(ctx, reg, semanticCfg, log)
	if err != nil {
		return nil, fmt.Errorf("register semantic layer: %w", err)
	}
	if err := analyst.RegisterFromConfig(ctx, reg, semanticCfg, log); err != nil {
		return nil, fmt.Errorf("register advertising analyst: %w", err)
	}

	store, err := memory.Open(ctx, memoryCfg, log)
	if err != nil {
		return nil, fmt.Errorf("open memory store: %w", err)
	}
	if err := memory.RegisterTools(reg, store); err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("register memory tools: %w", err)
	}

	recallLimit := memoryCfg.SearchLimit
	if recallLimit < 1 {
		recallLimit = 8
	}

	ttl := guardCfg.ApprovalTTL.Duration()
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}

	budget := agentCfg.Budget
	if budget.MaxToolCalls < 1 {
		budget.MaxToolCalls = 50
	}
	if budget.MaxRuntime.Duration() <= 0 {
		budget.MaxRuntime = config.Duration(10 * time.Minute)
	}
	if budget.MaxLLMCost < 0 {
		budget.MaxLLMCost = 1.0
	}

	var extra Deps
	if len(extras) > 0 {
		extra = extras[0]
	}

	tel := extra.Telemetry
	ownsTel := false
	if tel == nil {
		tel, err = observability.Setup(ctx, observability.Config{
			ServiceName: "autonomous-agent",
			Cost: observability.CostModel{
				InputUSDPer1K:  budget.InputUSDPer1K,
				OutputUSDPer1K: budget.OutputUSDPer1K,
			},
		}, log)
		if err != nil {
			_ = store.Close()
			return nil, fmt.Errorf("setup observability: %w", err)
		}
		ownsTel = true
	}

	runs := extra.Runs
	ownsRuns := false
	if runs == nil {
		runs, err = runstore.Open(ctx, memoryCfg.PostgresURL, log)
		if err != nil {
			_ = store.Close()
			if ownsTel {
				_ = tel.Shutdown(ctx)
			}
			return nil, fmt.Errorf("open run store: %w", err)
		}
		ownsRuns = true
	}

	rt := &Runtime{
		engine:      engine,
		planner:     p,
		reflector:   refl,
		registry:    reg,
		memory:      store,
		remember:    memory.NewRecorder(store, log),
		model:       chatModel,
		role:        string(multi.RoleCoordinator),
		ownsStore:   true,
		maxSteps:    agentCfg.MaxSteps,
		maxTasks:    maxTasks,
		recallLimit: recallLimit,
		risk:        guardrail.NewEngine(guardrail.Policy{MediumAuto: guardCfg.MediumAutoExecute()}),
		approvals:   guardrail.NewApprovalManager(ttl),
		checkpoints: guardrail.NewCheckpointer(),
		logger:      log,
		telemetry:   tel,
		runs:        runs,
		catalog:     catalog,
		budget:      budget,
		agentName:   agentCfg.Name,
		ownsRuns:    ownsRuns,
		ownsTel:     ownsTel,
	}
	rt.team = multi.NewTeam(multi.DefaultSpecs(agentCfg.MaxSteps), rt.spawnSpecialist)
	return rt, nil
}

// Close releases stores owned by this runtime. Child specialists do not close parent resources.
func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}
	var first error
	if r.memory != nil && r.ownsStore {
		if err := r.memory.Close(); err != nil && first == nil {
			first = err
		}
	}
	if r.runs != nil && r.ownsRuns {
		if err := r.runs.Close(); err != nil && first == nil {
			first = err
		}
	}
	if r.telemetry != nil && r.ownsTel {
		if err := r.telemetry.Shutdown(context.Background()); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// Run sends a user goal through plan + decision loop and returns the terminal answer.
func (r *Runtime) Run(ctx context.Context, message string) (string, error) {
	message = strings.TrimSpace(message)
	if message == "" {
		return "", ErrEmptyMessage
	}

	state := decision.NewState(message, r.maxSteps)
	runID := strings.TrimSpace(requestid.From(ctx))
	if runID == "" {
		runID = requestid.New()
		ctx = requestid.With(ctx, runID)
	}
	if r.role != "" && r.role != string(multi.RoleCoordinator) {
		runID = memory.ScopeRunID(runID, r.role)
		ctx = memory.WithAgent(ctx, r.role)
	}
	state.RunID = runID
	ctx = memory.WithRunID(ctx, runID)
	obsRun := strings.TrimSpace(requestid.From(ctx))
	if obsRun == "" {
		obsRun = runID
	}
	ctx = observability.WithRunID(ctx, obsRun)
	if r.telemetry != nil && r.telemetry.Tracker != nil {
		r.telemetry.Tracker.StartRun(obsRun)
	}

	if r.budget.MaxRuntime.Duration() > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.budget.MaxRuntime.Duration())
		defer cancel()
	}
	if r.telemetry != nil {
		var spanEnd func()
		ctx, spanEnd = r.startSpan(ctx, "agent.run")
		defer spanEnd()
	}

	r.createRun(ctx, runID, message)

	decision.Understand(state)
	r.attachTools(state)
	r.attachAgents(state)
	r.remember.SaveGoal(ctx, runID, message)
	r.recall(ctx, state)

	r.logger.InfoContext(ctx, "agent run started",
		"request_id", requestid.From(ctx),
		"run_id", runID,
		"message_len", len(message),
		"max_steps", state.MaxSteps,
		"max_tasks", r.maxTasks,
		"tools", toolNames(state.AvailableTools),
	)

	plan, err := r.planner.Plan(ctx, state.Goal, *state)
	if err != nil {
		r.logger.ErrorContext(ctx, "agent plan failed",
			"request_id", requestid.From(ctx),
			"error", err,
		)
		r.failRun(ctx, runID, runstore.StatusFailed, err)
		return "", err
	}

	sched, err := r.installPlan(ctx, state, plan)
	if err != nil {
		r.failRun(ctx, runID, runstore.StatusFailed, err)
		return "", err
	}

	answer, err := r.loop(ctx, state, sched)
	r.finalizeRun(ctx, state, answer, err)
	return answer, err
}

func (r *Runtime) loop(ctx context.Context, state *decision.AgentState, sched *planner.TaskScheduler) (string, error) {
	for {
		if err := r.checkBudget(ctx, state); err != nil {
			r.failRun(ctx, state.RunID, runstore.StatusAborted, err)
			return "", err
		}
		r.syncObjective(state, sched)
		r.recall(ctx, state)

		stepID := observability.NewID()
		state.CurrentStepID = stepID
		ctx = observability.WithStepID(ctx, stepID)
		r.recordStep(ctx, state.RunID, stepID, state.CurrentStep)

		d, err := r.engine.Decide(ctx, state)
		if err != nil {
			r.logger.ErrorContext(ctx, "agent run failed",
				"request_id", requestid.From(ctx),
				"run_id", state.RunID,
				"error", err,
			)
			if r.telemetry != nil {
				r.telemetry.RecordError(ctx, err)
			}
			r.failRun(ctx, state.RunID, runstore.StatusFailed, err)
			return "", err
		}
		r.persistDecision(ctx, state, d)
		r.recordDecision(ctx, state.RunID, stepID, d)

		outcome := r.engine.Apply(state, d)

		switch d.Type {
		case decision.DecisionReplan:
			next, planErr := r.planner.Plan(ctx, state.Goal, *state)
			if planErr != nil {
				return "", planErr
			}
			sched, err = r.installPlan(ctx, state, next)
			if err != nil {
				return "", err
			}
		case decision.DecisionCallTool:
			if err := r.act(ctx, state, d, sched); err != nil {
				return "", err
			}
			refOutcome, terminal, nextSched, err := r.reflectOn(ctx, state, lastObservation(state), sched)
			if err != nil {
				return "", err
			}
			sched = nextSched
			if terminal != nil && (refOutcome == decision.OutcomeFinish || refOutcome == decision.OutcomeAskUser) {
				d = terminal
				outcome = refOutcome
			}
		case decision.DecisionDelegateAgent:
			rejected, err := r.delegate(ctx, state, d, sched)
			if err != nil {
				return "", err
			}
			if rejected {
				forced := &decision.Decision{
					Type:       decision.DecisionReplan,
					Reason:     "reviewer rejected the conclusion",
					Objective:  "replan after reviewer rejection",
					Confidence: d.Confidence,
					Risk:       "low",
				}
				if n := len(state.Delegations); n > 0 && strings.TrimSpace(state.Delegations[n-1].Reason) != "" {
					forced.Reason = "reviewer rejected: " + state.Delegations[n-1].Reason
				}
				r.persistDecision(ctx, state, forced)
				r.engine.Apply(state, forced)
				next, planErr := r.planner.Plan(ctx, state.Goal, *state)
				if planErr != nil {
					return "", planErr
				}
				sched, err = r.installPlan(ctx, state, next)
				if err != nil {
					return "", err
				}
			} else {
				refOutcome, terminal, nextSched, err := r.reflectOn(ctx, state, lastObservation(state), sched)
				if err != nil {
					return "", err
				}
				sched = nextSched
				if terminal != nil && (refOutcome == decision.OutcomeFinish || refOutcome == decision.OutcomeAskUser) {
					d = terminal
					outcome = refOutcome
				}
			}
		case decision.DecisionReflect:
			refOutcome, terminal, nextSched, err := r.reflectOn(ctx, state, lastObservation(state), sched)
			if err != nil {
				return "", err
			}
			sched = nextSched
			if terminal != nil && (refOutcome == decision.OutcomeFinish || refOutcome == decision.OutcomeAskUser) {
				d = terminal
				outcome = refOutcome
			}
		}

		switch outcome {
		case decision.OutcomeFinish, decision.OutcomeAskUser, decision.OutcomeAbort:
			answer := decision.TerminalAnswer(d)
			if report, ok := analyst.ParseReport(answer); ok {
				answer = analyst.MarshalReport(*report)
			} else if report, ok := lastReport(state); ok && outcome == decision.OutcomeFinish {
				answer = analyst.MarshalReport(*report)
			}
			if answer == "" {
				return "", ErrEmptyAnswer
			}
			state.Status = decision.StatusFinished
			r.remember.SaveConclusion(ctx, state.RunID, string(outcome), answer)
			r.logger.InfoContext(ctx, "agent run completed",
				"request_id", requestid.From(ctx),
				"run_id", state.RunID,
				"outcome", outcome,
				"answer_len", len(answer),
				"steps", state.CurrentStep+1,
			)
			return answer, nil
		case decision.OutcomeContinue:
			state.CurrentStep++
			continue
		default:
			return "", fmt.Errorf("unknown decision outcome %q", outcome)
		}
	}
}

func (r *Runtime) attachTools(state *decision.AgentState) {
	if state == nil || r == nil || r.registry == nil {
		return
	}
	_, hideRawES := r.registry.Get(semantic.ToolName)
	list := r.registry.List()
	state.AvailableTools = make([]decision.ToolListing, 0, len(list))
	for _, info := range list {
		if hideRawES && semantic.IsRawSearchTool(info.Name) {
			continue
		}
		state.AvailableTools = append(state.AvailableTools, decision.ToolListing{
			Name:        info.Name,
			Description: info.Description,
			InputSchema: info.InputSchema,
			Risk:        string(info.Risk),
		})
	}
}

func toolNames(list []decision.ToolListing) string {
	names := make([]string, 0, len(list))
	for _, t := range list {
		names = append(names, t.Name)
	}
	return strings.Join(names, ",")
}

func (r *Runtime) attachAgents(state *decision.AgentState) {
	if state == nil || r == nil || r.team == nil {
		return
	}
	if r.role != "" && r.role != string(multi.RoleCoordinator) {
		state.AvailableAgents = nil
		return
	}
	state.AvailableAgents = r.team.Listings()
}

func (r *Runtime) spawnSpecialist(spec multi.Spec) (multi.Runner, error) {
	if r == nil {
		return nil, multi.ErrNoRunner
	}
	if r.model == nil {
		return nil, fmt.Errorf("%w: chat model is required", multi.ErrNoRunner)
	}
	eng, err := decision.NewEngine(r.model, spec.Instruction, r.logger)
	if err != nil {
		return nil, err
	}
	p, err := planner.New(r.model, spec.Instruction, r.maxTasks, r.logger)
	if err != nil {
		return nil, err
	}
	refl, err := reflection.NewEngine(r.model, spec.Instruction, r.logger)
	if err != nil {
		return nil, err
	}
	maxSteps := spec.MaxSteps
	if maxSteps < 1 {
		maxSteps = r.maxSteps
	}
	scoped := memory.Scope(r.memory, spec.Name)
	return &Runtime{
		engine:      eng,
		planner:     p,
		reflector:   refl,
		registry:    multi.FilterRegistry(r.registry, spec.Tools),
		memory:      scoped,
		remember:    memory.NewRecorder(scoped, r.logger),
		model:       r.model,
		team:        nil,
		role:        spec.Name,
		ownsStore:   false,
		maxSteps:    maxSteps,
		maxTasks:    r.maxTasks,
		recallLimit: r.recallLimit,
		risk:        r.risk,
		approvals:   r.approvals,
		checkpoints: r.checkpoints,
		logger:      r.logger,
		telemetry:   r.telemetry,
		runs:        r.runs,
		catalog:     r.catalog,
		budget:      r.budget,
		agentName:   spec.Name,
		ownsRuns:    false,
		ownsTel:     false,
	}, nil
}

func (r *Runtime) delegate(ctx context.Context, state *decision.AgentState, d *decision.Decision, sched *planner.TaskScheduler) (bool, error) {
	agent, task := decision.DelegateTarget(d)
	if agent == "" || task == "" {
		r.observeDelegate(ctx, state, d, agent, task, nil, fmt.Errorf("%w: agent and task are required", multi.ErrInvalidDelegate))
		return false, nil
	}
	if r.team == nil || (r.role != "" && r.role != string(multi.RoleCoordinator)) {
		r.observeDelegate(ctx, state, d, agent, task, nil, multi.ErrNestedDelegate)
		return false, nil
	}

	extra := map[string]any{}
	for k, v := range d.Arguments {
		extra[k] = v
	}
	extra["goal"] = ""
	if state != nil {
		extra["goal"] = state.Goal
	}
	if agent == string(multi.RoleReviewer) {
		if stringArg(extra, "conclusion") == "" || stringArg(extra, "evidence") == "" {
			if last := lastSpecialistDelegation(state); last != nil {
				if stringArg(extra, "conclusion") == "" {
					extra["conclusion"] = last.Result
				}
				if stringArg(extra, "evidence") == "" {
					extra["evidence"] = last.Result
				}
				if stringArg(extra, "source_agent") == "" {
					extra["source_agent"] = last.Agent
				}
			}
		}
	}

	var taskNode *planner.Task
	if sched != nil {
		taskNode = sched.NextReady()
		if taskNode != nil {
			if err := sched.Start(taskNode.ID); err != nil {
				return false, err
			}
		}
	}

	out, err := r.team.Delegate(ctx, multi.DelegateRequest{Agent: agent, Task: task, Extra: extra})
	r.observeDelegate(ctx, state, d, agent, task, out, err)

	if taskNode != nil {
		if err == nil && out != nil && !out.Rejected() {
			if cerr := sched.Complete(taskNode.ID); cerr != nil {
				return false, cerr
			}
			state.CompletedTasks = append(state.CompletedTasks, taskNode.ID)
			r.remember.SaveSuccess(ctx, state.RunID, taskNode.ID, formatToolData(out))
			r.remember.SaveEvidence(ctx, state.RunID, taskNode.ID, formatToolData(out))
		} else {
			_ = sched.Fail(taskNode.ID)
			state.FailedTasks = append(state.FailedTasks, taskNode.ID)
			msg := ""
			if out != nil && out.Rejected() && out.Review != nil {
				msg = out.Review.Reason
			} else if err != nil {
				msg = err.Error()
			}
			r.remember.SaveFailure(ctx, state.RunID, taskNode.ID, msg)
		}
	}

	if out != nil && out.Rejected() {
		return true, nil
	}
	return false, nil
}

func (r *Runtime) observeDelegate(ctx context.Context, state *decision.AgentState, d *decision.Decision, agent, task string, out *multi.AgentOutput, execErr error) {
	if state == nil {
		return
	}
	summary := "delegate " + agent + " result"
	evidence := ""
	success := execErr == nil && out != nil
	if !success {
		summary = "delegate " + agent + " error"
		if execErr != nil {
			evidence = execErr.Error()
		}
	}
	rec := decision.DelegationRecord{Agent: agent, Task: task}
	if out != nil {
		evidence = formatToolData(out)
		rec.Result = evidence
		if out.Review != nil {
			approved := out.Review.Approved
			rec.Approved = &approved
			rec.Reason = out.Review.Reason
			if out.Rejected() {
				summary = "delegate " + agent + " rejected"
				success = false
			}
		} else if out.Conclusion != "" {
			rec.Result = out.Conclusion
			if out.Evidence != "" {
				evidence = out.Evidence
			}
		}
	}
	state.Delegations = append(state.Delegations, rec)
	conf := 0.0
	reason := ""
	if d != nil {
		conf = d.Confidence
		reason = d.Reason
	}
	state.Observations = append(state.Observations, decision.Observation{
		Summary:    summary,
		Reason:     reason,
		Evidence:   evidence,
		Confidence: conf,
	})
	if r != nil && r.logger != nil {
		r.logger.InfoContext(ctx, "agent delegate observation",
			"request_id", requestid.From(ctx),
			"agent", agent,
			"task", task,
			"success", success,
			"summary", summary,
			"rejected", rec.Approved != nil && !*rec.Approved,
		)
	}
	if r != nil && r.remember != nil {
		r.remember.SaveObservation(ctx, state.RunID, summary, reason, evidence, conf)
	}
}

func lastSpecialistDelegation(state *decision.AgentState) *decision.DelegationRecord {
	if state == nil {
		return nil
	}
	for i := len(state.Delegations) - 1; i >= 0; i-- {
		d := state.Delegations[i]
		if d.Agent == string(multi.RoleReviewer) {
			continue
		}
		if strings.TrimSpace(d.Result) == "" {
			continue
		}
		return &state.Delegations[i]
	}
	return nil
}

func stringArg(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	s := strings.TrimSpace(fmt.Sprint(v))
	if s == "" || s == "<nil>" {
		return ""
	}
	return s
}

func (r *Runtime) installPlan(ctx context.Context, state *decision.AgentState, plan *planner.Plan) (*planner.TaskScheduler, error) {
	if state != nil {
		state.ClearActionHistory()
	}
	if err := planner.ValidatePlan(plan, r.maxTasks); err != nil {
		return nil, err
	}
	state.Plan = plan.Summaries()
	sched := planner.NewTaskScheduler(plan)
	r.remember.SavePlan(ctx, state.RunID, state.Plan)
	if plan != nil {
		for _, task := range plan.Tasks {
			r.remember.SaveTask(ctx, state.RunID, task.ID, task.Title, task.Description, task.ExpectedOutput, task.Status)
		}
	}
	r.logger.InfoContext(ctx, "agent task graph ready",
		"request_id", requestid.From(ctx),
		"run_id", state.RunID,
		"task_count", sched.Len(),
		"ready_count", len(sched.Ready()),
	)
	return sched, nil
}

func (r *Runtime) syncObjective(state *decision.AgentState, sched *planner.TaskScheduler) {
	if state == nil || sched == nil {
		return
	}
	if next := sched.NextReady(); next != nil {
		state.CurrentObjective = strings.TrimSpace(next.Title + ": " + next.Description)
		return
	}
	if sched.AllComplete() {
		state.CurrentObjective = "All planned tasks are complete. Finish with the final answer."
	}
}

func (r *Runtime) act(ctx context.Context, state *decision.AgentState, d *decision.Decision, sched *planner.TaskScheduler) error {
	return r.executeTool(ctx, state, d, sched, true)
}

func (r *Runtime) executeTool(ctx context.Context, state *decision.AgentState, d *decision.Decision, sched *planner.TaskScheduler, checkGuard bool) error {
	if r == nil || r.registry == nil {
		return fmt.Errorf("tool registry is not configured")
	}

	if err := r.checkToolBudget(ctx, state); err != nil {
		return err
	}

	input := d.Arguments
	if input == nil {
		input = map[string]any{}
	}

	if checkGuard {
		if err := r.maybeInterrupt(ctx, state, d, sched, input); err != nil {
			return err
		}
	}

	toolCallID := observability.NewID()
	ctx = observability.WithToolCallID(ctx, toolCallID)
	if n := len(state.ToolCalls); n > 0 && strings.TrimSpace(state.ToolCalls[n-1].ID) == "" {
		state.ToolCalls[n-1].ID = toolCallID
	}

	var task *planner.Task
	if sched != nil {
		task = sched.NextReady()
		if task != nil {
			if err := sched.Start(task.ID); err != nil {
				return err
			}
		}
	}

	result, execErr := r.registry.Execute(ctx, d.Tool, input)
	if r.telemetry != nil {
		r.telemetry.RecordTool(ctx, d.Tool, result.Success, result.Duration)
	}
	r.recordToolCall(ctx, state.RunID, toolCallID, state.CurrentStepID, d, result)
	r.observe(ctx, state, d, result)

	if task != nil {
		if result.Success {
			if err := sched.Complete(task.ID); err != nil {
				return err
			}
			state.CompletedTasks = append(state.CompletedTasks, task.ID)
			r.remember.SaveSuccess(ctx, state.RunID, task.ID, formatToolData(result.Data))
			r.remember.SaveEvidence(ctx, state.RunID, task.ID, formatToolData(result.Data))
		} else {
			_ = sched.Fail(task.ID)
			state.FailedTasks = append(state.FailedTasks, task.ID)
			errMsg := result.Error
			if errMsg == "" && execErr != nil {
				errMsg = execErr.Error()
			}
			r.remember.SaveFailure(ctx, state.RunID, task.ID, errMsg)
		}
	}

	if execErr != nil && result.Error == "" {
		return execErr
	}
	return nil
}

func (r *Runtime) maybeInterrupt(ctx context.Context, state *decision.AgentState, d *decision.Decision, sched *planner.TaskScheduler, input map[string]any) error {
	if r == nil || r.risk == nil || d == nil {
		return nil
	}
	assessment, err := r.risk.Evaluate(guardrail.Action{
		Tool:           d.Tool,
		Arguments:      input,
		RegisteredRisk: lookupRisk(r.registry, d.Tool),
	})
	if err != nil {
		return err
	}
	if !assessment.RequiresApproval() {
		return nil
	}
	return r.interrupt(ctx, state, d, sched, assessment)
}

func (r *Runtime) interrupt(ctx context.Context, state *decision.AgentState, d *decision.Decision, sched *planner.TaskScheduler, assessment guardrail.Assessment) error {
	if r.approvals == nil || r.checkpoints == nil {
		return fmt.Errorf("approval manager is not configured")
	}
	runID := r.checkpointID(ctx, state)
	created, err := r.approvals.CreateApproval(ctx, guardrail.ApprovalRequest{
		RunID:     runID,
		Tool:      d.Tool,
		Arguments: d.Arguments,
		Risk:      assessment.Level,
		Reason:    assessment.Reason,
	})
	if err != nil {
		return err
	}

	state.Status = decision.StatusWaitingApproval
	state.PendingApprovalID = created.ID

	cp := guardrail.Checkpoint{
		RunID:           runID,
		ApprovalID:      created.ID,
		State:           state.Clone(),
		Plan:            sched.Snapshot(),
		PendingDecision: cloneDecision(d),
		SavedAt:         time.Now(),
	}
	if err := r.checkpoints.Save(cp); err != nil {
		return err
	}
	if state.RunID != "" && state.RunID != runID {
		cp.RunID = state.RunID
		_ = r.checkpoints.Save(cp)
	}

	r.logger.InfoContext(ctx, "agent interrupted for approval",
		"request_id", requestid.From(ctx),
		"run_id", runID,
		"approval_id", created.ID,
		"tool", d.Tool,
		"risk", assessment.Level,
	)
	r.remember.SaveObservation(ctx, state.RunID, "waiting_approval: "+d.Tool, assessment.Reason, created.ID, d.Confidence)

	r.markWaiting(ctx, runID, created.ID)

	return &guardrail.WaitingError{
		RunID:      runID,
		ApprovalID: created.ID,
		Tool:       d.Tool,
		Risk:       assessment.Level,
		Reason:     assessment.Reason,
	}
}

func (r *Runtime) checkpointID(ctx context.Context, state *decision.AgentState) string {
	if id := strings.TrimSpace(requestid.From(ctx)); id != "" {
		return id
	}
	if state != nil {
		if id := strings.TrimSpace(state.RunID); id != "" {
			return id
		}
	}
	return requestid.New()
}

func lookupRisk(reg tools.Registry, name string) tools.RiskLevel {
	if reg == nil {
		return ""
	}
	name = strings.ToLower(strings.TrimSpace(name))
	for _, info := range reg.List() {
		if info.Name == name {
			return info.Risk
		}
	}
	return ""
}

func cloneDecision(d *decision.Decision) *decision.Decision {
	if d == nil {
		return nil
	}
	out := *d
	if d.Arguments != nil {
		out.Arguments = make(map[string]any, len(d.Arguments))
		for k, v := range d.Arguments {
			out.Arguments[k] = v
		}
	}
	return &out
}

// Resume restores a WAITING_APPROVAL checkpoint after Approve or Reject.
func (r *Runtime) Resume(ctx context.Context, runID string) (string, error) {
	if r == nil || r.checkpoints == nil || r.approvals == nil {
		return "", guardrail.ErrNoCheckpoint
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		runID = strings.TrimSpace(requestid.From(ctx))
	}
	if runID == "" {
		return "", guardrail.ErrNoCheckpoint
	}
	if requestid.From(ctx) == "" {
		ctx = requestid.With(ctx, runID)
	}

	cp, err := r.checkpoints.Load(runID)
	if err != nil {
		return "", err
	}
	if cp.State == nil || cp.PendingDecision == nil {
		return "", guardrail.ErrNoCheckpoint
	}

	appr, err := r.approvals.Get(ctx, cp.ApprovalID)
	if err != nil {
		return "", err
	}

	state := cp.State
	state.Status = decision.StatusResume
	ctx = memory.WithRunID(ctx, state.RunID)
	ctx = observability.WithRunID(ctx, state.RunID)
	sched := planner.NewTaskScheduler(cp.Plan)

	r.logger.InfoContext(ctx, "agent resume",
		"request_id", requestid.From(ctx),
		"run_id", runID,
		"approval_id", cp.ApprovalID,
		"approval_status", appr.Status,
		"tool", cp.PendingDecision.Tool,
	)

	switch appr.Status {
	case guardrail.StatusApproved:
		if err := r.executeTool(ctx, state, cp.PendingDecision, sched, false); err != nil {
			return "", err
		}
		refOutcome, terminal, nextSched, err := r.reflectOn(ctx, state, lastObservation(state), sched)
		if err != nil {
			return "", err
		}
		sched = nextSched
		state.PendingApprovalID = ""
		state.Status = decision.StatusRunning
		_ = r.checkpoints.Delete(runID)
		if state.RunID != "" && state.RunID != runID {
			_ = r.checkpoints.Delete(state.RunID)
		}
		if terminal != nil && (refOutcome == decision.OutcomeFinish || refOutcome == decision.OutcomeAskUser) {
			answer := decision.TerminalAnswer(terminal)
			if report, ok := analyst.ParseReport(answer); ok {
				answer = analyst.MarshalReport(*report)
			} else if report, ok := lastReport(state); ok && refOutcome == decision.OutcomeFinish {
				answer = analyst.MarshalReport(*report)
			}
			if answer == "" {
				return "", ErrEmptyAnswer
			}
			state.Status = decision.StatusFinished
			r.remember.SaveConclusion(ctx, state.RunID, string(refOutcome), answer)
			r.finalizeRun(ctx, state, answer, nil)
			return answer, nil
		}
		state.CurrentStep++
		answer, err := r.loop(ctx, state, sched)
		r.finalizeRun(ctx, state, answer, err)
		return answer, err
	case guardrail.StatusRejected:
		r.observeRejectedApproval(ctx, state, cp.PendingDecision, appr)
		state.PendingApprovalID = ""
		state.Status = decision.StatusRunning
		_ = r.checkpoints.Delete(runID)
		if state.RunID != "" && state.RunID != runID {
			_ = r.checkpoints.Delete(state.RunID)
		}
		state.CurrentStep++
		answer, err := r.loop(ctx, state, sched)
		r.finalizeRun(ctx, state, answer, err)
		return answer, err
	case guardrail.StatusExpired:
		return "", guardrail.ErrExpired
	default:
		return "", &guardrail.WaitingError{
			RunID:      runID,
			ApprovalID: cp.ApprovalID,
			Tool:       cp.PendingDecision.Tool,
			Risk:       appr.Risk,
			Reason:     "approval is still pending",
		}
	}
}

// Approve confirms a pending HIGH/CRITICAL (or gated MEDIUM) action. Resume executes it.
func (r *Runtime) Approve(ctx context.Context, approvalID string) error {
	if r == nil || r.approvals == nil {
		return guardrail.ErrNotFound
	}
	_, err := r.approvals.Approve(ctx, approvalID)
	return err
}

// Reject denies a pending action. Resume observes the rejection and continues the loop.
func (r *Runtime) Reject(ctx context.Context, approvalID, reason string) error {
	if r == nil || r.approvals == nil {
		return guardrail.ErrNotFound
	}
	_, err := r.approvals.Reject(ctx, approvalID, reason)
	return err
}

func (r *Runtime) observeRejectedApproval(ctx context.Context, state *decision.AgentState, d *decision.Decision, appr *guardrail.ApprovalRequest) {
	if state == nil || d == nil {
		return
	}
	note := "approval rejected"
	if appr != nil && strings.TrimSpace(appr.Note) != "" {
		note = "approval rejected: " + appr.Note
	}
	if n := len(state.ToolCalls); n > 0 && strings.TrimSpace(state.ToolCalls[n-1].Result) == "" {
		state.ToolCalls[n-1].Result = note
	}
	state.Observations = append(state.Observations, decision.Observation{
		Summary:    strings.TrimSpace(d.Tool) + " rejected",
		Reason:     d.Reason,
		Evidence:   note,
		Confidence: d.Confidence,
	})
	if r.remember != nil {
		r.remember.SaveObservation(ctx, state.RunID, strings.TrimSpace(d.Tool)+" rejected", d.Reason, note, d.Confidence)
	}
}

func (r *Runtime) observe(ctx context.Context, state *decision.AgentState, d *decision.Decision, result tools.ToolResult) {
	evidence := result.Error
	summary := strings.TrimSpace(d.Tool) + " error"
	if result.Success {
		summary = strings.TrimSpace(d.Tool) + " result"
		evidence = formatToolData(result.Data)
	}
	if n := len(state.ToolCalls); n > 0 {
		state.ToolCalls[n-1].Result = evidence
	}
	state.Observations = append(state.Observations, decision.Observation{
		Summary:    summary,
		Reason:     d.Reason,
		Evidence:   evidence,
		Confidence: d.Confidence,
	})
	logArgs := []any{
		"request_id", requestid.From(ctx),
		"trace_id", requestid.From(ctx),
		"tool", d.Tool,
		"success", result.Success,
		"summary", summary,
		"reason", d.Reason,
		"confidence", d.Confidence,
	}
	if hash := metadataString(result.Metadata, "query_hash"); hash != "" {
		logArgs = append(logArgs, "query_hash", hash, "evidence_len", len(evidence))
	} else {
		logArgs = append(logArgs, "evidence", evidence)
	}
	r.logger.InfoContext(ctx, "agent tool observation", logArgs...)
	r.recordObservation(ctx, state.RunID, state.CurrentStepID, state.Observations[len(state.Observations)-1])
	evidenceOut := evidence
	if n := len(state.ToolCalls); n > 0 {
		r.remember.SaveToolCall(ctx, state.RunID, d.Tool, state.ToolCalls[n-1].Arguments, state.ToolCalls[n-1].Result)
	}
	r.remember.SaveObservation(ctx, state.RunID, summary, d.Reason, evidenceOut, d.Confidence)
}

func (r *Runtime) persistDecision(ctx context.Context, state *decision.AgentState, d *decision.Decision) {
	if r == nil || r.remember == nil || d == nil {
		return
	}
	runID := ""
	if state != nil {
		runID = state.RunID
	}
	r.remember.SaveDecision(ctx, runID, map[string]any{
		"type":            string(d.Type),
		"reason":          d.Reason,
		"objective":       d.Objective,
		"tool":            d.Tool,
		"arguments":       d.Arguments,
		"expected_result": d.ExpectedResult,
		"confidence":      d.Confidence,
		"risk":            d.Risk,
	})
}

func (r *Runtime) recall(ctx context.Context, state *decision.AgentState) {
	if r == nil || r.remember == nil || state == nil {
		return
	}
	query := strings.TrimSpace(state.Goal + " " + state.CurrentObjective)
	recs := r.remember.Recall(ctx, state.RunID, query, r.recallLimit)
	state.Memories = make([]decision.MemoryRecall, 0, len(recs))
	for _, rec := range recs {
		state.Memories = append(state.Memories, decision.MemoryRecall{
			ID:       rec.ID,
			Layer:    string(rec.Layer),
			Kind:     string(rec.Kind),
			Content:  rec.Content,
			Metadata: rec.Metadata,
		})
	}
}

func (r *Runtime) reflectOn(ctx context.Context, state *decision.AgentState, obs decision.Observation, sched *planner.TaskScheduler) (decision.Outcome, *decision.Decision, *planner.TaskScheduler, error) {
	if r == nil || r.reflector == nil {
		return decision.OutcomeContinue, nil, sched, nil
	}

	ref, err := r.reflector.Reflect(ctx, state, obs)
	if err != nil {
		r.logger.WarnContext(ctx, "agent reflection failed",
			"request_id", requestid.From(ctx),
			"run_id", state.RunID,
			"error", err,
		)
		return decision.OutcomeContinue, nil, sched, nil
	}
	if ref == nil {
		return decision.OutcomeContinue, nil, sched, nil
	}
	analyst.GateReflection(state, ref)

	state.RecordReflection(ref.View())
	r.recordReflection(ctx, state.RunID, state.CurrentStepID, ref)
	if r.remember != nil {
		r.remember.SaveObservation(ctx, state.RunID, "reflection: "+ref.NextAction, strings.Join(ref.Errors, "; "), formatToolData(ref), ref.Confidence)
	}

	switch strings.ToUpper(strings.TrimSpace(ref.NextAction)) {
	case reflection.NextFinish:
		d := &decision.Decision{
			Type:       decision.DecisionFinish,
			Reason:     "reflection: goal satisfied",
			Objective:  finishAnswer(ref, state),
			Confidence: ref.Confidence,
			Risk:       "low",
		}
		r.persistDecision(ctx, state, d)
		r.engine.Apply(state, d)
		return decision.OutcomeFinish, d, sched, nil
	case reflection.NextAskUser:
		objective := askUserText(ref)
		if reflection.DataBackendUnavailable(state, lastObservation(state)) {
			objective = reflection.DataBackendMissingAsk
		}
		d := &decision.Decision{
			Type:       decision.DecisionAskUser,
			Reason:     "reflection: data backend unavailable",
			Objective:  objective,
			Confidence: ref.Confidence,
			Risk:       "low",
		}
		r.persistDecision(ctx, state, d)
		r.engine.Apply(state, d)
		return decision.OutcomeAskUser, d, sched, nil
	case reflection.NextReplan:
		forced := &decision.Decision{
			Type:       decision.DecisionReplan,
			Reason:     "reflection: repeated action loop or insufficient plan",
			Objective:  "replan to make progress",
			Confidence: ref.Confidence,
			Risk:       "low",
		}
		r.persistDecision(ctx, state, forced)
		r.engine.Apply(state, forced)
		next, planErr := r.planner.Plan(ctx, state.Goal, *state)
		if planErr != nil {
			return decision.OutcomeAbort, nil, sched, planErr
		}
		sched, err = r.installPlan(ctx, state, next)
		if err != nil {
			return decision.OutcomeAbort, nil, sched, err
		}
		return decision.OutcomeContinue, nil, sched, nil
	default:
		return decision.OutcomeContinue, nil, sched, nil
	}
}

func lastReport(state *decision.AgentState) (*analyst.Report, bool) {
	if state == nil {
		return nil, false
	}
	for i := len(state.Observations) - 1; i >= 0; i-- {
		if r, ok := analyst.ParseReport(state.Observations[i].Evidence); ok {
			return r, true
		}
	}
	for i := len(state.ToolCalls) - 1; i >= 0; i-- {
		if r, ok := analyst.ParseReport(state.ToolCalls[i].Result); ok {
			return r, true
		}
	}
	return nil, false
}

func lastObservation(state *decision.AgentState) decision.Observation {
	if state == nil || len(state.Observations) == 0 {
		return decision.Observation{}
	}
	return state.Observations[len(state.Observations)-1]
}

func finishAnswer(ref *reflection.Reflection, state *decision.AgentState) string {
	if state != nil {
		for i := len(state.Observations) - 1; i >= 0; i-- {
			o := state.Observations[i]
			if strings.HasPrefix(o.Summary, "reflection:") {
				continue
			}
			if ev := strings.TrimSpace(o.Evidence); ev != "" {
				if report, ok := analyst.ParseReport(ev); ok {
					return analyst.MarshalReport(*report)
				}
				return ev
			}
			if s := strings.TrimSpace(o.Summary); s != "" {
				return s
			}
		}
		if obj := strings.TrimSpace(state.CurrentObjective); obj != "" {
			return obj
		}
		if g := strings.TrimSpace(state.Goal); g != "" {
			return g
		}
	}
	if ref != nil && ref.GoalSatisfied {
		return "goal completed"
	}
	return "done"
}

func askUserText(ref *reflection.Reflection) string {
	if ref == nil {
		return "need more information from the user"
	}
	if len(ref.MissingInformation) > 0 {
		return strings.Join(ref.MissingInformation, "; ")
	}
	if len(ref.Errors) > 0 {
		return strings.Join(ref.Errors, "; ")
	}
	return "need more information from the user"
}

func metadataString(meta map[string]any, key string) string {
	if meta == nil {
		return ""
	}
	v, ok := meta[key]
	if !ok || v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func formatToolData(data any) string {
	if data == nil {
		return ""
	}
	if s, ok := data.(string); ok {
		return s
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return fmt.Sprint(data)
	}
	return string(raw)
}

var (
	// ErrEmptyMessage is returned when the user input is blank.
	ErrEmptyMessage = errors.New("message is required")
	// ErrEmptyAnswer is returned when the agent produced no assistant text.
	ErrEmptyAnswer = errors.New("agent returned an empty answer")
)
