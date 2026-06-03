package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/polina2410/scout/backend/internal/db"
)

const uploadLinkTTL = 15 * time.Minute

// CreateUploadLink handles POST /photos/{photoId}/upload-link.
func (a *App) CreateUploadLink(w http.ResponseWriter, r *http.Request) {
	photoID := r.PathValue("photoId")
	if _, err := uuid.Parse(photoID); err != nil {
		WriteValidationError(w, r, []ValidationDetail{
			{Field: "photoId", Issue: "must be a valid UUID"},
		})
		return
	}

	var body struct {
		ContentType string `json:"contentType"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ContentType == "" {
		WriteValidationError(w, r, []ValidationDetail{
			{Field: "contentType", Issue: "required"},
		})
		return
	}

	if body.ContentType != "image/jpeg" {
		WriteValidationError(w, r, []ValidationDetail{
			{Field: "contentType", Issue: "must be image/jpeg"},
		})
		return
	}

	if _, err := a.DB.GetPhoto(r.Context(), photoID); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			WriteNotFoundError(w, r, photoID)
			return
		}
		WriteError(w, r, http.StatusInternalServerError, ErrCodeInternal, "failed to look up photo")
		return
	}

	presignURL, headers, expiresAt, err := a.Store.PresignedPutURL(r.Context(), photoID, body.ContentType, uploadLinkTTL)
	if err != nil {
		a.Log.Error("presign PUT failed", "photo_id", photoID, "error", err)
		WriteError(w, r, http.StatusInternalServerError, ErrCodeInternal, "failed to generate upload link")
		return
	}

	WriteJSON(w, http.StatusOK, UploadLinkResponse{
		URL:       presignURL,
		Method:    "PUT",
		Headers:   headers,
		ExpiresAt: expiresAt.Format(time.RFC3339),
	})
}
