import { describe, it, expect } from 'vitest'
import { bboxToPixels } from './bboxUtils'

describe('bboxToPixels', () => {
  it('converts normalized bbox to CSS pixels', () => {
    const result = bboxToPixels({ xMin: 0.1, yMin: 0.2, xMax: 0.5, yMax: 0.8 }, 400, 300)
    expect(result.x).toBeCloseTo(40)
    expect(result.y).toBeCloseTo(60)
    expect(result.w).toBeCloseTo(160)
    expect(result.h).toBeCloseTo(180)
  })

  it('full-extent bbox fills the rendered area', () => {
    const result = bboxToPixels({ xMin: 0, yMin: 0, xMax: 1, yMax: 1 }, 240, 135)
    expect(result).toEqual({ x: 0, y: 0, w: 240, h: 135 })
  })

  it('zero-size bbox at a point', () => {
    const result = bboxToPixels({ xMin: 0.5, yMin: 0.5, xMax: 0.5, yMax: 0.5 }, 400, 300)
    expect(result).toEqual({ x: 200, y: 150, w: 0, h: 0 })
  })

  it('non-square aspect ratio', () => {
    const result = bboxToPixels({ xMin: 0, yMin: 0, xMax: 1, yMax: 0.5 }, 800, 450)
    expect(result).toEqual({ x: 0, y: 0, w: 800, h: 225 })
  })

  it('width and height scale independently', () => {
    const result = bboxToPixels({ xMin: 0.25, yMin: 0.5, xMax: 0.75, yMax: 1 }, 200, 100)
    expect(result).toEqual({ x: 50, y: 50, w: 100, h: 50 })
  })
})
