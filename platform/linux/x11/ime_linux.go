package x11

import (
	"errors"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"
)

var errNoInputMethod = errors.New("x11: no input method available")

// newInputMethod creates the x11 input method for window.
func (p *Platform) newInputMethod(window common.Window, handler common.InputMethodHandler) (*inputMethod, error) {
	if p.im == 0 {
		return nil, errNoInputMethod
	}
	win, ok := window.(*Window)
	if !ok {
		return nil, errNoInputMethod
	}
	ic := p.im.CreateIC(win.wid)
	if ic == 0 {
		return nil, errNoInputMethod
	}
	im := &inputMethod{window: win, ic: ic, handler: handler}
	win.im = im
	return im, nil
}

// inputMethod is the x11 InputMethod: a window's XIC plus the output handler.
type inputMethod struct {
	window  *Window
	ic      xlib.XIC
	handler common.InputMethodHandler
	enabled bool
	spotX   int16 // last pushed candidate spot (physical px), deduped
	spotY   int16
	spotSet bool
}

func (im *inputMethod) SetEnabled(enabled bool) {
	if im.ic == 0 || im.enabled == enabled {
		return
	}
	im.enabled = enabled
	im.spotSet = false
	if enabled {
		im.ic.SetFocus()
	} else {
		im.ic.UnsetFocus()
	}
}

func (im *inputMethod) SetCaretRect(rect geometry.Rectangle) {
	if im.ic == 0 || !im.enabled {
		return
	}
	scale := currentScale()
	x := int16(rect.X * scale)
	y := int16((rect.Y + rect.Height) * scale)
	if im.spotSet && x == im.spotX && y == im.spotY {
		return
	}
	im.spotX, im.spotY, im.spotSet = x, y, true
	im.ic.SetSpot(x, y)
}

func (im *inputMethod) Reset() {
	im.ic.ResetIC()
}

func (im *inputMethod) Destroy() {
	if im.ic != 0 {
		im.ic.Destroy()
		im.ic = 0
	}
	if im.window != nil {
		im.window.im = nil
		im.window = nil
	}
}

// handleKey feeds a key-down to the IC: committed text goes to the handler; a key
// the IM did not turn into text becomes an ordinary KeyEvent.
func (im *inputMethod) handleKey(event *xlib.KeyEvent) {
	text, keysym, status := im.ic.Utf8LookupString(event)
	if hasCommittableText(text, status) {
		im.handler(common.InputMethodResult{Kind: common.InputMethodCommit, Text: text})
		return
	}

	key, location := keyFromKeysym(keysym, event.State, platform.numLockMask)
	im.window.onEvent(events.KeyEvent{
		EventType: events.KeyDown,
		Key:       key,
		Code:      events.KeyCodeUnknown,
		Location:  location,
		Modifiers: keyModifiers(events.KeyDown, key, event.State),
		Repeat:    false,
	})
}

// hasCommittableText reports whether the lookup produced real text. Control
// characters (Enter -> "\r", Tab, Backspace, Ctrl+letter, ...) are not text;
// those keys fall through to a KeyEvent.
func hasCommittableText(text string, status xlib.Status) bool {
	if status != xlib.XLookupChars && status != xlib.XLookupBoth {
		return false
	}
	for _, r := range text {
		if r >= 0x20 && r != 0x7f {
			return true
		}
	}
	return false
}
