package gui

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
)

// IMContext 与窗口输入路由。

// TextInput must satisfy IMClient so the window can find its context on focus.
var _ IMClient = (*TextInput)(nil)

func TestIMContextEmitsCommitAndPreedit(t *testing.T) {
	c := NewIMContext()

	var commits []string
	var preeditText string
	var preeditCaret int
	c.ConnectCommit(func(commit IMCommit) { commits = append(commits, commit.Text) })
	c.ConnectPreedit(func(s string, caret int) {
		preeditText = s
		preeditCaret = caret
	})

	c.emitCommit(IMCommit{Text: "你好"})
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

func TestIMContextCommitMetadataSurvivesReentry(t *testing.T) {
	c := NewIMContext()
	var got []IMCommit
	c.ConnectCommit(func(commit IMCommit) {
		if commit.Composed {
			c.emitCommit(IMCommit{Text: "nested"})
		}
	})
	c.ConnectCommit(func(commit IMCommit) { got = append(got, commit) })
	c.emitCommit(IMCommit{Text: "outer", Composed: true})
	if len(got) != 2 || got[0] != (IMCommit{Text: "nested"}) || got[1] != (IMCommit{Text: "outer", Composed: true}) {
		t.Fatalf("reentrant commit changed outer metadata: %+v", got)
	}
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
	if input.preedit == nil || input.preedit.Text() != "shi" {
		t.Fatalf("preedit not set: %q", input.preedit)
	}
	if got := input.displayParagraph(0); got != "你好shi" {
		t.Fatalf("unexpected display text: %q", got)
	}

	// A commit clears the preedit and inserts the converted text.
	win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodCommit, Text: "是"})
	if input.preedit != nil {
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

func TestWindowCommitTextUsesEditingPath(t *testing.T) {
	editor, win, _ := newEditorFixture(t, "abc")
	editor.SetSelection(TextSelection{Anchor: 1, Caret: 2})
	changes := 0
	editor.ConnectChange(func(TextChange) { changes++ })
	var received IMCommit
	editor.im.ConnectCommit(func(commit IMCommit) { received = commit })
	text := "你👋e\u0301"
	if err := win.CommitText(text); err != nil {
		t.Fatal(err)
	}
	if editor.Model().Text() != "a"+text+"c" || changes != 1 || received != (IMCommit{Text: text}) {
		t.Fatalf("commit bypassed edit path: text=%q changes=%d metadata=%+v", editor.Model().Text(), changes, received)
	}
	editorKey(t, win, events.KeyZ, textCommandModifier())
	if editor.Model().Text() != "abc" || editor.Selection() != (TextSelection{Anchor: 1, Caret: 2}) {
		t.Fatal("commit did not preserve normal undo and selection")
	}
	if err := win.CommitText(""); err != nil || editor.Model().Text() != "abc" || editor.Selection() != (TextSelection{1, 2}) {
		t.Fatal("empty commit deleted selection")
	}
}

func TestWindowCommitTextRejectsUnavailableInput(t *testing.T) {
	for _, condition := range []string{"no-focus", "read-only", "disabled", "hidden-widget", "hidden-window", "detached", "destroyed", "modal", "preedit", "invalid-utf8"} {
		t.Run(condition, func(t *testing.T) {
			editor, win, _ := newEditorFixture(t, "abc")
			text := "unexpected"
			switch condition {
			case "no-focus":
				win.SetFocusedWidget(nil)
			case "read-only":
				editor.SetReadOnly(true)
			case "disabled":
				editor.SetEnabled(false)
			case "hidden-widget":
				editor.SetVisible(false)
			case "hidden-window":
				_ = win.DispatchEvent(events.StateEvent{State: WindowStateHidden})
			case "detached":
				win.SetWidget(nil)
			case "destroyed":
				win.Destroy()
			case "modal":
				win.SetModalTarget(&popover{})
			case "preedit":
				win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodPreedit, Text: "ni", Caret: 2})
			case "invalid-utf8":
				text = string([]byte{0xff})
			}
			if err := win.CommitText(text); err == nil || editor.Model().Text() != "abc" {
				t.Fatalf("%s accepted input: %q err=%v", condition, editor.Model().Text(), err)
			}
			if condition == "preedit" {
				if editor.preedit == nil || editor.preedit.Text() != "ni" {
					t.Fatal("synthetic commit changed native preedit")
				}
				win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodCommit, Text: "你", Composed: true})
				if err := win.CommitText("x"); err != nil || editor.Model().Text() != "你xabc" {
					t.Fatalf("native commit left stale composition state: %q %v", editor.Model().Text(), err)
				}
			}
		})
	}
}

func TestCommitTextAllowsDestructionDuringNotification(t *testing.T) {
	editor, win, _ := newEditorFixture(t, "abc")
	editor.ConnectChange(func(TextChange) { win.Destroy() })
	if err := win.CommitText("你"); err != nil || !win.destroyed {
		t.Fatalf("commit reentry failed: %v", err)
	}
	if err := win.CommitText("late"); err == nil {
		t.Fatal("destroyed window accepted another commit")
	}
}

func TestPopoverCommitTextUsesModalLogicalFocus(t *testing.T) {
	_, win, _ := newEditorFixture(t, "owner")
	App.(*application).timers = newTimerTestQueue(t).s
	p := &popover{owner: win, modal: true, visible: true}
	input := NewTextInput()
	input.SetText("abc")
	p.SetWidget(input)
	p.SetFocusedWidget(input)
	win.SetModalTarget(p)
	input.SetSelection(TextSelection{1, 2})
	if err := p.CommitText("你👋"); err != nil || input.Text() != "a你👋c" {
		t.Fatalf("popover text commit: %q %v", input.Text(), err)
	}
	if err := win.CommitText("background"); err == nil {
		t.Fatal("owner accepted text while a popup owns keyboard input")
	}
	input.im.emitPreedit("ni", 2)
	if err := p.CommitText("late"); err == nil || input.preedit == nil || input.Text() != "a你👋c" {
		t.Fatal("popover commit overwrote preedit")
	}
	input.im.emitPreedit("", 0)
	win.SetModalTarget(nil)
	if err := p.CommitText("late"); err == nil {
		t.Fatal("popover without modal ownership accepted text")
	}
	win.SetModalTarget(p)
	p.Hide()
	if err := p.CommitText("late"); err == nil || input.Text() != "a你👋c" {
		t.Fatal("hidden popover accepted text")
	}
	p.Destroy()
}

type reentrantIMClient struct {
	WidgetBase
	im    IMContext
	query func()
}

func (w *reentrantIMClient) IMContext() IMContext {
	if w.query != nil {
		w.query()
	}
	return w.im
}

func TestCommitTextRechecksHostAfterIMClientQuery(t *testing.T) {
	for _, action := range []string{"destroy", "modal", "change-focus"} {
		t.Run(action, func(t *testing.T) {
			win := &window{}
			input := &reentrantIMClient{im: NewIMContext()}
			input.SetFocusable(true)
			win.SetWidget(input)
			win.SetFocusedWidget(input)
			commits := 0
			input.im.ConnectCommit(func(IMCommit) { commits++ })
			input.query = func() {
				input.query = nil
				switch action {
				case "destroy":
					win.Destroy()
				case "modal":
					win.SetModalTarget(&popover{})
				case "change-focus":
					win.SetFocusedWidget(nil)
				}
			}
			if err := win.CommitText("stale"); err == nil || commits != 0 {
				t.Fatal("input survived reentrant loss of lifetime, ownership or focus")
			}
		})
	}
}

func TestPopoverKeyboardRejectsLostModalOwnership(t *testing.T) {
	win := &window{}
	first := &popover{owner: win, visible: true, modal: true}
	second := &popover{owner: win, visible: true, modal: true}
	win.SetModalTarget(second)
	if err := first.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: events.KeyArrowDown}); err == nil {
		t.Fatal("old modal popup accepted keyboard input belonging to another popup")
	}
	if err := second.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: events.KeyArrowDown}); err != nil {
		t.Fatal(err)
	}
}
