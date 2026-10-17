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

export interface ChannelSlice {
  name: string
  spend: number
  revenue: number
}

export function parseBoardNumber(value: string): number {
  const text = value.trim().replace(/[$,]/g, '').replace(/%$/, '')
  if (!text) return 0
  const numeric = Number(text)
  return Number.isFinite(numeric) ? numeric : 0
}

function columnNamed(columns: string[], name: string): string {
  return columns.find((column) => column.trim().toLowerCase() === name) ?? ''
}

/** Spend and revenue rolled up by channel, for the default pie and bar charts. */
export function channelSlices(columns: string[], rows: Record<string, string>[]): ChannelSlice[] {
  const names = columns.length ? columns : Object.keys(rows[0] ?? {})
  const channelCol = columnNamed(names, 'channel')
  const spendCol = columnNamed(names, 'spend')
  const revenueCol = columnNamed(names, 'revenue')
  if (!channelCol || !spendCol) return []
  const totals = new Map<string, ChannelSlice>()
  for (const row of rows) {
    const name = String(row[channelCol] ?? '').trim()
    if (!name) continue
    const current = totals.get(name) ?? { name, spend: 0, revenue: 0 }
    current.spend += parseBoardNumber(String(row[spendCol] ?? ''))
    if (revenueCol) current.revenue += parseBoardNumber(String(row[revenueCol] ?? ''))
    totals.set(name, current)
  }
  return [...totals.values()]
    .filter((slice) => slice.spend > 0 || slice.revenue > 0)
    .sort((a, b) => b.spend - a.spend)
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
