package db

// ClassID is the set of known detection class names.
type ClassID string

const (
	ClassPowderyMildew ClassID = "powdery_mildew"
	ClassMirid         ClassID = "mirid"
	ClassWhiteflyAphid ClassID = "whitefly_aphid"
	ClassMinerTuta     ClassID = "miner_tuta"
	ClassThrips        ClassID = "thrips"
	ClassSpiderMites   ClassID = "spider_mites"
)

// BoundingBox holds a normalized [0,1] bounding box.
type BoundingBox struct {
	XMin float64
	YMin float64
	XMax float64
	YMax float64
}

// Prediction is one model detection attached to a photo.
type Prediction struct {
	ID         string
	ClassID    ClassID
	Confidence float64
	BBox       BoundingBox
}

// Photo is a camera photo with its position and all model predictions.
// OriginalURL is not stored in the DB — the handler layer fills it from MinIO before serialising.
type Photo struct {
	ID          string
	X, Y, H     float64
	Width       int
	Height      int
	CapturedAt  string // RFC3339
	OriginalURL string
	Predictions []Prediction
}

// ValidClassIDs is the set of accepted class_id values.
var ValidClassIDs = map[ClassID]struct{}{
	ClassPowderyMildew: {},
	ClassMirid:         {},
	ClassWhiteflyAphid: {},
	ClassMinerTuta:     {},
	ClassThrips:        {},
	ClassSpiderMites:   {},
}

// ValidClassID reports whether id is a known detection class.
func ValidClassID(id ClassID) bool {
	_, ok := ValidClassIDs[id]
	return ok
}

// ListParams controls pagination and filtering for ListPhotos.
type ListParams struct {
	Cursor        string
	Limit         int
	ClassID       ClassID
	MinConfidence float64
}
