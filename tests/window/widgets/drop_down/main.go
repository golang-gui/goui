// DropDown popup, content, pointer/keyboard selection and reconciliation window
// checks, without DevServer.
// Environment: interactive Linux X11, Windows or macOS. Record OS/WM, backend
// and scale. Run: go run ./tests/window/widgets/drop_down
// -gui selects imperative controls; -fallback-style uses explicit widget defaults;
// -destroy-on-selected checks destruction in the signal. Use
// GOUI_PLAT_PAINTER=software/opengl (also direct2d on Windows), GOUI_PLAT_SCALE=2.
// Initially Integrated 850x640 DIP, light/blue: Basic unselected, Custom=0,
// Long=9998, Uncontrolled unselected (0 with -gui), Disabled disabled at 0,
// Empty has no items, and item 1 is disabled.
// Steps and expected results:
//  1. Click Basic: a separate rounded/shadowed list matches its width, with
//     content inside corners. Press then release outside does not open it;
//     Disabled/Empty cannot open. Item 1 cannot confirm. Choosing C increments
//     Selected once and collapses to C; confirming C again adds no signal.
//  2. Tab/Shift+Tab navigate; Enter/Space/Alt+Down open. Down skips disabled item 1;
//     browsing leaves the collapsed value unchanged. Enter confirms, Esc/outside
//     click cancels. Tab closes and moves to the next available control; Shift+Tab
//     goes backward. Closed-state wheel leaves selection unchanged.
//  3. Custom rows show a bullet, name and description; UI collapsed display shows
//     only the name (-gui retains row display). Real widget instances are not
//     moved between locations; package tests check isolation. At 1x/2x all four
//     rows fit without a scrollbar. Close, Shrink, reopen: one row. Close, Reset,
//     reopen: four rows with no scrollbar, including native popup resizing.
//  4. Long shows bounded rows and a draggable bar, initially revealing selected
//  9998. Item margins match (default 6 DIP); highlights/checks stay out of them.
//     Clicking between items and scrollbar neither confirms nor closes. Home/End
//     and arrows browse without constructing 10,000 rows or freezing. Height is
//     at most 320 DIP, with stable width during scrolling. Reopening follows the
//     moved owner's current position; near the screen bottom, placement moves
//     above/inside the work area at both scales.
//  5. Set silently selects Basic. Rebuild retains widget/model identity, selection
//     and one signal connection. Uncontrolled omits Selected and keeps user edits.
//     Shrink reduces Basic to one item and silently clears out-of-range selection;
//     Reset restores data/counts.
//  6. Dark/Accent switches mode and blue/red. Item fills stay neutral, with
//     readable text/arrows/checks/shadows. Only keyboard focus adds a softened
//     2 DIP border. Check/F6 prints ASSERT PASS and window-local DIP BOUNDS;
//     the popup has its own Root.
//  7. -destroy-on-selected closes on first confirmation; later Selected callbacks
//     do not run. DESTROY PASS, no leftover popup or panic. Normal close also
//     prints DESTROY PASS.
//
// Platform differences: fonts, antialiasing and decorations vary; Integrated may
// fall back to Native. Side effects: windows/logs only; no clipboard/files/settings.
// Reset: Reset or restart. Exit: close the main window. State/identity/snapshot
// assertion failures panic; inspect appearance separately.
package main

import (
	"flag"
	"fmt"
	"image/color"
	"runtime"

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

var options = []widgets.DropDownItem{{Text: "A · Follow system"}, {Text: "B · Disabled", Disabled: true}, {Text: "C · Light"}, {Text: "D · Dark"}}

func main() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	imperative := flag.Bool("gui", false, "命令式分支")
	fallback := flag.Bool("fallback-style", false, "显式兜底")
	destroy := flag.Bool("destroy-on-selected", false, "首次用户选择后销毁")
	flag.Parse()
	if *imperative {
		runGUI(*fallback, *destroy)
	} else {
		runUI(*fallback, *destroy)
	}
}

func sheet(dark, red, fallback bool, family string, size float32) style.StyleSheet {
	if fallback {
		return style.Sheet(append(gui.DefaultStyleRules(), widgetstyle.Rules()...)...)
	}
	accent := color.RGBA{R: 70, G: 130, B: 220, A: 255}
	if red {
		accent = color.RGBA{R: 210, G: 48, B: 48, A: 255}
	}
	return modern.Sheet(modern.Options{Dark: dark, AccentColor: accent, FontFamily: family, FontSize: size})
}

func longModel() *gui.SliceListModel[widgets.DropDownItem] {
	items := make([]widgets.DropDownItem, 10000)
	for i := range items {
		items[i].Text = fmt.Sprintf("Option %05d", i)
	}
	return gui.NewSliceListModel(items)
}

func check(win gui.Window, previous map[string]*widgets.DropDown, want map[string]int, changes, late int) {
	for _, id := range []string{"basic", "custom", "long", "uncontrolled", "disabled", "empty"} {
		d, ok := gui.FindWidget(win, id).(*widgets.DropDown)
		if !ok || previous[id] != nil && previous[id] != d {
			panic("widget identity: " + id)
		}
		previous[id] = d
		if value, controlled := want[id]; controlled && d.Selected() != value {
			panic(fmt.Sprintf("%s selected %d, want %d", id, d.Selected(), value))
		}
		info := d.Snapshot()
		attr := info.Attributes[widgets.DropDownInfoKey].(widgets.DropDownInfo)
		if info.Role != widgets.RoleComboBox || attr.Index != d.Selected() || attr.Expanded != d.Opened() || info.Enabled != (id != "disabled") {
			panic("snapshot: " + id)
		}
		fmt.Printf("BOUNDS %s %.2f %.2f %.2f %.2f SELECTED %d OPEN %v\n", id, info.Bounds.X, info.Bounds.Y, info.Bounds.Width, info.Bounds.Height, d.Selected(), d.Opened())
		if info.Focused {
			fmt.Printf("FOCUS %s visible=%v\n", id, info.FocusVisible)
		}
	}
	fmt.Printf("ASSERT PASS changes=%d late=%d retained=true\n", changes, late)
}

func runUI(fallback, destroy bool) {
	model := gui.NewSliceListModel(options)
	long := longModel()
	empty := gui.NewSliceListModel[widgets.DropDownItem](nil)
	basic, custom, longSelection := -1, 0, 9998
	changes, late := 0, 0
	dark, red := false, false
	previous := make(map[string]*widgets.DropDown)
	connected := false
	err := ui.Run("org.golang-gui.DropDownTest", func(app ui.App) ui.RootView {
		gui.App.SetQuitOnLastWindowClosed(false)
		update := func() { app.RequestUpdate() }
		change := func(id string, value *int) func(int) {
			return func(index int) {
				*value = index
				changes++
				fmt.Printf("SELECT %s %d changes=%d\n", id, index, changes)
				if destroy {
					app.FindWindow("choices").Destroy()
				} else {
					update()
				}
			}
		}
		verify := func() {
			win := app.FindWindow("choices")
			check(win, previous, map[string]int{"basic": basic, "custom": custom, "long": longSelection, "disabled": 0, "empty": -1}, changes, late)
			for _, id := range []string{"set", "rebuild", "shrink", "dark", "accent", "reset", "check"} {
				r := gui.FindWidget(win, id).Snapshot().Bounds
				fmt.Printf("BOUNDS %s %.2f %.2f %.2f %.2f\n", id, r.X, r.Y, r.Width, r.Height)
			}
			if !connected {
				previous["basic"].ConnectSelected(func(int) { late++ })
				connected = true
			}
		}
		settings := app.Settings()
		makeChoice := func(id string, m gui.ListData[widgets.DropDownItem]) *wui.DropDownView {
			return wui.DropDown(m).ID(id).MinWidth(240).Placeholder("Choose an option").OnOpenError(func(err error) { panic(err) })
		}
		return ui.Root().StyleSheet(sheet(dark, red, fallback, settings.FontFamily(), settings.FontSize())).Windows(
			ui.Window("choices").Title("GOUI DropDown").Size(850, 640).Chrome(ui.WindowChromeIntegrated).
				OnDestroy(func() {
					app.Post(func() {
						for _, d := range previous {
							if !d.Destroyed() || d.Opened() {
								panic("destroy retained live widget/popup")
							}
						}
						if destroy && (changes != 1 || late != 0) {
							panic("destroyed signal continued")
						}
						fmt.Println("DESTROY PASS")
						app.Quit()
					})
				}).Content(ui.VBox(
				ui.HeaderBar(ui.Label("DropDown · Selected / OnSelected")),
				ui.VBox(
					ui.Label(fmt.Sprintf("Selected events %d | Basic %d | Custom %d | Long %d", changes, basic, custom, longSelection)),
					ui.HBox(
						ui.Button("Set").ID("set").OnClick(func() { basic = 0; update() }),
						ui.Button("Rebuild").ID("rebuild").OnClick(update),
						ui.Button("Shrink").ID("shrink").OnClick(func() {
							model.SetItems(options[:1])
							if basic >= 1 {
								basic = -1
							}
							if custom >= 1 {
								custom = -1
							}
							update()
						}),
						ui.Button("Dark").ID("dark").OnClick(func() { dark = !dark; update() }),
						ui.Button("Accent").ID("accent").OnClick(func() { red = !red; update() }),
						ui.Button("Reset").ID("reset").OnClick(func() { model.SetItems(options); basic, custom, longSelection = -1, 0, 9998; changes = 0; update() }),
						ui.Button("Check").ID("check").OnClick(verify),
					).Spacing(8),
					ui.HBox(ui.Label("Basic"), makeChoice("basic", model).Selected(basic).OnSelected(change("basic", &basic))).Spacing(16),
					ui.HBox(ui.Label("Custom"), makeChoice("custom", model).Selected(custom).OnSelected(change("custom", &custom)).
						Item(func(_ int, item wui.DropDownItem) ui.View {
							name := "drop-down-item-text"
							if item.Disabled {
								name += "-disabled"
							}
							return ui.VBox(ui.Label("● "+item.Text).Style(name), ui.Label("A custom Widget subtree").Style(modern.MutedText)).Spacing(2)
						}).
						SelectedItem(func(_ int, item wui.DropDownItem) ui.View { return ui.Label(item.Text).Style("drop-down-text") })).Spacing(16),
					ui.HBox(ui.Label("Long"), makeChoice("long", long).Selected(longSelection).OnSelected(change("long", &longSelection))).Spacing(16),
					ui.HBox(ui.Label("Uncontrolled"), makeChoice("uncontrolled", model).OnSelected(func(index int) {
						changes++
						fmt.Printf("SELECT uncontrolled %d changes=%d\n", index, changes)
						update()
					})).Spacing(16),
					ui.HBox(ui.Label("Disabled"), makeChoice("disabled", model).Selected(0).Enabled(false)).Spacing(16),
					ui.HBox(ui.Label("Empty"), makeChoice("empty", empty)).Spacing(16),
					ui.Label("Tab / Shift+Tab, Enter / Space, Up / Down, Home / End, Esc. F6 checks."),
				).Padding(20).Spacing(20).CrossAlign(layout.CrossStart),
			).CrossAlign(layout.CrossStretch).Shortcuts(ui.Shortcut(ui.KeyF6).OnActivate(verify))),
		)
	})
	if err != nil {
		panic(err)
	}
}

type customDelegate struct {
	model gui.ListData[widgets.DropDownItem]
}

func (d customDelegate) Setup() gui.Widget {
	box := gui.NewLinearBox(layout.DirectionVertical)
	box.SetSpacing(2)
	title, description := gui.NewLabel(""), gui.NewLabel("A custom Widget subtree")
	title.SetStyleName("drop-down-item-text")
	description.SetStyleName(modern.MutedText)
	box.AddChild(title)
	box.AddChild(description)
	return box
}
func (d customDelegate) Bind(index int, w gui.Widget) {
	item := d.model.ItemAt(index)
	label := w.Children()[0].(*gui.Label)
	name := "drop-down-item-text"
	if item.Disabled {
		name += "-disabled"
	}
	label.SetStyleName(name)
	label.SetText("● " + item.Text)
}
func (customDelegate) Unbind(int, gui.Widget) {}

func runGUI(fallback, destroy bool) {
	app, err := gui.NewApplication("org.golang-gui.DropDownTest")
	if err != nil {
		panic(err)
	}
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 850, Height: 640}, Chrome: gui.WindowChromeIntegrated})
	if err != nil {
		panic(err)
	}
	if err = win.SetTitle("GOUI DropDown · GUI"); err != nil {
		panic(err)
	}
	model, long := gui.NewSliceListModel(options), longModel()
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetSpacing(20)
	root.SetPadding(20)
	root.SetCrossAlign(layout.CrossStart)
	status := gui.NewLabel("Selected events 0")
	root.AddChild(status)
	changes, late := 0, 0
	dark, red := false, false
	previous := make(map[string]*widgets.DropDown)
	choices := make(map[string]*widgets.DropDown)
	for _, id := range []string{"basic", "custom", "long", "uncontrolled", "disabled", "empty"} {
		d := widgets.NewDropDown()
		d.SetID(id)
		d.SetMinSize(geometry.Size{Width: 240})
		d.SetPlaceholder("Choose an option")
		m := gui.ListData[widgets.DropDownItem](model)
		if id == "long" {
			m = long
		} else if id == "empty" {
			m = nil
		}
		d.SetModel(m)
		if id == "custom" {
			d.SetDelegate(customDelegate{model})
			d.SetSelected(0)
		}
		if id == "long" {
			d.SetSelected(9998)
		}
		if id == "disabled" {
			d.SetSelected(0)
			d.SetEnabled(false)
		}
		if id == "uncontrolled" {
			d.SetSelected(0)
		}
		d.ConnectOpenError(func(err error) { panic(err) })
		d.ConnectSelected(func(index int) {
			changes++
			fmt.Printf("SELECT %s %d changes=%d\n", id, index, changes)
			status.SetText(fmt.Sprintf("Selected events %d", changes))
			if destroy {
				win.Destroy()
			}
		})
		row := gui.NewLinearBox(layout.DirectionHorizontal)
		row.SetSpacing(16)
		row.AddChild(gui.NewLabel(id))
		row.AddChild(d)
		root.AddChild(row)
		choices[id] = d
	}
	choices["basic"].ConnectSelected(func(int) { late++ })
	buttons := gui.NewLinearBox(layout.DirectionHorizontal)
	buttons.SetSpacing(8)
	for _, entry := range []struct {
		text string
		fn   func()
	}{
		{"Set", func() { choices["basic"].SetSelected(0) }},
		{"Shrink", func() { model.SetItems(options[:1]) }},
		{"Reset", func() {
			model.SetItems(options)
			choices["basic"].SetSelected(-1)
			choices["custom"].SetSelected(0)
			choices["long"].SetSelected(9998)
			changes = 0
			status.SetText("Selected events 0")
		}},
		{"Dark", func() {
			dark = !dark
			settings := app.Settings()
			app.SetStyleSheet(sheet(dark, red, fallback, settings.FontFamily(), settings.FontSize()))
		}},
		{"Accent", func() {
			red = !red
			settings := app.Settings()
			app.SetStyleSheet(sheet(dark, red, fallback, settings.FontFamily(), settings.FontSize()))
		}},
		{"Check", func() { check(win, previous, nil, changes, late) }},
	} {
		b := gui.NewButton()
		b.SetChild(gui.NewLabel(entry.text))
		b.ConnectClicked(entry.fn)
		buttons.AddChild(b)
	}
	root.AddChild(buttons)
	win.SetWidget(root)
	verify := gui.NewShortcut(gui.KeyGesture{Key: gui.KeyF6})
	verify.ConnectActivate(func() { check(win, previous, nil, changes, late) })
	win.Shortcuts().AddShortcut(verify)
	win.ConnectDestroy(func() {
		if destroy && (changes != 1 || late != 0) {
			panic("destroyed signal continued")
		}
		fmt.Println("DESTROY PASS")
	})
	settings := app.Settings()
	app.SetStyleSheet(sheet(false, false, fallback, settings.FontFamily(), settings.FontSize()))
	if err = win.Show(); err != nil {
		panic(err)
	}
	app.Run()
}
