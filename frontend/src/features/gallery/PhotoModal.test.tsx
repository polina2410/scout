import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { Provider } from 'react-redux'
import { configureStore } from '@reduxjs/toolkit'
import { filtersReducer } from '../filters/filtersSlice'
import { selectedPhotoReducer } from './selectedPhotoSlice'
import { PhotoModal } from './PhotoModal'
import { selectPhoto, clearSelectedPhoto } from './selectedPhotoSlice'
import type { Photo } from '../../api'

vi.mock('../../api', () => ({
  listPhotos: vi.fn(() => new Promise(() => {})),
  getPhoto: vi.fn(),
  ApiError: class ApiError extends Error {
    status = 0; code = ''; requestId = ''
  },
}))

import { getPhoto } from '../../api'
const mockGetPhoto = vi.mocked(getPhoto)

function makeStore(photoId: string | null = null) {
  const store = configureStore({
    reducer: { filters: filtersReducer, selectedPhoto: selectedPhotoReducer },
  })
  if (photoId !== null) store.dispatch(selectPhoto(photoId))
  return store
}

function renderWithStore(store = makeStore()) {
  return { store, ...render(<Provider store={store}><PhotoModal /></Provider>) }
}

function makePhoto(id: string): Photo {
  return {
    id,
    x: 1, y: 2, h: 3,
    width: 2560, height: 1440,
    capturedAt: '2024-01-01T00:00:00Z',
    originalUrl: `http://minio/photos/${id}.jpg`,
    predictions: [
      { classId: 'mirid', confidence: 0.92, bbox: { xMin: 0.1, yMin: 0.1, xMax: 0.3, yMax: 0.3 } },
      { classId: 'thrips', confidence: 0.75, bbox: { xMin: 0.5, yMin: 0.5, xMax: 0.7, yMax: 0.7 } },
    ],
  }
}

describe('PhotoModal', () => {
  beforeEach(() => vi.clearAllMocks())

  it('renders nothing when no photo is selected', () => {
    const { container } = renderWithStore(makeStore(null))
    expect(container).toBeEmptyDOMElement()
  })

  it('shows loading state while fetching', () => {
    mockGetPhoto.mockReturnValue(new Promise(() => {}))
    renderWithStore(makeStore('abc'))
    expect(screen.getByText(/loading/i)).toBeInTheDocument()
  })

  it('renders image and predictions on success', async () => {
    mockGetPhoto.mockResolvedValue(makePhoto('abc'))
    renderWithStore(makeStore('abc'))
    await waitFor(() => {
      expect(screen.getByRole('img')).toBeInTheDocument()
      expect(screen.getByText(/mirid/i)).toBeInTheDocument()
      expect(screen.getByText(/thrips/i)).toBeInTheDocument()
      expect(screen.getByText('92%')).toBeInTheDocument()
    })
  })

  it('shows error message on fetch failure', async () => {
    mockGetPhoto.mockRejectedValue(new Error('not found'))
    renderWithStore(makeStore('abc'))
    await waitFor(() => {
      expect(screen.getByText(/not found/i)).toBeInTheDocument()
    })
  })

  it('close button dispatches clearSelectedPhoto', async () => {
    mockGetPhoto.mockResolvedValue(makePhoto('abc'))
    const store = makeStore('abc')
    const dispatch = vi.spyOn(store, 'dispatch')
    renderWithStore(store)
    await waitFor(() => screen.getByRole('img'))
    fireEvent.click(screen.getByRole('button', { name: /close/i }))
    expect(dispatch).toHaveBeenCalledWith(clearSelectedPhoto())
  })

  it('backdrop click dispatches clearSelectedPhoto', async () => {
    mockGetPhoto.mockResolvedValue(makePhoto('abc'))
    const store = makeStore('abc')
    const dispatch = vi.spyOn(store, 'dispatch')
    const { container } = renderWithStore(store)
    await waitFor(() => screen.getByRole('img'))
    fireEvent.click(container.firstChild as HTMLElement)
    expect(dispatch).toHaveBeenCalledWith(clearSelectedPhoto())
  })
})
