# Spec 13 — Filters

**Plan ref:** Phase 7, Step 13  
**Goal:** A `FilterBar` component lets users filter the gallery by pest class and minimum confidence; filter state lives in Redux and drives `GET /photos` query params by wiring into `usePhotos`.

---

## 1. What is already in place — do not redo

- `filtersSlice.ts` — `setClassId(ClassId | null)`, `setMinConfidence(number)`, `resetFilters()`; clamped to `[0,1]`; wired into the Redux store
- `CLASS_IDS` tuple and `ClassId` union type in `features/filters/types.ts`
- `CLASS_COLORS` in `features/gallery/bboxUtils.ts` — reuse for button accent colours
- `usePhotos(params?)` already accepts `Pick<ListPhotosParams, 'classId' | 'minConfidence'>` and resets+refetches when params change — it just isn't being called with params yet
- `GalleryGrid` calls `usePhotos()` with no args — needs to pass Redux state

---

## 2. `FilterBar` component — `src/features/filters/FilterBar.tsx`

```typescript
export function FilterBar(): React.ReactElement
```

Reads `classId` and `minConfidence` from Redux via `useAppSelector`; dispatches via `useAppDispatch`. No props.

**Class buttons** — one pill button per class ID, plus an "All" button:

```tsx
<div className={styles.classes}>
  <button
    className={`${styles.classBtn} ${classId === null ? styles.active : ''}`}
    onClick={() => dispatch(setClassId(null))}
  >
    All
  </button>
  {CLASS_IDS.map((id) => (
    <button
      key={id}
      className={`${styles.classBtn} ${classId === id ? styles.active : ''}`}
      style={{ '--accent': CLASS_COLORS[id] } as React.CSSProperties}
      onClick={() => dispatch(setClassId(id))}
    >
      {CLASS_LABEL[id]}
    </button>
  ))}
</div>
```

Export a `CLASS_LABEL` record from `FilterBar.tsx` (not from `types.ts`) mapping each class ID to a human-readable label:

```typescript
export const CLASS_LABEL: Record<ClassId, string> = {
  powdery_mildew: 'Powdery Mildew',
  mirid:          'Mirid',
  whitefly_aphid: 'Whitefly / Aphid',
  miner_tuta:     'Miner / Tuta',
  thrips:         'Thrips',
  spider_mites:   'Spider Mites',
}
```

The active button uses `--accent` to tint its border/background from `CLASS_COLORS`. "All" has no accent.

**Confidence slider:**

```tsx
<div className={styles.confidence}>
  <label htmlFor="confidence-slider" className={styles.label}>
    Min confidence: {Math.round(minConfidence * 100)}%
  </label>
  <input
    id="confidence-slider"
    type="range"
    min={0}
    max={100}
    step={1}
    value={Math.round(minConfidence * 100)}
    onChange={(e) => dispatch(setMinConfidence(Number(e.target.value) / 100))}
    className={styles.slider}
  />
</div>
```

**Reset button** — only rendered when filters are non-default (`classId !== null || minConfidence > 0`):

```tsx
{(classId !== null || minConfidence > 0) && (
  <button className={styles.resetBtn} onClick={() => dispatch(resetFilters())}>
    Reset
  </button>
)}
```

Layout: `FilterBar` is a horizontal bar (`display: flex`, `align-items: center`, `gap: 16px`, `flex-wrap: wrap`, `padding: 12px 16px`). Class buttons wrap naturally on narrow viewports.

---

## 3. CSS — `src/features/filters/FilterBar.module.css`

```css
.bar {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
  padding: 12px 16px;
  background: #111;
  border-bottom: 1px solid #222;
}

.classes {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.classBtn {
  padding: 4px 12px;
  border-radius: 9999px;
  border: 1px solid #444;
  background: transparent;
  color: #ccc;
  cursor: pointer;
  font-size: 13px;
  transition: border-color 0.15s, background 0.15s;
}

.classBtn:hover {
  border-color: #888;
  color: #fff;
}

.classBtn.active {
  border-color: var(--accent, #4a9eff);
  background: color-mix(in srgb, var(--accent, #4a9eff) 15%, transparent);
  color: #fff;
}

.confidence {
  display: flex;
  align-items: center;
  gap: 8px;
}

.label {
  font-size: 13px;
  color: #ccc;
  white-space: nowrap;
  min-width: 11ch;
}

.slider {
  width: 120px;
  accent-color: #4a9eff;
}

.resetBtn {
  padding: 4px 12px;
  border-radius: 4px;
  border: 1px solid #555;
  background: transparent;
  color: #aaa;
  cursor: pointer;
  font-size: 13px;
}

.resetBtn:hover {
  border-color: #888;
  color: #fff;
}
```

---

## 4. Wire `GalleryGrid` to Redux filter state

`GalleryGrid` reads filters from Redux and passes them to `usePhotos`:

```typescript
export function GalleryGrid() {
  const classId = useAppSelector((s) => s.filters.classId)
  const minConfidence = useAppSelector((s) => s.filters.minConfidence)
  const { photos, status, error, loadMoreError, hasMore, loadMore } = usePhotos(
    { classId: classId ?? undefined, minConfidence }
  )
  // ... rest unchanged
}
```

`usePhotos` already destructures `classId` and `minConfidence` individually as effect deps, so a filter change will reset the photo list and refetch page 1 automatically.

---

## 5. Wire `FilterBar` into `App.tsx`

Add `FilterBar` between the `<header>` and `<main>`:

```tsx
import { FilterBar } from './features/filters/FilterBar'

// inside App():
<div className={styles.layout}>
  <header className={styles.header}>
    <h1 className={styles.title}>Scout</h1>
  </header>
  <FilterBar />
  <main className={styles.main}>
    <GalleryGrid />
  </main>
</div>
```

Update `features/filters/index.ts` to also re-export `FilterBar` and `CLASS_LABEL`.

---

## 6. Tests — `src/features/filters/FilterBar.test.tsx`

Mock the Redux store using `configureStore` (same pattern as `GalleryGrid.test.tsx`). Mock `../../api` with `vi.mock` so `listPhotos` never fires during filter tests.

| Test | Arrange | Assert |
|---|---|---|
| `renders all class buttons and All` | default store | 7 buttons visible (All + 6 classes) |
| `active class button matches Redux state` | store with `classId: 'mirid'` | mirid button has `active` style; All does not |
| `clicking a class button dispatches setClassId` | spy on dispatch | click `thrips` → `setClassId('thrips')` dispatched |
| `clicking All dispatches setClassId(null)` | store with `classId: 'mirid'` | click All → `setClassId(null)` dispatched |
| `reset button hidden when filters are default` | default store (`classId:null`, `minConfidence:0`) | no reset button in DOM |
| `reset button visible when classId is set` | store with `classId: 'thrips'` | reset button present |
| `clicking reset dispatches resetFilters` | store with `classId: 'mirid'` | click reset → `resetFilters()` dispatched |

---

## Acceptance criteria

- [ ] `pnpm test` passes — all new tests green, no existing tests broken
- [ ] `pnpm build` passes — no TypeScript errors
- [ ] `pnpm lint` passes — no errors
- [ ] `FilterBar` visible between header and gallery in the browser
- [ ] Clicking a class button filters the grid — only photos with that pest class shown
- [ ] Clicking "All" returns the full unfiltered grid
- [ ] Moving the confidence slider filters the grid — low-confidence detections excluded
- [ ] Reset button appears when a filter is active and clears both filters when clicked
- [ ] Changing a filter resets the grid to page 1 (no stale photos from prior query)
- [ ] No `any` types in new files
- [ ] CSS Modules for all styles — no inline styles except the `--accent` CSS custom property
