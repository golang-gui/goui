package common

import "image"

// Surface is a native paint target — a Window or a Popup. It carries exactly
// what painter construction needs, so a Painter can be created for either:
//   - NativeHandle binds a GPU rendering context (OpenGL / Direct2D).
//   - Draw presents a finished image, the software painter's fallback path.
type Surface interface {
	// SurfaceState observes the current drawing target. Ready is independent of
	// visibility. Generation identifies one native target and changes when that
	// target is replaced; it is never a logical window ID.
	SurfaceState() SurfaceState
	// Transparent reports whether the surface was created with per-pixel alpha.
	// It does not describe input hit testing or guarantee a compositor remains
	// available after creation. The property is immutable.
	Transparent() bool
	// NativeHandle borrows the current native target, or returns zero while it
	// is unavailable or destroyed. Temporary target loss does not destroy the
	// logical window. A query does not extend a native object's lifetime.
	NativeHandle() uintptr
	// Draw blits a finished image onto the surface. It is the present path used
	// by the software painter. A transparent surface consumes premultiplied
	// RGBA/BGRA pixels; the image's storage is not retained after Draw returns.
	Draw(img image.Image) error

	RequestPaint() error
}

// SurfaceState describes an observed native drawing target. Generation is
// monotonic within one Surface. A lost target retains its generation until a
// replacement arrives. Final window destruction is reported separately.
type SurfaceState struct {
	Ready      bool
	Generation uint64
}

// NativeTarget is a borrowed native drawing target. A backend may expose an
// AcquireTarget method returning it when native target destruction can race
// rendering. The lease must be released on the acquiring thread before window
// destruction, including on failure. Release is idempotent. It protects the
// target's lifetime, not its visibility or the success of presentation.
type NativeTarget interface {
	NativeHandle() uintptr
	Generation() uint64
	Release()
}
