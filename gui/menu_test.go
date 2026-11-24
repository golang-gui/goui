package gui

import (
	"errors"
	"fmt"
	"image/color"
	"testing"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/style"
)

var layoutLooseFull = layout.Constraint{Min: geometry.Size{}, Max: geometry.Size{Width: layout.Inf, Height: layout.Inf}}

// --- Menu model ---

func TestMenuFormattedLabelSnapshot(t *testing.T) {
	source := `\{format\}复\{bold:制\}`
	m := NewMenu()
	item := m.Append(source, nil)
	row := newMenuItemRow(nil)
	row.bind(item)
	if info := row.Snapshot(); info.Role != RoleMenuItem || info.Text != "复制" || item.Label() != source {
		t.Fatal("菜单语义泄漏格式标记或修改了模型源文案")
	}
}

func TestMenuModelAppendAndQuery(t *testing.T) {
	m := NewMenu()
	copy := m.Append("Copy", func() {})
	m.AppendSeparator()
	m.Append("Delete", func() {})

	if got := m.ItemsCount(); got != 3 {
		t.Fatalf("ItemsCount = %d, want 3", got)
	}
	if copy.Separator() {
		t.Fatal("Copy should not be a separator")
	}
	sep := m.ItemAt(1)
	if !sep.Separator() {
		t.Fatal("item 1 should be a separator")
	}
	if m.ItemAt(0).Label() != "Copy" {
		t.Fatalf("item 0 label = %q, want Copy", m.ItemAt(0).Label())
	}
}

func TestMenuItemChangesNotifyModel(t *testing.T) {
	m := NewMenu()
	item := m.Append("Copy", func() {})

	changes := 0
	h := m.ConnectItems(func() { changes++ })
	defer h.Disconnect()

	item.SetEnabled(false)
	if changes != 1 {
		t.Fatalf("SetEnabled should notify model once, got %d", changes)
	}
	item.SetVisible(false)
	if changes != 2 {
		t.Fatalf("SetVisible should notify model once, got %d", changes)
	}
	// No-op changes do not notify.
	item.SetEnabled(false)
	if changes != 2 {
		t.Fatalf("no-op SetEnabled should not notify, got %d", changes)
	}
	item.SetLabel("复制")
	item.SetLabel("复制")
	if changes != 3 || item.Label() != "复制" || m.ItemAt(0) != item || item.Action() == nil {
		t.Fatal("label change must notify once and preserve item identity/action")
	}
}

// --- menuContent rendering ---

func TestMenuContentNaturalSize(t *testing.T) {
	m := NewMenu()
	for i := 0; i < 3; i++ {
		m.Append("Item", func() {})
	}
	m.AppendSeparator()

	mc := newMenuContent(m, 0, func(*MenuItem) {})
	size := mc.Measure(layoutLooseFull)

	if size.Width != 160 {
		t.Fatalf("menu body width = %v, want default minimum 160", size.Width)
	}
	if want := float32(2*menuItemPadding + 2*menuContentPadding); mc.naturalW != want {
		t.Fatalf("intrinsic width = %v, want padding-only width %v without typography", mc.naturalW, want)
	}
	// Without typography, rows use their minimum height, plus shell padding.
	want := 3*menuItemMinHeight + menuSeparatorHeight + 2*menuContentPadding
	if size.Height != float32(want) {
		t.Fatalf("natural height = %v, want %v", size.Height, want)
	}
}

func TestMenuContentWidthConstraints(t *testing.T) {
	useTestApplication(t, &application{typo: &styleTypography{}, style: textStyleSheet(10, color.Black)})
	for _, tt := range []struct {
		name  string
		label string
		c     layout.Constraint
		want  float32
	}{
		{"short Chinese", "复制", layout.Unbounded(), 160},
		{"short English", "Copy", layout.Unbounded(), 160},
		// The typography fixture measures each byte as 10 DIP; include row and body padding.
		{"long content", "A very long menu item", layout.Unbounded(), 210 + 16 + 12},
		{"narrow work area", "复制", layout.Loose(geometry.Size{Width: 100, Height: 100}), 100},
		{"parent minimum", "复制", layout.Tight(geometry.Size{Width: 220, Height: 40}), 220},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMenu()
			m.Append(tt.label, nil)
			mc := newMenuContent(m, 0, nil)
			size := mc.Measure(tt.c).Size
			if size != (geometry.Size{Width: tt.want, Height: 40}) {
				t.Fatalf("body size = %v, want %vx40", size, tt.want)
			}
		})
	}
}

func TestMenuContentSkipsInvisibleItems(t *testing.T) {
	m := NewMenu()
	m.Append("A", func() {})
	hidden := m.Append("B", func() {})
	hidden.SetVisible(false)
	m.Append("C", func() {})

	mc := newMenuContent(m, 0, func(*MenuItem) {})
	size := mc.Measure(layoutLooseFull)

	want := 2*menuItemMinHeight + 2*menuContentPadding
	if size.Height != float32(want) {
		t.Fatalf("invisible item should not contribute height, got %v want %v", size.Height, want)
	}
}

func TestMenuContentCapsAtMaxHeight(t *testing.T) {
	m := NewMenu()
	for i := 0; i < 100; i++ {
		m.Append("Item", func() {})
	}
	mc := newMenuContent(m, 60, func(*MenuItem) {})
	size := mc.Measure(layoutLooseFull)
	if size.Height != 60 {
		t.Fatalf("height should cap at maxHeight 60, got %v", size.Height)
	}
}

// --- menu row activation ---

func TestMenuItemRowActivates(t *testing.T) {
	mi := NewMenuItem("Copy", nil)

	var activated *MenuItem
	row := newMenuItemRow(func(m *MenuItem) { activated = m })
	row.bind(mi)
	row.Arrange(geometry.Rect(0, 0, 120, 24))

	dispatchClick(row, t)
	if activated != mi {
		t.Fatal("click should activate the bound item")
	}
}

func TestMenuItemRowSeparatorNotInteractive(t *testing.T) {
	sep := &MenuItem{separator: true, visible: true}
	var activated *MenuItem
	row := newMenuItemRow(func(m *MenuItem) { activated = m })
	row.bind(sep)
	row.Arrange(geometry.Rect(0, 0, 120, 9))

	dispatchClick(row, t)
	if activated != nil {
		t.Fatal("separator row should not activate")
	}
}

func dispatchClick(row *menuItemRow, t *testing.T) {
	t.Helper()
	down := &eventContext{
		current: row,
		event: events.PointerEvent{
			EventType: events.PointerDown,
			Button:    events.PointerButtonLeft,
			Position:  geometry.Point{X: 10, Y: 10},
		},
	}
	row.click.HandleEvent(down)
	up := &eventContext{
		current: row,
		event: events.PointerEvent{
			EventType: events.PointerUp,
			Button:    events.PointerButtonLeft,
			Position:  geometry.Point{X: 10, Y: 10},
		},
	}
	row.click.HandleEvent(up)
}

// --- PopoverMenu ---

func TestPopoverMenuActivateRunsAction(t *testing.T) {
	var ran string
	mi := NewMenuItem("Copy", func() { ran = "copy" })
	pm := NewPopoverMenu(nil) // no anchor: Hide is a safe no-op
	pm.activate(mi)
	if ran != "copy" {
		t.Fatalf("activate should run the item action, ran=%q", ran)
	}
}

func TestPopoverMenuClosesBeforeRunningAction(t *testing.T) {
	owner := testWindowWithSurface(&window{})
	p := &popover{owner: owner, visible: true, modal: true}
	owner.SetModalTarget(p)
	pm := &PopoverMenu{popover: p}
	closed := 0
	p.ConnectClosed(func() { closed++ })
	called := false
	pm.activate(NewMenuItem("Open", func() {
		called = true
		if p.Visible() || owner.modalTarget != nil || closed != 1 {
			t.Fatal("menu must hide and release its modal target before the action opens a native dialog")
		}
	}))
	if !called || closed != 1 {
		t.Fatalf("called=%t, closed=%d", called, closed)
	}
}

func TestPopoverMenuActivateIgnoresDisabled(t *testing.T) {
	var ran bool
	mi := NewMenuItem("Copy", func() { ran = true })
	mi.SetEnabled(false)
	pm := NewPopoverMenu(nil)
	pm.activate(mi)
	if ran {
		t.Fatal("disabled item must not run its action")
	}
}

// 正常关闭会清空临时模型和重复 Hide，不能因此丢失已经选中的动作。
func TestPopoverMenuActivationSurvivesCloseCleanup(t *testing.T) {
	for _, nested := range []bool{false, true} {
		for _, command := range []bool{false, true} {
			for _, keyboard := range []bool{false, true} {
				t.Run(fmt.Sprint(nested, command, keyboard), func(t *testing.T) {
					w, _, makePopup := transientFixture(t)
					pm := NewPopoverMenu(makePopup(nil).anchor)
					t.Cleanup(pm.Destroy)
					calls, closed := 0, 0
					run := func() {
						calls++
						if pm.Visible() || len(w.Popovers()) != 0 || w.modalTarget != nil || closed != 1 {
							t.Fatal("动作执行前没有完成整链关闭和模态释放")
						}
					}
					leaf := NewMenuItem("Run", run)
					if command {
						c := NewCommand()
						c.ConnectActivate(run)
						leaf.SetCommand(c)
					}
					// 切片模型不可比较，不能用 interface 相等判断模型是否被替换。
					var model MenuModel = menuSliceModel{leaf}
					if nested {
						model = menuSliceModel{NewMenuItem("Branch", nil)}
						model.ItemAt(0).SetSubmenu(menuSliceModel{leaf})
					}
					pm.SetMenu(model)
					if err := pm.ShowAt(geometry.Point{}); err != nil {
						t.Fatal(err)
					}
					key := func(k events.Key) {
						t.Helper()
						if err := w.DispatchEvent(shortcutPress(k, 0)); err != nil {
							t.Fatal(err)
						}
					}
					key(events.KeyHome)
					target := pm
					if nested {
						key(events.KeyArrowRight)
						target = pm.child
						target.ConnectClosed(func() { target.SetMenu(nil); target.Hide() })
					}
					pm.ConnectClosed(func() {
						closed++
						pm.SetMenu(nil)
						pm.Hide()
					})
					if keyboard {
						key(events.KeyEnter)
					} else {
						point := target.rowFor(leaf).windowRect().Center()
						for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
							if err := target.popover.DispatchEvent(events.PointerEvent{EventType: kind, Position: point, Button: events.PointerButtonLeft}); err != nil {
								t.Fatal(err)
							}
						}
					}
					if calls != 1 || pm.Menu() != nil || pm.Visible() {
						t.Fatal("关闭清理跳过了已选动作或重复执行", calls)
					}
				})
			}
		}
	}
}

func TestPopoverMenuCleanupStillRejectsStaleActivation(t *testing.T) {
	for _, change := range []string{"replace-model", "reopen-hide", "other-popup", "remove", "disable-cycle", "command-cycle", "replace-action", "destroy-owner", "destroy-menu"} {
		t.Run(change, func(t *testing.T) {
			w, _, makePopup := transientFixture(t)
			pm := NewPopoverMenu(makePopup(nil).anchor)
			t.Cleanup(pm.Destroy)
			calls := 0
			model := NewMenu()
			item := model.Append("Run", func() { calls++ })
			var command *Command
			if change == "command-cycle" {
				command = NewCommand()
				command.ConnectActivate(func() { calls++ })
				item.SetCommand(command)
			}
			pm.SetMenu(model)
			if err := pm.ShowAt(geometry.Point{}); err != nil {
				t.Fatal(err)
			}
			done := false
			pm.ConnectClosed(func() {
				if done {
					return
				}
				done = true
				pm.SetMenu(nil)
				switch change {
				case "replace-model":
					pm.SetMenu(menuSliceModel{item})
					pm.SetMenu(nil)
				case "reopen-hide":
					pm.SetMenu(model)
					if err := pm.ShowAt(geometry.Point{}); err != nil {
						t.Fatal(err)
					}
					pm.Hide()
				case "other-popup":
					other := makePopup(nil)
					if err := other.Show(); err != nil {
						t.Fatal(err)
					}
					other.Hide()
				case "remove":
					model.Remove(0)
				case "disable-cycle":
					item.SetEnabled(false)
					item.SetEnabled(true)
				case "command-cycle":
					command.SetEnabled(false)
					command.SetEnabled(true)
				case "replace-action":
					item.SetAction(func() { calls += 10 })
				case "destroy-owner":
					w.Destroy()
				case "destroy-menu":
					pm.Destroy()
				}
			})
			pm.activate(item)
			if calls != 0 {
				t.Fatal("清理后仍执行了失效动作", calls)
			}
		})
	}
}

func TestPopoverMenuVisibleFalseBeforeShow(t *testing.T) {
	m := NewMenu()
	m.Append("Copy", func() {})
	pm := NewPopoverMenu(newTestWidget())
	pm.SetMenu(m)
	if pm.Menu() != m {
		t.Fatal("Menu() should return the set model")
	}
	if pm.Visible() {
		t.Fatal("not shown popover should not be visible")
	}
}

func TestPopoverMenuShowAtUnmountedAnchorErrors(t *testing.T) {
	// The anchor is not mounted in a window, so ShowAt must fail gracefully
	// (anchorWindow fails) rather than panic — exercising the pre-App path.
	m := NewMenu()
	m.Append("Copy", func() {})
	pm := NewPopoverMenu(newTestWidget())
	pm.SetMenu(m)
	err := pm.ShowAt(geometry.Point{X: 10, Y: 10})
	if err == nil {
		t.Fatal("ShowAt with unmounted anchor should return an error")
	}
}

// --- MenuButton ---

func TestMenuButtonSetMenuNoMenuIsNoop(t *testing.T) {
	b := NewMenuButton()
	b.SetChild(NewLabel("File"))
	b.openMenu() // menu nil → should not panic / do nothing
	if b.pm != nil {
		t.Fatal("openMenu with nil menu should not create a popover")
	}
}

func TestMenuButtonCreatesPopoverOnOpen(t *testing.T) {
	m := NewMenu()
	m.Append("Copy", func() {})
	b := NewMenuButton()
	b.SetChild(NewLabel("File"))
	b.SetMenu(m)
	b.openMenu()
	if b.pm == nil {
		t.Fatal("openMenu with a menu should create a PopoverMenu")
	}
	if b.pm.Menu() != m {
		t.Fatal("MenuButton should forward its menu to the PopoverMenu")
	}
}

func TestMenuButtonKeyboardUsesOpenPath(t *testing.T) {
	for _, key := range []events.Key{events.KeySpace, events.KeyEnter} {
		b := NewMenuButton()
		menu := NewMenu()
		menu.Append("Copy", func() {})
		b.SetMenu(menu)
		win := testWindowWithSurface(&window{})
		win.SetWidget(b)
		_ = win.DispatchEvent(shortcutPress(events.KeyTab, 0))
		_ = win.DispatchEvent(shortcutPress(key, 0))
		// No native surface is supplied: showing fails, but the real open path
		// must have created the menu and forwarded its model, not just a counter.
		if b.pm == nil || b.pm.Menu() != menu {
			t.Fatal("keyboard bypassed menu open path")
		}
		win.Destroy()
	}
}

// 复用普通原生面替身及真实 GUI 分发；不创建桌面或注入菜单语义动作。
func menuButtonGroupFixture(t *testing.T) (*window, *testWidget, []*MenuButton, *placementPlatform) {
	t.Helper()
	_, _, platform, app := placementFixture(t, geometry.Rect(0, 0, 1000, 800))
	app.typo = &testTypography{}
	w := app.windows[0]
	w.focused = true
	w.SetWidget(nil)
	root := newTestWidget()
	w.SetWidget(root)
	root.Arrange(geometry.Rect(0, 0, 500, 300))
	group := NewMenuButtonGroup()
	root.AddChild(group)
	group.Arrange(geometry.Rect(0, 0, 500, 300))
	var buttons []*MenuButton
	for i := 0; i < 3; i++ {
		button := NewMenuButton()
		menu := NewMenu()
		menu.Append("Action", nil)
		button.SetMenu(menu)
		button.SetChild(NewLabel("Menu"))
		group.AddChild(button)
		button.Arrange(geometry.Rect(20+float32(i)*100, 20, 80, 40))
		button.Child().Arrange(geometry.Rect(10, 10, 60, 20))
		buttons = append(buttons, button)
	}
	t.Cleanup(w.Destroy)
	return w, root, buttons, platform
}
func menuButtonPointer(t *testing.T, w *window, kind events.EventType, x float32) {
	t.Helper()
	if err := w.DispatchEvent(events.PointerEvent{EventType: kind, Position: geometry.Point{X: x, Y: 40}, Button: events.PointerButtonLeft}); err != nil {
		t.Fatal(err)
	}
}
func menuButtonClick(t *testing.T, w *window, x float32) {
	t.Helper()
	menuButtonPointer(t, w, events.PointerDown, x)
	menuButtonPointer(t, w, events.PointerUp, x)
}
func menuButtonKey(t *testing.T, w *window, key events.Key) {
	t.Helper()
	if err := w.DispatchEvent(shortcutPress(key, 0)); err != nil {
		t.Fatal(err)
	}
}
func TestMenuButtonGroupPointerKeyboardAndMixedButtons(t *testing.T) {
	w, _, b, _ := menuButtonGroupFixture(t)
	menuButtonPointer(t, w, events.PointerMove, 150)
	if len(w.Popovers()) != 0 {
		t.Fatal("空闲悬停打开了菜单")
	}
	menuButtonClick(t, w, 50)
	if !b[0].Opened() || w.modalTarget != nil || w.FocusedWidget() != b[0] || b[0].pm.popover.Modal() || !b[0].pm.popover.AutoDismiss() {
		t.Fatal("菜单没有使用非模态自动关闭及按钮焦点")
	}
	closed := 0
	b[0].ConnectClosed(func() { closed++ })
	menuButtonPointer(t, w, events.PointerMove, 150)
	if b[0].Opened() || !b[1].Opened() || closed != 1 {
		t.Fatal("命中 Label 后没有切换到同组菜单")
	}
	b[2].SetEnabled(false)
	menuButtonPointer(t, w, events.PointerMove, 250)
	if !b[1].Opened() || b[2].Opened() {
		t.Fatal("禁用菜单参与了悬停切换")
	}
	ordinary := NewButton()
	b[0].Group().AddChild(ordinary)
	ordinary.Arrange(geometry.Rect(350, 20, 100, 40))
	clicks := 0
	ordinary.ConnectClicked(func() { clicks++ })
	menuButtonClick(t, w, 400)
	if b[1].Opened() || clicks != 1 {
		t.Fatal("外部点击没有关闭并执行一次混排按钮", clicks)
	}
	if err := b[0].Open(); err != nil {
		t.Fatal(err)
	}
	menuButtonKey(t, w, events.KeyArrowRight)
	if !b[1].Opened() || b[0].Opened() {
		t.Fatal("右键没有切换菜单")
	}
	menuButtonKey(t, w, events.KeyArrowRight)
	if !b[0].Opened() {
		t.Fatal("没有跳过禁用成员并循环")
	}
	menuButtonKey(t, w, events.KeyArrowLeft)
	if !b[1].Opened() {
		t.Fatal("左键没有反向循环")
	}
	menuButtonKey(t, w, events.KeyEscape)
	menuButtonPointer(t, w, events.PointerMove, 50)
	if b[0].Opened() || b[1].Opened() {
		t.Fatal("Esc 后悬停仍能开启菜单")
	}
	menuButtonClick(t, w, 50)
	menuButtonClick(t, w, 50)
	if b[0].Opened() {
		t.Fatal("锚点点击先关闭后又重开")
	}
	if err := b[0].Open(); err != nil {
		t.Fatal(err)
	}
	menuButtonKey(t, w, events.KeyTab)
	if b[0].Opened() || w.FocusedWidget() != b[1] {
		t.Fatal("Tab 没有关闭并继续焦点导航")
	}
	info := b[0].Snapshot()
	data := info.Attributes[MenuButtonInfoKey].(MenuButtonInfo)
	if info.Role != RoleButton || !data.HasMenu || !data.Grouped || data.Opened {
		t.Fatal("菜单按钮快照错误", data)
	}
}

func TestMenuButtonGroupEligibilityAndOrdering(t *testing.T) {
	w, root, b, _ := menuButtonGroupFixture(t)
	other := NewMenuButtonGroup()
	if other == b[0].Group() {
		t.Fatal("组没有独立身份")
	}
	if err := b[0].Open(); err != nil {
		t.Fatal(err)
	}
	root.AddChild(other)
	other.Arrange(geometry.Rect(0, 0, 500, 300))
	other.AddChild(b[1])
	menuButtonPointer(t, w, events.PointerMove, 150)
	if !b[0].Opened() || b[1].Opened() {
		t.Fatal("切换到了不同组")
	}
	b[0].Group().AddChild(b[1])
	separator := NewMenu()
	separator.AppendSeparator()
	b[1].SetMenu(separator)
	menuButtonKey(t, w, events.KeyArrowRight)
	if !b[2].Opened() {
		t.Fatal("空菜单参与导航")
	}
	root.AddChild(b[2])
	// 移出容器卸载旧面；重新开启后作为独立菜单保留。
	if err := b[2].Open(); err != nil {
		t.Fatal(err)
	}
	menuButtonPointer(t, w, events.PointerMove, 50)
	if !b[2].Opened() || b[0].Opened() {
		t.Fatal("解除分组没有结束联动或关闭独立菜单")
	}
	b[2].Close()
	b[1].SetMenu(b[0].Menu())
	b[0].Group().AddChild(b[2])
	b[0].Group().MoveChildBefore(b[2], b[0])
	if err := b[0].Open(); err != nil {
		t.Fatal(err)
	}
	menuButtonKey(t, w, events.KeyArrowLeft)
	if !b[2].Opened() {
		t.Fatal("导航使用注册顺序而非实际树顺序")
	}
	b[2].Close()
	disabled := NewMenu()
	disabled.Append("Unavailable", nil).SetEnabled(false)
	b[1].SetMenu(disabled)
	if err := b[1].Open(); err != nil || !b[1].Opened() {
		t.Fatal("全禁用但有可见项目的菜单被跳过", err)
	}
	b[1].Close()
	cover := newTestWidget()
	root.AddChild(cover)
	cover.Arrange(geometry.Rect(120, 20, 80, 40))
	if err := b[0].Open(); err != nil {
		t.Fatal(err)
	}
	menuButtonPointer(t, w, events.PointerMove, 150)
	if !b[0].Opened() || b[1].Opened() {
		t.Fatal("遮挡对象下方的菜单参与悬停")
	}
}

func TestMenuButtonGroupClosedReentry(t *testing.T) {
	for _, action := range []string{"destroy source", "destroy candidate", "change group", "open another", "reopen source"} {
		t.Run(action, func(t *testing.T) {
			w, root, b, _ := menuButtonGroupFixture(t)
			if err := b[0].Open(); err != nil {
				t.Fatal(err)
			}
			called := false
			b[0].ConnectClosed(func() {
				if called {
					return
				}
				called = true
				switch action {
				case "destroy source":
					b[0].base().destroy(b[0])
				case "destroy candidate":
					b[1].base().destroy(b[1])
				case "change group":
					other := NewMenuButtonGroup()
					root.AddChild(other)
					other.AddChild(b[1])
				case "open another":
					if err := b[2].Open(); err != nil {
						t.Fatal(err)
					}
				case "reopen source":
					if err := b[0].Open(); err != nil {
						t.Fatal(err)
					}
				}
			})
			menuButtonPointer(t, w, events.PointerMove, 150)
			if !called || b[1].Opened() {
				t.Fatal("关闭回调改变状态后旧切换仍执行")
			}
			if action == "open another" && !b[2].Opened() {
				t.Fatal("回调打开的菜单被覆盖")
			}
			if action == "reopen source" && !b[0].Opened() {
				t.Fatal("回调重开的源菜单被覆盖")
			}
		})
	}
}

func TestMenuButtonGroupInputErrorsAndIsolation(t *testing.T) {
	w, _, b, platform := menuButtonGroupFixture(t)
	if err := b[0].Open(); err != nil {
		t.Fatal(err)
	}
	platform.natives[0].showErr = errors.New("native popup failure")
	errorsSeen := 0
	b[1].ConnectOpenError(func(error) { errorsSeen++ })
	menuButtonPointer(t, w, events.PointerMove, 150)
	if errorsSeen != 1 || b[0].Opened() || b[1].Opened() || w.modalTarget != nil || len(w.Popovers()) != 0 {
		t.Fatal("打开失败没有清理或重复报告", errorsSeen)
	}
	if err := b[1].Open(); err == nil || errorsSeen != 1 {
		t.Fatal("同步失败没有返回错误，或重复发信号")
	}
	platform.natives[0].showErr = nil
	if err := b[0].Open(); err != nil {
		t.Fatal(err)
	}
	other := testWindowWithSurface(&window{surfaceBase: surfaceBase{platformWindow: &placementWindow{area: geometry.Rect(0, 0, 1000, 800)}}})
	candidate := NewMenuButton()
	candidate.SetMenu(b[0].Menu())
	otherGroup := NewMenuButtonGroup()
	other.SetWidget(otherGroup)
	otherGroup.AddChild(candidate)
	otherGroup.Arrange(geometry.Rect(0, 0, 80, 40))
	candidate.Arrange(geometry.Rect(0, 0, 80, 40))
	t.Cleanup(other.Destroy)
	menuButtonPointer(t, other, events.PointerMove, 40)
	if !b[0].Opened() || candidate.Opened() {
		t.Fatal("不同窗口的局部组发生联动")
	}
}

func TestMenuButtonClosedNotificationStopsAfterDestruction(t *testing.T) {
	w, _, b, _ := menuButtonGroupFixture(t)
	if err := b[0].Open(); err != nil {
		t.Fatal(err)
	}
	calls := 0
	b[0].ConnectClosed(func() { calls++; w.Destroy() })
	b[0].ConnectClosed(func() { calls += 100 })
	b[0].Close()
	if calls != 1 || !b[0].Destroyed() {
		t.Fatal("销毁后仍执行后续 Closed 回调", calls)
	}
}

// 模型接口不要求可比较；切片型实现也必须能安装、重开及更新菜单。
type menuSliceData []*MenuItem

func (m menuSliceData) ItemsCount() int                   { return len(m) }
func (m menuSliceData) ItemAt(index int) *MenuItem        { return m[index] }
func (m menuSliceData) ConnectItems(func()) signal.Handle { return signal.Handles(nil) }

func TestMenuButtonAcceptsUncomparableModel(t *testing.T) {
	w, _, buttons, _ := menuButtonGroupFixture(t)
	button := buttons[0]
	model := menuSliceData{NewMenuItem("Original", nil)}
	button.SetMenu(model)
	button.SetMenu(model)
	if err := button.Open(); err != nil {
		t.Fatal(err)
	}
	menuButtonKey(t, w, events.KeyArrowDown)
	if button.pm.content.active != model[0] {
		t.Fatal("切片型菜单模型没有参与键盘导航")
	}
	model[0] = NewMenuItem("Replacement", nil)
	button.SetMenu(model)
	if !button.Opened() || button.pm.content.active != nil {
		t.Fatal("显式重新安装模型没有清除旧选择或意外关闭菜单")
	}
	menuButtonKey(t, w, events.KeyArrowDown)
	if button.pm.content.active != model[0] {
		t.Fatal("重新安装后仍选择旧模型项目")
	}
	button.Close()
	if err := button.Open(); err != nil || !button.Opened() {
		t.Fatal("切片型菜单模型不能重新打开", err)
	}
}

func TestMenuButtonSetMenuReinstallsSameModel(t *testing.T) {
	w, _, buttons, _ := menuButtonGroupFixture(t)
	button := buttons[0]
	model := NewMenu()
	item := model.Append("Original", nil)
	button.SetMenu(model)
	if err := button.Open(); err != nil {
		t.Fatal(err)
	}
	menuButtonKey(t, w, events.KeyArrowDown)
	if button.pm.content.active != item {
		t.Fatal("初始选择无效")
	}
	button.SetMenu(model)
	if !button.Opened() || button.pm.content.active != nil {
		t.Fatal("显式 SetMenu 被相等判断吞掉，没有重新安装模型")
	}
}

func TestMenuButtonOpenErrorStopsAfterDestruction(t *testing.T) {
	w, _, buttons, platform := menuButtonGroupFixture(t)
	platform.transparentErr = errors.New("原生 popup 创建失败")
	calls := 0
	buttons[0].ConnectOpenError(func(error) { calls++; w.Destroy() })
	buttons[0].ConnectOpenError(func(error) { calls += 100 })
	menuButtonClick(t, w, 50)
	if calls != 1 || !buttons[0].Destroyed() || buttons[0].Opened() {
		t.Fatal("OpenError 回调销毁后仍通知后续监听者或残留原生菜单", calls)
	}
}

func TestMenuButtonGroupKeepsKeyboardChoiceOnStationaryMotion(t *testing.T) {
	w, _, buttons, _ := menuButtonGroupFixture(t)
	menuButtonClick(t, w, 50)
	menuButtonPointer(t, w, events.PointerMove, 150)
	if !buttons[1].Opened() {
		t.Fatal("真实移动没有切换菜单")
	}
	menuButtonKey(t, w, events.KeyArrowLeft)
	menuButtonPointer(t, w, events.PointerMove, 150)
	if !buttons[0].Opened() || buttons[1].Opened() {
		t.Fatal("重复原生移动消息覆盖了键盘选择")
	}
	menuButtonPointer(t, w, events.PointerMove, 151)
	if !buttons[1].Opened() {
		t.Fatal("同一按钮内的真实移动没有恢复鼠标切换")
	}
	menuButtonKey(t, w, events.KeyArrowLeft)
	menuButtonPointer(t, w, events.PointerLeave, 151)
	menuButtonPointer(t, w, events.PointerEnter, 151)
	menuButtonPointer(t, w, events.PointerMove, 151)
	if !buttons[0].Opened() {
		t.Fatal("静止位置的原生 Crossing 覆盖了键盘选择")
	}
	menuButtonPointer(t, w, events.PointerLeave, 450)
	menuButtonPointer(t, w, events.PointerEnter, 151)
	if !buttons[0].Opened() {
		t.Fatal("仅 Crossing 打开了菜单")
	}
	menuButtonPointer(t, w, events.PointerMove, 151)
	if !buttons[1].Opened() {
		t.Fatal("移开再回到同一位置时没有切换")
	}
}

// 导航只访问当前容器的可见后代，嵌套组和普通按钮不占菜单顺序。
func TestMenuButtonGroupNestedBoundary(t *testing.T) {
	w, _, buttons, _ := menuButtonGroupFixture(t)
	outer := buttons[0].Group()
	nested := NewMenuButtonGroup()
	outer.AddChild(nested)
	nested.AddChild(buttons[1])
	nested.Arrange(geometry.Rect(0, 0, 500, 300))
	if err := buttons[0].Open(); err != nil {
		t.Fatal(err)
	}
	menuButtonKey(t, w, events.KeyArrowRight)
	if !buttons[2].Opened() || buttons[1].Opened() {
		t.Fatal("导航进入了嵌套组")
	}
	buttons[2].Close()
	if err := buttons[1].Open(); err != nil {
		t.Fatal(err)
	}
	menuButtonKey(t, w, events.KeyArrowRight)
	if !buttons[1].Opened() {
		t.Fatal("单个嵌套成员切换到了外层组")
	}
	if info := outer.Snapshot(); info.Role != RoleHBox || len(info.Children) != 3 || len(info.Actions) != 0 {
		t.Fatal("容器的语义子树或动作不正确", info)
	}
}

func TestMenuCommandLiveEnablementAndCancelledPress(t *testing.T) {
	c := NewCommand()
	activated := 0
	c.ConnectActivate(func() { activated++ })
	model := NewMenu()
	first := model.Append("Copy", nil)
	first.SetCommand(c)
	second := model.Append("Other", nil)
	content := newMenuContent(model, 0, func(item *MenuItem) {
		if item.Command() != nil {
			item.Command().Activate()
		}
	})
	win := testWindowWithSurface(&window{})
	win.SetWidget(content)
	defer win.Destroy()
	content.observeCommands(true)
	size := content.Measure(layout.Unbounded()).Size
	content.Arrange(geometry.Rect(0, 0, size.Width, size.Height))
	if len(content.commands) != 1 {
		t.Fatal("命令观察未去重或缺失")
	}
	row := content.list.items[0].(*menuItemRow)
	point := row.Snapshot().Bounds.Center()
	for _, kind := range []events.EventType{events.PointerMove, events.PointerDown} {
		_ = win.DispatchEvent(events.PointerEvent{EventType: kind, Position: point, Button: events.PointerButtonLeft})
	}
	c.SetEnabled(false)
	info := row.Snapshot()
	if info.Enabled || info.Pressed || info.Selected || content.active != nil || !first.Enabled() {
		t.Fatal("实时禁用状态或局部 Enabled 被错误覆盖")
	}
	c.SetEnabled(true)
	_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerUp, Position: point, Button: events.PointerButtonLeft})
	if activated != 0 {
		t.Fatal("禁用再恢复后执行了过期点击")
	}
	c.SetEnabled(false)
	_ = win.DispatchEvent(shortcutPress(events.KeyHome, 0))
	if content.active != second {
		t.Fatal("键盘导航未跳过禁用操作")
	}
	first.SetEnabled(false)
	c.SetEnabled(true)
	if row.Snapshot().Enabled {
		t.Fatal("恢复命令错误恢复了局部禁用入口")
	}
	content.observeCommands(false)
	if len(content.commands) != 0 {
		t.Fatal("菜单隐藏仍保留观察连接")
	}
}

func TestMenuCommandColumnsAndLiveGesture(t *testing.T) {
	useTestApplication(t, &application{typo: &styleTypography{}, style: textStyleSheet(10, color.Black)})
	a, b := NewCommand(), NewCommand()
	x := NewShortcut(KeyGesture{Key: KeyN, Modifiers: ModControl})
	x.SetCommand(a)
	y := NewShortcut(KeyGesture{Key: KeyS, Modifiers: ModControl | ModShift})
	y.SetCommand(b)
	model := NewMenu()
	model.Append("New", nil).SetCommand(a)
	model.AppendSeparator()
	model.Append("A longer title", nil).SetCommand(b)
	model.Append("No shortcut", nil)
	content := newMenuContent(model, 0, nil)
	win := testWindowWithSurface(&window{})
	win.SetWidget(content)
	defer win.Destroy()
	content.observeCommands(true)
	arrange := func() {
		size := content.Measure(layout.Unbounded()).Size
		content.Arrange(geometry.Rect(0, 0, size.Width, size.Height))
	}
	arrange()
	first, last := content.list.items[0].(*menuItemRow), content.list.items[2].(*menuItemRow)
	if first.shortcut.Text() != x.Gesture().DisplayText() || last.shortcut.Text() != y.Gesture().DisplayText() {
		t.Fatal("提示未从关联命令获取")
	}
	right := func(r *menuItemRow) float32 { return r.shortcut.Rect().X + r.shortcut.Rect().Width }
	if first.shortcut.Rect().Width != measureWidget(first.shortcut, layout.Unbounded()).Width {
		t.Fatal("短快捷键必须以自然文字框贴齐列右侧")
	}
	if right(first) != right(last) || first.label.Rect().X != last.label.Rect().X {
		t.Fatal("菜单未共享列边界")
	}
	// 图标／字体无关：按独立测量结果检验总宽，不依赖被测列宽计算。
	titleW := float32(len("A longer title") * 10)
	hintW := measureWidget(last.shortcut, layout.Unbounded()).Width
	if content.naturalW != titleW+hintW+menuShortcutGap+2*menuItemPadding+2*menuContentPadding {
		t.Fatalf("自然列宽错误：%v", content.naturalW)
	}
	y.SetGesture(KeyGesture{Key: KeyF2})
	if last.shortcut.Text() != "F2" || content.valid {
		t.Fatal("打开期间的手势修改没有更新文字／尺寸")
	}
	arrange()
	if right(first) != right(last) {
		t.Fatal("动态缩短提示后列边界不一致")
	}
	x.SetEnabled(false)
	if first.shortcut.Text() == "" || !first.Snapshot().Enabled {
		t.Fatal("局部快捷键禁用不应隐藏提示或禁用菜单")
	}
	y.SetCommand(nil)
	if last.shortcut.Visible() {
		t.Fatal("解除末个快捷键关联后仍显示旧提示")
	}
	if got := first.Snapshot().Attributes[MenuItemInfoKey].(MenuItemInfo).Shortcut; got != x.Gesture().DisplayText() {
		t.Fatal("快照未反映最新提示")
	}
}

func TestMenuCheckedCommandColumnsAndProjection(t *testing.T) {
	useTestApplication(t, &application{typo: &styleTypography{}, style: textStyleSheet(10, color.Black)})
	command := NewCommand()
	model := NewMenu()
	model.Append("Long ordinary title", nil)
	model.Append("Check", nil).SetCommand(command)
	model.Append("Again", nil).SetCommand(command)
	content := newMenuContent(model, 0, nil)
	win := testWindowWithSurface(&window{})
	win.SetWidget(content)
	defer win.Destroy()
	content.observeCommands(true)
	arrange := func() {
		size := content.Measure(layout.Unbounded()).Size
		content.Arrange(geometry.Rect(0, 0, size.Width, size.Height))
	}
	arrange()
	before := content.naturalW
	first, checked := content.list.items[0].(*menuItemRow), content.list.items[1].(*menuItemRow)
	command.SetCheckable(true)
	if content.valid {
		t.Fatal("增加勾选能力没有失效菜单列宽")
	}
	arrange()
	// 指示列固定 16 DIP，后跟 8 DIP 间隔；不以被测宽度函数作为预期。
	if content.naturalW != before+24 || first.label.Rect().X != 32 || checked.label.Rect().X != 32 {
		t.Fatal("混合菜单没有预留一致的指示列", content.naturalW, first.label.Rect(), checked.label.Rect())
	}
	content.selectItem(model.ItemAt(1))
	command.SetChecked(true)
	info := checked.Snapshot()
	state := info.Attributes[MenuItemInfoKey].(MenuItemInfo)
	if !content.valid || !info.Selected || info.Role != RoleMenuItemCheckbox || !state.Checkable || !state.Checked || !checked.State().Has(style.Checked) {
		t.Fatal("仅更新勾选值重排菜单、丢失活动项或没有投影状态", info)
	}
	command.SetEnabled(false)
	if info = checked.Snapshot(); info.Enabled || !info.Attributes[MenuItemInfoKey].(MenuItemInfo).Checked || info.Selected {
		t.Fatal("禁用项丢失勾选值或残留导航高亮", info)
	}
	command.SetEnabled(true)
	command.SetCheckable(false)
	arrange()
	if content.naturalW != before || checked.Snapshot().Role != RoleMenuItem || checked.State().Has(style.Checked) {
		t.Fatal("去掉勾选能力没有恢复普通菜单")
	}
	command.SetCheckable(true)
	command.SetChecked(true)
	model.ItemAt(1).SetVisible(false)
	model.ItemAt(2).SetVisible(false)
	arrange()
	if content.naturalW != before {
		t.Fatal("隐藏勾选项仍参与指示列计算")
	}
	row := newMenuItemRow(nil)
	row.bind(model.ItemAt(1))
	row.bind(model.ItemAt(0))
	if row.State().Has(style.Checked) || row.Snapshot().Role != RoleMenuItem {
		t.Fatal("复用行保留了前一条目的勾选状态")
	}
	content.observeCommands(false)
	if len(content.commands) != 0 {
		t.Fatal("隐藏呈现后仍订阅命令状态")
	}
}

func TestMenuCheckedCommandActivationUsesBusinessHandler(t *testing.T) {
	command := NewCommand()
	command.SetCheckable(true)
	activated, accept := 0, false
	command.ConnectActivate(func() {
		activated++
		if accept {
			command.SetChecked(!command.Checked())
		}
	})
	model := NewMenu()
	item := model.Append("Check", nil)
	item.SetCommand(command)
	pm := NewPopoverMenu(nil)
	pm.SetMenu(model)
	content := newMenuContent(model, 0, pm.activate)
	win := testWindowWithSurface(&window{})
	win.SetWidget(content)
	defer win.Destroy()
	press := func(key events.Key) {
		t.Helper()
		if err := win.DispatchEvent(shortcutPress(key, 0)); err != nil {
			t.Fatal(err)
		}
	}
	press(events.KeyHome)
	press(events.KeyEnter)
	if activated != 1 || command.Checked() {
		t.Fatal("菜单替业务处理函数提交了未接受的勾选值")
	}
	accept = true
	press(events.KeySpace)
	if activated != 2 || !command.Checked() {
		t.Fatal("菜单重复执行或没有使用业务提交结果")
	}
	shortcut := NewShortcut(KeyGesture{Key: KeyF5})
	shortcut.SetCommand(command)
	defer shortcut.SetCommand(nil)
	win.Shortcuts().AddShortcut(shortcut)
	press(events.KeyF5)
	if activated != 3 || command.Checked() {
		t.Fatal("快捷键与菜单没有共享同一激活路径")
	}
	command.SetChecked(true)
	if activated != 3 {
		t.Fatal("程序更新状态触发了业务操作")
	}
}

func TestMenuCheckIndicatorPartAndBounds(t *testing.T) {
	// Software 输出是预乘 RGBA。只核对有覆盖像素的通道比例和指示区域边界，
	// 不约束斜线抗锯齿的具体覆盖值，也不依赖字体或平台金图。
	useTestApplication(t, &application{style: style.Sheet(
		style.Name(styleNameMenuItem).Part("indicator").ForegroundColor(color.RGBA{R: 255, A: 255}),
		style.Name(styleNameMenuItem).Part("indicator").State(style.Disabled).ForegroundColor(color.RGBA{B: 255, A: 255}),
	)})
	command := NewCommand()
	command.SetCheckable(true)
	item := NewMenuItem("", nil)
	item.SetCommand(command)
	row := newMenuItemRow(nil)
	row.bind(item)
	row.Arrange(geometry.Rect(0, 0, 80, 28))
	for _, checked := range []bool{false, true} {
		command.SetChecked(checked)
		for _, disabled := range []bool{false, true} {
			command.SetEnabled(!disabled)
			for _, scale := range []float32{1, 2} {
				out := new(transparentFrame)
				backend, err := software.NewPainter(out)
				if err != nil {
					t.Fatal(err)
				}
				backend.Begin(80*scale, 28*scale, scale)
				backend.Clear(graphics.Color{})
				row.Paint(newPainter(backend, geometry.Rect(0, 0, 80, 28), scale))
				backend.End()
				ink := 0
				for y := 0; y < int(28*scale); y++ {
					for x := 0; x < int(80*scale); x++ {
						r, g, b, a := out.image.At(x, y).RGBA()
						if a == 0 {
							continue
						}
						ink++
						if !checked || float32(x) < 8*scale || float32(x) >= 24*scale || float32(y) < 6*scale || float32(y) >= 22*scale {
							t.Fatalf("勾号越出指示区域或未勾选仍有绘制：checked=%v scale=%g pixel=%d,%d", checked, scale, x, y)
						}
						if g != 0 || !disabled && (r != a || b != 0) || disabled && (b != a || r != 0) {
							t.Fatal("indicator Part 或 Disabled 前景色没有生效")
						}
					}
				}
				backend.Destroy()
				if checked && ink == 0 {
					t.Fatal("已勾选项缺少矢量指示")
				}
			}
		}
	}
}

func TestMenuCommandObservationFollowsVisibleModel(t *testing.T) {
	a, b := NewCommand(), NewCommand()
	m := NewMenu()
	m.Append("First", nil).SetCommand(a)
	m.Append("Again", nil).SetCommand(a)
	c := newMenuContent(m, 0, nil)
	win := testWindowWithSurface(&window{})
	win.SetWidget(c)
	defer win.Destroy()
	c.observeCommands(true)
	if len(c.commands) != 1 {
		t.Fatal("同一操作被重复订阅")
	}
	m.ItemAt(0).SetCommand(b)
	if len(c.commands) != 2 {
		t.Fatal("模型修改没有更新命令集合")
	}
	m.ItemAt(1).SetVisible(false)
	if len(c.commands) != 1 {
		t.Fatal("不可见项目仍保留命令观察")
	}
	replacement := NewMenu()
	replacement.Append("Replacement", nil).SetCommand(a)
	c.SetModel(replacement)
	if !c.observing || len(c.commands) != 1 || c.commands[a] == nil {
		t.Fatal("打开期间更换模型丢失观察或保留旧关系")
	}
	c.observeCommands(false)
	a.SetEnabled(false)
	c.observeCommands(true)
	if len(c.commands) != 1 {
		t.Fatal("重新打开没有恢复观察")
	}
}

func TestMenuCommandCloseCallbackInvalidatesActivation(t *testing.T) {
	for _, mode := range []string{"replace", "disable-cycle", "destroy"} {
		t.Run(mode, func(t *testing.T) {
			old, next := NewCommand(), NewCommand()
			n := 0
			old.ConnectActivate(func() { n++ })
			next.ConnectActivate(func() { n++ })
			item := NewMenuItem("Run", nil)
			item.SetCommand(old)
			owner := testWindowWithSurface(&window{})
			p := &popover{owner: owner, visible: true}
			anchor := newTestWidget()
			owner.SetWidget(anchor)
			p.anchor = anchor
			pm := &PopoverMenu{popover: p, anchor: anchor}
			p.ConnectClosed(func() {
				switch mode {
				case "replace":
					item.SetCommand(next)
				case "disable-cycle":
					old.SetEnabled(false)
					old.SetEnabled(true)
				case "destroy":
					owner.Destroy()
				}
			})
			pm.activate(item)
			if n != 0 {
				t.Fatal("关闭回调改变生命周期后仍执行过期操作")
			}
		})
	}
}

func TestMenuButtonFocusPoliciesPreservePointerOpening(t *testing.T) {
	w, root, buttons, _ := menuButtonGroupFixture(t)
	input := newTestWidget()
	input.SetFocusable(true)
	input.Arrange(geometry.Rect(350, 200, 80, 30))
	root.AddChild(input)
	w.SetFocusedWidget(input)
	lost := 0
	input.ConnectFocused(func(focused bool) {
		if !focused {
			lost++
		}
	})
	for _, b := range buttons {
		b.SetFocusOnPointer(false)
	}
	menuButtonClick(t, w, 50)
	if !buttons[0].Opened() || w.FocusedWidget() != input || lost != 0 {
		t.Fatal("菜单指针打开重复强制聚焦锚点")
	}
	menuButtonPointer(t, w, events.PointerMove, 150)
	if buttons[0].Opened() || !buttons[1].Opened() || w.FocusedWidget() != input || lost != 0 {
		t.Fatal("菜单悬停切换绕过指针聚焦设置")
	}
	if err := buttons[0].Open(); err != nil || w.FocusedWidget() != buttons[0] {
		t.Fatal("显式Open被指针策略禁止", err)
	}
	menuButtonKey(t, w, events.KeyArrowRight)
	if !buttons[1].Opened() || w.FocusedWidget() != buttons[1] {
		t.Fatal("键盘菜单切换错误受指针策略限制")
	}
	buttons[0].Group().SetCanContainFocus(false)
	if w.FocusedWidget() != nil || buttons[1].Opened() {
		t.Fatal("关闭准入没有结束锚点焦点及其菜单")
	}
	w.SetFocusedWidget(input)
	menuButtonClick(t, w, 250)
	if !buttons[2].Opened() || w.FocusedWidget() != input {
		t.Fatal("禁止锚点焦点错误禁止了鼠标打开菜单")
	}
}

// --- 菜单键盘、指针与虚拟行输入 ---

func TestMenuPointerAndKeyboardShareOneActiveItem(t *testing.T) {
	normal, highlight := color.NRGBA{R: 255, G: 255, B: 255, A: 255}, color.NRGBA{B: 220, A: 255}
	useTestApplication(t, &application{typo: &styleTypography{}, style: style.Sheet(
		style.Name(styleNameMenuItemText).FontSize(12),
		style.Name(styleNameMenuItemTextDisabled).FontSize(12),
		style.Name(styleNameMenuItem).BackgroundColor(normal),
		style.Name(styleNameMenuItem).State(style.Hovered).BackgroundColor(highlight),
		style.Name(styleNameMenuItem).State(style.Pressed).BackgroundColor(highlight),
	)})
	m := NewMenu()
	first, second := m.Append("First", nil), m.Append("Second", nil)
	disabled := m.Append("Disabled", nil)
	disabled.SetEnabled(false)
	m.AppendSeparator()
	m.Append("Last", nil)
	var activated *MenuItem
	content := newMenuContent(m, 0, func(item *MenuItem) { activated = item })
	win := testWindowWithSurface(&window{})
	win.SetWidget(content)
	defer win.Destroy()
	size := content.Measure(layout.Unbounded()).Size
	content.Arrange(geometry.Rect(0, 0, size.Width, size.Height))
	row := func(index int) *menuItemRow {
		t.Helper()
		r, ok := content.list.items[index].(*menuItemRow)
		if !ok {
			t.Fatalf("row %d was not realized", index)
		}
		return r
	}
	point := func(index int, text bool) geometry.Point {
		r := row(index)
		bounds := r.Snapshot().Bounds
		p := geometry.Point{X: bounds.X + 2, Y: bounds.Y + bounds.Height/2}
		var want Widget = r
		if text {
			bounds = r.label.Snapshot().Bounds
			p = geometry.Point{X: bounds.X + bounds.Width/2, Y: bounds.Y + bounds.Height/2}
			want = r.label
		}
		if got := Pick(content, p); got != want {
			t.Fatalf("point did not hit intended target: %T, want %T", got, want)
		}
		return p
	}
	assertActive := func(want *MenuItem) {
		t.Helper()
		painted, selected := 0, 0
		for _, widget := range content.list.items {
			r := widget.(*menuItemRow)
			background, ok := r.resolvedStyle().BackgroundColor()
			lit := ok && background != nil && graphics.ColorOf(background) == graphics.ColorOf(highlight)
			if lit {
				painted++
			}
			if r.Snapshot().Selected {
				selected++
			}
			if lit != (want != nil && r.mi == want) || lit != r.Snapshot().Selected {
				t.Errorf("row %q: painted=%t selected=%t, want active=%v", r.mi.Label(), lit, r.Snapshot().Selected, want)
			}
		}
		count := 0
		if want != nil {
			count = 1
		}
		if content.active != want || painted != count || selected != count {
			t.Fatalf("active=%v, painted=%d selected=%d; want %v/%d", content.active, painted, selected, want, count)
		}
	}
	move := func(index int, text bool) {
		t.Helper()
		if err := win.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: point(index, text)}); err != nil {
			t.Fatal(err)
		}
	}
	press := func(key events.Key) {
		t.Helper()
		if err := win.DispatchEvent(shortcutPress(key, 0)); err != nil {
			t.Fatal(err)
		}
	}

	move(0, false)
	assertActive(first)
	if err := win.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Button: events.PointerButtonLeft, Position: point(0, false)}); err != nil {
		t.Fatal(err)
	}
	press(events.KeyEnd)
	assertActive(m.ItemAt(4))
	if err := win.DispatchEvent(events.PointerEvent{EventType: events.PointerUp, Button: events.PointerButtonLeft, Position: point(0, false)}); err != nil {
		t.Fatal(err)
	}
	// 指针不移动时，新的按下也应接管键盘活动项。
	if err := win.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Button: events.PointerButtonLeft, Position: point(0, false)}); err != nil {
		t.Fatal(err)
	}
	assertActive(first)
	if err := win.DispatchEvent(events.PointerEvent{EventType: events.PointerUp, Button: events.PointerButtonLeft, Position: point(0, false)}); err != nil {
		t.Fatal(err)
	}
	// 直接从一行留白移到另一行的 Label。
	move(1, true)
	assertActive(second)
	for range 100 {
		move(0, true)
		assertActive(first)
		move(1, true)
		assertActive(second)
	}
	press(events.KeyHome)
	assertActive(first)
	// 指针仍在 Second 的 Label 内，没有 crossing，普通移动也必须接管。
	move(1, true)
	assertActive(second)
	move(1, false)
	assertActive(second)
	move(1, true)
	assertActive(second)
	move(2, true)
	assertActive(nil)
	move(3, false)
	assertActive(nil)
	move(0, true)
	assertActive(first)
	if err := win.DispatchEvent(events.PointerEvent{EventType: events.PointerLeave}); err != nil {
		t.Fatal(err)
	}
	assertActive(nil)
	for _, widget := range content.list.items {
		if widget.Snapshot().Hovered {
			t.Fatal("离开菜单后仍有物理悬停")
		}
	}
	// 原生 Enter 只有 crossing，没有普通 Motion。
	if err := win.DispatchEvent(events.PointerEvent{EventType: events.PointerEnter, Position: point(1, true)}); err != nil {
		t.Fatal(err)
	}
	assertActive(second)
	// 顶部留白不能保留上一项高亮。
	blank := geometry.Point{X: size.Width / 2, Y: 1}
	if target := Pick(content, blank); target != content {
		t.Fatalf("留白命中 %T，预期菜单内容", target)
	}
	if err := win.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: blank}); err != nil {
		t.Fatal(err)
	}
	assertActive(nil)
	press(events.KeyHome)
	assertActive(first)
	press(events.KeyEnd)
	assertActive(m.ItemAt(4))
	// 指针静止时键盘活动项保持，下一次移动才接管。
	move(1, true)
	assertActive(second)
	press(events.KeyEnter)
	if activated != second {
		t.Fatal("keyboard activation ignored the pointer's active item")
	}
}

// 滚动、Reveal 和虚拟行复用不能把旧物理悬停当作新的菜单选择。
func TestMenuActiveItemAcrossScrollingAndRowReuse(t *testing.T) {
	useTestApplication(t, &application{typo: &styleTypography{}})
	model := NewMenu()
	for range 50 {
		model.Append("Item", nil)
	}
	content := newMenuContent(model, 100, func(*MenuItem) {})
	win := testWindowWithSurface(&window{})
	win.SetWidget(content)
	defer win.Destroy()
	size := content.Measure(layout.Unbounded()).Size
	content.Arrange(geometry.Rect(13, 27, size.Width, size.Height))
	if !content.sv.vbar.Visible() {
		t.Fatal("回归场景没有实际出现滚动条")
	}
	move := func(position geometry.Point) {
		t.Helper()
		if err := win.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: position}); err != nil {
			t.Fatal(err)
		}
	}
	first := content.list.items[0].Snapshot().Bounds
	point := geometry.Point{X: first.X + 2, Y: first.Y + first.Height/2}
	move(point)
	if content.active != model.ItemAt(0) {
		t.Fatal("未选择首行")
	}
	move(geometry.Point{X: first.X + first.Width + 1, Y: point.Y})
	if content.active != nil {
		t.Fatal("移入菜单行与滚动条之间的间距后仍有活动项")
	}
	move(point)
	bar := content.sv.vbar.Snapshot().Bounds
	move(geometry.Point{X: bar.X + bar.Width/2, Y: bar.Y + bar.Height/2})
	if content.active != nil {
		t.Fatal("移入滚动条后仍有活动项")
	}
	move(point)
	if err := win.DispatchEvent(shortcutPress(events.KeyEnd, 0)); err != nil {
		t.Fatal(err)
	}
	// Reveal 请求在下一轮布局生效；此测试没有原生帧循环，显式执行该阶段。
	content.Arrange(content.Rect())
	last := model.ItemAt(49)
	if content.active != last || content.sv.ScrollY() <= 0 || content.list.items[49] == nil {
		t.Fatalf("键盘未选中并显示末行：active=%p want=%p scroll=%g rows=%v", content.active, last, content.sv.ScrollY(), content.list.items)
	}
	if content.list.items[0] != nil || !content.list.items[49].Snapshot().Selected {
		t.Fatal("首行未回收或末行缺少活动项投影")
	}
	content.sv.SetScrollY(0)
	if content.active != last || content.list.items[0] == nil {
		t.Fatal("单纯布局和行复用改变了键盘活动项")
	}
	for _, widget := range content.list.items {
		row := widget.(*menuItemRow)
		if row.Snapshot().Selected != (row.mi == last) {
			t.Fatal("复用行保留了旧选中状态")
		}
	}
	move(point)
	if content.active != model.ItemAt(0) || !content.list.items[0].Snapshot().Selected {
		t.Fatal("滚回后指针未在原位置接管活动项")
	}
}

// 内容有窗口局部偏移时，普通移动、空白和原生 Enter 使用相同的命中坐标。
func TestMenuPointerSelectionWithContentOffset(t *testing.T) {
	useTestApplication(t, &application{typo: &styleTypography{}})
	model := NewMenu()
	first := model.Append("First", nil)
	second := model.Append("Second", nil)
	content := newMenuContent(model, 0, func(*MenuItem) {})
	win := testWindowWithSurface(&window{})
	win.SetWidget(content)
	defer win.Destroy()
	size := content.Measure(layout.Unbounded()).Size
	origin := geometry.Point{X: 19, Y: 31}
	content.Arrange(geometry.Rectangle{Pos: origin, Size: size})
	for _, eventType := range []events.EventType{events.PointerMove, events.PointerEnter} {
		for index, item := range []*MenuItem{first, second} {
			if err := win.DispatchEvent(events.PointerEvent{EventType: events.PointerLeave}); err != nil {
				t.Fatal(err)
			}
			if content.active != nil {
				t.Fatal("离开后活动项未清空")
			}
			bounds := content.list.items[index].Snapshot().Bounds
			position := geometry.Point{X: bounds.X + 2, Y: bounds.Y + bounds.Height/2}
			if err := win.DispatchEvent(events.PointerEvent{EventType: eventType, Position: position}); err != nil {
				t.Fatal(err)
			}
			if content.active != item || !content.list.items[index].Snapshot().Selected {
				t.Fatalf("事件 %v 未在有偏移的菜单选择 %q", eventType, item.Label())
			}
			if err := win.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: geometry.Point{X: origin.X + 1, Y: origin.Y + 1}}); err != nil {
				t.Fatal(err)
			}
			if content.active != nil || content.list.items[index].Snapshot().Selected {
				t.Fatal("移动到有偏移的菜单留白后仍有活动项")
			}
		}
	}
}

func TestMenuKeyboardNavigationAndActivation(t *testing.T) {
	m := NewMenu()
	m.Append("disabled", nil).SetEnabled(false)
	first := m.Append("first", nil)
	m.AppendSeparator()
	m.Append("hidden", nil).SetVisible(false)
	last := m.Append("last", nil)
	var activated *MenuItem
	content := newMenuContent(m, 0, func(item *MenuItem) { activated = item })
	win := testWindowWithSurface(&window{})
	win.SetWidget(content)
	defer win.Destroy()
	press := func(key events.Key) {
		t.Helper()
		if err := win.DispatchEvent(shortcutPress(key, 0)); err != nil {
			t.Fatal(err)
		}
	}
	press(events.KeyArrowDown)
	if content.active != first {
		t.Fatal("did not skip disabled item")
	}
	press(events.KeyArrowDown)
	if content.active != last {
		t.Fatal("did not skip hidden/separator items")
	}
	press(events.KeyArrowDown)
	if content.active != first {
		t.Fatal("did not wrap")
	}
	press(events.KeyEnd)
	if content.active != last {
		t.Fatal("End")
	}
	press(events.KeyEnter)
	if activated != last {
		t.Fatal("Enter did not activate selected item")
	}
	last.SetEnabled(false)
	press(events.KeyArrowUp)
	if content.active != first {
		t.Fatal("disabled active item retained")
	}
	first.SetVisible(false)
	activated = nil
	press(events.KeySpace)
	if activated != nil || content.active != nil {
		t.Fatal("activated unavailable item")
	}
	first.SetVisible(true)
	press(events.KeyHome)
	press(events.KeySpace)
	if activated != first {
		t.Fatal("Home/Space")
	}
	row := newMenuItemRow(nil)
	row.bind(first)
	row.SetState(style.Selected, true)
	if !row.Snapshot().Selected {
		t.Fatal("keyboard highlight missing from snapshot")
	}
	row.bind(last)
	if row.Snapshot().Selected {
		t.Fatal("pooled row retained selection")
	}
}

// 递归模型的变更来源互斥，且列表的全部成员变更入口都维护通知。
func TestMenuSubmenuSourcesAndSharedMembership(t *testing.T) {
	item := NewMenuItem("branch", func() {})
	child := NewMenu()
	membership := NewMenu()
	notifications := 0
	membership.ConnectItems(func() {
		notifications++
		if membership.ItemAt(0).Submenu() == nil {
			t.Fatal("AppendSubmenu 暴露了未初始化条目")
		}
	})
	membership.AppendSubmenu("Ready", child)
	if notifications != 1 {
		t.Fatal("AppendSubmenu 没有原子发布完整条目", notifications)
	}
	item.SetSubmenu(child)
	if item.Action() != nil || item.Command() != nil || item.Submenu() != child {
		t.Fatal("Submenu 未替换旧动作")
	}
	command := NewCommand()
	item.SetCommand(command)
	if item.Submenu() != nil || item.Command() != command {
		t.Fatal("Command 未替换分支")
	}
	item.SetSubmenu(nil)
	if item.Submenu() != nil || item.Command() != nil || item.Action() != nil {
		t.Fatal("清除分支恢复了旧来源")
	}
	a, b := NewMenu(), NewMenu()
	a.SetItems([]*MenuItem{item})
	b.Insert(0, item)
	ac, bc := 0, 0
	a.ConnectItems(func() { ac++ })
	b.ConnectItems(func() { bc++ })
	item.SetLabel("new")
	if ac != 1 || bc != 1 {
		t.Fatal("共享条目没有通知所有成员", ac, bc)
	}
	a.Remove(0)
	ac = 0
	bc = 0
	item.SetEnabled(false)
	if ac != 0 || bc != 1 {
		t.Fatal("移除条目未解除通知", ac, bc)
	}
	item.SetAction(func() {})
	bc = 0
	item.SetAction(func() {})
	if bc != 0 {
		t.Fatal("单纯更新 Action 不应重建菜单行")
	}
}

func TestMenuSubmenuKeyboardChainAndActivation(t *testing.T) {
	for _, modal := range []bool{true, false} {
		t.Run(fmt.Sprint(modal), func(t *testing.T) {
			w, plat, makePopup := transientFixture(t)
			pm := NewPopoverMenu(makePopup(nil).anchor)
			pm.SetModal(modal)
			t.Cleanup(pm.Destroy)
			calls := 0
			grand := NewMenu()
			grand.Append("Leaf", func() { calls++ })
			child := NewMenu()
			child.Append("Disabled", nil).SetEnabled(false)
			child.AppendSubmenu("More", grand)
			model := NewMenu()
			first := model.AppendSubmenu("Recent", child)
			model.Append("Other", nil)
			pm.SetMenu(model)
			if err := pm.ShowAt(geometry.Point{}); err != nil {
				t.Fatal(err)
			}
			key := func(k events.Key) {
				t.Helper()
				e := shortcutPress(k, 0)
				if modal {
					if err := w.DispatchEvent(e); err != nil {
						t.Fatal(err)
					}
				} else {
					if err := pm.popover.DispatchEvent(e); err != nil {
						t.Fatal(err)
					}
				}
			}
			key(events.KeyArrowDown)
			key(events.KeyArrowRight)
			if pm.child == nil || !pm.child.Visible() || pm.expanded != first || !pm.Visible() || pm.child.content.active != child.ItemAt(1) {
				t.Fatal("右键未展开可用子项")
			}
			info := pm.rowFor(first).Snapshot().Attributes[MenuItemInfoKey].(MenuItemInfo)
			if !info.HasSubmenu || !info.Expanded {
				t.Fatal("分支语义缺失")
			}
			key(events.KeyArrowRight)
			if pm.child.child == nil || !pm.child.child.Visible() || len(w.Popovers()) != 3 {
				t.Fatal("孙菜单未展开")
			}
			if modal && w.modalTarget != pm.nativeHost() {
				t.Fatal("子菜单抢占根模态登记")
			}
			if err := pm.child.popover.DispatchEvent(shortcutPress(events.KeyArrowDown, 0)); modal && err == nil {
				t.Fatal("公共子层输入绕过了模态所有权")
			}
			key(events.KeyEscape)
			if !pm.child.Visible() || pm.child.child.Visible() || !pm.Visible() {
				t.Fatal("Esc 未只关闭当前层")
			}
			key(events.KeyArrowRight)
			key(events.KeyEnter)
			if calls != 1 || pm.Visible() || len(w.Popovers()) != 0 {
				t.Fatal("叶项未整链关闭并执行一次", calls, len(w.Popovers()))
			}
			if err := pm.ShowAt(geometry.Point{}); err != nil {
				t.Fatal(err)
			}
			key(events.KeyArrowDown)
			key(events.KeyArrowRight)
			key(events.KeyArrowRight)
			if len(plat.natives) != 3 {
				t.Fatal("重开多级菜单增加了缓存数量", len(plat.natives))
			}
			key(events.KeyArrowLeft)
			if pm.child.child.Visible() || !pm.child.Visible() {
				t.Fatal("Left 没有返回父层")
			}
			key(events.KeyEscape)
			key(events.KeyEscape)
			if pm.Visible() {
				t.Fatal("根 Esc 没有关闭菜单链")
			}
		})
	}
}

func TestMenuSubmenuModelChangesAndCycles(t *testing.T) {
	_, _, makePopup := transientFixture(t)
	pm := NewPopoverMenu(makePopup(nil).anchor)
	t.Cleanup(pm.Destroy)
	child := NewMenu()
	leaf := child.Append("Leaf", nil)
	m := NewMenu()
	branch := m.AppendSubmenu("Branch", child)
	pm.SetMenu(m)
	if err := pm.ShowAt(geometry.Point{}); err != nil {
		t.Fatal(err)
	}
	pm.content.selectItem(branch)
	pm.activate(branch)
	if pm.child == nil || !pm.child.Visible() {
		t.Fatal("分支未展开")
	}
	leaf.SetVisible(false)
	if pm.child.Visible() || pm.expanded != nil {
		t.Fatal("子模型变空后仍展开")
	}
	leaf.SetVisible(true)
	pm.activate(branch)
	branch.SetEnabled(false)
	if pm.child.Visible() {
		t.Fatal("父项禁用后子层仍可交互")
	}
	branch.SetEnabled(true)
	branch.SetSubmenu(m)
	failures := 0
	pm.ConnectError(func(error) { failures++ })
	pm.activate(branch)
	if failures != 1 || pm.expanded != nil || !pm.Visible() {
		t.Fatal("循环模型没有隔离为一次展开失败")
	}
	empty := NewMenu()
	branch.SetSubmenu(empty)
	pm.activate(branch)
	if pm.expanded != nil || failures != 1 {
		t.Fatal("空分支展开或被当作失败")
	}
	child.ItemAt(0).SetEnabled(false)
	branch.SetSubmenu(child)
	pm.activate(branch)
	if !pm.child.Visible() || pm.child.content.active != nil {
		t.Fatal("全禁用菜单应能查看而不可选")
	}
}

func TestMenuSubmenuCloseCallbackInvalidatesActivation(t *testing.T) {
	for _, change := range []string{"disable", "replace", "reopen", "owner", "clear-remove-parent", "clear-remove-leaf", "clear-replace-child", "destroy-child"} {
		t.Run(change, func(t *testing.T) {
			w, _, makePopup := transientFixture(t)
			pm := NewPopoverMenu(makePopup(nil).anchor)
			t.Cleanup(pm.Destroy)
			calls := 0
			child := NewMenu()
			leaf := child.Append("Leaf", func() { calls++ })
			m := NewMenu()
			branch := m.AppendSubmenu("Branch", child)
			pm.SetMenu(m)
			if err := pm.ShowAt(geometry.Point{}); err != nil {
				t.Fatal(err)
			}
			pm.activate(branch)
			h := pm.ConnectClosed(func() {
				switch change {
				case "disable":
					branch.SetEnabled(false)
				case "replace":
					leaf.SetAction(func() { calls += 10 })
				case "reopen":
					if err := pm.ShowAt(geometry.Point{}); err != nil {
						t.Fatal(err)
					}
				case "owner":
					w.Destroy()
				case "clear-remove-parent":
					pm.SetMenu(nil)
					m.Remove(0)
				case "clear-remove-leaf":
					pm.SetMenu(nil)
					pm.child.SetMenu(nil)
					child.Remove(0)
				case "clear-replace-child":
					pm.SetMenu(nil)
					pm.child.SetMenu(menuSliceModel{leaf})
					pm.child.SetMenu(nil)
				case "destroy-child":
					pm.child.Destroy()
				}
			})
			t.Cleanup(h.Disconnect)
			pm.child.activate(leaf)
			if calls != 0 {
				t.Fatal("关闭回调变更来源后执行了旧动作", calls)
			}
		})
	}
}

func TestMenuSubmenuHoverUsesCancelableTimer(t *testing.T) {
	w, _, makePopup := transientFixture(t)
	q := newTimerTestQueue(t)
	w.app = q.app
	pm := NewPopoverMenu(makePopup(nil).anchor)
	t.Cleanup(pm.Destroy)
	child := NewMenu()
	child.Append("Leaf", nil)
	m := NewMenu()
	branch := m.AppendSubmenu("Branch", child)
	m.Append("Other", nil)
	pm.SetMenu(m)
	if err := pm.ShowAt(geometry.Point{}); err != nil {
		t.Fatal(err)
	}
	host := pm.nativeHost()
	point := pm.rowFor(branch).windowRect().Center()
	if err := pm.popover.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: point}); err != nil {
		t.Fatal(err)
	}
	if pm.pending != branch || pm.hoverTimer == nil || !pm.hoverTimer.Active() {
		t.Fatal("悬停未安排展开任务")
	}
	pm.content.selectItem(m.ItemAt(1))
	_ = pm.popover.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: point})
	if pm.content.active != branch {
		t.Fatal("普通 PointerMove 未恢复实际命中的活动项")
	}
	host.paint()
	if !pm.hoverTimer.Active() {
		t.Fatal("绘制取消了待展开分支")
	}
	q.advance(100 * time.Millisecond)
	point.X++
	_ = pm.popover.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: point})
	q.advance(150 * time.Millisecond)
	q.dispatch(t)
	if pm.child == nil || !pm.child.Visible() {
		t.Fatal("悬停到期未展开")
	}
	// 合成重复移动不会重新安排或关闭刚展开的菜单。
	_ = pm.popover.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: point})
	if !pm.child.Visible() || pm.hoverTimer.Active() {
		t.Fatal("重复移动改变了当前分支")
	}
	leafPoint := pm.child.rowFor(child.ItemAt(0)).windowRect().Center()
	_ = pm.child.popover.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: leafPoint})
	if !pm.keyboardChild {
		t.Fatal("指针进入子层后键盘活动层未更新")
	}
	_ = w.DispatchEvent(shortcutPress(events.KeyEnter, 0))
	if pm.Visible() {
		t.Fatal("指针选中子项后，键盘未在该层确认")
	}
	if pm.hoverTimer.Active() {
		t.Fatal("关闭后仍有悬停任务")
	}
}

// 非可比较的模型也可以借给多个呈现者；循环检测不能对接口做无条件 ==。
type menuSliceModel []*MenuItem

func (m menuSliceModel) ItemsCount() int                   { return len(m) }
func (m menuSliceModel) ItemAt(i int) *MenuItem            { return m[i] }
func (m menuSliceModel) ConnectItems(func()) signal.Handle { return signal.Handles(nil) }

func TestMenuSubmenuBorrowsNonComparableModel(t *testing.T) {
	_, _, makePopup := transientFixture(t)
	pm := NewPopoverMenu(makePopup(nil).anchor)
	t.Cleanup(pm.Destroy)
	shared := menuSliceModel{NewMenuItem("Leaf", nil)}
	m := NewMenu()
	a := m.AppendSubmenu("A", shared)
	b := m.AppendSubmenu("B", shared)
	pm.SetMenu(m)
	if err := pm.ShowAt(geometry.Point{}); err != nil {
		t.Fatal(err)
	}
	pm.activate(a)
	if pm.child == nil || !pm.child.Visible() {
		t.Fatal("非可比较模型未展开")
	}
	old := pm.child.nativeHost().platformPopup
	pm.activate(b)
	if !pm.child.Visible() || pm.expanded != b || pm.child.nativeHost().platformPopup != old {
		t.Fatal("共享模型的兄弟项未复用同层资源")
	}
}

func TestMenuSubmenuShowFailureLeavesParentUsable(t *testing.T) {
	w, plat, makePopup := transientFixture(t)
	pm := NewPopoverMenu(makePopup(nil).anchor)
	t.Cleanup(pm.Destroy)
	child := NewMenu()
	child.Append("Leaf", nil)
	m := NewMenu()
	m.AppendSubmenu("Branch", child)
	calls := 0
	m.Append("Other", func() { calls++ })
	pm.SetMenu(m)
	failures := 0
	pm.ConnectError(func(error) { failures++ })
	if err := pm.ShowAt(geometry.Point{}); err != nil {
		t.Fatal(err)
	}
	plat.transparentErr = errors.New("注入子层创建失败")
	_ = w.DispatchEvent(shortcutPress(events.KeyArrowDown, 0))
	_ = w.DispatchEvent(shortcutPress(events.KeyArrowRight, 0))
	if failures != 1 || !pm.Visible() || pm.expanded != nil || len(w.Popovers()) != 1 {
		t.Fatal("子层失败破坏了父层", failures)
	}
	_ = w.DispatchEvent(shortcutPress(events.KeyArrowDown, 0))
	_ = w.DispatchEvent(shortcutPress(events.KeyEnter, 0))
	if calls != 1 || pm.Visible() {
		t.Fatal("失败后父层不能继续执行")
	}
}

func TestMenuSubmenuPointerBodiesAndLeaf(t *testing.T) {
	_, _, makePopup := transientFixture(t)
	pm := NewPopoverMenu(makePopup(nil).anchor)
	t.Cleanup(pm.Destroy)
	child := NewMenu()
	calls := 0
	child.Append("Leaf", func() { calls++ })
	m := NewMenu()
	branch := m.AppendSubmenu("Branch", child)
	pm.SetMenu(m)
	if err := pm.ShowAt(geometry.Point{}); err != nil {
		t.Fatal(err)
	}
	click := func(p *PopoverMenu, item *MenuItem) {
		t.Helper()
		point := p.rowFor(item).windowRect().Center()
		for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
			if err := p.popover.DispatchEvent(events.PointerEvent{EventType: kind, Position: point, Button: events.PointerButtonLeft}); err != nil {
				t.Fatal(err)
			}
		}
	}
	click(pm, branch)
	click(pm, branch)
	if pm.child == nil || !pm.child.Visible() || !pm.Visible() {
		t.Fatal("重复点击展开项关闭了子层")
	}
	click(pm.child, child.ItemAt(0))
	if calls != 1 || pm.Visible() {
		t.Fatal("原生层级的指针分发未整链执行一次")
	}
}

func TestMenuSubmenuPointerEnterHonorsStationaryBaseline(t *testing.T) {
	w, _, makePopup := transientFixture(t)
	q := newTimerTestQueue(t)
	w.app = q.app
	pm := NewPopoverMenu(makePopup(nil).anchor)
	t.Cleanup(pm.Destroy)
	child := NewMenu()
	child.Append("Leaf", nil)
	model := NewMenu()
	branch := model.AppendSubmenu("Branch", child)
	pm.SetMenu(model)
	if err := pm.ShowAt(geometry.Point{}); err != nil {
		t.Fatal(err)
	}
	point := pm.rowFor(branch).windowRect().Center()
	host := pm.nativeHost()
	ownerPoint := geometry.Point{X: point.X + host.requestedPosition.X, Y: point.Y + host.requestedPosition.Y}
	pm.Hide()
	w.cursorInside, w.cursorPosition = true, ownerPoint
	if err := pm.ShowAt(geometry.Point{}); err != nil {
		t.Fatal(err)
	}
	_ = pm.popover.DispatchEvent(events.PointerEvent{EventType: events.PointerEnter, Position: point})
	if pm.hoverTimer != nil && pm.hoverTimer.Active() {
		t.Fatal("静止指针的 Enter 启动了展开任务")
	}
	_ = pm.popover.DispatchEvent(events.PointerEvent{EventType: events.PointerLeave, Position: geometry.Point{X: -1, Y: -1}})
	point.X++
	_ = pm.popover.DispatchEvent(events.PointerEvent{EventType: events.PointerEnter, Position: point})
	if pm.hoverTimer == nil || !pm.hoverTimer.Active() {
		t.Fatal("真正移入菜单行后仍需额外移动才可展开")
	}
	q.advance(250 * time.Millisecond)
	q.dispatch(t)
	if pm.child == nil || !pm.child.Visible() {
		t.Fatal("首次真正 Enter 未按延迟展开")
	}
}

func TestMenuSubmenuBodyOverlap(t *testing.T) {
	for _, left := range []bool{false, true} {
		t.Run(fmt.Sprint(left), func(t *testing.T) {
			_, _, makePopup := transientFixture(t)
			pm := NewPopoverMenu(makePopup(nil).anchor)
			t.Cleanup(pm.Destroy)
			child := NewMenu()
			leaf := child.Append("Leaf", nil)
			model := NewMenu()
			branch := model.AppendSubmenu("Branch", child)
			pm.SetMenu(model)
			position := geometry.Point{}
			if left {
				position.X = 850
			}
			if err := pm.ShowAt(position); err != nil {
				t.Fatal(err)
			}
			pm.activate(branch)
			if pm.child == nil || !pm.child.Visible() {
				t.Fatal("分支未展开")
			}
			parent, descendant := pm.nativeHost(), pm.child.nativeHost()
			parentBody, childBody := parent.bodyRect(), descendant.bodyRect()
			parentBody.X += parent.requestedPosition.X
			childBody.X += descendant.requestedPosition.X
			overlap := parentBody.X + parentBody.Width - childBody.X
			if left {
				overlap = childBody.X + childBody.Width - parentBody.X
			}
			// 独立预期：菜单主体重叠 4 DIP，不以阴影边缘或被测常量作参考。
			if overlap != 4 {
				t.Fatal("父子主体重叠不等于 4 DIP", overlap)
			}
			parentY := pm.rowFor(branch).windowRect().Y + parent.requestedPosition.Y
			childY := pm.child.rowFor(leaf).windowRect().Y + descendant.requestedPosition.Y
			if parentY != childY {
				t.Fatal("水平重叠改变了首行的垂直对齐", parentY, childY)
			}
		})
	}
}

// 契约调整：延迟只改变展开路径，不能延迟当前项与语义反馈。
func TestMenuSubmenuDelaysBranchChangeWithImmediateSelection(t *testing.T) {
	for _, target := range []string{"leaf", "disabled", "padding", "branch", "empty"} {
		t.Run(target, func(t *testing.T) {
			w, plat, makePopup := transientFixture(t)
			q := newTimerTestQueue(t)
			w.app = q.app
			pm := NewPopoverMenu(makePopup(nil).anchor)
			t.Cleanup(pm.Destroy)
			child := NewMenu()
			child.Append("Leaf", nil)
			model := NewMenu()
			source := model.AppendSubmenu("Source", child)
			other := model.Append("Other", nil)
			disabled := model.Append("Disabled", nil)
			disabled.SetEnabled(false)
			next := model.AppendSubmenu("Next", child)
			empty := model.AppendSubmenu("Empty", NewMenu())
			pm.SetMenu(model)
			if err := pm.ShowAt(geometry.Point{}); err != nil {
				t.Fatal(err)
			}
			pm.activate(source)
			sourceRow := pm.rowFor(source)
			move := func(point geometry.Point) {
				t.Helper()
				if err := pm.popover.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: point}); err != nil {
					t.Fatal(err)
				}
			}
			move(sourceRow.windowRect().Center())
			var point geometry.Point
			var want *MenuItem
			switch target {
			case "leaf":
				want = other
			case "disabled":
				point = pm.rowFor(disabled).windowRect().Center()
			case "branch":
				want = next
			case "empty":
				want = empty
			case "padding":
				point = geometry.Point{X: sourceRow.windowRect().Center().X, Y: 1}
			}
			if want != nil {
				point = pm.rowFor(want).windowRect().Center()
			}
			move(point)
			if !pm.child.Visible() || pm.expanded != source || sourceRow.presentationState().Has(style.Hovered) || sourceRow.Snapshot().Selected {
				t.Fatal("关闭宽限未保留旧分支，或旧分支仍伪装成当前项")
			}
			if pm.content.active != want || pm.hoverTimer == nil || !pm.hoverTimer.Active() {
				t.Fatal("即时活动项或延迟任务不符", pm.content.active, want)
			}
			q.advance(249 * time.Millisecond)
			if len(q.posts) != 0 {
				q.dispatch(t)
			}
			if !pm.child.Visible() || pm.expanded != source {
				t.Fatal("旧分支未满 250 ms 就被关闭或替换")
			}
			move(point) // 同一候选中的重复输入不得重启计时。
			q.advance(time.Millisecond)
			q.dispatch(t)
			if target == "branch" {
				if !pm.child.Visible() || pm.expanded != next || len(plat.natives) != 2 {
					t.Fatal("宽限后未切换分支或未复用同层资源")
				}
			} else if pm.child.Visible() || pm.expanded != nil {
				t.Fatal("宽限结束后旧子层仍可见")
			}
			if pm.content.active != want {
				t.Fatal("分支变化覆盖了即时活动项")
			}
		})
	}
}

func TestMenuSubmenuTransitionHasOneDeadline(t *testing.T) {
	w, _, makePopup := transientFixture(t)
	q := newTimerTestQueue(t)
	w.app = q.app
	pm := NewPopoverMenu(makePopup(nil).anchor)
	t.Cleanup(pm.Destroy)
	child := NewMenu()
	for range 8 {
		child.Append("Leaf", nil)
	}
	model := NewMenu()
	branch := model.AppendSubmenu("Branch", child)
	first := model.Append("Other 1", nil)
	last := model.Append("Other 2", nil)
	pm.SetMenu(model)
	if err := pm.ShowAt(geometry.Point{}); err != nil {
		t.Fatal(err)
	}
	pm.activate(branch)
	move := func(point geometry.Point) {
		t.Helper()
		if err := pm.popover.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: point}); err != nil {
			t.Fatal(err)
		}
	}
	move(pm.rowFor(branch).windowRect().Center())
	// 右下斜移经过兄弟行，仍处于通向子菜单的三角区域。
	move(geometry.Point{X: 120, Y: 48})
	if !pm.child.Visible() || pm.content.active != first || pm.hoverTimer == nil || !pm.hoverTimer.Active() {
		t.Fatal("朝子菜单斜移时未保留分支或冻结了当前项")
	}
	q.advance(200 * time.Millisecond)
	move(geometry.Point{X: 130, Y: 76})
	if pm.content.active != last || pm.rowFor(branch).presentationState().Has(style.Hovered) {
		t.Fatal("跨行保护冻结了活动项或保留了源行主高亮")
	}
	q.advance(299 * time.Millisecond)
	if len(q.posts) != 0 {
		q.dispatch(t)
	}
	if !pm.child.Visible() || !pm.hoverTimer.Active() {
		t.Fatal("过渡保护提前结束")
	}
	q.advance(time.Millisecond)
	q.dispatch(t)
	if pm.child.Visible() || pm.expanded != nil || pm.content.active != last {
		t.Fatal("换行延长了原先的 500 ms 截止时间")
	}
}

func TestMenuSubmenuEnterCancelsTransition(t *testing.T) {
	for _, leaveFirst := range []bool{false, true} {
		t.Run(fmt.Sprint(leaveFirst), func(t *testing.T) {
			w, _, makePopup := transientFixture(t)
			q := newTimerTestQueue(t)
			w.app = q.app
			pm := NewPopoverMenu(makePopup(nil).anchor)
			t.Cleanup(pm.Destroy)
			child := NewMenu()
			for range 8 {
				child.Append("Leaf", nil)
			}
			model := NewMenu()
			branch := model.AppendSubmenu("Branch", child)
			model.Append("Other", nil)
			pm.SetMenu(model)
			if err := pm.ShowAt(geometry.Point{}); err != nil {
				t.Fatal(err)
			}
			pm.activate(branch)
			point := pm.rowFor(branch).windowRect().Center()
			_ = pm.popover.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: point})
			_ = pm.popover.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: geometry.Point{X: 120, Y: 48}})
			// 先把到期任务送入 Post 队列，再用真实 Enter 路径取消。
			q.advance(500 * time.Millisecond)
			leave := func() {
				_ = pm.popover.DispatchEvent(events.PointerEvent{EventType: events.PointerLeave, Position: point})
			}
			if leaveFirst {
				leave()
			}
			leafPoint := pm.child.rowFor(child.ItemAt(0)).windowRect().Center()
			_ = pm.child.popover.DispatchEvent(events.PointerEvent{EventType: events.PointerEnter, Position: leafPoint})
			if !leaveFirst {
				leave()
			}
			if !pm.child.Visible() || pm.rowFor(branch).presentationState().Has(style.Hovered) || pm.rowFor(branch).Snapshot().Selected {
				t.Fatal("父层 Leave 误关了子层或丢失展开路径提示")
			}
			if pm.hoverTimer.Active() {
				t.Fatal("进入子层后仍有旧过渡任务")
			}
			if len(q.posts) != 0 {
				q.dispatch(t)
			}
			if !pm.child.Visible() {
				t.Fatal("旧任务仍可关闭已进入的子菜单")
			}
		})
	}
}

func TestMenuSubmenuOwnerMotionPreservesPath(t *testing.T) {
	for _, modal := range []bool{false, true} {
		t.Run(fmt.Sprint(modal), func(t *testing.T) {
			w, _, makePopup := transientFixture(t)
			_ = w.DispatchEvent(events.SizeEvent{Width: 1000, Height: 800, PixelWidth: 1000, PixelHeight: 800})
			pm := NewPopoverMenu(makePopup(nil).anchor)
			pm.SetModal(modal)
			t.Cleanup(pm.Destroy)
			child := NewMenu()
			child.Append("Leaf", nil)
			model := NewMenu()
			branch := model.AppendSubmenu("Branch", child)
			pm.SetMenu(model)
			if err := pm.ShowAt(geometry.Point{}); err != nil {
				t.Fatal(err)
			}
			pm.activate(branch)
			_ = pm.popover.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: pm.rowFor(branch).windowRect().Center()})
			outside := events.PointerEvent{EventType: events.PointerMove, Position: geometry.Point{X: 500, Y: 500}}
			_ = w.DispatchEvent(outside)
			if !pm.Visible() || !pm.child.Visible() || pm.expanded != branch || pm.content.active != nil || pm.child.content.active != nil {
				t.Fatal("链外移动关闭了展开路径，或留下了旧活动项")
			}
			pm.content.selectItem(branch)
			_ = pm.popover.DispatchEvent(shortcutPress(events.KeyArrowRight, 0))
			_ = w.DispatchEvent(outside)
			if !pm.child.Visible() {
				t.Fatal("静止坐标的重复移动关闭了键盘展开的子菜单")
			}
		})
	}
}

func TestMenuSubmenuDelayedChangePreservesReentrantBranch(t *testing.T) {
	for _, replacing := range []bool{false, true} {
		t.Run(fmt.Sprint(replacing), func(t *testing.T) {
			w, _, makePopup := transientFixture(t)
			q := newTimerTestQueue(t)
			w.app = q.app
			pm := NewPopoverMenu(makePopup(nil).anchor)
			t.Cleanup(pm.Destroy)
			child := NewMenu()
			child.Append("Leaf", nil)
			model := NewMenu()
			a := model.AppendSubmenu("A", child)
			b := model.AppendSubmenu("B", child)
			target := model.Append("Other", nil)
			if replacing {
				target = model.AppendSubmenu("C", child)
			}
			pm.SetMenu(model)
			if err := pm.ShowAt(geometry.Point{}); err != nil {
				t.Fatal(err)
			}
			pm.activate(a)
			previous := w.Popovers()[1]
			previous.ConnectClosed(func() { pm.activate(b) })
			_ = pm.popover.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: pm.rowFor(target).windowRect().Center()})
			q.advance(250 * time.Millisecond)
			q.dispatch(t)
			if !pm.Visible() || !pm.child.Visible() || pm.expanded != b || previous.Visible() {
				t.Fatal("旧延迟切换覆盖了关闭回调新建的分支")
			}
		})
	}
}

func TestMenuSubmenuTransitionStartsOnSourceRow(t *testing.T) {
	w, _, makePopup := transientFixture(t)
	q := newTimerTestQueue(t)
	w.app = q.app
	pm := NewPopoverMenu(makePopup(nil).anchor)
	t.Cleanup(pm.Destroy)
	child := NewMenu()
	for range 8 {
		child.Append("Leaf", nil)
	}
	model := NewMenu()
	other := model.Append("Other", nil)
	model.Append("Middle", nil)
	model.AppendSubmenu("Branch", child)
	w.cursorInside, w.cursorPosition = true, geometry.Point{X: 80, Y: 40}
	pm.SetMenu(model)
	if err := pm.ShowAt(geometry.Point{}); err != nil {
		t.Fatal(err)
	}
	_ = pm.popover.DispatchEvent(shortcutPress(events.KeyEnd, 0))
	_ = pm.popover.DispatchEvent(shortcutPress(events.KeyArrowRight, 0))
	_ = pm.popover.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: pm.rowFor(other).windowRect().Center()})
	if !pm.child.Visible() || !pm.protectUntil.IsZero() || pm.content.active != other {
		t.Fatal("旧坐标不在源行时仍启动了斜移保护，或未保留关闭宽限")
	}
	q.advance(250 * time.Millisecond)
	q.dispatch(t)
	if pm.child.Visible() {
		t.Fatal("没有斜移保护时关闭宽限未按 250 ms 结束")
	}
}

func TestMenuSubmenuOwnerMotionPreservesCapture(t *testing.T) {
	w, _, makePopup := transientFixture(t)
	pm := NewPopoverMenu(makePopup(nil).anchor)
	t.Cleanup(pm.Destroy)
	child := NewMenu()
	leaf := child.Append("Leaf", nil)
	model := NewMenu()
	branch := model.AppendSubmenu("Branch", child)
	pm.SetMenu(model)
	if err := pm.ShowAt(geometry.Point{}); err != nil {
		t.Fatal(err)
	}
	pm.activate(branch)
	row := pm.child.rowFor(leaf)
	capture := &gestureProbe{EventControllerBase: NewEventControllerBase(PhaseCapture)}
	row.AddEventController(capture)
	point := row.windowRect().Center()
	_ = pm.child.popover.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Position: point, Button: events.PointerButtonLeft, Buttons: events.PointerButtonLeftDown})
	if pm.child.nativeHost().dispatcher.captureTarget == nil {
		t.Fatal("前提失败：子项未取得捕获")
	}
	_ = w.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: geometry.Point{X: 500, Y: 500}, Buttons: events.PointerButtonLeftDown})
	if !pm.child.Visible() || pm.child.nativeHost().dispatcher.captureTarget == nil {
		t.Fatal("宿主移动越过了子项的既有捕获")
	}
	_ = pm.child.popover.DispatchEvent(events.PointerEvent{EventType: events.PointerUp, Position: point, Button: events.PointerButtonLeft})
	if !pm.child.Visible() || pm.child.nativeHost().dispatcher.captureTarget != nil || capture.accepted != 1 || capture.canceled != 0 {
		t.Fatal("捕获释放未正常完成原有手势")
	}
}

func TestMenuSubmenuKeyboardReturnKeepsSource(t *testing.T) {
	for _, key := range []events.Key{events.KeyArrowLeft, events.KeyEscape} {
		t.Run(fmt.Sprint(key), func(t *testing.T) {
			_, _, makePopup := transientFixture(t)
			pm := NewPopoverMenu(makePopup(nil).anchor)
			t.Cleanup(pm.Destroy)
			child := NewMenu()
			child.Append("Leaf", nil)
			model := NewMenu()
			branch := model.AppendSubmenu("Branch", child)
			model.Append("Other", nil)
			pm.SetMenu(model)
			if err := pm.ShowAt(geometry.Point{}); err != nil {
				t.Fatal(err)
			}
			_ = pm.popover.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: pm.rowFor(branch).windowRect().Center()})
			_ = pm.popover.DispatchEvent(shortcutPress(events.KeyHome, 0))
			_ = pm.popover.DispatchEvent(shortcutPress(events.KeyArrowRight, 0))
			_ = pm.popover.DispatchEvent(events.PointerEvent{EventType: events.PointerLeave, Position: pm.rowFor(branch).windowRect().Center()})
			if pm.content.active != nil {
				t.Fatal("父表面离开后仍保留鼠标活动项")
			}
			_ = pm.child.popover.DispatchEvent(events.PointerEvent{EventType: events.PointerEnter, Position: pm.child.rowFor(child.ItemAt(0)).windowRect().Center()})
			_ = pm.popover.DispatchEvent(shortcutPress(key, 0))
			if !pm.Visible() || pm.child.Visible() || pm.content.active != branch || !pm.rowFor(branch).Snapshot().Selected {
				t.Fatal("键盘返回父层后丢失源项选中位置")
			}
		})
	}
}

func TestMenuSubmenuLeaveCancelsQueuedChange(t *testing.T) {
	for _, destination := range []string{"owner", "popup-outside", "native-leave", "source"} {
		t.Run(destination, func(t *testing.T) {
			w, _, makePopup := transientFixture(t)
			q := newTimerTestQueue(t)
			w.app = q.app
			pm := NewPopoverMenu(makePopup(nil).anchor)
			t.Cleanup(pm.Destroy)
			grandchild := NewMenu()
			grandchild.Append("Leaf", nil)
			child := NewMenu()
			more := child.AppendSubmenu("More", grandchild)
			model := NewMenu()
			branch := model.AppendSubmenu("Branch", child)
			other := model.Append("Other", nil)
			pm.SetMenu(model)
			if err := pm.ShowAt(geometry.Point{}); err != nil {
				t.Fatal(err)
			}
			pm.activate(branch)
			pm.child.activate(more)
			move := func(kind events.EventType, point geometry.Point) {
				t.Helper()
				if err := pm.popover.DispatchEvent(events.PointerEvent{EventType: kind, Position: point}); err != nil {
					t.Fatal(err)
				}
			}
			move(events.PointerMove, pm.rowFor(other).windowRect().Center())
			q.advance(250 * time.Millisecond) // 已排队的旧任务也必须取消。
			switch destination {
			case "owner":
				_ = w.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: geometry.Point{X: 800, Y: 600}})
			case "popup-outside":
				move(events.PointerMove, geometry.Point{X: -20, Y: -20})
			case "native-leave":
				move(events.PointerLeave, pm.rowFor(other).windowRect().Center())
			case "source":
				move(events.PointerMove, pm.rowFor(branch).windowRect().Center())
			}
			q.dispatch(t)
			if !pm.Visible() || !pm.child.Visible() || !pm.child.child.Visible() || pm.expanded != branch || pm.child.expanded != more {
				t.Fatal("离开菜单或回到源行后，旧任务仍改变了展开路径")
			}
			for layer := pm; layer != nil; layer = layer.child {
				if layer.hoverTimer != nil && layer.hoverTimer.Active() || !layer.protectUntil.IsZero() {
					t.Fatal("离开或返回后仍有待执行任务")
				}
				want := (*MenuItem)(nil)
				if destination == "source" && layer == pm {
					want = branch
				}
				if layer.content.active != want {
					t.Fatal("非命中层仍有活动项", layer.content.active, want)
				}
			}
			// 返回子菜单后，继续走原输入路径执行叶项且只执行一次。
			calls := 0
			leaf := grandchild.ItemAt(0)
			leaf.SetAction(func() { calls++ })
			point := pm.child.child.rowFor(leaf).windowRect().Center()
			_ = pm.child.child.popover.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: point})
			_ = pm.popover.DispatchEvent(shortcutPress(events.KeyEnter, 0))
			if pm.Visible() || calls != 1 {
				t.Fatal("离开后返回的菜单无法正常执行", calls)
			}
		})
	}
}

func TestMenuSubmenuAbandonedProtectionKeepsDeadline(t *testing.T) {
	for _, expired := range []bool{false, true} {
		t.Run(fmt.Sprint(expired), func(t *testing.T) {
			w, _, makePopup := transientFixture(t)
			q := newTimerTestQueue(t)
			w.app = q.app
			pm := NewPopoverMenu(makePopup(nil).anchor)
			t.Cleanup(pm.Destroy)
			child := NewMenu()
			for range 8 {
				child.Append("Leaf", nil)
			}
			model := NewMenu()
			branch := model.AppendSubmenu("Branch", child)
			model.Append("Other 1", nil)
			last := model.Append("Other 2", nil)
			pm.SetMenu(model)
			if err := pm.ShowAt(geometry.Point{}); err != nil {
				t.Fatal(err)
			}
			pm.activate(branch)
			move := func(point geometry.Point) {
				t.Helper()
				if err := pm.popover.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: point}); err != nil {
					t.Fatal(err)
				}
			}
			move(pm.rowFor(branch).windowRect().Center())
			move(geometry.Point{X: 120, Y: 48})
			if pm.protectUntil.IsZero() {
				t.Fatal("前提失败：没有进入斜移保护")
			}
			elapsed := 400 * time.Millisecond
			if expired {
				elapsed = 501 * time.Millisecond
			}
			q.advance(elapsed)
			// 改为远离子菜单；不得在旧截止时间之后另加 250 ms。
			move(geometry.Point{X: 20, Y: 76})
			if !expired {
				q.advance(99 * time.Millisecond)
				if !pm.child.Visible() {
					t.Fatal("恢复宽限时提前关闭")
				}
				q.advance(time.Millisecond)
			}
			q.dispatch(t)
			if pm.child.Visible() || pm.content.active != last {
				t.Fatal("放弃斜移后超过了原保护期限或丢失活动项")
			}
		})
	}
}

func TestMenuSubmenuExpandedPartIsIndependentOfSelection(t *testing.T) {
	_, _, makePopup := transientFixture(t)
	normal, active, path := color.RGBA{R: 240, A: 255}, color.RGBA{B: 220, A: 255}, color.RGBA{G: 120, A: 255}
	App.(*application).style = style.Sheet(
		style.Name(styleNameMenuItem).BackgroundColor(normal),
		style.Name(styleNameMenuItem).State(style.Hovered).BackgroundColor(active),
		style.Name(styleNameMenuItem).Part("expanded").BackgroundColor(path).BorderWidth(2).Radius(3),
	)
	pm := NewPopoverMenu(makePopup(nil).anchor)
	t.Cleanup(pm.Destroy)
	child := NewMenu()
	child.Append("Leaf", nil)
	model := NewMenu()
	branch := model.AppendSubmenu("Branch", child)
	other := model.Append("Other", nil)
	pm.SetMenu(model)
	if err := pm.ShowAt(geometry.Point{}); err != nil {
		t.Fatal(err)
	}
	pm.activate(branch)
	row := pm.rowFor(branch)
	_ = pm.popover.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: pm.rowFor(other).windowRect().Center()})
	background, _ := row.resolvedStyle().BackgroundColor()
	width, _ := row.resolvedStyle().BorderWidth()
	radius, _ := row.resolvedStyle().CornerRadius()
	info := row.Snapshot()
	if background != path || width != 2 || radius != geometry.UniformCornerRadius(3) || info.Selected || row.presentationState().Has(style.Hovered) || !info.Attributes[MenuItemInfoKey].(MenuItemInfo).Expanded {
		t.Fatal("展开 Part 未独立生效或污染了即时选中语义", background, info)
	}
	_ = pm.popover.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: row.windowRect().Center()})
	background, _ = row.resolvedStyle().BackgroundColor()
	if background != active || !row.Snapshot().Selected {
		t.Fatal("活动项没有优先采用主要高亮")
	}
	pm.closeChild()
	pm.content.selectItem(nil)
	background, _ = row.resolvedStyle().BackgroundColor()
	if background != normal || row.Snapshot().Attributes[MenuItemInfoKey].(MenuItemInfo).Expanded {
		t.Fatal("关闭后遗留展开背景或语义")
	}
}

func TestMenuSubmenuPressDoesNotWaitForHover(t *testing.T) {
	for _, branchTarget := range []bool{false, true} {
		t.Run(fmt.Sprint(branchTarget), func(t *testing.T) {
			w, _, makePopup := transientFixture(t)
			q := newTimerTestQueue(t)
			w.app = q.app
			pm := NewPopoverMenu(makePopup(nil).anchor)
			t.Cleanup(pm.Destroy)
			calls := 0
			child := NewMenu()
			child.Append("Leaf", nil)
			model := NewMenu()
			source := model.AppendSubmenu("Source", child)
			target := model.Append("Other", func() { calls++ })
			if branchTarget {
				target.SetSubmenu(child)
			}
			pm.SetMenu(model)
			if err := pm.ShowAt(geometry.Point{}); err != nil {
				t.Fatal(err)
			}
			pm.activate(source)
			point := pm.rowFor(target).windowRect().Center()
			for _, kind := range []events.EventType{events.PointerMove, events.PointerDown, events.PointerUp} {
				if err := pm.popover.DispatchEvent(events.PointerEvent{EventType: kind, Position: point, Button: events.PointerButtonLeft}); err != nil {
					t.Fatal(err)
				}
			}
			if branchTarget {
				if !pm.child.Visible() || pm.expanded != target {
					t.Fatal("点击分支仍等待悬停宽限")
				}
			} else if pm.Visible() || calls != 1 {
				t.Fatal("点击普通项没有立即关闭并执行一次", calls)
			}
			if pm.hoverTimer.Active() {
				t.Fatal("明确点击后遗留旧悬停任务")
			}
		})
	}
}

func TestMenuSubmenuOutsideCrossingPreservesReplacement(t *testing.T) {
	w, _, makePopup := transientFixture(t)
	pm := NewPopoverMenu(makePopup(nil).anchor)
	t.Cleanup(pm.Destroy)
	child := NewMenu()
	leaf := child.Append("Leaf", nil)
	model := NewMenu()
	a := model.AppendSubmenu("A", child)
	b := model.AppendSubmenu("B", child)
	pm.SetMenu(model)
	if err := pm.ShowAt(geometry.Point{}); err != nil {
		t.Fatal(err)
	}
	pm.activate(a)
	motion := NewMotionEventController()
	pm.content.AddEventController(motion)
	_ = pm.popover.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: pm.rowFor(a).windowRect().Center()})
	motion.ConnectContainsHover(func(hovered bool) {
		if !hovered {
			pm.openChild(b, true)
		}
	})
	_ = w.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: geometry.Point{X: 800, Y: 600}})
	if !pm.child.Visible() || pm.expanded != b || pm.child.content.active != leaf {
		t.Fatal("链外旧事件清除了 crossing 回调新展开的菜单状态")
	}
}
