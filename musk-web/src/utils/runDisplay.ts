import { isSampleRunId } from '@/utils/timeline'

export const SAMPLE_CATALOG_HINT =
  'Sample catalog id. Start a live run from Dashboard — musk-back will open /runs/<uuid>.'

export function isLiveSampleCatalog(runId: string, mockMode: boolean): boolean {
  return !mockMode && isSampleRunId(runId)
}

export function isSampleCatalogMessage(message: string | null | undefined): boolean {
  return Boolean(message && /sample catalog/i.test(message))
}

/** Red error banner: real FAILED/ABORTED only. Catalog hints never qualify. */
export function runFailureBanner(status: string, error: string | null | undefined): string | null {
  if (isSampleCatalogMessage(error)) return null
  if (status !== 'FAILED' && status !== 'ABORTED') return null
  const text = error?.trim()
  return text || 'agent run failed'
}
