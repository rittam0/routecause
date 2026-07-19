import { useEffect, useRef, useState } from 'react'
import { api } from './api'
import { LayerPath } from './components/LayerPath'
import { ResultView } from './components/ResultView'
import type { Capabilities, DiagnosticSession, HistoryItem, LayerState } from './types'

const initialLayers: LayerState[] = [
  { id: 'device', label: 'Device', status: 'waiting' },
  { id: 'local', label: 'Wi-Fi / Ethernet', status: 'waiting' },
  { id: 'gateway', label: 'Gateway', status: 'waiting' },
  { id: 'internet', label: 'DNS / Internet', status: 'waiting' },
  { id: 'destinations', label: 'Destinations', status: 'waiting' },
]

export default function App() {
  const [capabilities, setCapabilities] = useState<Capabilities | null>(null)
  const [history, setHistory] = useState<HistoryItem[]>([])
  const [duration, setDuration] = useState<'quick' | 'full'>('quick')
  const [scenario, setScenario] = useState('healthy')
  const [layers, setLayers] = useState(initialLayers)
  const [activeID, setActiveID] = useState<string | null>(null)
  const [resultID, setResultID] = useState<string | null>(null)
  const [startedAt, setStartedAt] = useState<number | null>(null)
  const [elapsed, setElapsed] = useState(0)
  const [session, setSession] = useState<DiagnosticSession | null>(null)
  const [error, setError] = useState('')
  const eventsRef = useRef<(() => void) | null>(null)

  useEffect(() => {
    Promise.all([api.capabilities(), api.history()])
      .then(([caps, items]) => {
        setCapabilities(caps)
        setHistory(items)
        const active = items.find(
          (item) => item.status === 'running' || item.status === 'cancelling',
        )
        if (active) {
          setActiveID(active.id)
          setStartedAt(new Date(active.started_at).getTime())
          setLayers(active.layers)
        }
      })
      .catch((cause: Error) => setError(cause.message))
  }, [])

  useEffect(() => {
    if (!startedAt || !activeID) return
    const update = () => setElapsed(Math.max(0, Math.floor((Date.now() - startedAt) / 1000)))
    update()
    const timer = window.setInterval(update, 1000)
    return () => window.clearInterval(timer)
  }, [startedAt, activeID])

  useEffect(() => {
    if (!activeID) return
    const close = api.events(activeID, {
      started: (data) => {
        setLayers(data.layers)
      },
      layer: (data) => {
        setLayers((current) =>
          current.map((layer) =>
            layer.id === data.layer
              ? { ...layer, status: data.status as LayerState['status'] }
              : layer,
          ),
        )
      },
      completed: async (data) => {
        setLayers(data.layers)
        eventsRef.current?.()
        try {
          const completed = await api.session(activeID)
          setSession(completed)
          setResultID(activeID)
          setHistory(await api.history())
        } catch (cause) {
          setError((cause as Error).message)
        }
        setActiveID(null)
      },
      stopped: (data) => {
        setLayers(data.layers)
        eventsRef.current?.()
        setActiveID(null)
        setError(data.message || 'Diagnosis cancelled')
      },
    })
    eventsRef.current = close
    return close
  }, [activeID])

  async function start() {
    setError('')
    setSession(null)
    setResultID(null)
    setLayers(initialLayers)
    try {
      const response = await api.start(duration, capabilities?.demo_mode ? scenario : undefined)
      setActiveID(response.id)
      setStartedAt(Date.now())
      setElapsed(0)
    } catch (cause) {
      setError((cause as Error).message)
    }
  }

  async function cancel() {
    if (!activeID) return
    try {
      await api.cancel(activeID)
    } catch (cause) {
      setError((cause as Error).message)
    }
  }

  const running = Boolean(activeID)
  return (
    <>
      {capabilities?.demo_mode && (
        <div className="demo-banner" role="status">
          Interactive Demo — deterministic fixtures, not your live connection
        </div>
      )}
      <main>
        <header className="hero">
          <p className="wordmark">RouteCause</p>
          <h1>Find out why your internet is slow—not merely how slow it is.</h1>
          <p className="hero__lede">
            A private, local diagnosis across your device, local network, internet path, and
            destinations.
          </p>
        </header>

        <section className="diagnose" aria-labelledby="diagnose-title">
          <h2 id="diagnose-title" className="sr-only">
            Start a diagnosis
          </h2>
          {!running && !session && (
            <div className="start-controls">
              <fieldset>
                <legend>Test length</legend>
                <label>
                  <input
                    type="radio"
                    name="duration"
                    checked={duration === 'quick'}
                    onChange={() => setDuration('quick')}
                  />{' '}
                  Quick ({capabilities?.quick_seconds ?? 15}s)
                </label>
                <label>
                  <input
                    type="radio"
                    name="duration"
                    checked={duration === 'full'}
                    onChange={() => setDuration('full')}
                  />{' '}
                  Full ({capabilities?.full_seconds ?? 60}s)
                </label>
              </fieldset>
              {capabilities?.demo_mode && (
                <label className="scenario">
                  Demonstration scenario
                  <select value={scenario} onChange={(event) => setScenario(event.target.value)}>
                    {capabilities.demo_scenarios.map((item) => (
                      <option value={item} key={item}>
                        {item.replaceAll('_', ' ')}
                      </option>
                    ))}
                  </select>
                </label>
              )}
              <button className="button button--primary button--large" onClick={start}>
                Diagnose my connection
              </button>
              <p className="privacy-note">
                Runs only on this device. No account, upload, analytics, or browser storage.
              </p>
            </div>
          )}

          <LayerPath layers={layers} />

          {running && (
            <div className="run-state" aria-live="polite">
              <p>
                <span className="testing-dot" aria-hidden="true" />
                Testing supported connection layers
              </p>
              <p>
                Elapsed time: <strong>{elapsed}s</strong>
              </p>
              <button className="button button--quiet" onClick={cancel}>
                Cancel diagnosis
              </button>
            </div>
          )}
          {error && (
            <p className="error" role="alert">
              {error}
            </p>
          )}
        </section>

        {session && resultID && activeID === null && (
          <>
            <ResultView session={session} sessionID={resultID} demo={capabilities?.demo_mode} />
            <button
              className="button button--quiet new-test"
              onClick={() => {
                setSession(null)
                setResultID(null)
                setLayers(initialLayers)
                setError('')
              }}
            >
              Run another diagnosis
            </button>
          </>
        )}

        {history.length > 0 && !running && (
          <aside className="history" aria-labelledby="history-title">
            <h2 id="history-title">Recent local sessions</h2>
            <ul>
              {history.slice(0, 5).map((item) => (
                <li key={item.id}>
                  <span>{new Date(item.started_at).toLocaleString()}</span>
                  <strong>{item.conclusion || item.status}</strong>
                  {item.demo && <em>Demo</em>}
                </li>
              ))}
            </ul>
          </aside>
        )}
      </main>
      <div className="sr-only" aria-live="assertive">
        {session ? `Diagnosis complete: ${session.diagnosis.conclusion}` : ''}
      </div>
    </>
  )
}
