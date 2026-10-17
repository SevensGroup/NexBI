import { existsSync, readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import path from 'node:path'
import { test, expect } from '@playwright/test'

const uuidRe = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i
const goal = 'Analyze why Meta ad ROI declined yesterday'

type Artifact = { run_id?: string; status?: string; error?: string }

function loadArtifact(): Artifact | null {
  const fromEnv = process.env.MUSK_SELFTEST_RUN_ID
  if (fromEnv && uuidRe.test(fromEnv)) return { run_id: fromEnv }
  const p = path.join(path.dirname(fileURLToPath(import.meta.url)), '.last-live-run.json')
  if (!existsSync(p)) return null
  try {
    return JSON.parse(readFileSync(p, 'utf8')) as Artifact
  } catch {
    return null
  }
}

test.describe('live run pages', () => {
  test.describe.configure({ timeout: 6 * 60_000 })

  test('live run URL is uuid; timeline is inspectable; failures show backend error', async ({
    page,
    request,
  }) => {
    const health = await request.get('http://127.0.0.1:8080/health')
    expect(health.ok(), 'start musk-back with musk-back/.env (cd musk-back && make run)').toBeTruthy()

    await page.addInitScript(() => localStorage.setItem('musk-demo-session', '1'))
    const prior = loadArtifact()
    let runId = prior?.run_id && uuidRe.test(prior.run_id) ? prior.run_id : ''

    if (runId) {
      await page.goto(`/runs/${runId}`)
    } else {
      await page.goto('/dashboard')
      await expect(page.getByTestId('backend-connection')).toHaveText('Connected', { timeout: 20_000 })
      await page.getByTestId('dashboard-goal').fill(goal)
      await page.getByTestId('dashboard-run').click()
      await expect(page).toHaveURL(/\/runs\/[0-9a-f-]{36}/i, { timeout: 30_000 })
      runId = page.url().split('/runs/')[1]?.split(/[?#]/)[0] ?? ''
    }

    expect(runId, 'expected musk-back uuid, not run-ads-01').toMatch(uuidRe)
    expect(runId).not.toBe('run-ads-01')
    await expect(page).toHaveURL(new RegExp(`/runs/${runId}`))

    await expect(page.getByTestId('timeline-row').filter({ hasText: 'Goal' }).first()).toBeVisible({
      timeout: 60_000,
    })

    const failBanner = page.getByTestId('run-failure-banner')
    const failRow = page.getByTestId('timeline-row').filter({ hasText: 'Failed' })
    if ((await failBanner.count()) > 0) {
      const text = (await failBanner.innerText()).trim()
      expect(text.length, 'FAILED banner must include backend error string').toBeGreaterThan(8)
      expect(text).toMatch(/FAILED/)
      expect(text).not.toMatch(/sample catalog/i)
      await expect(failRow.first()).toBeVisible()
    }

    const goalRow = page.getByTestId('timeline-row').filter({ hasText: 'Goal' }).first()
    const decision = page.getByTestId('timeline-row').filter({ hasText: 'Autonomous Decision' }).first()
    const clickable = (await decision.count()) > 0 ? decision : goalRow
    await clickable.click()
    const detail = page.getByTestId('timeline-detail')
    await expect(detail).toBeVisible({ timeout: 10_000 })
    const body = await detail.innerText()
    expect(body.length).toBeGreaterThan(10)
    expect(body.endsWith('...')).toBeFalsy()
    if ((await decision.count()) > 0) {
      expect(body).toMatch(/DELEGATE_AGENT|CALL_TOOL|REPLAN|REFLECT|FINISH|Reasoning|Decision/i)
    }
  })
})
