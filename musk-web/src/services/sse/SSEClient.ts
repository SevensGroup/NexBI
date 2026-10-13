export type ConnectionState = 'Connected' | 'Reconnecting' | 'Disconnected'

export interface SSEClientOptions {
  url: string
  lastEventId?: string
  heartbeatMs?: number
  timeoutMs?: number
  reconnect?: boolean
  onEvent: (event: SSEParsedEvent) => void
  onStatus?: (status: ConnectionState) => void
  onError?: (err: Error) => void
}

export interface SSEParsedEvent {
  id?: string
  event?: string
  data: string
  retry?: number
}

const DEFAULT_HEARTBEAT = 25_000
const DEFAULT_TIMEOUT = 45_000

export class SSEClient {
  private abort: AbortController | null = null
  private timer: ReturnType<typeof setTimeout> | null = null
  private heartbeat: ReturnType<typeof setInterval> | null = null
  private lastEventId: string | undefined
  private closed = false
  private attempt = 0
  private lastMessageAt = 0
  private readonly opts: SSEClientOptions

  constructor(opts: SSEClientOptions) {
    this.opts = opts
    this.lastEventId = opts.lastEventId
  }

  connect(): void {
    this.closed = false
    void this.loop()
  }

  disconnect(): void {
    this.closed = true
    this.clearTimers()
    this.abort?.abort()
    this.abort = null
    this.opts.onStatus?.('Disconnected')
  }

  getLastEventId(): string | undefined {
    return this.lastEventId
  }

  private clearTimers(): void {
    if (this.timer) clearTimeout(this.timer)
    if (this.heartbeat) clearInterval(this.heartbeat)
    this.timer = null
    this.heartbeat = null
  }

  private async loop(): Promise<void> {
    while (!this.closed) {
      try {
        this.opts.onStatus?.(this.attempt === 0 ? 'Connected' : 'Reconnecting')
        await this.consume()
        this.attempt = 0
        if (this.closed) return
        this.opts.onStatus?.('Reconnecting')
      } catch (err) {
        if (this.closed) return
        this.opts.onError?.(err instanceof Error ? err : new Error(String(err)))
        this.opts.onStatus?.('Reconnecting')
      }
      if (!this.opts.reconnect && this.attempt > 0) {
        this.opts.onStatus?.('Disconnected')
        return
      }
      this.attempt += 1
      const wait = Math.min(8_000, 400 * 2 ** Math.min(this.attempt, 5))
      await delay(wait)
    }
  }

  private async consume(): Promise<void> {
    this.abort = new AbortController()
    const headers: Record<string, string> = {
      Accept: 'text/event-stream',
    }
    if (this.lastEventId) headers['Last-Event-ID'] = this.lastEventId

    const res = await fetch(this.opts.url, {
      method: 'GET',
      headers,
      signal: this.abort.signal,
    })
    if (!res.ok || !res.body) {
      throw new Error(`SSE HTTP ${res.status}`)
    }

    this.opts.onStatus?.('Connected')
    this.lastMessageAt = Date.now()
    this.armHeartbeat()

    const reader = res.body.getReader()
    const decoder = new TextDecoder()
    let buffer = ''

    while (!this.closed) {
      const { value, done } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })
      const chunks = buffer.split('\n\n')
      buffer = chunks.pop() ?? ''
      for (const chunk of chunks) {
        const parsed = parseSSEChunk(chunk)
        if (!parsed) continue
        if (parsed.id) this.lastEventId = parsed.id
        this.lastMessageAt = Date.now()
        if (parsed.event === 'ping' || parsed.data === ':heartbeat') continue
        this.opts.onEvent(parsed)
      }
    }
  }

  private armHeartbeat(): void {
    if (this.heartbeat) clearInterval(this.heartbeat)
    const hb = this.opts.heartbeatMs ?? DEFAULT_HEARTBEAT
    const timeout = this.opts.timeoutMs ?? DEFAULT_TIMEOUT
    this.heartbeat = setInterval(() => {
      if (Date.now() - this.lastMessageAt > timeout) {
        this.abort?.abort()
      }
    }, hb)
  }
}

export function parseSSEChunk(chunk: string): SSEParsedEvent | null {
  const lines = chunk.split('\n')
  const parsed: SSEParsedEvent = { data: '' }
  const dataLines: string[] = []
  for (const raw of lines) {
    const line = raw.replace(/\r$/, '')
    if (!line || line.startsWith(':')) continue
    const idx = line.indexOf(':')
    const field = idx === -1 ? line : line.slice(0, idx)
    let value = idx === -1 ? '' : line.slice(idx + 1)
    if (value.startsWith(' ')) value = value.slice(1)
    if (field === 'id') parsed.id = value
    else if (field === 'event') parsed.event = value
    else if (field === 'retry') parsed.retry = Number(value)
    else if (field === 'data') dataLines.push(value)
  }
  parsed.data = dataLines.join('\n')
  if (!parsed.data && !parsed.event && !parsed.id) return null
  return parsed
}

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}
