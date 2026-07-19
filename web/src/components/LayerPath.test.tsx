import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { LayerPath } from './LayerPath'
import type { LayerState } from '../types'

describe('LayerPath', () => {
  it('renders every supported status in text, not colour alone', () => {
    const statuses: LayerState['status'][] = [
      'waiting',
      'testing',
      'healthy',
      'degraded',
      'unavailable',
      'not_measured',
      'inconclusive',
    ]
    render(<LayerPath layers={statuses.map((status) => ({ id: status, label: status, status }))} />)
    for (const label of [
      'Waiting',
      'Testing',
      'Healthy',
      'Issue observed',
      'Insufficient data',
      'Not measured',
    ])
      expect(screen.getAllByText(label, { exact: true }).length).toBeGreaterThan(0)
    expect(screen.getByRole('list', { name: 'Connection path' })).toBeInTheDocument()
  })
})
