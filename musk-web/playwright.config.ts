import { defineConfig, devices } from '@playwright/test'

const baseURL = process.env.MUSK_WEB_URL || 'http://localhost:5173'

export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  forbidOnly: Boolean(process.env.CI),
  retries: 0,
  workers: 1,
  testMatch: /.*\.(spec|e2e)\.ts/,
  expect: { timeout: 15_000 },
  use: {
    baseURL,
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  webServer: {
    command: 'npm run dev',
    url: baseURL,
    reuseExistingServer: true,
    timeout: 120_000,
    env: {
      ...process.env,
      VITE_USE_MOCK: 'false',
      VITE_API_BASE_URL: process.env.VITE_API_BASE_URL || 'http://127.0.0.1:8080',
    },
  },
})
