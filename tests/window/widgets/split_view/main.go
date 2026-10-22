// SplitView two-pane/nested layout and input window checks, without DevServer.
//
// Environment: interactive Linux/X11, Windows or macOS. Record OS, Linux WM,
// actual backend and scale. Run from the repository root:
//
//	go run ./tests/window/widgets/split_view
//
// Default UI bindings can be replaced by -gui for imperative controls.
// Use GOUI_PLAT_PAINTER=software/opengl (also direct2d on Windows),
// GOUI_PLAT_SCALE=2. -fallback-style uses explicit GUI+widget defaults; environment
// values are requests only. Initially Native 900x600 DIP, Modern light, left
// minimum 120 DIP and right minimum 240 DIP. Editor and terminal are vertically
// split, terminal initially fixed at 140 DIP; left content accepts text. Default
// outer ratio follows measured panes, not necessarily 0.5; minimum window hint
// is 360x260 DIP.
// Steps and expected results:
//  1. Drag the horizontal split's separator within 4 DIP on either side. Panes
//     allocate continuously without initial jump or bounce; hover uses a resize
//     cursor, while the line stays a neutral 1 DIP. Repeat vertically, releasing
//     outside to stop; content stays clipped to its own pane. On Windows rapidly
//     drag left/right for at least 20 seconds, then vertically: remain responsive
//     without 0xC0000005. Snapshot assertions still pass after release.
//  2. Focus the separator and use axis arrows/Shift+arrows for 1/10 DIP, Home/End
//     for minimum-size limits. Cross-axis or Control+arrows do not resize. Esc
//     during dragging restores the initial size; release does not reapply the
//     canceled position. Leaving after drag removes hover emphasis while arrow
//     adjustment remains available with focus.
//  3. Resize the window after dragging: Ratio retains proportions, Start/End
//     retain preferred DIP. Temporary compression recovers when room returns;
//     insufficient combined minimums cause no negative dimensions/crash. Nested
//     vertical panes remain independently adjustable.
//  4. Mode cycles measured ratio/Start 220/End 400/Ratio 0.5. GUI dragging updates
//     preferences; UI OnResize updates its model then rebuilds. Rebuild preserves
//     uncontrolled ratio, input text/selection and scroll positions.
//  5. Hide/Show left: right fills and outer separator disappears; showing
//     restores sizing policy. Toggle Dark changes neutral separator color only.
//  6. Snapshot logs actual pane/separator dimensions. Assertions check pane
//     sum plus 1 DIP against the container axis and real subtree/separator bounds.
//     ASSERT PASS verifies geometry, not the input/appearance steps above.
//
// Platform differences: this adds no window-decoration behavior; activate the
// macOS window before input. Side effects: this window, in-memory text and logs;
// no application files, clipboard or settings changed. Reset: restart.
// Exit: close the window; any geometry failure panics and exits nonzero.
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"runtime"
	"strings"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	widgetstyle "github.com/golang-gui/goui/widgets/style"
	wui "github.com/golang-gui/goui/widgets/ui"
)

var modeNames = []string{"Content ratio", "Start 220", "End 400", "Ratio 0.5"}
var document = strings.Repeat("SplitView text: edit, select and scroll independently.\n", 80)

func main() {
	guiMode := flag.Bool("gui", false, "use imperative GUI instead of UI binding")
	fallback := flag.Bool("fallback-style", false, "compose GUI and widgets fallback rules")
	flag.Parse()
	var err error
	if *guiMode {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		err = runGUI(*fallback)
	} else {
		err = runUI(*fallback)
	}
	if err != nil {
		log.Fatal(err)
	}
}
func sheet(dark, fallback bool) style.StyleSheet {
	if fallback {
		return style.Sheet(append(gui.DefaultStyleRules(), widgetstyle.Rules()...)...)
	}
	return modern.Sheet(modern.Options{Dark: dark})
}
func assertSplit(view *widgets.SplitView) {
	a, b := view.Sizes()
	extent := view.Rect().Width
	if view.Direction() == layout.DirectionVertical {
		extent = view.Rect().Height
	}
	info := view.Snapshot()
	active := view.StartChild() != nil && view.StartChild().Visible() && view.EndChild() != nil && view.EndChild().Visible()
	if active {
		if math.Abs(float64(a+b+1-extent)) > .01 || a < 0 || b < 0 {
			panic("split allocation does not fill view")
		}
		separator := info.Children[len(info.Children)-1]
		if separator.Role != widgets.RoleSeparator || separator.Range == nil || separator.Range.Value != a || a < separator.Range.Min-.01 || a > separator.Range.Max+.01 {
			panic("separator snapshot/range incorrect")
		}
		bounds, parent := separator.Bounds, info.Bounds
		if bounds.X < parent.X-.01 || bounds.Y < parent.Y-.01 || bounds.X+bounds.Width > parent.X+parent.Width+.01 || bounds.Y+bounds.Height > parent.Y+parent.Height+.01 {
			panic("separator hit bounds escaped split view")
		}
		hitWidth := bounds.Width
		if view.Direction() == layout.DirectionVertical {
			hitWidth = bounds.Height
		}
		if hitWidth < 0 || hitWidth > 9.01 || separator.Range.Direction != view.Direction() {
			panic("separator hit width/direction incorrect")
		}
		fmt.Printf("separator bounds=%v range=%+v\n", separator.Bounds, *separator.Range)
	}
	fmt.Printf("ASSERT PASS %s sizes=(%.1f,%.1f) extent=%.1f\n", view.ID(), a, b, extent)
}
func runUI(fallback bool) error {
	model := ui.NewTextModel(document)
	mode, dark, hidden, rebuilds := 0, false, false, 0
	startSize, endSize, ratio, terminal := float32(220), float32(400), float32(.5), float32(140)
	return ui.Run("org.golang-gui.SplitViewProbe", func(app ui.App) ui.RootView {
		left := ui.VBox(ui.Label("Sidebar / left"), ui.TextInput().ID("sidebar-input"), ui.Label("Minimum: 120 DIP")).Padding(12).Spacing(8).MinSize(120, 0).Hidden(hidden)
		right := wui.VSplit(
			ui.ScrollView(ui.TextView().ID("editor").Model(model)).MinSize(0, 100),
			ui.ScrollView(ui.Label(strings.Repeat("Terminal output\n", 40))).MinSize(0, 60),
		).ID("inner").MinSize(240, 0).EndSize(terminal).OnResize(func(_, end float32) { terminal = end; app.RequestUpdate() })
		split := wui.HSplit(left, right).ID("outer").OnResize(func(start, end float32) {
			switch mode {
			case 1:
				startSize = start
			case 2:
				endSize = end
			case 3:
				ratio = start / (start + end)
			}
			fmt.Printf("outer resize %.1f %.1f\n", start, end)
			app.RequestUpdate()
		})
		switch mode {
		case 1:
			split.StartSize(startSize)
		case 2:
			split.EndSize(endSize)
		case 3:
			split.Ratio(ratio)
		}
		return ui.Root().StyleSheet(sheet(dark, fallback)).Windows(
			ui.Window("split").Title("GOUI SplitView UI").Size(900, 600).MinSize(360, 260).Content(
				ui.VBox(
					ui.HBox(
						ui.Button("Mode").OnClick(func() { mode = (mode + 1) % 4; startSize, endSize, ratio = 220, 400, .5; app.RequestUpdate() }),
						ui.Button("Rebuild").OnClick(func() { rebuilds++; app.RequestUpdate() }),
						ui.Button("Hide/Show").OnClick(func() { hidden = !hidden; app.RequestUpdate() }),
						ui.Button("Toggle Dark").OnClick(func() { dark = !dark; app.RequestUpdate() }),
						ui.Button("Snapshot").OnClick(func() {
							assertSplit(app.FindWidget("split", "outer").(*widgets.SplitView))
							assertSplit(app.FindWidget("split", "inner").(*widgets.SplitView))
						}),
					).Spacing(8).Padding(8),
					ui.Label(fmt.Sprintf("%s | Rebuilds=%d | Drag; arrows / Shift / Home / End / Esc", modeNames[mode], rebuilds)),
					split,
				).CrossAlign(layout.CrossStretch),
			),
		)
	})
}
func runGUI(fallback bool) error {
	app, err := gui.NewApplication("org.golang-gui.SplitViewGUIProbe")
	if err != nil {
		return err
	}
	app.SetStyleSheet(sheet(false, fallback))
	window, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 900, Height: 600}})
	if err != nil {
		return err
	}
	defer window.Destroy()
	if err = window.SetTitle("GOUI SplitView GUI"); err != nil {
		return err
	}
	window.SetMinSize(geometry.Size{Width: 360, Height: 260})
	left := gui.NewLinearBox(layout.DirectionVertical)
	left.SetPadding(12)
	left.SetSpacing(8)
	left.SetMinSize(geometry.Size{Width: 120})
	left.AddChild(gui.NewLabel("Sidebar / left"))
	left.AddChild(gui.NewTextInput())
	left.AddChild(gui.NewLabel("Minimum: 120 DIP"))
	editor := gui.NewTextView()
	editor.SetModel(gui.NewTextModel(document))
	scroll := gui.NewScrollView()
	scroll.SetChild(editor)
	scroll.SetMinSize(geometry.Size{Height: 100})
	terminal := gui.NewScrollView()
	terminal.SetChild(gui.NewLabel(strings.Repeat("Terminal output\n", 40)))
	terminal.SetMinSize(geometry.Size{Height: 60})
	inner := widgets.NewSplitView(layout.DirectionVertical)
	inner.SetID("inner")
	inner.SetStartChild(scroll)
	inner.SetEndChild(terminal)
	inner.SetEndSize(140)
	inner.SetMinSize(geometry.Size{Width: 240})
	outer := widgets.NewSplitView(layout.DirectionHorizontal)
	outer.SetID("outer")
	outer.SetStartChild(left)
	outer.SetEndChild(inner)
	outer.ConnectResize(func(a, b float32) { fmt.Printf("outer resize %.1f %.1f\n", a, b) })
	inner.ConnectResize(func(a, b float32) { fmt.Printf("inner resize %.1f %.1f\n", a, b) })
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetCrossAlign(layout.CrossStretch)
	toolbar := gui.NewLinearBox(layout.DirectionHorizontal)
	toolbar.SetPadding(8)
	toolbar.SetSpacing(8)
	mode, dark, rebuilds := 0, false, 0
	status := gui.NewLabel(modeNames[0])
	button := func(text string, fn func()) {
		b := gui.NewButton()
		b.SetChild(gui.NewLabel(text))
		b.ConnectClicked(fn)
		toolbar.AddChild(b)
	}
	button("Mode", func() {
		mode = (mode + 1) % 4
		switch mode {
		case 0: // Retain current proportional allocation, like UI omission.
			a, b := outer.Sizes()
			if a+b > 0 {
				outer.SetRatio(a / (a + b))
			}
		case 1:
			outer.SetStartSize(220)
		case 2:
			outer.SetEndSize(400)
		case 3:
			outer.SetRatio(.5)
		}
		status.SetText(modeNames[mode])
	})
	button("Rebuild", func() {
		rebuilds++
		status.SetText(fmt.Sprintf("%s | Rebuilds=%d", modeNames[mode], rebuilds))
		root.RequestLayout()
	})
	button("Hide/Show", func() { left.SetVisible(!left.Visible()) })
	button("Toggle Dark", func() { dark = !dark; app.SetStyleSheet(sheet(dark, fallback)) })
	button("Snapshot", func() { assertSplit(outer); assertSplit(inner) })
	root.AddChild(toolbar)
	root.AddChild(status)
	root.AddChild(outer)
	window.SetWidget(root)
	if err = window.Show(); err != nil {
		return err
	}
	app.Run()
	return nil
}
