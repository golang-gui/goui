package gui

import (
	"errors"
	"image/color"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
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
	owner := &window{}
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
		win := &window{}
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
	platform.transparentErr = errors.New("native popup failure")
	errorsSeen := 0
	b[1].ConnectOpenError(func(error) { errorsSeen++ })
	menuButtonPointer(t, w, events.PointerMove, 150)
	if errorsSeen != 1 || b[0].Opened() || b[1].Opened() || w.modalTarget != nil || len(w.Popovers()) != 0 {
		t.Fatal("打开失败没有清理或重复报告", errorsSeen)
	}
	if err := b[1].Open(); err == nil || errorsSeen != 1 {
		t.Fatal("同步失败没有返回错误，或重复发信号")
	}
	platform.transparentErr = nil
	if err := b[0].Open(); err != nil {
		t.Fatal(err)
	}
	other := &window{platformWindow: &placementWindow{area: geometry.Rect(0, 0, 1000, 800)}}
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
