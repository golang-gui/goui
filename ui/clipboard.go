package ui

import "github.com/golang-gui/goui/gui"

// Clipboard is the thread-safe UI-layer view of the system clipboard.
// It is always usable: operations run on the UI thread
// automatically, and when the platform clipboard is unavailable they degrade to
// no-ops / empty reads, so callers never need nil checks or thread handling.
type Clipboard struct {
	current   *app
	clipboard gui.Clipboard
}

// Clipboard returns the system clipboard view. It is never nil.
func (a *app) Clipboard() Clipboard {
	return Clipboard{
		current:   a,
		clipboard: a.gui.Clipboard(),
	}
}

// SetText replaces the clipboard contents with text. It runs on the UI thread and
// is best-effort: failures are ignored (the clipboard may be unavailable).
func (c Clipboard) SetText(text string) {
	c.current.Sync(func() {
		c.clipboard.SetText(text)
	})
}

// RequestText requests the clipboard's text. The callback always runs on the UI
// thread; ok is false when there is no text or the clipboard is unavailable.
func (c Clipboard) RequestText(callback func(text string, ok bool)) {
	c.current.Sync(func() {
		c.clipboard.RequestText(callback)
	})
}
