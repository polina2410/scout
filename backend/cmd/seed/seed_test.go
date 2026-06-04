package main

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

type stubChecker struct {
	exists bool
	err    error
}

func (s *stubChecker) ObjectExists(_ context.Context, _ string) (bool, error) {
	return s.exists, s.err
}

func writeTestJPEG(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.White)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode test JPEG: %v", err)
	}
	path := filepath.Join(dir, name+".jpg")
	if err := os.WriteFile(path, buf.Bytes(), 0600); err != nil {
		t.Fatalf("write test JPEG: %v", err)
	}
	return path
}

func TestUploadPhoto_Skip(t *testing.T) {
	var putCalled atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		putCalled.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	imagePath := writeTestJPEG(t, "test-photo-id")

	skipped, err := uploadPhoto(
		context.Background(),
		&stubChecker{exists: true},
		srv.Client(),
		srv.URL,
		"test-key",
		"test-photo-id",
		imagePath,
	)
	if err != nil {
		t.Fatalf("uploadPhoto: %v", err)
	}
	if !skipped {
		t.Error("expected skipped=true, got false")
	}
	if putCalled.Load() {
		t.Error("HTTP endpoint was called but should not have been (object exists)")
	}
}

func TestUploadPhoto_Upload(t *testing.T) {
	imagePath := writeTestJPEG(t, "test-photo-id")
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		t.Fatalf("read test image: %v", err)
	}

	var putBody []byte
	var putContentType string

	putSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		putBody, _ = io.ReadAll(r.Body)
		putContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer putSrv.Close()

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		link := uploadLinkResponse{
			URL:     putSrv.URL + "/object",
			Headers: map[string]string{"Content-Type": "image/jpeg"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(link) //nolint:errcheck
	}))
	defer apiSrv.Close()

	skipped, err := uploadPhoto(
		context.Background(),
		&stubChecker{exists: false},
		apiSrv.Client(),
		apiSrv.URL,
		"test-key",
		"test-photo-id",
		imagePath,
	)
	if err != nil {
		t.Fatalf("uploadPhoto: %v", err)
	}
	if skipped {
		t.Error("expected skipped=false, got true")
	}
	if !bytes.Equal(putBody, imageData) {
		t.Errorf("PUT body: got %d bytes, want %d bytes", len(putBody), len(imageData))
	}
	if putContentType != "image/jpeg" {
		t.Errorf("PUT Content-Type = %q, want image/jpeg", putContentType)
	}
}
