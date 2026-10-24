package gui

import (
	"image/color"
	"testing"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/style"
)

// 使用真实 GUI 事件分发及可推进的计时器，不需要窗口系统或 sleep。
func tooltipFixture(t *testing.T) (*window, *testWidget, *testWidget, *testWidget, *timerTestQueue, *placementPlatform) {
	t.Helper()
	q := newTimerTestQueue(t)
	plat := &placementPlatform{syncSize: true}
	app := &application{platform: plat, timers: q.s}
	useTestApplication(t, app)
	w := &window{platformWindow: &placementWindow{area: geometry.Rect(0, 0, 1000, 800)}, focused: true}
	w.app, w.width, w.height = app, 400, 300
	app.windows = []*window{w}
	root, a, b := newTestWidget(), newTestWidget(), newTestWidget()
	root.AddChild(a)
	root.AddChild(b)
	w.SetWidget(root)
	root.Arrange(geometry.Rect(0, 0, 400, 300))
	a.Arrange(geometry.Rect(20, 20, 100, 40))
	b.Arrange(geometry.Rect(150, 20, 100, 40))
	a.SetTooltip("first")
	b.SetTooltip("second")
	t.Cleanup(w.Destroy)
	return w, root, a, b, q, plat
}

func tooltipMove(t *testing.T, w *window, x, y float32) {
	t.Helper()
	if err := w.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: geometry.Point{X: x, Y: y}}); err != nil {
		t.Fatal(err)
	}
}

func tooltipAdvance(t *testing.T, q *timerTestQueue, delay time.Duration) {
	t.Helper()
	q.advance(delay)
	for len(q.posts) > 0 {
		q.dispatch(t)
	}
}

func TestTooltipTextIsIndependentAndDetached(t *testing.T) {
	w := newTestWidget()
	w.SetName("name")
	w.SetTooltip("help\nsecond line")
	if w.Tooltip() != "help\nsecond line" || w.Snapshot().Tooltip != w.Tooltip() || w.Name() != "name" || len(w.EventControllers()) != 0 {
		t.Fatal("提示改变了名称、语义或创建了控制器")
	}
	w.SetTooltip("")
	if w.Tooltip() != "" || w.Snapshot().Tooltip != "" {
		t.Fatal("空字符串没有移除提示")
	}
}

func TestTooltipDelayJitterSwitchAndCleanup(t *testing.T) {
	w, root, a, b, q, _ := tooltipFixture(t)
	tooltipMove(t, w, 40, 40)
	if w.tooltip.target != a || w.tooltip.timer == nil || len(w.Popovers()) != 0 {
		t.Fatal("悬停没有等待或过早创建浮层")
	}
	tooltipAdvance(t, q, 300*time.Millisecond)
	tooltipMove(t, w, 42, 40)
	tooltipAdvance(t, q, 399*time.Millisecond)
	if len(w.Popovers()) != 0 {
		t.Fatal("初始等待不足 700ms")
	}
	tooltipAdvance(t, q, time.Millisecond)
	if len(w.Popovers()) != 1 || w.tooltip.popup == nil || w.tooltip.popup.Modal() || w.FocusedWidget() != nil {
		t.Fatal("提示没有显示或抢走了焦点／键盘")
	}
	first := w.tooltip.popup
	tooltipMove(t, w, 160, 40)
	tooltipAdvance(t, q, 699*time.Millisecond)
	if len(w.Popovers()) != 0 || !first.destroyed || w.tooltip.target != b {
		t.Fatal("切换目标没有释放旧提示或跳过了等待")
	}
	tooltipAdvance(t, q, time.Millisecond)
	if len(w.Popovers()) != 1 || w.tooltip.text != "second" {
		t.Fatal("切换目标等待 700ms 后没有显示提示")
	}
	tooltipMove(t, w, 300, 200)
	tooltipMove(t, w, 160, 40)
	tooltipAdvance(t, q, 699*time.Millisecond)
	if len(w.Popovers()) != 0 {
		t.Fatal("重进原目标没有完整等待 700ms")
	}
	tooltipAdvance(t, q, time.Millisecond)
	if len(w.Popovers()) != 1 {
		t.Fatal("重进原目标等待 700ms 后没有显示提示")
	}
	root.RemoveChild(b)
	if len(w.Popovers()) != 0 || w.tooltip.target != nil || w.tooltip.timer.Active() {
		t.Fatal("卸载没有清除提示和计时")
	}
}

func TestTooltipMovementAndQueuedCancellation(t *testing.T) {
	w, _, a, _, q, plat := tooltipFixture(t)
	tooltipMove(t, w, 40, 40)
	tooltipAdvance(t, q, 400*time.Millisecond)
	tooltipMove(t, w, 50, 40)
	tooltipAdvance(t, q, 100*time.Millisecond)
	if len(w.Popovers()) != 0 {
		t.Fatal("明显移动没有重新等待")
	}
	q.advance(600 * time.Millisecond) // 通知已投递，尚未在 GUI 线程执行。
	a.SetVisible(false)
	for len(q.posts) > 0 {
		q.dispatch(t)
	}
	if len(w.Popovers()) != 0 || len(plat.created) != 0 || w.tooltip.target != nil {
		t.Fatal("取消后过期的投递仍创建原生提示")
	}
}

// 按钮本身与其 Label 子控件共享同一提示目标，四周留白也必须参与命中。
func TestTooltipButtonPaddingAndContent(t *testing.T) {
	w, _, _, _, q, _ := tooltipFixture(t)
	button := NewButton()
	label := NewLabel("content")
	button.SetChild(label)
	button.SetTooltip("button help")
	w.SetWidget(button)
	button.Arrange(geometry.Rect(20, 20, 180, 60))
	label.Arrange(geometry.Rect(30, 16, 120, 28))
	for _, point := range []geometry.Point{{X: 2, Y: 30}, {X: 178, Y: 30}, {X: 90, Y: 2}, {X: 90, Y: 58}, {X: 90, Y: 30}} {
		tooltipMove(t, w, 300, 200)
		tooltipMove(t, w, 20+point.X, 20+point.Y)
		tooltipAdvance(t, q, 699*time.Millisecond)
		if !button.hovered || w.tooltip.target != button || len(w.Popovers()) != 0 {
			t.Fatalf("位置 %v 没有命中按钮、丢失 Hover 或过早显示提示", point)
		}
		tooltipAdvance(t, q, time.Millisecond)
		p := w.tooltip.popup
		tooltipAdvance(t, q, 2*time.Second)
		if p == nil || w.tooltip.popup != p || !p.Visible() || !button.hovered {
			t.Fatalf("位置 %v 的提示或 Hover 没有保持稳定", point)
		}
	}
}

func TestTooltipDisabledAncestorAndClickPropagation(t *testing.T) {
	w, _, a, _, q, _ := tooltipFixture(t)
	icon := newTestWidget()
	a.AddChild(icon)
	icon.Arrange(geometry.Rect(0, 0, 50, 40))
	a.SetEnabled(false)
	tooltipMove(t, w, 40, 40)
	tooltipAdvance(t, q, tooltipDelay)
	if len(w.Popovers()) != 1 || w.tooltip.target != a || a.Enabled() || icon.Tooltip() != "" {
		t.Fatal("禁用控件或图标没有找到父级说明")
	}
	a.SetEnabled(true)
	clicks := 0
	c := NewClickEventController()
	c.ConnectClicked(func(EventContext) { clicks++ })
	a.AddEventController(c)
	for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
		if err := w.DispatchEvent(events.PointerEvent{EventType: kind, Position: geometry.Point{X: 80, Y: 40}, Button: events.PointerButtonLeft}); err != nil {
			t.Fatal(err)
		}
	}
	if clicks != 1 || len(w.Popovers()) != 0 {
		t.Fatal("提示关闭吞掉了原控件的点击")
	}
}

func TestTooltipEscapeFocusAndWindowLifetime(t *testing.T) {
	w, _, a, b, q, _ := tooltipFixture(t)
	a.SetFocusable(true)
	b.SetFocusable(true)
	if err := w.DispatchEvent(shortcutPress(events.KeyTab, 0)); err != nil {
		t.Fatal(err)
	}
	tooltipAdvance(t, q, tooltipDelay)
	if w.FocusedWidget() != a || !a.FocusVisible() || len(w.Popovers()) != 1 {
		t.Fatal("Tab 焦点提示没有显示")
	}
	_ = w.DispatchEvent(shortcutPress(events.KeyEscape, 0))
	tooltipAdvance(t, q, time.Second)
	if len(w.Popovers()) != 0 || w.FocusedWidget() != a {
		t.Fatal("Escape 后提示重开或改变了焦点")
	}
	_ = w.DispatchEvent(shortcutPress(events.KeyTab, 0))
	tooltipAdvance(t, q, tooltipDelay)
	if w.FocusedWidget() != b || len(w.Popovers()) != 1 {
		t.Fatal("下一个键盘目标没有显示提示")
	}
	_ = w.DispatchEvent(events.FocusEvent{Focused: false})
	if len(w.Popovers()) != 0 || w.tooltip.timer.Active() {
		t.Fatal("窗口失焦没有清理提示")
	}
	_ = w.DispatchEvent(events.FocusEvent{Focused: true})
	tooltipMove(t, w, 40, 40)
	w.Destroy()
	tooltipAdvance(t, q, time.Second)
	if len(w.Popovers()) != 0 || w.tooltip.target != nil || w.tooltip.timer.Active() {
		t.Fatal("销毁后仍保留任务或提示")
	}
}

func TestTooltipTextScopeAndClipping(t *testing.T) {
	w, root, a, _, q, _ := tooltipFixture(t)
	scope := NewStyleScope()
	scope.SetStyleSheet(style.Sheet(style.Name("tooltip").BackgroundColor(color.RGBA{R: 21, A: 255})))
	root.RemoveChild(a)
	scope.SetChild(a)
	root.AddChild(scope)
	scope.Arrange(geometry.Rect(20, 20, 50, 40))
	a.Arrange(geometry.Rect(-20, 0, 100, 40))
	tooltipMove(t, w, 40, 40)
	tooltipAdvance(t, q, tooltipDelay)
	p := w.tooltip.popup
	if p == nil || w.tooltip.bounds != geometry.Rect(20, 20, 50, 40) {
		t.Fatal("提示没有使用目标的可见区域")
	}
	bg, _ := p.resolvedStyle().BackgroundColor()
	if bg != (color.RGBA{R: 21, A: 255}) {
		t.Fatal("提示没有使用目标的样式作用域")
	}
	a.SetTooltip("updated")
	if !p.destroyed || len(w.Popovers()) != 0 {
		t.Fatal("文本更新保留了旧浮层")
	}
	tooltipAdvance(t, q, tooltipDelay)
	if w.tooltip.popup == nil || w.tooltip.text != "updated" {
		t.Fatal("更新后的提示没有显示")
	}
	scope.SetVisible(false)
	if len(w.Popovers()) != 0 {
		t.Fatal("隐藏父级没有关闭提示")
	}
}

func TestTooltipReentrantPreparationAndFailedCreation(t *testing.T) {
	t.Run("reentrant", func(t *testing.T) {
		w, _, a, _, q, plat := tooltipFixture(t)
		native := w.platformWindow.(*placementWindow)
		native.onQuery = func() {
			native.onQuery = nil
			a.SetTooltip("")
		}
		tooltipMove(t, w, 40, 40)
		tooltipAdvance(t, q, tooltipDelay)
		if len(w.Popovers()) != 0 || w.tooltip.target != nil || len(plat.created) != 0 {
			t.Fatal("工作区查询回调取消后仍创建了提示")
		}
	})
	for _, unavailable := range []bool{false, true} {
		t.Run(map[bool]string{false: "opaque-fallback", true: "failure"}[unavailable], func(t *testing.T) {
			w, _, _, _, q, plat := tooltipFixture(t)
			plat.transparentErr = platform.ErrUnsupported
			if unavailable {
				plat.transparentErr = platform.ErrUnavailable
			}
			tooltipMove(t, w, 40, 40)
			tooltipAdvance(t, q, tooltipDelay)
			if unavailable {
				if len(w.Popovers()) != 0 || w.tooltip.target != nil || w.tooltip.timer.Active() || len(plat.options) != 1 {
					t.Fatal("失败没有释放状态或错误地重复创建")
				}
			} else if len(w.Popovers()) != 1 || w.tooltip.popup.Transparent() || len(plat.options) != 2 {
				t.Fatal("不支持 alpha 时没有明确降级为不透明提示")
			}
		})
	}
}

func TestTooltipInsideModalPopoverUsesOwnerCoordinates(t *testing.T) {
	w, _, a, _, q, _ := tooltipFixture(t)
	parent := NewPopover(a, nil).(*popover)
	t.Cleanup(parent.Destroy)
	content, child := newTestWidget(), newTestWidget()
	content.SetMinSize(geometry.Size{Width: 120, Height: 60})
	content.AddChild(child)
	child.Arrange(geometry.Rect(10, 10, 40, 20))
	child.SetTooltip("inside")
	parent.SetWidget(content)
	parent.SetPosition(geometry.Point{X: 200, Y: 80})
	parent.SetModal(true)
	if err := parent.Show(); err != nil {
		t.Fatal(err)
	}
	settlePlacement(t, parent)
	if err := parent.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: geometry.Point{X: 20, Y: 20}}); err != nil {
		t.Fatal(err)
	}
	tooltipAdvance(t, q, tooltipDelay)
	tip := w.tooltip.popup
	if tip == nil || !tip.Visible() || tip.owner != w || tip.placementSnapshot.anchor != geometry.Rect(230, 110, 40, 20) {
		t.Fatalf("嵌套提示没有转换为原生所有者坐标: %v", tip)
	}
	parent.Hide()
	if w.tooltip.target != nil || !tip.destroyed || len(w.Popovers()) != 0 {
		t.Fatal("父浮层关闭后仍保留提示")
	}
}

func TestTooltipCloseCallbacksPreserveNewModalTarget(t *testing.T) {
	w, _, a, _, q, _ := tooltipFixture(t)
	tooltipMove(t, w, 40, 40)
	tooltipAdvance(t, q, tooltipDelay)
	outer, replacement := NewPopover(a, nil).(*popover), NewPopover(a, nil).(*popover)
	t.Cleanup(outer.Destroy)
	t.Cleanup(replacement.Destroy)
	w.tooltip.popup.ConnectClosed(func() { w.SetModalTarget(replacement) })
	w.SetModalTarget(outer)
	if w.modalTarget != replacement {
		t.Fatal("关闭提示的回调设置的新模态目标被旧操作覆盖")
	}
	w.SetModalTarget(nil)
	tooltipMove(t, w, 160, 40)
	tooltipAdvance(t, q, tooltipDelay)
	w.tooltip.popup.ConnectClosed(w.Destroy)
	_ = w.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Position: geometry.Point{X: 160, Y: 40}, Button: events.PointerButtonLeft})
	if !w.destroyed || w.tooltip.target != nil {
		t.Fatal("事件取消提示时销毁窗口后仍继续旧操作")
	}
}

// Windows 创建原生 Popup 时会同步报告初始 Hidden；它不是锚点窗口隐藏。
type tooltipInitialStatePlatform struct{ *placementPlatform }

func (p tooltipInitialStatePlatform) NewPopup(owner platform.Window, size geometry.Size, handler platform.EventHandler, options platform.PopupOptions) (platform.Popup, error) {
	handler(events.StateEvent{State: WindowStateHidden})
	handler(events.FocusEvent{Focused: false})
	return p.placementPlatform.NewPopup(owner, size, handler, options)
}

func TestTooltipCreationIgnoresItsOwnNativeState(t *testing.T) {
	w, _, _, _, q, plat := tooltipFixture(t)
	w.app.platform = tooltipInitialStatePlatform{plat}
	tooltipMove(t, w, 40, 40)
	tooltipAdvance(t, q, tooltipDelay)
	if len(w.Popovers()) != 1 || w.tooltip.popup == nil || !w.tooltip.popup.Visible() || !w.focused {
		t.Fatal("提示表面自己的初始状态取消了锚点会话")
	}
}

func TestTooltipStationaryPointerKeepsKeyboardHelp(t *testing.T) {
	w, _, a, b, q, _ := tooltipFixture(t)
	a.SetFocusable(true)
	tooltipMove(t, w, 160, 40)
	_ = w.DispatchEvent(shortcutPress(events.KeyTab, 0))
	tooltipAdvance(t, q, tooltipDelay)
	if w.tooltip.target != a || !w.tooltip.keyboard || w.tooltip.popup == nil {
		t.Fatal("键盘提示场景没有建立")
	}
	p := w.tooltip.popup
	tooltipMove(t, w, 160, 40) // 原生窗口显示可重新报告没有改变的真实鼠标位置。
	if w.tooltip.popup != p || w.tooltip.target != a || !w.tooltip.keyboard {
		t.Fatal("静止鼠标抢走了键盘提示")
	}
	tooltipMove(t, w, 161, 40)
	if w.tooltip.target != b || w.tooltip.keyboard || !p.destroyed {
		t.Fatal("真实移动没有切换为鼠标提示")
	}
}
