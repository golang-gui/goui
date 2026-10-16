package gui

import (
	"errors"

	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/common"
)

// ErrClipboardTimeout means the system clipboard owner did not answer in time.
var ErrClipboardTimeout = common.ErrClipboardTimeout

type Clipboard interface {
	// SetText replaces the clipboard contents with UTF-8 text.
	SetText(text string) error

	// RequestText requests the clipboard's text. The callback is invoked exactly
	// once on the platform thread, callers must not assume it is deferred.
	// Empty or absent text is ("", nil); errors describe read failures.
	RequestText(callback func(text string, err error))
}

type clipboard struct {
	clipboard platform.Clipboard
}

func newClipboard(platClip platform.Clipboard) clipboard {
	return clipboard{clipboard: platClip}
}

func (c clipboard) SetText(text string) error {
	if c.clipboard != nil {
		return c.clipboard.SetText(text)
	}
	return errors.New("clipboard: unavailable")
}

func (c clipboard) RequestText(callback func(text string, err error)) {
	if c.clipboard != nil {
		c.clipboard.RequestText(callback)
	} else {
		callback("", errors.New("clipboard: unavailable"))
	}
}
