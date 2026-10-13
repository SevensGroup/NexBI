Continue the project.

Implement the Reflection Engine.

Directory:

internal/agent/reflection/

Goal:

After a tool runs, the agent judges whether the result is sufficient before it finishes.

Implement:

Reflection:

type Reflection struct {
    GoalSatisfied bool
    EvidenceSufficient bool
    ResultCorrect bool
    MissingInformation []string
    Errors []string
    NextAction string
    Confidence float64
}

ReflectionEngine:

Reflect(ctx, state, observation)

Reflection must answer:

1. Is the goal complete?
2. Is the evidence sufficient?
3. Is the data complete?
4. Is there an anomaly?
5. Should the agent continue?
6. What is the best next action?

Support:

CONTINUE
REPLAN
FINISH
ASK_USER

Wire it into the Decision Engine.

Final loop:

Decision
→ Tool
→ Observation
→ Reflection
→ Decision

Requirements:

Cap execution at MAX_STEPS.

Add a loop guard.

If the same decision, the same tool, and the same arguments repeat three times in a row, trigger:

REPLAN

Finish the tests.
