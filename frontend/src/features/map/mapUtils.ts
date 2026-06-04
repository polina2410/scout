import type { Photo } from '../../api'

export const FLOOR_SIZE_M = 40
export const CANVAS_SIZE_PX = 400

export function metersToCanvas(
  xM: number,
  yM: number,
  scalePxPerM: number,
): { cx: number; cy: number } {
  return {
    cx: xM * scalePxPerM,
    cy: (FLOOR_SIZE_M - yM) * scalePxPerM,
  }
}

export function canvasToMeters(
  cx: number,
  cy: number,
  scalePxPerM: number,
): { x: number; y: number } {
  return {
    x: cx / scalePxPerM,
    y: FLOOR_SIZE_M - cy / scalePxPerM,
  }
}

export function distanceM(x1: number, y1: number, x2: number, y2: number): number {
  return Math.sqrt((x2 - x1) ** 2 + (y2 - y1) ** 2)
}

export function photosNearby(
  photos: Photo[],
  cx: number,
  cy: number,
  radius: number,
): Photo[] {
  return photos.filter((p) => distanceM(p.x, p.y, cx, cy) <= radius)
}
