import type { DiagnosisEvents } from './api'
import type {
  Capabilities,
  DiagnosticSession,
  DiagnosisCode,
  HistoryItem,
  LayerState,
} from './types'

const scenarios = ['healthy', 'dns', 'wifi', 'gateway', 'upstream', 'destination', 'unknown']
const codes: Record<string, DiagnosisCode> = {
  healthy: 'no_material_problem_observed',
  dns: 'likely_dns_degradation',
  wifi: 'likely_local_wifi_instability',
  gateway: 'likely_local_gateway_instability',
  upstream: 'likely_upstream_degradation',
  destination: 'likely_destination_specific_degradation',
  unknown: 'unknown',
}
const conclusions: Record<DiagnosisCode, string> = {
  no_material_problem_observed: 'No significant problem was observed during this test.',
  likely_dns_degradation: 'DNS showed repeated failures during this test.',
  likely_destination_specific_degradation:
    'Tested destinations showed repeated failures during this test.',
  device_resource_pressure: 'Wi-Fi/Ethernet showed repeated failures during this test.',
  likely_local_wifi_instability: 'Wi-Fi/Ethernet showed repeated failures during this test.',
  likely_local_gateway_instability: 'Gateway showed repeated failures during this test.',
  likely_upstream_degradation: 'Internet showed repeated failures during this test.',
  unknown: 'A brief interruption was observed, but this test could not isolate it to one layer.',
}
const baseLayers: LayerState[] = [
  ['local', 'Wi-Fi / Ethernet'],
  ['gateway', 'Gateway'],
  ['dns', 'DNS'],
  ['internet', 'Internet'],
  ['destinations', 'Tested destinations'],
].map(([id, label]) => ({ id, label, status: 'waiting' }))

let current: {
  id: string
  scenario: string
  cancelled: boolean
  session?: DiagnosticSession
} | null = null
let history: HistoryItem[] = []

function fixture(scenario: string): DiagnosticSession {
  const code = codes[scenario] ?? codes.unknown
  const now = new Date().toISOString()
  return {
    schema_version: '1.3.0',
    id: `interactive-demo-${scenario}`,
    started_at: now,
    ended_at: now,
    raw_results: [{ collector_type: 'fixture', target: 'deterministic_demo', success: true }],
    summaries: [
      { collector_type: 'fixture', target: 'deterministic_demo', attempts: 3, successes: 3 },
    ],
    evidence: [],
    diagnosis: {
      code,
      conclusion: conclusions[code],
      confidence: code === 'unknown' ? undefined : 'moderate',
      supporting_evidence: [
        {
          id: 'demo-evidence',
          statement:
            'RouteCause deterministic demonstration measurements support this example result.',
          supports: true,
          summary_keys: ['demo'],
        },
      ],
      contradicting_evidence: [],
      alternative_explanations: [
        'This is an interactive fixture, not a measurement of the current connection.',
      ],
    },
    limitations: [
      'INTERACTIVE DEMO: browsers cannot access native Wi-Fi, route, gateway ICMP, or device measurements. No live probes were run.',
    ],
  }
}

export const staticApi = {
  capabilities: async (): Promise<Capabilities> => ({
    platform: {
      native_windows: false,
      wifi_api_available: false,
      wifi_adapter_present: false,
      wifi_connected: false,
      default_ipv4_route_present: false,
      gateway_icmp_available: false,
    },
    demo_mode: true,
    quick_seconds: 60,
    full_seconds: 60,
    demo_scenarios: scenarios,
  }),
  history: async () => history,
  start: async (_duration: 'quick' | 'full', scenario = 'healthy') => {
    if (current && !current.session && !current.cancelled)
      throw new Error('diagnosis already running')
    current = { id: `static-${Date.now()}`, scenario, cancelled: false }
    history = [
      {
        id: current.id,
        started_at: new Date().toISOString(),
        duration: 'fixture',
        status: 'running',
        demo: true,
        demo_scenario: scenario,
        layers: baseLayers,
      },
      ...history,
    ].slice(0, 10)
    return { id: current.id, events_url: '', session_url: '' }
  },
  session: async (id: string) => {
    if (!current || current.id !== id || !current.session)
      throw new Error('Demo result is not complete')
    return current.session
  },
  cancel: async (id: string) => {
    if (current?.id === id) current.cancelled = true
  },
  events: (id: string, handlers: DiagnosisEvents) => {
    const timers: number[] = []
    const schedule = (fn: () => void, delay: number) => timers.push(window.setTimeout(fn, delay))
    schedule(() => handlers.started({ layers: baseLayers }), 20)
    baseLayers.forEach((layer, index) =>
      schedule(
        () => {
          if (current?.id === id && current.cancelled) {
            handlers.stopped({ layers: baseLayers, message: 'Diagnosis cancelled' })
            return
          }
          handlers.layer({ layer: layer.id, status: 'testing' })
        },
        100 + index * 160,
      ),
    )
    schedule(() => {
      if (!current || current.id !== id) return
      if (current.cancelled) {
        handlers.stopped({ layers: baseLayers, message: 'Diagnosis cancelled' })
        return
      }
      current.session = fixture(current.scenario)
      const completeLayers = baseLayers.map((layer) => ({
        ...layer,
        status:
          current?.scenario === 'unknown'
            ? ('inconclusive' as const)
            : (current?.scenario === 'wifi' && layer.id === 'local') ||
                (current?.scenario === 'gateway' && layer.id === 'gateway') ||
                (current?.scenario === 'dns' && layer.id === 'dns') ||
                (current?.scenario === 'upstream' && layer.id === 'internet') ||
                (current?.scenario === 'destination' && layer.id === 'destinations')
              ? ('degraded' as const)
              : ('healthy' as const),
      }))
      history = history.map((item) =>
        item.id === id
          ? {
              ...item,
              status: 'completed',
              conclusion: current?.session?.diagnosis.conclusion,
              diagnosis_code: current?.session?.diagnosis.code,
              layers: completeLayers,
            }
          : item,
      )
      handlers.completed({ layers: completeLayers })
    }, 1050)
    return () => timers.forEach(window.clearTimeout)
  },
}
