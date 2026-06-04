# Spec 09 — Frontend Setup

**Plan ref:** Phase 6, Step 9  
**Goal:** Wire the Redux store with a `filters` slice and a `selectedPhoto` slice, provide it to the React tree, and replace the Vite placeholder `App.tsx` with a minimal Scout shell — leaving the codebase ready for the gallery and API client steps.

---

## 1. What is already done — do not redo

The following are already in place and must not be regenerated or reinstalled:

- `frontend/` Vite + React 19 + TypeScript project (`package.json`, `vite.config.ts`, `tsconfig*.json`)
- All npm packages installed (`node_modules/` present)
- `pnpm generate` script — `openapi-typescript ../openapi.yaml -o src/api/generated/schema.ts`
- `src/api/generated/schema.ts` — already generated; do not modify it by hand
- Directory stubs: `src/features/filters/index.ts`, `src/features/gallery/index.ts`, `src/features/map/index.ts`, `src/store/index.ts`
- Test infrastructure: `vitest`, `@testing-library/react`, `@testing-library/jest-dom`, `src/test-setup.ts`
- Dev-server proxy: `/api/*` → `http://localhost:8080/*` (already in `vite.config.ts`)

---

## 2. `ClassId` type and constants — `src/features/filters/types.ts`

Define the known detection class names as a `const` tuple so the type is derived from the values — no hand-written union:

```typescript
export const CLASS_IDS = [
  'powdery_mildew',
  'mirid',
  'whitefly_aphid',
  'miner_tuta',
  'thrips',
  'spider_mites',
] as const

export type ClassId = (typeof CLASS_IDS)[number]
```

These values come from the openapi.yaml description of `classId`; the generated schema expresses `classId` as `string`, so `ClassId` is local to the frontend and not derived from the generated types.

---

## 3. Filters slice — `src/features/filters/filtersSlice.ts`

```typescript
interface FiltersState {
  classId: ClassId | null   // null = no class filter (show all)
  minConfidence: number     // [0, 1], default 0
}

const initialState: FiltersState = { classId: null, minConfidence: 0 }
```

Actions:

| Action | Payload | Behaviour |
|---|---|---|
| `setClassId` | `ClassId \| null` | Replace `classId` |
| `setMinConfidence` | `number` | Replace `minConfidence`; clamp to `[0, 1]` inside the reducer |
| `resetFilters` | — | Restore `initialState` |

Export the reducer as the default export. Export action creators as named exports.

Update `src/features/filters/index.ts` to re-export everything from `filtersSlice.ts` and `types.ts` that consumers need.

---

## 4. Selected photo slice — `src/features/gallery/selectedPhotoSlice.ts`

```typescript
interface SelectedPhotoState {
  photoId: string | null   // null = no photo selected (modal closed)
}

const initialState: SelectedPhotoState = { photoId: null }
```

Actions:

| Action | Payload | Behaviour |
|---|---|---|
| `selectPhoto` | `string` | Set `photoId` |
| `clearSelectedPhoto` | — | Reset `photoId` to `null` |

Export reducer as default; export action creators as named exports.

Update `src/features/gallery/index.ts` to re-export what consumers need.

---

## 5. Store — `src/store/index.ts` and `src/store/hooks.ts`

**`src/store/index.ts`** — replace the empty stub:

```typescript
import { configureStore } from '@reduxjs/toolkit'
import filtersReducer from '../features/filters/filtersSlice'
import selectedPhotoReducer from '../features/gallery/selectedPhotoSlice'

export const store = configureStore({
  reducer: {
    filters: filtersReducer,
    selectedPhoto: selectedPhotoReducer,
  },
})

export type RootState = ReturnType<typeof store.getState>
export type AppDispatch = typeof store.dispatch
```

**`src/store/hooks.ts`** — typed wrappers so components never import raw `useSelector`/`useDispatch`:

```typescript
import { useDispatch, useSelector } from 'react-redux'
import type { TypedUseSelectorHook } from 'react-redux'
import type { RootState, AppDispatch } from './index'

export const useAppDispatch: () => AppDispatch = useDispatch
export const useAppSelector: TypedUseSelectorHook<RootState> = useSelector
```

---

## 6. Wire `Provider` in `src/main.tsx`

Wrap `<App />` with `<Provider store={store}>`. The store must be the outermost wrapper inside `<StrictMode>`:

```tsx
import { Provider } from 'react-redux'
import { store } from './store'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <Provider store={store}>
      <App />
    </Provider>
  </StrictMode>,
)
```

---

## 7. App shell — `src/App.tsx` and `src/App.module.css`

Replace the Vite default `App.tsx` with a minimal Scout shell. The shell must not reference Vite logo assets or the counter button.

```tsx
import styles from './App.module.css'

export default function App() {
  return (
    <div className={styles.layout}>
      <header className={styles.header}>
        <h1 className={styles.title}>Scout</h1>
      </header>
      <main className={styles.main}>
        {/* Gallery and map render here in later steps */}
      </main>
    </div>
  )
}
```

`src/App.module.css` defines `.layout`, `.header`, `.title`, `.main` with minimal styles (full-height flex column, header with a dark background, main taking remaining space). Delete the existing `src/App.css` (Vite default) so no global class strings remain.

`src/index.css` may keep its CSS reset/base rules but must not define class selectors — only element or `:root` selectors. Review it and remove any Vite-specific class rules.

---

## 8. Tests

Two test files — one per slice. No component rendering required; these are pure reducer tests.

**`src/features/filters/filtersSlice.test.ts`**

| Test | Assert |
|---|---|
| `initial state` | `classId === null`, `minConfidence === 0` |
| `setClassId sets a class` | dispatching `setClassId('mirid')` → `classId === 'mirid'` |
| `setClassId(null) clears the filter` | dispatching `setClassId(null)` after setting → `classId === null` |
| `setMinConfidence sets value` | dispatching `setMinConfidence(0.7)` → `minConfidence === 0.7` |
| `setMinConfidence clamps below 0` | dispatching `setMinConfidence(-0.1)` → `minConfidence === 0` |
| `setMinConfidence clamps above 1` | dispatching `setMinConfidence(1.5)` → `minConfidence === 1` |
| `resetFilters restores initial state` | after setting both fields, `resetFilters` → `initialState` |

**`src/features/gallery/selectedPhotoSlice.test.ts`**

| Test | Assert |
|---|---|
| `initial state` | `photoId === null` |
| `selectPhoto sets photoId` | dispatching `selectPhoto('abc-123')` → `photoId === 'abc-123'` |
| `clearSelectedPhoto resets to null` | after `selectPhoto`, dispatching `clearSelectedPhoto` → `photoId === null` |

Call `reducer(undefined, action)` directly in each test — do not create a full store.

---

## Acceptance criteria

- [ ] `pnpm test` passes (all slice tests green)
- [ ] `pnpm build` passes with no TypeScript errors (`tsc -b && vite build`)
- [ ] `pnpm lint` passes with no errors
- [ ] `pnpm dev` starts and the browser shows the Scout shell (header + empty main, no Vite logos or counter)
- [ ] `store.getState()` returns `{ filters: { classId: null, minConfidence: 0 }, selectedPhoto: { photoId: null } }` on a fresh store
- [ ] `CLASS_IDS` and `ClassId` exported from `src/features/filters/`
- [ ] `useAppDispatch` and `useAppSelector` exported from `src/store/hooks.ts`
- [ ] No `any` types anywhere in new or modified files
- [ ] CSS Modules used for all component styles — no global class strings in `.tsx` files
