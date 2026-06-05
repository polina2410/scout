import { describe, it, expect } from 'vitest'
import { filtersReducer as reducer, setClassId, setMinConfidence, resetFilters } from './filtersSlice'

describe('filtersSlice', () => {
  it('has correct initial state', () => {
    const state = reducer(undefined, { type: '@@INIT' })
    expect(state.classId).toBeNull()
    expect(state.minConfidence).toBe(0)
  })

  it('setClassId sets a class', () => {
    const state = reducer(undefined, setClassId('mirid'))
    expect(state.classId).toBe('mirid')
  })

  it('setClassId(null) clears the filter', () => {
    const withClass = reducer(undefined, setClassId('thrips'))
    const cleared = reducer(withClass, setClassId(null))
    expect(cleared.classId).toBeNull()
  })

  it('setMinConfidence sets value', () => {
    const state = reducer(undefined, setMinConfidence(0.7))
    expect(state.minConfidence).toBe(0.7)
  })

  it('setMinConfidence clamps below 0', () => {
    const state = reducer(undefined, setMinConfidence(-0.1))
    expect(state.minConfidence).toBe(0)
  })

  it('setMinConfidence clamps above 1', () => {
    const state = reducer(undefined, setMinConfidence(1.5))
    expect(state.minConfidence).toBe(1)
  })

  it('resetFilters restores initial state', () => {
    const modified = reducer(
      reducer(undefined, setClassId('spider_mites')),
      setMinConfidence(0.9),
    )
    const reset = reducer(modified, resetFilters())
    expect(reset.classId).toBeNull()
    expect(reset.minConfidence).toBe(0)
  })
})
