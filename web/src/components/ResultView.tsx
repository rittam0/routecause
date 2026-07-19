import type { DiagnosticSession, DiagnosisCode } from '../types'

const recommendations: Record<DiagnosisCode, string[]> = {
  no_material_problem_observed: [
    'Repeat the test when the problem is happening.',
    'Check whether the issue is limited to one app or device.',
  ],
  likely_dns_degradation: [
    'Restart the affected application and retry the test.',
    'Compare your configured DNS service with another trusted resolver.',
  ],
  likely_destination_specific_degradation: [
    'Check the affected service status and retry later.',
    'Compare the same destination from another connection if available.',
  ],
  device_resource_pressure: [
    'Close resource-intensive applications and repeat the diagnosis.',
    'Check operating-system resource usage for sustained pressure.',
  ],
  likely_local_wifi_instability: [
    'Move closer to the access point and repeat the diagnosis.',
    'Reduce nearby radio interference or test with Ethernet if available.',
  ],
  likely_local_gateway_instability: [
    'Check whether other local devices show the same behavior.',
    'Restart the router only if doing so will not disrupt other users.',
  ],
  likely_upstream_degradation: [
    'Repeat the test to confirm the pattern across independent targets.',
    'Share the exported evidence with your network provider if it persists.',
  ],
  unknown: [
    'Run a full test while the problem is actively occurring.',
    'Review unavailable or contradictory layers in the evidence below.',
  ],
}

function EvidenceList({
  title,
  items,
}: {
  title: string
  items: { id: string; statement: string }[]
}) {
  return (
    <section>
      <h3>{title}</h3>
      {items.length ? (
        <ul className="plain-list">
          {items.map((item, index) => (
            <li key={`${item.id}-${index}`}>{item.statement}</li>
          ))}
        </ul>
      ) : (
        <p className="muted">None recorded.</p>
      )}
    </section>
  )
}

export function ResultView({
  session,
  sessionID,
  demo = false,
}: {
  session: DiagnosticSession
  sessionID: string
  demo?: boolean
}) {
  const diagnosis = session.diagnosis
  const staticExport = `data:application/json;charset=utf-8,${encodeURIComponent(JSON.stringify(session, null, 2))}`
  return (
    <article className="result" aria-labelledby="result-title">
      {demo && <p className="demo-stamp">Demonstration result — no live measurements</p>}
      <p className="eyebrow">Likely cause</p>
      <h2 id="result-title">{diagnosis.conclusion}</h2>
      <p className="confidence">
        Confidence:{' '}
        <strong>
          {diagnosis.confidence ? diagnosis.confidence : 'withheld — evidence is inconclusive'}
        </strong>
      </p>

      <div className="result__columns">
        <EvidenceList title="Supporting evidence" items={diagnosis.supporting_evidence} />
        <EvidenceList title="Contradicting evidence" items={diagnosis.contradicting_evidence} />
      </div>

      <section>
        <h3>What to do next</h3>
        <ol className="action-list">
          {recommendations[diagnosis.code].map((item) => (
            <li key={item}>{item}</li>
          ))}
        </ol>
      </section>

      <div className="result__columns result__secondary">
        <section>
          <h3>Other explanations</h3>
          <ul className="plain-list">
            {diagnosis.alternative_explanations.map((item) => (
              <li key={item}>{item}</li>
            ))}
          </ul>
        </section>
        <section>
          <h3>Limitations</h3>
          <ul className="plain-list">
            {session.limitations.map((item) => (
              <li key={item}>{item}</li>
            ))}
          </ul>
        </section>
      </div>

      <div className="result__actions">
        <a
          className="button button--primary"
          href={
            import.meta.env.VITE_STATIC_DEMO === 'true'
              ? staticExport
              : `/api/v1/diagnoses/${sessionID}/export`
          }
          download={`routecause-${sessionID}.json`}
        >
          Export report
        </a>
      </div>

      <details className="advanced">
        <summary>Advanced raw measurements</summary>
        <pre>
          {JSON.stringify(
            { summaries: session.summaries, raw_results: session.raw_results },
            null,
            2,
          )}
        </pre>
      </details>
    </article>
  )
}
