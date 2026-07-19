import { useState } from 'react'

const sourceURL = 'https://github.com/rittam0/routecause'
const downloadURL = '/download/windows'

const runs = [
  {
    id: 'offline',
    label: 'Wi-Fi switched off',
    result: 'Connection unavailable',
    detail: 'DNS, gateway, TCP and HTTPS checks completed with no successful network operations.',
    tone: 'issue',
  },
  {
    id: 'vpn',
    label: 'Proton VPN active',
    result: 'Different path behaviour observed',
    detail:
      'Wi-Fi remained connected while gateway ICMP failed and public probes behaved differently. This observation does not assign fault to the VPN or router.',
    tone: 'mixed',
  },
  {
    id: 'healthy',
    label: 'Normal Wi-Fi',
    result: 'No significant problem observed',
    detail:
      'Gateway completed 30/30 checks, HTTPS 12/12, TCP 60/60 and DNS 89/91 during the controlled native-Windows run.',
    tone: 'healthy',
  },
] as const

export default function PortfolioApp() {
  const [selected, setSelected] = useState<(typeof runs)[number]>(runs[2])

  return (
    <main className="portfolio-page">
      <header className="portfolio-hero">
        <p className="wordmark">RouteCause</p>
        <p className="portfolio-eyebrow">Native Windows network snapshot</p>
        <h1>See what happened across every layer of your connection.</h1>
        <p className="portfolio-lede">
          RouteCause measures Wi-Fi, the local gateway, DNS and internet destinations for 60
          seconds, then produces a private report on your own computer.
        </p>
        <div className="portfolio-actions">
          <a className="button button--primary button--large" href={downloadURL}>
            Download for Windows
          </a>
          <a className="button button--quiet" href={sourceURL}>
            View source
          </a>
        </div>
        <p className="portfolio-boundary">
          This website is a product overview. Browsers cannot access native Windows Wi-Fi or gateway
          measurements; the downloadable application performs the real snapshot.
        </p>
      </header>

      <section className="portfolio-section" aria-labelledby="validated-title">
        <div className="portfolio-section__heading">
          <p className="portfolio-kicker">Tested on native Windows</p>
          <h2 id="validated-title">Three controlled conditions, three supported observations</h2>
        </div>
        <div className="validation-picker" role="group" aria-label="Validated native runs">
          {runs.map((run) => (
            <button
              className={
                selected.id === run.id ? 'validation-tab validation-tab--active' : 'validation-tab'
              }
              key={run.id}
              onClick={() => setSelected(run)}
              type="button"
            >
              {run.label}
            </button>
          ))}
        </div>
        <article
          className={`validation-result validation-result--${selected.tone}`}
          aria-live="polite"
        >
          <p>{selected.label}</p>
          <h3>{selected.result}</h3>
          <p>{selected.detail}</p>
        </article>
        <p className="portfolio-caption">
          These are aggregate results from real controlled runs, not browser simulations or
          population-wide accuracy claims.
        </p>
      </section>

      <section
        className="portfolio-section engineering-grid"
        id="engineering"
        aria-labelledby="engineering-title"
      >
        <div className="portfolio-section__heading engineering-grid__heading">
          <p className="portfolio-kicker">Engineering evidence</p>
          <h2 id="engineering-title">A small interface over a serious systems core</h2>
        </div>
        <article className="engineering-card">
          <strong>Native collection</strong>
          <p>
            Go collectors use Windows WLAN, IP Helper and ICMP APIs without administrator access.
          </p>
        </article>
        <article className="engineering-card">
          <strong>Private by design</strong>
          <p>
            Runs on 127.0.0.1 with no account, cloud upload, analytics or stored network
            identifiers.
          </p>
        </article>
        <article className="engineering-card">
          <strong>Concurrent and observable</strong>
          <p>Bounded collectors stream typed progress through replayable Server-Sent Events.</p>
        </article>
        <article className="engineering-card">
          <strong>Reproducible load test</strong>
          <p>
            10,000 simulated SSE clients completed with 0% observed errors in the recorded WSL run.
          </p>
        </article>
      </section>

      <section className="portfolio-section run-native" id="run-native" aria-labelledby="run-title">
        <div>
          <p className="portfolio-kicker">Native Windows release</p>
          <h2 id="run-title">One download. No account or administrator access.</h2>
          <p>
            RouteCause runs locally and opens its private interface in your browser. Network
            evidence stays on the computer unless you choose to export it.
          </p>
        </div>
        <div className="portfolio-actions portfolio-actions--download">
          <a className="button button--primary button--large" href={downloadURL}>
            Download .exe
          </a>
          <a className="button button--quiet" href={`${sourceURL}/releases/latest`}>
            Release notes
          </a>
        </div>
      </section>

      <footer className="portfolio-footer">
        <strong>RouteCause</strong>
        <span>Local-first. Evidence-bounded. Open source.</span>
      </footer>
    </main>
  )
}
