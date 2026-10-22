// Linux Integrated window checks for content insets on all four sides.
//
// Environment: Linux X11 with a compositing WM. Record desktop, actual backend
// and scale. Run: go run ./tests/window/gui/integrated_layout
// Repeat with GOUI_PLAT_PAINTER=software/opengl and GOUI_PLAT_SCALE=1 or 2.
// Initially the Integrated window has a 40 DIP HeaderBar, no root Padding and
// content overflowing both axes.
// Steps and expected results:
//  1. Both scrollbars remain a full 8 DIP wide, with window background outside
//     the content on every side. Drag to either end and back; outer edges remain
//     fully visible and hittable.
//  2. Resize from all sides/corners. The top edge outside HeaderBar resizes;
//     the caption moves, and caption buttons do not move the window. Edge input
//     must not reach widgets underneath.
//  3. F7 switches title height between 40/64 DIP. Window controls stay vertically
//     centered without duplicated right insets. F8 maximizes, removing edge
//     margins; F9 restores them.
//  4. Type ASCII/Chinese in an input and navigate with Tab. IME candidates follow
//     the caret. F10 enters fullscreen; F9 restores, with no stale scrollbar/title
//     offsets. Controls are hidden in fullscreen.
//  5. F6 checks public Rect/Snapshot and scrollbar bounds, printing LAYOUT PASS.
//     F11 toggles light/dark. Repeat at 1x/2x: an 8 DIP scrollbar spans 8/16 pixels.
//
// Only Linux client-drawn Integrated adds these GUI insets; Windows/macOS do not.
// Without a compositor Linux may fall back to Native and report not applicable;
// that is not a passed Integrated check.
// Side effects: window and logs only; no automatic clipboard/file access,
// system changes or DevServer. Reset: F9 for normal state, restart to reset scroll
// and title height. Exit: close the main window.
package main

import (
	"fmt"
	"image"
	"log"
	"math"
	"runtime"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
)

type backendProbe struct {
	gui.WidgetBase
	scale    float32
	reported bool
}

func (b *backendProbe) Paint(p gui.Painter) {
	b.scale = p.PixelScale()
	if !b.reported {
		b.reported = true
		img, err := p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
		if err != nil {
			panic(err)
		}
		fmt.Printf("backend=%T scale=%g\n", img, b.scale)
		img.Destroy()
	}
}

func main() {
	runtime.LockOSThread()
	app, err := gui.NewApplication("org.golang-gui.IntegratedLayout")
	if err != nil {
		log.Fatal(err)
	}
	dark := false
	app.SetStyleSheet(modern.Sheet(modern.Options{}))
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 760, Height: 560}, Chrome: gui.WindowChromeIntegrated})
	if err != nil {
		log.Fatal(err)
	}
	defer win.Destroy()
	if err := win.SetTitle("GOUI Integrated layout"); err != nil {
		log.Fatal(err)
	}
	var info gui.ChromeInfo
	win.Chrome().ConnectInfo(func(v gui.ChromeInfo) { info = v })
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetCrossAlign(layout.CrossStretch)
	header := gui.NewHeaderBar()
	header.SetPadding(4)
	height := float32(40)
	row := gui.NewLinearBox(layout.DirectionHorizontal)
	row.SetSpacing(12)
	row.AddChild(gui.NewLabel("Integrated layout — F6 check / F7 height"))
	toggleHeight := func() {
		height = 104 - height
		header.SetMinSize(geometry.Size{Height: height})
	}
	button := gui.NewButton()
	button.SetPadding(4)
	button.SetChild(gui.NewLabel("40 / 64 DIP"))
	button.ConnectClicked(toggleHeight)
	row.AddChild(button)
	probe := &backendProbe{scale: 1}
	probe.SetMinSize(geometry.Size{Width: 1, Height: 1})
	row.AddChild(probe)
	header.SetChild(row)
	root.AddChild(header)
	content := gui.NewLinearBox(layout.DirectionVertical)
	content.SetMinSize(geometry.Size{Width: 1000})
	content.SetSpacing(8)
	input := gui.NewTextInput()
	input.SetText("Type here — no root Padding")
	content.AddChild(input)
	for i := range 45 {
		content.AddChild(gui.NewLabel(fmt.Sprintf("Row %02d — F8 maximize / F9 normal / F10 fullscreen / F11 theme", i)))
	}
	scroll := gui.NewScrollView()
	scroll.SetChild(content)
	root.AddChild(scroll)
	win.SetWidget(root)
	// Keep both axes overflowing initially. The explicit minimum still describes
	// window size rather than content size.
	win.SetMinSize(geometry.Size{Width: 500, Height: 300})
	bind := func(key gui.Key, fn func()) {
		s := gui.NewShortcut(gui.KeyGesture{Key: key})
		s.ConnectActivate(fn)
		win.Shortcuts().AddShortcut(s)
	}
	bind(gui.KeyF6, func() {
		snapshot := win.Snapshot()
		inset := float32(0)
		if runtime.GOOS == "linux" && info.Enabled && info.Mode == gui.WindowChromeIntegrated && win.Transparent() {
			if win.State() != gui.WindowStateMaximized && win.State() != gui.WindowStateFullscreen {
				inset = float32(math.Ceil(float64(5*probe.scale))) / probe.scale
			}
		} else if runtime.GOOS == "linux" {
			fmt.Println("Linux 自绘 Integrated 不适用：未建立所需窗口样式")
			return
		}
		want := geometry.Rect(inset, inset, snapshot.Bounds.Width-2*inset, snapshot.Bounds.Height-2*inset)
		if root.Rect() != want || snapshot.Widget.Bounds != want {
			panic(fmt.Sprintf("root=%v snapshot=%v want=%v", root.Rect(), snapshot.Widget.Bounds, want))
		}
		for _, child := range scroll.Children() {
			bar, ok := child.(*gui.ScrollBar)
			if !ok || !bar.Visible() {
				continue
			}
			r := bar.Snapshot().Bounds
			if r.Intersect(want) != r || bar.Orientation() == layout.DirectionVertical && r.Width != 8 ||
				bar.Orientation() == layout.DirectionHorizontal && r.Height != 8 {
				panic(fmt.Sprintf("scrollbar outside layout: %v", r))
			}
		}
		if info.ControlsBounds.Width > 0 && info.ControlsBounds.Height > 0 && win.State() != gui.WindowStateFullscreen &&
			info.ControlsBounds.Center().Y != header.Snapshot().Bounds.Center().Y {
			panic(fmt.Sprintf("controls=%v header=%v", info.ControlsBounds, header.Snapshot().Bounds))
		}
		fmt.Printf("LAYOUT PASS state=%v root=%v controls=%v\n", win.State(), root.Rect(), info.ControlsBounds)
	})
	bind(gui.KeyF7, toggleHeight)
	bind(gui.KeyF8, func() { win.RequestState(gui.WindowStateMaximized) })
	bind(gui.KeyF9, func() { win.RequestState(gui.WindowStateNormal) })
	bind(gui.KeyF10, func() { win.RequestState(gui.WindowStateFullscreen) })
	bind(gui.KeyF11, func() {
		dark = !dark
		app.SetStyleSheet(modern.Sheet(modern.Options{Dark: dark}))
	})
	win.ConnectDestroy(app.Quit)
	if err := win.Show(); err != nil {
		log.Fatal(err)
	}
	app.Run()
}
