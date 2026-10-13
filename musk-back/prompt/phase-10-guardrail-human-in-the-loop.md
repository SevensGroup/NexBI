Continue development.

Implement agent safety controls.

Directory:

internal/guardrail/

Implement a Risk Engine.

Risk levels:

LOW
MEDIUM
HIGH
CRITICAL

Rules:

LOW:

query data
calculate
search

MEDIUM:

large queries
long-running tasks

HIGH:

modify business data
modify ad configuration
call an external API

CRITICAL:

delete
funds
irreversible production operations

Requirements:

LOW runs automatically.

MEDIUM runs according to configuration.

HIGH and CRITICAL require:

Agent
→ Approval Request
→ User Confirm
→ Execute

Implement:

ApprovalManager

Support:

CreateApproval
Approve
Reject
Expire

Agent state supports:

WAITING_APPROVAL

and:

RESUME

Add:

interrupt / resume

A task must be able to resume from a checkpoint.

Test:

Agent
→ HIGH risk action
→ Interrupt
→ Approval
→ Resume
→ Execute
→ Finish
