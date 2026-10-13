Continue from Phase 2.

Implement the Planner.

Goal:

Let the agent break a complex user goal into a task graph.

Create:

internal/agent/planner/

Files:

- planner.go
- task.go
- dependency.go

Task:

type Task struct {
    ID string
    Title string
    Description string
    Dependencies []string
    Priority int
    Status string
    ExpectedOutput string
}

Planner:

Plan(ctx context.Context, goal string, state AgentState) (*Plan, error)

Plan:

type Plan struct {
    Goal string
    Tasks []Task
}

Requirements:

1. The LLM decomposes the goal into tasks.
2. Every task must have a clear ExpectedOutput.
3. Detect dependencies automatically.
4. Support a DAG.
5. Tasks with no dependencies may run in parallel.
6. Do not split tasks without a bound.
7. MAX_TASKS must be configurable.
8. Planner output must be structured.
9. Detect an invalid DAG.
10. Detect circular dependencies.

Add:

TaskScheduler

It owns:

- ready tasks
- running tasks
- completed tasks
- failed tasks

Do not connect Elasticsearch yet.

Verify the flow with a mock tool:

Goal
→ Plan
→ Task Graph
→ Execute Mock
→ Complete

Tests must cover:

- single task
- multi task
- dependency
- parallel tasks
- circular dependency
