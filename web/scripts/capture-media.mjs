import { chromium } from '@playwright/test'
import { mkdir, copyFile } from 'node:fs/promises'

const output = new URL('../../docs/assets/', import.meta.url)
await mkdir(output, { recursive: true })
const browser = await chromium.launch()
const context = await browser.newContext({
  viewport: { width: 1440, height: 1000 },
  recordVideo: { dir: output, size: { width: 1280, height: 720 } },
})
const page = await context.newPage()
await page.goto('http://127.0.0.1:8765/')
await page.screenshot({ path: new URL('routecause-start.png', output).pathname, fullPage: true })
await page.getByLabel('Demonstration scenario').selectOption('gateway')
await page.getByRole('button', { name: 'Diagnose my connection' }).click()
await page.getByText('Demonstration result — no live measurements').waitFor()
await page.screenshot({ path: new URL('routecause-result.png', output).pathname, fullPage: true })
const video = page.video()
await context.close()
if (video) await copyFile(await video.path(), new URL('routecause-demo.webm', output))
await browser.close()
