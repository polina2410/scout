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

// Bound the eager full-dataset fetch so a misbehaving cursor (or an unexpectedly
// large dataset) can never trigger an unbounded request loop for the map view.
const PAGE_SIZE = 50
const MAX_PAGES = 20

export interface UseAllPhotosResult {
  photos: Photo[]
  status: 'loading' | 'success' | 'error'
}

async function fetchAllPhotos(signal: AbortSignal): Promise<Photo[]> {
  const all: Photo[] = []
  let cursor: string | undefined
  for (let page = 0; page < MAX_PAGES; page++) {
    const result = await listPhotos({ limit: PAGE_SIZE, cursor }, signal)
    all.push(...result.items)
    cursor = result.next_token ?? undefined
    if (!cursor) break
  }
  return all
}

export function useAllPhotos(): UseAllPhotosResult {
  const [state, dispatch] = useReducer(reducer, initial)

  useEffect(() => {
    const controller = new AbortController()
    fetchAllPhotos(controller.signal)
      .then((photos) => {
        if (!controller.signal.aborted) dispatch({ type: 'SUCCESS', photos })
      })
      .catch(() => {
        // An abort (unmount) is expected cleanup, not a user-facing error.
        if (!controller.signal.aborted) dispatch({ type: 'ERROR' })
      })
    return () => controller.abort()
  }, [])

  return state
}
