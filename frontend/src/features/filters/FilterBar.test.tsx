import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { Provider } from 'react-redux'
import { configureStore } from '@reduxjs/toolkit'
import { filtersReducer } from './filtersSlice'
import { selectedPhotoReducer } from '../gallery/selectedPhotoSlice'
import { FilterBar } from './FilterBar'
import { setClassId, resetFilters } from './filtersSlice'
import type { ClassId } from './types'

vi.mock('../../api', () => ({
  listPhotos: vi.fn(() => new Promise(() => {})),
  ApiError: class ApiError extends Error {},
}))

function makeStore(preloadedFilters?: Partial<{ classId: ClassId | null; minConfidence: number }>) {
  return configureStore({
    reducer: { filters: filtersReducer, selectedPhoto: selectedPhotoReducer },
    preloadedState: {
      filters: { classId: null, minConfidence: 0, locationFilter: null, ...preloadedFilters },
    },
  })
}

function renderWithStore(store = makeStore()) {
  return { store, ...render(<Provider store={store}><FilterBar /></Provider>) }
}

describe('FilterBar', () => {
  beforeEach(() => vi.clearAllMocks())

  it('renders All button and all 6 class buttons', () => {
    renderWithStore()
    expect(screen.getByRole('button', { name: 'All' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /powdery mildew/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /mirid/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /whitefly/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /miner/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /thrips/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /spider mites/i })).toBeInTheDocument()
  })

  it('marks the active class button as pressed via aria-pressed', () => {
    const store = makeStore({ classId: 'mirid' })
    renderWithStore(store)
    expect(screen.getByRole('button', { name: /mirid/i })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: 'All' })).toHaveAttribute('aria-pressed', 'false')
  })

  it('dispatches setClassId when clicking a class button', () => {
    const store = makeStore()
    const dispatch = vi.spyOn(store, 'dispatch')
    renderWithStore(store)
    fireEvent.click(screen.getByRole('button', { name: /thrips/i }))
    expect(dispatch).toHaveBeenCalledWith(setClassId('thrips'))
  })

  it('dispatches setClassId(null) when clicking All', () => {
    const store = makeStore({ classId: 'mirid' })
    const dispatch = vi.spyOn(store, 'dispatch')
    renderWithStore(store)
    fireEvent.click(screen.getByRole('button', { name: 'All' }))
    expect(dispatch).toHaveBeenCalledWith(setClassId(null))
  })

  it('does not render Reset button when filters are default', () => {
    renderWithStore(makeStore())
    expect(screen.queryByRole('button', { name: /reset/i })).not.toBeInTheDocument()
  })

  it('renders Reset button when classId is set', () => {
    renderWithStore(makeStore({ classId: 'thrips' }))
    expect(screen.getByRole('button', { name: /reset/i })).toBeInTheDocument()
  })

  it('dispatches resetFilters when clicking Reset', () => {
    const store = makeStore({ classId: 'mirid' })
    const dispatch = vi.spyOn(store, 'dispatch')
    renderWithStore(store)
    fireEvent.click(screen.getByRole('button', { name: /reset/i }))
    expect(dispatch).toHaveBeenCalledWith(resetFilters())
  })
})
