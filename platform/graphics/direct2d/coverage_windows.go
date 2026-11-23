package direct2d

import (
	"image"
	"math"
	"unsafe"

	"github.com/golang-gui/goui/platform/windows/sdk/d2d1"
	"github.com/golang-gui/goui/platform/windows/sdk/dxgi"
)

// Coverage is sampled in device pixels, independently of the content brush.
// Three exact 2:1 bilinear reductions average the 8x8 samples without a blur
// kernel. Only curved geometry uses this path; text and images stay native.
const coverageSamples = 8
const coverageTileSize = 128
const coverageCacheCapacity = 256
const coverageCacheBytes = 4 << 20

type coverageKey struct {
	geometry *d2d1.Geometry
	matrix   d2d1.Matrix3x2F
	width    float32
	size     image.Point
}

type coverageEntry struct {
	key    coverageKey
	bitmap *d2d1.Bitmap1
	age    uint64
}

// Scratch targets and cached masks belong to this Painter's D2D device. Masks
// are immutable, include subpixel placement, and hold a geometry reference.
// Device loss and Painter destruction release them before their D2D device.
type geometryCoverage struct {
	render  *d2d1.DeviceContext
	brush   *d2d1.SolidColorBrush
	scratch [3]*d2d1.Bitmap1
	entries []coverageEntry
	clock   uint64
	bytes   int
}

func (c *geometryCoverage) release() {
	for _, entry := range c.entries {
		entry.bitmap.Release()
		entry.key.geometry.Release()
	}
	c.entries = nil
	c.bytes, c.clock = 0, 0
	if c.render != nil {
		c.render.SetTarget(nil)
	}
	for i, bitmap := range c.scratch {
		if bitmap != nil {
			bitmap.Release()
			c.scratch[i] = nil
		}
	}
	if c.brush != nil {
		c.brush.Release()
		c.brush = nil
	}
	if c.render != nil {
		c.render.Release()
		c.render = nil
	}
}

func (p *Painter) prepareCoverage() bool {
	c := &p.coverage
	if c.render != nil {
		return true
	}
	ready := false
	defer func() {
		if !ready {
			c.release()
		}
	}()
	context, hr := p.d2dDevice.CreateDeviceContext(d2d1.D2D1_DEVICE_CONTEXT_OPTIONS_NONE)
	if hr.Failed() {
		return false
	}
	c.render = context
	white := d2d1.ColorF{R: 1, G: 1, B: 1, A: 1}
	c.brush, hr = context.CreateSolidColorBrush(&white, nil)
	if hr.Failed() {
		return false
	}
	for i := range c.scratch {
		size := uint32(coverageTileSize * coverageSamples >> i)
		c.scratch[i], hr = context.CreateBitmap1(d2d1.SizeU{Width: size, Height: size}, nil, 0, coverageBitmapProperties())
		if hr.Failed() {
			return false
		}
	}
	ready = true
	return true
}

func coverageBitmapProperties() *d2d1.BitmapProperties1 {
	return &d2d1.BitmapProperties1{
		PixelFormat: d2d1.PixelFormat{Format: dxgi.DXGI_FORMAT_B8G8R8A8_UNORM, AlphaMode: d2d1.D2D1_ALPHA_MODE_PREMULTIPLIED},
		DpiX:        96, DpiY: 96, BitmapOptions: d2d1.D2D1_BITMAP_OPTIONS_TARGET,
	}
}

func (p *Painter) coverageMask(key coverageKey) *d2d1.Bitmap1 {
	c := &p.coverage
	c.clock++
	for i := range c.entries {
		if c.entries[i].key == key {
			c.entries[i].age = c.clock
			return c.entries[i].bitmap
		}
	}
	mask, hr := c.render.CreateBitmap1(d2d1.SizeU{Width: uint32(key.size.X), Height: uint32(key.size.Y)}, nil, 0, coverageBitmapProperties())
	if hr.Failed() {
		return nil
	}
	keep := false
	defer func() {
		if !keep {
			mask.Release()
		}
	}()
	context := c.render
	identity := d2d1.Matrix3x2F{M11: 1, M22: 1}
	context.SetTransform(&identity)
	context.SetTarget((*d2d1.Image)(unsafe.Pointer(c.scratch[0])))
	context.BeginDraw()
	region := d2d1.RectF{Right: float32(key.size.X * coverageSamples), Bottom: float32(key.size.Y * coverageSamples)}
	context.PushAxisAlignedClip(&region, d2d1.D2D1_ANTIALIAS_MODE_ALIASED)
	context.Clear(&d2d1.ColorF{})
	context.SetTransform(&key.matrix)
	if key.width > 0 {
		context.DrawGeometry(key.geometry, &c.brush.Brush, key.width, nil)
	} else {
		context.FillGeometry(key.geometry, &c.brush.Brush, nil)
	}
	context.PopAxisAlignedClip()
	context.SetTransform(&identity)
	for i, source := range c.scratch {
		target := mask
		if i+1 < len(c.scratch) {
			target = c.scratch[i+1]
		}
		samples := float32(int(coverageSamples) >> i)
		src := d2d1.RectF{Right: float32(key.size.X) * samples, Bottom: float32(key.size.Y) * samples}
		dst := d2d1.RectF{Right: src.Right / 2, Bottom: src.Bottom / 2}
		context.SetTarget((*d2d1.Image)(unsafe.Pointer(target)))
		context.PushAxisAlignedClip(&dst, d2d1.D2D1_ANTIALIAS_MODE_ALIASED)
		context.Clear(&d2d1.ColorF{})
		context.DrawBitmap(&source.Bitmap, &dst, 1, d2d1.D2D1_BITMAP_INTERPOLATION_MODE_LINEAR, &src)
		context.PopAxisAlignedClip()
	}
	hr = context.EndDraw(nil, nil)
	context.SetTarget(nil)
	if hr.Failed() {
		return nil
	}
	key.geometry.AddRef()
	entry := coverageEntry{key: key, bitmap: mask, age: c.clock}
	bytes := key.size.X * key.size.Y * 4
	for len(c.entries) >= coverageCacheCapacity || c.bytes+bytes > coverageCacheBytes {
		oldest := 0
		for i := range c.entries {
			if c.entries[i].age < c.entries[oldest].age {
				oldest = i
			}
		}
		c.entries[oldest].bitmap.Release()
		c.entries[oldest].key.geometry.Release()
		c.bytes -= c.entries[oldest].key.size.X * c.entries[oldest].key.size.Y * 4
		c.entries[oldest] = c.entries[len(c.entries)-1]
		c.entries = c.entries[:len(c.entries)-1]
	}
	c.entries = append(c.entries, entry)
	c.bytes += bytes
	keep = true
	return mask
}

func (p *Painter) drawCoverage(geometry *d2d1.Geometry, brush *d2d1.Brush, width float32, erase bool) bool {
	if !(p.scale > 0) || math.IsInf(float64(p.scale), 0) {
		return false
	}
	bounds, hr := geometry.GetBounds(&p.matrix)
	if width > 0 {
		bounds, hr = geometry.GetWidenedBounds(width, nil, &p.matrix, p.curveTolerance())
	}
	if hr.Failed() {
		return false
	}
	values := [...]float32{bounds.Left, bounds.Top, bounds.Right, bounds.Bottom}
	for _, value := range values {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return false
		}
	}
	size := p.activeSize
	left, top := max(0, float64(bounds.Left*p.scale)), max(0, float64(bounds.Top*p.scale))
	right, bottom := min(float64(size.X), float64(bounds.Right*p.scale)), min(float64(size.Y), float64(bounds.Bottom*p.scale))
	if p.clipActive {
		left, top = max(left, float64(p.clip.Left*p.scale)), max(top, float64(p.clip.Top*p.scale))
		right, bottom = min(right, float64(p.clip.Right*p.scale)), min(bottom, float64(p.clip.Bottom*p.scale))
	}
	if left >= right || top >= bottom {
		return true
	}
	if !p.prepareCoverage() {
		return false
	}
	type tile struct {
		bitmap *d2d1.Bitmap1
		bounds image.Rectangle
	}
	var tiles []tile
	defer func() {
		for _, item := range tiles {
			item.bitmap.Release()
		}
	}()
	// Prepare every tile before touching the destination. Allocation failure
	// can then fall back without drawing any translucent region twice.
	for y := int(math.Floor(top)); y < int(math.Ceil(bottom)); y += coverageTileSize {
		for x := int(math.Floor(left)); x < int(math.Ceil(right)); x += coverageTileSize {
			r := image.Rect(x, y, min(x+coverageTileSize, int(math.Ceil(right))), min(y+coverageTileSize, int(math.Ceil(bottom))))
			matrix := p.matrix
			factor := p.scale * coverageSamples
			matrix.M11 *= factor
			matrix.M12 *= factor
			matrix.M21 *= factor
			matrix.M22 *= factor
			matrix.M31 = (matrix.M31*p.scale - float32(x)) * coverageSamples
			matrix.M32 = (matrix.M32*p.scale - float32(y)) * coverageSamples
			mask := p.coverageMask(coverageKey{geometry: geometry, matrix: matrix, width: width, size: r.Size()})
			if mask == nil {
				return false
			}
			mask.AddRef()
			tiles = append(tiles, tile{bitmap: mask, bounds: r})
		}
	}
	oldTransform, oldAA := p.render.GetTransform(), p.render.GetAntialiasMode()
	identity := d2d1.Matrix3x2F{M11: 1, M22: 1}
	p.render.SetTransform(&identity)
	p.render.SetAntialiasMode(d2d1.D2D1_ANTIALIAS_MODE_ALIASED)
	defer p.render.SetTransform(&oldTransform)
	defer p.render.SetAntialiasMode(oldAA)
	if brush != nil {
		oldBrush := brush.GetTransform()
		brush.SetTransform(&p.matrix)
		defer brush.SetTransform(&oldBrush)
	}
	for _, item := range tiles {
		r := item.bounds
		source := d2d1.RectF{Right: float32(r.Dx()), Bottom: float32(r.Dy())}
		if erase {
			// COPY would clear the mask's transparent bounding rectangle too.
			// Destination-out uses only the mask alpha as erased coverage.
			offset := d2d1.Point2F{X: float32(r.Min.X), Y: float32(r.Min.Y)}
			physical := d2d1.Matrix3x2F{M11: 1 / p.scale, M22: 1 / p.scale}
			p.render.SetTransform(&physical)
			p.render.DrawImage((*d2d1.Image)(unsafe.Pointer(item.bitmap)), &offset, &source, d2d1.D2D1_INTERPOLATION_MODE_LINEAR, d2d1.D2D1_COMPOSITE_MODE_DESTINATION_OUT)
			p.render.SetTransform(&identity)
		} else {
			destination := d2d1.RectF{Left: float32(r.Min.X) / p.scale, Top: float32(r.Min.Y) / p.scale, Right: float32(r.Max.X) / p.scale, Bottom: float32(r.Max.Y) / p.scale}
			p.render.FillOpacityMask(&item.bitmap.Bitmap, brush, &destination, &source)
		}
	}
	return true
}
