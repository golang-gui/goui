package gui

import (
	"image"
	"math"
	"reflect"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
)

// IconSource is prepared icon content, independent of a Window or native Painter.
// Load, decode and validate content before supplying a source to an Icon.
// Implementations may be shared between Icons.
type IconSource interface {
	// Image supplies immutable pixels for the requested size and premultiplied
	// foreground. Icon calls it synchronously on the GUI thread with dimensions
	// in 1..4096 physical pixels, including HiDPI and transform stretch.
	//
	// The source may select existing pixels or generate and cache a new image;
	// the result need not match the requested size. Icon preserves its aspect.
	// Nil or empty output means missing content and displays a placeholder.
	// Do not load files, mutate GUI state or retain a Painter here. Keep content
	// stable until SetSource explicitly refreshes the Icon. Returned pixels must
	// stay immutable; supply a new image when pixel content changes.
	Image(width, height int, foreground Color) image.Image
}

// Color is a premultiplied drawing color, shared with the graphics backend.
type Color = graphics.Color

// Icon displays style-aware content supplied as immutable rendered images.
// It owns its image caches and does not inherit its parent's style.
type Icon struct {
	WidgetBase
	source IconSource
	size   float32
	cache  iconImageCache
}

// NewIcon creates an icon with a preferred size of 16 DIP. A nil source has
// zero natural size; a source supplying no image displays a placeholder.
// Source data can be shared; native images are per Widget.
func NewIcon(source IconSource) *Icon {
	i := &Icon{source: source, size: 16}
	i.ConnectUnmount(i.clearImageCache)
	return i
}

func (i *Icon) Source() IconSource { return i.source }

// SetSource explicitly refreshes content, even when passed the same object.
// Sources need not be comparable. Presence changes also invalidate measurement.
// The next Paint requests pixels before reusing any native image. A nil source
// releases the cache immediately.
func (i *Icon) SetSource(source IconSource) {
	contentChanged := (i.source == nil) != (source == nil)
	i.source = source
	if source == nil {
		i.clearImageCache()
	} else {
		// Keep the previous image only as a reuse candidate, never as fallback.
		i.cache.valid = false
	}
	if contentChanged {
		i.RequestLayout()
	}
	i.RequestPaint()
}

// Size returns the preferred square edge in DIP, not the allocated size.
func (i *Icon) Size() float32 { return i.size }

// SetSize sets the preferred edge. Negative and non-finite values become zero.
// Parent constraints and Widget min/max sizes still determine allocation.
func (i *Icon) SetSize(size float32) {
	size = normalizeLayoutValue(size)
	if i.size == size {
		return
	}
	i.size = size
	i.RequestLayout()
}

func (i *Icon) Measure(c layout.Constraint) layout.Measurement {
	if !i.Visible() {
		return layout.Measurement{}
	}
	var natural geometry.Size
	if i.source != nil {
		natural = geometry.Size{Width: i.size, Height: i.size}
	}
	return layout.Measured(i.constrain(c, natural))
}

func (i *Icon) Paint(p Painter) {
	if !i.Visible() || i.source == nil {
		return
	}
	bounds := i.Rect()
	edge := min(bounds.Width, bounds.Height)
	if edge <= 0 {
		return
	}
	name := i.StyleName()
	if name == "" {
		name = styleNameIcon
	}
	s := ResolveStyle(name, "", style.Normal)
	var foreground Color
	if c, ok := s.ForegroundColor(); ok && c != nil {
		foreground = graphics.ColorOf(c)
	}
	rect := geometry.Rect((bounds.Width-edge)/2, (bounds.Height-edge)/2, edge, edge)
	i.paintImage(p, rect, foreground)
}

func (i *Icon) Snapshot() WidgetInfo {
	info := i.WidgetBase.Snapshot()
	info.Role = RoleImage
	return info
}

// Only final pixels and native resources live here. Source owns any intermediate
// masks, parsed vectors or CPU caches; Icon never applies its own recoloring.
type iconImageCache struct {
	bitmap image.Image
	native graphics.Image
	key    iconImageKey
	valid  bool // Includes a cached missing result.
}

type iconImageKey struct {
	width, height int
	color         Color
}

func (i *Icon) paintImage(p Painter, rect geometry.Rectangle, foreground Color) {
	scale := p.PixelScale()
	if scale <= 0 {
		return
	}
	w, h := math.Ceil(float64(rect.Width)*float64(scale)), math.Ceil(float64(rect.Height)*float64(scale))
	if w < 1 || h < 1 || w > 4096 || h > 4096 || math.IsNaN(w) || math.IsNaN(h) {
		i.drawMissing(p, rect, foreground)
		return
	}
	key := iconImageKey{width: int(w), height: int(h), color: foreground}
	if !i.cache.valid || i.cache.key != key {
		bitmap := i.source.Image(key.width, key.height, foreground)
		if bitmap != nil && bitmap.Bounds().Empty() {
			bitmap = nil
		}
		// A source can select the same pixels for different requests. Compare
		// only comparable values; custom image implementations may contain slices.
		same := bitmap != nil && i.cache.bitmap != nil &&
			reflect.ValueOf(bitmap).Comparable() && reflect.ValueOf(i.cache.bitmap).Comparable() &&
			bitmap == i.cache.bitmap
		if !same {
			i.releaseNativeImage()
		}
		i.cache.bitmap, i.cache.key, i.cache.valid = bitmap, key, true
	}
	if i.cache.bitmap == nil {
		i.drawMissing(p, rect, foreground)
		return
	}
	if i.cache.native == nil {
		native, err := p.NewImage(i.cache.bitmap)
		if err != nil {
			// Retain pixels for a later upload attempt without regenerating them.
			i.drawMissing(p, rect, foreground)
			return
		}
		i.cache.native = native
	}
	i.drawImage(p, rect)
}

// The placeholder uses ordinary primitives, not another fallible image upload.
func (i *Icon) drawMissing(p Painter, rect geometry.Rectangle, foreground Color) {
	edge := min(rect.Width, rect.Height)
	inset, stroke := edge/8, edge/16
	left, top := rect.X+inset, rect.Y+inset
	right, bottom := rect.X+rect.Width-inset, rect.Y+rect.Height-inset
	p.DrawRect(geometry.Rect(left, top, right-left, bottom-top), stroke, foreground)
	p.DrawLine(geometry.Point{X: left, Y: top}, geometry.Point{X: right, Y: bottom}, stroke, foreground)
	p.DrawLine(geometry.Point{X: right, Y: top}, geometry.Point{X: left, Y: bottom}, stroke, foreground)
}

func (i *Icon) drawImage(p Painter, rect geometry.Rectangle) {
	w, h := i.cache.native.Size()
	scale := min(rect.Width/float32(w), rect.Height/float32(h))
	width, height := float32(w)*scale, float32(h)*scale
	p.DrawImage(geometry.Rect(rect.X+(rect.Width-width)/2, rect.Y+(rect.Height-height)/2, width, height), i.cache.native)
}

func (i *Icon) releaseNativeImage() {
	if old := i.cache.native; old != nil {
		i.cache.native = nil
		old.Destroy()
	}
}

func (i *Icon) clearImageCache() {
	i.releaseNativeImage()
	i.cache = iconImageCache{}
}
