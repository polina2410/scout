# Spec 14 — Photo Modal

**Plan ref:** Phase 7, Step 14  
**Goal:** Clicking a photo card opens a full-screen modal that fetches and displays the original image with all prediction bounding boxes overlaid and a sidebar listing each detection's class and confidence.

---

## 1. What is already in place — do not redo

- `selectedPhotoSlice` — `photoId: string | null`; `selectPhoto(id)` dispatched by `PhotoCard` on click; `clearSelectedPhoto()` to close
- `getPhoto(photoId)` in `src/api/` — fetches `Photo` with fresh `originalUrl` and full `predictions[]`
- `BboxCanvas` — reuse as-is; it observes its own size, so it works at any dimensions
- `CLASS_LABEL` in `features/filters/classLabels.ts`
- `CLASS_COLORS` in `features/gallery/bboxUtils.ts`
- `useAppSelector` / `useAppDispatch` from `store/hooks`

---

## 2. `usePhotoDetail` hook — `src/features/gallery/usePhotoDetail.ts`

Encapsulates the async fetch so `PhotoModal` stays a pure rendering component.

```typescript
interface UsePhotoDetailResult {
  photo: Photo | null
  status: 'idle' | 'loading' | 'success' | 'error'
  error: string | null
}

export function usePhotoDetail(photoId: string | null): UsePhotoDetailResult
```

Behaviour:
- When `photoId` is `null`: return `{ photo: null, status: 'idle', error: null }` immediately, no fetch
- When `photoId` changes to a non-null string: set `status: 'loading'`, call `getPhoto(photoId)`, on success set `status: 'success'` + `photo`; on failure set `status: 'error'` + `error: err.message`
- Use a cancellation flag (`let cancelled = false`) so a stale response from a prior `photoId` is discarded
- Reset to `idle` (clear photo) when `photoId` becomes `null` again

Use `useEffect` with `photoId` as the sole dependency.

---

## 3. `PhotoModal` component — `src/features/gallery/PhotoModal.tsx`

```typescript
export function PhotoModal(): React.ReactElement | null
```

Reads `photoId` from Redux (`s.selectedPhoto.photoId`). Returns `null` when `photoId` is `null` (no DOM output at all).

When `photoId` is non-null, renders:

```tsx
<div className={styles.backdrop} onClick={handleBackdropClick}>
  <div
    role="dialog"
    aria-modal="true"
    aria-label="Photo detail"
    className={styles.dialog}
    onClick={(e) => e.stopPropagation()}
  >
    <button
      className={styles.closeBtn}
      aria-label="Close"
      onClick={() => dispatch(clearSelectedPhoto())}
    >
      ×
    </button>

    {/* loading */}
    {status === 'loading' && <div className={styles.loading}>Loading…</div>}

    {/* error */}
    {status === 'error' && <div className={styles.error}>{error}</div>}

    {/* success */}
    {status === 'success' && photo && (
      <>
        <div className={styles.imageSection}>
          <div className={styles.imageWrapper}>
            <img
              src={photo.originalUrl}
              alt={`Photo ${photo.id}`}
              className={styles.image}
            />
            <BboxCanvas predictions={photo.predictions} />
          </div>
        </div>
        <div className={styles.sidebar}>
          <h2 className={styles.sidebarTitle}>Detections</h2>
          {photo.predictions.length === 0 ? (
            <p className={styles.noDetections}>No detections.</p>
          ) : (
            <ul className={styles.predList}>
              {photo.predictions.map((pred, i) => (
                <li key={i} className={styles.predItem}>
                  <span
                    className={styles.dot}
                    style={{ background: CLASS_COLORS[pred.classId] ?? FALLBACK_COLOR }}
                  />
                  <span className={styles.predClass}>
                    {CLASS_LABEL[pred.classId] ?? pred.classId}
                  </span>
                  <span className={styles.predConf}>
                    {Math.round(pred.confidence * 100)}%
                  </span>
                </li>
              ))}
            </ul>
          )}
        </div>
      </>
    )}
  </div>
</div>
```

**Closing behaviour:**
- Close button (`×`): dispatches `clearSelectedPhoto()`
- Backdrop click (`handleBackdropClick`): dispatches `clearSelectedPhoto()` — `stopPropagation` on the inner dialog prevents bubbling
- `Escape` key: `useEffect` adds a `keydown` listener on `document`; cleans up on unmount

**Focus management:**
- On mount (when `photoId` becomes non-null): move focus to the close button via a `ref` + `useEffect`
- On unmount: focus returns to the previously focused element — capture `document.activeElement` before setting focus, restore it in the cleanup

---

## 4. CSS — `src/features/gallery/PhotoModal.module.css`

```css
.backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.8);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
}

.dialog {
  position: relative;
  display: flex;
  background: #1a1a1a;
  border-radius: 8px;
  overflow: hidden;
  max-width: min(1200px, 95vw);
  max-height: 90vh;
  width: 100%;
}

.closeBtn {
  position: absolute;
  top: 8px;
  right: 12px;
  background: transparent;
  border: none;
  color: #fff;
  font-size: 24px;
  line-height: 1;
  cursor: pointer;
  z-index: 1;
  padding: 4px 8px;
}

.closeBtn:hover {
  color: #aaa;
}

.imageSection {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #000;
  overflow: hidden;
}

.imageWrapper {
  position: relative;
  width: 100%;
  height: 100%;
  max-height: 90vh;
  display: flex;
  align-items: center;
  justify-content: center;
}

.image {
  max-width: 100%;
  max-height: 90vh;
  object-fit: contain;
  display: block;
}

.sidebar {
  width: 240px;
  flex-shrink: 0;
  padding: 48px 16px 16px;
  overflow-y: auto;
  border-left: 1px solid #333;
}

.sidebarTitle {
  font-size: 14px;
  font-weight: 600;
  color: #ccc;
  margin: 0 0 12px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.noDetections {
  font-size: 13px;
  color: #666;
}

.predList {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.predItem {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
}

.dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
}

.predClass {
  flex: 1;
  color: #ddd;
}

.predConf {
  color: #888;
  font-variant-numeric: tabular-nums;
}

.loading,
.error {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  padding: 48px;
  color: #888;
}

.error {
  color: #e05c5c;
}
```

---

## 5. Wire `PhotoModal` into `App.tsx`

```tsx
import { PhotoModal } from './features/gallery/PhotoModal'

// inside App():
<div className={styles.layout}>
  <header ...>...</header>
  <FilterBar />
  <main className={styles.main}>
    <GalleryGrid />
  </main>
  <PhotoModal />
</div>
```

`PhotoModal` returns `null` when no photo is selected, so it adds no DOM overhead in the default state.

Update `src/features/gallery/index.ts` to also re-export `PhotoModal`.

---

## 6. Tests — `src/features/gallery/PhotoModal.test.tsx`

Mock `../../api` with `vi.mock`. Mock `getPhoto` specifically.

| Test | Arrange | Assert |
|---|---|---|
| `returns null when no photo selected` | store with `photoId: null` | nothing rendered (`container` is empty) |
| `shows loading state while fetching` | `getPhoto` never resolves; store with `photoId: 'abc'` | `Loading…` text visible |
| `renders image and predictions on success` | `getPhoto` resolves with a photo with 2 predictions | `<img>` in DOM; 2 prediction items visible |
| `shows error message on fetch failure` | `getPhoto` rejects with `new Error('not found')` | error text visible |
| `close button dispatches clearSelectedPhoto` | success state | click close button → `clearSelectedPhoto()` dispatched |
| `backdrop click dispatches clearSelectedPhoto` | success state | click backdrop div → `clearSelectedPhoto()` dispatched |

---

## Acceptance criteria

- [ ] `pnpm test` passes — all new tests green, no existing tests broken
- [ ] `pnpm build` passes — no TypeScript errors
- [ ] `pnpm lint` passes — no errors
- [ ] Clicking a photo card opens the modal with the full-size image
- [ ] Bounding boxes are drawn over the full-size image (correctly scaled, not thumbnail-sized)
- [ ] Prediction list shows all detections with class name, colour dot, and confidence %
- [ ] Close button, Escape key, and backdrop click all close the modal
- [ ] Focus moves to close button on open; returns to the card that was clicked on close
- [ ] No `any` types in new files
- [ ] CSS Modules for all styles — `--dot-color` inline style is the only exception
