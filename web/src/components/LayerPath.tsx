import type { LayerState } from '../types'

const statusLabels: Record<LayerState['status'], string> = {
  waiting: 'Waiting',
  testing: 'Testing',
  healthy: 'Healthy',
  degraded: 'Degraded',
  unavailable: 'Unavailable',
  inconclusive: 'Inconclusive',
}

export function LayerPath({ layers }: { layers: LayerState[] }) {
  return (
    <ol className="path" aria-label="Connection path">
      {layers.map((layer) => (
        <li className={`path__layer path__layer--${layer.status}`} key={layer.id}>
          <span className="path__marker" aria-hidden="true" />
          <span className="path__copy">
            <strong>{layer.label}</strong>
            <span>{statusLabels[layer.status]}</span>
          </span>
        </li>
      ))}
    </ol>
  )
}
