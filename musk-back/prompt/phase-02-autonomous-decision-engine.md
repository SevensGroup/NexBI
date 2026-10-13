Continue from the Phase 1 project.

Implement the Autonomous Decision Engine.

Goal:

Upgrade the agent from:

LLM → Answer

to:

Goal
→ Understand
→ Decide
→ Act
→ Observe
→ Evaluate
→ Continue / Finish

Implement:

internal/agent/decision/

Include:

- engine.go
- decision.go
- policy.go
- validator.go

Define:

DecisionType:

- CALL_TOOL
- DELEGATE_AGENT
- REPLAN
- REFLECT
- ASK_USER
- FINISH
- ABORT

Decision:

type Decision struct {
    Type DecisionType
    Reason string
    Objective string
    Tool string
    Arguments map[string]any
    ExpectedResult string
    Confidence float64
    Risk string
}

Requirements:

1. The LLM must output a structured Decision.
2. Constrain structured output with JSON Schema or Eino.
3. Every Decision must pass the Validator.
4. Confidence must be between 0 and 1.
5. Reject invalid Decisions.
6. Make the maximum step count configurable:
   AGENT_MAX_STEPS.
7. Prevent repeating an identical action.
8. Support:
   CONTINUE
   FINISH
   ASK_USER
   ABORT
9. The agent must not fabricate a tool result.
10. Log every Decision.

Implement the Decision Engine:

DecisionEngine.Decide(ctx, state) (*Decision, error)

Add:

AgentState:

- Goal
- CurrentObjective
- Plan
- CompletedTasks
- FailedTasks
- Observations
- ToolCalls
- CurrentStep
- MaxSteps
- Confidence
- Finished

Add unit tests:

- valid decision
- invalid decision
- max steps
- duplicate action
- finish decision
- abort decision

Focus:

Do not store the model's full chain of thought.

Store only:

decision summary
reason
evidence
confidence

When finished:

go test ./...
go vet ./...
go build ./...
