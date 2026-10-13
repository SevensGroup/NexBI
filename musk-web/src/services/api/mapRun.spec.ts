import { describe, expect, it } from 'vitest'
import { eventsFromBackendDelta, normalizeReport } from './mapRun'
import type { BackendRun } from '@/types/run'

const base = (over: Partial<BackendRun> = {}): BackendRun => ({
  id: 'rid-1',
  goal: 'Analyze why Meta ad ROI declined yesterday',
  status: 'RUNNING',
  created_at: '2026-08-26T02:00:00Z',
  updated_at: '2026-08-26T02:00:01Z',
  decisions: [],
  tool_calls: [],
  observations: [],
  reflections: [],
  ...over,
})

describe('eventsFromBackendDelta', () => {
  it('emits only new decisions on subsequent snapshots', () => {
    const first = base({
      decisions: [{ type: 'CALL_TOOL', reason: 'query', confidence: 0.8, at: '2026-08-26T02:00:02Z' }],
    })
    const boot = eventsFromBackendDelta(null, first, { skipBootstrap: true })
    expect(boot.some((e) => e.type === 'agent.started')).toBe(false)
    expect(boot.filter((e) => e.type === 'decision.created')).toHaveLength(1)

    const second = base({
      decisions: [
        { type: 'CALL_TOOL', reason: 'query', confidence: 0.8, at: '2026-08-26T02:00:02Z' },
        { type: 'FINISH', reason: 'done', confidence: 0.9, at: '2026-08-26T02:00:10Z' },
      ],
    })
    const delta = eventsFromBackendDelta(first, second, { skipBootstrap: true })
    expect(delta.filter((e) => e.type === 'decision.created')).toHaveLength(1)
    expect((delta[0].payload.decision as { type: string }).type).toBe('FINISH')
  })

  it('maps snake_case report fields from musk-back', () => {
    const report = normalizeReport({
      summary: 'ROAS dropped',
      metrics: { roas: 1.4 },
      root_causes: [{ statement: 'creative fatigue', evidence_ids: ['ev-1'] }],
      recommendations: [{ action: 'pause CR-1', evidence_ids: ['ev-1'] }],
      anomalies: [{ entity: 'Campaign', name: 'US', metric: 'ROAS', change_pct: -0.2, evidence_id: 'ev-1' }],
      confidence: 0.91,
    })
    expect(report.rootCauses[0].evidenceIds).toEqual(['ev-1'])
    expect(report.anomalies[0].evidenceId).toBe('ev-1')
    expect(report.anomalies[0].changePct).toBe(-0.2)
  })

  it('maps FAILED snapshots to agent.failed with the backend error string', () => {
    const failed = base({
      status: 'FAILED',
      updated_at: '2026-08-26T02:52:23Z',
      error: 'invalid decision: agent is required for DELEGATE_AGENT',
      decisions: [
        {
          type: 'DELEGATE_AGENT',
          reason:
            'Currently on plan step 1 "data collection and baseline comparison"; need to pull yesterday Meta ad ROI, spend, revenue, impressions, clicks, CPM, CTR, and CVR',
          confidence: 0.72,
          at: '2026-08-26T02:52:09Z',
        },
      ],
    })
    const events = eventsFromBackendDelta(null, failed)
    const failEv = events.find((e) => e.type === 'agent.failed')
    expect(failEv).toBeTruthy()
    expect(failEv?.payload.error).toBe('invalid decision: agent is required for DELEGATE_AGENT')
    const decision = events.find((e) => e.type === 'decision.created')
    expect((decision?.payload.decision as { reason: string }).reason).toContain('data collection and baseline comparison')
  })

  it('emits agent.failed when polling sees FAILED even with skipBootstrap', () => {
    const running = base({ status: 'RUNNING' })
    const failed = base({
      status: 'FAILED',
      error: 'invalid decision: agent is required for DELEGATE_AGENT',
    })
    const events = eventsFromBackendDelta(running, failed, { skipBootstrap: true })
    expect(events.some((e) => e.type === 'agent.started')).toBe(false)
    expect(events.find((e) => e.type === 'agent.failed')?.payload.error).toBe(
      'invalid decision: agent is required for DELEGATE_AGENT',
    )
  })
})
