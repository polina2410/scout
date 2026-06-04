import { useReducer, useEffect, useCallback } from 'react'
import { listPhotos, type Photo, type ListPhotosParams } from '../../api'

const PAGE_LIMIT = 20

type Status = 'loading' | 'loading-more' | 'success' | 'error'

interface State {
  photos: Photo[]
  status: Status
  error: string | null
  cursor: string | undefined
}

type Action =
  | { type: 'RESET' }
  | { type: 'FIRST_PAGE_SUCCESS'; photos: Photo[]; cursor: string | undefined }
  | { type: 'MORE_PAGE_SUCCESS'; photos: Photo[]; cursor: string | undefined }
  | { type: 'LOAD_MORE' }
  | { type: 'ERROR'; message: string }
  | { type: 'MORE_ERROR'; message: string }

function reducer(state: State, action: Action): State {
  switch (action.type) {
    case 'RESET':
      return { photos: [], status: 'loading', error: null, cursor: undefined }
    case 'FIRST_PAGE_SUCCESS':
      return { ...state, status: 'success', photos: action.photos, cursor: action.cursor, error: null }
    case 'LOAD_MORE':
      return { ...state, status: 'loading-more' }
    case 'MORE_PAGE_SUCCESS':
      return { ...state, status: 'success', photos: [...state.photos, ...action.photos], cursor: action.cursor, error: null }
    case 'ERROR':
      return { ...state, status: 'error', error: action.message }
    case 'MORE_ERROR':
      return { ...state, status: 'error', error: action.message }
    default:
      return state
  }
}

export interface UsePhotosResult {
  photos: Photo[]
  status: Status
  error: string | null
  hasMore: boolean
  loadMore: () => void
}

export function usePhotos(
  params?: Pick<ListPhotosParams, 'classId' | 'minConfidence'>,
): UsePhotosResult {
  const [state, dispatch] = useReducer(reducer, {
    photos: [],
    status: 'loading',
    error: null,
    cursor: undefined,
  })

  const classId = params?.classId
  const minConfidence = params?.minConfidence

  useEffect(() => {
    dispatch({ type: 'RESET' })
    let cancelled = false

    listPhotos({ classId: classId ?? undefined, minConfidence, limit: PAGE_LIMIT })
      .then((page) => {
        if (!cancelled) {
          dispatch({
            type: 'FIRST_PAGE_SUCCESS',
            photos: page.items,
            cursor: page.next_token ?? undefined,
          })
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          const message = err instanceof Error ? err.message : 'Failed to load photos'
          dispatch({ type: 'ERROR', message })
        }
      })

    return () => {
      cancelled = true
    }
  }, [classId, minConfidence])

  const loadMore = useCallback(() => {
    if (state.status === 'loading' || state.status === 'loading-more' || !state.cursor) {
      return
    }

    dispatch({ type: 'LOAD_MORE' })

    listPhotos({ classId: classId ?? undefined, minConfidence, cursor: state.cursor, limit: PAGE_LIMIT })
      .then((page) => {
        dispatch({
          type: 'MORE_PAGE_SUCCESS',
          photos: page.items,
          cursor: page.next_token ?? undefined,
        })
      })
      .catch((err: unknown) => {
        const message = err instanceof Error ? err.message : 'Failed to load more photos'
        dispatch({ type: 'MORE_ERROR', message })
      })
  }, [state.status, state.cursor, classId, minConfidence])

  return {
    photos: state.photos,
    status: state.status,
    error: state.error,
    hasMore: state.cursor !== undefined,
    loadMore,
  }
}
