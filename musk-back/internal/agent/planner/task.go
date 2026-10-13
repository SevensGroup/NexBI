package planner

import (
	"errors"
	"fmt"
	"strings"
)

const (
	// StatusPending is a task that has not become ready yet.
	StatusPending = "pending"
	// StatusReady is a task whose dependencies are satisfied and may run (including in parallel).
	StatusReady = "ready"
	// StatusRunning is a task currently being executed.
	StatusRunning = "running"
	// StatusCompleted is a successfully finished task.
	StatusCompleted = "completed"
	// StatusFailed is a task that failed execution.
	StatusFailed = "failed"

	defaultMaxTasks = 12
)

var (
	// ErrInvalidPlan is returned when a structured plan fails validation.
	ErrInvalidPlan = errors.New("invalid plan")
	// ErrCyclicDependency is returned when the task graph contains a cycle.
	ErrCyclicDependency = errors.New("circular dependency")
	// ErrTooManyTasks is returned when the plan exceeds MAX_TASKS.
	ErrTooManyTasks = errors.New("too many tasks")
	// ErrEmptyGoal is returned when Plan is called without a goal.
	ErrEmptyGoal = errors.New("goal is required")
)

// Task is one node in the planned DAG.
type Task struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	Dependencies   []string `json:"dependencies"`
	Priority       int      `json:"priority"`
	Status         string   `json:"status"`
	ExpectedOutput string   `json:"expected_output"`
}

// Plan is a structured task graph for a user goal.
type Plan struct {
	Goal  string `json:"goal"`
	Tasks []Task `json:"tasks"`
}

// Summaries returns stable "id: title" lines for AgentState.Plan.
func (p *Plan) Summaries() []string {
	if p == nil {
		return []string{}
	}
	out := make([]string, 0, len(p.Tasks))
	for _, t := range p.Tasks {
		id := strings.TrimSpace(t.ID)
		title := strings.TrimSpace(t.Title)
		switch {
		case id != "" && title != "":
			out = append(out, id+": "+title)
		case title != "":
			out = append(out, title)
		case id != "":
			out = append(out, id)
		}
	}
	return out
}

func normalizeTask(t *Task, index int, used map[string]struct{}) {
	t.ID = strings.TrimSpace(t.ID)
	t.Title = strings.TrimSpace(t.Title)
	t.Description = strings.TrimSpace(t.Description)
	t.ExpectedOutput = strings.TrimSpace(t.ExpectedOutput)
	t.Status = StatusPending
	if t.Priority < 0 {
		t.Priority = 0
	}
	t.Dependencies = uniqueIDs(t.Dependencies)

	if t.ID == "" {
		n := index + 1
		for {
			id := fmt.Sprintf("t%d", n)
			if _, exists := used[id]; !exists {
				t.ID = id
				break
			}
			n++
		}
	}
	used[t.ID] = struct{}{}

	if t.Description == "" {
		t.Description = t.Title
	}
}

func uniqueIDs(ids []string) []string {
	if len(ids) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func cloneTask(t Task) Task {
	t.Dependencies = append([]string(nil), t.Dependencies...)
	return t
}
