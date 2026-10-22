// TextView window checks. Record each scenario as passed, failed or not executed.
//
// Environment: desktop session and native font libraries. Record OS, actual
// graphics backend and scale. Run with -mb 1 or -mb 10 and add -numbered to mark
// both ends of each paragraph, so repeated text cannot hide viewport jumps.
// Windows PowerShell example:
//
//	$env:GOUI_PLAT_PAINTER='direct2d'; $env:GOUI_PLAT_SCALE='1'
//	go run ./tests/window/gui/textview -mb 10 -numbered
//
// macOS/Linux example:
//
//	GOUI_PLAT_PAINTER=opengl GOUI_PLAT_SCALE=2 go run ./tests/window/gui/textview -mb 10 -numbered
//
// Repeat direct2d/opengl/software on Windows and opengl/software on macOS/Linux,
// at 1x/2x. Direct2D is preferred on Windows but may fall back after initialization
// failure. First-paint logs identify actual image types and PixelScale; record
// these rather than just requested environment values. OpenGL may use Mesa
// software rendering; record native driver details such as glxinfo too.
// Restore environment values afterward. Forced 2x does not verify cross-screen DPI.
// macOS uses Command for clipboard/undo/select-all, Option for word navigation
// and Command+Home/End for document ends; Fn+Left/Right can supply Home/End.
// Other platforms use Control for commands/word movement and Control+Home/End.
// On isolated Linux desktops without an external IME, XMODIFIERS=@im=none selects
// libX11's local input method. A stale fcitx/ibus setting with no text commits
// does not verify typing. Chinese preedit/candidate checks need a real IME in
// the same desktop and its matching XMODIFIERS.
// Initially two independent ScrollView/TextView pairs share one model; the left
// caret is at the document start.
// Steps and expected results:
//  1. Type ASCII and paste Chinese/combining characters/emoji on the left: both
//     views update, with independent carets and scroll positions. Status reports
//     byte length and each selection.
//  2. Click, Shift+arrow, drag-select and double-click a word. Selection follows
//     native clusters without splitting them. Partial bidi selection may have
//     disjoint highlighted spans.
//  3. Move vertically across long/short/long lines: retain the desired X across
//     the short line. Home/End use display lines; Control+Home/End (Command on
//     macOS) use the document.
//  4. Enter splits a line; Delete at its end or Backspace at the next line's
//     start joins them. Control/Command+Z undoes and Shift+Z redoes. Continuous
//     typing groups together; switching editing views starts a separate group.
//  5. Make the left view readonly: navigation, selection and copy still work;
//     edits/paste do not. The right view remains editable.
//  6. Wheel to distant text without the caret pulling the viewport back; keyboard
//     navigation reveals it. Select all and scroll to the last page: neither text
//     nor selection may paint over the toolbar/status outside the viewport.
//     Check Software at both 1x/2x. Toggle wrapping and resize: text/selection clips
//     and scrollbars stay correct. The top retains its logical text position
//     after reflow, although the new line may include preceding text; the last
//     page must settle. With -numbered, record the top paragraph/word, narrow/widen
//     and check the same word remains on the top display line in each independent
//     view. Return to the original width without scrolling/editing: recover the
//     same top text and clipped height, with measurement matching painted wraps.
//     Control/Command+Home then Control/Command+Shift+End must actually display
//     the final number and trailing empty-line caret; selection byte counts alone
//     do not verify navigation.
//  7. Focus the left text: the caret blinks about every 500 ms, stops when another
//     application activates and resumes on return. Hold a selection drag outside
//     its viewport: only that view autoscrolls; releasing stops it.
//  8. Replace a left selection using a Chinese IME. Preedit changes neither the
//     right view nor byte length. Commit synchronizes both; one undo restores
//     original text. Esc/focus changes cancel without deleting the original
//     selection. Windows/macOS check inline preedit and candidates; Linux uses
//     supported XIM callbacks inline or lets the IME display composition. Record
//     Fcitx 4 UseOnTheSpotStyle. Separately type a then commit U+4F60 via Pinyin:
//     first undo removes only U+4F60, the next removes a. Replacing a nonempty
//     selection does not substitute for this grouping check. Candidates stay near
//     the caret. Record libX11 for callback styles: upstream versions before 1.8.2
//     reject that style's spot location. A root-style pass does not verify callback
//     positioning; consult DesignIME for the native limitation.
//  9. Reset restores initial text and clears history. Closing during blinking or
//     composition must exit normally.
//  10. Test XIM server loss only in an isolated desktop/IME session, never by
//     stopping the user's IME. Select Edi, start inline preedit without committing,
//     then stop that session's IME. Preedit disappears; Edi and the shared model
//     stay intact. Verify selection/full text by copy; arrows and close still work.
//     Text input is unavailable and currently does not reconnect automatically.
//     Wait for process exit before cleanup. Loss inside synchronous Reset or
//     similar calls is a separate libX11 blocking limitation (DesignIME); fault
//     injection may block the process and must be recorded separately.
//  11. Without -accept-tab, normal focus navigation handles Tab and inserts no
//     text. Restart with -accept-tab: Tab inside the left text inserts U+0009,
//     increases byte length by one and updates the right view. Undo removes it.
//     The right keeps default Tab behavior; readonly left accepts no insertion.
//     Record both modes separately.
//
// See tests/window/ui/textediting for Modern, TextInput and UI binding checks;
// verify each platform separately. Side effects: explicit copy/cut changes the
// system clipboard; no files/settings are written. Reset: Reset or restart.
// Exit: close the window. No DevServer is used.
package main

import (
	"flag"
	"fmt"
	"image"
	"log"
	"runtime"
	"strings"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
)

func main() {
	runtime.LockOSThread()
	megabytes := flag.Int("mb", 1, "approximate document size (1–10 MiB)")
	numbered := flag.Bool("numbered", false, "label paragraphs for viewport anchoring checks")
	acceptTab := flag.Bool("accept-tab", false, "allow Tab insertion in the left editor only")
	flag.Parse()
	if err := run(min(10, max(1, *megabytes)), *numbered, *acceptTab); err != nil {
		log.Fatal(err)
	}
}

func run(megabytes int, numbered, acceptTab bool) error {
	app, err := gui.NewApplication("org.golang-gui.TextViewTest")
	if err != nil {
		return err
	}
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 1050, Height: 700}, Chrome: gui.WindowChromeNative})
	if err != nil {
		return err
	}
	defer win.Destroy()
	if err = win.SetTitle("GOUI TextView — shared model, independent views"); err != nil {
		return err
	}
	header := "Edit here: abc 中文 e\u0301 👩‍💻 ffi\nBidi: abc אבג def العربية xyz\nA long navigation line\nx\nA long navigation line\n\n"
	line := "Repeated paragraph: text editing, viewport layout, selection and undo. 中文测试。\n"
	var initial string
	if numbered {
		var text strings.Builder
		text.Grow(megabytes * 1024 * 1024)
		text.WriteString(header)
		for row := 0; text.Len() < megabytes*1024*1024; row++ {
			fmt.Fprintf(&text, "[%06d:start] text editing, viewport layout; [%06d:middle] selection and undo; [%06d:end] 中文测试。\n", row, row, row)
		}
		initial = text.String()
	} else {
		initial = header + strings.Repeat(line, megabytes*1024*1024/len(line))
	}
	model := gui.NewTextModel(initial)
	left, right := gui.NewTextView(), gui.NewTextView()
	left.SetModel(model)
	right.SetModel(model)
	left.SetAcceptsTab(acceptTab)
	left.SetID("editor-left")
	right.SetID("editor-right")
	scrolls := gui.NewLinearBox(layout.DirectionHorizontal)
	scrolls.SetMainWeight(1)
	scrolls.SetCrossAlign(layout.CrossStretch)
	scrolls.SetSpacing(12)
	for _, editor := range []*gui.TextView{left, right} {
		scroll := gui.NewScrollView()
		scroll.SetChild(editor)
		scroll.SetMainWeight(1)
		scrolls.AddChild(scroll)
	}
	toolbar := gui.NewLinearBox(layout.DirectionHorizontal)
	toolbar.SetSpacing(8)
	var update func()
	button := func(text string, fn func()) {
		b := gui.NewButton()
		b.SetChild(gui.NewLabel(text))
		b.ConnectClicked(fn)
		toolbar.AddChild(b)
	}
	button("左侧只读", func() { left.SetReadOnly(!left.ReadOnly()); update() })
	button("换行", func() {
		mode := gui.WrapNone
		if left.WrapMode() == gui.WrapNone {
			mode = gui.WrapWordChar
		}
		left.SetWrapMode(mode)
		right.SetWrapMode(mode)
	})
	button("复位", func() { model.SetText(initial) })
	status := &diagnosticLabel{Label: gui.NewLabel("")}
	update = func() {
		status.SetText(fmt.Sprintf("%d bytes / %d lines | L %+v | R %+v | left readonly %t", model.Len(), model.LineCount(), left.Selection(), right.Selection(), left.ReadOnly()))
	}
	connections := signal.Handles{
		model.ConnectChange(func(gui.TextChange) { update() }),
		left.ConnectSelection(func(gui.TextSelection) { update() }),
		right.ConnectSelection(func(gui.TextSelection) { update() }),
	}
	defer connections.Disconnect()
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetPadding(12)
	root.SetSpacing(8)
	root.SetCrossAlign(layout.CrossStretch)
	root.AddChild(toolbar)
	root.AddChild(scrolls)
	root.AddChild(status)
	win.SetWidget(root)
	update()
	win.SetFocusedWidget(left)
	if err = win.Show(); err != nil {
		return err
	}
	app.Run()
	return nil
}

// Diagnostic only: observe the current painter through its public image API,
// without exporting GUI internals or changing platform fallback policy.
type diagnosticLabel struct {
	*gui.Label
	reported bool
}

func (l *diagnosticLabel) Paint(p gui.Painter) {
	if !l.reported {
		l.reported = true
		img, err := p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
		if err != nil {
			log.Printf("painter probe failed: %v", err)
		} else {
			log.Printf("painter image=%T, pixel scale=%g", img, p.PixelScale())
			img.Destroy()
		}
	}
	l.Label.Paint(p)
}
