package gui

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
)

// TextInput must satisfy IMClient so the window can find its context on focus.
var _ IMClient = (*TextInput)(nil)

func TestIMContextEmitsCommitAndPreedit(t *testing.T) {
	c := NewIMContext()

	var commits []string
	var preeditText string
	var preeditCaret int
	c.ConnectCommit(func(s string) { commits = append(commits, s) })
	c.ConnectPreedit(func(s string, caret int) {
		preeditText = s
		preeditCaret = caret
	})

	c.emitCommit("你好")
	c.emitPreedit("shi", 2)

	if len(commits) != 1 || commits[0] != "你好" {
		t.Fatalf("unexpected commits: %v", commits)
	}
	if preeditText != "shi" || preeditCaret != 2 {
		t.Fatalf("unexpected preedit: %q caret=%d", preeditText, preeditCaret)
	}
}

func TestIMContextUnboundCaretAndResetAreSafe(t *testing.T) {
	// Before the owning widget is focused the context has no window bound, so
	// SetCaretRect/Reset must be safe no-ops rather than panic.
	c := NewIMContext()
	c.SetCaretRect(geometry.Rect(1, 2, 1, 10))
	c.Reset()
}

func TestWindowBindsAndRoutesIMEToFocusedTextInput(t *testing.T) {
	win := &window{}
	input := NewTextInput()
	win.SetWidget(input)
	if !win.SetFocusedWidget(input) {
		t.Fatal("text input should accept focus")
	}
	if win.activeIM != input.im {
		t.Fatal("window should bind the focused text input's IMContext")
	}

	// win.onInputMethod is the platform InputMethodHandler; native commit/preedit
	// route through it to the focused widget's IMContext.

	// Commit inserts real text.
	win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodCommit, Text: "你好"})
	if input.Text() != "你好" {
		t.Fatalf("commit not inserted: %q", input.Text())
	}

	// Preedit shows inline but never leaks into Text().
	win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodPreedit, Text: "shi", Caret: 3})
	if input.Text() != "你好" {
		t.Fatalf("preedit leaked into committed text: %q", input.Text())
	}
	if input.preedit != "shi" {
		t.Fatalf("preedit not set: %q", input.preedit)
	}
	if got := input.displayText(); got != "你好shi" {
		t.Fatalf("unexpected display text: %q", got)
	}

	// A commit clears the preedit and inserts the converted text.
	win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodCommit, Text: "是"})
	if input.preedit != "" {
		t.Fatal("commit should clear preedit")
	}
	if input.Text() != "你好是" {
		t.Fatalf("unexpected text after commit: %q", input.Text())
	}
}

func TestWindowUnbindsIMEWhenFocusLeaves(t *testing.T) {
	win := &window{}
	input := NewTextInput()
	win.SetWidget(input)
	win.SetFocusedWidget(input)
	input.SetText("hi")

	win.SetFocusedWidget(nil)
	if win.activeIM != nil {
		t.Fatal("clearing focus should unbind the IME")
	}

	// A stray commit after unbinding must go nowhere, not panic or insert.
	win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodCommit, Text: "x"})
	if input.Text() != "hi" {
		t.Fatalf("commit after unbind should not insert: %q", input.Text())
	}
}

func TestTextInputCommitInsertsAtCaret(t *testing.T) {
	input := NewTextInput()
	input.SetText("ac")
	input.caret = len("a")

	input.onCommit("b")

	if input.Text() != "abc" {
		t.Fatalf("unexpected text: %q", input.Text())
	}
	if input.caret != len("ab") {
		t.Fatalf("unexpected caret: %d", input.caret)
	}
}
