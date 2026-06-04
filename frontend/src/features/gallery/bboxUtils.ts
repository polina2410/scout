import type { BoundingBox } from '../../api'

export interface PixelRect {
  x: number
  y: number
  w: number
  h: number
}

export function bboxToPixels(
  bbox: BoundingBox,
  renderedWidth: number,
  renderedHeight: number,
): PixelRect {
  return {
    x: bbox.xMin * renderedWidth,
    y: bbox.yMin * renderedHeight,
    w: (bbox.xMax - bbox.xMin) * renderedWidth,
    h: (bbox.yMax - bbox.yMin) * renderedHeight,
  }
}

export const CLASS_COLORS: Record<string, string> = {
  powdery_mildew: '#ffffff',
  mirid:          '#ef4444',
  whitefly_aphid: '#facc15',
  miner_tuta:     '#fb923c',
  thrips:         '#60a5fa',
  spider_mites:   '#e879f9',
}

export const FALLBACK_COLOR = '#22d3ee'
