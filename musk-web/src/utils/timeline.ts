import type { AgentEvent } from '@/types/events'
import type { Decision } from '@/types/decision'
import type { Observation, ToolCall } from '@/types/tool'
import type { Reflection } from '@/types/reflection'

export const SAMPLE_RUN_IDS = new Set(['run-ads-01', 'run-hitl-01', 'run-fail-01'])

export function isSampleRunId(id: string): boolean {
  return SAMPLE_RUN_IDS.has(id)
}

export function timelineDetailCopy(event: AgentEvent | null | undefined): string {
  if (!event) return ''
  const p = event.payload
  switch (event.type) {
    case 'decision.created': {
      const d = p.decision as Decision | undefined
      if (!d) return JSON.stringify(p)
      return [
        `Type: ${d.type}`,
        d.objective ? `Objective: ${d.objective}` : '',
        d.summary ? `Decision: ${d.summary}` : '',
        d.reason ? `Reasoning summary: ${d.reason}` : '',
        d.agent ? `Agent: ${d.agent}` : '',
        d.tool ? `Tool: ${d.tool}` : '',
        `Confidence: ${d.confidence}`,
        `Risk: ${d.risk}`,
        d.nextAction ? `Next action: ${d.nextAction}` : '',
        d.arguments ? `Arguments: ${JSON.stringify(d.arguments, null, 2)}` : '',
      ]
        .filter(Boolean)
        .join('\n')
    }
    case 'observation.created': {
      const o = p.observation as Observation | undefined
      if (!o) return String(p.summary ?? '')
      return [o.summary, o.reason, o.evidence].filter(Boolean).join('\n')
    }
    case 'reflection.created': {
      const r = p.reflection as Reflection | undefined
      if (!r) return String(p.summary ?? p.nextAction ?? '')
      return [
        r.summary,
        `Next action: ${r.nextAction}`,
        `Goal satisfied: ${r.goalSatisfied}`,
        `Evidence sufficient: ${r.evidenceSufficient}`,
        r.missingInformation?.length ? `Missing: ${r.missingInformation.join('; ')}` : '',
      ]
        .filter(Boolean)
        .join('\n')
    }
    case 'tool.started':
    case 'tool.completed':
    case 'tool.failed': {
      const tc = p.toolCall as ToolCall | undefined
      const err = p.error != null ? String(p.error) : tc?.error
      return [tc?.toolName, tc?.description, err, tc?.result != null ? JSON.stringify(tc.result, null, 2) : '']
        .filter(Boolean)
        .join('\n')
    }
    case 'agent.failed':
      return String(p.error ?? 'agent failed')
    case 'goal.understood':
    case 'agent.started':
      return String(p.summary ?? p.goal ?? p.raw ?? '')
    default:
      try {
        return JSON.stringify(p, null, 2)
      } catch {
        return event.type
      }
  }
}
