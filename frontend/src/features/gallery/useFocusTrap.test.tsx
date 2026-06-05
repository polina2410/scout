import { describe, it, expect } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { useFocusTrap } from './useFocusTrap'

function Trapped({ active }: { active: boolean }) {
  const ref = useFocusTrap<HTMLDivElement>(active)
  return (
    <div ref={ref}>
      <button>first</button>
      <button>last</button>
    </div>
  )
}

describe('useFocusTrap', () => {
  it('wraps focus from last to first on Tab', () => {
    render(<Trapped active />)
    const buttons = screen.getAllByRole('button')
    const first = buttons[0]!
    const last = buttons[buttons.length - 1]!
    last.focus()
    fireEvent.keyDown(last, { key: 'Tab' })
    expect(document.activeElement).toBe(first)
  })

  it('wraps focus from first to last on Shift+Tab', () => {
    render(<Trapped active />)
    const buttons = screen.getAllByRole('button')
    const first = buttons[0]!
    const last = buttons[buttons.length - 1]!
    first.focus()
    fireEvent.keyDown(first, { key: 'Tab', shiftKey: true })
    expect(document.activeElement).toBe(last)
  })

  it('does nothing when inactive', () => {
    render(<Trapped active={false} />)
    const buttons = screen.getAllByRole('button')
    const last = buttons[buttons.length - 1]!
    last.focus()
    fireEvent.keyDown(last, { key: 'Tab' })
    expect(document.activeElement).toBe(last)
  })
})
