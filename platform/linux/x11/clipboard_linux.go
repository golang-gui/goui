package x11

import (
	"github.com/golang-gui/goui/platform/linux/libs/xlib"

	"github.com/goexlib/cgo"
)

// clipboard implements the X11 CLIPBOARD selection. Its owner window
// (platform.helper) and atoms (platform.atoms) are process-global platform
// resources; selection events are routed here by handleEvent.
type clipboard struct {
	text    string // value we currently offer to other clients
	hasText bool
	pending func(text string, ok bool) // in-flight RequestText callback
}

func newClipboard() *clipboard {
	return &clipboard{}
}

func (c *clipboard) SetText(text string) error {
	c.text = text
	c.hasText = true
	// CurrentTime (0); v1 does not track the triggering event's server time.
	platform.display.SetSelectionOwner(platform.atoms.CLIPBOARD, platform.helper, 0)
	platform.display.Flush()
	return nil
}

func (c *clipboard) RequestText(callback func(text string, ok bool)) {
	d := platform.display
	// Fast path: we already own the clipboard.
	if c.hasText && d.GetSelectionOwner(platform.atoms.CLIPBOARD) == platform.helper {
		callback(c.text, true)
		return
	}
	if c.pending != nil {
		callback("", false)
		return
	}
	c.pending = callback
	d.ConvertSelection(platform.atoms.CLIPBOARD, platform.atoms.UTF8_STRING, platform.atoms.GOUI_CLIPBOARD, platform.helper, 0)
	d.Flush()
}

// handleSelectionRequest answers another client pasting our value.
func (c *clipboard) handleSelectionRequest(ev *xlib.SelectionRequestEvent) {
	d := platform.display

	property := ev.Property
	if property == 0 { // obsolete requestor
		property = ev.Target
	}

	switch {
	case !c.hasText:
		property = 0
	case ev.Target == platform.atoms.TARGETS:
		list := []xlib.Atom{platform.atoms.TARGETS, platform.atoms.UTF8_STRING, xlib.AtomString}
		d.ChangeProperty(ev.Requestor, property, xlib.AtomAtom, 32, xlib.PropModeReplace, cgo.CSlice(list), len(list))
	case ev.Target == platform.atoms.UTF8_STRING || ev.Target == xlib.AtomString:
		typ := platform.atoms.UTF8_STRING
		if ev.Target == xlib.AtomString {
			typ = xlib.AtomString
		}
		data := []byte(c.text)
		d.ChangeProperty(ev.Requestor, property, typ, 8, xlib.PropModeReplace, cgo.CSlice(data), len(data))
	default:
		property = 0 // refuse
	}

	var reply xlib.Event
	sel := reply.SelectionEvent()
	sel.Type = xlib.SelectionNotify
	sel.Requestor = ev.Requestor
	sel.Selection = ev.Selection
	sel.Target = ev.Target
	sel.Property = property
	sel.Time = ev.Time
	d.SendEvent(ev.Requestor, false, 0, &reply)
	d.Flush()
}

// handleSelectionNotify completes an in-flight RequestText.
func (c *clipboard) handleSelectionNotify(ev *xlib.SelectionEvent) {
	cb := c.pending
	c.pending = nil
	if cb == nil {
		return
	}
	if ev.Property == 0 { // conversion refused
		cb("", false)
		return
	}
	data, _ := platform.display.GetWindowPropertyBytes(platform.helper, ev.Property, 0)
	platform.display.DeleteProperty(platform.helper, ev.Property)
	cb(string(data), true)
}

func (c *clipboard) handleSelectionClear(ev *xlib.SelectionClearEvent) {
	if ev.Selection == platform.atoms.CLIPBOARD {
		c.hasText = false
		c.text = ""
	}
}
