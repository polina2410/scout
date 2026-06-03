package minio

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	miniogo "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// ErrObjectNotFound is returned by GetOriginal when the object does not exist in the bucket.
var ErrObjectNotFound = errors.New("object not found in bucket")

const (
	getURLTTL  = time.Hour
	maxPutTTL  = time.Hour // MinIO's default max presigned URL lifetime is 7 days; we cap lower
	bucketCheckTimeout = 5 * time.Second
)

// ObjectKey returns the MinIO object key for a given photo ID.
// photoID must be a bare UUID or alphanumeric-plus-hyphen string — no slashes, dots,
// or path metacharacters. Pre-validate at the handler layer (Step 5) before calling.
// The .jpg extension is fixed: the dataset is exclusively JPEG and contentType
// must be validated as "image/jpeg" at the handler layer before calling this.
func ObjectKey(photoID string) string {
	return "photos/" + photoID + ".jpg"
}

// Presigner generates presigned URLs for object storage operations.
type Presigner interface {
	// PresignedPutURL returns a short-lived presigned PUT URL for uploading a photo's original bytes.
	// ttl must not exceed maxPutTTL (1 hour) — an error is returned if it does.
	// The returned headers map must be forwarded verbatim as HTTP headers on the PUT request —
	// this is what causes MinIO to store the object with the correct Content-Type.
	PresignedPutURL(ctx context.Context, photoID string, contentType string, ttl time.Duration) (url string, headers map[string]string, expiresAt time.Time, err error)

	// PresignedGetURL returns a fresh presigned GET URL for reading a photo.
	// TTL is fixed at 1 hour inside the implementation and is not exposed on the interface —
	// CLAUDE.md mandates 1-hour TTL for all photo GET URLs and callers must not override it.
	// Must be called fresh on every API response — do not cache the result.
	PresignedGetURL(ctx context.Context, photoID string) (string, error)
}

// Client implements Presigner using the MinIO SDK.
// It has no Close method — the underlying connection pool is managed by the SDK.
type Client struct {
	mc     *miniogo.Client
	bucket string
}

// New creates a MinIO client and verifies the bucket exists.
// Uses a 5-second timeout for the connectivity check so a misconfigured
// or unreachable MinIO does not block server startup indefinitely.
func New(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*Client, error) {
	mc, err := miniogo.New(endpoint, &miniogo.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), bucketCheckTimeout)
	defer cancel()

	exists, err := mc.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("check bucket %q: %w", bucket, err)
	}
	if !exists {
		return nil, fmt.Errorf("bucket %q does not exist", bucket)
	}

	return &Client{mc: mc, bucket: bucket}, nil
}

// PresignedPutURL returns a presigned PUT URL and a headers map the caller must
// forward on the PUT request. MinIO does not sign Content-Type into the URL, so
// the stored object's content type equals whatever Content-Type header the client
// sends — omitting it results in application/octet-stream.
// ttl must not exceed maxPutTTL; the returned expiresAt reflects the actual ttl used.
func (c *Client) PresignedPutURL(ctx context.Context, photoID, contentType string, ttl time.Duration) (string, map[string]string, time.Time, error) {
	if ttl <= 0 || ttl > maxPutTTL {
		return "", nil, time.Time{}, fmt.Errorf("ttl %v out of range (0, %v]", ttl, maxPutTTL)
	}
	key := ObjectKey(photoID)
	u, err := c.mc.PresignedPutObject(ctx, c.bucket, key, ttl)
	if err != nil {
		return "", nil, time.Time{}, fmt.Errorf("presign PUT for %q: %w", photoID, err)
	}
	headers := map[string]string{"Content-Type": contentType}
	return u.String(), headers, time.Now().Add(ttl), nil
}

// GetOriginal streams the original JPEG bytes for the given photo.
// Returns ErrObjectNotFound (wrapped) if the object does not exist in the bucket.
// Caller must close the returned ReadCloser.
func (c *Client) GetOriginal(ctx context.Context, photoID string) (io.ReadCloser, error) {
	obj, err := c.mc.GetObject(ctx, c.bucket, ObjectKey(photoID), miniogo.GetObjectOptions{})
	if err != nil {
		resp := miniogo.ToErrorResponse(err)
		if resp.Code == "NoSuchKey" {
			return nil, ErrObjectNotFound
		}
		return nil, fmt.Errorf("get object %q: %w", photoID, err)
	}
	// Trigger the actual HTTP request by calling Stat; GetObject is lazy.
	if _, err := obj.Stat(); err != nil {
		obj.Close()
		resp := miniogo.ToErrorResponse(err)
		if resp.Code == "NoSuchKey" {
			return nil, ErrObjectNotFound
		}
		return nil, fmt.Errorf("stat object %q: %w", photoID, err)
	}
	return obj, nil
}

// PresignedGetURL returns a fresh 1-hour presigned GET URL for the given photo.
// reqParams are nil — no Content-Disposition or other query param overrides needed.
func (c *Client) PresignedGetURL(ctx context.Context, photoID string) (string, error) {
	key := ObjectKey(photoID)
	u, err := c.mc.PresignedGetObject(ctx, c.bucket, key, getURLTTL, nil)
	if err != nil {
		return "", fmt.Errorf("presign GET for %q: %w", photoID, err)
	}
	return u.String(), nil
}
