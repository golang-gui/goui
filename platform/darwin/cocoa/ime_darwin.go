package cocoa

import (
	"errors"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"

	. "github.com/golang-gui/goui/platform/darwin/frameworks/appkit"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_graphics"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
)

// inputMethod is the cocoa InputMethod. The content view is an NSTextInputClient
// (registered by appkit via the NSViewOverride text-input funcs below); the
// protocol methods find this per-window state via windowForView(self).
type inputMethod struct {
	window       *Window
	handler      common.InputMethodHandler
	enabled      bool
	marked       string  // current preedit
	caret        NSRect  // caret rect in screen coordinates
	pending      NSEvent // key being interpreted, re-emitted from doCommandBySelector
	pendingValid bool
}

// newInputMethod creates the cocoa input method for window.
func newInputMethod(window common.Window, handler common.InputMethodHandler) (*inputMethod, error) {
	win, ok := window.(*Window)
	if !ok {
		return nil, errors.New("cocoa: invalid window for input method")
	}
	im := &inputMethod{window: win, handler: handler}
	win.im = im
	return im, nil
}

func (im *inputMethod) SetEnabled(enabled bool) {
	if im.enabled == enabled {
		return
	}
	im.enabled = enabled
	if im.window != nil {
		if ctx := im.window.view.InputContext(); ctx.Valid() {
			if enabled {
				ctx.Activate()
			} else {
				ctx.Deactivate()
			}
		}
	}
	if !enabled {
		im.discardMarked()
	}
}

// SetCaretRect converts the caret rect from window-logical (top-left) coordinates
// to screen coordinates for the candidate window. The content view is not
// flipped, so Y is flipped against the view height. (Candidate placement may need
// tuning on-device.)
func (im *inputMethod) SetCaretRect(rect geometry.Rectangle) {
	if im.window == nil || !im.enabled {
		return
	}
	view := im.window.view
	vb := view.Bounds()
	local := NSMakeRect(
		CGFloat(rect.X),
		vb.Size.Height-CGFloat(rect.Y)-CGFloat(rect.Height),
		CGFloat(rect.Width),
		CGFloat(rect.Height),
	)
	im.caret = im.window.window.ConvertRectToScreen(view.ConvertRectToWindow(local))
}

func (im *inputMethod) Reset() {
	im.discardMarked()
}

func (im *inputMethod) Destroy() {
	if im.window != nil {
		im.window.im = nil
		im.window = nil
	}
}

// interpret routes a key press through the input context (called from keyDown
// while a text widget is focused). Text becomes insertText/setMarkedText; other
// keys come back via doCommandBySelector and are re-emitted as KeyEvents.
func (im *inputMethod) interpret(view NSView, event NSEvent) {
	im.pending = event
	im.pendingValid = true
	if ctx := view.InputContext(); ctx.Valid() {
		ctx.HandleEvent(event)
	}
	im.pendingValid = false
}

func (im *inputMethod) discardMarked() {
	if im.window != nil {
		if ctx := im.window.view.InputContext(); ctx.Valid() {
			ctx.DiscardMarkedText()
		}
	}
	if im.marked != "" {
		im.marked = ""
		if im.handler != nil {
			im.handler(common.InputMethodResult{Kind: common.InputMethodPreedit})
		}
	}
}

// --- NSTextInputClient method implementations (wired via the NSViewOverride) ---

func imForView(self NSView) *inputMethod {
	if w := windowForView(self); w != nil {
		return w.im
	}
	return nil
}

func imInsertText(self NSView, text string, replace NSRange) {
	im := imForView(self)
	if im == nil {
		return
	}
	im.marked = ""
	if im.handler != nil {
		im.handler(common.InputMethodResult{Kind: common.InputMethodCommit, Text: text})
	}
}

func imSetMarkedText(self NSView, text string, selected, replace NSRange) {
	im := imForView(self)
	if im == nil {
		return
	}
	im.marked = text
	if im.handler != nil {
		im.handler(common.InputMethodResult{
			Kind:  common.InputMethodPreedit,
			Text:  text,
			Caret: byteOffsetForUTF16(text, int(selected.Location)),
		})
	}
}

func imUnmarkText(self NSView) {
	im := imForView(self)
	if im == nil {
		return
	}
	im.marked = ""
	if im.handler != nil {
		im.handler(common.InputMethodResult{Kind: common.InputMethodPreedit})
	}
}

func imHasMarkedText(self NSView) bool {
	im := imForView(self)
	return im != nil && im.marked != ""
}

func imMarkedRange(self NSView) NSRange {
	im := imForView(self)
	if im == nil || im.marked == "" {
		return NSRange{Location: NSNotFound}
	}
	return NSRange{Location: 0, Length: NSUInteger(utf16Len(im.marked))}
}

func imSelectedRange(self NSView) NSRange {
	// No document model is exposed to the IME; report "no selection".
	return NSRange{Location: NSNotFound}
}

func imFirstRect(self NSView, r NSRange, actual uintptr) NSRect {
	im := imForView(self)
	if im == nil {
		return NSRect{}
	}
	return im.caret
}

func imAttributedSubstring(self NSView, r NSRange, actual uintptr) ID { return 0 }

func imValidAttributes(self NSView) ID { return 0 }

func imCharacterIndexForPoint(self NSView, point NSPoint) uint { return NSNotFound }

func imDoCommandBySelector(self NSView, selector SEL) {
	im := imForView(self)
	if im == nil || im.window == nil || !im.pendingValid {
		return
	}
	// interpretKeyEvents turned this key into an editing/navigation command; the
	// widget handles it via the ordinary key path.
	im.window.emitKey(events.KeyDown, im.pending, im.pending.IsARepeat())
}

// --- helpers ---

func utf16Len(s string) (n int) {
	for _, r := range s {
		n += utf16.RuneLen(r)
	}
	return
}

// byteOffsetForUTF16 converts a UTF-16 code-unit offset within s to a byte offset.
func byteOffsetForUTF16(s string, utf16Pos int) int {
	if utf16Pos <= 0 {
		return 0
	}
	offset := 0
	u16 := 0
	for offset < len(s) && u16 < utf16Pos {
		r, size := utf8.DecodeRuneInString(s[offset:])
		rlen := utf16.RuneLen(r)
		if u16+rlen > utf16Pos {
			break
		}
		u16 += rlen
		offset += size
	}
	return offset
}
