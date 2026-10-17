import { describe, expect, it } from 'vitest'
import { channelSlices, parseBoardNumber } from './board'

const columns = ['date', 'channel', 'campaign', 'spend', 'revenue', 'roas']
const rows = [
  { date: '2026-10-14', channel: 'Meta', campaign: 'Brand Autumn Launch', spend: '42000.00', revenue: '79800.00', roas: '1.90' },
  { date: '2026-10-14', channel: 'TikTok', campaign: 'Acquisition Short Video', spend: '31,500.00', revenue: '47,200.00', roas: '1.50' },
  { date: '2026-10-14', channel: 'Meta', campaign: 'Performance Remarketing', spend: '26800', revenue: '35100', roas: '1.31' },
  { date: '2026-10-14', channel: 'Google', campaign: 'Brand Search', spend: '18400.00', revenue: '55200.00', roas: '3.00' },
]

describe('channelSlices', () => {
  it('parses grouped numbers', () => {
    expect(parseBoardNumber('31,500.00')).toBe(31500)
    expect(parseBoardNumber('1.33%')).toBe(1.33)
    expect(parseBoardNumber('')).toBe(0)
  })

  it('rolls spend and revenue up by channel, highest spend first', () => {
    expect(channelSlices(columns, rows)).toEqual([
      { name: 'Meta', spend: 68800, revenue: 114900 },
      { name: 'TikTok', spend: 31500, revenue: 47200 },
      { name: 'Google', spend: 18400, revenue: 55200 },
    ])
  })

  it('returns nothing when the table has no channel or spend column', () => {
    expect(channelSlices(['campaign', 'roas'], [{ campaign: 'Brand Search', roas: '3.00' }])).toEqual([])
  })
})
