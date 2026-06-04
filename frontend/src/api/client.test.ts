import { afterAll, afterEach, describe, it, expect, vi } from 'vitest'
import type { Photo, PhotoPage } from './types'

const BASE_URL = 'http://localhost:8080'
const API_KEY = 'test-key'

vi.stubEnv('VITE_API_URL', BASE_URL)
vi.stubEnv('VITE_API_KEY', API_KEY)
afterAll(() => vi.unstubAllEnvs())

const { listPhotos, getPhoto, ApiError } = await import('./client')

function mockFetch(response: {
  ok: boolean
  status: number
  json: () => unknown
}) {
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue({
      ok: response.ok,
      status: response.status,
      json: () => Promise.resolve(response.json()),
    }),
  )
}

const photoPage: PhotoPage = {
  items: [],
  next_token: undefined,
}

const photo: Photo = {
  id: 'abc-123',
  x: 1,
  y: 2,
  h: 3,
  width: 2560,
  height: 1440,
  capturedAt: '2024-01-01T00:00:00Z',
  originalUrl: 'http://example.com/photo.jpg',
  predictions: [],
}

describe('listPhotos', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('returns PhotoPage on 200', async () => {
    mockFetch({ ok: true, status: 200, json: () => photoPage })
    const result = await listPhotos()
    expect(result).toEqual(photoPage)
  })

  it('serialises params into query string', async () => {
    mockFetch({ ok: true, status: 200, json: () => photoPage })
    await listPhotos({ classId: 'mirid', minConfidence: 0.8, cursor: 'tok' })
    const url = vi.mocked(globalThis.fetch).mock.calls[0]![0] as string
    expect(url).toContain('classId=mirid')
    expect(url).toContain('minConfidence=0.8')
    expect(url).toContain('cursor=tok')
  })

  it('omits minConfidence when 0', async () => {
    mockFetch({ ok: true, status: 200, json: () => photoPage })
    await listPhotos({ minConfidence: 0 })
    const url = vi.mocked(globalThis.fetch).mock.calls[0]![0] as string
    expect(url).not.toContain('minConfidence')
  })

  it('throws ApiError on 401', async () => {
    mockFetch({
      ok: false,
      status: 401,
      json: () => ({
        code: 'AuthenticationRequired',
        message: 'Missing API key',
        request_id: 'req-1',
      }),
    })
    await expect(listPhotos()).rejects.toSatisfy(
      (e: unknown) => e instanceof ApiError && e.status === 401 && e.code === 'AuthenticationRequired',
    )
  })

  it('throws ApiError on 500', async () => {
    mockFetch({
      ok: false,
      status: 500,
      json: () => ({
        code: 'InternalServerError',
        message: 'Something went wrong',
        request_id: 'req-2',
      }),
    })
    await expect(listPhotos()).rejects.toSatisfy(
      (e: unknown) => e instanceof ApiError && e.status === 500,
    )
  })
})

describe('getPhoto', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('returns Photo on 200', async () => {
    mockFetch({ ok: true, status: 200, json: () => photo })
    const result = await getPhoto('abc-123')
    expect(result).toEqual(photo)
  })

  it('throws ApiError on 404', async () => {
    mockFetch({
      ok: false,
      status: 404,
      json: () => ({
        code: 'NotFound',
        message: 'Photo not found',
        request_id: 'req-3',
        resource_id: 'abc-123',
      }),
    })
    await expect(getPhoto('abc-123')).rejects.toSatisfy(
      (e: unknown) => e instanceof ApiError && e.status === 404,
    )
  })
})

describe('apiFetch (via listPhotos)', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('attaches X-API-Key header on every request', async () => {
    mockFetch({ ok: true, status: 200, json: () => photoPage })
    await listPhotos()
    const init = vi.mocked(globalThis.fetch).mock.calls[0]![1] as RequestInit
    const headers = init?.headers as Record<string, string>
    expect(headers['X-API-Key']).toBe(API_KEY)
  })
})
