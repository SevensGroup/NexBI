package planner

import (
	"context"
	"fmt"
	"strings"
)

// MockTool is a stand-in executor used to verify Goal → Plan → Task Graph → Complete
// in planner tests. Runtime Act uses the Tool Registry (calculator), not this mock.
type MockTool struct{}

// NewMockTool constructs the verification mock.
func NewMockTool() *MockTool {
	return &MockTool{}
}

// Name identifies the mock tool.
func (m *MockTool) Name() string {
	return "mock"
}

// Execute returns a deterministic observation from the task's ExpectedOutput.
func (m *MockTool) Execute(ctx context.Context, task Task) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	expected := strings.TrimSpace(task.ExpectedOutput)
	if expected == "" {
		return "", fmt.Errorf("%w: task %q missing expected_output", ErrInvalidPlan, task.ID)
	}
	title := strings.TrimSpace(task.Title)
	if title == "" {
		title = task.ID
	}
	return fmt.Sprintf("mock completed %s: %s", title, expected), nil
}

// ExecuteMock walks the DAG in ready waves. Independent tasks in a wave are started
// together (parallel-eligible) before they are completed.
func ExecuteMock(ctx context.Context, sched *TaskScheduler, tool *MockTool) error {
	if sched == nil {
		return fmt.Errorf("%w: scheduler is required", ErrInvalidPlan)
	}
	if tool == nil {
		tool = NewMockTool()
	}
	if sched.Len() == 0 {
		return fmt.Errorf("%w: at least one task is required", ErrInvalidPlan)
	}

	limit := sched.Len() + 1
	for i := 0; i < limit; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if sched.AllComplete() {
			return nil
		}

		ready := sched.Ready()
		if len(ready) == 0 {
			if len(sched.Failed()) > 0 {
				return fmt.Errorf("%w: task graph is stuck after failures", ErrInvalidPlan)
			}
			return fmt.Errorf("%w: no ready tasks (illegal or cyclic graph)", ErrInvalidPlan)
		}

		wave := make([]Task, 0, len(ready))
		for _, t := range ready {
			if err := sched.Start(t.ID); err != nil {
				return err
			}
			wave = append(wave, t)
		}

		for _, t := range wave {
			if _, err := tool.Execute(ctx, t); err != nil {
				_ = sched.Fail(t.ID)
				return err
			}
			if err := sched.Complete(t.ID); err != nil {
				return err
			}
		}
	}

	if !sched.AllComplete() {
		return fmt.Errorf("%w: mock execution did not complete", ErrInvalidPlan)
	}
	return nil
}
