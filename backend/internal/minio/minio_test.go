package minio

import (
	"bytes"
	"context"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// unreachableEndpoint uses a port chosen to be unoccupied in typical dev/CI environments.
const unreachableEndpoint = "localhost:19999"

// newTimeoutBound is 2× the internal bucket-check timeout — the call must return before this.
const newTimeoutBound = 2 * bucketCheckTimeout

func TestObjectKey(t *testing.T) {
	cases := []struct {
		photoID string
		want    string
	}{
		{"abc-123", "photos/abc-123.jpg"},
		{"550e8400-e29b-41d4-a716-446655440000", "photos/550e8400-e29b-41d4-a716-446655440000.jpg"},
		{"x", "photos/x.jpg"},
	}
	for _, tc := range cases {
		got := ObjectKey(tc.photoID)
		if got != tc.want {
			t.Errorf("ObjectKey(%q) = %q, want %q", tc.photoID, got, tc.want)
		}
	}
}

// skipIfNoMinIO skips the test if MINIO_ENDPOINT is not set and returns a live Client.
func skipIfNoMinIO(t *testing.T) *Client {
	t.Helper()
	endpoint := os.Getenv("MINIO_ENDPOINT")
	if endpoint == "" {
		t.Skip("MINIO_ENDPOINT not set — skipping MinIO integration tests")
	}
	accessKey := os.Getenv("MINIO_ACCESS_KEY")
	secretKey := os.Getenv("MINIO_SECRET_KEY")
	bucket := os.Getenv("MINIO_BUCKET")
	if accessKey == "" || secretKey == "" || bucket == "" {
		t.Skip("MINIO_ACCESS_KEY, MINIO_SECRET_KEY, MINIO_BUCKET must all be set")
	}
	useSSL := strings.EqualFold(os.Getenv("MINIO_USE_SSL"), "true")
	c, err := New(endpoint, accessKey, secretKey, bucket, useSSL)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func randomSuffix() string {
	return fmt.Sprintf("%08x", rand.Uint32())
}

// TestPresignedRoundTrip runs PUT then GET as a single test so the GET Content-Type
// assertion proves the PUT headers were forwarded correctly. Splitting the tests would
// allow the GET to be skipped while PUT passes, hiding the content-type failure.
func TestPresignedRoundTrip(t *testing.T) {
	c := skipIfNoMinIO(t)
	ctx := context.Background()
	photoID := "test-" + randomSuffix()
	putTTL := time.Minute

	// --- PUT ---
	const contentType = "image/jpeg"
	putURL, headers, expiresAt, err := c.PresignedPutURL(ctx, photoID, contentType, putTTL)
	if err != nil {
		t.Fatalf("PresignedPutURL: %v", err)
	}
	if putURL == "" {
		t.Fatal("PresignedPutURL returned empty URL")
	}
	if headers["Content-Type"] != contentType {
		t.Errorf("headers[Content-Type] = %q, want %q", headers["Content-Type"], contentType)
	}
	// Allow ±5s clock skew around the requested TTL.
	lo, hi := putTTL-5*time.Second, putTTL+5*time.Second
	if d := time.Until(expiresAt); d < lo || d > hi {
		t.Errorf("expiresAt duration %v not in [%v, %v]", d, lo, hi)
	}

	// Minimal valid JPEG bytes (a 1×1 pixel JPEG).
	minimalJPEG := []byte{
		0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46, 0x00, 0x01,
		0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0xff, 0xdb, 0x00, 0x43,
		0x00, 0x08, 0x06, 0x06, 0x07, 0x06, 0x05, 0x08, 0x07, 0x07, 0x07, 0x09,
		0x09, 0x08, 0x0a, 0x0c, 0x14, 0x0d, 0x0c, 0x0b, 0x0b, 0x0c, 0x19, 0x12,
		0x13, 0x0f, 0x14, 0x1d, 0x1a, 0x1f, 0x1e, 0x1d, 0x1a, 0x1c, 0x1c, 0x20,
		0x24, 0x2e, 0x27, 0x20, 0x22, 0x2c, 0x23, 0x1c, 0x1c, 0x28, 0x37, 0x29,
		0x2c, 0x30, 0x31, 0x34, 0x34, 0x34, 0x1f, 0x27, 0x39, 0x3d, 0x38, 0x32,
		0x3c, 0x2e, 0x33, 0x34, 0x32, 0xff, 0xc0, 0x00, 0x0b, 0x08, 0x00, 0x01,
		0x00, 0x01, 0x01, 0x01, 0x11, 0x00, 0xff, 0xc4, 0x00, 0x1f, 0x00, 0x00,
		0x01, 0x05, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0a, 0x0b, 0xff, 0xc4, 0x00, 0xb5, 0x10, 0x00, 0x02, 0x01, 0x03,
		0x03, 0x02, 0x04, 0x03, 0x05, 0x05, 0x04, 0x04, 0x00, 0x00, 0x01, 0x7d,
		0x01, 0x02, 0x03, 0x00, 0x04, 0x11, 0x05, 0x12, 0x21, 0x31, 0x41, 0x06,
		0x13, 0x51, 0x61, 0x07, 0x22, 0x71, 0x14, 0x32, 0x81, 0x91, 0xa1, 0x08,
		0x23, 0x42, 0xb1, 0xc1, 0x15, 0x52, 0xd1, 0xf0, 0x24, 0x33, 0x62, 0x72,
		0x82, 0x09, 0x0a, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x25, 0x26, 0x27, 0x28,
		0x29, 0x2a, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39, 0x3a, 0x43, 0x44, 0x45,
		0x46, 0x47, 0x48, 0x49, 0x4a, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59,
		0x5a, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69, 0x6a, 0x73, 0x74, 0x75,
		0x76, 0x77, 0x78, 0x79, 0x7a, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89,
		0x8a, 0x93, 0x94, 0x95, 0x96, 0x97, 0x98, 0x99, 0x9a, 0xa2, 0xa3, 0xa4,
		0xa5, 0xa6, 0xa7, 0xa8, 0xa9, 0xaa, 0xb2, 0xb3, 0xb4, 0xb5, 0xb6, 0xb7,
		0xb8, 0xb9, 0xba, 0xc2, 0xc3, 0xc4, 0xc5, 0xc6, 0xc7, 0xc8, 0xc9, 0xca,
		0xd2, 0xd3, 0xd4, 0xd5, 0xd6, 0xd7, 0xd8, 0xd9, 0xda, 0xe1, 0xe2, 0xe3,
		0xe4, 0xe5, 0xe6, 0xe7, 0xe8, 0xe9, 0xea, 0xf1, 0xf2, 0xf3, 0xf4, 0xf5,
		0xf6, 0xf7, 0xf8, 0xf9, 0xfa, 0xff, 0xda, 0x00, 0x08, 0x01, 0x01, 0x00,
		0x00, 0x3f, 0x00, 0xfb, 0xd5, 0xff, 0xd9,
	}

	putReq, err := http.NewRequestWithContext(ctx, http.MethodPut, putURL, bytes.NewReader(minimalJPEG))
	if err != nil {
		t.Fatalf("build PUT request: %v", err)
	}
	for k, v := range headers {
		putReq.Header.Set(k, v)
	}
	putResp, err := http.DefaultClient.Do(putReq)
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	putResp.Body.Close()
	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("PUT status = %d, want 200", putResp.StatusCode)
	}

	// --- GET — must follow PUT in the same test to prove Content-Type was stored correctly ---
	getURL, err := c.PresignedGetURL(ctx, photoID)
	if err != nil {
		t.Fatalf("PresignedGetURL: %v", err)
	}
	if getURL == "" {
		t.Fatal("PresignedGetURL returned empty URL")
	}

	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, getURL, nil)
	if err != nil {
		t.Fatalf("build GET request: %v", err)
	}
	getResp, err := http.DefaultClient.Do(getReq)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d, want 200", getResp.StatusCode)
	}
	if ct := getResp.Header.Get("Content-Type"); ct != contentType {
		t.Errorf("GET Content-Type = %q, want %q", ct, contentType)
	}
}

func TestNew_BucketNotFound(t *testing.T) {
	endpoint := os.Getenv("MINIO_ENDPOINT")
	if endpoint == "" {
		t.Skip("MINIO_ENDPOINT not set")
	}
	accessKey := os.Getenv("MINIO_ACCESS_KEY")
	secretKey := os.Getenv("MINIO_SECRET_KEY")
	if accessKey == "" || secretKey == "" {
		t.Skip("MINIO_ACCESS_KEY and MINIO_SECRET_KEY must be set")
	}
	useSSL := strings.EqualFold(os.Getenv("MINIO_USE_SSL"), "true")

	_, err := New(endpoint, accessKey, secretKey, "bucket-does-not-exist-xyz", useSSL)
	if err == nil {
		t.Fatal("expected error for non-existent bucket, got nil")
	}
	t.Logf("got expected error: %v", err)
}

func TestNew_Unreachable(t *testing.T) {
	start := time.Now()
	_, err := New(unreachableEndpoint, "access", "secret", "scout", false)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error for unreachable endpoint, got nil")
	}
	if elapsed > newTimeoutBound {
		t.Errorf("New took %v, want < %v (timeout should fire at %v)", elapsed, newTimeoutBound, bucketCheckTimeout)
	}
	t.Logf("returned in %v with: %v", elapsed, err)
}

func TestPresignedPutURL_TTLTooLarge(t *testing.T) {
	c := skipIfNoMinIO(t)
	ctx := context.Background()
	_, _, _, err := c.PresignedPutURL(ctx, "photo-1", "image/jpeg", maxPutTTL+time.Second)
	if err == nil {
		t.Fatal("expected error for TTL exceeding maxPutTTL, got nil")
	}
}
