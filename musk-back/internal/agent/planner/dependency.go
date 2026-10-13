package planner

import (
	"fmt"
	"strings"
)

// ValidatePlan checks task fields, MAX_TASKS, and that the graph is a legal DAG.
func ValidatePlan(plan *Plan, maxTasks int) error {
	if plan == nil {
		return fmt.Errorf("%w: plan is nil", ErrInvalidPlan)
	}
	plan.Goal = strings.TrimSpace(plan.Goal)
	if len(plan.Tasks) == 0 {
		return fmt.Errorf("%w: at least one task is required", ErrInvalidPlan)
	}
	if maxTasks > 0 && len(plan.Tasks) > maxTasks {
		return fmt.Errorf("%w: %d tasks exceeds MAX_TASKS %d", ErrTooManyTasks, len(plan.Tasks), maxTasks)
	}

	used := make(map[string]struct{}, len(plan.Tasks))
	for i := range plan.Tasks {
		normalizeTask(&plan.Tasks[i], i, used)
		if err := validateTaskFields(plan.Tasks[i]); err != nil {
			return err
		}
	}

	ids := make(map[string]struct{}, len(plan.Tasks))
	for _, t := range plan.Tasks {
		if _, dup := ids[t.ID]; dup {
			return fmt.Errorf("%w: duplicate task id %q", ErrInvalidPlan, t.ID)
		}
		ids[t.ID] = struct{}{}
	}
	dropUnknownDependencies(plan.Tasks, ids)

	return ValidateDAG(plan.Tasks)
}

func validateTaskFields(t Task) error {
	if t.ID == "" {
		return fmt.Errorf("%w: task id is required", ErrInvalidPlan)
	}
	if t.Title == "" {
		return fmt.Errorf("%w: task %q title is required", ErrInvalidPlan, t.ID)
	}
	if t.ExpectedOutput == "" {
		return fmt.Errorf("%w: task %q must have a clear expected_output", ErrInvalidPlan, t.ID)
	}
	return nil
}

// ValidateDAG rejects unknown / self dependencies and circular dependencies.
func ValidateDAG(tasks []Task) error {
	ids := make(map[string]struct{}, len(tasks))
	for _, t := range tasks {
		id := strings.TrimSpace(t.ID)
		if id == "" {
			return fmt.Errorf("%w: task id is required", ErrInvalidPlan)
		}
		if _, dup := ids[id]; dup {
			return fmt.Errorf("%w: duplicate task id %q", ErrInvalidPlan, id)
		}
		ids[id] = struct{}{}
	}

	for _, t := range tasks {
		for _, dep := range t.Dependencies {
			if dep == t.ID {
				return fmt.Errorf("%w: task %q depends on itself", ErrCyclicDependency, t.ID)
			}
			if _, ok := ids[dep]; !ok {
				return fmt.Errorf("%w: task %q depends on unknown id %q", ErrInvalidPlan, t.ID, dep)
			}
		}
	}

	if hasCycle(tasks) {
		return fmt.Errorf("%w", ErrCyclicDependency)
	}
	return nil
}

func dropUnknownDependencies(tasks []Task, ids map[string]struct{}) {
	for i := range tasks {
		if len(tasks[i].Dependencies) == 0 {
			continue
		}
		kept := make([]string, 0, len(tasks[i].Dependencies))
		for _, dep := range tasks[i].Dependencies {
			if _, ok := ids[dep]; ok {
				kept = append(kept, dep)
			}
		}
		tasks[i].Dependencies = kept
	}
}

func hasCycle(tasks []Task) bool {
	indegree := make(map[string]int, len(tasks))
	outgoing := make(map[string][]string, len(tasks))
	for _, t := range tasks {
		indegree[t.ID] = 0
	}
	for _, t := range tasks {
		seen := make(map[string]struct{}, len(t.Dependencies))
		for _, dep := range t.Dependencies {
			if _, ok := seen[dep]; ok {
				continue
			}
			seen[dep] = struct{}{}
			if _, ok := indegree[dep]; !ok {
				return true
			}
			indegree[t.ID]++
			outgoing[dep] = append(outgoing[dep], t.ID)
		}
	}

	queue := make([]string, 0, len(tasks))
	for id, n := range indegree {
		if n == 0 {
			queue = append(queue, id)
		}
	}

	seen := 0
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		seen++
		for _, next := range outgoing[id] {
			indegree[next]--
			if indegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}
	return seen != len(tasks)
}

// ReadyTasks returns pending/ready tasks whose dependencies are all completed.
// Independent tasks are returned together so they may run in parallel.
func ReadyTasks(tasks []Task, completed map[string]bool) []Task {
	if completed == nil {
		completed = map[string]bool{}
	}
	var ready []Task
	for _, t := range tasks {
		switch t.Status {
		case StatusCompleted, StatusFailed, StatusRunning:
			continue
		}
		if dependenciesSatisfied(t, completed) {
			ready = append(ready, t)
		}
	}
	sortTasks(ready)
	return ready
}

func dependenciesSatisfied(t Task, completed map[string]bool) bool {
	for _, dep := range t.Dependencies {
		if !completed[dep] {
			return false
		}
	}
	return true
}
