package thumb

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	minioclient "github.com/polina2410/scout/backend/internal/minio"
	"golang.org/x/image/draw"
	"golang.org/x/sync/singleflight"

	"github.com/polina2410/scout/backend/internal/handler"
)

const (
	maxConcurrent = 4  // semaphore capacity
	retryAfterSec = 5  // Retry-After value (seconds) when semaphore is full
	jpegQuality   = 85 // JPEG encode quality [0,100]
)

var errAtCapacity = errors.New("thumbnail generation at capacity")

// Downloader fetches original photo bytes from object storage.
// Defined here so the thumb package owns the interface it needs;
// *minio.Client satisfies it without importing this package.
type Downloader interface {
	GetOriginal(ctx context.Context, photoID string) (io.ReadCloser, error)
}

// ThumbMetrics is a point-in-time snapshot of Service counters.
type ThumbMetrics struct {
	CacheHits   int64
	CacheMisses int64
	GenOK       int64
	GenErr      int64
	GenTotalNs  int64 // divide by GenOK for mean generation time
}

// Service handles thumbnail generation, caching, and concurrency control.
type Service struct {
	store Downloader
	cache *DiskCache
	sem   chan struct{} // capacity maxConcurrent
	log   *slog.Logger

	hits   atomic.Int64
	misses atomic.Int64
	genOK  atomic.Int64
	genErr atomic.Int64
	genNs  atomic.Int64

	group singleflight.Group
}

// New creates a thumbnail Service.
func New(store Downloader, cache *DiskCache, log *slog.Logger) *Service {
	return &Service{
		store: store,
		cache: cache,
		sem:   make(chan struct{}, maxConcurrent),
		log:   log,
	}
}

// Metrics returns a point-in-time snapshot of service counters.
func (s *Service) Metrics() ThumbMetrics {
	return ThumbMetrics{
		CacheHits:   s.hits.Load(),
		CacheMisses: s.misses.Load(),
		GenOK:       s.genOK.Load(),
		GenErr:      s.genErr.Load(),
		GenTotalNs:  s.genNs.Load(),
	}
}

// Handle is the HTTP handler for GET /thumbnails/{photoId}.
func (s *Service) Handle(w http.ResponseWriter, r *http.Request) {
	p, details := ParseParams(r.PathValue("photoId"), r.URL.Query())
	if len(details) > 0 {
		handler.WriteValidationError(w, r, details)
		return
	}

	// Use the effective format (the one actually encoded) in the cache key so that
	// a future CGO-enabled build producing real WebP doesn't serve JPEG under a webp key.
	effectiveFmt := effectiveFormat(p.Fmt)
	key := fmt.Sprintf("%s_%d_%d_%s", p.PhotoID, p.W, p.DPR, effectiveFmt)

	if data, ok := s.cache.Get(key); ok {
		s.hits.Add(1)
		serveImage(w, data, effectiveFmt, "HIT")
		return
	}
	s.misses.Add(1)

	val, err, _ := s.group.Do(key, func() (any, error) {
		// Non-blocking select: reject immediately when all slots are busy.
		// Callers sharing the same singleflight key all get the same 503 — by design.
		select {
		case s.sem <- struct{}{}:
			defer func() { <-s.sem }()
		default:
			return nil, errAtCapacity
		}
		return s.generate(r.Context(), p)
	})

	if err != nil {
		if errors.Is(err, errAtCapacity) {
			w.Header().Set("Retry-After", strconv.Itoa(retryAfterSec))
			handler.WriteError(w, r, http.StatusServiceUnavailable, handler.ErrCodeServiceUnavailable, "thumbnail generation at capacity, retry shortly")
			return
		}
		if errors.Is(err, minioclient.ErrObjectNotFound) {
			handler.WriteNotFoundError(w, r, p.PhotoID)
			return
		}
		s.log.Error("thumbnail generation failed", "photo_id", p.PhotoID, "error", err)
		handler.WriteError(w, r, http.StatusInternalServerError, handler.ErrCodeInternal, "failed to generate thumbnail")
		return
	}

	data := val.([]byte)
	if putErr := s.cache.Put(key, data); putErr != nil {
		s.log.Warn("thumbnail cache write failed", "key", key, "error", putErr)
	}
	serveImage(w, data, effectiveFmt, "MISS")
}

func (s *Service) generate(ctx context.Context, p Params) ([]byte, error) {
	t0 := time.Now()

	rc, err := s.store.GetOriginal(ctx, p.PhotoID)
	if err != nil {
		s.genErr.Add(1)
		return nil, err
	}
	defer rc.Close()

	src, err := jpeg.Decode(rc)
	if err != nil {
		s.genErr.Add(1)
		return nil, fmt.Errorf("decode JPEG: %w", err)
	}

	dstW := p.PxWidth
	dstH := src.Bounds().Dy() * dstW / src.Bounds().Dx()
	dst := image.NewNRGBA(image.Rect(0, 0, dstW, dstH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)

	var buf bytes.Buffer
	// WebP encoding requires CGO (github.com/chai2010/webp). In environments without
	// a C compiler, both fmt=webp and fmt=jpeg are served as JPEG — a valid fallback
	// per the CLAUDE.md spec ("webp preferred | jpeg fallback"). Enable by adding
	// the import and swapping the webp branch when CGO is available (e.g. in Docker).
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: jpegQuality}); err != nil {
		s.genErr.Add(1)
		return nil, fmt.Errorf("encode jpeg: %w", err)
	}

	s.genNs.Add(time.Since(t0).Nanoseconds())
	s.genOK.Add(1)
	return buf.Bytes(), nil
}

// effectiveFormat returns the format that will actually be encoded.
// WebP encoding requires CGO (github.com/chai2010/webp); without it both
// "webp" and "jpeg" requests produce JPEG. When CGO is available, replace
// this function to return imgFmt unchanged.
func effectiveFormat(imgFmt string) string {
	_ = imgFmt
	return "jpeg"
}

func mimeType(imgFmt string) string {
	if imgFmt == "webp" {
		return "image/webp"
	}
	return "image/jpeg"
}

func serveImage(w http.ResponseWriter, data []byte, imgFmt, cacheStatus string) {
	w.Header().Set("Content-Type", mimeType(imgFmt))
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("X-Cache", cacheStatus)
	w.WriteHeader(http.StatusOK)
	w.Write(data) //nolint:errcheck
}
