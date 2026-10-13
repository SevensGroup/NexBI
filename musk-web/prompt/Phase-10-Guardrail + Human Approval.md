# Musk Web Phase 10 — Guardrail + Human Approval

Implement guardrails and human approval.
Risk levels: LOW, MEDIUM, HIGH, CRITICAL.
Approval: id, runId, action, description, risk, reason, confidence, status (PENDING/APPROVED/REJECTED).
Page: /approvals.
Create ApprovalCenter, ApprovalCard, RiskBadge, and ApprovalDetailDrawer.
Show the high-risk action, current value, proposed value, reason, and confidence.
Buttons: Reject and Approve.
Timeline: Decision → Risk Check → WAITING_APPROVAL → Approval → Resume → Tool.
Show NEEDS APPROVAL at the top.

When finished, run npm run build and confirm there are no obvious TypeScript errors, unhandled exceptions, or meaningless TODOs.
