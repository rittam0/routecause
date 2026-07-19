import type { Capabilities, DiagnosticSession, HistoryItem } from './types'
import { staticApi } from './static-api'

export interface DiagnosisEvents {
  started(data: { layers: HistoryItem['layers'] }): void
  layer(data: { layer: string; status: string }): void
  completed(data: { layers: HistoryItem['layers'] }): void
  stopped(data: { layers: HistoryItem['layers']; message?: string }): void
}

async function json<T>(input: RequestInfo, init?: RequestInit): Promise<T> {
  const response = await fetch(input, init)
  if (!response.ok) {
    const body = await response.json().catch(() => ({ error: 'Request failed' }))
    throw new Error(body.error ?? `Request failed (${response.status})`)
  }
  return response.json() as Promise<T>
}

const nativeApi = {
  capabilities: () => json<Capabilities>('/api/v1/capabilities'),
  history: () => json<HistoryItem[]>('/api/v1/history'),
  start: (duration: 'quick' | 'full', demoScenario?: string) =>
    json<{ id: string; events_url: string; session_url: string }>('/api/v1/diagnoses', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ duration, demo_scenario: demoScenario || undefined }),
    }),
  session: (id: string) => json<DiagnosticSession>(`/api/v1/diagnoses/${id}`),
  cancel: async (id: string) => {
    const response = await fetch(`/api/v1/diagnoses/${id}`, { method: 'DELETE' })
    if (!response.ok) throw new Error('Could not cancel the diagnosis')
  },
  events: (id: string, handlers: DiagnosisEvents) => {
    const source = new EventSource(`/api/v1/diagnoses/${id}/events`)
    source.addEventListener('started', (event) => handlers.started(JSON.parse(event.data)))
    source.addEventListener('layer', (event) => handlers.layer(JSON.parse(event.data)))
    source.addEventListener('completed', (event) => handlers.completed(JSON.parse(event.data)))
    source.addEventListener('cancelled', (event) => handlers.stopped(JSON.parse(event.data)))
    source.addEventListener('failed', (event) => handlers.stopped(JSON.parse(event.data)))
    return () => source.close()
  },
}

export const api = import.meta.env.VITE_STATIC_DEMO === 'true' ? staticApi : nativeApi
