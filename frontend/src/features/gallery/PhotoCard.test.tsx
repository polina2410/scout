import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { Provider } from 'react-redux'
import { configureStore } from '@reduxjs/toolkit'
import { filtersReducer } from '../filters/filtersSlice'
import { selectedPhotoReducer } from './selectedPhotoSlice'
import { PhotoCard } from './PhotoCard'
import { selectPhoto } from './selectedPhotoSlice'
import type { Photo } from '../../api'

function makeStore() {
  return configureStore({
    reducer: { filters: filtersReducer, selectedPhoto: selectedPhotoReducer },
  })
}

function makePhoto(predictions: Photo['predictions'] = []): Photo {
  return {
    id: 'photo-1',
    x: 1, y: 2, h: 3,
    width: 2560, height: 1440,
    capturedAt: '2024-01-01T00:00:00Z',
    originalUrl: 'http://minio/photo-1.jpg',
    predictions,
  }
}

function renderCard(photo: Photo) {
  const store = makeStore()
  const dispatch = vi.spyOn(store, 'dispatch')
  render(<Provider store={store}><PhotoCard photo={photo} /></Provider>)
  return { dispatch }
}

describe('PhotoCard', () => {
  beforeEach(() => vi.clearAllMocks())

  it('renders as a native button (keyboard-accessible by default)', () => {
    renderCard(makePhoto())
    expect(screen.getByRole('button')).toBeInTheDocument()
  })

  it('exposes detections in the accessible name', () => {
    renderCard(makePhoto([
      { classId: 'mirid', confidence: 0.92, bbox: { xMin: 0, yMin: 0, xMax: 1, yMax: 1 } },
    ]))
    expect(
      screen.getByRole('button', { name: /greenhouse photo, mirid 92%\. view details\./i }),
    ).toBeInTheDocument()
  })

  it('says "no detections" when there are none', () => {
    renderCard(makePhoto())
    expect(screen.getByRole('button', { name: /no detections/i })).toBeInTheDocument()
  })

  it('dispatches selectPhoto on click', () => {
    const { dispatch } = renderCard(makePhoto())
    fireEvent.click(screen.getByRole('button'))
    expect(dispatch).toHaveBeenCalledWith(selectPhoto('photo-1'))
  })
})
