package reflection

import (
	"autonomous-agent/internal/agent/decision"
)

// LoopDetected reports whether the last ConsecutiveRepeatLimit applied
// decisions (or tool calls) share the same decision type, tool, and arguments.
func LoopDetected(state *decision.AgentState) bool {
	if state == nil {
		return false
	}
	return state.ConsecutiveActionRepeats(ConsecutiveRepeatLimit) ||
		state.ConsecutiveToolRepeats(ConsecutiveRepeatLimit)
}

func loopReflection() *Reflection {
	return &Reflection{
		GoalSatisfied:      false,
		EvidenceSufficient: false,
		ResultCorrect:      false,
		MissingInformation: []string{},
		Errors:             []string{"same decision, tool, and arguments repeated three times"},
		NextAction:         NextReplan,
		Confidence:         1,
	}
}
