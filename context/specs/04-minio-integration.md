# Spec 04 — MinIO Integration

**Plan ref:** Phase 2, Step 4  
**Goal:** Implement the MinIO client package (`internal/minio`) with presigned PUT and GET URL generation, expose a `Presigner` interface for handler-layer mocking, and wire it into `main.go` — so Step 5 can call presigning without knowing MinIO details.

---

## 1. Dependency

Add the official MinIO Go SDK:

```sh
cd backend && go get github.com/minio/minio-go/v7
```

`github.com/minio/minio-go/v7` has been published since 2015 — well past the 1-week rule.

---

## 2. Object key convention

All photos are stored under a single prefix. The key is deterministic from the photo ID:

```
photos/{photoID}.jpg
```

The `.jpg` extension is hardcoded because the dataset is exclusively 2560×1440 JPEGs and `contentType` is always `image/jpeg`. Step 5 (API routes) **must** validate that the `contentType` field in `POST /photos/{photoId}/upload-link` equals `"image/jpeg"` and return `400 BAD_REQUEST` for anything else — otherwise the stored object and the key extension will be mismatched and the thumbnail engine will receive non-JPEG bytes at a `.jpg` key.

Expose the key as an exported function so handler, seed, and thumbnail engine all use the same derivation:

```go
// ObjectKey returns the MinIO object key for a given photo ID.
func ObjectKey(photoID string) string {
    return "photos/" + photoID + ".jpg"
}
```

---

## 3. `Presigner` interface — `internal/minio/minio.go`

Define an interface so the API handler layer (Step 5) depends on an abstraction, not the MinIO SDK directly. This enables mocking in handler tests.

```go
// Presigner generates presigned URLs for object storage operations.
type Presigner interface {
    // PresignedPutURL returns a short-lived presigned PUT URL for uploading a photo's original bytes.
    // The returned headers map must be forwarded verbatim as HTTP headers on the PUT request —
    // this is what causes MinIO to store the object with the correct Content-Type.
    PresignedPutURL(ctx context.Context, photoID string, contentType string, ttl time.Duration) (url string, headers map[string]string, expiresAt time.Time, err error)

    // PresignedGetURL returns a fresh presigned GET URL for reading a photo.
    // TTL is fixed at 1 hour inside the implementation — it is not exposed on the interface
    // because CLAUDE.md mandates 1-hour TTL for all photo GET URLs and callers must not override it.
    // Must be called fresh on every API response — do not cache the result.
    PresignedGetURL(ctx context.Context, photoID string) (string, error)
}
```

> **Design note — TTL not on the GET interface:** The 1-hour TTL for `PresignedGetURL` is an intentional contract defined in CLAUDE.md, not a caller preference. Hiding it from the interface prevents Step 5 handlers from accidentally varying the TTL. Any mock implementation of `Presigner` used in handler tests must return a non-empty URL string — the TTL is not testable at the handler layer and belongs in the MinIO integration tests.

---

## 4. `Client` struct and `New`

```go
// Client implements Presigner using the MinIO SDK.
type Client struct {
    mc     *minio.Client
    bucket string
}

// New creates a MinIO client from explicit config values.
// Returns an error if the client cannot be initialised or the bucket cannot be reached.
// Uses a 5-second timeout for the connectivity check — a slow or misconfigured MinIO
// must not block server startup indefinitely.
func New(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*Client, error)
```

**Behaviour of `New`:**
- Construct `minio.New(endpoint, &minio.Options{...})` with the given credentials
- Verify connectivity via `mc.BucketExists` using `context.WithTimeout(context.Background(), 5*time.Second)` — do not use a bare `context.Background()` as it will block forever if MinIO is unreachable
- If `BucketExists` returns `false` without error, return `fmt.Errorf("bucket %q does not exist", bucket)`
- Do **not** create the bucket — it must already exist (created by `minio-init` in Docker Compose)

---

## 5. `PresignedPutURL`

```go
func (c *Client) PresignedPutURL(
    ctx context.Context,
    photoID string,
    contentType string,
    ttl time.Duration,
) (url string, headers map[string]string, expiresAt time.Time, err error)
```

**Behaviour:**
- Object key: `ObjectKey(photoID)`
- Call `mc.PresignedPutObject(ctx, bucket, key, ttl, nil)` — returns `*url.URL`
- Return the URL string, `map[string]string{"Content-Type": contentType}` as headers, and `time.Now().Add(ttl)` as `expiresAt`

**On the `headers` map:** MinIO's SDK does not sign `Content-Type` into the presigned PUT URL — the stored object gets whatever `Content-Type` the client actually sends on the PUT. The `headers` map is the mechanism by which this spec guarantees the correct content type is stored: the caller (handler or seed script) **must** forward every key-value pair from `headers` as HTTP request headers on the PUT. If a caller omits the headers, MinIO stores the object as `application/octet-stream`, which breaks browser rendering. The integration test in section 9 verifies the full round-trip by asserting `Content-Type: image/jpeg` on a subsequent GET — run both PUT and GET tests together to catch this.

**TTL:** caller-supplied; the API handler (Step 5) will use 15 minutes.

---

## 6. `PresignedGetURL`

```go
func (c *Client) PresignedGetURL(ctx context.Context, photoID string) (string, error)
```

**Behaviour:**
- Object key: `ObjectKey(photoID)`
- TTL: always `time.Hour` — hardcoded, not caller-supplied (see design note in section 3)
- Call `mc.PresignedGetObject(ctx, bucket, key, time.Hour, nil)` — returns `*url.URL`
- Return the URL string
- **Never cache the result** — called fresh on every `GET /photos` and `GET /photos/{photoId}` response

---

## 7. Wire into `main.go`

Initialise the MinIO client after `db.Open`, before the mux is built:

```go
store, err := minioclient.New(
    cfg.MinIOEndpoint,
    cfg.MinIOAccessKey,
    cfg.MinIOSecretKey,
    cfg.MinIOBucket,
    cfg.MinIOUseSSL,
)
if err != nil {
    log.Error("failed to connect to MinIO", "error", err)
    os.Exit(1)
}
```

Store `store` alongside `database` for passing to handlers in Step 5. At this stage, opening the client and verifying bucket existence is sufficient — no handlers use it yet.

> **Package name clash:** the import path `github.com/minio/minio-go/v7` uses package name `minio`. Since `internal/minio` also uses package `minio`, alias one of them at the import site: `minioclient "github.com/polina2410/scout/backend/internal/minio"` (or similar).

---

## 8. Carry-forward items from spec 03

Apply these in this step when touching `internal/handler/handler.go` and `main.go`:

- **`WriteJSON` bare `any`** — change to `func WriteJSON[T any](w http.ResponseWriter, status int, v T)`
- **Magic `"dev"` string** in health handler — extract as `const version = "dev"` at package level in `main.go`

---

## 9. Tests — `internal/minio/minio_test.go`

### `TestObjectKey`

No server required. Pure unit test:

```
ObjectKey("abc-123") == "photos/abc-123.jpg"
```

### Integration tests (require live MinIO)

The PUT and GET integration tests **must run as a single `TestPresignedRoundTrip` test function** — not as separate independent tests. The GET assertion (`Content-Type: image/jpeg`) is the only verification that the PUT headers were forwarded correctly; splitting them into independent tests allows the GET test to be skipped while the PUT test passes, hiding the content-type failure.

```go
func TestPresignedRoundTrip(t *testing.T) {
    c := skipIfNoMinIO(t)
    photoID := "test-" + randomSuffix()

    // PUT
    putURL, headers, expiresAt, err := c.PresignedPutURL(ctx, photoID, "image/jpeg", time.Minute)
    // assert non-empty URL, headers["Content-Type"] == "image/jpeg",
    // expiresAt within 5s of time.Now().Add(time.Minute)
    // PUT minimal JPEG bytes, forwarding all headers — assert HTTP 200

    // GET — must follow the PUT in the same test to verify Content-Type was stored correctly
    getURL, err := c.PresignedGetURL(ctx, photoID)
    // assert non-empty URL
    // GET the URL — assert HTTP 200 and Content-Type header == "image/jpeg"
}
```

### Test helper

```go
func skipIfNoMinIO(t *testing.T) *Client {
    t.Helper()
    endpoint := os.Getenv("MINIO_ENDPOINT")
    if endpoint == "" {
        t.Skip("MINIO_ENDPOINT not set — skipping MinIO integration tests")
    }
    // construct client from MINIO_* env vars
}
```

### `TestNew_BucketNotFound` (integration)

- Call `New` with valid credentials but a non-existent bucket name
- Assert error is non-nil and message contains the bucket name

### `TestNew_Unreachable` (integration or unit with bad endpoint)

- Call `New` with an unreachable endpoint (e.g. `"localhost:19999"`)
- Assert the call returns within ~6 seconds (timeout fires) and error is non-nil

---

## Acceptance criteria

- [ ] `go get github.com/minio/minio-go/v7` succeeds; `go mod tidy` is clean
- [ ] `ObjectKey("abc-123")` returns `"photos/abc-123.jpg"`
- [ ] `New(...)` returns within 6 seconds and errors when the bucket does not exist or MinIO is unreachable
- [ ] `PresignedPutURL` returns a non-empty URL, `{"Content-Type": contentType}` headers, and a future `expiresAt`
- [ ] `TestPresignedRoundTrip` passes: PUT with forwarded headers → GET confirms `Content-Type: image/jpeg`
- [ ] Server starts and logs no errors when `MINIO_*` vars point at the running Docker MinIO
- [ ] `go build ./...` passes
- [ ] `go test ./internal/minio/... -run TestObjectKey` passes without a live MinIO
- [ ] `WriteJSON` carry-forward applied: generic signature `WriteJSON[T any]`
- [ ] `version` carry-forward applied: `const version = "dev"` in `main.go`
- [ ] Step 5 constraint documented: `contentType` must be validated as `"image/jpeg"` at the handler — not in this package
