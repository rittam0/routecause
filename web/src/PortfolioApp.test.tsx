import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import PortfolioApp from './PortfolioApp'

describe('portfolio product page', () => {
  it('states the browser boundary and presents controlled native evidence', () => {
    render(<PortfolioApp />)

    expect(screen.getByText(/browsers cannot access native Windows Wi-Fi/i)).toBeInTheDocument()
    expect(
      screen.getByRole('heading', { name: /three controlled conditions/i }),
    ).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /snapshot/i })).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Wi-Fi switched off' }))
    expect(screen.getByRole('heading', { name: 'Connection unavailable' })).toBeInTheDocument()
  })
})
