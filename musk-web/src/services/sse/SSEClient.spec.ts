import { describe, expect, it } from 'vitest'
import { parseSSEChunk } from '@/services/sse/SSEClient'

describe('parseSSEChunk', () => {
  it('parses id, event, and multiline data', () => {
    const parsed = parseSSEChunk('id: 42\nevent: decision.created\ndata: {"a":1}\ndata: {"b":2}')
    expect(parsed?.id).toBe('42')
    expect(parsed?.event).toBe('decision.created')
    expect(parsed?.data).toBe('{"a":1}\n{"b":2}')
  })

  it('ignores comments and empty chunks', () => {
    expect(parseSSEChunk(': heartbeat')).toBeNull()
    expect(parseSSEChunk('')).toBeNull()
  })
})
