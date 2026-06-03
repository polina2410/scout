# Spec 03 — Data Layer (SQLite)

**Plan ref:** Phase 1, Step 3  
**Goal:** Implement a read-only SQLite package (`internal/db`) that opens `predictions.db`, fetches a single photo with all its predictions, and paginates/filters the photo list — then wire it into `main.go` startup with tests passing.

---

## 1. Dependency

Add `modernc.org/sqlite` — the pure-Go SQLite driver (no CGo required).

```sh
cd backend && go get modernc.org/sqlite
```

Import the driver with a blank import in the `db` package so the `database/sql` driver registers itself:

```go
import _ "modernc.org/sqlite"
```

Do **not** add `mattn/go-sqlite3` — it requires CGo and breaks cross-platform builds.

---

## 2. Types — `internal/db/types.go`

```go
package db

// ClassID is the set of known detection class names.
type ClassID string

const (
    ClassPowderyMildew ClassID = "powdery_mildew"
    ClassMirid         ClassID = "mirid"
    ClassWhiteflyAphid ClassID = "whitefly_aphid"
    ClassMinerTuta     ClassID = "miner_tuta"
    ClassThrips        ClassID = "thrips"
    ClassSpiderMites   ClassID = "spider_mites"
)

// BoundingBox holds a normalized [0,1] bounding box.
type BoundingBox struct {
    XMin float64
    YMin float64
    XMax float64
    YMax float64
}

// Prediction is one model detection attached to a photo.
type Prediction struct {
    ID         string
    ClassID    ClassID
    Confidence float64
    BBox       BoundingBox
}

// Photo is a camera photo with its position and all model predictions.
// OriginalURL is NOT stored in the DB — the handler layer fills it from MinIO before serialising.
type Photo struct {
    ID          string
    X, Y, H     float64
    Width       int
    Height      int
    CapturedAt  string // RFC3339
    OriginalURL string // filled by handler, empty here
    Predictions []Prediction
}

// ListParams controls pagination and filtering for ListPhotos.
type ListParams struct {
    Cursor        string  // opaque token from a previous response
    Limit         int     // 1–200; caller must clamp
    ClassID       string  // empty = no filter
    MinConfidence float64 // 0 = no filter
}
```

---

## 3. `internal/db/db.go` — open, close, sentinel error

```go
// ErrNotFound is returned by GetPhoto when the photo ID does not exist.
var ErrNotFound = errors.New("not found")

// DB wraps a read-only connection pool to predictions.db.
type DB struct {
    db *sql.DB
}

// Open opens predictions.db in read-only mode.
// path is the filesystem path to the SQLite file.
func Open(path string) (*DB, error)

// Close releases the connection pool.
func Close() error
```

**Behaviour:**

- Open using a URI DSN: `file:{path}?mode=ro&_journal=WAL&_busy_timeout=5000`
- Use `database/sql` with driver name `"sqlite"` (registered by `modernc.org/sqlite`)
- Call `db.Ping()` immediately after open; return the error if it fails (fails fast on bad path or locked file)
- Set `db.SetMaxOpenConns(1)` — SQLite performs best with a single writer; reads can share

---

## 4. `GetPhoto(ctx context.Context, id string) (*Photo, error)`

Fetches one photo and all its predictions.

**SQL — photo row:**

```sql
SELECT id, x, y, h, width, height, captured_at
FROM photos
WHERE id = ?
```

Return `ErrNotFound` (not a wrapped error) when `sql.ErrNoRows` is returned.

**SQL — predictions:**

```sql
SELECT id, class_id, confidence, bbox_xmin, bbox_ymin, bbox_xmax, bbox_ymax
FROM predictions
WHERE photo_id = ?
```

Scan all rows into `[]Prediction`. An empty slice (no predictions) is valid — do not error.

Populate `Photo.Predictions` from the slice. Leave `Photo.OriginalURL` empty.

---

## 5. `ListPhotos(ctx context.Context, p ListParams) ([]Photo, string, error)`

Returns a page of photos and an opaque `nextCursor` string (empty on the last page).

### Sort order

`captured_at DESC, id DESC` — stable across pages even when timestamps collide.

### Cursor encoding

The cursor encodes the `captured_at` and `id` of the **last item on the current page**:

```
base64(captured_at + "|" + id)   // standard base64, URL-safe
```

Decode the cursor at the start of the query to obtain `(cursorTime, cursorID)`.  
Add a `WHERE` clause when a cursor is present:

```sql
AND (captured_at < ? OR (captured_at = ? AND id < ?))
```

### Filter — single-prediction invariant

A photo is included only if **one prediction** satisfies **both** `classId` AND `minConfidence` simultaneously. Implement with a subquery:

```sql
AND photos.id IN (
    SELECT DISTINCT photo_id FROM predictions
    WHERE (? = '' OR class_id = ?)
      AND (? = 0  OR confidence >= ?)
)
```

Pass each filter parameter twice (once for the "disabled" check, once for the value).

### Full query skeleton

```sql
SELECT id, x, y, h, width, height, captured_at
FROM photos
WHERE 1=1
  -- cursor clause (only when cursor present)
  AND (captured_at < ? OR (captured_at = ? AND id < ?))
  -- filter clause
  AND photos.id IN (
      SELECT DISTINCT photo_id FROM predictions
      WHERE (? = '' OR class_id = ?)
        AND (? = 0  OR confidence >= ?)
  )
ORDER BY captured_at DESC, id DESC
LIMIT ?
```

### Predictions for matched photos

After fetching the photo page, load all predictions for those photos in a single query using `WHERE photo_id IN (?, ?, ...)`. Attach them to their parent `Photo`. Do **not** issue one query per photo.

### nextCursor

If the number of rows returned equals `Limit`, encode the last row's `captured_at` and `id` as the cursor. Otherwise return `""`.

---

## 6. Updated `main.go`

Wire `db.Open` after config load. Fail fast if the DB cannot be opened.

```go
database, err := db.Open(cfg.DBPath)
if err != nil {
    log.Error("failed to open database", "error", err)
    os.Exit(1)
}
defer database.Close()
```

The `*db.DB` value is stored on an `App` struct (or passed directly) so handlers can use it in a later step. At this stage, opening and deferring close is sufficient.

---

## 7. Tests — `internal/db/db_test.go`

Use an **in-memory SQLite** database (`:memory:`) seeded with the real schema and fixture rows. Do not read from `dataset/predictions.db` in tests.

### Schema to create in test setup

```sql
CREATE TABLE photos (
    id TEXT PRIMARY KEY,
    x REAL NOT NULL, y REAL NOT NULL, h REAL NOT NULL,
    width INTEGER NOT NULL, height INTEGER NOT NULL,
    captured_at TEXT NOT NULL
);
CREATE TABLE predictions (
    id TEXT PRIMARY KEY,
    photo_id TEXT NOT NULL REFERENCES photos(id),
    class_id TEXT NOT NULL,
    confidence REAL NOT NULL,
    bbox_xmin REAL NOT NULL, bbox_ymin REAL NOT NULL,
    bbox_xmax REAL NOT NULL, bbox_ymax REAL NOT NULL
);
```

### Fixture data

Insert at least 3 photos with varied positions and timestamps, and 4–5 predictions spread across them (different classes and confidence levels) so all filter combinations can be tested.

### Required test cases

| Test | Input | Expected |
|---|---|---|
| `GetPhoto` happy path | valid ID | returns photo with correct fields and predictions |
| `GetPhoto` not found | unknown ID | returns `ErrNotFound` |
| `ListPhotos` no filter | no params | returns all photos |
| `ListPhotos` class filter | `ClassID = "thrips"` | returns only photos with a thrips prediction |
| `ListPhotos` confidence filter | `MinConfidence = 0.9` | returns only photos with confidence ≥ 0.9 |
| `ListPhotos` combined filter | `ClassID = "thrips", MinConfidence = 0.9` | returns only photos where a **single** prediction is thrips AND ≥ 0.9 |
| `ListPhotos` pagination | `Limit = 1`, then cursor | second page returns remaining photo(s) |

The combined filter test must have at least one photo with a thrips prediction < 0.9 **and** a non-thrips prediction ≥ 0.9, to confirm the subquery correctly rejects it.

---

## Carry-forward to next handler-touching spec

The following issue was caught during the Step 03 review but is out of scope here — apply it in the spec that first adds API route handlers (Step 04 or equivalent), when `internal/handler/handler.go` is already being modified:

- **`WriteJSON` uses bare `any`** — violates the project rule (`any` is banned; use `unknown` or proper generics). Fix: make it generic: `func WriteJSON[T any](w http.ResponseWriter, status int, v T)`.
- **Magic string `"dev"` in health handler** (`cmd/server/main.go` line 35) — extract as `const version = "dev"` or wire via `-ldflags` at build time.

---

## Acceptance criteria

- [ ] `go build ./...` passes with no errors
- [ ] `go test ./internal/db/...` passes — all 7 test cases green
- [ ] Server starts and `GET /health` still returns `{"status":"ok","version":"dev"}` with `DB_PATH` set to `dataset/predictions.db`
- [ ] Opening a non-existent `DB_PATH` causes the server to exit non-zero at startup with an error log line
- [ ] `predictions.db` is never written to — open flags use `mode=ro`
- [ ] The combined filter test proves a single prediction must satisfy both filters (not one prediction per filter)
- [ ] `go vet ./...` passes with no warnings
