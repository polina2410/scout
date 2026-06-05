import { describe, it, expect } from 'vitest'
import { selectedPhotoReducer as reducer, selectPhoto, clearSelectedPhoto } from './selectedPhotoSlice'

describe('selectedPhotoSlice', () => {
  it('has correct initial state', () => {
    const state = reducer(undefined, { type: '@@INIT' })
    expect(state.photoId).toBeNull()
  })

  it('selectPhoto sets photoId', () => {
    const state = reducer(undefined, selectPhoto('abc-123'))
    expect(state.photoId).toBe('abc-123')
  })

  it('clearSelectedPhoto resets to null', () => {
    const withPhoto = reducer(undefined, selectPhoto('abc-123'))
    const cleared = reducer(withPhoto, clearSelectedPhoto())
    expect(cleared.photoId).toBeNull()
  })

  it('selectPhoto overwrites an existing selection', () => {
    const first = reducer(undefined, selectPhoto('photo-1'))
    const second = reducer(first, selectPhoto('photo-2'))
    expect(second.photoId).toBe('photo-2')
  })
})
