export const CARD_CSS_WIDTH = 240

// Union of formats the thumbnail endpoint accepts. Not an interface: this is a
// closed set of string literals, not an object shape.
export type ThumbnailFormat = 'webp' | 'jpeg'

export function thumbnailUrl(
  photoId: string,
  cssWidth: number,
  dpr: 1 | 2 | 3,
  fmt: ThumbnailFormat = 'jpeg',
): string {
  const base = import.meta.env.VITE_API_URL ?? ''
  return `${base}/thumbnails/${photoId}?w=${cssWidth}&dpr=${dpr}&fmt=${fmt}`
}

export function thumbnailSrcSet(
  photoId: string,
  cssWidth: number,
  fmt: ThumbnailFormat = 'jpeg',
): string {
  return [1, 2, 3]
    .map((dpr) => `${thumbnailUrl(photoId, cssWidth, dpr as 1 | 2 | 3, fmt)} ${dpr}x`)
    .join(', ')
}
