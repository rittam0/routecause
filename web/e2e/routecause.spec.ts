import { expect, test } from '@playwright/test'

async function start(page: import('@playwright/test').Page, scenario = 'healthy') {
  await page.getByLabel('Demonstration scenario').selectOption(scenario)
  await page.getByRole('button', { name: /Full .*second snapshot/ }).click()
}

test('loads the labelled local demo application', async ({ page }) => {
  await page.goto('/')
  await expect(page).toHaveTitle(/RouteCause/)
  await expect(page.getByText('Interactive Demo — deterministic fixtures')).toBeVisible()
  await expect(page.getByRole('heading', { level: 1 })).toContainText('each layer')
})

test('starts, receives SSE progress, reconnects after refresh, and completes', async ({ page }) => {
  await page.goto('/')
  await start(page, 'healthy')
  await expect(page.getByText('Measuring supported connection layers')).toBeVisible()
  await expect(page.getByText('Testing').first()).toBeVisible()
  await page.reload()
  await expect(page.getByText('Measuring supported connection layers')).toBeVisible()
  await expect(page.getByRole('heading', { name: /No significant problem/ })).toBeVisible()
  await expect(page.getByText('Demonstration result — no live measurements')).toBeVisible()
})

test('cancels an active diagnosis', async ({ page }) => {
  await page.goto('/')
  await start(page, 'healthy')
  await page.getByRole('button', { name: 'Cancel diagnosis' }).click()
  await expect(page.getByRole('alert')).toContainText('Diagnosis cancelled')
})

test('exports a completed report with the RouteCause filename', async ({ page }) => {
  await page.goto('/')
  await start(page, 'dns')
  await expect(page.getByRole('heading', { name: /DNS showed repeated failures/ })).toBeVisible()
  const downloadPromise = page.waitForEvent('download')
  await page.getByRole('link', { name: 'Export report' }).click()
  const download = await downloadPromise
  expect(download.suggestedFilename()).toMatch(/^routecause-.*\.json$/)
})

for (const scenario of [
  'healthy',
  'dns',
  'wifi',
  'gateway',
  'upstream',
  'destination',
  'unknown',
]) {
  test(`runs the labelled ${scenario} demo scenario`, async ({ page }) => {
    await page.goto('/')
    await start(page, scenario)
    await expect(page.getByText('Demonstration result — no live measurements')).toBeVisible()
    await expect(page.locator('.result')).toBeVisible()
    await expect(page.locator('.history em').first()).toHaveText('Demo')
  })
}
