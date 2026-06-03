package db

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	defaultPageLimit = 50
	maxPageLimit     = 200 // matches openapi.yaml GET /photos limit.maximum
)

// ErrNotFound is returned by GetPhoto when the photo ID does not exist.
var ErrNotFound = errors.New("not found")

// DB wraps a read-only connection pool to predictions.db.
type DB struct {
	db *sql.DB
}

// NewDB wraps an existing *sql.DB. Intended for testing — production code should use Open.
func NewDB(sqlDB *sql.DB) *DB {
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	return &DB{db: sqlDB}
}

// Open opens the SQLite file at path in read-only mode.
// The file must already be in WAL mode if WAL is desired; this connection cannot set it.
func Open(path string) (*DB, error) {
	dsn := fmt.Sprintf("file:%s?mode=ro&_busy_timeout=5000", path)
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	d := NewDB(sqlDB)
	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return d, nil
}

// Close releases the connection pool.
func (d *DB) Close() error {
	return d.db.Close()
}

// GetPhoto fetches a single photo and all its predictions.
// Returns ErrNotFound when the ID does not exist.
func (d *DB) GetPhoto(ctx context.Context, id string) (*Photo, error) {
	const photoQuery = `
		SELECT id, x, y, h, width, height, captured_at
		FROM photos
		WHERE id = ?`

	row := d.db.QueryRowContext(ctx, photoQuery, id)
	photo := &Photo{}
	err := row.Scan(&photo.ID, &photo.X, &photo.Y, &photo.H, &photo.Width, &photo.Height, &photo.CapturedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	photo.Predictions, err = d.predictionsForPhoto(ctx, id)
	if err != nil {
		return nil, err
	}
	return photo, nil
}

// ListPhotos returns a page of photos and an opaque next-cursor (empty on the last page).
func (d *DB) ListPhotos(ctx context.Context, p ListParams) ([]Photo, string, error) {
	if p.Limit <= 0 {
		p.Limit = defaultPageLimit
	}
	if p.Limit > maxPageLimit {
		p.Limit = maxPageLimit
	}

	// Decode cursor into (capturedAt, id) pair.
	var cursorTime, cursorID string
	if p.Cursor != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(p.Cursor)
		if err != nil {
			return nil, "", fmt.Errorf("invalid cursor: %w", err)
		}
		parts := strings.SplitN(string(decoded), "|", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, "", errors.New("invalid cursor format")
		}
		if _, err := time.Parse(time.RFC3339, parts[0]); err != nil {
			return nil, "", fmt.Errorf("invalid cursor timestamp: %w", err)
		}
		cursorTime, cursorID = parts[0], parts[1]
	}

	args := []any{}
	var where []string

	if cursorTime != "" {
		where = append(where, "(captured_at < ? OR (captured_at = ? AND id < ?))")
		args = append(args, cursorTime, cursorTime, cursorID)
	}

	// Single-prediction filter: one prediction must satisfy both classId AND minConfidence.
	where = append(where, `photos.id IN (
		SELECT DISTINCT photo_id FROM predictions
		WHERE (? = '' OR class_id = ?)
		  AND (? = 0  OR confidence >= ?)
	)`)
	args = append(args, string(p.ClassID), string(p.ClassID), p.MinConfidence, p.MinConfidence)

	whereClause := "WHERE " + strings.Join(where, " AND ")

	// Fetch one extra row to detect whether a next page exists without an extra COUNT query.
	query := fmt.Sprintf(`
		SELECT id, x, y, h, width, height, captured_at
		FROM photos
		%s
		ORDER BY captured_at DESC, id DESC
		LIMIT ?`, whereClause)
	args = append(args, p.Limit+1)

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	photos := make([]Photo, 0, p.Limit)
	for rows.Next() {
		var ph Photo
		if err := rows.Scan(&ph.ID, &ph.X, &ph.Y, &ph.H, &ph.Width, &ph.Height, &ph.CapturedAt); err != nil {
			return nil, "", err
		}
		photos = append(photos, ph)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	// Trim the lookahead row and build cursor before fetching predictions,
	// so we don't load predictions for a row we will discard.
	var nextCursor string
	if len(photos) > p.Limit {
		photos = photos[:p.Limit]
		last := photos[len(photos)-1]
		raw := last.CapturedAt + "|" + last.ID
		nextCursor = base64.RawURLEncoding.EncodeToString([]byte(raw))
	}

	if len(photos) == 0 {
		return photos, "", nil
	}

	// Load predictions for all page photos in one query.
	ids := make([]string, len(photos))
	for i, ph := range photos {
		ids[i] = ph.ID
	}
	predMap, err := d.predictionsForPhotos(ctx, ids)
	if err != nil {
		return nil, "", err
	}
	for i, ph := range photos {
		photos[i].Predictions = predMap[ph.ID]
	}

	return photos, nextCursor, nil
}

func (d *DB) predictionsForPhoto(ctx context.Context, photoID string) ([]Prediction, error) {
	m, err := d.predictionsForPhotos(ctx, []string{photoID})
	if err != nil {
		return nil, err
	}
	return m[photoID], nil
}

func (d *DB) predictionsForPhotos(ctx context.Context, ids []string) (map[string][]Prediction, error) {
	if len(ids) == 0 {
		return map[string][]Prediction{}, nil
	}

	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	query := fmt.Sprintf(`
		SELECT id, photo_id, class_id, confidence, bbox_xmin, bbox_ymin, bbox_xmax, bbox_ymax
		FROM predictions
		WHERE photo_id IN (%s)`, placeholders)

	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]Prediction)
	for rows.Next() {
		var pred Prediction
		var photoID string
		err := rows.Scan(
			&pred.ID, &photoID, &pred.ClassID, &pred.Confidence,
			&pred.BBox.XMin, &pred.BBox.YMin, &pred.BBox.XMax, &pred.BBox.YMax,
		)
		if err != nil {
			return nil, err
		}
		result[photoID] = append(result[photoID], pred)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
