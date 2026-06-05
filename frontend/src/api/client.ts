import type { Photo, PhotoPage, ListPhotosParams } from './types'

export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly requestId: string

  constructor(status: number, code: string, message: string, requestId: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.requestId = requestId
  }
}

const baseUrl: string = import.meta.env.VITE_API_URL
const apiKey: string = import.meta.env.VITE_API_KEY
if (!baseUrl || !apiKey) {
  throw new Error('VITE_API_URL and VITE_API_KEY must be set')
}

async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${baseUrl}${path}`, {
    ...init,
    headers: {
      ...(init?.headers as Record<string, string> | undefined),
      ...(init?.body !== undefined ? { 'Content-Type': 'application/json' } : {}),
      'X-API-Key': apiKey,
    },
  })

  if (!response.ok) {
    let code = 'UnknownError'
    let message = 'Unexpected response'
    let requestId = ''
    try {
      const body = (await response.json()) as {
        code?: string
        message?: string
        request_id?: string
      }
      code = body.code ?? code
      message = body.message ?? message
      requestId = body.request_id ?? requestId
    } catch {
      // use defaults
    }
    throw new ApiError(response.status, code, message, requestId)
  }

  return response.json() as Promise<T>
}

export async function listPhotos(
  params?: ListPhotosParams,
  signal?: AbortSignal,
): Promise<PhotoPage> {
  const qs = new URLSearchParams()
  if (params) {
    if (params.cursor !== undefined) qs.set('cursor', params.cursor)
    if (params.limit !== undefined) qs.set('limit', String(params.limit))
    if (params.classId !== undefined) qs.set('classId', params.classId)
    if (params.minConfidence !== undefined && params.minConfidence !== 0) {
      qs.set('minConfidence', String(params.minConfidence))
    }
  }
  const query = qs.toString()
  return apiFetch<PhotoPage>(`/photos${query ? `?${query}` : ''}`, { signal })
}

export async function getPhoto(photoId: string): Promise<Photo> {
  return apiFetch<Photo>(`/photos/${photoId}`)
}
