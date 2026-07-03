package ui

import "github.com/golang-gui/goui/platform"

// Clipboard is the thread-safe UI-layer view of the system clipboard. It is
// always usable: operations run on the UI thread automatically, and when the
// platform clipboard is unavailable they degrade to no-ops / empty reads, so
// callers never need nil checks or thread handling.
type Clipboard struct{}

// Clipboard returns the system clipboard. It is never nil.
func (app) Clipboard() Clipboard {
	return Clipboard{}
}

// SetText replaces the clipboard contents with text. It runs on the UI thread
// and is best-effort: failures are ignored (the clipboard may be unavailable).
func (Clipboard) SetText(text string) {
	App.Sync(func() {
		if pc := currentPlatformClipboard(); pc != nil {
			_ = pc.SetText(text) // TODO: log the error once the framework has logging.
		}
	})
}

// RequestText requests the clipboard's text. The callback always runs on the UI
// thread; ok is false when there is no text or the clipboard is unavailable.
func (Clipboard) RequestText(callback func(text string, ok bool)) {
	if callback == nil {
		return
	}
	App.Sync(func() {
		pc := currentPlatformClipboard()
		if pc == nil {
			callback("", false)
			return
		}
		pc.RequestText(callback)
	})
}

func currentPlatformClipboard() platform.Clipboard {
	rt := currentAppRuntime()
	if rt == nil || rt.gui == nil {
		return nil
	}
	return rt.gui.Clipboard()
}
