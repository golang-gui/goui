package gui

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
)

// IMClient is implemented by widgets that accept text input. On every focus
// change the window asks the newly focused widget for its IMContext and, when
// present, binds it to the window's native input method for the duration of
// focus.
type IMClient interface {
	IMContext() IMContext
}

// IMContext is a text widget's handle to the input method. The widget creates
// one with NewIMContext, connects Commit/Preedit, and reports its caret; the
// framework does the rest — focus binding, key routing, candidate positioning.
type IMContext interface {
	// ConnectCommit registers a handler for committed text (ordinary typed text,
	// a converted CJK phrase, an emoji, ...). This is the widget's text path.
	ConnectCommit(fn func(text string)) signal.Handle
	// ConnectPreedit registers a handler for the in-progress composition string.
	// caret is the cursor position within text as a byte offset; empty text means
	// the composition ended.
	ConnectPreedit(fn func(text string, caret int)) signal.Handle
	// SetCaretRect reports the caret rectangle in the widget's local coordinates
	// so the native IME can position the candidate window. It is a no-op while
	// the context is not the focused one.
	SetCaretRect(rect geometry.Rectangle)
	// Reset cancels any in-progress composition. Widgets rarely need it (focus
	// changes reset automatically); it exists for cases like dropping a
	// composition on Escape.
	Reset()

	// Framework-internal
	emitCommit(text string)
	emitPreedit(text string, caret int)
	setWindow(w *window) // binds to the focused window; nil unbinds
}

// imContext is the sole IMContext implementation.
type imContext struct {
	commit  signal.Signal1[string]
	preedit signal.Signal2[string, int]
	window  *window // non-nil only while the owning widget is focused
}

// NewIMContext creates an input-method context. A text widget owns one for its
// lifetime and returns it from IMClient.IMContext.
func NewIMContext() IMContext {
	return &imContext{}
}

func (c *imContext) ConnectCommit(fn func(text string)) signal.Handle {
	return c.commit.Connect(fn)
}

func (c *imContext) ConnectPreedit(fn func(text string, caret int)) signal.Handle {
	return c.preedit.Connect(fn)
}

func (c *imContext) SetCaretRect(rect geometry.Rectangle) {
	if c.window != nil {
		c.window.imSetCaretRect(rect)
	}
}

func (c *imContext) Reset() {
	if c.window != nil {
		c.window.imReset()
	}
}

func (c *imContext) emitCommit(text string)             { c.commit.Emit(text) }
func (c *imContext) emitPreedit(text string, caret int) { c.preedit.Emit(text, caret) }
func (c *imContext) setWindow(w *window)                { c.window = w }
