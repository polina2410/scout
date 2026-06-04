import { afterAll, describe, it, expect, vi } from 'vitest'

const BASE_URL = 'http://localhost:8080'

vi.stubEnv('VITE_API_URL', BASE_URL)
afterAll(() => vi.unstubAllEnvs())

const { thumbnailUrl, thumbnailSrcSet } = await import('./thumbnailUrl')

describe('thumbnailUrl', () => {
  it('builds correct URL with all params', () => {
    const url = thumbnailUrl('abc', 240, 2)
    expect(url).toContain('/thumbnails/abc')
    expect(url).toContain('w=240')
    expect(url).toContain('dpr=2')
    expect(url).toContain('fmt=jpeg')
  })

  it('includes the base VITE_API_URL', () => {
    const url = thumbnailUrl('abc', 240, 1)
    expect(url.startsWith(BASE_URL)).toBe(true)
  })
})

describe('thumbnailSrcSet', () => {
  it('includes 1x, 2x, 3x entries with correct URLs', () => {
    const srcSet = thumbnailSrcSet('abc', 240)
    expect(srcSet).toContain('1x')
    expect(srcSet).toContain('2x')
    expect(srcSet).toContain('3x')
    expect(srcSet).toContain('/thumbnails/abc')
    const parts = srcSet.split(', ')
    expect(parts).toHaveLength(3)
  })
})
