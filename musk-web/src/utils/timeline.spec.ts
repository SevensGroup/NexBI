import { describe, expect, it } from 'vitest'
import { timelineDetailCopy } from './timeline'
import type { AgentEvent } from '@/types/events'

describe('timelineDetailCopy', () => {
  it('keeps the full decision reasoning, not a truncated timeline subtitle', () => {
    const reason =
      'Currently on plan step 1 "data collection and baseline comparison"; need to pull yesterday Meta ad ROI, spend, revenue, impressions, clicks, CPM, CTR, and CVR as baseline comparison inputs'
    const event: AgentEvent = {
      id: 'e-decision',
      runId: 'rid',
      step: 1,
      type: 'decision.created',
      timestamp: '2026-08-26T02:52:09Z',
      payload: {
        decision: {
          id: 'd1',
          runId: 'rid',
          step: 1,
          type: 'DELEGATE_AGENT',
          objective: 'data collection and baseline comparison',
          summary: reason,
          reason,
          confidence: 0.72,
          risk: 'LOW',
          nextAction: 'DELEGATE_AGENT',
          timestamp: '2026-08-26T02:52:09Z',
          agent: 'analyst',
        },
      },
    }
    const copy = timelineDetailCopy(event)
    expect(copy).toContain('DELEGATE_AGENT')
    expect(copy).toContain(reason)
    expect(copy).toContain('analyst')
    expect(copy.endsWith('...')).toBe(false)
  })

  it('surfaces the exact backend error on agent.failed', () => {
    const error = 'invalid decision: agent is required for DELEGATE_AGENT'
    const copy = timelineDetailCopy({
      id: 'e-fail',
      runId: 'rid',
      step: 4,
      type: 'agent.failed',
      timestamp: '2026-08-26T02:52:24Z',
      payload: { error },
    })
    expect(copy).toBe(error)
  })
})
