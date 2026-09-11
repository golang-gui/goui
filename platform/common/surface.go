package common

import "image"

// Surface is a native paint target — a Window or a Popup. It carries exactly
// what painter construction needs, so a Painter can be created for either:
//   - NativeHandle binds a GPU rendering context (OpenGL / Direct2D).
//   - Draw presents a finished image, the software painter's fallback path.
type Surface interface {
	// Transparent reports whether the surface was created with per-pixel alpha.
	// It does not describe input hit testing or guarantee a compositor remains
	// available after creation. The property is immutable.
	Transparent() bool
	// NativeHandle returns the OS handle of the surface (X11 Window / HWND /
	// NSView id).
	NativeHandle() uintptr
	// Draw blits a finished image onto the surface. It is the present path used
	// by the software painter. A transparent surface consumes premultiplied
	// RGBA/BGRA pixels; the image's storage is not retained after Draw returns.
	Draw(img image.Image) error

	RequestPaint() error
}
