package handler

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/polina2410/scout/backend/internal/db"
)

// GetPhoto handles GET /photos/{photoId}.
func (a *App) GetPhoto(w http.ResponseWriter, r *http.Request) {
	photoID := r.PathValue("photoId")
	if _, err := uuid.Parse(photoID); err != nil {
		WriteNotFoundError(w, r, photoID)
		return
	}

	photo, err := a.DB.GetPhoto(r.Context(), photoID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			WriteNotFoundError(w, r, photoID)
			return
		}
		WriteError(w, r, http.StatusInternalServerError, ErrCodeInternal, "failed to fetch photo")
		return
	}

	originalURL, err := a.Store.PresignedGetURL(r.Context(), photoID)
	if err != nil {
		a.Log.Error("presign GET failed", "photo_id", photoID, "error", err)
		WriteError(w, r, http.StatusInternalServerError, ErrCodeInternal, "failed to generate photo URL")
		return
	}

	WriteJSON(w, http.StatusOK, photoToResponse(*photo, originalURL))
}
