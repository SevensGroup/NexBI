import { describe, expect, it } from 'vitest'
import { isProxy, isReactive, reactive } from 'vue'
import { applyAgentEvent, createEmptyRunState } from '@/stores/reducer'
import { clonePlain, deepToRaw } from '@/utils/clone'
import { buildAdvertisingEvents, buildHitlEvents } from '@/mocks/events'

describe('applyAgentEvent', () => {
  it('maps advertising analyst events to a completed report', () => {
    let state = createEmptyRunState()
    for (const event of buildAdvertisingEvents('run-test')) {
      state = applyAgentEvent(state, event)
    }
    expect(state.status).toBe('COMPLETED')
    expect(state.understanding?.channel).toBe('Meta')
    expect(state.plan?.tasks.length).toBe(8)
    expect(state.decisions.some((d) => d.type === 'CALL_TOOL')).toBe(true)
    expect(state.decisions.some((d) => d.type === 'DELEGATE_AGENT')).toBe(true)
    expect(state.toolCalls.length).toBeGreaterThan(0)
    expect(state.observations.length).toBeGreaterThan(0)
    expect(state.reflections.at(-1)?.goalSatisfied).toBe(true)
    expect(state.report?.confidence).toBeGreaterThan(0.8)
    expect(state.timeline.some((t) => t.kind === 'semantic')).toBe(true)
    expect(state.timeline.some((t) => t.kind === 'mcp')).toBe(true)
  })

  it('enters WAITING_APPROVAL on approval.required', () => {
    let state = createEmptyRunState()
    for (const event of buildHitlEvents('run-hitl')) {
      state = applyAgentEvent(state, event)
    }
    expect(state.status).toBe('WAITING_APPROVAL')
    expect(state.approvals[0]?.status).toBe('PENDING')
  })

  it('does not expose chain-of-thought fields on decisions', () => {
    let state = createEmptyRunState()
    state = applyAgentEvent(state, {
      id: 'e1',
      runId: 'r',
      step: 1,
      type: 'decision.created',
      timestamp: '2026-08-26T00:00:00Z',
      payload: {
        decision: {
          id: 'd',
          runId: 'r',
          step: 1,
          type: 'REFLECT',
          objective: 'check',
          summary: 'short summary',
          reason: 'reasoning summary only',
          confidence: 0.5,
          risk: 'LOW',
          nextAction: 'REFLECT',
          timestamp: '2026-08-26T00:00:00Z',
        },
      },
    })
    expect(state.decisions[0]?.reason).toBe('reasoning summary only')
    expect(JSON.stringify(state.decisions[0])).not.toMatch(/chain of thought/i)
  })

  it('clones Vue reactive / Pinia-like state without DataCloneError', () => {
    const prev = reactive(createEmptyRunState())
    expect(isReactive(prev)).toBe(true)
    expect(() => structuredClone(prev)).toThrow()

    const next = applyAgentEvent(prev, {
      id: 'e1',
      runId: 'r',
      step: 0,
      type: 'agent.started',
      timestamp: '2026-08-26T00:00:00Z',
      payload: { goal: 'Why did Meta ad ROI decline yesterday?' },
    })

    expect(next.goal).toContain('Meta')
    expect(next).not.toBe(prev)
    expect(isProxy(next)).toBe(false)

    prev.goal = 'mutated-source'
    expect(next.goal).toContain('Meta')

    const second = applyAgentEvent(reactive(next), {
      id: 'e2',
      runId: 'r',
      step: 0,
      type: 'goal.understood',
      timestamp: '2026-08-26T00:00:01Z',
      payload: { raw: next.goal, channel: 'Meta', metric: 'ROAS', summary: 'understood' },
    })
    expect(second.understanding?.channel).toBe('Meta')
    expect(second.events).toHaveLength(2)
  })
})

describe('clonePlain', () => {
  it('unwraps nested Vue proxies into structuredClone-able objects', () => {
    const nested = reactive({
      runId: 'r1',
      plan: { id: 'p', goal: 'g', tasks: [{ id: 't1', title: 'Get Data' }] },
    })
    const plain = clonePlain(nested)
    expect(plain.plan.tasks[0].title).toBe('Get Data')
    expect(isProxy(plain)).toBe(false)
    expect(isProxy(deepToRaw(nested))).toBe(false)
    nested.plan.goal = 'changed'
    expect(plain.plan.goal).toBe('g')
  })
})

describe('agent.failed', () => {
  it('sets FAILED and appends a timeline error event with the backend message', () => {
    const error = 'invalid decision: agent is required for DELEGATE_AGENT'
    let state = createEmptyRunState()
    state = applyAgentEvent(state, {
      id: 'e1',
      runId: '0490b2cd-4950-4c99-a118-ce5d1f3d575a',
      step: 0,
      type: 'agent.started',
      timestamp: '2026-08-26T02:51:29Z',
      payload: { goal: 'Why did Meta ad ROI decline yesterday?' },
    })
    state = applyAgentEvent(state, {
      id: 'e2',
      runId: '0490b2cd-4950-4c99-a118-ce5d1f3d575a',
      step: 3,
      type: 'agent.failed',
      timestamp: '2026-08-26T02:52:23Z',
      payload: { error },
    })
    expect(state.status).toBe('FAILED')
    expect(state.error).toBe(error)
    const failed = state.timeline.find((t) => t.kind === 'error' && t.title === 'Failed')
    expect(failed?.subtitle).toBe(error)
  })
})
