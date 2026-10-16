package common

import "errors"

// ErrClipboardTimeout means the selection owner did not complete a text read
// within the backend's deadline (currently five seconds on X11).
var ErrClipboardTimeout = errors.New("clipboard: read timed out")

// Clipboard is the system clipboard. It is thread-affine and must be used on
// the thread that owns the platform.
type Clipboard interface {
	// SetText replaces the clipboard contents with UTF-8 text.
	SetText(text string) error

	// RequestText requests the clipboard's text. The callback is invoked exactly
	// once on the platform thread, callers must not assume it is deferred.
	// Empty or absent text is ("", nil); errors describe read failures.
	// X11 reads require a live event loop; pending reads fail on exit.
	RequestText(callback func(text string, err error))
}
