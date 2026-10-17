import { isSampleRunId } from '@/utils/timeline'

export const SAMPLE_CATALOG_HINT =
  'Sample catalog id. Start a live run from Dashboard — NexBI will open /runs/<uuid>.'

export function isLiveSampleCatalog(runId: string, mockMode: boolean): boolean {
  return !mockMode && isSampleRunId(runId)
}

export function isSampleCatalogMessage(message: string | null | undefined): boolean {
  return Boolean(message && /sample catalog/i.test(message))
}

const llmFailure =
  /failed to create chat completion|status code:\s*429|status code:\s*401|quota exceeded|too many requests|rate limit|resource_exhausted|errorresponse|incorrect api key|invalid api key/i

/** Provider failures stay one short line. The raw response stays in the server log. */
export function presentRunError(error: string | null | undefined): string {
  const text = error?.trim() ?? ''
  if (!text) return ''
  if (llmFailure.test(text)) return 'llm error'
  return text
}

/** Red error banner: real FAILED/ABORTED only. Catalog hints never qualify. */
export function runFailureBanner(status: string, error: string | null | undefined): string | null {
  if (isSampleCatalogMessage(error)) return null
  if (status !== 'FAILED' && status !== 'ABORTED') return null
  return presentRunError(error) || 'agent run failed'
}
