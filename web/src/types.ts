export type LayerStatus =
  'waiting' | 'testing' | 'healthy' | 'degraded' | 'unavailable' | 'inconclusive'

export interface LayerState {
  id: string
  label: string
  status: LayerStatus
}

export interface Evidence {
  id: string
  statement: string
  supports: boolean
  summary_keys: string[]
}

export type DiagnosisCode =
  | 'no_material_problem_observed'
  | 'likely_dns_degradation'
  | 'likely_destination_specific_degradation'
  | 'device_resource_pressure'
  | 'likely_local_wifi_instability'
  | 'likely_local_gateway_instability'
  | 'likely_upstream_degradation'
  | 'unknown'

export interface DiagnosticSession {
  schema_version: string
  id: string
  started_at: string
  ended_at: string
  raw_results: unknown[]
  summaries: unknown[]
  evidence: Evidence[]
  diagnosis: {
    code: DiagnosisCode
    conclusion: string
    confidence?: string
    supporting_evidence: Evidence[]
    contradicting_evidence: Evidence[]
    alternative_explanations: string[]
  }
  limitations: string[]
}

export interface Capabilities {
  platform: {
    native_windows: boolean
    wifi_api_available: boolean
    wifi_adapter_present: boolean
    wifi_connected: boolean
    default_ipv4_route_present: boolean
    default_route_kind?: string
    gateway_icmp_available: boolean
  }
  demo_mode: boolean
  quick_seconds: number
  full_seconds: number
  demo_scenarios: string[]
}

export interface HistoryItem {
  id: string
  started_at: string
  ended_at?: string
  duration: string
  status: string
  diagnosis_code?: DiagnosisCode
  conclusion?: string
  confidence?: string
  demo: boolean
  demo_scenario?: string
  layers: LayerState[]
}
