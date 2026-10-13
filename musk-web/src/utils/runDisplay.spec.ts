import { describe, expect, it } from 'vitest'
import {
  isLiveSampleCatalog,
  isSampleCatalogMessage,
  runFailureBanner,
  SAMPLE_CATALOG_HINT,
} from './runDisplay'

describe('live sample catalog', () => {
  it('recognizes run-ads-01 only in live mode', () => {
    expect(isLiveSampleCatalog('run-ads-01', false)).toBe(true)
    expect(isLiveSampleCatalog('run-ads-01', true)).toBe(false)
    expect(isLiveSampleCatalog('0490b2cd-4950-4c99-a118-ce5d1f3d575a', false)).toBe(false)
  })

  it('does not map catalog id / hint as agent.failed banner copy', () => {
    expect(isSampleCatalogMessage(SAMPLE_CATALOG_HINT)).toBe(true)
    expect(runFailureBanner('UNDERSTANDING', SAMPLE_CATALOG_HINT)).toBeNull()
    expect(runFailureBanner('FAILED', SAMPLE_CATALOG_HINT)).toBeNull()
    expect(runFailureBanner('FAILED', 'invalid decision: agent is required for DELEGATE_AGENT')).toBe(
      'invalid decision: agent is required for DELEGATE_AGENT',
    )
  })
})
