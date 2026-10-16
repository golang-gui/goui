package ui

import (
	"errors"

	"github.com/golang-gui/goui/gui"
)

// ErrClipboardTimeout means the system clipboard owner did not answer in time.
var ErrClipboardTimeout = gui.ErrClipboardTimeout

// Clipboard is the thread-safe UI-layer view of the system clipboard.
// Operations run on the UI thread automatically and preserve native errors.
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
// returns ErrAppStopped if the application has stopped.
func (c Clipboard) SetText(text string) error {
	err := error(ErrAppStopped)
	c.current.Sync(func() {
		if c.clipboard == nil {
			err = errors.New("clipboard: unavailable")
			return
		}
		err = c.clipboard.SetText(text)
	})
	return err
}

// RequestText returns ErrAppStopped without calling callback if dispatch is
// rejected during shutdown. Otherwise it returns nil and callback runs exactly
// once on the UI thread, possibly before this method returns. Read failures,
// including ErrClipboardTimeout, are delivered to callback. Empty or absent
// text is ("", nil).
func (c Clipboard) RequestText(callback func(text string, err error)) error {
	err := error(ErrAppStopped)
	c.current.Sync(func() {
		err = nil
		if c.clipboard == nil {
			callback("", errors.New("clipboard: unavailable"))
			return
		}
		c.clipboard.RequestText(callback)
	})
	return err
}
