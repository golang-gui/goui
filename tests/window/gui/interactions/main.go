// Native window checks for context menus, composed tree rows, drag observation
// and autoscroll.
//
// Environment: interactive Linux/X11, Windows or macOS. Record OS/WM, backend
// and scale. Run: go run ./tests/window/gui/interactions
// Use GOUI_PLAT_PAINTER=software/opengl (also direct2d on Windows) and
// GOUI_PLAT_SCALE=2 for 2x. Initially 640x540 DIP, Modern light, first branch
// expanded, 120 numbered nodes, menus/clicks/drops=0 and scroll=0. clicks counts
// selection changes rather than raw pointer events.
// Steps and expected results:
//  1. Click the first arrow: expand/collapse only. A hidden current descendant
//     falls back to its parent under the existing tree rules. The label selects;
//     arrow, indentation and trailing space all hit the same row. Arrow-key
//     navigation still works after expansion.
//  2. Right-click trailing row space: the menu identifies that node and menus
//     increments once. Windows opens on release; Linux/macOS on press. macOS
//     Control+click also opens it without ordinary clicks or drag Begin. Esc closes.
//  3. Select a row and press Shift+F10: target Current, not the last pointer
//     position. Holding the key must not reopen repeatedly. Also test the menu
//     key on Windows/Linux. After Esc, right-click still works; caption/buttons
//     retain their usual actions.
//  4. Drag Drag source to a middle row: blue preview, outlined row, observer=true
//     and Target shown. Release: drops+1, Drop ID matches the outlined row,
//     observer=false and status stops changing. Leaving the list clears the outline.
//  5. Hold at the bottom edge for at least one second: scroll keeps increasing
//     and Target follows it. The viewport center stops scrolling; the top reverses
//     it. Release: Drop ID matches the last Target. Completion or Esc stops scrolling.
//  6. Set Auto scroll: OFF and repeat: drops still work, scroll stays unchanged.
//     Re-enable and Reset: scroll=0 and subsequent drags still work.
//  7. Drag a file from the native file manager to an edge: autoscroll and drops+1
//     on release, without file modification. Repeat steps 1-6 across 1x/2x,
//     light/dark and window sizes; hits, menus and drops stay correct.
//
// Platform differences: menu timing is described above; native cancellation
// feedback varies. Side effects: menus/previews only, no file/clipboard/settings
// writes. Reset: Reset or restart. Exit: close the window. Errors show FAIL in
// status/logs and cause a nonzero exit.
package main

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/widgets"
)

type probe struct {
	tree                           *widgets.TreeView
	model                          *widgets.TreeStore[string]
	scroll                         *gui.ScrollView
	status                         *gui.Label
	target, drop                   string
	menus, clicks, drops, failures int
	observed                       bool
}

type row struct {
	*widgets.TreeExpander
	label *gui.Label
	id    string
	probe *probe
}

func (r *row) Paint(p gui.Painter) {
	if r.id != "" && r.id == r.probe.target {
		p.DrawRoundRect(geometry.Rect(.5, .5, max(0, r.Rect().Width-1), max(0, r.Rect().Height-1)), 4, 1, graphics.RGB(30, 110, 220))
	}
}

func (s *probe) Setup() gui.Widget {
	r := &row{TreeExpander: widgets.NewTreeExpander(), label: gui.NewLabel(""), probe: s}
	r.label.SetStyleName("tree-item-text")
	r.SetChild(r.label)
	r.ConnectToggle(func() {
		if r.id != "" {
			s.tree.SetExpanded(r.id, !s.tree.Expanded(r.id))
		}
	})
	return r
}
func (s *probe) Bind(state widgets.TreeRow, w gui.Widget) {
	r := w.(*row)
	r.id = state.ID
	r.label.SetText(s.model.Item(state.ID))
	r.SetDepth(state.Depth)
	r.SetIndentation(s.tree.Indentation())
	r.SetExpandable(state.Expandable)
	r.SetExpanded(state.Expanded)
}
func (*probe) Unbind(_ widgets.TreeRow, w gui.Widget) {
	r := w.(*row)
	r.id = ""
	r.SetExpandable(false)
}
func (s *probe) feedback(id string) {
	if s.target != id {
		s.target = id
		s.tree.RequestPaint()
		fmt.Printf("target=%s scroll=%.0f\n", id, s.scroll.ScrollY())
	}
}
func (s *probe) fail(err error) { s.failures++; fmt.Printf("FAIL: %v\n", err) }

func main() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	app, err := gui.NewApplication("org.golang-gui.InteractionProbe")
	if err != nil {
		log.Fatal(err)
	}
	app.SetStyleSheet(modern.Sheet(modern.Options{}))
	window, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 640, Height: 540}, Chrome: gui.WindowChromeIntegrated})
	if err != nil {
		log.Fatal(err)
	}
	defer window.Destroy()
	_ = window.SetTitle("GOUI Interaction Probe")
	s := &probe{tree: widgets.NewTreeView(), model: widgets.NewTreeStore[string](), scroll: gui.NewScrollView(), status: gui.NewLabel("")}
	s.model.Modify(func(m *widgets.TreeStore[string]) {
		m.Append("", "folder", "Folder: click the disclosure arrow")
		for i := 0; i < 120; i++ {
			m.Append("folder", fmt.Sprintf("row-%03d", i), fmt.Sprintf("Item %03d — full row drag/drop", i))
		}
	})
	s.tree.SetModel(s.model)
	s.tree.SetDelegate(s)
	s.tree.SetExpanded("folder", true)
	s.tree.ConnectSelection(func([]string) { s.clicks++ })
	s.tree.ConnectContextMenu(func(id string, result *gui.MenuModel) {
		s.menus++
		fmt.Printf("menu=%s count=%d\n", id, s.menus)
		menu := gui.NewMenu()
		menu.Append("Context: "+id, nil).SetEnabled(false)
		menu.Append("Close menu", func() {})
		*result = menu
	})
	s.tree.ConnectContextMenuError(s.fail)
	s.scroll.SetChild(s.tree)
	target := gui.NewDropTarget(gui.DragFormatText, gui.DragFormatFiles)
	motion := func(e *gui.DragMotion) {
		if r, _, ok := s.tree.RowAt(e.Position); ok {
			s.feedback(r.ID)
		} else {
			s.feedback("")
			e.Action = 0
		}
	}
	target.ConnectEnter(motion)
	target.ConnectMotion(motion)
	target.ConnectLeave(func() { s.feedback("") })
	target.ConnectDrop(func(e *gui.DropRequest) {
		r, _, ok := s.tree.RowAt(e.Position)
		if !ok {
			return
		}
		if r.ID != s.target {
			s.fail(fmt.Errorf("drop=%s but feedback=%s", r.ID, s.target))
			return
		}
		s.drops++
		s.drop = r.ID
		e.Accepted = true
		fmt.Printf("drop=%s count=%d\n", r.ID, s.drops)
	})
	target.ConnectError(s.fail)
	s.tree.AddEventController(target)
	observer := gui.NewDragMotionEventController()
	observer.ConnectEnter(func(geometry.Point) { s.observed = true })
	observer.ConnectLeave(func() { s.observed = false })
	s.scroll.AddEventController(observer)
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetCrossAlign(layout.CrossStretch)
	root.SetSpacing(8)
	root.SetPadding(12)
	root.AddChild(gui.NewLabel("Menu / disclosure / drag at viewport edges"))
	toolbar := gui.NewLinearBox(layout.DirectionHorizontal)
	toolbar.SetSpacing(8)
	button := func(text string, fn func()) *gui.Button {
		b := gui.NewButton()
		b.SetChild(gui.NewLabel(text))
		if fn != nil {
			b.ConnectClicked(fn)
		}
		toolbar.AddChild(b)
		return b
	}
	sourceButton := button("Drag source", nil)
	source := gui.NewDragSource()
	preview := image.NewRGBA(image.Rect(0, 0, 48, 32))
	for y := 2; y < 30; y++ {
		for x := 2; x < 46; x++ {
			preview.SetRGBA(x, y, color.RGBA{R: 30, G: 110, B: 220, A: 255})
		}
	}
	source.ConnectPrepare(func(e *gui.DragPrepare) {
		e.Data = new(gui.DragData)
		e.Data.SetText("interaction probe")
		e.Preview = gui.DragPreview{Image: preview, Scale: 1, Hotspot: geometry.Point{X: 12, Y: 12}}
	})
	source.ConnectEnd(func(r gui.DragResult) {
		if r.Err != nil {
			s.fail(r.Err)
		}
		fmt.Printf("end action=%d canceled=%v\n", r.Action, r.Canceled)
	})
	sourceButton.AddEventController(source)
	auto := button("Auto scroll: ON", nil)
	auto.ConnectClicked(func() {
		enabled := !s.scroll.DragAutoScroll()
		s.scroll.SetDragAutoScroll(enabled)
		text := "Auto scroll: OFF"
		if enabled {
			text = "Auto scroll: ON"
		}
		auto.Child().(*gui.Label).SetText(text)
	})
	button("Reset", func() { s.scroll.SetScrollY(0); s.feedback("") })
	dark := false
	button("Theme", func() { dark = !dark; app.SetStyleSheet(modern.Sheet(modern.Options{Dark: dark})) })
	root.AddChild(toolbar)
	root.AddChild(s.scroll)
	root.AddChild(s.status)
	window.SetWidget(root)
	last := ""
	timer := app.NewTimer()
	timer.ConnectTimeout(func() {
		text := fmt.Sprintf("menus=%d clicks=%d drops=%d FAIL=%d\nobserver=%v Target=%s Drop=%s scroll=%.0f", s.menus, s.clicks, s.drops, s.failures, s.observed, s.target, s.drop, s.scroll.ScrollY())
		if text != last {
			last = text
			s.status.SetText(text)
		}
	})
	if err := timer.Start(100 * time.Millisecond); err != nil {
		log.Fatal(err)
	}
	defer timer.Stop()
	if err := window.Show(); err != nil {
		log.Fatal(err)
	}
	app.Run()
	if s.failures > 0 {
		os.Exit(1)
	}
}
