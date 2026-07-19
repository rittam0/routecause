import { afterEach, describe, expect, it, vi } from 'vitest'
import { staticApi } from './static-api'

afterEach(() => vi.useRealTimers())

describe('static portfolio fixtures', () => {
  it.each([
    ['healthy', 'no_material_problem_observed'],
    ['dns', 'likely_dns_degradation'],
    ['wifi', 'likely_local_wifi_instability'],
    ['gateway', 'likely_local_gateway_instability'],
    ['upstream', 'likely_upstream_degradation'],
    ['destination', 'likely_destination_specific_degradation'],
    ['unknown', 'unknown'],
  ])('runs the labelled %s fixture without native measurements', async (scenario, code) => {
    vi.useFakeTimers()
    const started = await staticApi.start('quick', scenario)
    const completed = new Promise<void>((resolve) => {
      staticApi.events(started.id, {
        started: () => undefined,
        layer: () => undefined,
        completed: () => resolve(),
        stopped: () => undefined,
      })
    })
    await vi.runAllTimersAsync()
    await completed
    const session = await staticApi.session(started.id)
    expect(session.diagnosis.code).toBe(code)
    expect(session.limitations.join(' ')).toMatch(/browsers cannot access native Wi-Fi/)
  })
})
