package db

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	_ "modernc.org/sqlite"
)

const schema = `
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

// seed data:
//
//	photo-1  2026-06-01  → pred-1a: thrips      0.95
//	                     → pred-1b: mirid       0.80
//	photo-2  2026-06-02  → pred-2a: thrips      0.70  (thrips but < 0.9)
//	                     → pred-2b: spider_mites 0.95  (≥ 0.9 but not thrips)
//	photo-3  2026-06-03  → pred-3a: powdery_mildew 0.60
const fixtures = `
INSERT INTO photos VALUES
	('photo-1', 1.0, 2.0, 3.0, 2560, 1440, '2026-06-01T10:00:00Z'),
	('photo-2', 4.0, 5.0, 3.0, 2560, 1440, '2026-06-02T10:00:00Z'),
	('photo-3', 7.0, 8.0, 3.0, 2560, 1440, '2026-06-03T10:00:00Z');

INSERT INTO predictions VALUES
	('pred-1a', 'photo-1', 'thrips',         0.95, 0.1, 0.1, 0.2, 0.2),
	('pred-1b', 'photo-1', 'mirid',          0.80, 0.3, 0.3, 0.4, 0.4),
	('pred-2a', 'photo-2', 'thrips',         0.70, 0.1, 0.1, 0.2, 0.2),
	('pred-2b', 'photo-2', 'spider_mites',   0.95, 0.5, 0.5, 0.6, 0.6),
	('pred-3a', 'photo-3', 'powdery_mildew', 0.60, 0.2, 0.2, 0.3, 0.3);`

func openTestDB(t *testing.T) *DB {
	t.Helper()
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	if _, err := sqlDB.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	if _, err := sqlDB.Exec(schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	if _, err := sqlDB.Exec(fixtures); err != nil {
		t.Fatalf("insert fixtures: %v", err)
	}
	d := NewDB(sqlDB)
	t.Cleanup(func() { d.Close() })
	return d
}

func TestGetPhoto_HappyPath(t *testing.T) {
	d := openTestDB(t)

	photo, err := d.GetPhoto(context.Background(), "photo-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if photo.ID != "photo-1" {
		t.Errorf("id: got %q, want %q", photo.ID, "photo-1")
	}
	if len(photo.Predictions) != 2 {
		t.Errorf("predictions: got %d, want 2", len(photo.Predictions))
	}
	// Verify at least one prediction has expected field values (guards against wrong Scan order).
	found := false
	for _, p := range photo.Predictions {
		if p.ClassID == ClassThrips && p.Confidence == 0.95 {
			found = true
		}
	}
	if !found {
		t.Error("expected a thrips prediction with confidence 0.95")
	}
}

func TestGetPhoto_NotFound(t *testing.T) {
	d := openTestDB(t)

	_, err := d.GetPhoto(context.Background(), "does-not-exist")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error: got %v, want ErrNotFound", err)
	}
}

func TestListPhotos_NoFilter(t *testing.T) {
	d := openTestDB(t)

	photos, _, err := d.ListPhotos(context.Background(), ListParams{Limit: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(photos) != 3 {
		t.Errorf("count: got %d, want 3", len(photos))
	}
}

func TestListPhotos_ClassFilter(t *testing.T) {
	d := openTestDB(t)

	// thrips appears in photo-1 (0.95) and photo-2 (0.70)
	photos, _, err := d.ListPhotos(context.Background(), ListParams{Limit: 50, ClassID: ClassThrips})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(photos) != 2 {
		t.Errorf("count: got %d, want 2", len(photos))
	}
}

func TestListPhotos_ConfidenceFilter(t *testing.T) {
	d := openTestDB(t)

	// pred-1a (thrips 0.95) and pred-2b (spider_mites 0.95) both qualify → photo-1 and photo-2
	photos, _, err := d.ListPhotos(context.Background(), ListParams{Limit: 50, MinConfidence: 0.9})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(photos) != 2 {
		t.Errorf("count: got %d, want 2", len(photos))
	}
}

func TestListPhotos_CombinedFilter(t *testing.T) {
	d := openTestDB(t)

	// Must be thrips AND confidence >= 0.9 in the SAME prediction.
	// photo-1: pred-1a is thrips at 0.95 → matches
	// photo-2: pred-2a is thrips at 0.70 (fails confidence), pred-2b is spider_mites at 0.95 (fails class) → rejected
	photos, _, err := d.ListPhotos(context.Background(), ListParams{
		Limit:         50,
		ClassID:       ClassThrips,
		MinConfidence: 0.9,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(photos) != 1 {
		t.Errorf("count: got %d, want 1", len(photos))
	}
	if photos[0].ID != "photo-1" {
		t.Errorf("id: got %q, want photo-1", photos[0].ID)
	}
}

func TestListPhotos_Pagination(t *testing.T) {
	d := openTestDB(t)

	// First page: limit 1 — should get photo-3 (newest captured_at)
	page1, cursor, err := d.ListPhotos(context.Background(), ListParams{Limit: 1})
	if err != nil {
		t.Fatalf("page1 error: %v", err)
	}
	if len(page1) != 1 {
		t.Fatalf("page1 count: got %d, want 1", len(page1))
	}
	if cursor == "" {
		t.Fatal("expected non-empty cursor after first page")
	}

	// Second page: should get photo-2
	page2, cursor2, err := d.ListPhotos(context.Background(), ListParams{Limit: 1, Cursor: cursor})
	if err != nil {
		t.Fatalf("page2 error: %v", err)
	}
	if len(page2) != 1 {
		t.Fatalf("page2 count: got %d, want 1", len(page2))
	}
	if page2[0].ID != "photo-2" {
		t.Errorf("page2 id: got %q, want photo-2", page2[0].ID)
	}

	// Third page: should get photo-1, no more cursor
	page3, cursor3, err := d.ListPhotos(context.Background(), ListParams{Limit: 1, Cursor: cursor2})
	if err != nil {
		t.Fatalf("page3 error: %v", err)
	}
	if len(page3) != 1 {
		t.Fatalf("page3 count: got %d, want 1", len(page3))
	}
	if page3[0].ID != "photo-1" {
		t.Errorf("page3 id: got %q, want photo-1", page3[0].ID)
	}
	if cursor3 != "" {
		t.Errorf("expected empty cursor on last page, got %q", cursor3)
	}
}

func TestListPhotos_MalformedCursor(t *testing.T) {
	d := openTestDB(t)

	cases := []struct {
		name   string
		cursor string
	}{
		{"not base64", "!!!invalid!!!"},
		{"missing pipe separator", "MjAyNi0wNi0wMVQxMDowMDowMFo"}, // base64("2026-06-01T10:00:00Z") no pipe
		{"empty timestamp", "fHBob3RvLTE"},                         // base64("|photo-1")
		{"empty id", "MjAyNi0wNi0wMVQxMDowMDowMFp8"},              // base64("2026-06-01T10:00:00Z|")
		{"invalid timestamp", "Zm9vfHBob3RvLTE"},                   // base64("foo|photo-1")
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := d.ListPhotos(context.Background(), ListParams{Limit: 10, Cursor: tc.cursor})
			if err == nil {
				t.Error("expected error for malformed cursor, got nil")
			}
		})
	}
}

func TestListPhotos_EmptyResult(t *testing.T) {
	d := openTestDB(t)

	// No photos have class "unknown_class"
	photos, cursor, err := d.ListPhotos(context.Background(), ListParams{Limit: 50, ClassID: "unknown_class"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if photos == nil {
		t.Error("expected non-nil empty slice, got nil")
	}
	if len(photos) != 0 {
		t.Errorf("count: got %d, want 0", len(photos))
	}
	if cursor != "" {
		t.Errorf("expected empty cursor, got %q", cursor)
	}
}
