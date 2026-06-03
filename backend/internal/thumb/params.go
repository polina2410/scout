package thumb

import (
	"net/url"
	"strconv"

	"github.com/google/uuid"
	"github.com/polina2410/scout/backend/internal/handler"
)

const maxOriginalWidth = 2560

// Params holds the validated parameters for a single thumbnail request.
type Params struct {
	PhotoID string // bare UUID (already validated)
	W       int    // CSS pixel width (1–2560)
	DPR     int    // device pixel ratio (1, 2, or 3)
	Fmt     string // "webp" or "jpeg"
	PxWidth int    // W × DPR, clamped to maxOriginalWidth
}

// ParseParams parses and validates query parameters.
// Returns a non-empty details slice when any param is invalid — caller must
// call handler.WriteValidationError and return without using Params.
// All errors are collected rather than stopping at the first.
func ParseParams(photoID string, q url.Values) (Params, []handler.ValidationDetail) {
	var details []handler.ValidationDetail
	p := Params{DPR: 1, Fmt: "webp"}

	if _, err := uuid.Parse(photoID); err != nil {
		details = append(details, handler.ValidationDetail{
			Field: "photoId",
			Issue: "must be a valid UUID",
		})
	} else {
		p.PhotoID = photoID
	}

	if raw := q.Get("w"); raw == "" {
		details = append(details, handler.ValidationDetail{
			Field: "w",
			Issue: "required",
		})
	} else if v, err := strconv.Atoi(raw); err != nil || v < 1 || v > maxOriginalWidth {
		details = append(details, handler.ValidationDetail{
			Field: "w",
			Issue: "must be an integer between 1 and 2560",
		})
	} else {
		p.W = v
	}

	if raw := q.Get("dpr"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 1 || v > 3 {
			details = append(details, handler.ValidationDetail{
				Field: "dpr",
				Issue: "must be 1, 2, or 3",
			})
		} else {
			p.DPR = v
		}
	}

	if raw := q.Get("fmt"); raw != "" {
		if raw != "webp" && raw != "jpeg" {
			details = append(details, handler.ValidationDetail{
				Field: "fmt",
				Issue: `must be "webp" or "jpeg"`,
			})
		} else {
			p.Fmt = raw
		}
	}

	if len(details) == 0 {
		px := p.W * p.DPR
		if px > maxOriginalWidth {
			px = maxOriginalWidth
		}
		p.PxWidth = px
	}

	return p, details
}
