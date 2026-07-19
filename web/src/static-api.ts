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
  no_material_problem_observed: 'No material problem was observed during this demo session.',
  likely_dns_degradation:
    'The system DNS path is likely degraded relative to comparison resolvers.',
  likely_destination_specific_degradation:
    'Degradation appears specific to one measured destination.',
  device_resource_pressure: 'Device resource pressure may be affecting connection performance.',
  likely_local_wifi_instability: 'Physical Wi-Fi instability is a likely local contributor.',
  likely_local_gateway_instability: 'The local gateway or local network path is likely unstable.',
  likely_upstream_degradation: 'Degradation is likely beyond the stable local network path.',
  unknown: 'There is insufficient or contradictory evidence to identify one cause.',
}
const baseLayers: LayerState[] = [
  ['device', 'Device'],
  ['local', 'Wi-Fi / Ethernet'],
  ['gateway', 'Gateway'],
  ['internet', 'DNS / Internet'],
  ['destinations', 'Destinations'],
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
    quick_seconds: 3,
    full_seconds: 5,
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
        status: current?.scenario === 'unknown' ? ('inconclusive' as const) : ('healthy' as const),
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
