# Spec 12 — Bounding Box Overlay

**Plan ref:** Phase 7, Step 12  
**Goal:** Draw prediction bounding boxes on a canvas overlay on each `PhotoCard` thumbnail — normalized bbox coordinates converted to rendered CSS pixels, correct at every size and DPR, updating when the card resizes.

---

## 1. What is already in place — do not redo

- `PhotoCard.tsx` — `imageWrapper` has `position: relative` and `aspect-ratio: 16/9`; the canvas goes inside `imageWrapper`, after `<img>`
- `Prediction` and `BoundingBox` types exported from `src/api`
- `CARD_CSS_WIDTH = 240` exported from `thumbnailUrl.ts`
- `Photo` carries `predictions: Prediction[]` in every API response

---

## 2. Coordinate transform utility — `src/features/gallery/bboxUtils.ts`

Pure function, no React or DOM dependencies — independently testable.

```typescript
import type { BoundingBox } from '../../api'

export interface PixelRect {
  x: number  // left edge in CSS pixels
  y: number  // top edge in CSS pixels
  w: number  // width in CSS pixels
  h: number  // height in CSS pixels
}

export function bboxToPixels(
  bbox: BoundingBox,
  renderedWidth: number,
  renderedHeight: number,
): PixelRect
```

Rules:
- `x = bbox.xMin * renderedWidth`
- `y = bbox.yMin * renderedHeight`
- `w = (bbox.xMax - bbox.xMin) * renderedWidth`
- `h = (bbox.yMax - bbox.yMin) * renderedHeight`
- **Never** multiply by the original image dimensions (`photo.width` / `photo.height`) — the rendered size is always what matters
- All inputs and outputs are CSS pixels; DPR scaling is done in the draw layer, not here

Also export class-to-colour mapping:

```typescript
export const CLASS_COLORS: Record<string, string> = {
  powdery_mildew: '#ffffff',
  mirid:          '#ef4444',
  whitefly_aphid: '#facc15',
  miner_tuta:     '#fb923c',
  thrips:         '#60a5fa',
  spider_mites:   '#e879f9',
}

export const FALLBACK_COLOR = '#22d3ee'
```

---

## 3. `BboxCanvas` component — `src/features/gallery/BboxCanvas.tsx`

```typescript
interface BboxCanvasProps {
  predictions: Prediction[]
}

export function BboxCanvas({ predictions }: BboxCanvasProps): React.ReactElement
```

Renders:

```tsx
<canvas ref={canvasRef} className={styles.canvas} />
```

CSS (`BboxCanvas.module.css`):

```css
.canvas {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;
}
```

`pointer-events: none` ensures clicks pass through to the `PhotoCard` article beneath.

**Drawing logic** — implement as an inner `draw()` function called from effects:

1. Read `canvas.clientWidth` and `canvas.clientHeight` — the rendered CSS pixel dimensions
2. If either is zero, return early (element not yet laid out)
3. Set `canvas.width = clientWidth * devicePixelRatio` and `canvas.height = clientHeight * devicePixelRatio`
4. Get 2D context; call `ctx.clearRect(0, 0, canvas.width, canvas.height)`
5. Call `ctx.scale(devicePixelRatio, devicePixelRatio)` so all subsequent draw calls use CSS pixel units
6. For each prediction:
   - Compute `{ x, y, w, h } = bboxToPixels(pred.bbox, clientWidth, clientHeight)`
   - `ctx.strokeStyle = CLASS_COLORS[pred.classId] ?? FALLBACK_COLOR`
   - `ctx.lineWidth = 2`
   - `ctx.strokeRect(x, y, w, h)`

**Effect:** Use a single `useEffect` that depends on `predictions`. Inside:
1. Draw immediately (handles the initial render and predictions changes)
2. Create a `ResizeObserver` on the canvas element that calls `draw()` on every resize entry
3. Return a cleanup that calls `observer.disconnect()`

This means the observer is recreated when `predictions` changes — acceptable given card-level granularity.

---

## 4. Wire into `PhotoCard.tsx`

Add `BboxCanvas` inside `imageWrapper`, after `<img>`:

```tsx
<div className={styles.imageWrapper}>
  <img
    src={thumbnailUrl(photo.id, CARD_CSS_WIDTH, 1)}
    srcSet={thumbnailSrcSet(photo.id, CARD_CSS_WIDTH)}
    alt={`Photo ${photo.id}`}
    className={styles.image}
    loading="lazy"
    decoding="async"
  />
  <BboxCanvas predictions={photo.predictions} />
</div>
```

No other changes to `PhotoCard`.

---

## 5. Tests — `src/features/gallery/bboxUtils.test.ts`

No jsdom required — pure Vitest.

| Test | Arrange | Assert |
|---|---|---|
| `converts normalized bbox to CSS pixels` | `bbox={xMin:0.1, yMin:0.2, xMax:0.5, yMax:0.8}`, 400×300 | `{x:40, y:60, w:160, h:180}` |
| `full-extent bbox fills rendered area` | `bbox={xMin:0, yMin:0, xMax:1, yMax:1}`, 240×135 | `{x:0, y:0, w:240, h:135}` |
| `zero-size bbox at a point` | `bbox={xMin:0.5, yMin:0.5, xMax:0.5, yMax:0.5}`, 400×300 | `{x:200, y:150, w:0, h:0}` |
| `non-square aspect ratio` | `bbox={xMin:0, yMin:0, xMax:1, yMax:0.5}`, 800×450 | `{x:0, y:0, w:800, h:225}` |
| `renderedWidth and renderedHeight are independent` | `bbox={xMin:0.25, yMin:0.5, xMax:0.75, yMax:1}`, 200×100 | `{x:50, y:50, w:100, h:50}` |

---

## Acceptance criteria

- [ ] `pnpm test` passes — all new tests green, no existing tests broken
- [ ] `pnpm build` passes — no TypeScript errors
- [ ] `pnpm lint` passes — no errors
- [ ] Bounding boxes are visible on photo cards in the browser
- [ ] Box positions are correct — aligned with the detected objects, not offset
- [ ] Boxes remain correctly positioned when the browser window is resized
- [ ] `pointer-events: none` on canvas — clicking a card still dispatches `selectPhoto`
- [ ] No `any` types in new files
- [ ] CSS Modules for all styles — no inline styles in `.tsx` files
- [ ] `bboxToPixels` never uses `photo.width` or `photo.height` — only rendered dimensions
