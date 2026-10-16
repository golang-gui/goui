package x11

import (
	"errors"
	"time"
	"unicode/utf8"
	"unsafe"

	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"

	"github.com/goexlib/cgo"
)

// clipboard implements the X11 CLIPBOARD selection. Its owner window
// (platform.helper) and atoms (platform.atoms) are process-global platform
// resources; selection events are routed here by handleEvent.
type clipboard struct {
	text    string // value we currently offer to other clients
	hasText bool
	pending *clipboardRead
	closed  bool
}

const clipboardReadTimeout = 5 * time.Second
const clipboardReadLimit = 16 << 20 // maximum non-INCR read, in bytes

type clipboardRead struct {
	callback   func(string, error)
	requestor  xlib.Window
	deadline   time.Time
	target     xlib.Atom
	advertised bool
}

func newClipboard() *clipboard {
	return &clipboard{}
}

func (c *clipboard) SetText(text string) error {
	if c.closed {
		return common.ErrUnavailable
	}
	c.text = text
	c.hasText = true
	// CurrentTime (0); v1 does not track the triggering event's server time.
	platform.display.SetSelectionOwner(platform.atoms.CLIPBOARD, platform.helper, 0)
	platform.display.Flush()
	if platform.display.GetSelectionOwner(platform.atoms.CLIPBOARD) != platform.helper {
		c.hasText = false
		return errors.New("clipboard: could not acquire selection ownership")
	}
	return nil
}

func (c *clipboard) RequestText(callback func(text string, err error)) {
	if c.closed || platform.eventLoop == nil || platform.eventLoop.state.Quitting() {
		callback("", common.ErrUnavailable)
		return
	}
	d := platform.display
	if c.pending != nil {
		callback("", errors.New("clipboard: another read is pending"))
		return
	}
	if c.hasText && d.GetSelectionOwner(platform.atoms.CLIPBOARD) == platform.helper {
		callback(c.text, nil)
		return
	}
	if d.GetSelectionOwner(platform.atoms.CLIPBOARD) == 0 {
		callback("", nil)
		return
	}
	// A fresh requestor identifies each read, including late replies after a
	// timeout. Reusing the helper/property would let an old reply finish a new read.
	w := d.CreateWindow(platform.defScreen.Root, 0, 0, 1, 1, 0,
		int(platform.defScreen.RootDepth), xlib.WindowClassInputOutput, platform.defScreen.RootVisual, 0, nil)
	if w == 0 {
		callback("", common.ErrUnavailable)
		return
	}
	c.pending = &clipboardRead{callback: callback, requestor: w, deadline: time.Now().Add(clipboardReadTimeout)}
	c.convert(platform.atoms.TARGETS)
}

func (c *clipboard) convert(target xlib.Atom) {
	r := c.pending
	r.target = target
	platform.display.ConvertSelection(platform.atoms.CLIPBOARD, target, platform.atoms.GOUI_CLIPBOARD, r.requestor, 0)
	platform.display.Flush()
}

func (c *clipboard) finish(text string, err error) {
	r := c.pending
	if r == nil {
		return
	}
	c.pending = nil
	// Release all read state before calling application code, which may reenter.
	if r.requestor != 0 {
		platform.display.DestroyWindow(r.requestor)
		platform.display.Flush()
	}
	r.callback(text, err)
}

func (c *clipboard) expire(now time.Time) {
	if c.pending != nil && !now.Before(c.pending.deadline) {
		c.finish("", common.ErrClipboardTimeout)
	}
}

// pollTimeout returns a rounded-up deadline in milliseconds, or -1 for no read.
func (c *clipboard) pollTimeout(now time.Time) int {
	if c.pending == nil {
		return -1
	}
	remaining := c.pending.deadline.Sub(now)
	if remaining <= 0 {
		return 0
	}
	return int((remaining + time.Millisecond - 1) / time.Millisecond)
}

func (c *clipboard) close() {
	c.closed = true
	c.finish("", common.ErrUnavailable)
}

// handleSelectionRequest answers another client pasting our value.
func (c *clipboard) handleSelectionRequest(ev *xlib.SelectionRequestEvent) {
	if ev.Selection != platform.atoms.CLIPBOARD {
		return
	}
	d := platform.display

	property := ev.Property
	if property == 0 { // obsolete requestor
		property = ev.Target
	}

	switch {
	case !c.hasText:
		property = 0
	case ev.Target == platform.atoms.TARGETS:
		list := []xlib.Atom{platform.atoms.TARGETS, platform.atoms.UTF8_STRING}
		if _, ok := clipboardLatin1(c.text); ok {
			list = append(list, xlib.AtomString)
		}
		d.ChangeProperty(ev.Requestor, property, xlib.AtomAtom, 32, xlib.PropModeReplace, cgo.CSlice(list), len(list))
	case ev.Target == platform.atoms.UTF8_STRING || ev.Target == xlib.AtomString:
		typ := platform.atoms.UTF8_STRING
		if ev.Target == xlib.AtomString {
			typ = xlib.AtomString
		}
		data := []byte(c.text)
		if ev.Target == xlib.AtomString {
			var ok bool
			data, ok = clipboardLatin1(c.text)
			if !ok {
				property = 0
				break
			}
		}
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
	r := c.pending
	if r == nil || ev.Requestor != r.requestor || ev.Selection != platform.atoms.CLIPBOARD || ev.Target != r.target {
		return
	}
	if ev.Property == 0 {
		// Older owners may not implement TARGETS. Try the two supported encodings.
		if !r.advertised && r.target == platform.atoms.TARGETS {
			c.convert(platform.atoms.UTF8_STRING)
			return
		}
		if !r.advertised && r.target == platform.atoms.UTF8_STRING {
			c.convert(xlib.AtomString)
			return
		}
		if r.advertised {
			c.finish("", errors.New("clipboard: advertised conversion refused"))
		} else {
			c.finish("", nil)
		}
		return
	}
	if ev.Property != platform.atoms.GOUI_CLIPBOARD {
		return
	}
	var typ xlib.Atom
	var format int32
	var count, remaining uint
	var data *byte
	status := platform.display.GetWindowProperty(r.requestor, ev.Property, 0, clipboardReadLimit/4, true, xlib.AtomAny,
		&typ, &format, &count, &remaining, &data)
	if data != nil {
		defer xlib.Free(data)
	}
	if status != 0 || remaining != 0 || (count != 0 && data == nil) {
		c.finish("", errors.New("clipboard: incomplete or oversized property"))
		return
	}
	if r.target == platform.atoms.TARGETS {
		if typ != xlib.AtomAtom || format != 32 || count > clipboardReadLimit/4 {
			c.finish("", errors.New("clipboard: invalid TARGETS property"))
			return
		}
		targets := unsafe.Slice((*xlib.Atom)(unsafe.Pointer(data)), int(count))
		var target xlib.Atom
		for _, candidate := range targets {
			if candidate == platform.atoms.UTF8_STRING {
				target = candidate
				break
			}
			if candidate == xlib.AtomString {
				target = candidate
			}
		}
		if target == 0 {
			c.finish("", nil)
			return
		}
		r.advertised = true
		c.convert(target)
		return
	}
	// INCR (format 32) and all unexpected types are explicit failures. Never
	// interpret packed/native-long property data as UTF-8 bytes.
	if typ != r.target || format != 8 || count > clipboardReadLimit {
		c.finish("", errors.New("clipboard: unsupported selection conversion"))
		return
	}
	bytes := unsafe.Slice(data, int(count))
	if typ == xlib.AtomString {
		runes := make([]rune, len(bytes))
		for i, b := range bytes {
			runes[i] = rune(b)
		}
		c.finish(string(runes), nil)
	} else if !utf8.Valid(bytes) {
		c.finish("", errors.New("clipboard: invalid UTF-8 text"))
	} else {
		c.finish(string(bytes), nil)
	}
}

func clipboardLatin1(text string) ([]byte, bool) {
	data := make([]byte, 0, len(text))
	for _, r := range text {
		if r > 255 {
			return nil, false
		}
		data = append(data, byte(r))
	}
	return data, true
}

func (c *clipboard) handleSelectionClear(ev *xlib.SelectionClearEvent) {
	if ev.Selection == platform.atoms.CLIPBOARD {
		c.hasText = false
		c.text = ""
	}
}
