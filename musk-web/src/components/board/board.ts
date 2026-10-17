export interface BoardMetric {
  label: string
  value: string
}

export interface BoardView {
  code: string
  name: string
  summary: string
  metrics: BoardMetric[]
  columns: string[]
  rows: Record<string, string>[]
  url: string
}

export function boardCodeFromUrl(url: string): string {
  const match = url.match(/\/bi\/([a-z0-9]+)/i)
  return match?.[1] ?? ''
}

export function formatBoardValue(value: string): string {
  const text = value.trim()
  if (!text || text.endsWith('%')) return text
  const numeric = Number(text)
  if (!Number.isFinite(numeric)) return text
  if (/e/i.test(text)) {
    return new Intl.NumberFormat('en-US', { maximumFractionDigits: 0 }).format(numeric)
  }
  if (text.includes('.')) {
    const digits = text.split('.')[1]?.length ?? 0
    return new Intl.NumberFormat('en-US', { minimumFractionDigits: digits, maximumFractionDigits: digits }).format(numeric)
  }
  if (Math.abs(numeric) >= 1000) return new Intl.NumberFormat('en-US').format(numeric)
  return text
}
