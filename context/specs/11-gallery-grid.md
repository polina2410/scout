# Spec 11 — Gallery Grid

**Plan ref:** Phase 7, Step 11  
**Goal:** A scrolling, cursor-paginated photo grid that fetches from `GET /photos`, displays thumbnails via `srcset` at multiple DPRs, and renders correct loading, empty, and error states — wired into the Scout app shell.

---

## 1. What is already in place — do not redo

- `src/api/` — `listPhotos`, `getPhoto`, `ApiError`, `AsyncState<T>`, `Photo`, `PhotoPage` all exported
- `src/features/filters/` — `FiltersState`, `CLASS_IDS`, `ClassId`; `classId` and `minConfidence` live in Redux
- `src/features/gallery/index.ts` — re-exports `selectPhoto`, `clearSelectedPhoto`, `selectedPhotoReducer`
- `src/store/hooks.ts` — `useAppDispatch`, `useAppSelector`
- `App.tsx` — Scout shell with `<main className={styles.main}>` ready to receive the gallery

---

## 2. Thumbnail URL utility — `src/features/gallery/thumbnailUrl.ts`

The thumbnail endpoint is `GET /thumbnails/{photoId}?w={width}&dpr={dpr}&fmt={fmt}` served from the same origin as the API (`VITE_API_URL`). This file has no component dependencies and is independently testable.

```typescript
export const CARD_CSS_WIDTH = 240

export function thumbnailUrl(
  photoId: string,
  cssWidth: number,
  dpr: 1 | 2 | 3,
): string

export function thumbnailSrcSet(photoId: string, cssWidth: number): string
```

- `thumbnailUrl` builds: `${VITE_API_URL}/thumbnails/${photoId}?w=${cssWidth}&dpr=${dpr}&fmt=jpeg`
- `thumbnailSrcSet` returns `"...url 1x, ...url 2x, ...url 3x"` for dpr 1, 2, 3 using `thumbnailUrl`
- Both read `import.meta.env.VITE_API_URL` at call time (no module-level `throw` — unlike the API client, an unconfigured thumbnail URL is visible in the UI, not a crash)
- `fmt` is always `jpeg`; WebP delivery is deferred until CGO is available on the build host

The `CARD_CSS_WIDTH` constant is exported so `PhotoCard` and tests share the same value.

---

## 3. `usePhotos` hook — `src/features/gallery/usePhotos.ts`

```typescript
const PAGE_LIMIT = 20

interface UsePhotosResult {
  photos: Photo[]
  status: 'loading' | 'loading-more' | 'success' | 'error'
  error: string | null
  hasMore: boolean
  loadMore: () => void
}

export function usePhotos(
  params?: Pick<ListPhotosParams, 'classId' | 'minConfidence'>,
): UsePhotosResult
```

Behaviour:

- On mount and whenever `params` changes: resets `photos` to `[]`, clears the cursor, sets `status: 'loading'`, then calls `listPhotos({ ...params, limit: PAGE_LIMIT })`
- On success (first page): sets `photos` to `items`, records `next_token` as cursor if present, sets `status: 'success'`
- `loadMore()`: no-op if `status` is `'loading'` or `'loading-more'` or `hasMore` is false; otherwise sets `status: 'loading-more'` and calls `listPhotos({ ...params, cursor, limit: PAGE_LIMIT })`
- On `loadMore` success: **appends** `items` to existing `photos`, updates cursor, sets `status: 'success'`
- On any fetch error: sets `status: 'error'` and `error` to `err.message`; existing photos are preserved on a `loadMore` failure so the grid does not go blank
- `hasMore` is `true` when the last successful response contained a `next_token`

Use `useReducer` internally to avoid stale closure issues with `status` and `cursor`. The reducer manages: `{ photos, status, error, cursor }`.

The hook accepts `params` for filter integration in Step 13. In this step, `GalleryGrid` calls `usePhotos()` with no arguments.

---

## 4. `PhotoCard` component — `src/features/gallery/PhotoCard.tsx`

```typescript
interface PhotoCardProps {
  photo: Photo
}
```

Structure:

```tsx
<article className={styles.card} onClick={handleClick}>
  <div className={styles.imageWrapper}>
    <img
      src={thumbnailUrl(photo.id, CARD_CSS_WIDTH, 1)}
      srcSet={thumbnailSrcSet(photo.id, CARD_CSS_WIDTH)}
      alt={`Photo ${photo.id}`}
      className={styles.image}
      loading="lazy"
      decoding="async"
    />
    {/* Step 12: bbox canvas overlay goes here, inside imageWrapper */}
  </div>
</article>
```

- `handleClick` dispatches `selectPhoto(photo.id)` via `useAppDispatch`
- `imageWrapper` has `position: relative` in CSS so Step 12 can position the canvas overlay with `position: absolute; inset: 0`
- `image` fills `imageWrapper` with `width: 100%; height: 100%; object-fit: cover`
- Card has a fixed aspect ratio (16:9) set via `aspect-ratio: 16/9` on `imageWrapper`; this keeps the grid stable while images load
- No `width` or `height` attributes needed on `<img>` because `aspect-ratio` handles layout shift prevention

---

## 5. `GalleryGrid` component — `src/features/gallery/GalleryGrid.tsx`

```typescript
export function GalleryGrid(): React.ReactElement
```

Calls `usePhotos()` (no params in this step). Renders four states:

| `status` | Render |
|---|---|
| `'loading'` | `<div className={styles.loading}>Loading…</div>` — centered text, not a blank screen |
| `'error'` | `<div className={styles.error}>{error}</div>` — shows the error message |
| `'success'` + `photos.length === 0` | `<div className={styles.empty}>No photos found.</div>` |
| `'success'` + photos present | Grid of `<PhotoCard>` elements with infinite scroll sentinel |

Grid layout (`GalleryGrid.module.css`):

```css
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 12px;
  padding: 16px;
}
```

**Infinite scroll sentinel:** a `<div ref={sentinelRef} className={styles.sentinel} />` rendered after the grid. An `IntersectionObserver` watches it; when it enters the viewport and `hasMore` is true and `status` is not `'loading-more'`, calls `loadMore()`. The observer is created in a `useEffect` and disconnected on cleanup.

**Load-more indicator:** when `status === 'loading-more'`, render `<div className={styles.loadingMore}>Loading more…</div>` below the grid (above the sentinel). This gives feedback without hiding the existing photos.

The `sentinelRef` element needs a minimum height so the observer can detect it:

```css
.sentinel {
  height: 1px;
}
```

---

## 6. Wire into `App.tsx`

Import `GalleryGrid` and render it inside `<main>`:

```tsx
import { GalleryGrid } from './features/gallery/GalleryGrid'

// inside App():
<main className={styles.main}>
  <GalleryGrid />
</main>
```

Update `src/features/gallery/index.ts` to also re-export `GalleryGrid`.

---

## 7. Tests

**`src/features/gallery/thumbnailUrl.test.ts`**

| Test | Assert |
|---|---|
| `thumbnailUrl builds correct URL` | `thumbnailUrl('abc', 240, 2)` → contains `/thumbnails/abc`, `w=240`, `dpr=2`, `fmt=jpeg` |
| `thumbnailUrl includes base URL` | result starts with the stubbed `VITE_API_URL` |
| `thumbnailSrcSet includes 1x, 2x, 3x entries` | result contains `1x`, `2x`, `3x` and all three photo URLs |

Stub `VITE_API_URL` with `vi.stubEnv` as in `client.test.ts`.

**`src/features/gallery/GalleryGrid.test.tsx`**

Mock `listPhotos` from `src/api` with `vi.mock('../../../api')` (or the relative path from the test file). Use `@testing-library/react`'s `render` and `screen`.

| Test | Arrange | Assert |
|---|---|---|
| `shows loading state initially` | `listPhotos` never resolves | `screen.getByText(/loading/i)` present |
| `renders photo cards on success` | `listPhotos` resolves with 3 photos | 3 card elements rendered (by role or test id) |
| `shows empty state when no photos` | `listPhotos` resolves with `{ items: [] }` | `screen.getByText(/no photos/i)` present |
| `shows error state on failure` | `listPhotos` rejects with `new Error('network error')` | `screen.getByText(/network error/i)` present |

Do not test `IntersectionObserver` logic in unit tests — it requires a browser environment that `jsdom` does not fully support.

---

## Acceptance criteria

- [ ] `pnpm test` passes — all new tests green, no existing tests broken
- [ ] `pnpm build` passes — no TypeScript errors
- [ ] `pnpm lint` passes — no errors
- [ ] Gallery grid is visible at `http://localhost:5174` after `make seed` (50 photo cards rendered)
- [ ] Thumbnails load at correct sizes — network tab shows `/thumbnails/{id}?w=240&dpr=...` requests
- [ ] Scrolling to the bottom of the page triggers `loadMore` and additional cards appear
- [ ] Loading state is visible on first render (not a blank screen)
- [ ] Empty state renders when API returns zero photos (verifiable by filtering to an impossible query manually)
- [ ] Error state renders when the backend is unreachable (verifiable by stopping the backend)
- [ ] `PhotoCard`'s `imageWrapper` has `position: relative` — ready for Step 12 bbox canvas
- [ ] `usePhotos` resets and refetches when `params` argument changes (required for Step 13 filter wiring)
- [ ] No `any` types in new files
- [ ] CSS Modules used for all styles — no global class strings in `.tsx` files
