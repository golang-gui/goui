package common

// Clipboard is the system clipboard. It is thread-affine and must be used on
// the thread that owns the platform.
type Clipboard interface {
	// SetText replaces the clipboard contents with UTF-8 text.
	SetText(text string) error

	// RequestText requests the clipboard's text. The callback is invoked exactly
	// once on the platform thread, callers must not assume it is deferred.
	// ok is false when there is no text or the conversion failed.
	RequestText(callback func(text string, ok bool))
}
