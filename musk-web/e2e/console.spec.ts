import { test, expect } from '@playwright/test'

test.describe('console live mode (no extra LLM)', () => {
  test('Dashboard shows Connected and no sample-catalog error', async ({ page, request }) => {
    const health = await request.get('http://127.0.0.1:8080/health')
    expect(health.ok(), 'start musk-back with musk-back/.env (cd musk-back && make run)').toBeTruthy()

    await page.goto('/dashboard')
    await expect(page.getByTestId('backend-connection')).toHaveText('Connected', { timeout: 20_000 })
    await expect(page.getByTestId('backend-connection')).not.toHaveText('Disconnected')
    await expect(page.getByTestId('sample-catalog-hint')).toHaveCount(0)
    await expect(page.getByTestId('run-failure-banner')).toHaveCount(0)
    await expect(page.getByText('Sample catalog run.')).toHaveCount(0)
    await expect(page.getByRole('heading', { name: 'Dashboard' })).toBeVisible()
  })

  test('/runs/run-ads-01 is not FAILED', async ({ page, request }) => {
    const health = await request.get('http://127.0.0.1:8080/health')
    expect(health.ok(), 'start musk-back with musk-back/.env').toBeTruthy()

    await page.goto('/runs/run-ads-01')
    await expect(page.getByTestId('backend-connection')).toHaveText('Connected', { timeout: 20_000 })
    await expect(page.getByTestId('sample-catalog-hint')).toBeVisible()
    await expect(page.getByTestId('run-failure-banner')).toHaveCount(0)
    await expect(page.getByText('Sample catalog run.')).toHaveCount(0)
    await expect(page.getByText('Idle')).toBeVisible()
  })
})
