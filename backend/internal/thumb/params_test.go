package thumb

import (
	"net/url"
	"testing"

	"github.com/polina2410/scout/backend/internal/handler"
)

const validUUID = "11111111-1111-1111-1111-111111111111"

func makeQuery(kv ...string) url.Values {
	q := url.Values{}
	for i := 0; i+1 < len(kv); i += 2 {
		q.Set(kv[i], kv[i+1])
	}
	return q
}

func hasFieldError(details []handler.ValidationDetail, field string) bool {
	for _, d := range details {
		if d.Field == field {
			return true
		}
	}
	return false
}

func TestParseParams_OK(t *testing.T) {
	p, details := ParseParams(validUUID, makeQuery("w", "400", "dpr", "2", "fmt", "webp"))
	if len(details) != 0 {
		t.Fatalf("unexpected errors: %v", details)
	}
	if p.W != 400 {
		t.Errorf("W: got %d, want 400", p.W)
	}
	if p.DPR != 2 {
		t.Errorf("DPR: got %d, want 2", p.DPR)
	}
	if p.Fmt != "webp" {
		t.Errorf("Fmt: got %q, want webp", p.Fmt)
	}
	if p.PxWidth != 800 {
		t.Errorf("PxWidth: got %d, want 800 (400×2)", p.PxWidth)
	}
	if p.PhotoID != validUUID {
		t.Errorf("PhotoID: got %q, want %q", p.PhotoID, validUUID)
	}
}

func TestParseParams_DefaultDPRFmt(t *testing.T) {
	p, details := ParseParams(validUUID, makeQuery("w", "200"))
	if len(details) != 0 {
		t.Fatalf("unexpected errors: %v", details)
	}
	if p.DPR != 1 {
		t.Errorf("DPR default: got %d, want 1", p.DPR)
	}
	if p.Fmt != "webp" {
		t.Errorf("Fmt default: got %q, want webp", p.Fmt)
	}
	if p.PxWidth != 200 {
		t.Errorf("PxWidth: got %d, want 200", p.PxWidth)
	}
}

func TestParseParams_MissingW(t *testing.T) {
	_, details := ParseParams(validUUID, makeQuery())
	if !hasFieldError(details, "w") {
		t.Errorf("expected error on field 'w', got: %v", details)
	}
}

func TestParseParams_WZero(t *testing.T) {
	_, details := ParseParams(validUUID, makeQuery("w", "0"))
	if !hasFieldError(details, "w") {
		t.Errorf("expected error on field 'w', got: %v", details)
	}
}

func TestParseParams_WTooLarge(t *testing.T) {
	_, details := ParseParams(validUUID, makeQuery("w", "9999"))
	if !hasFieldError(details, "w") {
		t.Errorf("expected error on field 'w', got: %v", details)
	}
}

func TestParseParams_InvalidDPR(t *testing.T) {
	_, details := ParseParams(validUUID, makeQuery("w", "400", "dpr", "4"))
	if !hasFieldError(details, "dpr") {
		t.Errorf("expected error on field 'dpr', got: %v", details)
	}
}

func TestParseParams_InvalidFmt(t *testing.T) {
	_, details := ParseParams(validUUID, makeQuery("w", "400", "fmt", "avif"))
	if !hasFieldError(details, "fmt") {
		t.Errorf("expected error on field 'fmt', got: %v", details)
	}
}

func TestParseParams_PxWidthClamped(t *testing.T) {
	// 2560 × 3 = 7680 — should clamp to 2560
	p, details := ParseParams(validUUID, makeQuery("w", "2560", "dpr", "3"))
	if len(details) != 0 {
		t.Fatalf("unexpected errors: %v", details)
	}
	if p.PxWidth != 2560 {
		t.Errorf("PxWidth: got %d, want 2560 (clamped from 7680)", p.PxWidth)
	}
}

func TestParseParams_InvalidUUID(t *testing.T) {
	_, details := ParseParams("not-a-uuid", makeQuery("w", "400"))
	if !hasFieldError(details, "photoId") {
		t.Errorf("expected error on field 'photoId', got: %v", details)
	}
}
