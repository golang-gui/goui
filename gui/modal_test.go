package gui

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/dragdrop"
	"github.com/golang-gui/goui/platform/events"
)

// 使用真实 Window 分发但不创建原生窗口，隔离验证焦点、传播和生命周期。
func modalFixture(t *testing.T) (*window, *testWidget, *testWidget, *testWidget, *testWidget, *testWidget, *ModalScope) {
	t.Helper()
	root, back, layer, a, b := newTestWidget(), newTestWidget(), newTestWidget(), newTestWidget(), newTestWidget()
	root.Arrange(geometry.Rect(0, 0, 300, 180))
	back.Arrange(geometry.Rect(0, 0, 80, 80))
	layer.Arrange(geometry.Rect(100, 0, 160, 150))
	a.Arrange(geometry.Rect(0, 0, 40, 40))
	b.Arrange(geometry.Rect(60, 0, 40, 40))
	back.SetFocusable(true)
	a.SetFocusable(true)
	b.SetFocusable(true)
	layer.AddChild(a)
	layer.AddChild(b)
	layer.SetVisible(false)
	root.AddChild(back)
	root.AddChild(layer)
	w := &window{focused: true}
	w.SetWidget(root)
	w.SetFocusedWidget(back)
	s := NewModalScope(layer)
	t.Cleanup(w.Destroy)
	return w, root, back, layer, a, b, s
}

func TestModalShortcutsAllPhasesAndPointerOcclusion(t *testing.T) {
	for _, phase := range []PropagationPhase{PhaseCapture, PhaseTarget, PhaseBubble} {
		t.Run(string(rune('0'+phase)), func(t *testing.T) {
			w, root, _, layer, a, _, _ := modalFixture(t)
			calls := 0
			c := NewShortcutController()
			c.SetPhase(phase)
			s := NewShortcut(KeyGesture{Key: KeyF5})
			s.ConnectActivate(func() { calls++ })
			c.AddShortcut(s)
			w.dispatcher.shortcuts = c
			layer.SetVisible(true)
			modalKey(t, w, events.KeyF5)
			w.SetFocusedWidget(nil)
			modalKey(t, w, events.KeyF5)
			if calls != 0 {
				t.Fatal("空焦点或传播阶段绕过了边界")
			}
			front := newTestWidget()
			front.Arrange(geometry.Rect(100, 0, 100, 100))
			root.AddChild(front)
			clicked := 0
			c1 := NewClickEventController()
			c1.ConnectClicked(func(EventContext) { clicked++ })
			a.AddEventController(c1)
			for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
				_ = w.DispatchEvent(events.PointerEvent{EventType: kind, Position: geometry.Point{X: 110, Y: 10}, Button: events.PointerButtonLeft})
			}
			if clicked != 0 {
				t.Fatal("前方非模态覆盖项不能穿透到模态内容")
			}
		})
	}
}

func TestModalPreventsBackgroundHoverWheelAndDrop(t *testing.T) {
	w, root, back, layer, a, _, _ := modalFixture(t)
	motion := NewMotionEventController()
	back.AddEventController(motion)
	wheelCalls, drops := 0, 0
	wheel := NewWheelEventController()
	wheel.ConnectScroll(func(EventContext, events.WheelEvent) { wheelCalls++ })
	back.AddEventController(wheel)
	target := NewDropTarget(DragFormatText)
	target.ConnectEnter(func(*DragMotion) { drops++ })
	back.AddEventController(target)
	ancestor := NewDropTarget(DragFormatText)
	ancestor.ConnectEnter(func(*DragMotion) { drops++ })
	root.AddEventController(ancestor)
	_ = w.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: geometry.Point{X: 10, Y: 10}})
	if !motion.Hover() {
		t.Fatal("前置悬停没有建立")
	}
	layer.SetVisible(true)
	_ = w.DispatchEvent(events.WheelEvent{Position: geometry.Point{X: 10, Y: 10}})
	_ = w.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: geometry.Point{X: 10, Y: 10}})
	if motion.Hover() || wheelCalls != 0 {
		t.Fatal("后台悬停或滚轮没有隔离")
	}
	offer := &testDragOffer{id: 1, formats: []dragdrop.Format{dragdrop.FormatText}}
	reply := dragdrop.Action(0)
	for _, p := range []geometry.Point{{X: 10, Y: 10}, {X: 110, Y: 10}} {
		_ = w.DispatchEvent(events.DragOfferEvent{EventType: events.DragEnter, Position: p, Offer: offer, Actions: dragdrop.Copy, ActionReply: &reply})
	}
	if drops != 0 || reply != 0 {
		t.Fatal("后台或边界外祖先 DropTarget 接收了拖放")
	}
	inside := NewDropTarget(DragFormatText)
	a.AddEventController(inside)
	inside.ConnectEnter(func(*DragMotion) { layer.SetVisible(false) })
	_ = w.DispatchEvent(events.DragOfferEvent{EventType: events.DragEnter, Position: geometry.Point{X: 110, Y: 10}, Offer: offer, Actions: dragdrop.Copy, ActionReply: &reply})
	if reply != 0 {
		t.Fatal("回调改变模态后仍接受旧拖放上下文")
	}
}

type modalProbeTarget struct {
	anchor          Widget
	keys, dismisses int
}

func (t *modalProbeTarget) DispatchEvent(events.Event) error { t.keys++; return nil }
func (t *modalProbeTarget) RequestDismiss()                  { t.dismisses++ }
func (t *modalProbeTarget) Anchor() Widget                   { return t.anchor }

// 明确只实现原来的两个方法，验证兼容契约而非意外提升 Anchor。
type modalLegacyTarget struct{ calls int }

func (t *modalLegacyTarget) DispatchEvent(events.Event) error { t.calls++; return nil }
func (t *modalLegacyTarget) RequestDismiss()                  {}

func TestModalPopoverAnchorAndLegacyTarget(t *testing.T) {
	w, _, back, layer, a, _, _ := modalFixture(t)
	legacy := new(modalLegacyTarget)
	w.SetModalTarget(legacy)
	modalKey(t, w, events.KeyF5)
	if legacy.calls != 1 {
		t.Fatal("无 Widget 模态时原 ModalTarget 契约改变")
	}
	w.SetModalTarget(nil)
	layer.SetVisible(true)
	w.SetModalTarget(legacy)
	if w.modalTarget != nil {
		t.Fatal("无来源目标绕过边界")
	}
	blocked := &modalProbeTarget{anchor: back}
	w.SetModalTarget(blocked)
	if w.modalTarget != nil {
		t.Fatal("后台锚点接管输入")
	}
	allowed := &modalProbeTarget{anchor: a}
	w.SetModalTarget(allowed)
	modalKey(t, w, events.KeyF5)
	if allowed.keys != 1 {
		t.Fatal("内部锚点无法接管输入")
	}
	w.SetModalTarget(nil)
	popup := NewPopover(a, nil).(*popover)
	popup.owner = w
	content := newTestWidget()
	content.SetFocusable(true)
	popup.SetWidget(content)
	if !popup.SetFocusedWidget(content) {
		t.Fatal("内部 Popover 焦点受阻")
	}
	popup.anchor = back
	if popup.SetFocusedWidget(content) {
		t.Fatal("后台 Popover 仍可以获取焦点")
	}
	popup.Destroy()
	if err := NewPopover(back, nil).Show(); err == nil {
		t.Fatal("后台 Popup 展示应拒绝")
	}
}

func TestModalIMEAndCaption(t *testing.T) {
	w, root, _, layer, _, _, _ := modalFixture(t)
	back, inside := NewTextInput(), NewTextInput()
	root.AddChild(back)
	layer.AddChild(inside)
	w.SetFocusedWidget(back)
	w.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodPreedit, Text: "pending", Caret: 2})
	layer.SetVisible(true)
	w.SetFocusedWidget(inside)
	w.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodCommit, Text: "中文"})
	if back.Text() != "" || inside.Text() != "中文" {
		t.Fatal("IME 提交未限定为内部编辑器")
	}
	if w.SetFocusedWidget(back) {
		t.Fatal("后台编辑器可以抢焦点")
	}
	if err := w.CommitText("abc"); err != nil || inside.Text() != "中文abc" {
		t.Fatal("CommitText 未走内部焦点")
	}
	c := w.Chrome().(*windowChrome)
	c.info.Enabled = true
	c.ConnectQueryRegion(func(p geometry.Point, r *ChromeRegion) { *r = ChromeRegionCaption })
	if c.queryRegion(geometry.Point{X: 110, Y: 100}) != ChromeRegionCaption {
		t.Fatal("活动遮罩应可声明 Caption")
	}
	if c.queryRegion(geometry.Point{X: 10, Y: 100}) != ChromeRegionClient {
		t.Fatal("后台 Caption 应阻挡")
	}
}

func modalKey(t *testing.T, w *window, key events.Key) bool {
	t.Helper()
	handled := false
	if err := w.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: key, Handled: &handled}); err != nil {
		t.Fatal(err)
	}
	return handled
}

func TestModalInputFocusAndPropagation(t *testing.T) {
	w, root, back, layer, a, b, scope := modalFixture(t)
	ancestor, global, inside, outside := 0, 0, 0, 0
	capture := NewKeyEventController()
	capture.SetPhase(PhaseCapture)
	capture.ConnectKeyDown(func(EventContext, events.KeyEvent) { ancestor++ })
	root.AddEventController(capture)
	w.dispatcher.shortcuts = NewShortcutController()
	w.dispatcher.shortcuts.SetPhase(PhaseCapture)
	shortcut := NewShortcut(KeyGesture{Key: KeyF5})
	shortcut.ConnectActivate(func() { global++ })
	w.dispatcher.shortcuts.AddShortcut(shortcut)
	keys := NewKeyEventController()
	keys.ConnectKeyDown(func(EventContext, events.KeyEvent) { inside++ })
	a.AddEventController(keys)
	click := NewClickEventController()
	click.ConnectClicked(func(EventContext) { outside++ })
	back.AddEventController(click)
	layer.SetVisible(true)
	if !scope.Active() || w.FocusedWidget() != a {
		t.Fatal("激活后应聚焦内部首项")
	}
	if w.SetFocusedWidget(back) || w.FocusedWidget() != a {
		t.Fatal("程序聚焦绕过了模态")
	}
	modalKey(t, w, events.KeyF5)
	if ancestor != 0 || global != 0 || inside != 1 {
		t.Fatalf("传播越界: ancestor=%d global=%d inside=%d", ancestor, global, inside)
	}
	for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
		_ = w.DispatchEvent(events.PointerEvent{EventType: kind, Position: geometry.Point{X: 20, Y: 20}, Button: events.PointerButtonLeft})
	}
	if outside != 0 {
		t.Fatal("边界外点击被处理")
	}
	if !modalKey(t, w, events.KeyTab) || w.FocusedWidget() != b {
		t.Fatal("Tab 未进入内部第二项")
	}
	modalKey(t, w, events.KeyTab)
	if w.FocusedWidget() != a {
		t.Fatal("Tab 未在内部循环")
	}
	layer.SetVisible(false)
	if scope.Active() || w.FocusedWidget() != back {
		t.Fatal("隐藏后未恢复原焦点")
	}
	modalKey(t, w, events.KeyF5)
	if global != 1 {
		t.Fatal("退出模态后全局快捷键未恢复")
	}
}

func TestModalEmptyDisabledAndSnapshot(t *testing.T) {
	w, _, back, layer, a, b, scope := modalFixture(t)
	a.SetFocusable(false)
	b.SetFocusable(false)
	layer.SetVisible(true)
	if w.FocusedWidget() != nil || !modalKey(t, w, events.KeyTab) {
		t.Fatal("空候选不得让 Tab 逃逸")
	}
	layer.SetEnabled(false)
	if !scope.Active() || w.SetFocusedWidget(back) {
		t.Fatal("禁用模态内容不能解锁后台")
	}
	info := layer.Snapshot()
	if got, ok := info.Attributes[ModalInfoKey].(ModalInfo); !ok || !got.Active {
		t.Fatal("缺少活动模态语义")
	}
	if !back.Snapshot().Enabled {
		t.Fatal("模态不应伪造后台 Disabled")
	}
	scope.Destroy()
	scope.Destroy()
	if scope.Active() || w.FocusedWidget() != back {
		t.Fatal("移除边界应恢复后台")
	}
}

func TestModalNestedOrderAndMiddleRemoval(t *testing.T) {
	w, root, back, layer, a, _, outer := modalFixture(t)
	layer.SetVisible(true)
	front, first := newTestWidget(), newTestWidget()
	first.SetFocusable(true)
	front.AddChild(first)
	root.AddChild(front)
	inner := NewModalScope(front)
	ensureModal(w)
	if !inner.Active() || outer.Active() || w.FocusedWidget() != first {
		t.Fatal("应由前方边界接管")
	}
	root.MoveChildBefore(front, layer)
	ensureModal(w)
	if !outer.Active() || w.FocusedWidget() != a {
		t.Fatal("模态顺序应遵循重排")
	}
	root.MoveChildBefore(front, nil)
	ensureModal(w)
	outer.Destroy()
	if !inner.Active() || w.FocusedWidget() != first {
		t.Fatal("移除中间层不能改变顶层焦点")
	}
	front.SetVisible(false)
	if w.FocusedWidget() != back {
		t.Fatal("中间层移除后仍需恢复最初后台焦点")
	}
}

func TestModalCallbackCannotReachOldPath(t *testing.T) {
	w, root, _, layer, a, _, _ := modalFixture(t)
	layer.SetVisible(true)
	background := 0
	keys := NewKeyEventController()
	keys.SetPhase(PhaseBubble)
	keys.ConnectKeyDown(func(EventContext, events.KeyEvent) { background++ })
	root.AddEventController(keys)
	inside := NewKeyEventController()
	inside.ConnectKeyDown(func(EventContext, events.KeyEvent) { layer.SetVisible(false) })
	a.AddEventController(inside)
	modalKey(t, w, events.KeyEscape)
	if background != 0 {
		t.Fatal("关闭边界的按键冒泡到了后台")
	}
	layer.SetVisible(true)
	a.ConnectFocused(func(focused bool) {
		if !focused {
			w.Destroy()
		}
	})
	layer.SetVisible(false)
	if !w.destroyed {
		t.Fatal("焦点回调未执行")
	}
}

func TestModalLifecycleAndRestoreInvalidFocus(t *testing.T) {
	w, root, back, layer, _, _, scope := modalFixture(t)
	layer.SetVisible(true)
	root.RemoveChild(back)
	layer.SetVisible(false)
	if w.FocusedWidget() != nil {
		t.Fatal("不能恢复已移除的焦点")
	}
	layer.SetVisible(true)
	other := &window{focused: true}
	t.Cleanup(other.Destroy)
	root.RemoveChild(layer)
	other.SetWidget(layer)
	if !scope.Active() || w.modal.active != nil {
		t.Fatal("边界未跟随跨 Root 挂载")
	}
	scope.Destroy()
	if scope.Active() {
		t.Fatal("销毁后仍报告活动")
	}
}

func TestModalCancelsBackgroundPress(t *testing.T) {
	w, _, back, layer, _, _, _ := modalFixture(t)
	click := NewClickEventController()
	clicks := 0
	click.ConnectClicked(func(EventContext) { clicks++ })
	back.AddEventController(click)
	point := geometry.Point{X: 20, Y: 20}
	_ = w.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Button: events.PointerButtonLeft, Position: point})
	if !click.Pressed() {
		t.Fatal("前置按压没有建立")
	}
	layer.SetVisible(true)
	if click.Pressed() || w.dispatcher.gesture != nil {
		t.Fatal("模态激活没有取消后台按压")
	}
	layer.SetVisible(false)
	_ = w.DispatchEvent(events.PointerEvent{EventType: events.PointerUp, Button: events.PointerButtonLeft, Position: point})
	if clicks != 0 {
		t.Fatal("旧抬起事件组成了后台点击")
	}
}

type modalLayoutProbe struct {
	WidgetBase
	measures int
}

func (p *modalLayoutProbe) Measure(c layout.Constraint) layout.Measurement {
	p.measures++
	return layout.Measured(c.Clamp(geometry.Size{}))
}

func TestModalFocusCallbackStopsDestroyedLayout(t *testing.T) {
	w := &window{focused: true}
	t.Cleanup(w.Destroy)
	root, back, layer := &modalLayoutProbe{}, newTestWidget(), newTestWidget()
	back.SetFocusable(true)
	layer.SetFocusable(true)
	root.AddChild(root, back)
	root.AddChild(root, layer)
	w.SetWidget(root)
	w.SetFocusedWidget(back)
	back.ConnectFocused(func(focused bool) {
		if !focused {
			w.Destroy()
		}
	})
	NewModalScope(layer)
	w.layoutDirty = true
	w.layoutFrame(root)
	if !w.destroyed || root.measures != 0 {
		t.Fatal("焦点回调销毁窗口后仍继续布局")
	}
}

func TestModalBlockedPopoverStillReleasesDestroyedSurface(t *testing.T) {
	_, _, back, layer, _, _, _ := modalFixture(t)
	layer.SetVisible(true)
	p := NewPopover(back, nil).(*popover)
	p.visible = true
	defer p.Destroy()
	_ = p.DispatchEvent(events.DestroyEvent{})
	if p.visible || p.lifecycle == 0 {
		t.Fatal("模态输入限制阻止了原生表面销毁清理")
	}
}
