package textbitmap

import (
	"fmt"
	"image"
	"math"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/utils"
)

// Rasterize native layouts in bounded blocks, assembled before composition.
// This avoids artificial antialiased edges between adjacent image quads.
const TileSize = 1024
const CacheCapacity = 4

// Region returns a visible region in the common layout raster pixel plane.
// Edges are aligned to stable blocks so small scrolls reuse cached images.
// origin has already been snapped once for the entire layout. clip is the
// effective device-pixel clip, intersected with the current render target.
func Region(layout typography.TextLayout, origin geometry.Point, transform geometry.Transform, deviceScale, rasterScale float32, clip image.Rectangle) image.Rectangle {
	if clip.Empty() || rasterScale <= 0 || deviceScale <= 0 || !finitePoint(origin) {
		return image.Rectangle{}
	}
	w, h := layout.MeasureSize()
	boxWidth, _ := layout.Size()
	// Alignment and bidi can position text away from X=0. Include the layout
	// box as well as overflowing text, rather than treating MeasureSize as ink
	// bounds. There is no vertical alignment in TextFormat.
	w = max(w, boxWidth)
	if w < 0 || h <= 0 || !finite(float64(w)) || !finite(float64(h)) {
		return image.Rectangle{}
	}
	width, height := math.Ceil(float64(w)*float64(rasterScale)), math.Ceil(float64(h)*float64(rasterScale))
	if width > math.MaxInt32-2 || height > math.MaxInt32-2 {
		return image.Rectangle{}
	}
	bounds := image.Rect(-1, -1, int(width)+1, int(height)+1)
	t := geometry.Scale(deviceScale, deviceScale).Multiply(transform)
	a, b, c, d := float64(t.A11), float64(t.A12), float64(t.A21), float64(t.A22)
	det := a*d - b*c
	if math.Abs(det) < 1e-12 || !finite(det) || !finite(float64(t.TX)) || !finite(float64(t.TY)) {
		return image.Rectangle{}
	}
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, p := range []image.Point{clip.Min, {X: clip.Max.X, Y: clip.Min.Y}, clip.Max, {X: clip.Min.X, Y: clip.Max.Y}} {
		x, y := float64(p.X)-float64(t.TX), float64(p.Y)-float64(t.TY)
		lx := ((d*x-b*y)/det - float64(origin.X)) * float64(rasterScale)
		ly := ((a*y-c*x)/det - float64(origin.Y)) * float64(rasterScale)
		minX, maxX = min(minX, lx), max(maxX, lx)
		minY, maxY = min(minY, ly), max(maxY, ly)
	}
	// Outward rounding includes the filter footprint. Clamp before converting
	// to int, including very large inverse bounds near singular transforms.
	left, top := max(float64(bounds.Min.X), math.Floor(minX)-1), max(float64(bounds.Min.Y), math.Floor(minY)-1)
	right, bottom := min(float64(bounds.Max.X), math.Ceil(maxX)+1), min(float64(bounds.Max.Y), math.Ceil(maxY)+1)
	if left >= right || top >= bottom {
		return image.Rectangle{}
	}
	visible := image.Rect(int(left), int(top), int(right), int(bottom))
	firstX := bounds.Min.X + (visible.Min.X-bounds.Min.X)/TileSize*TileSize
	firstY := bounds.Min.Y + (visible.Min.Y-bounds.Min.Y)/TileSize*TileSize
	lastX := bounds.Min.X + ((visible.Max.X-bounds.Min.X+TileSize-1)/TileSize)*TileSize
	lastY := bounds.Min.Y + ((visible.Max.Y-bounds.Min.Y+TileSize-1)/TileSize)*TileSize
	return image.Rect(firstX, firstY, min(lastX, bounds.Max.X), min(lastY, bounds.Max.Y)).Inset(-1)
}

// Rasterize assembles bounded native rasterizations into a single image, so
// Software and NanoVG never blend artificial tile boundaries. All blocks share
// the layout's raster pixel grid; clipping never reshapes partial strings.
func Rasterize(layout typography.TextLayout, scale float32, region image.Rectangle, buf []byte) (typography.TextBitmap, error) {
	width, height, err := utils.RasterDimensions(region)
	if err != nil || width == 0 || height == 0 {
		return typography.TextBitmap{}, err
	}
	if width <= TileSize && height <= TileSize {
		return layout.Rasterize(scale, region, buf)
	}
	bitmap := typography.TextBitmap{Width: width, Height: height, Stride: width * 4}
	n := bitmap.Stride * height
	if cap(buf) >= n {
		bitmap.Pixels = buf[:n]
	} else {
		bitmap.Pixels = make([]byte, n)
	}
	var scratch []byte
	for y := region.Min.Y; y < region.Max.Y; y += TileSize {
		for x := region.Min.X; x < region.Max.X; x += TileSize {
			part := image.Rect(x, y, min(x+TileSize, region.Max.X), min(y+TileSize, region.Max.Y))
			tile, err := layout.Rasterize(scale, part, scratch)
			if err != nil {
				return typography.TextBitmap{}, err
			}
			if tile.Width != part.Dx() || tile.Height != part.Dy() {
				return typography.TextBitmap{}, fmt.Errorf("textbitmap: raster dimensions do not match requested region")
			}
			scratch = tile.Pixels
			for row := 0; row < tile.Height; row++ {
				dst := bitmap.PixOffset(x-region.Min.X, y-region.Min.Y+row)
				copy(bitmap.Pixels[dst:dst+tile.Width*4], tile.Pixels[row*tile.Stride:row*tile.Stride+tile.Width*4])
			}
		}
	}
	return bitmap, nil
}

// DrawRect places a raster region without separately snapping its origin.
func DrawRect(origin geometry.Point, region image.Rectangle, scale float32) geometry.Rectangle {
	return geometry.Rect(origin.X+float32(region.Min.X)/scale, origin.Y+float32(region.Min.Y)/scale, float32(region.Dx())/scale, float32(region.Dy())/scale)
}
