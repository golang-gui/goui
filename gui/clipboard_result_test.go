package gui

import (
	"errors"
	"testing"

	"github.com/golang-gui/goui/platform/events"
)

type clipboardResultStub struct {
	err  error
	text string
}

func TestClipboardFailedCutPreservesDocumentAndHistory(t *testing.T) {
	editor, win, _ := newEditorFixture(t, "abc")
	native := &clipboardResultStub{err: errors.New("native write failure")}
	App.(*application).clipboard = newClipboard(native)
	editor.SetSelection(TextSelection{Anchor: 1, Caret: 2})
	editorKey(t, win, events.KeyX, textCommandModifier())
	if editor.Model().Text() != "abc" || editor.Selection() != (TextSelection{Anchor: 1, Caret: 2}) || editor.Model().CanUndo() {
		t.Fatal("failed Cut changed document, selection or history")
	}
	native.err = nil
	editorKey(t, win, events.KeyX, textCommandModifier())
	if native.text != "b" || editor.Model().Text() != "ac" || !editor.Model().CanUndo() {
		t.Fatal("successful Cut did not copy/delete once")
	}
}

func TestClipboardEmptyPastePreservesSelectionAndHistory(t *testing.T) {
	editor, win, _ := newEditorFixture(t, "abc")
	App.(*application).clipboard = newClipboard(&clipboardResultStub{})
	editor.SetSelection(TextSelection{Anchor: 1, Caret: 2})
	editorKey(t, win, events.KeyV, textCommandModifier())
	if editor.Model().Text() != "abc" || editor.Selection() != (TextSelection{Anchor: 1, Caret: 2}) || editor.Model().CanUndo() {
		t.Fatal("normal empty clipboard paste changed document, selection or history")
	}
}

func (c *clipboardResultStub) SetText(text string) error {
	if c.err != nil {
		return c.err
	}
	c.text = text
	return nil
}
func (c *clipboardResultStub) RequestText(fn func(string, error)) { fn(c.text, c.err) }
func TestClipboardPreservesResults(t *testing.T) {
	native := &clipboardResultStub{}
	adapter := newClipboard(native)
	if err := adapter.SetText(""); err != nil {
		t.Fatal(err)
	}
	called := 0
	adapter.RequestText(func(text string, err error) {
		called++
		if text != "" || err != nil {
			t.Fatal("empty text is a successful value", err)
		}
	})
	native.text = "value"
	adapter.RequestText(func(text string, err error) {
		called++
		if text != "value" || err != nil {
			t.Fatal("lost text result", text, err)
		}
	})
	native.err = errors.New("native write failure")
	if err := adapter.SetText("new"); !errors.Is(err, native.err) {
		t.Fatal("lost write failure")
	}
	adapter.RequestText(func(_ string, err error) {
		called++
		if !errors.Is(err, native.err) {
			t.Fatal("lost native read failure")
		}
	})
	if called != 3 {
		t.Fatal("callback count", called)
	}
	unavailable := newClipboard(nil)
	if unavailable.SetText("new") == nil {
		t.Fatal("nil clipboard write succeeded")
	}
	unavailable.RequestText(func(_ string, err error) {
		if err == nil {
			t.Fatal("nil clipboard hid unavailability")
		}
	})
}
