import { describe, expect, it } from 'vitest'
import {
  isLiveSampleCatalog,
  isSampleCatalogMessage,
  presentRunError,
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

  it('collapses a Gemini quota body to llm error', () => {
    const raw =
      'generate decision: failed to create chat completion: error, status code: 429, message: Quota exceeded for metric: generate_content_free_tier_requests, model: gemini-3.6-flash'
    expect(presentRunError(raw)).toBe('llm error')
    expect(runFailureBanner('FAILED', raw)).toBe('llm error')
  })
})
