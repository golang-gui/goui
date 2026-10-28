package ui

import (
	"runtime"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/events"
)

func TestTextDeclarationFlagsPreserveZeroValuesAndBaseModifiers(t *testing.T) {
	t.Run("view", func(t *testing.T) {
		r := newRoot()
		t.Cleanup(r.unmountWindow)
		w := r.update(TextView().ID("editor").Text("abc").Selection(TextSelection{Anchor: 1, Caret: 2})).(*gui.TextView)
		r.update(TextView().ID("editor").Text("").Selection(TextSelection{}).Padding(0).WrapMode(WrapNone))
		if w.ID() != "editor" || w.Model().Text() != "" || w.Selection() != (TextSelection{}) || w.Padding() != (geometry.Insets{}) || w.WrapMode() != WrapNone {
			t.Fatal("explicit zero values or base modifier lost")
		}
		w.Model().SetText("kept")
		w.SetSelection(TextSelection{Anchor: 1, Caret: 3})
		r.update(TextView().ID("editor"))
		if w.ID() != "editor" || w.Model().Text() != "kept" || w.Selection() != (TextSelection{Anchor: 1, Caret: 3}) || w.Padding() != geometry.UniformInsets(4) || w.WrapMode() != WrapWordChar {
			t.Fatal("unset modifiers did not preserve content/selection or restore defaults")
		}
	})
	t.Run("input", func(t *testing.T) {
		r := newRoot()
		t.Cleanup(r.unmountWindow)
		w := r.update(TextInput().ID("input").Text("abc").Selection(TextSelection{Anchor: 1, Caret: 2})).(*gui.TextInput)
		r.update(TextInput().ID("input").Text("").Selection(TextSelection{}).Padding(0))
		if w.ID() != "input" || w.Text() != "" || w.Selection() != (TextSelection{}) || w.Padding() != (geometry.Insets{}) {
			t.Fatal("explicit zero values or base modifier lost")
		}
		w.SetText("kept")
		w.SetSelection(TextSelection{Anchor: 1, Caret: 3})
		r.update(TextInput().ID("input"))
		if w.ID() != "input" || w.Text() != "kept" || w.Selection() != (TextSelection{Anchor: 1, Caret: 3}) || w.Padding() != (geometry.Insets{Left: 8, Top: 4, Right: 8, Bottom: 4}) {
			t.Fatal("unset modifiers did not preserve content/selection or restore defaults")
		}
	})
}

// Value bindings use the same GUI input path as the existing model bindings.
func TestTextValueBindingsEditingAndRebuild(t *testing.T) {
	for _, multiline := range []bool{false, true} {
		t.Run(map[bool]string{false: "input", true: "view"}[multiline], func(t *testing.T) {
			r := newRoot()
			t.Cleanup(r.unmountWindow)
			win := &overlayInputWindow{testWindow: newTestWindow()}
			text := MakeState("abcd")
			selection := MakeState(TextSelection{Anchor: 1, Caret: 3})
			build := func() View {
				if multiline {
					return TextView().BindText(&text).BindSelection(&selection)
				}
				return TextInput().BindText(&text).BindSelection(&selection)
			}
			w := r.updateWindow(win, build())
			w.Arrange(geometry.Rect(0, 0, 200, 80))
			win.SetFocusedWidget(w)
			key := func(k events.Key, modifiers events.Modifiers) {
				t.Helper()
				if err := win.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: k, Modifiers: modifiers}); err != nil {
					t.Fatal(err)
				}
			}
			key(events.KeyBackspace, 0)
			if text.Get() != "ad" || selection.Get() != (TextSelection{Anchor: 1, Caret: 1}) {
				t.Fatalf("delete writeback: %q %+v", text.Get(), selection.Get())
			}
			if r.updateWindow(win, build()) != w {
				t.Fatal("rebuild replaced widget")
			}
			modifier := events.ModifierControl
			if runtime.GOOS == "darwin" {
				modifier = events.ModifierCommand
			}
			key(events.KeyZ, modifier)
			if text.Get() != "abcd" || selection.Get() != (TextSelection{Anchor: 1, Caret: 3}) {
				t.Fatalf("undo after echo: %q %+v", text.Get(), selection.Get())
			}
			r.updateWindow(win, build())
			key(events.KeyZ, modifier|events.ModifierShift)
			if text.Get() != "ad" {
				t.Fatalf("redo after echo: %q", text.Get())
			}
		})
	}
}

func TestTextViewValueReloadAndSelectionOrder(t *testing.T) {
	r := newRoot()
	t.Cleanup(r.unmountWindow)
	text := MakeState("abc")
	selection := MakeState(TextSelection{Anchor: 1, Caret: 2})
	sets := 0
	text.Connect(func() { sets++ })
	selection.Connect(func() { sets++ })
	build := func() View { return TextView().BindText(&text).BindSelection(&selection) }
	w := r.update(build()).(*gui.TextView)
	m := w.Model()
	if sets != 0 || w.Selection() != selection.Get() {
		t.Fatal("initial coordination wrote back or lost selection")
	}
	if err := m.Replace(TextRange{Start: 0, End: 0}, "X"); err != nil {
		t.Fatal(err)
	}
	revision, before := m.Revision(), sets
	r.update(build())
	if m.Revision() != revision || !m.CanUndo() || sets != before || w.Model() != m {
		t.Fatal("equal echo changed revision/history/model or emitted")
	}
	guiChanges := 0
	m.ConnectChange(func(TextChange) { guiChanges++ })
	text.Set("12345\r\n中\xff")
	selection.Set(TextSelection{Anchor: 8, Caret: 2}) // reverse selection, valid in normalized text
	before = sets
	r.update(build())
	if m.Text() != "12345\n中�" || w.Selection() != (TextSelection{Anchor: 6, Caret: 2}) || m.CanUndo() || m.CanRedo() {
		t.Fatalf("reload/UTF-8 selection: %q %+v", m.Text(), w.Selection())
	}
	if sets != before || text.Get() != "12345\r\n中\xff" || selection.Get() != (TextSelection{Anchor: 8, Caret: 2}) || guiChanges != 1 {
		t.Fatal("coordination wrote normalization back or blocked another subscriber")
	}
	r.update(TextView().Text("new"))
	if w.Selection() != (TextSelection{}) || w.Model() != m {
		t.Fatal("reload did not reset selection or replaced model")
	}
	r.update(TextView())
	if m.Text() != "new" {
		t.Fatal("omitting Text cleared private document")
	}
	r.update(TextView().Text(""))
	if m.Text() != "" {
		t.Fatal("explicit empty Text did not clear")
	}
}

func TestTextViewContentConflictBeforeMutation(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		r := newRoot()
		w := r.update(TextView().Text("private")).(*gui.TextView)
		m := w.Model()
		shared := NewTextModel("shared")
		v := TextView().Model(shared).Text("replacement")
		if reverse {
			v = TextView().Text("replacement").Model(shared)
		}
		func() {
			defer func() {
				if recover() == nil {
					t.Error("mixed content sources did not panic")
				}
			}()
			r.update(v)
		}()
		if w.Model() != m || m.Text() != "private" || shared.Text() != "shared" {
			t.Fatal("conflict mutated a document")
		}
		r.unmountWindow()
	}
}

func TestTextViewTextCallbacksLifetime(t *testing.T) {
	r := newRoot()
	t.Cleanup(r.unmountWindow)
	m := NewTextModel("a")
	first, second, changes := 0, 0, 0
	r.update(TextView().Model(m).OnText(func(string) { first++ }))
	r.update(TextView().Model(m).OnText(func(value string) {
		second++
		if value != "ab" {
			t.Fatalf("wrong current text: %q", value)
		}
		r.unmountWindow() // later signal slots must not call unmounted listeners
	}).OnChange(func(TextChange) { changes++ }))
	_ = m.Replace(TextRange{Start: 1, End: 1}, "b")
	_ = m.Replace(TextRange{Start: 2, End: 2}, "c")
	if first != 0 || second != 1 || changes != 0 {
		t.Fatalf("stale/unmounted callbacks: %d %d %d", first, second, changes)
	}
}

func TestTextBindingSugarOverridesAndNil(t *testing.T) {
	text := MakeState("a")
	selection := MakeState(TextSelection{Anchor: 1, Caret: 1})
	calls := 0
	v := TextView().BindText(&text).BindSelection(&selection).
		BindText(nil).BindSelection(nil).
		OnText(func(string) { calls++ }).OnSelection(func(TextSelection) { calls++ })
	v.onText("b")
	v.onSelection(TextSelection{})
	// 绑定在应用时读取；以实际控件内容验证，不再要求构造时保存字符串快照。
	r := newRoot()
	t.Cleanup(r.unmountWindow)
	editor := r.update(v).(*gui.TextView)
	if text.Get() != "a" || selection.Get() != (TextSelection{Anchor: 1, Caret: 1}) || calls != 2 || editor.Model().Text() != "a" || !v.fields.Check(textViewText) {
		t.Fatal("binding sugar silently composed or nil cleared declaration")
	}
	v.BindText(&text).BindSelection(&selection)
	v.onText("c")
	v.onSelection(TextSelection{})
	if text.Get() != "c" || selection.Get() != (TextSelection{}) || calls != 2 {
		t.Fatal("binding did not replace previous callbacks")
	}
	i := TextInput().BindSelection(&selection).BindSelection(nil).OnSelection(func(TextSelection) { calls++ })
	i.onSelection(TextSelection{Anchor: 1, Caret: 1})
	if selection.Get() != (TextSelection{}) || calls != 3 {
		t.Fatal("single-line selection sugar differs")
	}
}

func TestTextViewSharedValueDoesNotShareDocument(t *testing.T) {
	a, b := newRoot(), newRoot()
	t.Cleanup(a.unmountWindow)
	t.Cleanup(b.unmountWindow)
	text := MakeState("ab")
	build := func() View { return TextView().BindText(&text) }
	left := a.update(build()).(*gui.TextView)
	right := b.update(build()).(*gui.TextView)
	if left.Model() == right.Model() {
		t.Fatal("string binding unexpectedly shares a model")
	}
	_ = left.Model().Replace(TextRange{Start: 2, End: 2}, "c")
	if right.Model().Text() != "ab" {
		t.Fatal("state writeback bypassed declarative coordination")
	}
	a.update(build())
	b.update(build())
	if right.Model().Text() != "abc" || !left.Model().CanUndo() || right.Model().CanUndo() {
		t.Fatal("source echo/remote reload history semantics differ")
	}
}

func TestTextViewBindingRetainsModelAndSelection(t *testing.T) {
	r := newRoot()
	t.Cleanup(r.unmountWindow)
	m := NewTextModel("one\ntwo")
	changes, selections := 0, 0
	build := func() View {
		return ScrollView(TextView().Model(m).
			OnChange(func(TextChange) { changes++ }).OnSelection(func(TextSelection) { selections++ }))
	}
	scroll := r.update(build()).(*gui.ScrollView)
	editor := scroll.Child().(*gui.TextView)
	editor.SetSelection(TextSelection{Anchor: 1, Caret: 3})
	if selections != 1 {
		t.Fatal("selection connection missing")
	}
	if err := m.Replace(TextRange{Start: 0, End: 0}, "X"); err != nil {
		t.Fatal(err)
	}
	want := editor.Selection()
	if changes != 1 {
		t.Fatal("model change did not reach UI listener")
	}
	r.update(build())
	if scroll.Child() != editor || editor.Model() != m || editor.Selection() != want || changes != 1 {
		t.Fatal("rebuild replaced the model, widget or editing state")
	}
	info := editor.Snapshot()
	if info.Role != gui.RoleTextView || info.TextEditing == nil || info.TextEditing.Length != m.Len() {
		t.Fatal("UI wrapper lost GUI editing semantics")
	}
	r.update(Label("replaced"))
	before := changes
	_ = m.Replace(TextRange{Start: 0, End: 0}, "Y")
	if changes != before {
		t.Fatal("unmounted UI listener stayed connected")
	}
}

func TestTextViewBindingDefaultsAndDeclarativeSignalBlocking(t *testing.T) {
	r := newRoot()
	t.Cleanup(r.unmountWindow)
	editor := r.update(TextView()).(*gui.TextView)
	private := editor.Model()
	private.SetText("private")
	shared := NewTextModel("shared")
	calls := 0
	v := TextView().Model(shared).ReadOnly(true).AcceptsTab(true).WrapMode(WrapNone).Padding(9).
		Selection(TextSelection{Anchor: 1, Caret: 3}).OnSelection(func(TextSelection) { calls++ })
	r.update(v)
	if calls != 0 || editor.Model() != shared || editor.Selection() != (TextSelection{Anchor: 1, Caret: 3}) || !editor.ReadOnly() || !editor.AcceptsTab() || editor.WrapMode() != WrapNone || editor.Padding() != geometry.UniformInsets(9) {
		t.Fatal("declarative properties emitted or were not applied")
	}
	r.update(TextView())
	if editor.Model() != private || private.Text() != "private" || editor.ReadOnly() || editor.AcceptsTab() || editor.WrapMode() != WrapWordChar || editor.Padding() != geometry.UniformInsets(4) {
		t.Fatal("removing modifiers did not restore constructor defaults")
	}
	if calls != 0 {
		t.Fatal("removing a modifier emitted the old handler")
	}
}

func TestTextInputBindingKeepsUncontrolledTextAndEqualSelection(t *testing.T) {
	r := newRoot()
	t.Cleanup(r.unmountWindow)
	input := r.update(TextInput()).(*gui.TextInput)
	input.SetText("typed value")
	input.SetSelection(TextSelection{Anchor: 2, Caret: 5})
	r.update(TextInput().Padding(8))
	if input.Text() != "typed value" || input.Selection() != (TextSelection{Anchor: 2, Caret: 5}) {
		t.Fatal("unrelated rebuild erased uncontrolled input")
	}
	selections, texts := 0, 0
	r.update(TextInput().Text("typed value").OnText(func(string) { texts++ }).OnSelection(func(TextSelection) { selections++ }))
	if input.Selection() != (TextSelection{Anchor: 2, Caret: 5}) || texts != 0 || selections != 0 {
		t.Fatal("equal declarative text moved selection or emitted")
	}
	r.update(TextInput().Text("").Selection(TextSelection{}).ReadOnly(true))
	if input.Text() != "" || !input.ReadOnly() || selections != 0 || texts != 0 {
		t.Fatal("explicit empty text/read-only update failed or emitted old callbacks")
	}
}

func TestTextInputSubmitBindingUsesEventDispatcherAndLatestHandler(t *testing.T) {
	r := newRoot()
	t.Cleanup(r.unmountWindow)
	win := &overlayInputWindow{testWindow: newTestWindow()}
	first, second := 0, 0
	input := r.updateWindow(win, TextInput().OnSubmit(func() { first++ })).(*gui.TextInput)
	input.Arrange(geometry.Rect(0, 0, 200, 40))
	win.SetFocusedWidget(input)
	enter := func() {
		t.Helper()
		if err := win.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: events.KeyEnter}); err != nil {
			t.Fatal(err)
		}
	}
	enter()
	r.updateWindow(win, TextInput().OnSubmit(func() { second++ }))
	enter()
	r.updateWindow(win, TextInput())
	enter()
	if first != 1 || second != 1 || input.Text() != "" {
		t.Fatalf("submit binding duplicated or retained handler: %d/%d", first, second)
	}
}
