# Musk Web Phase 2 — Autonomous Decision UI

Visualize autonomous decisions. Define DecisionType: CALL_TOOL, DELEGATE_AGENT, REPLAN, REFLECT, ASK_USER, FINISH, ABORT.
Decision fields: id, runId, step, type, objective, summary, reason, confidence, risk, nextAction, timestamp.
Create DecisionCard, ConfidenceBar, RiskBadge, DecisionTimelineItem, ReasoningSummary, and AgentStateBadge.
Show Goal, Decision, Reason, Confidence, Risk, and Next Action.
Do not show the full chain of thought. Show only the reasoning summary.
Implement UNDERSTANDING, PLANNING, DECIDING, EXECUTING, OBSERVING, REFLECTING, WAITING_APPROVAL, COMPLETED, and FAILED, with restrained motion.

When finished, run npm run build and confirm there are no obvious TypeScript errors, unhandled exceptions, or meaningless TODOs.
