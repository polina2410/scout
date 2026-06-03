package handler

import "github.com/polina2410/scout/backend/internal/db"

// BoundingBoxResponse is the JSON form of db.BoundingBox.
type BoundingBoxResponse struct {
	XMin float64 `json:"xMin"`
	YMin float64 `json:"yMin"`
	XMax float64 `json:"xMax"`
	YMax float64 `json:"yMax"`
}

// PredictionResponse is the JSON form of db.Prediction.
type PredictionResponse struct {
	ClassID    string              `json:"classId"`
	Confidence float64             `json:"confidence"`
	BBox       BoundingBoxResponse `json:"bbox"`
}

// PhotoResponse is the JSON form of db.Photo with a filled OriginalURL.
type PhotoResponse struct {
	ID          string               `json:"id"`
	X           float64              `json:"x"`
	Y           float64              `json:"y"`
	H           float64              `json:"h"`
	Width       int                  `json:"width"`
	Height      int                  `json:"height"`
	CapturedAt  string               `json:"capturedAt"`
	OriginalURL string               `json:"originalUrl"`
	Predictions []PredictionResponse `json:"predictions"`
}

// PhotoPageResponse is the JSON body for GET /photos.
type PhotoPageResponse struct {
	Items     []PhotoResponse `json:"items"`
	NextToken string          `json:"next_token,omitempty"`
}

// UploadLinkResponse is the JSON body for POST /photos/{photoId}/upload-link.
type UploadLinkResponse struct {
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers,omitempty"`
	ExpiresAt string            `json:"expiresAt"`
}

func predictionToResponse(p db.Prediction) PredictionResponse {
	return PredictionResponse{
		ClassID:    string(p.ClassID),
		Confidence: p.Confidence,
		BBox: BoundingBoxResponse{
			XMin: p.BBox.XMin,
			YMin: p.BBox.YMin,
			XMax: p.BBox.XMax,
			YMax: p.BBox.YMax,
		},
	}
}

func photoToResponse(p db.Photo, originalURL string) PhotoResponse {
	preds := make([]PredictionResponse, len(p.Predictions))
	for i, pr := range p.Predictions {
		preds[i] = predictionToResponse(pr)
	}
	return PhotoResponse{
		ID:          p.ID,
		X:           p.X,
		Y:           p.Y,
		H:           p.H,
		Width:       p.Width,
		Height:      p.Height,
		CapturedAt:  p.CapturedAt,
		OriginalURL: originalURL,
		Predictions: preds,
	}
}
