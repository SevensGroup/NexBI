import { describe, expect, it, vi, beforeEach } from 'vitest'

vi.mock('./client', () => ({
  api: {
    post: vi.fn(),
    get: vi.fn(),
  },
  isMockMode: () => false,
}))

import { api } from './client'
import { extractApiError, runRequestBody, startAgentRun } from './agent'
import { AxiosError } from 'axios'

describe('extractApiError', () => {
  it('maps musk-back { error } from axios response body', () => {
    const err = new AxiosError('Request failed with status code 502')
    Object.assign(err, {
      response: {
        data: { request_id: 'rid-1', error: 'invalid decision: agent is required for DELEGATE_AGENT' },
        status: 502,
      },
    })
    expect(extractApiError(err)).toBe('invalid decision: agent is required for DELEGATE_AGENT')
  })
})

describe('startAgentRun', () => {
  beforeEach(() => {
    vi.mocked(api.post).mockReset()
  })

  it('POSTs only { message } to /api/v1/agent/run with X-Request-ID', async () => {
    vi.mocked(api.post).mockResolvedValue({
      data: { run_id: 'rid-1', status: 'COMPLETED', request_id: 'rid-1' },
      headers: { 'x-request-id': 'rid-1' },
    })
    const message = 'Analyze why Meta ad ROI declined yesterday'
    await startAgentRun({ message, runId: 'rid-1' })
    expect(runRequestBody(message)).toEqual({ message })
    expect(runRequestBody(message)).not.toHaveProperty('goal')
    expect(api.post).toHaveBeenCalledTimes(1)
    expect(api.post).toHaveBeenCalledWith(
      '/api/v1/agent/run',
      { message },
      expect.objectContaining({
        headers: { 'X-Request-ID': 'rid-1' },
      }),
    )
  })
})
