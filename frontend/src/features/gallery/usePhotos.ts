import { useReducer, useEffect, useCallback, useRef } from 'react'
import { listPhotos, type Photo, type ListPhotosParams } from '../../api'

const PAGE_LIMIT = 20

type Status = 'loading' | 'loading-more' | 'success' | 'error'

interface State {
  photos: Photo[]
  status: Status
  error: string | null
  loadMoreError: string | null
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
      return { photos: [], status: 'loading', error: null, loadMoreError: null, cursor: undefined }
    case 'FIRST_PAGE_SUCCESS':
      return { ...state, status: 'success', photos: action.photos, cursor: action.cursor, error: null, loadMoreError: null }
    case 'LOAD_MORE':
      return { ...state, status: 'loading-more', loadMoreError: null }
    case 'MORE_PAGE_SUCCESS':
      return { ...state, status: 'success', photos: [...state.photos, ...action.photos], cursor: action.cursor, error: null, loadMoreError: null }
    case 'ERROR':
      return { ...state, status: 'error', error: action.message }
    case 'MORE_ERROR':
      return { ...state, status: 'success', loadMoreError: action.message }
    default:
      return state
  }
}

export interface UsePhotosResult {
  photos: Photo[]
  status: Status
  error: string | null
  loadMoreError: string | null
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
    loadMoreError: null,
    cursor: undefined,
  })

  const classId = params?.classId
  const minConfidence = params?.minConfidence

  // Tracks the in-flight load-more request so it can be aborted when filters
  // change or the component unmounts, preventing a stale page from being appended.
  const loadMoreControllerRef = useRef<AbortController | null>(null)

  useEffect(() => {
    dispatch({ type: 'RESET' })
    const controller = new AbortController()

    listPhotos({ classId: classId ?? undefined, minConfidence, limit: PAGE_LIMIT }, controller.signal)
      .then((page) => {
        if (!controller.signal.aborted) {
          dispatch({
            type: 'FIRST_PAGE_SUCCESS',
            photos: page.items,
            cursor: page.next_token ?? undefined,
          })
        }
      })
      .catch((err: unknown) => {
        if (controller.signal.aborted) return
        const message = err instanceof Error ? err.message : 'Failed to load photos'
        dispatch({ type: 'ERROR', message })
      })

    return () => {
      controller.abort()
      // A load-more from the previous filter must not append onto a reset list.
      loadMoreControllerRef.current?.abort()
    }
  }, [classId, minConfidence])

  const loadMore = useCallback(() => {
    if (state.status === 'loading' || state.status === 'loading-more' || !state.cursor) {
      return
    }

    dispatch({ type: 'LOAD_MORE' })

    loadMoreControllerRef.current?.abort()
    const controller = new AbortController()
    loadMoreControllerRef.current = controller

    listPhotos(
      { classId: classId ?? undefined, minConfidence, cursor: state.cursor, limit: PAGE_LIMIT },
      controller.signal,
    )
      .then((page) => {
        if (controller.signal.aborted) return
        dispatch({
          type: 'MORE_PAGE_SUCCESS',
          photos: page.items,
          cursor: page.next_token ?? undefined,
        })
      })
      .catch((err: unknown) => {
        if (controller.signal.aborted) return
        const message = err instanceof Error ? err.message : 'Failed to load more photos'
        dispatch({ type: 'MORE_ERROR', message })
      })
  }, [state.status, state.cursor, classId, minConfidence])

  return {
    photos: state.photos,
    status: state.status,
    error: state.error,
    loadMoreError: state.loadMoreError,
    hasMore: state.cursor !== undefined,
    loadMore,
  }
}
