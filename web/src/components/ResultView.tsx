import type { DiagnosticSession } from '../types'

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
  const deviceResourcesNotMeasured = session.summaries.some((value) => {
    if (!value || typeof value !== 'object') return false
    const summary = value as { collector_type?: string; errors?: Record<string, number> }
    return summary.collector_type === 'device_resources' && Boolean(summary.errors?.unsupported)
  })
  const staticExport = `data:application/json;charset=utf-8,${encodeURIComponent(JSON.stringify(session, null, 2))}`
  return (
    <article className="result" aria-labelledby="result-title">
      {demo && <p className="demo-stamp">Demonstration result — no live measurements</p>}
      <p className="eyebrow">Test summary</p>
      <h2 id="result-title">{diagnosis.conclusion}</h2>

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
        <summary>Advanced report</summary>
        <p className="confidence">
          Confidence: <strong>{diagnosis.confidence || 'withheld'}</strong>
        </p>
        {deviceResourcesNotMeasured && <p>Device resources: Not measured</p>}
        <div className="result__columns">
          <EvidenceList title="Supporting evidence" items={diagnosis.supporting_evidence} />
          <EvidenceList title="Contradicting evidence" items={diagnosis.contradicting_evidence} />
        </div>
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
