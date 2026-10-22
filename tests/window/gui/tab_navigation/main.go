// Tab focus navigation and FocusVisible window checks, without DevServer.
//
// Environment: interactive Linux/X11, Windows or macOS. Record OS, Linux WM,
// actual backend and scale. Run: go run ./tests/window/gui/tab_navigation
// Use GOUI_PLAT_PAINTER=software/opengl (also direct2d on Windows) and
// GOUI_PLAT_SCALE=2 for 2x. The requested Native size is 680x600 DIP; content
// minimums and desktop limits may change the actual size. Initially Modern light
// has no logical focus. Main content order: first, second, action, editor, far.
// Toolbar buttons accept clicks but are excluded from Tab order. The form is on
// the left; far starts offscreen at the bottom of the independent right ScrollView.
// Steps and expected results:
//  1. Tab through the five targets, revealing far, then wrap to first; Shift+Tab
//     reverses. Focus status/logs agree. action and popup buttons show a 2 DIP
//     rounded, softened-accent focus border without changing fill. Inputs retain
//     their own border. With no initial focus, Shift+Tab reaches far. Held Tab repeats.
//  2. Type ASCII/Chinese into first/second: only the focused field changes; Tab
//     inserts nothing. Space/Enter on action each increment the action count,
//     without key-repeat activation or activation with Ctrl/Alt. editor accepts
//     text. Enable AcceptsTab: Tab inserts a tab and keeps focus, Shift+Tab returns
//     to action. Disable it again: Tab moves from editor to far.
//  3. F2 hides/shows second; F3 toggles its Focusable, not Disabled. Tab skips
//     hidden/nonfocusable fields. F5 clears focus, then Tab starts at first.
//     F4 enables a bubbling Tab shortcut: plain Tab prints consumed without
//     moving focus, while editor AcceptsTab takes precedence and Shift+Tab is
//     unaffected. Ctrl+Tab/Alt+Tab do not invoke framework traversal and may be
//     handled by the desktop.
//  4. F7 opens a modal Popover. Tab/Shift+Tab cycle only popup-first/popup-second;
//     main fields retain focus but receive no text. Esc closes it; F7/Tab starts
//     at popup-first again. Owner input resumes after close; outside clicks also close.
//  5. Tab to action, then hover: the focus hint coexists with Hover. Click action:
//     logical focus stays but the hint clears, and F6 shows owner-hint=false.
//     Shift+Tab then Tab restores it. Pointer focus adds no border. F8 switches
//     light/dark; F9 cycles blue/black/white/red accents with a softened theme
//     border, not an overlaid fill. F10 toggles ordinary/Primary action style:
//     both retain size and a 2 DIP inset border (2/4 physical pixels at 1x/2x),
//     with no outward growth, square corners or clipping.
//  6. F6 reads public Focused, FocusVisible, ContainsFocus and snapshot state,
//     printing ASSERT PASS without changing focus. This does not verify appearance
//     or IME. owner-hint and popup-hint are independent: a popup click leaves the
//     owner hint; closing/reopening starts popup-hint=false. Hint changes emit no
//     duplicate focus logs. Repeat at 1x/2x; revealed targets lie in the viewport.
//
// Platform differences: macOS uses physical Tab/Shift independently of Primary;
// native shortcuts or IME candidates may consume keys first. Popovers receive
// forwarded owner events and have no native keyboard focus.
// Side effects: windows, logs and in-memory text; explicit copy/paste accesses
// the clipboard. Reset: restart. Exit: close the main window; assertion failures
// panic and return nonzero.
package main

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"runtime"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
)

type backendProbe struct {
	gui.WidgetBase
	reported bool
}

func (p *backendProbe) Paint(painter gui.Painter) {
	if p.reported {
		return
	}
	p.reported = true
	resource, err := painter.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
	if err != nil {
		panic(err)
	}
	fmt.Printf("backend=%T pixel-scale=%g\n", resource, painter.PixelScale())
	resource.Destroy()
}

func main() {
	runtime.LockOSThread()
	app, err := gui.NewApplication("org.golang-gui.TabNavigationTest")
	if err != nil {
		log.Fatal(err)
	}
	dark := false
	accents := []color.Color{nil, color.Black, color.White, color.RGBA{R: 255, A: 255}}
	accentIndex := 0
	applyTheme := func() {
		app.SetStyleSheet(modern.Sheet(modern.Options{Dark: dark, AccentColor: accents[accentIndex]}))
	}
	applyTheme()
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 680, Height: 600}, Chrome: gui.WindowChromeNative})
	if err != nil {
		log.Fatal(err)
	}
	defer win.Destroy()
	if err := win.SetTitle("GOUI Tab navigation"); err != nil {
		log.Fatal(err)
	}
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetPadding(12)
	root.SetSpacing(8)
	root.SetCrossAlign(layout.CrossStretch)
	root.AddChild(gui.NewLabel("Tab / Shift+Tab — F6 checks focus state"))
	root.AddChild(gui.NewLabel("F2–F5 | F6 check | F7 popup | F8 dark | F9 color | F10 primary"))
	status := gui.NewLabel("focus: none")
	watch := func(widget gui.Widget, id string) {
		widget.SetID(id)
		widget.ConnectFocused(func(focused bool) {
			fmt.Printf("focus %s=%v hint=%v\n", id, focused, widget.FocusVisible())
			if focused {
				status.SetText("focus: " + id)
			} else if current := win.FocusedWidget(); current != nil {
				status.SetText("focus: " + current.ID())
			} else {
				status.SetText("focus: none")
			}
		})
	}
	first, second := gui.NewTextInput(), gui.NewTextInput()
	first.SetText("first — type here")
	second.SetText("second — optional focus")
	watch(first, "first")
	watch(second, "second")
	action := gui.NewButton()
	actionText := gui.NewLabel("action")
	action.SetChild(actionText)
	watch(action, "action")
	activations := 0
	action.ConnectClicked(func() {
		activations++
		status.SetText(fmt.Sprintf("action activations: %d", activations))
		fmt.Printf("action activations=%d\n", activations)
	})
	editor := gui.NewTextView()
	editor.SetModel(gui.NewTextModel("editor: enable AcceptsTab to insert tabs"))
	watch(editor, "editor")
	editorScroll := gui.NewScrollView()
	editorScroll.SetMinSize(geometry.Size{Height: 90})
	editorScroll.SetMaxSize(geometry.Size{Height: 90})
	editorScroll.SetChild(editor)
	far := gui.NewTextInput()
	far.SetText("far — last field")
	watch(far, "far")
	longContent := gui.NewLinearBox(layout.DirectionVertical)
	longContent.SetCrossAlign(layout.CrossStretch)
	spacer := gui.NewLabel("Far field below")
	spacer.SetMinSize(geometry.Size{Height: 550})
	longContent.AddChild(spacer)
	longContent.AddChild(far)
	longScroll := gui.NewScrollView()
	longScroll.SetMainWeight(1)
	longScroll.SetChild(longContent)
	form := gui.NewLinearBox(layout.DirectionVertical)
	form.SetMainWeight(1)
	form.SetCrossAlign(layout.CrossStretch)
	form.SetSpacing(8)
	for _, widget := range []gui.Widget{first, second, action, editorScroll} {
		form.AddChild(widget)
	}
	content := gui.NewLinearBox(layout.DirectionHorizontal)
	content.SetMainWeight(1)
	content.SetCrossAlign(layout.CrossStretch)
	content.SetSpacing(12)
	content.AddChild(form)
	content.AddChild(longScroll)
	toolbar := gui.NewLinearBox(layout.DirectionHorizontal)
	toolbar.SetSpacing(8)
	tool := func(text string, fn func()) *gui.Button {
		button := gui.NewButton()
		button.SetFocusable(false)
		button.SetChild(gui.NewLabel(text))
		if fn != nil {
			button.ConnectClicked(fn)
		}
		toolbar.AddChild(button)
		return button
	}
	tool("AcceptsTab", func() {
		editor.SetAcceptsTab(!editor.AcceptsTab())
		fmt.Printf("accepts-tab=%v\n", editor.AcceptsTab())
	})
	popupAnchor := tool("Popup (F7)", nil)
	popup := gui.NewPopover(popupAnchor, nil)
	popup.SetModal(true)
	defer popup.Destroy()
	popup.SetPosition(geometry.Point{Y: 40})
	popupRoot := gui.NewLinearBox(layout.DirectionVertical)
	popupRoot.SetPadding(12)
	popupRoot.SetSpacing(8)
	var popupButtons []gui.Widget
	for _, id := range []string{"popup-first", "popup-second"} {
		button := gui.NewButton()
		button.SetChild(gui.NewLabel(id))
		watch(button, id)
		popupRoot.AddChild(button)
		popupButtons = append(popupButtons, button)
	}
	popup.SetWidget(popupRoot)
	popup.ConnectDismissRequest(func() { popup.Hide() })
	showPopup := func() {
		if err := popup.Show(); err != nil {
			panic(err)
		}
		fmt.Println("popup shown")
	}
	popupAnchor.ConnectClicked(showPopup)
	consume := gui.NewShortcut(gui.KeyGesture{Key: gui.KeyTab})
	consume.SetEnabled(false)
	consume.ConnectActivate(func() { fmt.Println("Tab consumed by shortcut") })
	win.Shortcuts().AddShortcut(consume)
	bind := func(key gui.Key, fn func()) {
		s := gui.NewShortcut(gui.KeyGesture{Key: key})
		s.ConnectActivate(fn)
		win.Shortcuts().AddShortcut(s)
	}
	bind(gui.KeyF2, func() { second.SetVisible(!second.Visible()); fmt.Printf("second-visible=%v\n", second.Visible()) })
	bind(gui.KeyF3, func() {
		second.SetFocusable(!second.Focusable())
		fmt.Printf("second-focusable=%v\n", second.Focusable())
	})
	bind(gui.KeyF4, func() { consume.SetEnabled(!consume.Enabled()); fmt.Printf("consume-tab=%v\n", consume.Enabled()) })
	bind(gui.KeyF5, func() { win.SetFocusedWidget(nil); status.SetText("focus: none") })
	check := func() {
		if verifyFocus(root) > 1 || verifyFocus(popupRoot) > 1 {
			panic("multiple focused widgets in one host")
		}
		focused := "none"
		ownerHint := false
		if w := win.FocusedWidget(); w != nil {
			focused = w.ID()
			ownerHint = w.FocusVisible()
			if !w.Focused() {
				panic("host focus does not match widget state")
			}
		}
		popupFocus, popupHint := "none", false
		for _, button := range popupButtons {
			if button.Focused() {
				popupFocus, popupHint = button.ID(), button.FocusVisible()
			}
		}
		fmt.Printf("ASSERT PASS owner-focus=%s owner-hint=%v popup-focus=%s popup-hint=%v scroll-y=%g first=%q second=%q editor=%q far=%q action-bounds=%+v\n", focused, ownerHint, popupFocus, popupHint, longScroll.ScrollY(), first.Text(), second.Text(), editor.Model().Text(), far.Text(), action.Snapshot().Bounds)
	}
	bind(gui.KeyF6, check)
	popupShortcuts := gui.NewShortcutController()
	popupCheck := gui.NewShortcut(gui.KeyGesture{Key: gui.KeyF6})
	popupCheck.ConnectActivate(check)
	popupShortcuts.AddShortcut(popupCheck)
	popupRoot.AddEventController(popupShortcuts)
	bind(gui.KeyF7, showPopup)
	bind(gui.KeyF8, func() {
		dark = !dark
		applyTheme()
		fmt.Printf("dark=%v\n", dark)
	})
	bind(gui.KeyF9, func() {
		accentIndex = (accentIndex + 1) % len(accents)
		applyTheme()
		fmt.Printf("accent-index=%d (0=blue, 1=black, 2=white, 3=red)\n", accentIndex)
	})
	bind(gui.KeyF10, func() {
		name, contentName := modern.Primary, modern.PrimaryContent
		if action.StyleName() == name {
			name, contentName = "", ""
		}
		action.SetStyleName(name)
		actionText.SetStyleName(contentName)
		fmt.Printf("action-style=%q\n", name)
	})
	for _, widget := range []gui.Widget{toolbar, content, status, &backendProbe{}} {
		root.AddChild(widget)
	}
	win.SetWidget(root)
	if err := win.Show(); err != nil {
		log.Fatal(err)
	}
	app.Run()
}

func verifyFocus(root gui.Widget) int {
	count := 0
	if root.Focused() {
		count++
	}
	for _, child := range root.Children() {
		count += verifyFocus(child)
	}
	contains := count != 0
	info := root.Snapshot()
	if info.Focused != root.Focused() || info.FocusVisible != root.FocusVisible() || root.FocusVisible() && !root.Focused() || info.ContainsFocus != contains || root.ContainsFocus() != contains {
		panic(fmt.Sprintf("inconsistent focus state: id=%q focused=%v contains=%v snapshot=%+v", root.ID(), root.Focused(), contains, info))
	}
	return count
}
