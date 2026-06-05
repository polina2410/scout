import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { Provider } from 'react-redux'
import { configureStore } from '@reduxjs/toolkit'
import { filtersReducer } from '../filters/filtersSlice'
import { selectedPhotoReducer } from './selectedPhotoSlice'
import { GalleryGrid } from './GalleryGrid'
import type { PhotoPage } from '../../api'

vi.mock('../../api', () => ({
  listPhotos: vi.fn(),
  ApiError: class ApiError extends Error {
    status: number
    code: string
    requestId: string
    constructor(message: string, status: number, code: string, requestId: string) {
      super(message)
      this.status = status
      this.code = code
      this.requestId = requestId
    }
  },
}))

import { listPhotos } from '../../api'
const mockListPhotos = vi.mocked(listPhotos)

function makeStore() {
  return configureStore({
    reducer: {
      filters: filtersReducer,
      selectedPhoto: selectedPhotoReducer,
    },
  })
}

function renderWithStore(ui: React.ReactElement) {
  return render(<Provider store={makeStore()}>{ui}</Provider>)
}

const emptyPage: PhotoPage = { items: [], next_token: undefined }

function makePhoto(id: string) {
  return {
    id,
    x: 1,
    y: 2,
    h: 3,
    width: 2560,
    height: 1440,
    capturedAt: '2024-01-01T00:00:00Z',
    originalUrl: `http://example.com/${id}.jpg`,
    predictions: [],
  }
}

describe('GalleryGrid', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  it('shows loading state initially', () => {
    mockListPhotos.mockReturnValue(new Promise(() => {}))
    renderWithStore(<GalleryGrid />)
    expect(screen.getByText(/loading/i)).toBeInTheDocument()
  })

  it('renders photo cards on success', async () => {
    const page: PhotoPage = {
      items: [makePhoto('id-1'), makePhoto('id-2'), makePhoto('id-3')],
      next_token: undefined,
    }
    mockListPhotos.mockResolvedValue(page)
    renderWithStore(<GalleryGrid />)
    await waitFor(() => {
      expect(screen.getAllByRole('button')).toHaveLength(3)
    })
  })

  it('shows empty state when no photos', async () => {
    mockListPhotos.mockResolvedValue(emptyPage)
    renderWithStore(<GalleryGrid />)
    await waitFor(() => {
      expect(screen.getByText(/no photos/i)).toBeInTheDocument()
    })
  })

  it('shows error state on failure', async () => {
    mockListPhotos.mockRejectedValue(new Error('network error'))
    renderWithStore(<GalleryGrid />)
    await waitFor(() => {
      expect(screen.getByText(/network error|failed to load/i)).toBeInTheDocument()
    })
  })
})
