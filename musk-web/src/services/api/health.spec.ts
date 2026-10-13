import { describe, expect, it, vi, beforeEach } from 'vitest'

vi.mock('./client', () => ({
  api: { get: vi.fn() },
  isMockMode: () => false,
}))

import { api } from './client'
import { connectionStatusFromHealth, pingHealth } from './health'

describe('connectionStatusFromHealth', () => {
  it('is Connected when musk-back health is ok even if SSE /stream is missing', () => {
    expect(connectionStatusFromHealth({ backendOk: true, sseAvailable: false })).toBe('Connected')
  })

  it('is Disconnected only when the backend health ping fails', () => {
    expect(connectionStatusFromHealth({ backendOk: false, sseAvailable: true })).toBe('Disconnected')
  })
})

describe('pingHealth', () => {
  beforeEach(() => {
    vi.mocked(api.get).mockReset()
  })

  it('treats GET /health { status: ok } as up', async () => {
    vi.mocked(api.get).mockResolvedValue({ data: { status: 'ok' }, status: 200 })
    await expect(pingHealth()).resolves.toBe(true)
    expect(api.get).toHaveBeenCalledWith('/health', expect.objectContaining({ timeout: 4000 }))
  })

  it('does not use SSE /stream for liveness', async () => {
    vi.mocked(api.get).mockResolvedValue({ data: { status: 'ok' }, status: 200 })
    await pingHealth()
    expect(api.get).not.toHaveBeenCalledWith(expect.stringMatching(/stream/), expect.anything())
  })
})
