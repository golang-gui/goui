// UI text-editing window checks: shared multiline models, independent single-line
// editing and declarative rebuilds.
//
// Environment: desktop and font libraries. Record OS, actual backend, 1x/2x scale
// and Linux WM. Run with -mb 1 or -mb 10. Windows PowerShell:
//
//	$env:GOUI_PLAT_PAINTER='direct2d'; $env:GOUI_PLAT_SCALE='1'
//	go run ./tests/window/ui/textediting -mb 10
//
// macOS/Linux:
//
//	GOUI_PLAT_PAINTER=opengl GOUI_PLAT_SCALE=2 go run ./tests/window/ui/textediting -mb 10
//
// Repeat direct2d/opengl/software on Windows, opengl/software on macOS/Linux,
// at 1x/2x. Direct2D may fall back; first-frame resource types and PixelScale
// identify the actual backend. Submit logs count invocations. Restore environment
// values afterward; forced 2x does not verify real DPI/cross-screen candidates.
// Use Command for the editing shortcuts below on macOS; Fn+Left/Right supply
// Home/End where needed. On isolated Linux without an external IME, use
// XMODIFIERS=@im=none for libX11 local input. A stale fcitx/ibus setting without
// its service does not verify typing. Type draft key-by-key in the second empty
// input, then Enter: exactly draft (5 copied bytes) and one Submit. Pasting does
// not verify typing. Chinese preedit needs the matching IME on the same desktop;
// a missing-service configuration verifies only unavailability.
// Initially two single-line inputs sit above value-bound Notes and two multiline
// views sharing a model, in Modern light. The first input binds Text/Selection;
// the second omits Text. State and the shared model are created outside Build.
// Steps and expected results:
//  1. Position/Shift-select in the first input and type/paste multiline text:
//     replace selection, turn each normalized newline into one space, retain
//     single-line height and horizontal reveal. Label/input baselines align;
//     combining characters and emoji navigate/delete without splitting clusters.
//  2. Enter emits one Submit without a space/LF; rebuild retains the caret.
//     Control/Command+Z undoes, Shift+Control/Command+Z redoes. A paste or IME
//     commit is undone as one operation.
//  3. Type in the second field and Rebuild: retain text. Repeated Theme/Accent
//     changes retain all text/selections while updating background/selection/caret
//     without cumulative font scaling.
//  4. Edit lower-left: lower-right synchronizes with independent scroll/selection.
//     Home/End, document ends, paragraph-spanning selection, autoscroll, clicks,
//     double-clicks and readonly behavior follow the GUI TextView checks. Readonly
//     affects the controlled single-line input and lower-left only; Copy/selection
//     remain available and lower-right stays editable.
//  5. Real IME preedit/cancel leaves Text/shared model untouched; commit synchronizes.
//     Check inline composition/candidates on Windows/macOS; XIM root-style
//     composition is displayed by the IME.
//  6. Scroll/resize at 1/10 MiB and 1x/2x: retain visible text anchors and clip
//     caret/selection to the inputs. Closing during blink/preedit leaves no windows.
//  7. In the initial first input, End then Shift+Left copies the complete 👩‍💻.
//     Left, Left, Shift+Left then copies e plus combining acute (3 UTF-8 bytes).
//     In the second input paste אבג; after the async paste completes, select all,
//     Left, Shift+Right: copy ג. Backspace leaves אב; undo restores אבג and selected ג.
//     Do not change selection before paste completes: stale replies should be
//     dropped and cannot verify successful or failed paste delivery.
//  8. In an isolated Linux desktop only, choose fr layout and XMODIFIERS=@im=none.
//     Type a/z/é, dead ^ then e, AltGr+E into the second empty input. Copy exactly
//     azéê€ (9 UTF-8 bytes), not US-position q/w/2. Replace initial Edi in lower-left
//     with the same keys then Enter; copy from the right: "azéê€\n" plus the initial
//     text after its first three bytes, with all other content unchanged. Keep
//     running, switch only that isolated desktop to us intl, select all in the
//     second input and type q/w, dead apostrophe then e: copy qwé (4 bytes).
//     Record layout/IME; local Compose is not Chinese IME verification. Exit before
//     cleaning up the session; never change the user's working desktop.
//  9. Edit Notes, move the caret and reverse selection; its byte/selection status
//     follows. Rebuild/Theme retains state and history. Replace notes sets
//     "Replacement\n\u4e2d\u6587", caret=0 and clears old history. Select notes chooses
//     U+4E2D U+6587 (bytes 12-18); Empty notes clears it. Enable Auto rebuild and
//     compose with Chinese IME in Notes: one-second rebuilds neither count preedit
//     as committed text nor interrupt it; commit updates text, cancel does not.
//     Stop Auto rebuild afterward; exit also stops it. Avoid clicking during
//     composition, which changes focus and can cancel independently of rebuilds.
//  10. Right-click inside selection in a single-line field and Notes, including
//     the last character's right half: Undo/Redo, Cut/Copy/Paste/Delete and Select All
//     preserve selection. Outside selection moves the caret; also test macOS
//     Control+click. Menus use Modern corners/shadow/colors. Copy retains text;
//     Cut/Delete remove only selection; Primary+Z restores text/selection in one
//     step. Paste replaces it, normalizing newlines only in single-line inputs;
//     Undo/Redo reverse each other. With no selection Cut/Copy/Delete are disabled;
//     readonly permits only Copy/Select All. Opening alone accesses no clipboard;
//     empty/nontext clipboard paste changes nothing.
//  11. Shift+F10 opens the same menu below the caret; Escape closes it. An outside
//     click closes without moving the editor caret. Ordinary menu-time keys do
//     not reach the editor; typing resumes immediately after close. Scroll Notes
//     and reopen near the click; near desktop edges the menu remains visible.
//     Right-click cancels uncommitted Chinese preedit without adding history;
//     Copy contains only committed text. Closing the owner leaves no native popup
//     and does not prevent exit.
//
// Side effects: explicit copy/cut writes clipboard; no file/settings changes or
// DevServer. Reset: restart. Exit: close the window. Record each platform and
// scenario separately; startup is not interaction verification.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"runtime"
	"strings"
	"time"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
)

func main() {
	runtime.LockOSThread()
	mb := flag.Int("mb", 1, "approximate document size (1–10 MiB)")
	flag.Parse()
	line := "Repeated paragraph: editing, viewport, selection and undo. 中文测试。\n"
	initial := "Edit here: abc 中文 e\u0301 👩‍💻 ffi\nBidi: abc אבג def العربية xyz\n\n" + strings.Repeat(line, min(10, max(1, *mb))*1024*1024/len(line))
	model := ui.NewTextModel(initial)
	text := ui.MakeState("Search 中文 e\u0301 👩‍💻")
	searchSelection := ui.MakeState(ui.TextSelection{})
	notes := ui.MakeState("Notes: edit this small value-bound document.\n中文 e\u0301 👩‍💻")
	notesSelection := ui.MakeState(ui.TextSelection{})
	var rebuildTimer *ui.Timer
	dark, readonly, bare := false, false, false
	submits, rebuilds, accent := 0, 0, 0
	accents := []color.Color{nil, color.RGBA{R: 140, G: 70, B: 220, A: 255}, color.Black}
	var leftSelection, rightSelection ui.TextSelection
	if err := ui.Run("org.golang-gui.TextEditingUITest", func(app ui.App) ui.RootView {
		settings := app.Settings()
		var sheet style.StyleSheet
		if !bare {
			sheet = modern.Sheet(modern.Options{Dark: dark, AccentColor: accents[accent], FontFamily: settings.FontFamily(), FontSize: settings.FontSize()})
		}
		onSubmit := func() {
			submits++
			log.Printf("submit=%d", submits)
			app.RequestUpdate()
		}
		return ui.Root().StyleSheet(sheet).Windows(
			ui.Window("editing").Title("GOUI UI Text Editing").Chrome(ui.WindowChromeNative).Size(1080, 720).Content(
				ui.VBox(
					ui.HBox(
						ui.Button("Theme").OnClick(func() { dark = !dark; app.RequestUpdate() }),
						ui.Button("Accent").OnClick(func() { accent = (accent + 1) % len(accents); app.RequestUpdate() }),
						ui.Button("Bare GUI").OnClick(func() { bare = !bare; app.RequestUpdate() }),
						ui.Button("Rebuild").OnClick(func() { rebuilds++; app.RequestUpdate() }),
						ui.Button("Readonly").OnClick(func() { readonly = !readonly; app.RequestUpdate() }),
					).Spacing(8),
					ui.HBox(
						ui.Label("Search:"),
						ui.TextInput().ID("search").BindText(&text).BindSelection(&searchSelection).OnSubmit(onSubmit).ReadOnly(readonly).MainWeight(1),
						ui.Label("Uncontrolled:"),
						ui.TextInput().ID("uncontrolled").OnSubmit(onSubmit).MainWeight(1),
					).CrossAlign(layout.CrossBaseline).Spacing(8),
					ui.HBox(
						ui.Button("Replace notes").OnClick(func() {
							notes.Set("Replacement\n中文")
							notesSelection.Set(ui.TextSelection{})
						}),
						ui.Button("Select notes").OnClick(func() {
							notesSelection.Set(ui.TextSelection{Anchor: 12, Caret: len(notes.Get())})
						}),
						ui.Button("Empty notes").OnClick(func() {
							notes.Set("")
							notesSelection.Set(ui.TextSelection{})
						}),
						ui.Button("Auto rebuild").OnClick(func() {
							if rebuildTimer.Active() {
								rebuildTimer.Stop()
							} else if rebuildTimer != nil {
								rebuildTimer.Start(time.Second)
							} else {
								rebuildTimer = app.TickFunc(time.Second, func() { rebuilds++; app.RequestUpdate() })
							}
							app.RequestUpdate()
						}),
					).Spacing(8),
					ui.ScrollView(ui.TextView().ID("notes").BindText(&notes).
						BindSelection(&notesSelection).ReadOnly(readonly)).MinHeight(100).MaxHeight(100),
					ui.Label(fmt.Sprintf("Notes: %d bytes | selection %+v | Search %+v | auto rebuild %t", len(notes.Get()), notesSelection.Get(), searchSelection.Get(), rebuildTimer.Active())),
					ui.HBox(
						ui.ScrollView(ui.TextView().ID("left").Model(model).ReadOnly(readonly).
							OnChange(func(ui.TextChange) { app.RequestUpdate() }).
							OnSelection(func(s ui.TextSelection) { leftSelection = s; app.RequestUpdate() })),
						ui.ScrollView(ui.TextView().ID("right").Model(model).
							OnSelection(func(s ui.TextSelection) { rightSelection = s; app.RequestUpdate() })),
					).Spacing(12).CrossAlign(layout.CrossStretch).MainWeight(1),
					statusLabel(fmt.Sprintf("%d bytes | L %+v | R %+v | submits %d | rebuilds %d | readonly %t", model.Len(), leftSelection, rightSelection, submits, rebuilds, readonly)),
				).Padding(12).Spacing(10).CrossAlign(layout.CrossStretch),
			),
		)
	}); err != nil {
		log.Fatal(err)
	}
}

// A normal custom WidgetView: diagnostics use public drawing APIs, not private
// UI reconciliation state or an exported test-only production interface.
type statusView struct {
	ui.ViewBase[statusView]
	text string
}

func statusLabel(text string) *statusView {
	v := &statusView{text: text}
	v.Self = v
	return v
}

func (v *statusView) Build() ui.View { return v }
func (v *statusView) Mount(ui.BuildContext) gui.Widget {
	return &diagnosticLabel{Label: gui.NewLabel(v.text)}
}
func (v *statusView) Update(_ ui.BuildContext, w gui.Widget) {
	w.(*diagnosticLabel).SetText(v.text)
}
func (*statusView) Unmount(ui.BuildContext, gui.Widget) {}

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
