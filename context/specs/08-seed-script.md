# Spec 08 — Seed Script

**Plan ref:** Phase 5, Step 8  
**Goal:** A re-runnable Go binary at `backend/cmd/seed/main.go` that reads every JPEG from `dataset/images/`, uploads each via the `POST /photos/{photoId}/upload-link` → PUT flow, and silently skips photos already present in MinIO.

---

## 1. Binary location and invocation

The binary lives at `backend/cmd/seed/main.go`. The Makefile already wires it:

```makefile
seed:
    cd backend && go run ./cmd/seed
```

The working directory when invoked via `make seed` is `backend/`, so all default paths below are relative to that.

---

## 2. Configuration

The seed reads env vars inline — it does **not** call `internal/config.Load()` (that requires `DB_PATH` and other server-only vars).

| Env var | Required | Default | Notes |
|---|---|---|---|
| `MINIO_ENDPOINT` | yes | — | e.g. `localhost:9000` |
| `MINIO_ACCESS_KEY` | yes | — | |
| `MINIO_SECRET_KEY` | yes | — | |
| `MINIO_BUCKET` | yes | — | |
| `MINIO_USE_SSL` | no | `false` | case-insensitive `"true"` enables SSL |
| `API_KEY` | yes | — | forwarded as `X-API-Key` on every backend request |
| `API_URL` | no | `http://localhost:8080` | base URL of the running backend |
| `IMAGES_DIR` | no | `../dataset/images` | directory containing `*.jpg` files to seed |

If any required var is empty, print the full list of missing names to stderr and exit 1. Do not start the MinIO client if config is incomplete.

Add `API_URL` and `IMAGES_DIR` to `.env.example` under a new `# ── Seed ──` comment block with their defaults shown.

---

## 3. `ObjectExists` method on `internal/minio.Client`

Add one method for the idempotency check — do not add it to the `Presigner` interface:

```go
// ObjectExists reports whether a photo's object is present in the bucket.
// Returns (false, nil) when the object is absent — not an error.
func (c *Client) ObjectExists(ctx context.Context, photoID string) (bool, error)
```

Implementation: call `c.mc.StatObject(ctx, c.bucket, ObjectKey(photoID), miniogo.StatObjectOptions{})`.

- Success → `(true, nil)`
- Error response code `"NoSuchKey"` → `(false, nil)`
- Any other error → `(false, err)`

Add one test `TestObjectExists` in `internal/minio/minio_test.go` (skipped unless `MINIO_ENDPOINT` is set, consistent with existing tests in that file).

---

## 4. Core upload logic — `uploadPhoto`

Extract the per-photo logic into a testable function so `main` stays thin:

```go
// uploadPhoto checks existence, fetches a presigned PUT URL, and uploads one photo.
// Returns (true, nil) if skipped (already exists), (false, nil) on success, (false, err) on failure.
func uploadPhoto(
    ctx    context.Context,
    store  *minioclient.Client,
    httpCl *http.Client,
    apiURL string,
    apiKey string,
    photoID string,
    imagePath string,
) (skipped bool, err error)
```

Steps inside `uploadPhoto`:

1. Call `store.ObjectExists(ctx, photoID)` — return `(true, nil)` if `true`.
2. POST `{apiURL}/photos/{photoID}/upload-link` with `Content-Type: application/json`, `X-API-Key: {apiKey}`, body `{"contentType":"image/jpeg"}`.
3. Decode the response body into:
   ```go
   type uploadLinkResponse struct {
       URL     string            `json:"url"`
       Headers map[string]string `json:"headers"`
   }
   ```
4. Read the full file at `imagePath` into memory.
5. PUT the bytes to `response.URL` using `httpCl`. Forward every key/value from `response.Headers` as request headers.
6. Return `(false, nil)` on HTTP 200; return `(false, err)` for any non-2xx status or network error.

Use an `http.Client` with a **5-minute timeout** to accommodate originals (~15 MB each).

---

## 5. `main` orchestration

```go
func main() {
    // 1. Load config — exit 1 on missing required vars
    // 2. Create minioclient.New(...) — exit 1 on failure
    // 3. Glob IMAGES_DIR/*.jpg — exit 1 if directory is unreadable
    // 4. For each file:
    //    - photoID = filepath.Base(file) without extension
    //    - call uploadPhoto(...)
    //    - log "skip {photoID}" / "uploaded {photoID}" / "error {photoID}: {err}"
    //    - increment uploaded / skipped / errors counter
    // 5. Print summary: "done: {uploaded} uploaded, {skipped} skipped, {errors} errors"
    // 6. Exit 1 if errors > 0, else exit 0
}
```

The seed does **not** need the SQLite database — it does not call `db.Open`.

---

## 6. Tests — `backend/cmd/seed/seed_test.go`

Two unit tests; no live MinIO or backend required.

| Test | Setup | Assert |
|---|---|---|
| `TestUploadPhoto_Skip` | `store.ObjectExists` returns `true` (stub); httptest server that records calls | `uploadPhoto` returns `(true, nil)`; PUT endpoint is never called |
| `TestUploadPhoto_Upload` | `store.ObjectExists` returns `false`; httptest server returns a valid `uploadLinkResponse` with `url` pointing to a second httptest handler | `uploadPhoto` returns `(false, nil)`; PUT handler received the file bytes and correct headers |

Since `uploadPhoto` takes a `*minioclient.Client` (concrete type, not an interface), use a real client pointed at a mock httptest MinIO-compatible server, or extract the existence check behind a one-method interface for testing:

```go
type existenceChecker interface {
    ObjectExists(ctx context.Context, photoID string) (bool, error)
}
```

Change `uploadPhoto`'s first store parameter to `existenceChecker` so tests can pass a stub without needing a live MinIO connection. The `*minioclient.Client` satisfies this interface.

---

## Acceptance criteria

- [ ] `make seed` runs to completion when MinIO is up, the backend is running, and all required env vars are set
- [ ] Every JPEG in `dataset/images/` has a corresponding object in the `scout` bucket after the first run
- [ ] Running `make seed` a second time logs only `"skip {photoID}"` lines and exits 0 with `"0 uploaded"`
- [ ] Per-photo errors are logged and counted; remaining photos are still attempted
- [ ] Exit code is 1 if any photo failed, 0 if all succeeded or were skipped
- [ ] `API_URL` and `IMAGES_DIR` are present in `.env.example`
- [ ] `internal/minio.Client.ObjectExists` is implemented and passes `go vet ./...`
- [ ] `TestUploadPhoto_Skip` and `TestUploadPhoto_Upload` pass without any live services
- [ ] `go build ./...` passes
