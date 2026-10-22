package gui

import (
	"fmt"
	"image/color"
	"runtime"
	"strings"
	"testing"

	"github.com/golang-gui/goui/core/colors"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui/textedit"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/mlang"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/style"
)

// Shared editing, selection, menu, history and input behavior.

func TestTextEditorDisabledPreservesContentAndRejectsLateInput(t *testing.T) {
	editor, win, _ := newEditorFixture(t, "abc")
	editor.SetSelection(TextSelection{Anchor: 1, Caret: 2})
	selection := editor.Selection()
	clip := &editorClipboard{}
	App.(*application).clipboard = clip
	editor.paste()
	if clip.pending == nil {
		t.Fatal("paste did not request clipboard")
	}
	editor.onPreedit("候选", 1)
	if editor.preedit == nil {
		t.Fatal("preedit setup failed")
	}
	editor.SetEnabled(false)
	if editor.Model().Text() != "abc" || editor.Selection() != selection || editor.preedit != nil || win.FocusedWidget() != nil || editor.imContext() != nil {
		t.Fatal("disable changed content or retained active editing")
	}
	editor.onCommit(IMCommit{Text: "late"})
	editor.onPreedit("late", 0)
	if editor.Model().Text() != "abc" || editor.preedit != nil || editor.canEditCommand(textSelectAll) {
		t.Fatal("disabled editor accepted native input or menu command")
	}
	editor.SetEnabled(true)
	clip.pending("stale", nil)
	if editor.Model().Text() != "abc" || win.FocusedWidget() != nil {
		t.Fatal("reenable accepted old paste or stole focus")
	}
	editor.SetReadOnly(true)
	if !win.SetFocusedWidget(editor) || !editor.canEditCommand(textSelectAll) {
		t.Fatal("read-only was confused with disabled")
	}
	editor.SetEnabled(false)
	editor.Model().SetText("programmatic")
	if editor.Model().Text() != "programmatic" || editor.Snapshot().Enabled {
		t.Fatal("disabled editor rejected programmatic synchronization")
	}
}

func TestTextEditorDisableDuringSelectionNormalizationStopsCommit(t *testing.T) {
	for _, reenable := range []bool{false, true} {
		t.Run(fmt.Sprintf("reenable=%t", reenable), func(t *testing.T) {
			editor, _, _ := newEditorFixture(t, "e\u0301x")
			// A changed shaping boundary can leave a rune-aligned selection
			// inside a cluster until the next editing command normalizes it.
			paragraph := editor.paragraph(0)
			metrics := paragraph.layout.(*testTextLayout)
			metrics.clusters = []typography.TextCluster{
				{Start: 0, Length: 3, Width: 10, Height: 20},
				{Start: 3, Length: 1, X: 10, Width: 10, Height: 20},
			}
			paragraph.geometryValid = false
			paragraph.measured = false // invalidate cached native metrics too
			editor.selection = TextSelection{Anchor: 1, Caret: 1}
			normalizations := 0
			editor.ConnectSelection(func(TextSelection) {
				normalizations++
				editor.SetEnabled(false)
				if reenable {
					editor.SetEnabled(true)
				}
			})
			editor.onCommit(IMCommit{Text: "late"})
			if normalizations != 1 || editor.Model().Text() != "e\u0301x" {
				t.Fatal("commit continued after selection callback disabled the editor")
			}
		})
	}
}

type editorResetIM struct {
	IMContext
	onReset func()
}

func (c *editorResetIM) Reset() { c.onReset() }

func TestTextEditorDisabledResetStopsAfterIMEReentry(t *testing.T) {
	editor, win, _ := newEditorFixture(t, "abc")
	win.SetFocusedWidget(nil) // Exercise the controller cleanup, not focus exit.
	editor.onPreedit("候选", 1)
	resets := 0
	editor.im = &editorResetIM{IMContext: editor.im, onReset: func() {
		resets++
		editor.SetEnabled(true)
		// A synchronous callback owns the new state. Stale cleanup must not
		// restart its caret or break the new editing session's undo group.
		editor.caretVisible = false
	}}
	editor.SetEnabled(false)
	if resets != 1 || !IsEnabled(editor) || editor.preedit != nil || editor.caretVisible {
		t.Fatal("disabled-input cleanup continued after IME callback reenabling")
	}
}

func TestTextEditorCommandModifierIsNotSuper(t *testing.T) {
	for _, modifier := range []events.Modifiers{events.ModifierControl, events.ModifierCommand, events.ModifierSuper, events.ModifierWin, events.ModifierOption, textCommandModifier() | events.ModifierOption, textCommandModifier() | events.ModifierAlt} {
		editor, win, _ := newEditorFixture(t, "abc")
		editorKey(t, win, events.KeyA, modifier)
		want := TextSelection{}
		if runtime.GOOS == "darwin" && modifier == events.ModifierCommand || runtime.GOOS != "darwin" && modifier == events.ModifierControl {
			want = TextSelection{0, 3}
		}
		if editor.Selection() != want {
			t.Fatalf("modifier %v: selection %+v, want %+v", modifier, editor.Selection(), want)
		}
	}
}

func TestTextEditorCommandRejectsExtraModifiers(t *testing.T) {
	for _, extra := range []events.Modifiers{events.ModifierAltGraph, events.ModifierWin, events.ModifierSuper, 1 << 15} {
		event := events.KeyEvent{EventType: events.KeyDown, Key: events.KeyA, Modifiers: textCommandModifier() | extra}
		if textCommand(event.Modifiers) || cancelsTextPreedit(event) {
			t.Fatalf("extra modifier %v interpreted as editing command", extra)
		}
		editor, win, _ := newEditorFixture(t, "abc")
		editorKey(t, win, event.Key, event.Modifiers)
		if editor.Selection() != (TextSelection{}) {
			t.Fatal("extra modifier unexpectedly selected all text")
		}
		if (KeyGesture{Key: KeyA, Modifiers: ModPrimary}).Matches(event) {
			t.Fatal("extra modifier unexpectedly matched shortcut")
		}
	}
}

// Context menus use the real dispatcher, PopoverMenu and menu row controllers;
// only the native surface, typography and clipboard are replaced (no desktop).
func newEditorMenuFixture(t *testing.T, singleLine bool) (*textEditor, *window, *placementPlatform, *editorClipboard) {
	t.Helper()
	view, win, _ := newEditorFixture(t, "abcdef")
	editor := view.textEditor
	if singleLine {
		input := NewTextInput()
		input.SetText("abcdef")
		win.SetWidget(input)
		input.Arrange(geometry.Rect(0, 0, 209, 32))
		win.SetFocusedWidget(input)
		editor = input.textEditor
	}
	clip := &editorClipboard{}
	plat := &placementPlatform{syncSize: true}
	App.(*application).platform = plat
	App.(*application).clipboard = clip
	win.platformWindow = &placementWindow{area: geometry.Rect(0, 0, 800, 600)}
	return editor, win, plat, clip
}

func editorMenuOpen(t *testing.T, editor *textEditor, win *window) *PopoverMenu {
	t.Helper()
	editorKey(t, win, events.KeyF10, events.ModifierShift)
	if editor.contextMenu == nil || !editor.contextMenu.popup.Visible() {
		t.Fatal("Shift+F10 did not open the context menu")
	}
	pm := editor.contextMenu.popup
	settlePlacement(t, pm.popover.(*popover))
	return pm
}

func editorMenuClick(t *testing.T, pm *PopoverMenu, label string) {
	t.Helper()
	p := pm.popover.(*popover)
	settlePlacement(t, p)
	for _, widget := range pm.content.list.items {
		row := widget.(*menuItemRow)
		if row.mi.Label() != label {
			continue
		}
		pos := absOrigin(row)
		pos.X += row.Rect().Width / 2
		pos.Y += row.Rect().Height / 2
		for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
			if err := p.DispatchEvent(events.PointerEvent{EventType: kind, Position: pos, Button: events.PointerButtonLeft}); err != nil {
				t.Fatal(err)
			}
		}
		return
	}
	t.Fatalf("menu item %q is not visible", label)
}

func TestTextEditorContextMenuCommands(t *testing.T) {
	for _, singleLine := range []bool{false, true} {
		for _, command := range []string{"Undo", "Redo", "Cut", "Copy", "Paste", "Delete", "Select All"} {
			t.Run(fmt.Sprintf("single=%t/%s", singleLine, command), func(t *testing.T) {
				editor, win, _, clip := newEditorMenuFixture(t, singleLine)
				editor.setSelectionValue(TextSelection{1, 4})
				if command == "Undo" || command == "Redo" {
					editor.im.emitCommit(IMCommit{Text: "XY"})
					if command == "Redo" {
						editorKey(t, win, events.KeyZ, textCommandModifier())
					}
				}
				pm := editorMenuOpen(t, editor, win)
				if clip.pending != nil {
					t.Fatal("opening a context menu read clipboard text")
				}
				editorMenuClick(t, pm, command)
				if pm.Visible() || win.modalTarget != nil || win.FocusedWidget() != editor.owner {
					t.Fatal("activation did not dismiss modal menu and retain editor focus")
				}
				want := "abcdef"
				switch command {
				case "Cut", "Delete":
					want = "aef"
				case "Redo":
					want = "aXYef"
				case "Paste":
					if clip.pending == nil {
						t.Fatal("paste did not request clipboard")
					}
					clip.pending("X\nY", nil)
					want = "aX\nYef"
					if singleLine {
						want = "aX Yef"
					}
				}
				if got := editor.model.Text(); got != want {
					t.Fatalf("text=%q, want %q", got, want)
				}
				if (command == "Cut" || command == "Copy") && clip.text != "bcd" {
					t.Fatalf("clipboard=%q, want bcd", clip.text)
				}
				if command == "Select All" && editor.selection != (TextSelection{0, 6}) {
					t.Fatal("menu did not select all")
				}
				if command == "Cut" || command == "Delete" || command == "Paste" {
					editorKey(t, win, events.KeyZ, textCommandModifier())
					if editor.model.Text() != "abcdef" || editor.selection != (TextSelection{1, 4}) {
						t.Fatal("menu operation was not one undoable edit with selection restore")
					}
				}
			})
		}
	}
}

func TestTextEditorContextMenuLanguageAndLifecycle(t *testing.T) {
	previous := mlang.Language()
	t.Cleanup(func() { _ = mlang.SetLanguage(previous) })
	_ = mlang.SetLanguage("en")
	editor, win, _, clip := newEditorMenuFixture(t, false)
	editor.setSelectionValue(TextSelection{Anchor: 1, Caret: 4})
	pm := editorMenuOpen(t, editor, win)
	copyItem := editor.contextMenu.items[textCopy]
	model := pm.Menu()
	for _, tc := range []struct{ tag, copy string }{
		{"zh-CN", "复制"}, {"ja", "コピー"}, {"ko", "복사"}, {"de", "Kopieren"},
		{"fr", "Copier"}, {"ru", "Копировать"}, {"ar", "نسخ"},
	} {
		if err := mlang.SetLanguage(tc.tag); err != nil {
			t.Fatal(err)
		}
		settlePlacement(t, pm.popover.(*popover))
		if !pm.Visible() || pm.Menu() != model || editor.contextMenu.items[textCopy] != copyItem ||
			copyItem.Label() != tc.copy || editor.selection != (TextSelection{Anchor: 1, Caret: 4}) || editor.model.Text() != "abcdef" {
			t.Fatalf("%s: translation recreated menu or changed editor state", tc.tag)
		}
	}
	editorMenuClick(t, pm, "نسخ") // normal pointer dispatch still executes Copy
	if clip.text != "bcd" || pm.Visible() || win.FocusedWidget() != editor.owner {
		t.Fatal("translated item lost command or focus semantics")
	}
	owner := editor.owner
	win.SetWidget(nil)
	if editor.contextMenu.language != nil {
		t.Fatal("unmount retained the language connection")
	}
	_ = mlang.SetLanguage("ja")
	if copyItem.Label() != "نسخ" {
		t.Fatal("unmounted editor reacted to a language change")
	}
	win.SetWidget(owner)
	owner.Arrange(geometry.Rect(0, 0, 209, 108))
	win.SetFocusedWidget(owner)
	pm = editorMenuOpen(t, editor, win)
	if editor.contextMenu.language == nil || editor.contextMenu.items[textCopy].Label() != "コピー" {
		t.Fatal("remount did not reconnect or read the current language")
	}
	_ = mlang.SetLanguage("zh-CN")
	if !pm.Visible() || editor.contextMenu.items[textCopy].Label() != "复制" {
		t.Fatal("remounted menu did not translate live")
	}
}

func TestTextEditorContextMenuPointerAndDismiss(t *testing.T) {
	editor, win, plat, _ := newEditorMenuFixture(t, false)
	editor.setSelectionValue(TextSelection{4, 1})
	rightClick := func(x float32) {
		t.Helper()
		// Windows requests on release; other platforms request on press.
		// Dispatch the complete click rather than assuming either timing.
		for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
			if err := win.DispatchEvent(events.PointerEvent{EventType: kind, Button: events.PointerButtonRight, Position: geometry.Point{X: x, Y: 12}}); err != nil {
				t.Fatal(err)
			}
		}
	}
	rightClick(24) // Fixed fixture cells: padding 4 + two cells, inside [1,4).
	if editor.contextMenu == nil || !editor.contextMenu.popup.Visible() || editor.selection != (TextSelection{4, 1}) {
		t.Fatal("right click lost the existing reversed selection")
	}
	pm := editor.contextMenu.popup
	if pm.popover.Position() != (geometry.Point{24, 12}) || editor.input.CapturingPointer() {
		t.Fatal("wrong local popup origin or retained selection drag")
	}
	editorKey(t, win, events.KeyEscape, 0)
	if pm.Visible() || editor.selection != (TextSelection{4, 1}) || win.modalTarget != nil {
		t.Fatal("Escape changed selection or retained modal menu")
	}
	rightClick(42) // Last selected cell's trailing half maps to caret offset 4.
	if editor.selection != (TextSelection{4, 1}) {
		t.Fatal("nearest-caret rounding discarded a visually hit selection")
	}
	editorKey(t, win, events.KeyEscape, 0)
	rightClick(54)
	if editor.selection != (TextSelection{5, 5}) || !pm.Visible() || len(plat.natives) != 1 {
		t.Fatal("right click outside selection did not position caret/reuse popup")
	}
	// The first outside click only dismisses; it does not move the caret.
	win.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Button: events.PointerButtonLeft, Position: geometry.Point{X: 4, Y: 12}})
	if pm.Visible() || editor.selection != (TextSelection{5, 5}) {
		t.Fatal("outside click leaked into editor")
	}
	// Shift+F10 preserves the selection and anchors at the caret bottom.
	editor.setSelectionValue(TextSelection{1, 3})
	editorMenuOpen(t, editor, win)
	if pm.popover.Position() != (geometry.Point{34, 24}) || editor.selection != (TextSelection{1, 3}) {
		t.Fatalf("keyboard popup moved selection or used wrong caret: %v", pm.popover.Position())
	}
	win.DispatchEvent(events.FocusEvent{Focused: false})
	if pm.Visible() || win.modalTarget != nil {
		t.Fatal("window focus loss retained menu")
	}
}

func TestTextEditorContextMenuScrolledPosition(t *testing.T) {
	editor, win, _, _ := newEditorMenuFixture(t, false)
	editor.setModel(NewTextModel("first\nsecond\nthird"))
	editor.setSelectionValue(TextSelection{7, 10})
	editor.revealCaret = false
	editor.layoutVisible(geometry.Size{209, 60}, geometry.Point{Y: 20})
	for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
		if err := win.DispatchEvent(events.PointerEvent{EventType: kind, Button: events.PointerButtonRight, Position: geometry.Point{X: 24, Y: 12}}); err != nil {
			t.Fatal(err)
		}
	}
	if editor.contextMenu == nil || !editor.contextMenu.popup.Visible() || editor.selection != (TextSelection{7, 10}) || editor.contextMenu.popup.popover.Position() != (geometry.Point{24, 12}) {
		t.Fatal("scrolled selection or popup origin used document rather than local coordinates")
	}
}

func TestTextEditorControlClickContextMenu(t *testing.T) {
	editor, win, _, _ := newEditorMenuFixture(t, true)
	win.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Button: events.PointerButtonLeft, Modifiers: events.ModifierControl, Position: geometry.Point{X: 24, Y: 12}})
	visible := editor.contextMenu != nil && editor.contextMenu.popup.Visible()
	if visible != (runtime.GOOS == "darwin") {
		t.Fatalf("Control+click menu=%t on %s", visible, runtime.GOOS)
	}
}

func TestTextEditorCopyCommandDuringPreedit(t *testing.T) {
	editor, win, _, clip := newEditorMenuFixture(t, false)
	editor.setSelectionValue(TextSelection{1, 6})
	editor.im.emitPreedit("x", 1) // Projection is shorter than the model selection.
	editorKey(t, win, events.KeyC, textCommandModifier())
	if clip.text != "bcdef" || editor.preedit == nil || editor.selection != (TextSelection{1, 6}) {
		t.Fatalf("copy normalized model positions against preedit: %q, %+v", clip.text, editor.selection)
	}
}

func TestTextEditorContextMenuAvailability(t *testing.T) {
	editor, win, _, _ := newEditorMenuFixture(t, false)
	pm := editorMenuOpen(t, editor, win)
	check := func(enabled ...textEditCommand) {
		t.Helper()
		for command, item := range editor.contextMenu.items {
			want := false
			for _, e := range enabled {
				want = want || command == e
			}
			if item.Enabled() != want {
				t.Fatalf("%s enabled=%v, want %v", item.Label(), item.Enabled(), want)
			}
		}
	}
	check(textPaste, textSelectAll)
	editor.setSelectionValue(TextSelection{1, 4})
	check(textCut, textCopy, textPaste, textDeleteSelection, textSelectAll)
	editor.setReadOnly(true)
	check(textCopy, textSelectAll)
	editorMenuClick(t, pm, "Delete")
	if !pm.Visible() || editor.model.Text() != "abcdef" {
		t.Fatal("disabled menu item activated")
	}
	for _, widget := range pm.content.list.items {
		info := widget.Snapshot()
		if info.Text == "Delete" && (info.Enabled || len(info.Actions) != 0 || info.Role != RoleMenuItem) {
			t.Fatal("disabled menu item advertises interactive snapshot")
		}
	}
	editor.setReadOnly(false)
	_ = editor.model.Apply(TextEdit{Range: TextRange{0, 0}, Text: "!"}, textedit.EditOptions{Atomic: true})
	check(textUndo, textCut, textCopy, textPaste, textDeleteSelection, textSelectAll)
	editor.model.SetText("")
	check(textPaste)
}

func TestTextEditorContextMenuLifecycle(t *testing.T) {
	for _, kind := range []string{"unmount", "replace-model", "reopen", "preedit", "stale-paste"} {
		t.Run(kind, func(t *testing.T) {
			editor, win, plat, clip := newEditorMenuFixture(t, false)
			editor.setSelectionValue(TextSelection{1, 4})
			if kind == "preedit" {
				editor.im.emitPreedit("未提交", 3)
			}
			pm := editorMenuOpen(t, editor, win)
			switch kind {
			case "unmount":
				pm.ConnectClosed(func() { win.SetWidget(nil) })
				editorMenuClick(t, pm, "Delete")
				if editor.model.Text() != "abcdef" || !plat.natives[0].destroyed || win.modalTarget != nil {
					t.Fatal("activation after unmount or popup resource leak")
				}
			case "replace-model":
				model := NewTextModel("replacement")
				pm.ConnectClosed(func() { editor.setModel(model) })
				editorMenuClick(t, pm, "Delete")
				if editor.model != model || model.Text() != "replacement" {
					t.Fatal("old menu action edited replacement model")
				}
			case "preedit":
				if editor.preedit != nil || editor.model.Text() != "abcdef" {
					t.Fatal("opening menu committed temporary preedit")
				}
				editorMenuClick(t, pm, "Copy")
				if clip.text != "bcd" {
					t.Fatal("copy included preedit instead of committed selection")
				}
			case "reopen":
				once := true
				pm.ConnectClosed(func() {
					if once {
						once = false
						editorMenuOpen(t, editor, win)
					}
				})
				editorMenuClick(t, pm, "Delete")
				if !pm.Visible() || editor.model.Text() != "abcdef" {
					t.Fatal("old action ran after close callback opened a new menu")
				}
			case "stale-paste":
				editorMenuClick(t, pm, "Paste")
				editor.setSelectionValue(TextSelection{})
				clip.pending("stale", nil)
				if editor.model.Text() != "abcdef" {
					t.Fatal("late menu paste changed a different selection")
				}
			}
		})
	}
}

// 导航、选择、剪贴板与历史。

func TestTextViewKeysSelectionNativeCommitAndReadonly(t *testing.T) {
	editor, win, _ := newEditorFixture(t, "ab\ncd")
	editor.SetSelection(TextSelection{1, 1})
	editorKey(t, win, events.KeyArrowRight, events.ModifierShift)
	if editor.Selection() != (TextSelection{1, 2}) {
		t.Fatalf("shift selection %+v", editor.Selection())
	}
	editor.im.emitCommit(IMCommit{Text: "中"})
	if editor.model.Text() != "a中\ncd" || editor.selection != (TextSelection{4, 4}) {
		t.Fatalf("commit result %q %+v", editor.model.Text(), editor.selection)
	}
	editorKey(t, win, events.KeyDelete, 0)
	if editor.model.Text() != "a中cd" {
		t.Fatal("delete did not join paragraphs")
	}
	editorKey(t, win, events.KeyEnter, 0)
	if editor.model.Text() != "a中\ncd" {
		t.Fatal("enter did not insert LF")
	}
	editor.SetReadOnly(true)
	if editor.IMContext() != nil || win.activeIM != nil {
		t.Fatal("read-only still enables IME")
	}
	editorKey(t, win, events.KeyBackspace, 0)
	editor.im.emitCommit(IMCommit{Text: "NO"})
	if editor.model.Text() != "a中\ncd" {
		t.Fatal("read-only changed document")
	}
	editorKey(t, win, events.KeyHome, textCommandModifier())
	if editor.selection.Caret != 0 {
		t.Fatal("read-only navigation disabled")
	}
	editor.SetReadOnly(false)
	if win.activeIM != editor.im {
		t.Fatal("editable IME not rebound")
	}
}

func TestTextViewPointerCaptureAndVerticalColumn(t *testing.T) {
	editor, win, _ := newEditorFixture(t, "abc\nx\nabc")
	for _, e := range []events.PointerEvent{
		{EventType: events.PointerDown, Position: geometry.Point{X: 6, Y: 8}, Button: events.PointerButtonLeft},
		{EventType: events.PointerMove, Position: geometry.Point{X: 207, Y: 8}, Buttons: events.PointerButtonLeftDown},
		{EventType: events.PointerUp, Position: geometry.Point{X: 207, Y: 8}, Button: events.PointerButtonLeft},
	} {
		if err := win.DispatchEvent(e); err != nil {
			t.Fatal(err)
		}
	}
	if editor.selection != (TextSelection{0, 3}) || editor.input.CapturingPointer() {
		t.Fatalf("drag selection %+v", editor.selection)
	}
	editor.SetSelection(TextSelection{3, 3})
	editorKey(t, win, events.KeyArrowDown, 0)
	if editor.selection.Caret != 5 {
		t.Fatalf("short line caret=%d", editor.selection.Caret)
	}
	editorKey(t, win, events.KeyArrowDown, 0)
	if editor.selection.Caret != 9 {
		t.Fatalf("desired X not retained: %d", editor.selection.Caret)
	}
}

func TestTextViewPasteDropsStaleReply(t *testing.T) {
	editor, win, _ := newEditorFixture(t, "abc")
	clip := &editorClipboard{}
	App.(*application).clipboard = clip
	editor.SetSelection(TextSelection{1, 2})
	editorKey(t, win, events.KeyV, textCommandModifier())
	editor.SetSelection(TextSelection{3, 3})
	clip.pending("wrong", nil)
	if editor.model.Text() != "abc" {
		t.Fatal("stale selection received paste")
	}
	editorKey(t, win, events.KeyV, textCommandModifier())
	clip.pending("\r\nX", nil)
	if editor.model.Text() != "abc\nX" {
		t.Fatalf("paste not normalized: %q", editor.model.Text())
	}
	editorKey(t, win, events.KeyV, textCommandModifier())
	win.SetWidget(nil)
	win.SetWidget(editor)
	clip.pending("wrong", nil)
	if editor.model.Text() != "abc\nX" {
		t.Fatal("paste survived unmount/remount")
	}
}

func TestTextViewUndoRestoresInvokerSelectionAndSplitsEditors(t *testing.T) {
	a, win, _ := newEditorFixture(t, "ab")
	b := NewTextView()
	b.SetModel(a.model)
	other := &window{}
	other.SetWidget(b)
	b.Arrange(geometry.Rect(0, 0, 209, 108))
	t.Cleanup(func() { other.SetWidget(nil) })
	a.SetSelection(TextSelection{2, 2})
	b.SetSelection(TextSelection{2, 2})
	a.im.emitCommit(IMCommit{Text: "x"})
	a.im.emitCommit(IMCommit{Text: "y"})
	b.im.emitCommit(IMCommit{Text: "z"})
	editorKey(t, win, events.KeyZ, textCommandModifier())
	if a.model.Text() != "abxy" || a.selection != (TextSelection{4, 4}) {
		t.Fatalf("undo from other view: %q %+v", a.model.Text(), a.selection)
	}
	b.SetSelection(TextSelection{0, 0})
	editorKey(t, win, events.KeyZ, textCommandModifier())
	if a.model.Text() != "ab" || a.selection != (TextSelection{2, 2}) || b.selection != (TextSelection{}) {
		t.Fatalf("undo restored wrong view: %q %+v %+v", a.model.Text(), a.selection, b.selection)
	}
	if a.model.CanUndo() {
		t.Fatal("editing views should produce exactly two undo steps")
	}
	editorKey(t, win, events.KeyZ, textCommandModifier()|events.ModifierShift)
	if a.model.Text() != "abxy" || a.selection != (TextSelection{4, 4}) {
		t.Fatal("redo did not restore after-selection")
	}
}

func TestTextViewReentrantModelAndSelectionCallbacks(t *testing.T) {
	typo := &editorTypography{}
	setTestApplication(t, typo)
	m := NewTextModel("a")
	m.ConnectChange(func(change TextChange) {
		if change.Revision == 1 {
			_ = m.Replace(TextRange{0, 0}, "Y")
		}
	})
	editor := NewTextView()
	editor.SetModel(m)
	win := &window{}
	win.SetWidget(editor)
	t.Cleanup(func() { win.SetWidget(nil) })
	editor.Arrange(geometry.Rect(0, 0, 209, 108))
	editor.SetSelection(TextSelection{1, 1})
	editor.im.emitCommit(IMCommit{Text: "X"})
	if m.Text() != "YaX" || editor.selection != (TextSelection{3, 3}) {
		t.Fatalf("reentrant model: %q %+v", m.Text(), editor.selection)
	}
	editor.ConnectSelection(func(TextSelection) { win.SetWidget(nil) })
	editor.im.emitCommit(IMCommit{Text: "Z"})
	if editor.modelHandle != nil || !editor.suspended {
		t.Fatal("selection callback unmount was ignored")
	}
}

// 双向文本与视觉导航。

// Explicit geometry, not editor-derived expectations: Hebrew occupies x=10..30
// in reverse byte order. Ordinary text uses the existing 10 DIP fixture.
type bidiEditorTypography struct{ editorTypography }

func (c *bidiEditorTypography) NewTextLayout(text string, format typography.TextFormat, width, height float32) (typography.TextLayout, error) {
	l, err := c.editorTypography.NewTextLayout(text, format, width, height)
	if text != "aאבc" && text != "אב" {
		return l, err
	}
	p := l.(*testTextLayout)
	start, x := 0, float32(0)
	p.clusters = nil
	if text == "aאבc" {
		start, x = 1, 10
		p.clusters = append(p.clusters, typography.TextCluster{Length: 1, Width: 10, Height: 20})
	}
	p.clusters = append(p.clusters,
		typography.TextCluster{Start: start, Length: 2, X: x + 10, Width: 10, Height: 20, Direction: typography.TextRightToLeft},
		typography.TextCluster{Start: start + 2, Length: 2, X: x, Width: 10, Height: 20, Direction: typography.TextRightToLeft})
	if text == "aאבc" {
		p.clusters = append(p.clusters, typography.TextCluster{Start: 5, Length: 1, X: 30, Width: 10, Height: 20})
	}
	return p, err
}

func TestTextEditorCollapseSelectionVisually(t *testing.T) {
	for _, single := range []bool{false, true} {
		name := "multiline"
		if single {
			name = "singleline"
		}
		t.Run(name, func(t *testing.T) {
			for _, tc := range []struct {
				name, text  string
				selection   TextSelection
				left, right int
			}{
				{"rtl", "אב", TextSelection{0, 4}, 4, 0},
				{"rtl-reverse-selection", "אב", TextSelection{4, 0}, 4, 0},
				{"embedded-rtl", "aאבc", TextSelection{1, 5}, 5, 1},
				{"disjoint-selection", "aאבc", TextSelection{0, 3}, 0, 3},
				{"ltr", "abcd", TextSelection{1, 3}, 1, 3},
			} {
				t.Run(tc.name, func(t *testing.T) {
					setTestApplication(t, &bidiEditorTypography{})
					var widget Widget
					var editor *textEditor
					if single {
						input := NewTextInput()
						input.SetText(tc.text)
						widget, editor = input, input.textEditor
					} else {
						view := NewTextView()
						view.SetModel(NewTextModel(tc.text))
						widget, editor = view, view.textEditor
					}
					win := &window{}
					win.SetWidget(widget)
					t.Cleanup(func() { win.SetWidget(nil) })
					widget.Arrange(geometry.Rect(0, 0, 200, 80))
					win.SetFocusedWidget(widget)
					for _, direction := range []struct {
						key  events.Key
						want int
					}{{events.KeyArrowLeft, tc.left}, {events.KeyArrowRight, tc.right}} {
						editor.setSelectionValue(tc.selection)
						editorKey(t, win, direction.key, 0)
						if editor.selection != (TextSelection{direction.want, direction.want}) {
							t.Fatalf("key %v collapsed to %+v, want %d", direction.key, editor.selection, direction.want)
						}
					}
				})
			}
		})
	}
}

func TestTextEditorCollapseSelectionAcrossLines(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		selection  TextSelection
	}{
		{"paragraphs", "abcd\ne", TextSelection{3, 6}},
		{"soft-wrap", "abcdef", TextSelection{3, 5}},
		{"soft-wrap-boundary", "abcdefgh", TextSelection{4, 8}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			editor, win, _ := newEditorFixture(t, tc.text)
			editor.Arrange(geometry.Rect(0, 0, 48, 100)) // four 10 DIP cells plus padding
			for _, key := range []events.Key{events.KeyArrowLeft, events.KeyArrowRight} {
				editor.SetSelection(tc.selection)
				editorKey(t, win, key, 0)
				want := tc.selection.Anchor
				if key == events.KeyArrowRight {
					want = tc.selection.Caret
				}
				if editor.Selection() != (TextSelection{want, want}) {
					t.Fatalf("key %v = %+v, want %d", key, editor.Selection(), want)
				}
			}
		})
	}
}

// 预编辑提交与撤销分组。

// XIM can clear/end preedit before delivering its commit in a later key event.
// Keep that commit separate from ordinary typing on both sides, while still
// merging adjacent ordinary keystrokes. Exercise the window's actual IM route.
func TestTextEditorPreeditDoneBeforeCommitUndoGroups(t *testing.T) {
	for _, single := range []bool{false, true} {
		name := "multiline"
		if single {
			name = "singleline"
		}
		t.Run(name, func(t *testing.T) {
			var editor *textEditor
			var win *window
			if single {
				input, w, _ := newInputFixture(t, "")
				editor, win = input.textEditor, w
			} else {
				view, w, _ := newEditorFixture(t, "")
				editor, win = view.textEditor, w
			}
			commit := func(text string) {
				win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodCommit, Text: text})
			}
			commit("a")
			commit("b")
			win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodPreedit, Text: "你", Caret: 3})
			win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodPreedit})
			if editor.model.Text() != "ab" {
				t.Fatal("preedit changed committed text")
			}
			commit("你")
			commit("x")
			commit("y")
			if editor.model.Text() != "ab你xy" {
				t.Fatal("commit was lost")
			}
			for _, want := range []string{"ab你", "ab", ""} {
				editorKey(t, win, events.KeyZ, textCommandModifier())
				if got := editor.model.Text(); got != want {
					t.Fatalf("undo: %q want %q", got, want)
				}
			}
		})
	}
}

func TestTextEditorCommitWithoutPreeditUndoGroups(t *testing.T) {
	for _, single := range []bool{false, true} {
		name := "multiline"
		if single {
			name = "singleline"
		}
		t.Run(name, func(t *testing.T) {
			var editor *textEditor
			var win *window
			if single {
				input, w, _ := newInputFixture(t, "")
				editor, win = input.textEditor, w
			} else {
				view, w, _ := newEditorFixture(t, "")
				editor, win = view.textEditor, w
			}
			commit := func(text string, composed bool) {
				win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodCommit, Text: text, Composed: composed})
			}
			commit("a", false)
			commit("中", false)   // Unicode itself does not imply composition
			commit("word", true) // ASCII can be a native composition/service result
			commit("你", true)
			commit("x", false)
			commit("y", false)
			if editor.model.Text() != "a中word你xy" {
				t.Fatal("native text lost")
			}
			for _, want := range []string{"a中word你", "a中word", "a中", ""} {
				editorKey(t, win, events.KeyZ, textCommandModifier())
				if got := editor.model.Text(); got != want {
					t.Fatalf("undo = %q want %q", got, want)
				}
			}
			for _, want := range []string{"a中", "a中word", "a中word你", "a中word你xy"} {
				editorKey(t, win, events.KeyZ, textCommandModifier()|events.ModifierShift)
				if got := editor.model.Text(); got != want {
					t.Fatalf("redo = %q want %q", got, want)
				}
			}
		})
	}
}

func TestTextViewBlinkOnlyPaintsAndStopsWithFocus(t *testing.T) {
	editor, _, win, q, typo := timedEditorFixture(t)
	if editor.blinkTimer == nil || !editor.blinkTimer.Active() {
		t.Fatal("focused view did not start blink")
	}
	count := len(typo.calls)
	win.layoutDirty, win.paintDirty = false, false
	q.advance(textCaretInterval)
	q.dispatch(t)
	if editor.caretVisible || !win.paintDirty || win.layoutDirty || len(typo.calls) != count {
		t.Fatal("blink changed layout or failed to request paint")
	}
	editorKey(t, win, events.KeyArrowRight, 0)
	if !editor.caretVisible {
		t.Fatal("navigation did not show caret immediately")
	}
	q.advance(textCaretInterval)
	q.dispatch(t)
	if editor.caretVisible {
		t.Fatal("restarted blink did not fire")
	}
	_ = win.DispatchEvent(events.FocusEvent{Focused: false})
	if editor.blinkTimer.Active() {
		t.Fatal("background window kept blinking")
	}
	_ = win.DispatchEvent(events.FocusEvent{Focused: true})
	if !editor.blinkTimer.Active() || !editor.caretVisible {
		t.Fatal("returning window focus did not resume steady-to-blink cycle")
	}
	win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodPreedit, Text: "compose", Caret: 7})
	if editor.blinkTimer.Active() || !editor.caretVisible {
		t.Fatal("IME caret must remain steady")
	}
	win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodPreedit})
	if !editor.blinkTimer.Active() {
		t.Fatal("cancel did not resume blinking")
	}
	win.SetWidget(nil)
	if editor.blinkTimer != nil || editor.scrollTimer != nil || editor.windowFocusHandle != nil || len(q.s.queue) != 0 {
		t.Fatal("unmount retained timers/connections")
	}
}

// Shared baseline and hit geometry.

type sizingBaselineTypography struct{ editorTypography }

func (c *sizingBaselineTypography) NewTextLayout(text string, f typography.TextFormat, width, height float32) (typography.TextLayout, error) {
	l, err := c.editorTypography.NewTextLayout(text, f, width, height)
	if text != textInputHeightSample {
		p := l.(*testTextLayout)
		for i := range p.lines {
			p.lines[i].Baseline -= 3
		}
	}
	return l, err
}

func TestTextFieldsShareBaselineDrawingAndHitGeometry(t *testing.T) {
	setTestApplication(t, &sizingBaselineTypography{})
	v, input := NewTextView(), NewTextInput()
	v.Model().SetText("abc")
	input.SetText("abc")
	t.Cleanup(v.releaseParagraphs)
	t.Cleanup(input.releaseParagraphs)
	c := layout.Tight(geometry.Size{Width: 209, Height: 31})
	for _, field := range []Widget{v, input} {
		m := field.Measure(c)
		if m.Height != 31 || m.Baseline != 19 || !m.HasBaseline {
			t.Fatalf("shared baseline: %+v", m)
		}
		field.Arrange(geometry.Rectangle{Size: m.Size})
		p := &testLabelPainter{}
		field.Paint(p)
		if p.textOrigin.Y != 7 {
			t.Fatalf("native baseline displacement: %+v", p.textOrigin)
		}
	}
	if got := v.hitText(geometry.Point{X: 19, Y: 12}); got.Offset != 2 {
		t.Fatalf("hit did not use painted origin: %+v", got)
	}
	if caret, ok := v.caretRect(); !ok || caret.Y != 7 {
		t.Fatalf("caret did not use painted origin: %+v", caret)
	}
}

// Placeholder state, painting and lifecycle.

type placeholderEditor interface {
	Widget
	Placeholder() string
	SetPlaceholder(string)
}

func placeholderFixture(t *testing.T, singleLine bool) (placeholderEditor, *textEditor, *window, *editorTypography) {
	t.Helper()
	if singleLine {
		w, win, typo := newInputFixture(t, "")
		return w, w.textEditor, win, typo
	}
	w, win, typo := newEditorFixture(t, "")
	return w, w.textEditor, win, typo
}

func TestPlaceholderConfigurationDoesNotEdit(t *testing.T) {
	for _, singleLine := range []bool{false, true} {
		t.Run(map[bool]string{true: "input", false: "view"}[singleLine], func(t *testing.T) {
			w, e, win, _ := placeholderFixture(t, singleLine)
			if w.Placeholder() != "" {
				t.Fatal("default hint is not empty")
			}
			if err := e.model.Replace(TextRange{}, "abc"); err != nil {
				t.Fatal(err)
			}
			e.setSelectionValue(TextSelection{Anchor: 1, Caret: 2})
			win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodPreedit, Text: "候选", Caret: 3})
			preedit, selection, revision := e.preedit, e.selection, e.model.Revision()
			changes, selections := 0, 0
			e.connectChange(func(TextChange) { changes++ })
			e.connectSelection(func(TextSelection) { selections++ })
			w.SetPlaceholder("hint\r\n中\xff")
			want := "hint\n中�"
			if singleLine {
				want = "hint 中�"
			}
			if w.Placeholder() != want || e.model.Text() != "abc" || e.model.Revision() != revision ||
				e.selection != selection || e.preedit != preedit || preedit == nil || !e.model.CanUndo() || changes != 0 || selections != 0 {
				t.Fatal("setting hint changed editing state or failed normalization")
			}
			info := w.Snapshot()
			if info.TextEditing.Placeholder != want || info.TextEditing.Length != 3 || info.TextEditing.Preedit == nil || info.TextEditing.VisibleText != "abc" {
				t.Fatalf("hint leaked into document semantics: %+v", info.TextEditing)
			}
			win.layoutDirty, win.paintDirty = false, false
			w.SetPlaceholder(want)
			if win.layoutDirty || win.paintDirty {
				t.Fatal("equal normalized hint invalidated the view")
			}
			w.SetPlaceholder("")
			if win.layoutDirty || !win.paintDirty || e.preedit != preedit || changes != 0 || selections != 0 {
				t.Fatal("clearing hint changed layout, IME or emitted edit signals")
			}
		})
	}
}

func TestPlaceholderPaintTracksRealInputAndPreedit(t *testing.T) {
	for _, singleLine := range []bool{false, true} {
		t.Run(map[bool]string{true: "input", false: "view"}[singleLine], func(t *testing.T) {
			w, e, win, _ := placeholderFixture(t, singleLine)
			w.SetPlaceholder("Hint")
			paint := func(want string) {
				t.Helper()
				w.Arrange(geometry.Rect(0, 0, 100, 60))
				p := &testTextInputPainter{}
				w.Paint(p)
				got := ""
				if p.textLayout != nil {
					got = p.textLayout.Text()
				}
				if got != want {
					t.Fatalf("painted %q, want %q", got, want)
				}
			}
			paint("Hint") // Focused, with caret on top of the hint.
			win.SetFocusedWidget(nil)
			paint("Hint")
			win.SetFocusedWidget(w)
			win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodPreedit, Text: "中", Caret: 3})
			paint("中")
			if e.model.Len() != 0 || w.Snapshot().TextEditing.VisibleText != "" {
				t.Fatal("preedit or hint entered the model")
			}
			win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodPreedit})
			paint("Hint")
			win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodCommit, Text: " "})
			paint(" ")
			editorKey(t, win, events.KeyZ, textCommandModifier())
			paint("Hint")
			editorKey(t, win, events.KeyZ, textCommandModifier()|events.ModifierShift)
			paint(" ")
			editorKey(t, win, events.KeyBackspace, 0)
			paint("Hint")
			e.setReadOnly(true)
			paint("Hint")
			w.SetEnabled(false)
			paint("Hint")
			w.SetPlaceholder("")
			paint("")
		})
	}
}

func TestPlaceholderSharedModelKeepsViewStateIndependent(t *testing.T) {
	a, win, _ := newEditorFixture(t, "")
	b := NewTextView()
	b.SetModel(a.Model())
	b.SetPlaceholder("second\r\nline")
	a.SetPlaceholder("first")
	win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodPreedit, Text: "中", Caret: 3})
	if a.showsPlaceholder() || !b.showsPlaceholder() || b.Placeholder() != "second\nline" {
		t.Fatal("shared document shared hint/preedit presentation")
	}
	win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodCommit, Text: "x"})
	if a.showsPlaceholder() || b.showsPlaceholder() || b.Model().Text() != "x" {
		t.Fatal("shared commit did not hide both hints")
	}
	a.SetModel(NewTextModel(""))
	if a.Placeholder() != "first" || !a.showsPlaceholder() || b.showsPlaceholder() {
		t.Fatal("model replacement lost or shared view configuration")
	}
	b.Model().SetText("\n")
	if b.showsPlaceholder() {
		t.Fatal("a newline is real document content")
	}
}

func TestPlaceholderBaselineClipAndMeasurement(t *testing.T) {
	setTestApplication(t, &baselineInputTypography{})
	w := NewTextInput()
	before := w.Measure(layout.Unbounded())
	w.SetPlaceholder("中文 emoji 👩‍💻 " + strings.Repeat("long ", 100))
	if after := w.Measure(layout.Unbounded()); before != after {
		t.Fatalf("hint changed measurement: %+v => %+v", before, after)
	}
	for _, extra := range []float32{0, 20} {
		w.Arrange(geometry.Rect(0, 0, 100, before.Height+extra))
		size, offset := w.contentSize(), w.offset
		p := &testTextInputPainter{}
		w.Paint(p)
		lines, _ := p.textLayout.MeasureMetrics()
		if got := p.textOrigin.Y + lines[0].Baseline; got != before.Baseline+extra/2 {
			t.Fatalf("hint baseline %g, want %g", got, before.Baseline+extra/2)
		}
		if p.clipRect != geometry.Rect(4, 4, 92, before.Height+extra-8) || w.contentSize() != size || w.offset != offset {
			t.Fatal("hint changed content geometry or escaped content clipping")
		}
	}
}

func TestPlaceholderCacheStyleWidthAndUnmount(t *testing.T) {
	w, win, typo := newEditorFixture(t, "")
	w.SetStyleName("custom-editor")
	setStyle := func(size float32, fg color.Color) {
		App.SetStyleSheet(style.Sheet(append(DefaultStyleRules(),
			style.Name("custom-editor").FontSize(size),
			style.Name("custom-editor").Part("placeholder").ForegroundColor(fg),
		)...))
		w.StyleChanged()
	}
	setStyle(14, color.Black)
	w.SetPlaceholder(strings.Repeat("word ", 20))
	w.Arrange(geometry.Rect(0, 0, 100, 60))
	size := w.ContentSize()
	w.Paint(&testTextInputPainter{})
	first, calls := w.placeholderLayout, len(typo.calls)
	if first == nil || first.format.WrapMode != WrapWordChar || first.width != 91 || w.ContentSize() != size {
		t.Fatal("hint did not wrap to the view without changing document size")
	}
	w.Paint(&testTextInputPainter{})
	if w.placeholderLayout != first || len(typo.calls) != calls {
		t.Fatal("ordinary repaint reshaped hint")
	}
	setStyle(14, color.White)
	w.Paint(&testTextInputPainter{})
	if w.placeholderLayout != first || !colors.Equal(first.layout.(*testTextLayout).format.TextColor, color.White) {
		t.Fatal("color change failed or recreated layout")
	}
	w.Arrange(geometry.Rect(0, 0, 200, 60))
	w.Paint(&testTextInputPainter{})
	second := w.placeholderLayout
	if second == first || !first.layout.(*testTextLayout).destroyed || second.width != 191 {
		t.Fatal("wrapped width change retained stale hint geometry")
	}
	w.SetWrapMode(WrapNone)
	w.Arrange(geometry.Rect(0, 0, 200, 60))
	w.Paint(&testTextInputPainter{})
	if w.placeholderLayout.format.WrapMode != WrapNone || w.placeholderLayout.width != textInputMeasureExtent {
		t.Fatal("hint ignored WrapNone")
	}
	third := w.placeholderLayout
	setStyle(20, color.White)
	w.Paint(&testTextInputPainter{})
	if w.placeholderLayout == third || !third.layout.(*testTextLayout).destroyed {
		t.Fatal("font change retained native hint layout")
	}
	last := w.placeholderLayout
	win.SetWidget(nil)
	if w.placeholderLayout != nil || !last.layout.(*testTextLayout).destroyed {
		t.Fatal("unmount retained hint layout")
	}
	win.SetWidget(w)
	w.Arrange(geometry.Rect(0, 0, 200, 60))
	w.Paint(&testTextInputPainter{})
	if w.placeholderLayout == nil || w.placeholderLayout == last {
		t.Fatal("remount did not rebuild hint")
	}
}

func TestPlaceholderColorNotificationCanUnmount(t *testing.T) {
	w, win, _ := newEditorFixture(t, "")
	w.SetPlaceholder("hint")
	w.Paint(&testTextInputPainter{})
	old := w.placeholderLayout
	old.layout.ConnectChanged(func() { win.SetWidget(nil) })
	App.SetStyleSheet(style.Sheet(append(DefaultStyleRules(),
		style.Name("text-view").Part("placeholder").ForegroundColor(color.White),
	)...))
	w.Paint(&testTextInputPainter{})
	if w.placeholderLayout != nil || !old.layout.(*testTextLayout).destroyed {
		t.Fatal("unmount inside layout notification retained resources")
	}
}

func TestPlaceholderLayoutsBelongToEachView(t *testing.T) {
	setTestApplication(t, &editorTypography{})
	a, b := NewTextView(), NewTextView()
	b.SetModel(a.Model())
	box := NewLinearBox(layout.DirectionHorizontal)
	box.AddChild(a)
	box.AddChild(b)
	win := &window{}
	win.SetWidget(box)
	t.Cleanup(func() { win.SetWidget(nil) })
	for _, w := range []*TextView{a, b} {
		w.SetPlaceholder("same hint")
		w.Arrange(geometry.Rect(0, 0, 100, 60))
		w.Paint(&testTextInputPainter{})
	}
	first, second := a.placeholderLayout.layout.(*testTextLayout), b.placeholderLayout.layout.(*testTextLayout)
	if first == second {
		t.Fatal("shared model/text also shared mutable native hint layouts")
	}
	a.SetPlaceholder("")
	if !first.destroyed || second.destroyed || b.placeholderLayout == nil {
		t.Fatal("clearing one view's hint affected the other")
	}
	b.Paint(&testTextInputPainter{})
}

type reentrantPlaceholderTypography struct {
	editorTypography
	created func(typography.TextLayout)
}

func (c *reentrantPlaceholderTypography) NewTextLayout(text string, format typography.TextFormat, width, height float32) (typography.TextLayout, error) {
	l, err := c.editorTypography.NewTextLayout(text, format, width, height)
	if text == "old hint" && c.created != nil {
		c.created(l)
	}
	return l, err
}

func TestPlaceholderCreationRejectsReentrantConfiguration(t *testing.T) {
	typo := &reentrantPlaceholderTypography{}
	setTestApplication(t, typo)
	w := NewTextInput()
	w.Arrange(geometry.Rect(0, 0, 100, 40))
	w.SetPlaceholder("old hint")
	var stale *testTextLayout
	typo.created = func(l typography.TextLayout) {
		stale = l.(*testTextLayout)
		w.SetPlaceholder("new hint")
	}
	w.Paint(&testTextInputPainter{})
	if stale == nil || !stale.destroyed || w.placeholderLayout != nil {
		t.Fatal("stale native layout survived reentrant hint change")
	}
	p := &testTextInputPainter{}
	w.Paint(p)
	if p.textLayout == nil || p.textLayout.Text() != "new hint" {
		t.Fatal("new hint did not render after discarding obsolete result")
	}
}

// Check rendering order independently of native glyph rasterization.
type placeholderOrderPainter struct {
	testTextInputPainter
	ops []string
}

func (p *placeholderOrderPainter) DrawTextLayout(origin geometry.Point, l typography.TextLayout) {
	p.ops = append(p.ops, "hint")
	p.testTextInputPainter.DrawTextLayout(origin, l)
}

func (p *placeholderOrderPainter) FillRect(r geometry.Rectangle, b graphics.Brush) {
	p.ops = append(p.ops, "fill")
	p.testTextInputPainter.FillRect(r, b)
}

func TestPlaceholderCaretPaintedLast(t *testing.T) {
	w, _, _ := newInputFixture(t, "")
	w.SetPlaceholder("hint")
	p := &placeholderOrderPainter{}
	w.Paint(p)
	if strings.Join(p.ops, ",") != "fill,hint,fill" {
		t.Fatalf("paint order: %v", p.ops)
	}
}

// Caret reveal and reentrant layout requests.

func TestTextEditorRevealPreservesPendingLayout(t *testing.T) {
	v, _, win, _ := longWrappedEditor(t)
	v.SetPadding(8)
	v.SetSelection(TextSelection{Anchor: 50, Caret: 50})
	if !win.layoutDirty {
		t.Fatal("selection consumed padding invalidation")
	}
	settleEditorLayout(t, win)
	v.onPreedit("候选", len("候选"))
	if !win.layoutDirty {
		t.Fatal("composition failed to request layout")
	}
	settleEditorLayout(t, win)
	editorKey(t, win, events.KeyArrowRight, 0)
	if v.preedit != nil || !win.layoutDirty {
		t.Fatal("navigation consumed composition cancellation layout")
	}
	settleEditorLayout(t, win)
	v.Model().SetText("replacement")
	v.SetSelection(TextSelection{Anchor: 3, Caret: 3})
	if !win.layoutDirty {
		t.Fatal("selection consumed model replacement layout")
	}
	settleEditorLayout(t, win)
	if got := v.paragraphs[0].layout.Text(); got != "replacement" {
		t.Fatal("reveal retained stale paragraph")
	}
}

func TestTextEditorUnmeasuredCaretRequestsLayout(t *testing.T) {
	v, scroll, win, _, typo := timedEditorFixture(t)
	win.width, win.height = 220, 108
	settleEditorLayout(t, win)
	last := v.Model().LineCount() - 1
	if v.paragraphs[last] != nil {
		t.Fatal("fixture already measured the distant target")
	}
	calls := len(typo.calls)
	editorKey(t, win, events.KeyEnd, textCommandModifier())
	if !win.layoutDirty || !v.revealCaret || len(typo.calls) != calls {
		t.Fatal("uncached reveal must request layout without shaping in the setter")
	}
	settleEditorLayout(t, win)
	rect, ok := v.caretRect()
	if !ok || v.Selection().Caret != v.Model().Len() || v.offset.Y != scroll.ScrollY() ||
		rect.Y < v.offset.Y || rect.Y+rect.Height > v.offset.Y+v.viewport.Height {
		t.Fatal("layout did not resolve the previously unmeasured caret")
	}
}

func TestTextEditorRevealCallbackSupersedesSelection(t *testing.T) {
	for _, action := range []string{"selection", "model", "unmount", "destroy"} {
		t.Run(action, func(t *testing.T) {
			v, _, win, _ := longWrappedEditor(t)
			reveals, stale := 0, false
			v.ConnectSelection(func(s TextSelection) {
				if s.Caret == 10000 {
					stale = true
				}
			})
			handle := v.ConnectScrollIntoView(func(geometry.Rectangle) {
				reveals++
				if reveals != 1 {
					t.Fatal("reveal listener was called recursively")
				}
				switch action {
				case "selection":
					v.SetSelection(TextSelection{Anchor: 1, Caret: 1})
				case "model":
					v.SetModel(NewTextModel("new"))
				case "unmount":
					win.SetWidget(nil)
				case "destroy":
					win.SetWidget(nil)
					v.WidgetBase.destroy(v)
				}
			})
			editorKey(t, win, events.KeyEnd, textCommandModifier())
			handle.Disconnect()
			if reveals != 1 || stale {
				t.Fatalf("stale selection after reveal callback: calls=%d stale=%v", reveals, stale)
			}
			if action == "selection" {
				settleEditorLayout(t, win)
				if v.Selection().Caret != 1 || v.offset.Y != 0 {
					t.Fatal("nested selection reveal was lost")
				}
			}
		})
	}
}

// Visible selection geometry.

func TestTextEditorSelectionCullsWrappedLines(t *testing.T) {
	const count = 1000
	lines := make([]typography.TextLine, count)
	clusters := make([]typography.TextCluster, count)
	for i := range lines {
		lines[i] = typography.TextLine{Start: i, Length: 1, Y: float32(i * 20), Height: 20}
		clusters[i] = typography.TextCluster{Start: i, Length: 1, Width: 10, LineIndex: i}
	}
	paragraph := &textParagraph{
		geometry: textedit.NewGeometry(lines, clusters, count, 20), geometryValid: true,
	}
	editor := &textEditor{viewport: geometry.Size{Width: 200, Height: 48}, padding: 5}
	rects := editor.visibleSelection(paragraph, TextRange{End: count}, geometry.Point{Y: 5 - 10001})
	if len(rects) != 2 {
		t.Fatalf("selection produced %d rectangles, want 2", len(rects))
	}
	for i, rect := range rects {
		want := geometry.Rect(0, float32(10000+i*20), 10, 20)
		if rect != want {
			t.Fatalf("rectangle %d = %v, want %v", i, rect, want)
		}
	}
}
