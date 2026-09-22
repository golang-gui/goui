package graphics

import (
	"image"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/typography"
)

// Painter is thread-affine and not safe for concurrent use.
type Painter interface {
	Name() string
	// Destroy releases the painter and all of its images. It must not be
	// called between Begin and End.
	Destroy()
	// NewImage snapshots src and creates a painter-native image resource. The
	// returned image is bound to this Painter and must be destroyed when it is
	// no longer needed. It may be created inside or outside an active frame.
	// Destroy immediately invalidates it, deferring native release until End
	// when called during a frame. All calls obey Painter's thread affinity.
	NewImage(src image.Image) (Image, error)
	// RenderImage calls draw synchronously on this Painter with a transparent
	// offscreen target. width/height are physical pixels; drawing uses DIP at
	// scale pixels per DIP. It returns an independent premultiplied RGBA image.
	// It must be called outside Begin/End. draw must not call Begin, End,
	// Destroy or RenderImage; existing images remain usable, and images created
	// by draw retain their ordinary Painter ownership. No surface is presented.
	// Target state is restored even if draw panics (the panic propagates).
	// Dimensions must be positive, at most 16384 each and 64 million pixels
	// total; scale must be finite and positive. Native limits may be smaller.
	RenderImage(width, height int, scale float32, draw func()) (image.Image, error)
	// Begin starts a frame with a surface size in physical pixels. scale is the
	// number of physical pixels per DIP; subsequent drawing coordinates are DIP.
	Begin(width, height, scale float32)
	End()
	SetClipRect(rect Rectangle)
	SetTransform(transform geometry.Transform)
	Clear(color Color)
	DrawBoxShadow(rect Rectangle, radius float32, shadow BoxShadow)
	FillRect(rect Rectangle, brush Brush)
	FillRoundRect(rect Rectangle, radius float32, brush Brush)
	FillEllipse(center Point, xRadius, yRadius float32, brush Brush)
	FillPath(path Path, brush Brush)
	DrawLine(p0, p1 Point, strokeWidth float32, brush Brush)
	DrawRect(rect Rectangle, strokeWidth float32, brush Brush)
	DrawRoundRect(rect Rectangle, radius, strokeWidth float32, brush Brush)
	DrawEllipse(center Point, xRadius, yRadius, strokeWidth float32, brush Brush)
	DrawPath(path Path, strokeWidth float32, brush Brush)
	DrawTextLayout(origin Point, layout typography.TextLayout)
	DrawImage(rect Rectangle, img Image)
}
