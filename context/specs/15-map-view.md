# Spec 15 — Greenhouse Map

**Plan ref:** Phase 8, Step 15  
**Goal:** A Konva canvas showing every photo's position on the 40×40 m greenhouse floor — zoomable, pannable, click a photo dot to open the modal, click empty space to set a location filter that restricts the gallery to nearby photos.

---

## 1. What is already in place — do not redo

- `react-konva` and `konva` already installed
- `src/features/map/index.ts` exists as an empty barrel
- `filtersSlice` — `classId`, `minConfidence` — shared state already driving the gallery
- `selectPhoto` / `clearSelectedPhoto` in Redux — used by the map to open the modal
- `CLASS_COLORS` in `bboxUtils.ts`

---

## 2. `filtersSlice` additions — location filter

Add to `FiltersState` and `filtersSlice`:

```typescript
interface LocationFilter {
  x: number      // meters, center of the selected area
  y: number      // meters
  radius: number // meters — user-adjustable, default 5
}

interface FiltersState {
  classId: ClassId | null
  minConfidence: number
  locationFilter: LocationFilter | null  // NEW
}
```

New actions:
```typescript
setLocationFilter(state, action: PayloadAction<{ x: number; y: number }>) {
  const radius = state.locationFilter?.radius ?? 5
  state.locationFilter = { x: action.payload.x, y: action.payload.y, radius }
}
setLocationRadius(state, action: PayloadAction<number>) {
  if (state.locationFilter) state.locationFilter.radius = action.payload
}
clearLocationFilter(state) {
  state.locationFilter = null
}
```

`resetFilters` must also clear `locationFilter`.

Export the three new actions from `features/filters/index.ts`.

---

## 3. Map utilities — `src/features/map/mapUtils.ts`

Pure functions, no React or DOM dependencies.

```typescript
export const FLOOR_SIZE_M = 40       // greenhouse is 40×40 m
export const CANVAS_SIZE_PX = 400    // default canvas CSS size

export function metersToCanvas(
  xM: number,
  yM: number,
  scalePxPerM: number,
): { cx: number; cy: number }
```

- `cx = xM * scalePxPerM`
- `cy = (FLOOR_SIZE_M - yM) * scalePxPerM`  ← Y axis is inverted (canvas Y grows down)

```typescript
export function canvasToMeters(
  cx: number,
  cy: number,
  scalePxPerM: number,
): { x: number; y: number }
```

- `x = cx / scalePxPerM`
- `y = FLOOR_SIZE_M - cy / scalePxPerM`

```typescript
export function distanceM(
  x1: number, y1: number,
  x2: number, y2: number,
): number
```

- `Math.sqrt((x2 - x1) ** 2 + (y2 - y1) ** 2)`

```typescript
export function photosNearby(
  photos: Photo[],
  cx: number,
  cy: number,
  radius: number,
): Photo[]
```

- Returns photos where `distanceM(photo.x, photo.y, cx, cy) <= radius`

---

## 4. `useAllPhotos` hook — `src/features/map/useAllPhotos.ts`

Fetches all photos once (no pagination, no filter) to populate the map dots. Only called by `MapView`.

```typescript
interface UseAllPhotosResult {
  photos: Photo[]
  status: 'loading' | 'success' | 'error'
}

export function useAllPhotos(): UseAllPhotosResult
```

- Calls `listPhotos({ limit: 50 })` once on mount; does not re-fetch
- On success: `status: 'success'`, `photos: page.items`
- On error: `status: 'error'`, `photos: []`

---

## 5. `MapView` component — `src/features/map/MapView.tsx`

```typescript
export function MapView(): React.ReactElement
```

Uses `react-konva`. Reads `classId`, `minConfidence`, `locationFilter` from Redux via `useAppSelector`. Dispatches `selectPhoto`, `setLocationFilter`, `clearLocationFilter`.

**Stage setup:**

```tsx
<Stage
  width={CANVAS_SIZE_PX}
  height={CANVAS_SIZE_PX}
  draggable
  onWheel={handleWheel}
  scaleX={zoom}
  scaleY={zoom}
  x={panX}
  y={panY}
>
  <Layer>
    {/* grid lines */}
    {/* photo dots */}
    {/* location filter circle */}
  </Layer>
</Stage>
```

Manage `zoom` (default `1`), `panX`/`panY` (default `0`) in local state.

`handleWheel`: zoom in/out centred on cursor position:
```typescript
function handleWheel(e: KonvaEventObject<WheelEvent>) {
  e.evt.preventDefault()
  const scaleBy = 1.1
  const stage = e.target.getStage()!
  const pointer = stage.getPointerPosition()!
  const newZoom = e.evt.deltaY < 0 ? zoom * scaleBy : zoom / scaleBy
  const clamped = Math.min(Math.max(newZoom, 0.5), 8)
  const mousePointTo = { x: (pointer.x - panX) / zoom, y: (pointer.y - panY) / zoom }
  setPanX(pointer.x - mousePointTo.x * clamped)
  setPanY(pointer.y - mousePointTo.y * clamped)
  setZoom(clamped)
}
```

**Grid lines:** draw lines every 5 m:

```tsx
{Array.from({ length: 9 }, (_, i) => (i + 1) * (CANVAS_SIZE_PX / 8)).flatMap((pos, i) => [
  <Line key={`h${i}`} points={[0, pos, CANVAS_SIZE_PX, pos]} stroke="#333" strokeWidth={0.5} />,
  <Line key={`v${i}`} points={[pos, 0, pos, CANVAS_SIZE_PX]} stroke="#333" strokeWidth={0.5} />,
])}
```

Actually, simpler: draw lines at every 5 m = every `5 * (CANVAS_SIZE_PX / FLOOR_SIZE_M)` = every 50 px:

```tsx
{[...Array(7)].map((_, i) => {
  const pos = (i + 1) * 50
  return [
    <Line key={`h${i}`} points={[0, pos, CANVAS_SIZE_PX, pos]} stroke="#2a2a2a" strokeWidth={1} listening={false} />,
    <Line key={`v${i}`} points={[pos, 0, pos, CANVAS_SIZE_PX]} stroke="#2a2a2a" strokeWidth={1} listening={false} />,
  ]
})}
```

**Photo dots:** for each photo from `useAllPhotos()`:

```typescript
const scale = CANVAS_SIZE_PX / FLOOR_SIZE_M  // 10 px/m

function dotColor(photo: Photo): string {
  if (photo.predictions.length === 0) return '#555'
  const top = photo.predictions.reduce((a, b) => a.confidence > b.confidence ? a : b)
  return CLASS_COLORS[top.classId] ?? FALLBACK_COLOR
}

function matchesFilter(photo: Photo): boolean {
  if (!classId && minConfidence === 0) return true
  return photo.predictions.some(
    (p) =>
      (classId ? p.classId === classId : true) &&
      p.confidence >= minConfidence,
  )
}
```

Each dot:
```tsx
<Circle
  key={photo.id}
  x={metersToCanvas(photo.x, photo.y, scale).cx}
  y={metersToCanvas(photo.x, photo.y, scale).cy}
  radius={6}
  fill={dotColor(photo)}
  opacity={matchesFilter(photo) ? 1 : 0.2}
  onClick={() => dispatch(selectPhoto(photo.id))}
  style={{ cursor: 'pointer' }}
/>
```

**Location filter circle:** when `locationFilter` is set:
```tsx
<Circle
  x={metersToCanvas(locationFilter.x, locationFilter.y, scale).cx}
  y={locationFilter.y /* already in canvas coords via metersToCanvas */ }
  radius={locationFilter.radius * scale}
  stroke="#4a9eff"
  strokeWidth={1}
  dash={[4, 4]}
  fill="rgba(74,158,255,0.08)"
  listening={false}
/>
```

**Stage click (empty space):** clicking the Stage background sets the location filter. Distinguish dot clicks vs stage clicks by checking `e.target === e.target.getStage()`:

```typescript
function handleStageClick(e: KonvaEventObject<MouseEvent>) {
  if (e.target !== e.target.getStage()) return  // clicked a dot, not background
  const stage = e.target.getStage()!
  const pos = stage.getRelativePointerPosition()!
  const { x, y } = canvasToMeters(pos.x, pos.y, scale)
  dispatch(setLocationFilter({ x, y }))
}
```

**Radius control:** rendered below the stage in `MapView` (not on the canvas):

```tsx
<div className={styles.controls}>
  <label htmlFor="radius-slider" className={styles.label}>
    Radius: {locationFilter?.radius ?? 5} m
  </label>
  <input
    id="radius-slider"
    type="range"
    min={1}
    max={20}
    step={1}
    value={locationFilter?.radius ?? 5}
    onChange={(e) => dispatch(setLocationRadius(Number(e.target.value)))}
    className={styles.slider}
    disabled={!locationFilter}
  />
  {locationFilter && (
    <button className={styles.clearBtn} onClick={() => dispatch(clearLocationFilter())}>
      Clear
    </button>
  )}
</div>
```

---

## 6. CSS — `src/features/map/MapView.module.css`

```css
.container {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px;
  background: #0d0d0d;
  border-right: 1px solid #222;
  flex-shrink: 0;
}

.title {
  font-size: 12px;
  font-weight: 600;
  color: #888;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin: 0;
}

.stage {
  border: 1px solid #222;
  border-radius: 4px;
  cursor: crosshair;
}

.controls {
  display: flex;
  align-items: center;
  gap: 8px;
}

.label {
  font-size: 12px;
  color: #888;
  white-space: nowrap;
  min-width: 9ch;
}

.slider {
  width: 80px;
  accent-color: #4a9eff;
}

.clearBtn {
  font-size: 12px;
  padding: 2px 8px;
  border: 1px solid #444;
  border-radius: 4px;
  background: transparent;
  color: #aaa;
  cursor: pointer;
}

.clearBtn:hover {
  border-color: #888;
  color: #fff;
}
```

---

## 7. Gallery location filter — `GalleryGrid` change

`GalleryGrid` reads `locationFilter` from Redux and hides photos outside the radius from the already-fetched list:

```typescript
const locationFilter = useAppSelector((s) => s.filters.locationFilter)

// after usePhotos(...):
const visiblePhotos = locationFilter
  ? photosNearby(photos, locationFilter.x, locationFilter.y, locationFilter.radius)
  : photos
```

Render `visiblePhotos` instead of `photos` in the grid. The empty state message becomes `"No photos found."` regardless of which filter caused it.

---

## 8. Wire into `App.tsx`

Add a two-column content area between `FilterBar` and the closing `</div>`:

```tsx
import { MapView } from './features/map/MapView'

// inside App():
<div className={styles.content}>
  <MapView />
  <main className={styles.main}>
    <GalleryGrid />
  </main>
</div>
```

Add to `App.module.css`:
```css
.content {
  display: flex;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}
```

Update `src/features/map/index.ts` to re-export `MapView`.

---

## 9. Tests — `src/features/map/mapUtils.test.ts`

Pure Vitest, no jsdom.

| Test | Arrange | Assert |
|---|---|---|
| `metersToCanvas: origin maps to top-left` | `(0, 40, 10)` | `{ cx: 0, cy: 0 }` |
| `metersToCanvas: far corner maps to bottom-right` | `(40, 0, 10)` | `{ cx: 400, cy: 400 }` |
| `metersToCanvas: centre maps to centre` | `(20, 20, 10)` | `{ cx: 200, cy: 200 }` |
| `canvasToMeters is inverse of metersToCanvas` | round-trip `(15, 30, 10)` | same `(x, y)` back |
| `distanceM: same point is 0` | `(5, 5, 5, 5)` | `0` |
| `distanceM: 3-4-5 triangle` | `(0, 0, 3, 4)` | `5` |
| `photosNearby: returns only photos within radius` | 3 photos at distance 2, 5, 10; radius 6 | 2 photos returned |
| `photosNearby: empty when no photos nearby` | all photos outside radius | `[]` |

---

## Acceptance criteria

- [ ] `pnpm test` passes — all new tests green, no existing tests broken
- [ ] `pnpm build` passes — no TypeScript errors
- [ ] `pnpm lint` passes — no errors
- [ ] Map renders at `http://localhost:5175` alongside the gallery
- [ ] All 50 photo dots visible on the map at their correct positions
- [ ] Dots colored by most confident detection class; grey when no predictions
- [ ] Dots matching the current class/confidence filter are fully opaque; non-matching are dimmed
- [ ] Wheel zoom and drag pan work correctly
- [ ] Clicking a photo dot opens the photo modal
- [ ] Clicking empty canvas space draws a radius circle and filters the gallery
- [ ] Radius slider adjusts the circle and updates the gallery filter
- [ ] Clear button removes the location filter and restores the full gallery
- [ ] Class/confidence filter changes update both map dot opacity and gallery
- [ ] `resetFilters` clears the location filter too
- [ ] No `any` types in new files
- [ ] CSS Modules for all non-canvas styles
