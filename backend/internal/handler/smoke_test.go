package handler_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/polina2410/scout/backend/internal/db"
	"github.com/polina2410/scout/backend/internal/handler"
	"github.com/polina2410/scout/backend/internal/middleware"
	minioclient "github.com/polina2410/scout/backend/internal/minio"
)

// TestSmoke_IngestAndRead uploads a photo via the API and reads it back.
// Skipped when MINIO_ENDPOINT is not set.
func TestSmoke_IngestAndRead(t *testing.T) {
	endpoint := os.Getenv("MINIO_ENDPOINT")
	if endpoint == "" {
		t.Skip("MINIO_ENDPOINT not set — skipping smoke test")
	}

	accessKey := os.Getenv("MINIO_ACCESS_KEY")
	secretKey := os.Getenv("MINIO_SECRET_KEY")
	bucket := os.Getenv("MINIO_BUCKET")
	if accessKey == "" || secretKey == "" || bucket == "" {
		t.Skip("MINIO_ACCESS_KEY, MINIO_SECRET_KEY, or MINIO_BUCKET not set")
	}

	store, err := minioclient.New(endpoint, accessKey, secretKey, bucket, false)
	if err != nil {
		t.Fatalf("connect to minio: %v", err)
	}

	// Create an in-memory DB with one UUID photo.
	const smokePhotoID = "ffffffff-ffff-ffff-ffff-ffffffffffff"
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	if _, err := sqlDB.Exec(testSchema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	if _, err := sqlDB.Exec(
		`INSERT INTO photos VALUES (?, 5.0, 5.0, 2.0, 2560, 1440, '2026-06-01T12:00:00Z')`,
		smokePhotoID,
	); err != nil {
		t.Fatalf("seed smoke photo: %v", err)
	}
	if _, err := sqlDB.Exec(
		`INSERT INTO predictions VALUES ('smoke-pred', ?, 'thrips', 0.95, 0.1, 0.1, 0.3, 0.3)`,
		smokePhotoID,
	); err != nil {
		t.Fatalf("seed smoke prediction: %v", err)
	}
	database := db.NewDB(sqlDB)
	t.Cleanup(func() { database.Close() })

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	app := &handler.App{DB: database, Store: store, Log: log}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		handler.WriteJSON(w, http.StatusOK, handler.HealthResponse{Status: "ok", Version: "smoke"})
	})
	mux.HandleFunc("POST /photos/{photoId}/upload-link", app.CreateUploadLink)
	mux.HandleFunc("GET /photos/{photoId}", app.GetPhoto)
	mux.HandleFunc("GET /photos", app.ListPhotos)

	const smokeKey = "smoke-api-key"
	var h http.Handler = mux
	h = middleware.APIKeyAuth(smokeKey)(h)
	h = middleware.CorrelationID(log)(h)

	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	client := &http.Client{}
	authHeader := func(req *http.Request) *http.Request {
		req.Header.Set("X-API-Key", smokeKey)
		return req
	}

	// Step 1: POST /photos/{id}/upload-link
	uploadBody := `{"contentType":"image/jpeg"}`
	req1, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/photos/%s/upload-link", srv.URL, smokePhotoID),
		bytes.NewBufferString(uploadBody))
	req1.Header.Set("Content-Type", "application/json")
	resp1, err := client.Do(authHeader(req1))
	if err != nil {
		t.Fatalf("POST upload-link: %v", err)
	}
	if resp1.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp1.Body)
		t.Fatalf("POST upload-link status %d: %s", resp1.StatusCode, body)
	}

	var uploadResp struct {
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
	}
	if err := json.NewDecoder(resp1.Body).Decode(&uploadResp); err != nil {
		t.Fatalf("decode upload-link response: %v", err)
	}
	resp1.Body.Close()

	// Step 2: PUT the minimal JPEG to the presigned URL.
	// A minimal valid JPEG: SOI + EOI markers.
	minimalJPEG := []byte{0xFF, 0xD8, 0xFF, 0xD9}
	putReq, _ := http.NewRequest(http.MethodPut, uploadResp.URL, bytes.NewReader(minimalJPEG))
	for k, v := range uploadResp.Headers {
		putReq.Header.Set(k, v)
	}
	putResp, err := client.Do(putReq)
	if err != nil {
		t.Fatalf("PUT to presigned URL: %v", err)
	}
	putResp.Body.Close()
	if putResp.StatusCode/100 != 2 {
		t.Fatalf("PUT status: got %d, want 2xx", putResp.StatusCode)
	}

	// Step 3: GET /photos/{id} and verify.
	req3, _ := http.NewRequest(http.MethodGet,
		fmt.Sprintf("%s/photos/%s", srv.URL, smokePhotoID), nil)
	resp3, err := client.Do(authHeader(req3))
	if err != nil {
		t.Fatalf("GET photo: %v", err)
	}
	if resp3.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp3.Body)
		t.Fatalf("GET photo status %d: %s", resp3.StatusCode, body)
	}

	var photoResp handler.PhotoResponse
	if err := json.NewDecoder(resp3.Body).Decode(&photoResp); err != nil {
		t.Fatalf("decode photo response: %v", err)
	}
	resp3.Body.Close()

	if photoResp.ID != smokePhotoID {
		t.Errorf("id: got %q, want %q", photoResp.ID, smokePhotoID)
	}
	if photoResp.OriginalURL == "" {
		t.Error("originalUrl should be non-empty")
	}
	if len(photoResp.Predictions) != 1 {
		t.Errorf("predictions count: got %d, want 1", len(photoResp.Predictions))
	}
}
