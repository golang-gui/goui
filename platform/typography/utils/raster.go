package utils

import (
	"fmt"
	"image"
)

// A caller should rasterize large layouts in regions. Bound temporary native
// storage as well as Go allocation before passing dimensions to native APIs.
const maxRasterBytes = 64 << 20

// RasterDimensions validates a raster request without overflowing int arithmetic.
// Empty rectangles need no allocation, including non-canonical rectangles.
func RasterDimensions(region image.Rectangle) (width, height int, err error) {
	if region.Empty() {
		return 0, 0, nil
	}
	w := uint64(region.Max.X) - uint64(region.Min.X)
	h := uint64(region.Max.Y) - uint64(region.Min.Y)
	if w > maxRasterBytes/4 || h > maxRasterBytes/4 || w*h > maxRasterBytes/4 {
		return 0, 0, fmt.Errorf("typography: raster region %v exceeds 64 MiB RGBA storage limit", region)
	}
	return int(w), int(h), nil
}
