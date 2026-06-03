package thumb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	minioclient "github.com/polina2410/scout/backend/internal/minio"
)

const testCacheSizeBytes = 10 * 1024 * 1024

// ---- mock Downloader ----

type mockDownloader struct {
	data  []byte
	err   error
	calls atomic.Int64
}

func (m *mockDownloader) GetOriginal(_ context.Context, _ string) (io.ReadCloser, error) {
	m.calls.Add(1)
	if m.err != nil {
		return nil, m.err
	}
	return io.NopCloser(bytes.NewReader(m.data)), nil
}

func minimalJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.White)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode test JPEG: %v", err)
	}
	return buf.Bytes()
}

// ---- test server ----

func newTestService(t *testing.T, dl *mockDownloader) (*Service, *DiskCache) {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cache, err := NewDiskCache(t.TempDir(), testCacheSizeBytes)
	if err != nil {
		t.Fatalf("NewDiskCache: %v", err)
	}
	svc := New(dl, cache, log)
	return svc, cache
}

func doGet(t *testing.T, svc *Service, path string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	// Use a mux so r.PathValue works.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /thumbnails/{photoId}", svc.Handle)
	mux.ServeHTTP(rec, req)
	return rec.Result()
}

const thumbURL = "/thumbnails/" + validUUID + "?w=200&dpr=1&fmt=webp"

func TestHandle_CacheMiss_WebP(t *testing.T) {
	dl := &mockDownloader{data: minimalJPEG(t)}
	svc, _ := newTestService(t, dl)

	resp := doGet(t, svc, thumbURL)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	// WebP falls back to JPEG when CGO is unavailable.
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "image/") {
		t.Errorf("Content-Type: got %q, want image/*", ct)
	}
	if resp.Header.Get("X-Cache") != "MISS" {
		t.Errorf("X-Cache: got %q, want MISS", resp.Header.Get("X-Cache"))
	}
}

func TestHandle_CacheMiss_JPEG(t *testing.T) {
	dl := &mockDownloader{data: minimalJPEG(t)}
	svc, _ := newTestService(t, dl)

	resp := doGet(t, svc, "/thumbnails/"+validUUID+"?w=200&fmt=jpeg")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/jpeg" {
		t.Errorf("Content-Type: got %q, want image/jpeg", ct)
	}
}

func TestHandle_CacheHit(t *testing.T) {
	dl := &mockDownloader{data: minimalJPEG(t)}
	svc, cache := newTestService(t, dl)

	cacheKey := fmt.Sprintf("%s_%d_%d_%s", validUUID, 200, 1, effectiveFormat("webp"))
	fakeData := []byte("fake-image-bytes")
	if err := cache.Put(cacheKey, fakeData); err != nil {
		t.Fatalf("cache.Put: %v", err)
	}

	resp := doGet(t, svc, thumbURL)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	if resp.Header.Get("X-Cache") != "HIT" {
		t.Errorf("X-Cache: got %q, want HIT", resp.Header.Get("X-Cache"))
	}
	if dl.calls.Load() != 0 {
		t.Errorf("Downloader called %d times, want 0 (should use cache)", dl.calls.Load())
	}
}

func TestHandle_NotFound(t *testing.T) {
	dl := &mockDownloader{err: minioclient.ErrObjectNotFound}
	svc, _ := newTestService(t, dl)

	resp := doGet(t, svc, thumbURL)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404", resp.StatusCode)
	}
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	if body["code"] != "NotFound" {
		t.Errorf("code: got %v, want NotFound", body["code"])
	}
}

func TestHandle_AtCapacity(t *testing.T) {
	dl := &mockDownloader{data: minimalJPEG(t)}
	svc, _ := newTestService(t, dl)

	// Pre-fill the semaphore so no slot is available.
	for i := 0; i < maxConcurrent; i++ {
		svc.sem <- struct{}{}
	}
	t.Cleanup(func() {
		for i := 0; i < maxConcurrent; i++ {
			<-svc.sem
		}
	})

	resp := doGet(t, svc, thumbURL)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d, want 503", resp.StatusCode)
	}
	if ra := resp.Header.Get("Retry-After"); ra == "" {
		t.Error("Retry-After header should be present")
	}
}

func TestHandle_MissingW(t *testing.T) {
	svc, _ := newTestService(t, &mockDownloader{})

	resp := doGet(t, svc, "/thumbnails/"+validUUID)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", resp.StatusCode)
	}
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	if body["code"] != "ValidationError" {
		t.Errorf("code: got %v, want ValidationError", body["code"])
	}
}

func TestHandle_MetricsCounted(t *testing.T) {
	dl := &mockDownloader{data: minimalJPEG(t)}
	svc, cache := newTestService(t, dl)

	// First request: cache miss.
	doGet(t, svc, thumbURL)

	// Pre-seed cache for second request: cache hit.
	cacheKey := fmt.Sprintf("%s_%d_%d_%s", validUUID, 400, 1, effectiveFormat("webp"))
	cache.Put(cacheKey, []byte("cached")) //nolint:errcheck
	doGet(t, svc, "/thumbnails/"+validUUID+"?w=400&fmt=webp")

	m := svc.Metrics()
	if m.CacheHits != 1 {
		t.Errorf("CacheHits: got %d, want 1", m.CacheHits)
	}
	if m.CacheMisses != 1 {
		t.Errorf("CacheMisses: got %d, want 1", m.CacheMisses)
	}
	if m.GenP95Ms <= 0 {
		t.Errorf("GenP95Ms: got %v, want > 0 after one successful generation", m.GenP95Ms)
	}
}

func TestHandle_SecondRequestUsesCache(t *testing.T) {
	dl := &mockDownloader{data: minimalJPEG(t)}
	svc, _ := newTestService(t, dl)

	// First request: generates and caches.
	resp1 := doGet(t, svc, thumbURL)
	if resp1.Header.Get("X-Cache") != "MISS" {
		t.Errorf("first request X-Cache: got %q, want MISS", resp1.Header.Get("X-Cache"))
	}

	// Second identical request: must come from cache.
	resp2 := doGet(t, svc, thumbURL)
	if resp2.Header.Get("X-Cache") != "HIT" {
		t.Errorf("second request X-Cache: got %q, want HIT", resp2.Header.Get("X-Cache"))
	}
	if dl.calls.Load() != 1 {
		t.Errorf("Downloader calls: got %d, want 1 (second request should use cache)", dl.calls.Load())
	}
}
