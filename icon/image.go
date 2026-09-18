// Package icon provides reusable implementations of gui.IconSource.
package icon

import (
	"fmt"
	"image"
	"sync"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/icon/internal/pixels"
)

var _ gui.IconSource = (*Image)(nil)

// Image selects among immutable bitmap resolutions of the same icon.
// Create it with NewImage and reuse the pointer across UI rebuilds. The zero
// value supplies no image. Do not copy an Image after first use.
type Image struct {
	images    []image.Image
	rawColors bool

	mu         sync.Mutex
	tinted     image.Image
	index      int
	foreground gui.Color
}

// NewImage creates a source from already-decoded resolutions of the same icon.
// Every image must be non-nil and nonempty. It copies the image list, not pixels:
// callers must keep the supplied images immutable for the lifetime of the source.
func NewImage(images ...image.Image) (*Image, error) {
	if len(images) == 0 {
		return nil, fmt.Errorf("icon: at least one image is required")
	}
	for index, img := range images {
		if img == nil || img.Bounds().Empty() {
			return nil, fmt.Errorf("icon: image %d is nil or empty", index)
		}
	}
	return &Image{images: append([]image.Image{}, images...)}, nil
}

func (s *Image) WithRawColors() *Image {
	return &Image{images: s.images, rawColors: true}
}

// Image supplies the best available resolution, optionally tinted with foreground.
// The returned pixels are immutable and may be shared with previous results.
func (s *Image) Image(width, height int, foreground gui.Color) image.Image {
	if s == nil || width <= 0 || height <= 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var best image.Image
	bestScale := 0.0
	bestIndex := 0
	for index, img := range s.images {
		b := img.Bounds()
		// Icon contains the image without changing aspect ratio. The limiting
		// axis, not both axes independently, determines the required resolution.
		scale := max(float64(b.Dx())/float64(width), float64(b.Dy())/float64(height))
		if best == nil || (scale >= 1 && (bestScale < 1 || scale < bestScale)) || (scale < 1 && bestScale < 1 && scale > bestScale) {
			best, bestScale, bestIndex = img, scale, index
		}
	}
	if best == nil {
		return nil
	}
	if s.rawColors {
		return best
	}
	if s.tinted == nil || s.index != bestIndex || s.foreground != foreground {
		s.tinted = pixels.Tint(best, foreground)
		s.index, s.foreground = bestIndex, foreground
	}
	return s.tinted
}
