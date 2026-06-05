package handler_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/polina2410/scout/backend/internal/db"
	"github.com/polina2410/scout/backend/internal/handler"
	"github.com/polina2410/scout/backend/internal/middleware"
)

// ---- mock presigner ----

type mockPresigner struct {
	putURL string
	getURL string
	putErr error
	getErr error
}

func (m *mockPresigner) PresignedPutURL(_ context.Context, _ string, contentType string, ttl time.Duration) (string, map[string]string, time.Time, error) {
	return m.putURL, map[string]string{"Content-Type": contentType}, time.Now().Add(ttl), m.putErr
}

func (m *mockPresigner) PresignedGetURL(_ context.Context, _ string) (string, error) {
	return m.getURL, m.getErr
}

// ---- test DB setup ----

const testSchema = `
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
);`

const testFixtures = `
INSERT INTO photos VALUES
	('11111111-1111-1111-1111-111111111111', 1.0, 2.0, 3.0, 2560, 1440, '2026-06-01T10:00:00Z'),
	('22222222-2222-2222-2222-222222222222', 4.0, 5.0, 3.0, 2560, 1440, '2026-06-02T10:00:00Z'),
	('33333333-3333-3333-3333-333333333333', 7.0, 8.0, 3.0, 2560, 1440, '2026-06-03T10:00:00Z');

INSERT INTO predictions VALUES
	('pred-1a', '11111111-1111-1111-1111-111111111111', 'thrips',         0.95, 0.1, 0.1, 0.2, 0.2),
	('pred-1b', '11111111-1111-1111-1111-111111111111', 'mirid',          0.80, 0.3, 0.3, 0.4, 0.4),
	('pred-2a', '22222222-2222-2222-2222-222222222222', 'thrips',         0.70, 0.1, 0.1, 0.2, 0.2),
	('pred-2b', '22222222-2222-2222-2222-222222222222', 'spider_mites',   0.95, 0.5, 0.5, 0.6, 0.6),
	('pred-3a', '33333333-3333-3333-3333-333333333333', 'powdery_mildew', 0.60, 0.2, 0.2, 0.3, 0.3);`

const (
	photo1ID = "11111111-1111-1111-1111-111111111111"
	photo2ID = "22222222-2222-2222-2222-222222222222"
	photo3ID = "33333333-3333-3333-3333-333333333333"
)

func openTestDB(t *testing.T) *db.DB {
	t.Helper()
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	if _, err := sqlDB.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	if _, err := sqlDB.Exec(testSchema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	if _, err := sqlDB.Exec(testFixtures); err != nil {
		t.Fatalf("insert fixtures: %v", err)
	}
	d := db.NewDB(sqlDB)
	t.Cleanup(func() { d.Close() })
	return d
}

// ---- test server ----

const testAPIKey = "test-api-key"

func newTestServer(t *testing.T, d *db.DB, store *mockPresigner) *httptest.Server {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	app := &handler.App{DB: d, Store: store, Log: log}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		handler.WriteJSON(w, http.StatusOK, handler.HealthResponse{Status: "ok", Version: "test"})
	})
	mux.HandleFunc("POST /photos/{photoId}/upload-link", app.CreateUploadLink)
	mux.HandleFunc("GET /photos/{photoId}", app.GetPhoto)
	mux.HandleFunc("GET /photos", app.ListPhotos)

	var h http.Handler = mux
	h = middleware.APIKeyAuth(testAPIKey)(h)
	h = middleware.CorrelationID(log)(h)

	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func get(t *testing.T, srv *httptest.Server, path string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, srv.URL+path, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("X-API-Key", testAPIKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

func post(t *testing.T, srv *httptest.Server, path, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("X-API-Key", testAPIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

func decodeJSON(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer resp.Body.Close()
	var m map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return m
}

// ---- upload-link tests ----

func TestCreateUploadLink_OK(t *testing.T) {
	store := &mockPresigner{putURL: "https://minio.example/presigned-put"}
	srv := newTestServer(t, openTestDB(t), store)

	resp := post(t, srv, fmt.Sprintf("/photos/%s/upload-link", photo1ID), `{"contentType":"image/jpeg"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	body := decodeJSON(t, resp)
	if body["url"] != store.putURL {
		t.Errorf("url: got %v, want %v", body["url"], store.putURL)
	}
	if body["method"] != "PUT" {
		t.Errorf("method: got %v, want PUT", body["method"])
	}
	if body["expiresAt"] == "" || body["expiresAt"] == nil {
		t.Error("expiresAt should be set")
	}
}

func TestCreateUploadLink_NotFound(t *testing.T) {
	srv := newTestServer(t, openTestDB(t), &mockPresigner{})

	resp := post(t, srv, "/photos/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/upload-link", `{"contentType":"image/jpeg"}`)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404", resp.StatusCode)
	}
	body := decodeJSON(t, resp)
	if body["code"] != "NotFound" {
		t.Errorf("code: got %v, want NotFound", body["code"])
	}
	if body["resource_id"] == "" || body["resource_id"] == nil {
		t.Error("resource_id should be present")
	}
}

func TestCreateUploadLink_BadContentType(t *testing.T) {
	srv := newTestServer(t, openTestDB(t), &mockPresigner{})

	resp := post(t, srv, fmt.Sprintf("/photos/%s/upload-link", photo1ID), `{"contentType":"image/png"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", resp.StatusCode)
	}
	body := decodeJSON(t, resp)
	if body["code"] != "ValidationError" {
		t.Errorf("code: got %v, want ValidationError", body["code"])
	}
	details, ok := body["details"].([]any)
	if !ok || len(details) == 0 {
		t.Fatal("expected non-empty details array")
	}
	d := details[0].(map[string]any)
	if d["field"] != "contentType" {
		t.Errorf("details[0].field: got %v, want contentType", d["field"])
	}
}

func TestCreateUploadLink_MissingBody(t *testing.T) {
	srv := newTestServer(t, openTestDB(t), &mockPresigner{})

	resp := post(t, srv, fmt.Sprintf("/photos/%s/upload-link", photo1ID), `not json`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", resp.StatusCode)
	}
	body := decodeJSON(t, resp)
	if body["code"] != "ValidationError" {
		t.Errorf("code: got %v, want ValidationError", body["code"])
	}
}

// ---- get photo tests ----

func TestGetPhoto_OK(t *testing.T) {
	store := &mockPresigner{getURL: "https://minio.example/presigned-get"}
	srv := newTestServer(t, openTestDB(t), store)

	resp := get(t, srv, fmt.Sprintf("/photos/%s", photo1ID))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	body := decodeJSON(t, resp)
	if body["id"] != photo1ID {
		t.Errorf("id: got %v, want %v", body["id"], photo1ID)
	}
	if body["originalUrl"] != store.getURL {
		t.Errorf("originalUrl: got %v, want %v", body["originalUrl"], store.getURL)
	}
	preds, ok := body["predictions"].([]any)
	if !ok {
		t.Fatal("predictions should be an array")
	}
	if len(preds) != 2 {
		t.Errorf("predictions count: got %d, want 2", len(preds))
	}
}

func TestGetPhoto_NotFound(t *testing.T) {
	srv := newTestServer(t, openTestDB(t), &mockPresigner{})

	resp := get(t, srv, "/photos/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404", resp.StatusCode)
	}
	body := decodeJSON(t, resp)
	if body["code"] != "NotFound" {
		t.Errorf("code: got %v, want NotFound", body["code"])
	}
}

// ---- list photos tests ----

func TestListPhotos_OK(t *testing.T) {
	store := &mockPresigner{getURL: "https://minio.example/presigned-get"}
	srv := newTestServer(t, openTestDB(t), store)

	resp := get(t, srv, "/photos")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	body := decodeJSON(t, resp)
	items, ok := body["items"].([]any)
	if !ok {
		t.Fatal("items should be an array")
	}
	if len(items) != 3 {
		t.Errorf("items count: got %d, want 3", len(items))
	}
	if _, exists := body["next_token"]; exists {
		t.Error("next_token should be absent on last page")
	}
}

func TestListPhotos_Filter(t *testing.T) {
	store := &mockPresigner{getURL: "https://minio.example/presigned-get"}
	srv := newTestServer(t, openTestDB(t), store)

	// Only photo1 has a single prediction that is BOTH thrips AND confidence >= 0.9
	resp := get(t, srv, "/photos?classId=thrips&minConfidence=0.9")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	body := decodeJSON(t, resp)
	items := body["items"].([]any)
	if len(items) != 1 {
		t.Errorf("items count: got %d, want 1 (only photo-1 has thrips >= 0.9)", len(items))
	}
}

func TestListPhotos_Pagination(t *testing.T) {
	store := &mockPresigner{getURL: "https://minio.example/presigned-get"}
	srv := newTestServer(t, openTestDB(t), store)

	resp := get(t, srv, "/photos?limit=1")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	body := decodeJSON(t, resp)
	items := body["items"].([]any)
	if len(items) != 1 {
		t.Errorf("items count: got %d, want 1", len(items))
	}
	if body["next_token"] == nil || body["next_token"] == "" {
		t.Error("next_token should be present when more results exist")
	}
}

func TestListPhotos_BadClassID(t *testing.T) {
	srv := newTestServer(t, openTestDB(t), &mockPresigner{})

	// An unknown class must be a 400, not a silent empty result set.
	resp := get(t, srv, "/photos?classId=not_a_real_class")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", resp.StatusCode)
	}
	body := decodeJSON(t, resp)
	if body["code"] != "ValidationError" {
		t.Errorf("code: got %v, want ValidationError", body["code"])
	}
	details, ok := body["details"].([]any)
	if !ok || len(details) == 0 {
		t.Fatal("expected non-empty details array")
	}
	if d := details[0].(map[string]any); d["field"] != "classId" {
		t.Errorf("details[0].field: got %v, want classId", d["field"])
	}
}

func TestListPhotos_BadLimit(t *testing.T) {
	srv := newTestServer(t, openTestDB(t), &mockPresigner{})

	resp := get(t, srv, "/photos?limit=999")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", resp.StatusCode)
	}
	body := decodeJSON(t, resp)
	if body["code"] != "ValidationError" {
		t.Errorf("code: got %v, want ValidationError", body["code"])
	}
}

// ---- auth tests ----

func TestAuth_Missing(t *testing.T) {
	srv := newTestServer(t, openTestDB(t), &mockPresigner{})

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/photos", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", resp.StatusCode)
	}
	body := decodeJSON(t, resp)
	if body["code"] != "AuthenticationRequired" {
		t.Errorf("code: got %v, want AuthenticationRequired", body["code"])
	}
}

func TestAuth_Wrong(t *testing.T) {
	srv := newTestServer(t, openTestDB(t), &mockPresigner{})

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/photos", nil)
	req.Header.Set("X-API-Key", "wrong-key")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", resp.StatusCode)
	}
	body := decodeJSON(t, resp)
	if body["code"] != "AuthenticationRequired" {
		t.Errorf("code: got %v, want AuthenticationRequired", body["code"])
	}
}

func TestAuth_HealthBypass(t *testing.T) {
	srv := newTestServer(t, openTestDB(t), &mockPresigner{})

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/health", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (health should bypass auth)", resp.StatusCode)
	}
}
