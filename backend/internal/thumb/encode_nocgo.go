//go:build !cgo

package thumb

import (
	"bytes"
	"image"
	"image/jpeg"
)

func encodeImage(dst image.Image, _ string) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// effectiveFormat maps "webp" to "jpeg" when CGO is unavailable.
// This keeps the cache key and Content-Type consistent with what is actually encoded.
func effectiveFormat(imgFmt string) string {
	if imgFmt == "webp" {
		return "jpeg"
	}
	return imgFmt
}
