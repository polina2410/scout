//go:build cgo

package thumb

import (
	"bytes"
	"image"
	"image/jpeg"

	"github.com/chai2010/webp"
)

const webpQuality = 80

func encodeImage(dst image.Image, imgFmt string) ([]byte, error) {
	var buf bytes.Buffer
	if imgFmt == "webp" {
		if err := webp.Encode(&buf, dst, &webp.Options{Quality: webpQuality}); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func effectiveFormat(imgFmt string) string {
	return imgFmt
}
