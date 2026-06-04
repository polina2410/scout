export const CARD_CSS_WIDTH = 240

export function thumbnailUrl(
  photoId: string,
  cssWidth: number,
  dpr: 1 | 2 | 3,
): string {
  const base = import.meta.env.VITE_API_URL ?? ''
  return `${base}/thumbnails/${photoId}?w=${cssWidth}&dpr=${dpr}&fmt=jpeg`
}

export function thumbnailSrcSet(photoId: string, cssWidth: number): string {
  return [1, 2, 3]
    .map((dpr) => `${thumbnailUrl(photoId, cssWidth, dpr as 1 | 2 | 3)} ${dpr}x`)
    .join(', ')
}
