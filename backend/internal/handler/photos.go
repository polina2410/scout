package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"sync"

	"github.com/polina2410/scout/backend/internal/db"
)

const (
	defaultLimit   = 50
	maxLimit       = 200
	presignWorkers = 10
)

// ListPhotos handles GET /photos.
func (a *App) ListPhotos(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	cursor := q.Get("cursor")

	limit := defaultLimit
	if raw := q.Get("limit"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 1 || v > maxLimit {
			WriteValidationError(w, r, []ValidationDetail{
				{Field: "limit", Issue: "must be an integer between 1 and 200"},
			})
			return
		}
		limit = v
	}

	var minConfidence float64
	if raw := q.Get("minConfidence"); raw != "" {
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil || v < 0 || v > 1 {
			WriteValidationError(w, r, []ValidationDetail{
				{Field: "minConfidence", Issue: "must be a number between 0 and 1"},
			})
			return
		}
		minConfidence = v
	}

	// An empty classId means "no class filter"; only reject non-empty unknown values.
	// Without this, an invalid class silently returns 0 photos instead of a 400,
	// making frontend bugs hard to diagnose.
	classID := db.ClassID(q.Get("classId"))
	if classID != "" && !db.ValidClassID(classID) {
		WriteValidationError(w, r, []ValidationDetail{
			{Field: "classId", Issue: "must be one of the known detection classes"},
		})
		return
	}

	photos, nextCursor, err := a.DB.ListPhotos(r.Context(), db.ListParams{
		Cursor:        cursor,
		Limit:         limit,
		ClassID:       classID,
		MinConfidence: minConfidence,
	})
	if err != nil {
		if errors.Is(err, db.ErrInvalidCursor) {
			WriteValidationError(w, r, []ValidationDetail{
				{Field: "cursor", Issue: "invalid or malformed cursor"},
			})
			return
		}
		a.Log.Error("list photos failed", "error", err)
		WriteError(w, r, http.StatusInternalServerError, ErrCodeInternal, "failed to list photos")
		return
	}

	urlMap, err := a.presignAll(r, photos)
	if err != nil {
		a.Log.Error("presign GET failed during list", "error", err)
		WriteError(w, r, http.StatusInternalServerError, ErrCodeInternal, "failed to generate photo URLs")
		return
	}

	items := make([]PhotoResponse, len(photos))
	for i, ph := range photos {
		items[i] = photoToResponse(ph, urlMap[ph.ID])
	}

	WriteJSON(w, http.StatusOK, PhotoPageResponse{
		Items:     items,
		NextToken: nextCursor,
	})
}

func (a *App) presignAll(r *http.Request, photos []db.Photo) (map[string]string, error) {
	if len(photos) == 0 {
		return map[string]string{}, nil
	}

	// Derive a cancellable context from the request. On a client disconnect or the
	// first presign failure, workers still waiting to acquire the semaphore exit
	// immediately; a presign call already in flight runs to completion (the MinIO
	// SDK doesn't abort mid-call) but its result is simply discarded. Either way
	// no goroutine leaks, since the results channel is buffered to len(photos).
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	type result struct {
		id  string
		url string
		err error
	}

	workerCount := min(len(photos), presignWorkers)
	results := make(chan result, len(photos))
	sem := make(chan struct{}, workerCount)
	var wg sync.WaitGroup

	for _, ph := range photos {
		ph := ph
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				results <- result{id: ph.ID, err: ctx.Err()}
				return
			}
			url, err := a.Store.PresignedGetURL(ctx, ph.ID)
			<-sem
			results <- result{id: ph.ID, url: url, err: err}
		}()
	}

	go func() { wg.Wait(); close(results) }()

	urlMap := make(map[string]string, len(photos))
	for res := range results {
		if res.err != nil {
			// Cancel the remaining workers; the buffered channel and the waiter
			// goroutine above ensure they all drain without blocking or leaking.
			cancel()
			return nil, res.err
		}
		urlMap[res.id] = res.url
	}
	return urlMap, nil
}
