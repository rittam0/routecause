import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './e2e',
  timeout: 15_000,
  expect: { timeout: 5_000 },
  fullyParallel: false,
  workers: 1,
  reporter: 'line',
  use: {
    baseURL: 'http://127.0.0.1:8765',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  webServer: {
    command: '../bin/routecause serve --demo --no-open',
    url: 'http://127.0.0.1:8765/api/v1/capabilities',
    reuseExistingServer: false,
    timeout: 15_000,
  },
})
