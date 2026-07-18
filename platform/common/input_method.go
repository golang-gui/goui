package common

import "github.com/golang-gui/goui/core/geometry"

// InputMethod is a window's text-composition (IME) capability. A window's native
// input method is woven into its key handling; this interface exposes the
// controllable surface. Obtain one via Platform.NewInputMethod — not every
// platform or window supports it (the factory returns an error when it does not).
//
// It is thread-affine like the window it belongs to.
type InputMethod interface {
	// SetEnabled turns composition on/off. Enabled means a text widget is
	// focused, so keys are fed to the native IME first (producing results on the
	// handler); disabled means keys stay ordinary key events.
	SetEnabled(enabled bool)
	// SetCaretRect reports the caret rectangle in window-logical coordinates so
	// the candidate window can follow the caret.
	SetCaretRect(rect geometry.Rectangle)
	// Reset cancels any in-progress composition.
	Reset()
	// Destroy releases the native input context.
	Destroy()
}

// InputMethodKind distinguishes the two kinds of input-method output.
type InputMethodKind uint8

const (
	// InputMethodCommit: Text is committed (final) text to insert.
	InputMethodCommit InputMethodKind = iota

	// InputMethodPreedit: Text is the in-progress composition; Caret is a byte
	// offset within it; an empty Text ends the composition.
	InputMethodPreedit
)

// InputMethodResult is one piece of input-method output — committed text or a
// preedit (composition) update. It is delivered through the InputMethod's own
// handler, NOT the window event bus, so it is deliberately not an "...Event":
// you cannot receive it from Window's EventHandler.
type InputMethodResult struct {
	Kind  InputMethodKind
	Text  string
	Caret int // byte offset within Text; meaningful for InputMethodPreedit
}

// InputMethodHandler receives input-method output on the UI thread. It is a func
// to match the EventHandler convention used by Window/Popup.
type InputMethodHandler func(InputMethodResult)
