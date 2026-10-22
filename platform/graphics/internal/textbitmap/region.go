package textbitmap

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"sort"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/utils"
)

// Rasterize native layouts in bounded blocks, assembled before composition.
// This avoids artificial antialiased edges between adjacent image quads.
const TileSize = 1024
const CacheCapacity = 4

// A shaped source supplies many visible line/color ranges at the same time.
const GlyphCacheCapacity = 256

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
	return regionForSize(w, h, origin, transform, deviceScale, rasterScale, clip)
}

func regionForSize(w, h float32, origin geometry.Point, transform geometry.Transform, deviceScale, rasterScale float32, clip image.Rectangle) image.Rectangle {
	return regionForInk(w, h, geometry.Insets{}, origin, transform, deviceScale, rasterScale, clip)
}

func regionForInk(w, h float32, pad geometry.Insets, origin geometry.Point, transform geometry.Transform, deviceScale, rasterScale float32, clip image.Rectangle) image.Rectangle {
	if clip.Empty() || rasterScale <= 0 || deviceScale <= 0 || !finitePoint(origin) {
		return image.Rectangle{}
	}
	if w < 0 || h <= 0 || !finite(float64(w)) || !finite(float64(h)) {
		return image.Rectangle{}
	}
	width, height := math.Ceil(float64(w)*float64(rasterScale)), math.Ceil(float64(h)*float64(rasterScale))
	if width > math.MaxInt32-2 || height > math.MaxInt32-2 {
		return image.Rectangle{}
	}
	edges := [4]float64{math.Floor(-float64(pad.Left)*float64(rasterScale)) - 1, math.Floor(-float64(pad.Top)*float64(rasterScale)) - 1, math.Ceil((float64(w)+float64(pad.Right))*float64(rasterScale)) + 1, math.Ceil((float64(h)+float64(pad.Bottom))*float64(rasterScale)) + 1}
	for _, edge := range edges {
		if !finite(edge) || math.Abs(edge) > math.MaxInt32-2 {
			return image.Rectangle{}
		}
	}
	bounds := image.Rect(int(edges[0]), int(edges[1]), int(edges[2]), int(edges[3]))
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
	return rasterizeRegion(func(part image.Rectangle, buf []byte) (typography.TextBitmap, error) {
		return layout.Rasterize(scale, part, buf)
	}, region, buf)
}

func rasterizeRegion(raster func(image.Rectangle, []byte) (typography.TextBitmap, error), region image.Rectangle, buf []byte) (typography.TextBitmap, error) {
	width, height, err := utils.RasterDimensions(region)
	if err != nil || width == 0 || height == 0 {
		return typography.TextBitmap{}, err
	}
	if width <= TileSize && height <= TileSize {
		return raster(region, buf)
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
			tile, err := raster(part, scratch)
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

// GlyphRegion computes visibility in range-local coordinates using ordinary
// advances. It never queries the source's potentially overflowing whole width.
func GlyphRegion(layout typography.GlyphLayout, start, length int, offset, origin geometry.Point, transform geometry.Transform, deviceScale, rasterScale float32, clip image.Rectangle) image.Rectangle {
	m, err := layout.GlyphMetrics()
	if err != nil {
		return image.Rectangle{}
	}
	first := sort.Search(len(m.Clusters), func(i int) bool { return m.Clusters[i].Start >= start })
	w := float64(0)
	for i := first; i < len(m.Clusters) && m.Clusters[i].Start < start+length; i++ {
		w += float64(m.Clusters[i].Width)
	}
	pad := m.InkPadding
	pad.Left += float32(math.Abs(float64(offset.X)) / float64(rasterScale))
	pad.Right += float32(math.Abs(float64(offset.X)) / float64(rasterScale))
	pad.Top += float32(math.Abs(float64(offset.Y)) / float64(rasterScale))
	pad.Bottom += float32(math.Abs(float64(offset.Y)) / float64(rasterScale))
	return regionForInk(float32(w), m.Line.Y+m.Line.Height, pad, origin, transform, deviceScale, rasterScale, clip)
}

// RasterizeGlyphs uses the same pixel tiling and assembly as normal text.
func RasterizeGlyphs(layout typography.GlyphLayout, start, length int, foreground color.Color, offset geometry.Point, scale float32, region image.Rectangle, buf []byte) (typography.TextBitmap, error) {
	return rasterizeRegion(func(part image.Rectangle, buf []byte) (typography.TextBitmap, error) {
		return layout.RasterizeGlyphs(start, length, foreground, offset, scale, part, buf)
	}, region, buf)
}
