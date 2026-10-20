package typography

import "time"

// ReflowLayout optionally supports deferred changes to an already measured
// TextLayout's box. It preserves the current layout for drawing, measurement
// and input until AdvanceReflow publishes the new geometry atomically.
// Implementations remain thread-affine. This is an optimization capability;
// callers use SetSize when Reflow declines a request.
type ReflowLayout interface {
	TextLayout
	// Reflow records a box request without measuring it. A newer request cancels
	// earlier unfinished work. False means unsupported, with no state change.
	Reflow(width, height float32) bool
	// AdvanceReflow does work until deadline, or completion. The deadline is
	// soft: an individual native operation cannot be interrupted. True means
	// finished (including no pending request). Errors cancel the request and
	// leave the currently displayed layout valid. Changed fires at publication.
	AdvanceReflow(deadline time.Time) (done bool, err error)
	// CancelReflow discards unfinished work while retaining current geometry and
	// reusable measurement. It does not emit Changed.
	CancelReflow()
}
