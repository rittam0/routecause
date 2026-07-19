import { render, screen } from '@testing-library/react'
import axe from 'axe-core'
import { describe, expect, it } from 'vitest'
import { ResultView } from './ResultView'
import type { DiagnosticSession, DiagnosisCode } from '../types'

const states: DiagnosisCode[] = [
  'no_material_problem_observed',
  'likely_dns_degradation',
  'likely_destination_specific_degradation',
  'device_resource_pressure',
  'likely_local_wifi_instability',
  'likely_local_gateway_instability',
  'likely_upstream_degradation',
  'unknown',
]

function fixture(code: DiagnosisCode): DiagnosticSession {
  return {
    schema_version: '1.3.0',
    id: 'fixture',
    started_at: '2026-01-01T00:00:00Z',
    ended_at: '2026-01-01T00:01:00Z',
    raw_results: [{ collector_type: 'wifi', success: true }],
    summaries: [{ collector_type: 'wifi', success_rate: 1 }],
    evidence: [],
    diagnosis: {
      code,
      conclusion: `Plain result for ${code}`,
      confidence: code === 'unknown' ? undefined : 'moderate',
      supporting_evidence: [
        {
          id: 'support',
          statement: 'A typed supporting observation.',
          supports: true,
          summary_keys: [],
        },
      ],
      contradicting_evidence:
        code === 'unknown'
          ? [
              {
                id: 'conflict',
                statement: 'A typed contradictory observation.',
                supports: false,
                summary_keys: [],
              },
            ]
          : [],
      alternative_explanations: ['A bounded alternative explanation.'],
    },
    limitations: ['A quiet session cannot exclude intermittent behavior.'],
  }
}

describe('ResultView', () => {
  for (const code of states) {
    it(`renders the ${code} diagnosis state`, () => {
      render(<ResultView session={fixture(code)} sessionID="web-session" />)
      expect(screen.getByRole('heading', { name: `Plain result for ${code}` })).toBeInTheDocument()
      expect(screen.getByText('Test summary')).toBeInTheDocument()
      expect(screen.getByRole('link', { name: 'Export report' })).toHaveAttribute(
        'href',
        '/api/v1/diagnoses/web-session/export',
      )
      expect(screen.getByText('Advanced report')).toBeInTheDocument()
    })
  }

  it('visibly distinguishes a demo result and has no basic axe violations', async () => {
    const { container } = render(
      <ResultView session={fixture('likely_local_wifi_instability')} sessionID="demo" demo />,
    )
    expect(screen.getByText(/demonstration result/i)).toBeInTheDocument()
    const results = await axe.run(container)
    expect(results.violations).toEqual([])
  })

  it('keeps the primary result short and technical qualifications advanced', () => {
    const session = fixture('unknown')
    session.evidence = [
      {
        id: 'possible-gateway-latency-variation',
        statement: 'Possible local gateway latency variation was observed.',
        supports: true,
        summary_keys: ['gateway:local_gateway'],
      },
    ]
    session.diagnosis.supporting_evidence = session.evidence
    const { container } = render(<ResultView session={session} sessionID="variation" />)
    expect(screen.getByText(/Possible local gateway latency variation/)).not.toBeVisible()
    expect(container.querySelectorAll('article.result > section li')).toHaveLength(0)
    expect(screen.getByText('Advanced report')).toBeInTheDocument()
  })

  it('renders the required plain healthy result', () => {
    const session = fixture('no_material_problem_observed')
    session.diagnosis.conclusion = 'No significant problem was observed during this test.'
    session.diagnosis.supporting_evidence = [
      {
        id: 'healthy-connection-path',
        statement:
          'No significant problem was observed across Wi-Fi, gateway, DNS, internet and tested destinations.',
        supports: true,
        summary_keys: [],
      },
    ]
    render(<ResultView session={session} sessionID="healthy" />)
    expect(
      screen.getByRole('heading', {
        name: 'No significant problem was observed during this test.',
      }),
    ).toBeVisible()
    expect(screen.getByText('moderate')).not.toBeVisible()
  })

  it('shows unsupported Windows device resources neutrally in the advanced report', () => {
    const session = fixture('no_material_problem_observed')
    session.summaries = [
      {
        collector_type: 'device_resources',
        attempts: 1,
        errors: { unsupported: 1 },
      },
    ]
    render(<ResultView session={session} sessionID="windows" />)
    expect(screen.getByText('Device resources: Not measured')).not.toBeVisible()
  })
})
