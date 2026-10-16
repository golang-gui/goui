// Menu pointer/keyboard highlight regression. Needs a desktop/window manager,
// native fonts and a painter. Run: go run ./tests/window/gui/menu_pointer [-auto].
// Linux without an IME service: XMODIFIERS=@im=none. Initial: a popup menu with
// First/Second, a disabled item, separator and Last. Move quickly between text
// and row padding; use Home/End/Up/Down then move within the same text region.
// Expect exactly one highlighted item, matching Selected; disabled/separator
// regions clear selection. Keyboard activation must use the current item.
// -auto dispatches these inputs through Window/Popover.DispatchEvent and checks
// every step's snapshot plus rendered row-background pixels. READY leaves Second
// selected for external desktop capture; PASS also checks activation and reopen.
// Pixel samples are 2 DIP inside each row at mid-height, outside text and rounded
// corners. RenderWidget(1) uses DIP-sized pixels: compare converted straight
// NRGBA to the test's opaque sRGB background, tolerance 0. No font golden images.
// Record OS, actual painter, scale and WM. Physical input/IME remain separate.
// No clipboard/files/settings changed. Restart to reset, close to exit; closing
// before PASS with -auto is failure. Only this program's windows are operated.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"runtime"
	"sort"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/style"
)

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
func check(ok bool, message string) {
	if !ok {
		log.Fatal(message)
	}
}
func main() {
	runtime.LockOSThread()
	auto := flag.Bool("auto", false, "verify inputs and exit")
	flag.Parse()
	app, err := gui.NewApplication("org.golang-gui.MenuPointerRegression")
	must(err)
	normal, highlight := color.NRGBA{R: 240, G: 240, B: 240, A: 255}, color.NRGBA{R: 36, G: 104, B: 172, A: 255}
	rules := gui.DefaultStyleRules()
	rules = append(rules, style.Name("menu-item").BackgroundColor(normal), style.Name("menu-item").State(style.Hovered).BackgroundColor(highlight), style.Name("menu-item").State(style.Pressed).BackgroundColor(highlight), style.Name("menu-item").State(style.Disabled).BackgroundColor(normal))
	app.SetStyleSheet(style.Sheet(rules...))
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 640, Height: 360}})
	must(err)
	defer win.Destroy()
	must(win.SetTitle("GOUI menu pointer regression"))
	anchor := gui.NewButton()
	anchor.SetChild(gui.NewLabel("Menu highlight regression"))
	box := gui.NewLinearBox(layout.DirectionVertical)
	box.SetCrossAlign(layout.CrossStretch)
	box.AddChild(anchor)
	box.AddChild(gui.NewLabel("Move across text/padding; alternate pointer and keyboard navigation."))
	probe := &scaleProbe{}
	box.AddChild(probe)
	win.SetWidget(box)
	model := gui.NewMenu()
	activated := ""
	for _, name := range []string{"First", "Second", "Disabled", "Last"} {
		name := name
		if name == "Last" {
			model.AppendSeparator()
		}
		item := model.Append(name, func() { activated = name })
		if name == "Disabled" {
			item.SetEnabled(false)
		}
	}
	menu := gui.NewPopoverMenu(anchor)
	menu.SetMenu(model)
	must(win.Show())
	win.SetFocusedWidget(anchor)
	var popup gui.Popover
	key := func(k events.Key) {
		for _, kind := range []events.EventType{events.KeyDown, events.KeyUp} {
			must(win.DispatchEvent(events.KeyEvent{EventType: kind, Key: k}))
		}
	}
	rows := func() []gui.WidgetInfo {
		var found []gui.WidgetInfo
		var walk func(gui.WidgetInfo)
		walk = func(i gui.WidgetInfo) {
			if i.Role == gui.RoleMenuItem || i.Role == gui.RoleMenuSeparator {
				found = append(found, i)
			}
			for _, child := range i.Children {
				walk(child)
			}
		}
		walk(popup.Widget().Snapshot())
		sort.Slice(found, func(i, j int) bool { return found[i].Bounds.Y < found[j].Bounds.Y })
		return found
	}
	verify := func(want string) {
		rendered, err := gui.RenderWidget(popup.Widget(), 1)
		must(err)
		origin := popup.Widget().Snapshot().Bounds.Pos
		painted, selected := 0, 0
		for _, row := range rows() {
			if row.Role == gui.RoleMenuSeparator {
				check(!row.Selected, "separator selected")
				continue
			}
			r := row.Bounds
			p := geometry.Point{X: r.X + 2, Y: r.Y + r.Height/2}
			x, y := int(p.X-origin.X), int(p.Y-origin.Y)
			check(image.Pt(x, y).In(rendered.Bounds()), "pixel sample outside rendered panel")
			got := color.NRGBAModel.Convert(rendered.At(x, y)).(color.NRGBA)
			lit := got == highlight
			if lit {
				painted++
			}
			if row.Selected {
				selected++
			}
			expected := row.Text == want && want != ""
			if lit != expected || row.Selected != expected {
				log.Fatalf("row %q: pixel=%v selected=%t; want active %q", row.Text, got, row.Selected, want)
			}
			if !lit && got != normal {
				log.Fatalf("unexpected row background %q: %v", row.Text, got)
			}
		}
		count := 0
		if want != "" {
			count = 1
		}
		check(painted == count && selected == count, "painted and semantic selection count differ")
	}
	complete := false
	open := app.NewTimer()
	open.ConnectTimeout(func() {
		must(menu.ShowAt(geometry.Point{X: 12, Y: 36}))
		check(len(win.Popovers()) == 1, "menu not open")
		popup = win.Popovers()[0]
	})
	must(open.StartOnce(500 * time.Millisecond))
	ready := app.NewTimer()
	ready.ConnectTimeout(func() {
		check(popup != nil && len(rows()) == 5, "menu rows not realized")
		if *auto {
			move := func(index int, text bool) {
				r := rows()[index].Bounds
				p := geometry.Point{X: r.X + 2, Y: r.Y + r.Height/2}
				if text {
					r = rows()[index].Children[0].Bounds
					p = geometry.Point{X: r.X + r.Width/2, Y: r.Y + r.Height/2}
					check(gui.Pick(popup.Widget(), geometry.Point{X: p.X - popup.Widget().Snapshot().Bounds.X, Y: p.Y - popup.Widget().Snapshot().Bounds.Y}).Snapshot().Role == gui.RoleLabel, "text move did not hit Label")
				}
				must(popup.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: p}))
			}
			move(0, false)
			verify("First")
			move(1, true)
			verify("Second")
			for i := 0; i < 20; i++ {
				move(0, true)
				verify("First")
				move(1, true)
				verify("Second")
			}
			key(events.KeyHome)
			verify("First")
			move(1, true)
			verify("Second")
			move(1, false)
			verify("Second")
			move(1, true)
			verify("Second")
			move(2, true)
			verify("")
			move(3, false)
			verify("")
			move(0, true)
			verify("First")
			must(popup.DispatchEvent(events.PointerEvent{EventType: events.PointerLeave}))
			r := rows()[1].Children[0].Bounds
			must(popup.DispatchEvent(events.PointerEvent{EventType: events.PointerEnter, Position: geometry.Point{X: r.X + r.Width/2, Y: r.Y + r.Height/2}}))
			verify("Second")
		}
		log.Printf("READY painter=%s scale=%g typography=%s", probe.painter, probe.scale, app.Typography().Name())
	})
	must(ready.StartOnce(1000 * time.Millisecond))
	if *auto {
		finish := app.NewTimer()
		finish.ConnectTimeout(func() {
			key(events.KeyEnter)
			check(activated == "Second" && !menu.Visible(), "Enter activated stale item")
			must(menu.ShowAt(geometry.Point{X: 12, Y: 36}))
			check(len(win.Popovers()) == 1, "reopen failed")
			key(events.KeyEscape)
			check(!menu.Visible(), "Escape failed")
			log.Print("PASS rapid label/padding hover, single painted/semantic selection, keyboard takeover, disabled/separator, enter, activation and reopen")
			complete = true
			win.Destroy()
		})
		must(finish.StartOnce(2400 * time.Millisecond))
	}
	app.Run()
	if *auto && !complete {
		log.Fatal("verification aborted")
	}
}

type scaleProbe struct {
	gui.WidgetBase
	scale   float32
	painter string
}

func (p *scaleProbe) Measure(c layout.Constraint) layout.Measurement {
	return layout.Measured(c.Clamp(geometry.Size{Width: 1, Height: 1}))
}
func (p *scaleProbe) Paint(draw gui.Painter) {
	if p.scale != 0 {
		return
	}
	p.scale = draw.PixelScale()
	img, err := draw.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
	must(err)
	defer img.Destroy()
	p.painter = fmt.Sprintf("%T", img)
}
