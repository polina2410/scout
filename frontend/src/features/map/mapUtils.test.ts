import { describe, it, expect } from 'vitest'
import { metersToCanvas, canvasToMeters, distanceM, photosNearby, FLOOR_SIZE_M, CANVAS_SIZE_PX } from './mapUtils'
import type { Photo } from '../../api'

const SCALE = CANVAS_SIZE_PX / FLOOR_SIZE_M // 10 px/m

function makePhoto(id: string, x: number, y: number): Photo {
  return { id, x, y, h: 2, width: 2560, height: 1440, capturedAt: '', originalUrl: '', predictions: [] }
}

describe('metersToCanvas', () => {
  it('origin (0,40) maps to canvas top-left (0,0)', () => {
    expect(metersToCanvas(0, 40, SCALE)).toEqual({ cx: 0, cy: 0 })
  })

  it('far corner (40,0) maps to canvas bottom-right (400,400)', () => {
    expect(metersToCanvas(40, 0, SCALE)).toEqual({ cx: 400, cy: 400 })
  })

  it('centre (20,20) maps to canvas centre (200,200)', () => {
    expect(metersToCanvas(20, 20, SCALE)).toEqual({ cx: 200, cy: 200 })
  })
})

describe('canvasToMeters', () => {
  it('is the inverse of metersToCanvas', () => {
    const { cx, cy } = metersToCanvas(15, 30, SCALE)
    const { x, y } = canvasToMeters(cx, cy, SCALE)
    expect(x).toBeCloseTo(15)
    expect(y).toBeCloseTo(30)
  })
})

describe('distanceM', () => {
  it('same point is 0', () => {
    expect(distanceM(5, 5, 5, 5)).toBe(0)
  })

  it('3-4-5 right triangle gives distance 5', () => {
    expect(distanceM(0, 0, 3, 4)).toBe(5)
  })
})

describe('photosNearby', () => {
  it('returns only photos within radius', () => {
    const photos = [makePhoto('a', 0, 0), makePhoto('b', 3, 4), makePhoto('c', 8, 6)]
    // distances from (0,0): a=0, b=5, c=10
    const result = photosNearby(photos, 0, 0, 6)
    expect(result).toHaveLength(2)
    expect(result.map((p) => p.id)).toContain('a')
    expect(result.map((p) => p.id)).toContain('b')
  })

  it('returns empty array when no photos are nearby', () => {
    const photos = [makePhoto('a', 20, 20), makePhoto('b', 30, 30)]
    expect(photosNearby(photos, 0, 0, 5)).toHaveLength(0)
  })
})
