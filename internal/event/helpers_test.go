package event

import (
	"image"
	"image/color"
)

// newSolidRgba creates a small solid colored RGBA image used by the event package tests
// instead of decoding a real video frame.
func newSolidRgba(width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y += 1 {
		for x := 0; x < width; x += 1 {
			img.Set(x, y, color.RGBA{R: 128, G: 128, B: 128, A: 0xff})
		}
	}

	return img
}
