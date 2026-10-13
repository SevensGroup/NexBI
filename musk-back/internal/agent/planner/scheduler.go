package planner

import (
	"fmt"
	"sort"
	"strings"
)

// TaskScheduler tracks ready, running, completed, and failed tasks on a DAG.
type TaskScheduler struct {
	plan  *Plan
	tasks map[string]*Task
}

// NewTaskScheduler builds a scheduler for a validated plan.
// Tasks with no remaining dependencies are marked ready (and may run in parallel).
func NewTaskScheduler(plan *Plan) *TaskScheduler {
	s := &TaskScheduler{
		plan:  plan,
		tasks: map[string]*Task{},
	}
	if plan == nil {
		return s
	}
	for i := range plan.Tasks {
		t := &plan.Tasks[i]
		if t.Status == "" {
			t.Status = StatusPending
		}
		s.tasks[t.ID] = t
	}
	s.refreshReady()
	return s
}

// Ready returns tasks that can start now. Multiple results may run in parallel.
func (s *TaskScheduler) Ready() []Task {
	if s == nil {
		return nil
	}
	return s.collect(StatusReady)
}

// Running returns tasks currently in flight.
func (s *TaskScheduler) Running() []Task {
	if s == nil {
		return nil
	}
	return s.collect(StatusRunning)
}

// Completed returns successfully finished tasks.
func (s *TaskScheduler) Completed() []Task {
	if s == nil {
		return nil
	}
	return s.collect(StatusCompleted)
}

// Failed returns tasks that failed execution.
func (s *TaskScheduler) Failed() []Task {
	if s == nil {
		return nil
	}
	return s.collect(StatusFailed)
}

// Len is the number of tasks in the graph.
func (s *TaskScheduler) Len() int {
	if s == nil {
		return 0
	}
	return len(s.tasks)
}

// AllComplete reports whether every task finished successfully.
func (s *TaskScheduler) AllComplete() bool {
	if s == nil || s.Len() == 0 {
		return false
	}
	return len(s.Completed()) == s.Len()
}

// Start marks a ready task as running.
func (s *TaskScheduler) Start(id string) error {
	t, err := s.lookup(id)
	if err != nil {
		return err
	}
	if t.Status != StatusReady {
		return fmt.Errorf("%w: task %q is not ready (status %s)", ErrInvalidPlan, id, t.Status)
	}
	t.Status = StatusRunning
	return nil
}

// Complete marks a running task completed and promotes newly unblocked tasks to ready.
func (s *TaskScheduler) Complete(id string) error {
	t, err := s.lookup(id)
	if err != nil {
		return err
	}
	if t.Status != StatusRunning {
		return fmt.Errorf("%w: task %q is not running (status %s)", ErrInvalidPlan, id, t.Status)
	}
	t.Status = StatusCompleted
	s.refreshReady()
	return nil
}

// Fail marks a running task failed. Dependents stay blocked.
func (s *TaskScheduler) Fail(id string) error {
	t, err := s.lookup(id)
	if err != nil {
		return err
	}
	if t.Status != StatusRunning {
		return fmt.Errorf("%w: task %q is not running (status %s)", ErrInvalidPlan, id, t.Status)
	}
	t.Status = StatusFailed
	s.refreshReady()
	return nil
}

// Snapshot copies the plan with current task statuses so a run can resume from a checkpoint.
func (s *TaskScheduler) Snapshot() *Plan {
	if s == nil || s.plan == nil {
		return nil
	}
	tasks := make([]Task, 0, len(s.plan.Tasks))
	for _, t := range s.plan.Tasks {
		cloned := cloneTask(t)
		if live, ok := s.tasks[t.ID]; ok && live != nil {
			cloned.Status = live.Status
		}
		tasks = append(tasks, cloned)
	}
	return &Plan{Goal: s.plan.Goal, Tasks: tasks}
}

// NextReady returns the highest-priority ready task, if any.
func (s *TaskScheduler) NextReady() *Task {
	ready := s.Ready()
	if len(ready) == 0 {
		return nil
	}
	t := s.tasks[ready[0].ID]
	return t
}

func (s *TaskScheduler) lookup(id string) (*Task, error) {
	if s == nil {
		return nil, fmt.Errorf("%w: scheduler is required", ErrInvalidPlan)
	}
	id = strings.TrimSpace(id)
	t, ok := s.tasks[id]
	if !ok {
		return nil, fmt.Errorf("%w: unknown task %q", ErrInvalidPlan, id)
	}
	return t, nil
}

func (s *TaskScheduler) completedSet() map[string]bool {
	done := make(map[string]bool, len(s.tasks))
	for id, t := range s.tasks {
		if t.Status == StatusCompleted {
			done[id] = true
		}
	}
	return done
}

func (s *TaskScheduler) refreshReady() {
	done := s.completedSet()
	for _, t := range s.tasks {
		if t.Status != StatusPending && t.Status != StatusReady {
			continue
		}
		if dependenciesSatisfied(*t, done) {
			t.Status = StatusReady
		} else {
			t.Status = StatusPending
		}
	}
}

func (s *TaskScheduler) collect(status string) []Task {
	out := make([]Task, 0)
	for _, t := range s.tasks {
		if t.Status == status {
			out = append(out, cloneTask(*t))
		}
	}
	sortTasks(out)
	return out
}

func sortTasks(tasks []Task) {
	sort.SliceStable(tasks, func(i, j int) bool {
		if tasks[i].Priority != tasks[j].Priority {
			return tasks[i].Priority > tasks[j].Priority
		}
		return tasks[i].ID < tasks[j].ID
	})
}
