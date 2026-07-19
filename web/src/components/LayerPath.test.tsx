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
      'inconclusive',
    ]
    render(<LayerPath layers={statuses.map((status) => ({ id: status, label: status, status }))} />)
    for (const status of statuses)
      expect(screen.getAllByText(status, { exact: false }).length).toBeGreaterThan(0)
    expect(screen.getByRole('list', { name: 'Connection path' })).toBeInTheDocument()
  })
})
