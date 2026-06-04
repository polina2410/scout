import { useEffect, useReducer } from 'react'
import { listPhotos, type Photo } from '../../api'

type State =
  | { status: 'loading'; photos: [] }
  | { status: 'success'; photos: Photo[] }
  | { status: 'error'; photos: [] }

type Action =
  | { type: 'SUCCESS'; photos: Photo[] }
  | { type: 'ERROR' }

function reducer(_state: State, action: Action): State {
  switch (action.type) {
    case 'SUCCESS': return { status: 'success', photos: action.photos }
    case 'ERROR':   return { status: 'error', photos: [] }
  }
}

const initial: State = { status: 'loading', photos: [] }

export interface UseAllPhotosResult {
  photos: Photo[]
  status: 'loading' | 'success' | 'error'
}

export function useAllPhotos(): UseAllPhotosResult {
  const [state, dispatch] = useReducer(reducer, initial)

  useEffect(() => {
    let cancelled = false
    listPhotos({ limit: 50 })
      .then((page) => {
        if (!cancelled) dispatch({ type: 'SUCCESS', photos: page.items })
      })
      .catch(() => {
        if (!cancelled) dispatch({ type: 'ERROR' })
      })
    return () => { cancelled = true }
  }, [])

  return state
}
