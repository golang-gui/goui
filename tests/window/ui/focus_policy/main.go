// 焦点准入、指针聚焦与声明式局部更新验收，不使用DevServer。
//
// 环境：Linux/X11、Windows或macOS桌面；记录实际后端、倍率与Linux WM。
// 启动：go run ./tests/window/ui/focus_policy [-auto]。
// 可用GOUI_PLAT_PAINTER=software/opengl，Windows另可direct2d；GOUI_PLAT_SCALE=2请求2x。
// 初始：Modern亮色Native窗口，查找框已有文本并聚焦；Aa、Whole、Previous、Next
// 允许Tab但关闭指针聚焦。DropDown/Menu也关闭指针聚焦，其弹层使用各自Root。
//
// 操作与预期：
//  1. 在输入框中选中文字，逐个点击四个查找控件。计数/勾选变化，输入框焦点和选区
//     保留，无失焦日志；可在中文IME预编辑期间重试，预编辑不得因按钮默认聚焦重置。
//  2. Tab依次到Aa、Whole、Previous、Next、DropDown、Menu，再回输入框；Shift+Tab逆序。
//     Space/Enter激活已聚焦的查找按钮/CheckButton；键盘提示按现有FocusVisible呈现。
//  3. F2关闭/恢复查找按钮组CanContainFocus。关闭时Tab跳过整组，鼠标仍能操作；
//     若焦点在组内，关闭立即失焦；恢复不自动聚焦。后代自身设置保持true。
//  4. F3切换Next的FocusOnPointer；开启时点击取得焦点，关闭后不清除已有焦点，
//     后续点击保留当前焦点。F4全局重建，文本、选区、Widget身份和配置保持。
//  5. 输入框聚焦时鼠标打开DropDown/Menu，owner焦点不被锚点强制夺取。DropDown
//     的模态弹层可键盘操作；Menu为modeless，锚点未聚焦时不重定向输入框按键。
//     关闭弹层后可继续输入；显式Open仍可按原行为请求锚点焦点。
//  6. F6检查快照/配置/单焦点不变量，并输出焦点、失焦次数、操作计数和构建次数。
//
// -auto等待真实布局后，经Window.DispatchEvent逐项验证指针、Tab、Space/Enter、
// 准入变化、主动聚焦、弹层和局部/全局协调，PASS后退出；超时或断言失败返回非零。
// 普通启动只设置初始焦点，后续完全由人工操作，不持续注入事件或改写选区。
// 自动输入不能替代真实系统IME、触摸/笔或视觉验收；字体/装饰受平台影响。
// 副作用：仅本程序窗口、内存文本与终端日志，无文件/剪贴板/系统设置修改。
// 复位：重新启动。退出：关闭窗口；-auto提前关闭视为失败。
package main

import (
	"flag"
	"fmt"
	"image"
	"log"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	wui "github.com/golang-gui/goui/widgets/ui"
)

type scenario struct {
	app                                      ui.App
	allow, pointer, matchCase, whole         ui.State[bool]
	text                                     ui.State[string]
	status                                   ui.State[string]
	options                                  *gui.SliceListModel[widgets.DropDownItem]
	auto, started, done, reported            bool
	builds, localBuilds, initialBuilds, step int
	previous, next, changes, lost, popupLost int
	query                                    *gui.TextInput
	selection                                gui.TextSelection
	timer                                    *ui.Timer
	deadline                                 time.Time
}

func require(ok bool, message string) {
	if !ok {
		panic(message)
	}
}
func (s *scenario) widget(id string) gui.Widget { return s.app.FindWidget("main", id) }
func (s *scenario) win() gui.Window             { return s.app.FindWindow("main") }
func (s *scenario) refresh() {
	focus := "none"
	if w := s.win().FocusedWidget(); w != nil {
		focus = w.ID()
	}
	s.status.Set(fmt.Sprintf("focus=%s | lost=%d | prev=%d next=%d changes=%d | allow=%t pointer(next)=%t", focus, s.lost, s.previous, s.next, s.changes, s.allow.Get(), s.pointer.Get()))
}
func (s *scenario) check() {
	focused := s.win().FocusedWidget()
	for _, id := range []string{"query", "case", "whole", "prev", "next", "drop", "menu"} {
		w := s.widget(id)
		info := w.Snapshot()
		require(info.Focused == w.Focused() && info.CanContainFocus == w.CanContainFocus() && info.FocusOnPointer == w.FocusOnPointer(), "snapshot policy/state mismatch: "+id)
		require(w.Focused() == (w == focused), "host and widget focus disagree: "+id)
	}
	log.Printf("CHECK allow=%t nextPointer=%t focus=%v lost=%d prev=%d next=%d changes=%d builds=%d local=%d selection=%+v", s.allow.Get(), s.pointer.Get(), focusID(focused), s.lost, s.previous, s.next, s.changes, s.builds, s.localBuilds, s.query.Selection())
	s.refresh()
}
func focusID(w gui.Widget) string {
	if w == nil {
		return "none"
	}
	return w.ID()
}
func (s *scenario) click(id string) {
	point := s.widget(id).Snapshot().Bounds.Center()
	for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
		if err := s.win().DispatchEvent(events.PointerEvent{EventType: kind, Position: point, Button: events.PointerButtonLeft}); err != nil {
			panic(err)
		}
	}
}
func (s *scenario) key(key events.Key, mods events.Modifiers) {
	if err := s.win().DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: key, Modifiers: mods}); err != nil {
		panic(err)
	}
}
func (s *scenario) tick() {
	require(time.Now().Before(s.deadline), "focus policy verification timed out")
	if s.widget("query") == nil || s.widget("query").Rect().Width <= 0 {
		return
	}
	if s.step == 0 {
		s.query = s.widget("query").(*gui.TextInput)
		s.initialBuilds = s.builds
		require(s.win().SetFocusedWidget(s.query), "initial query focus")
		for _, id := range []string{"query", "case", "whole", "prev", "next", "drop", "menu"} {
			log.Printf("BOUNDS %s %+v", id, s.widget(id).Snapshot().Bounds)
		}
		if !s.auto {
			s.timer.Stop()
			s.refresh()
			log.Println("READY focus policy manual verification")
			return
		}
		s.query.SetSelection(gui.TextSelection{Anchor: 1, Caret: 4})
		s.selection = s.query.Selection()
	}
	switch s.step {
	case 1:
		for _, id := range []string{"case", "whole", "prev", "next"} {
			s.click(id)
		}
		require(s.win().FocusedWidget() == s.query && s.lost == 0 && s.query.Selection() == s.selection, "pointer controls changed query focus or selection")
		require(s.previous == 1 && s.next == 1 && s.changes == 2, "pointer actions did not run")
	case 2:
		s.key(events.KeyTab, 0)
		require(s.win().FocusedWidget() == s.widget("case"), "Tab to case")
		s.key(events.KeySpace, 0)
		s.key(events.KeyTab, 0)
		require(s.win().FocusedWidget() == s.widget("whole"), "Tab to whole")
		s.key(events.KeyEnter, 0)
		s.key(events.KeyTab, 0)
		s.key(events.KeySpace, 0)
		s.key(events.KeyTab, 0)
		s.key(events.KeyEnter, 0)
		require(s.previous == 2 && s.next == 2 && s.changes == 4, "keyboard activation failed")
		for range 4 {
			s.key(events.KeyTab, events.ModifierShift)
		}
		require(s.win().FocusedWidget() == s.query, "reverse Tab to query")
		s.allow.Set(false)
	case 3:
		require(!s.widget("controls").CanContainFocus() && s.widget("next").CanContainFocus(), "subtree restriction rewrote child settings")
		s.key(events.KeyTab, 0)
		require(s.win().FocusedWidget() == s.widget("drop"), "Tab did not skip blocked branch")
		s.key(events.KeyTab, events.ModifierShift)
		require(s.win().FocusedWidget() == s.query, "reverse Tab did not skip blocked branch")
		require(!s.win().SetFocusedWidget(s.widget("next")), "explicit focus bypassed ancestor")
		s.click("prev")
		require(s.previous == 3 && s.win().FocusedWidget() == s.query, "blocked branch rejected pointer actions")
		s.allow.Set(true)
	case 4:
		require(s.win().FocusedWidget() == s.query && s.win().SetFocusedWidget(s.widget("next")), "reopening gate or explicit focus failed")
		s.allow.Set(false)
	case 5:
		require(s.win().FocusedWidget() == nil, "closing gate retained focused descendant")
		s.allow.Set(true)
	case 6:
		require(s.win().FocusedWidget() == nil, "reopening gate restored focus")
		s.win().SetFocusedWidget(s.query)
		s.popupLost = s.lost
		s.click("drop")
		require(s.widget("drop").(*widgets.DropDown).Opened() && s.win().FocusedWidget() == s.query && s.lost == s.popupLost, "pointer DropDown forced owner focus")
		p := s.win().Popovers()
		require(len(p) == 1, "DropDown popup missing")
		host := p[0].(gui.EventTarget)
		require(host.FocusedWidget() != nil, "DropDown did not focus its independent root")
		s.widget("drop").(*widgets.DropDown).Close()
	case 7:
		d := s.widget("drop").(*widgets.DropDown)
		require(d.Open() == nil && s.win().FocusedWidget() == d, "explicit DropDown Open failed")
		d.Close()
		s.win().SetFocusedWidget(s.query)
		s.popupLost = s.lost
		s.click("menu")
		m := s.widget("menu").(*gui.MenuButton)
		require(m.Opened() && s.win().FocusedWidget() == s.query && s.lost == s.popupLost, "pointer MenuButton forced owner focus")
		m.Close()
		require(m.Open() == nil && s.win().FocusedWidget() == m, "explicit MenuButton Open failed")
		m.Close()
		s.win().SetFocusedWidget(s.query)
	case 8:
		require(s.builds == s.initialBuilds, "policy State changes rebuilt the whole application")
		s.app.RequestUpdate()
	case 9:
		require(s.query == s.widget("query") && s.win().FocusedWidget() == s.query && s.query.Selection() == s.selection, "global rebuild replaced query or reset focus/selection")
		s.check()
		s.done = true
		s.timer.Stop()
		log.Println("PASS focus policy pointer/Tab/activation/containment/popups/UI coordination")
		s.win().Destroy()
	}
	s.step++
}
func (s *scenario) view() ui.View {
	return ui.VBox(
		ui.Label("Find toolbar — pointer focus / Tab focus"),
		ui.TextInput().ID("query").BindText(&s.text).OnFocus(func(focused bool) {
			if !focused {
				s.lost++
			}
			log.Printf("FOCUS query=%t lost=%d", focused, s.lost)
			if s.query != nil {
				s.refresh()
			}
		}),
		ui.Build(func() ui.View {
			s.localBuilds++
			return ui.HBox(
				wui.CheckButton("Aa").ID("case").Appearance(wui.CheckAppearanceButton).Checked(s.matchCase.Get()).FocusOnPointer(false).OnChange(func(v wui.CheckState) { s.matchCase.Set(v == wui.CheckChecked); s.changes++; s.refresh() }),
				wui.CheckButton("Whole").ID("whole").Appearance(wui.CheckAppearanceButton).Checked(s.whole.Get()).FocusOnPointer(false).OnChange(func(v wui.CheckState) { s.whole.Set(v == wui.CheckChecked); s.changes++; s.refresh() }),
				ui.Button("Previous").ID("prev").FocusOnPointer(false).OnClick(func() { s.previous++; s.refresh() }),
				ui.Button("Next").ID("next").FocusOnPointer(s.pointer.Get()).OnClick(func() { s.next++; s.refresh() }),
			).ID("controls").Spacing(8).CanContainFocus(s.allow.Get())
		}),
		ui.HBox(
			wui.DropDown(s.options).ID("drop").Selected(0).FocusOnPointer(false),
			ui.MenuButton("Menu").ID("menu").FocusOnPointer(false).Menu(ui.MenuItem("Action", func() { s.refresh() })),
		).Spacing(8),
		ui.Label("F2: group focus gate | F3: Next pointer focus | F4: rebuild | F6: check"),
		ui.Label("Click toolbar controls while selecting text or composing with an IME.").WrapMode(ui.WrapWordChar),
		ui.Label().BindText(&s.status).WrapMode(ui.WrapWordChar),
	).Padding(16).Spacing(12).CrossAlign(layout.CrossStretch)
}
func main() {
	auto := flag.Bool("auto", false, "自动输入验收通过后退出")
	flag.Parse()
	s := &scenario{auto: *auto, allow: ui.MakeState(true), pointer: ui.MakeState(false), matchCase: ui.MakeState(false), whole: ui.MakeState(false), text: ui.MakeState("Find sample"), status: ui.MakeState("Starting"), deadline: time.Now().Add(45 * time.Second), options: gui.NewSliceListModel([]widgets.DropDownItem{{Text: "First"}, {Text: "Second"}})}
	sheet := modern.Sheet(modern.Options{})
	err := ui.Run("org.golanggui.FocusPolicy", func(app ui.App) ui.RootView {
		s.app = app
		s.builds++
		if !s.started {
			s.started = true
			s.timer = app.TickFunc(200*time.Millisecond, s.tick)
		}
		return ui.Root().StyleSheet(sheet).Windows(ui.Window("main").Title("GOUI focus policy").Size(720, 340).Center(true).
			Shortcuts(ui.Shortcut(ui.KeyF2).OnActivate(func() { s.allow.Set(!s.allow.Get()); s.refresh() }), ui.Shortcut(ui.KeyF3).OnActivate(func() { s.pointer.Set(!s.pointer.Get()); s.refresh() }), ui.Shortcut(ui.KeyF4).OnActivate(app.RequestUpdate), ui.Shortcut(ui.KeyF6).OnActivate(s.check)).
			OnAfterPaint(func(p gui.Painter, _ geometry.Rectangle) {
				if !s.reported {
					s.reported = true
					img, e := p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
					if e != nil {
						panic(e)
					}
					log.Printf("BACKEND %T scale=%g", img, p.PixelScale())
					img.Destroy()
				}
			}).Content(s.view()))
	})
	if err != nil {
		log.Fatal(err)
	}
	if *auto && !s.done {
		log.Fatal("window closed before focus policy verification completed")
	}
}
