import { useEffect, useReducer } from 'react'
import { getPhoto, type Photo } from '../../api'

type State =
  | { status: 'idle'; photo: null; error: null }
  | { status: 'loading'; photo: null; error: null }
  | { status: 'success'; photo: Photo; error: null }
  | { status: 'error'; photo: null; error: string }

type Action =
  | { type: 'RESET' }
  | { type: 'LOADING' }
  | { type: 'SUCCESS'; photo: Photo }
  | { type: 'ERROR'; error: string }

const idle: State = { status: 'idle', photo: null, error: null }

function reducer(_state: State, action: Action): State {
  switch (action.type) {
    case 'RESET':   return idle
    case 'LOADING': return { status: 'loading', photo: null, error: null }
    case 'SUCCESS': return { status: 'success', photo: action.photo, error: null }
    case 'ERROR':   return { status: 'error', photo: null, error: action.error }
  }
}

export interface UsePhotoDetailResult {
  photo: Photo | null
  status: 'idle' | 'loading' | 'success' | 'error'
  error: string | null
}

export function usePhotoDetail(photoId: string | null): UsePhotoDetailResult {
  const [state, dispatch] = useReducer(reducer, idle)

  useEffect(() => {
    if (photoId === null) {
      dispatch({ type: 'RESET' })
      return
    }

    const controller = new AbortController()
    dispatch({ type: 'LOADING' })

    getPhoto(photoId, controller.signal)
      .then((photo) => {
        if (!controller.signal.aborted) dispatch({ type: 'SUCCESS', photo })
      })
      .catch((err: unknown) => {
        // An abort (photo change / unmount) is expected cleanup, not an error.
        if (controller.signal.aborted) return
        const error = err instanceof Error ? err.message : 'Failed to load photo'
        dispatch({ type: 'ERROR', error })
      })

    return () => controller.abort()
  }, [photoId])

  return state
}
