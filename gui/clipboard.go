package gui

import "github.com/golang-gui/goui/platform"

type Clipboard interface {
	// SetText replaces the clipboard contents with UTF-8 text.
	SetText(text string)

	// RequestText requests the clipboard's text. The callback is invoked exactly
	// once on the platform thread, callers must not assume it is deferred.
	// ok is false when there is no text or the conversion failed.
	RequestText(callback func(text string, ok bool))
}

type clipboard struct {
	clipboard platform.Clipboard
}

func newClipboard(platClip platform.Clipboard) clipboard {
	return clipboard{clipboard: platClip}
}

func (c clipboard) SetText(text string) {
	if c.clipboard != nil {
		_ = c.clipboard.SetText(text) // TODO: log the error once the framework has logging.
	}
}

func (c clipboard) RequestText(callback func(text string, ok bool)) {
	if c.clipboard != nil {
		c.clipboard.RequestText(callback)
	} else {
		callback("", false)
	}
}
