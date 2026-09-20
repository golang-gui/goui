package x11

import (
	"errors"
	"slices"
	"unsafe"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/linux/libs/libc"
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
	style := inputStyle(p.im.QueryInputStyles())
	if style == 0 {
		return nil, errNoInputMethod
	}
	var callbacks *[4]xlib.XIMCallback
	if style&xlib.XIMPreeditCallbacks != 0 {
		callbacks = &preeditCallbacks
	}
	ic := p.im.CreateIC(win.wid, style, callbacks)
	if ic == 0 {
		return nil, errNoInputMethod
	}
	im := &inputMethod{window: win, ic: ic, handler: handler}
	inputContexts[ic] = im
	win.im = im
	return im, nil
}

// inputMethod is the x11 InputMethod: a window's XIC plus the output handler.
type inputMethod struct {
	window         *Window
	ic             xlib.XIC
	handler        common.InputMethodHandler
	enabled        bool
	spotX          int16 // last pushed candidate spot (physical px), deduped
	spotY          int16
	spotSet        bool
	preedit        []rune // XIM change/caret indexes count characters, not UTF-8 bytes
	preeditCaret   int
	preeditPending []common.InputMethodResult
	preeditLost    bool // cancellation queued by the native XIM destroy callback
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
	// Reset discards composition, including callbacks made during Xutf8ResetIC.
	im.preedit = nil
	im.preeditCaret = 0
	im.preeditPending = nil
	im.preeditLost = false
}

func (im *inputMethod) Destroy() {
	if im.ic != 0 {
		delete(inputContexts, im.ic)
		im.ic.Destroy()
		im.ic = 0
	}
	im.preedit = nil
	im.preeditPending = nil
	im.preeditLost = false
	if im.window != nil {
		im.window.im = nil
		im.window = nil
	}
}

// handleKey feeds a key-down to the IC: committed text goes to the handler; a key
// the IM did not turn into text becomes an ordinary KeyEvent.
func (im *inputMethod) handleKey(event *xlib.KeyEvent) {
	text, keysym, status := im.ic.Utf8LookupString(event)
	flushPreedit()
	if im.ic == 0 || !im.enabled || im.window == nil {
		return
	}
	if hasCommittableText(text, status) {
		im.handler(common.InputMethodResult{Kind: common.InputMethodCommit, Text: text,
			Composed: composedLookup(event.KeyCode, status)})
		return
	}

	key, location := keyFromKeysym(keysym, event.State, platform.numLockMask)
	im.window.emitEvent(events.KeyEvent{
		EventType: events.KeyDown,
		Key:       key,
		Code:      events.KeyCodeUnknown,
		Location:  location,
		Modifiers: keyModifiers(events.KeyDown, key, event.State),
		Repeat:    false,
	})
}

// libX11's XIM_COMMIT receive path queues a KeyPress with keycode 0, including
// commits that return both text and a keysym. XLookupChars separately identifies
// composed characters with no keysym. Neither decision inspects the text.
func composedLookup(keycode uint32, status xlib.Status) bool {
	return status == xlib.XLookupChars || (status == xlib.XLookupBoth && keycode == 0)
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

// Like preedit callbacks, this process-lifetime trampoline retains no Go object
// pointer in Xlib. There is one platform/display. Access is platform-thread-only.
var inputMethodDestroyCallback = xlib.XIMCallback{Callback: cgo.NewCallback(inputMethodDestroyed)}

// Native callbacks cannot notify GUI code until the enclosing Xlib call returns:
// a handler may reset/destroy a window. Hold only affected, still-owned contexts.
var lostInputContexts []*inputMethod

func inputMethodDestroyed(native xlib.XIM, _, _ uintptr) {
	if platform == nil || platform.im == 0 || platform.im != native {
		return
	}
	platform.im = 0
	for ic, im := range inputContexts {
		delete(inputContexts, ic)
		im.ic = 0 // Xlib owns destruction; never call DestroyIC on this handle.
		im.spotSet = false
		im.preedit = nil
		im.preeditCaret = 0
		im.preeditPending = nil
		if im.enabled {
			im.preeditLost = true
			lostInputContexts = append(lostInputContexts, im)
		}
		im.enabled = false // subsequent keys use the physical-key route, not XIC.
	}
}

func flushInputMethodLosses() {
	lost := lostInputContexts
	lostInputContexts = nil
	for _, im := range lost {
		if !im.preeditLost {
			continue // Reset/Destroy after the callback already consumed cancellation.
		}
		im.preeditLost = false
		if im.window != nil && im.handler != nil {
			im.handler(common.InputMethodResult{Kind: common.InputMethodPreedit})
		}
	}
}

// Accessed only on the platform thread. Four process-lifetime callbacks look
// up the live XIC, rather than retaining a Go object pointer in native storage
// or allocating another uncollectable callback for every window.
var inputContexts = map[xlib.XIC]*inputMethod{}

var preeditCallbacks = [4]xlib.XIMCallback{
	{Callback: cgo.NewCallback(preeditStart)},
	{Callback: cgo.NewCallback(preeditDone)},
	{Callback: cgo.NewCallback(preeditDraw)},
	{Callback: cgo.NewCallback(preeditCaret)},
}

func inputStyle(supported []uintptr) uintptr {
	for _, style := range []uintptr{
		xlib.XIMPreeditCallbacks | xlib.XIMStatusNothing,
		xlib.XIMPreeditCallbacks | xlib.XIMStatusNone,
		xlib.XIMPreeditNothing | xlib.XIMStatusNothing,
		xlib.XIMPreeditNothing | xlib.XIMStatusNone,
		xlib.XIMPreeditNone | xlib.XIMStatusNone,
	} {
		if slices.Contains(supported, style) {
			return style
		}
	}
	return 0
}

func preeditStart(ic xlib.XIC, _, _ uintptr) int32 {
	if im := inputContexts[ic]; im != nil {
		im.preedit = nil
		im.preeditCaret = 0
	}
	return -1 // XIM: unlimited preedit length
}

func preeditDone(ic xlib.XIC, _, _ uintptr) {
	if im := inputContexts[ic]; im != nil {
		im.preedit = nil
		im.preeditCaret = 0
		im.queuePreedit()
	}
}

func preeditDraw(ic xlib.XIC, _ uintptr, change *xlib.XIMPreeditDrawCallbackStruct) {
	if im := inputContexts[ic]; im != nil && change != nil {
		im.updatePreedit(change)
	}
}

func (im *inputMethod) updatePreedit(change *xlib.XIMPreeditDrawCallbackStruct) {
	// text == nil deletes characters; text.String == nil changes feedback
	// only. GOUI currently exposes a plain preedit string, not XIM decoration.
	if change.Text == nil || change.Text.String != nil {
		text, ok := preeditText(change.Text)
		if !ok {
			return
		}
		first := min(max(0, int(change.ChgFirst)), len(im.preedit))
		end := first + min(max(0, int(change.ChgLength)), len(im.preedit)-first)
		im.preedit = slices.Replace(im.preedit, first, end, text...)
	}
	im.preeditCaret = min(max(0, int(change.Caret)), len(im.preedit))
	im.queuePreedit()
}

func preeditText(text *xlib.XIMText) ([]rune, bool) {
	if text == nil || text.Length == 0 {
		return nil, true
	}
	if text.String == nil {
		return nil, false
	}
	if text.EncodingIsWchar != 0 {
		return slices.Clone(unsafe.Slice((*rune)(unsafe.Pointer(text.String)), int(text.Length))), true
	}
	// XIM multibyte text uses LC_CTYPE, not necessarily UTF-8. wchar_t is
	// 32-bit Unicode on Linux; conversion is bounded by XIM's character count.
	chars := make([]rune, int(text.Length))
	n := libc.Mbstowcs(chars, text.String)
	if n < 0 {
		return nil, false
	}
	return chars[:n], true
}

func preeditCaret(ic xlib.XIC, _ uintptr, caret *xlib.XIMPreeditCaretCallbackStruct) {
	im := inputContexts[ic]
	if im == nil || caret == nil {
		return
	}
	switch caret.Direction {
	case xlib.XIMForwardChar:
		im.preeditCaret++
	case xlib.XIMBackwardChar:
		im.preeditCaret--
	case xlib.XIMLineStart:
		im.preeditCaret = 0
	case xlib.XIMLineEnd:
		im.preeditCaret = len(im.preedit)
	case xlib.XIMAbsolutePosition:
		im.preeditCaret = int(caret.Position)
	}
	// For directions requiring GUI layout (e.g. a visual line), keep the
	// current position and return that actual position to XIM, as permitted by
	// the caret callback contract. Do not guess visual geometry in platform.
	im.preeditCaret = min(max(0, im.preeditCaret), len(im.preedit))
	caret.Position = int32(im.preeditCaret)
	im.queuePreedit()
}

func (im *inputMethod) queuePreedit() {
	if !im.enabled {
		return
	}
	im.preeditPending = append(im.preeditPending, common.InputMethodResult{
		Kind:  common.InputMethodPreedit,
		Text:  string(im.preedit),
		Caret: len(string(im.preedit[:im.preeditCaret])),
	})
}

func flushPreedit() {
	flushInputMethodLosses()
	for _, im := range inputContexts {
		for len(im.preeditPending) != 0 {
			result := im.preeditPending[0]
			im.preeditPending[0] = common.InputMethodResult{}
			im.preeditPending = im.preeditPending[1:]
			if im.ic != 0 && im.enabled && im.handler != nil {
				im.handler(result)
			}
			// A handler can Reset/Destroy, which clears the remaining updates.
		}
	}
}
