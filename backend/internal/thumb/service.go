package thumb

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log/slog"
	"math"
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

// genBucketBoundsMs are the upper bounds (inclusive, ms) for generation time buckets.
// Higher bounds than the request latency histogram because generation involves
// a MinIO fetch + JPEG decode + resize + encode — typically 100ms–2000ms.
var genBucketBoundsMs = [...]int64{50, 100, 250, 500, 750, 1000, 2000, 3000, 5000}

const numGenBuckets = len(genBucketBoundsMs) + 1

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
	GenTotalNs  int64
	GenP95Ms    float64
}

// Service handles thumbnail generation, caching, and concurrency control.
type Service struct {
	store Downloader
	cache *DiskCache
	sem   chan struct{} // capacity maxConcurrent
	log   *slog.Logger

	hits       atomic.Int64
	misses     atomic.Int64
	genOK      atomic.Int64
	genErr     atomic.Int64
	genNs      atomic.Int64
	genBuckets [numGenBuckets]atomic.Int64

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
	genOK := s.genOK.Load()
	var buckets [numGenBuckets]int64
	for i := range buckets {
		buckets[i] = s.genBuckets[i].Load()
	}
	return ThumbMetrics{
		CacheHits:   s.hits.Load(),
		CacheMisses: s.misses.Load(),
		GenOK:       genOK,
		GenErr:      s.genErr.Load(),
		GenTotalNs:  s.genNs.Load(),
		GenP95Ms:    genPercentileMs(buckets, genOK, 0.95),
	}
}

func genPercentileMs(buckets [numGenBuckets]int64, total int64, p float64) float64 {
	if total == 0 {
		return 0
	}
	target := int64(math.Ceil(p * float64(total)))
	var cum int64
	for i, count := range buckets {
		cum += count
		if cum >= target {
			if i < len(genBucketBoundsMs) {
				return float64(genBucketBoundsMs[i])
			}
			return float64(genBucketBoundsMs[len(genBucketBoundsMs)-1])
		}
	}
	return 0
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
		// Use context.Background() so the generation is not cancelled when the
		// originating HTTP connection closes. Every waiter sharing this singleflight
		// key benefits from the result; tying it to one caller's context would
		// poison all of them if that caller disconnects mid-generation.
		return s.generate(context.Background(), p)
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

	data, encErr := encodeImage(dst, p.Fmt)
	if encErr != nil {
		s.genErr.Add(1)
		return nil, fmt.Errorf("encode %s: %w", p.Fmt, encErr)
	}

	elapsed := time.Since(t0)
	durationMs := elapsed.Milliseconds()
	s.genNs.Add(elapsed.Nanoseconds())

	bucketIdx := numGenBuckets - 1
	for i, bound := range genBucketBoundsMs {
		if durationMs <= bound {
			bucketIdx = i
			break
		}
	}
	s.genBuckets[bucketIdx].Add(1)
	s.genOK.Add(1)
	return data, nil
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
