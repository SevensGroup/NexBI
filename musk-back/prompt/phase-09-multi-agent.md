Continue development.

Add multi-agent collaboration.

Architecture:

Coordinator
├── Analyst
├── Researcher
└── Reviewer

Implement this with Eino Agent-as-Tool, or another agent orchestration approach that fits the codebase.

Coordinator is responsible for:

Goal
Plan
Delegate
Review
Final

Analyst is responsible for:

data analysis
metric analysis
anomaly detection

Researcher is responsible for:

external search
industry context
background material

Reviewer is responsible for:

verifying conclusions
verifying evidence
finding logical errors

Requirements:

Each agent has:

- its own system prompt
- its own tool scope
- its own memory scope
- an explicit input and output schema

The Coordinator can:

DELEGATE_AGENT

Example:

{
  "agent": "analyst",
  "task": "Analyze why Meta ROAS declined yesterday"
}

The Reviewer must be able to reject a wrong conclusion:

{
  "approved": false,
  "reason": "insufficient evidence"
}

If the Reviewer rejects:

the Coordinator must:

REPLAN

and run again.

Add complete tests.
