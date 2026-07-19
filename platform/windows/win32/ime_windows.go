package win32

import (
	"errors"
	"unicode/utf16"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/windows/sdk/winapi"
)

// inputMethod is the win32 InputMethod: IMM32 driving composition for a window.
type inputMethod struct {
	window   *Window
	handler  common.InputMethodHandler
	enabled  bool
	savedIMC winapi.HIMC // context detached while IME is disabled (non-text focus)
	spotX    int32       // last pushed candidate spot (physical px), deduped
	spotY    int32
	spotSet  bool
}

// newInputMethod creates the IMM32 input method for window.
func newInputMethod(window common.Window, handler common.InputMethodHandler) (*inputMethod, error) {
	win, ok := window.(*Window)
	if !ok || win.hwnd == 0 {
		return nil, errors.New("win32: invalid window for input method")
	}
	im := &inputMethod{window: win, handler: handler}
	win.im = im // WndProc routes WM_IME_* to this
	return im, nil
}

// SetEnabled associates/detaches the window's input context so composition is on
// only while a text widget is focused.
func (im *inputMethod) SetEnabled(enabled bool) {
	if im.window == nil || im.window.hwnd == 0 || im.enabled == enabled {
		return
	}
	im.enabled = enabled
	im.spotSet = false
	if enabled {
		if im.savedIMC != 0 {
			winapi.ImmAssociateContext(im.window.hwnd, im.savedIMC)
			im.savedIMC = 0
		}
		return
	}
	im.savedIMC = winapi.ImmAssociateContext(im.window.hwnd, 0)
}

// SetCaretRect positions the composition and candidate windows at the caret.
// rect is window-logical; IMM wants client physical pixels.
// Composition (inline preedit) sits at the caret bottom; the candidate window
// sits at the caret top so it doesn't drift below the text line.
func (im *inputMethod) SetCaretRect(rect geometry.Rectangle) {
	if im.window == nil || im.window.hwnd == 0 || !im.enabled {
		return
	}
	scale := im.window.scale
	if scale == 0 {
		scale = 1
	}
	x := int32(rect.X * scale)
	yTop := int32(rect.Y * scale)
	yBottom := int32((rect.Y + rect.Height) * scale)
	if im.spotSet && x == im.spotX && yBottom == im.spotY {
		return
	}
	im.spotX, im.spotY, im.spotSet = x, yBottom, true

	himc := winapi.ImmGetContext(im.window.hwnd)
	if himc == 0 {
		return
	}
	defer winapi.ImmReleaseContext(im.window.hwnd, himc)

	winapi.ImmSetCompositionWindow(himc, &winapi.COMPOSITIONFORM{
		DwStyle:      winapi.CFS_POINT | winapi.CFS_FORCE_POSITION,
		PtCurrentPos: winapi.POINT{X: winapi.LONG(x), Y: winapi.LONG(yBottom)},
	})
	winapi.ImmSetCandidateWindow(himc, &winapi.CANDIDATEFORM{
		DwStyle:      winapi.CFS_CANDIDATEPOS,
		PtCurrentPos: winapi.POINT{X: winapi.LONG(x), Y: winapi.LONG(yTop)},
	})
}

func (im *inputMethod) Reset() {
	if im.window == nil || im.window.hwnd == 0 {
		return
	}
	himc := winapi.ImmGetContext(im.window.hwnd)
	if himc == 0 {
		return
	}
	winapi.ImmNotifyIME(himc, winapi.NI_COMPOSITIONSTR, winapi.CPS_CANCEL, 0)
	winapi.ImmReleaseContext(im.window.hwnd, himc)
}

func (im *inputMethod) Destroy() {
	if im.savedIMC != 0 && im.window != nil && im.window.hwnd != 0 {
		winapi.ImmAssociateContext(im.window.hwnd, im.savedIMC)
		im.savedIMC = 0
	}
	if im.window != nil {
		im.window.im = nil
		im.window = nil
	}
}

// handleComposition handles WM_IME_COMPOSITION: a result string commits, a
// composition string updates the inline preedit.
func (im *inputMethod) handleComposition(lParam winapi.LPARAM) {
	if im.window == nil || im.window.hwnd == 0 {
		return
	}
	himc := winapi.ImmGetContext(im.window.hwnd)
	if himc == 0 {
		return
	}
	defer winapi.ImmReleaseContext(im.window.hwnd, himc)

	flags := winapi.DWORD(lParam)
	if flags&winapi.GCS_RESULTSTR != 0 {
		if text := winapi.ImmGetCompositionString(himc, winapi.GCS_RESULTSTR); text != "" {
			im.handler(common.InputMethodResult{Kind: common.InputMethodCommit, Text: text})
		}
	}
	if flags&winapi.GCS_COMPSTR != 0 {
		text := winapi.ImmGetCompositionString(himc, winapi.GCS_COMPSTR)
		caret := imeByteCaret(text, winapi.ImmGetCompositionCursorPos(himc))
		im.handler(common.InputMethodResult{Kind: common.InputMethodPreedit, Text: text, Caret: caret})
	}
}

// endComposition clears the inline preedit.
func (im *inputMethod) endComposition() {
	im.handler(common.InputMethodResult{Kind: common.InputMethodPreedit})
}

// imeByteCaret converts a UTF-16-code-unit cursor position within the
// composition string to a byte offset into its UTF-8 form.
func imeByteCaret(text string, utf16Pos int) int {
	if utf16Pos <= 0 {
		return 0
	}
	units := utf16.Encode([]rune(text))
	if utf16Pos >= len(units) {
		return len(text)
	}
	return len(string(utf16.Decode(units[:utf16Pos])))
}
