package ui

import (
	"slices"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
)

func TestOverlayReconcilesMainIndependently(t *testing.T) {
	r := newRoot()
	t.Cleanup(r.unmountWindow)
	build := func(main View, text string) *OverlayView {
		return Overlay(main, OverlayItem(TextInput().Text(text).ID("find")))
	}
	o := r.update(build(Label("main"), "first")).(*gui.Overlay)
	main, floating := o.Child(), o.Children()[1].(*gui.TextInput)
	entry := r.root.state.(*overlayChildren).items[floating]
	handle := entry.position
	for _, child := range []View{nil, Button("new main"), Label("main again")} {
		if got := r.update(build(child, "updated")); got != o {
			t.Fatal("rebuild replaced overlay")
		}
		if floating.Parent() != o || floating.Text() != "updated" {
			t.Fatal("main replacement lost the floating slot")
		}
		state := r.root.state.(*overlayChildren)
		if state.items[floating] != entry || entry.position != handle {
			t.Fatal("rebuild replaced the floating state or signal")
		}
		if children := o.Children(); children[len(children)-1] != floating {
			t.Fatal("main insertion changed stacking order")
		}
	}
	if main.Parent() != nil {
		t.Fatal("replaced main is still attached")
	}
	r.update(Overlay(nil))
	if len(o.Children()) != 0 || floating.Parent() != nil || entry.position != nil {
		t.Fatal("clearing declarations retained widgets or signals")
	}
}

func TestOverlayReconcilesFloatingPositionsAndNilItems(t *testing.T) {
	r := newRoot()
	t.Cleanup(r.unmountWindow)
	builds := 0
	empty := &testCompositionView{builds: &builds}
	// 空组合节点现在保留声明位置；用明确 ID 验证插入它后仍保留已有视觉项。
	items := []*OverlayItemView{OverlayItem(Label("one").ID("one")), OverlayItem(TextInput().ID("middle")), OverlayItem(Label("three").ID("three"))}
	v := Overlay(Label("main"), items...)
	items[0] = nil // The caller's slice is not the declaration's storage.
	o := r.update(v).(*gui.Overlay)
	main := o.Child()
	old := o.Children()[1:]
	r.update(Overlay(nil).Child(Label("MAIN")).Overlays(
		nil, OverlayItem(nil), OverlayItem(empty),
		OverlayItem(Label("ONE").ID("one")), OverlayItem(Label("replacement").ID("middle")), OverlayItem(Label("THREE").ID("three")),
	))
	children := o.Children()
	if builds != 1 || len(children) != 4 || o.Child() != main || children[1] != old[0] {
		t.Fatal("nil filtering, composition expansion or prefix reuse failed")
	}
	if children[2] == old[1] || children[3] != old[2] || old[1].Parent() != nil || old[2].Parent() != o {
		t.Fatal("type mismatch must replace only that position and retain other matching nodes")
	}
	if children[1].(*gui.Label).Text() != "ONE" {
		t.Fatal("reused content was not updated")
	}
	r.update(Overlay(Label("main")).Overlays(OverlayItem(Label("ignored"))).Overlays())
	if len(o.Children()) != 1 || len(r.root.children) != 1 {
		t.Fatal("Overlays must replace, not append")
	}
}

func TestOverlayIDReorderKeepsPlacementBindingsAndSnapshotOrder(t *testing.T) {
	r := newRoot()
	t.Cleanup(r.unmountWindow)
	o := r.update(Overlay(Label("main"), OverlayItem(Label("a").ID("a")), OverlayItem(Label("b").ID("b")))).(*gui.Overlay)
	old := slices.Clone(o.Children())
	s := r.root.state.(*overlayChildren)
	a, b := s.items[old[1]], s.items[old[2]]
	r.update(Overlay(Label("MAIN"), OverlayItem(Label("B").ID("b")).Fill(true), OverlayItem(Label("A").ID("a"))))
	if !slices.Equal(o.Children(), []gui.Widget{old[0], old[2], old[1]}) || s.items[old[1]] != a || s.items[old[2]] != b || !o.OverlayFill(old[2]) || o.Snapshot().Children[1].ID != "b" {
		t.Fatal("reorder lost stacking, semantic order or placement state")
	}
}

func TestOverlayPositionFillAndVisibilityUpdates(t *testing.T) {
	r := newRoot()
	t.Cleanup(r.unmountWindow)
	oldCalls, newCalls := 0, 0
	oldPosition := func(_, _ geometry.Size, p *geometry.Point) { oldCalls++; p.X = 7 }
	newPosition := func(available, size geometry.Size, p *geometry.Point) {
		newCalls++
		*p = geometry.Point{X: available.Width - size.Width - 8, Y: 9}
	}
	content := func() *BoxView { return HBox().MinSize(40, 20).MaxSize(60, 30) }
	o := r.update(Overlay(nil, OverlayItem(content()).OnPosition(oldPosition))).(*gui.Overlay)
	child := o.Children()[0]
	entry := r.root.state.(*overlayChildren).items[child]
	handle := entry.position
	o.Arrange(geometry.Rect(20, 30, 200, 100))
	if child.Rect() != geometry.Rect(7, 0, 40, 20) || oldCalls != 1 {
		t.Fatal("default content sizing/initial callback failed")
	}
	r.update(Overlay(nil, OverlayItem(content()).Fill(true).OnPosition(oldPosition).OnPosition(newPosition)))
	o.Arrange(geometry.Rect(20, 30, 200, 100))
	if child.Rect() != geometry.Rect(-8, 9, 200, 100) || oldCalls != 1 || newCalls != 1 {
		t.Fatal("fill did not use GUI constraints or callback was not replaced")
	}
	r.update(Overlay(nil, OverlayItem(content()).OnPosition(newPosition)))
	o.Arrange(geometry.Rect(20, 30, 200, 100))
	if child.Rect() != geometry.Rect(152, 9, 40, 20) || o.OverlayFill(child) {
		t.Fatal("omitted Fill did not restore natural sizing")
	}
	// No UI rebuild is needed for resizing.
	o.Arrange(geometry.Rect(20, 30, 300, 150))
	if child.Rect() != geometry.Rect(252, 9, 40, 20) {
		t.Fatal("resizing did not query placement again")
	}
	r.update(Overlay(nil, OverlayItem(content().Visible(false)).OnPosition(newPosition)))
	before := newCalls
	o.Arrange(geometry.Rect(0, 0, 100, 80))
	if newCalls != before || child.Visible() || o.Snapshot().Children[0].Visible {
		t.Fatal("hidden content still queried placement or snapshot lost visibility")
	}
	r.update(Overlay(nil, OverlayItem(content()).Fill(true).Fill(false).OnPosition(newPosition).OnPosition(nil)))
	o.Arrange(geometry.Rect(0, 0, 100, 80))
	if child.Rect() != geometry.Rect(0, 0, 40, 20) || !child.Visible() || newCalls != before {
		t.Fatal("explicit callback reset/default visibility failed")
	}
	r.update(Overlay(nil, OverlayItem(content()).OnPosition(newPosition)))
	r.update(Overlay(nil, OverlayItem(content())))
	o.Arrange(geometry.Rect(0, 0, 100, 80))
	if child.Rect().Pos != (geometry.Point{}) || newCalls != before {
		t.Fatal("omitted callback retained stale placement")
	}
	if o.Children()[0] != child || entry.position != handle {
		t.Fatal("configuration/visibility updates recreated child or connection")
	}
}

type overlayDisconnectProbe struct {
	signal.Handle
	calls int
}

func (h *overlayDisconnectProbe) Disconnect() { h.calls++; h.Handle.Disconnect() }

func TestOverlayReleaseOwnsEveryNodeAndSignal(t *testing.T) {
	for _, destroying := range []bool{false, true} {
		t.Run(map[bool]string{false: "detach", true: "window-destroy"}[destroying], func(t *testing.T) {
			r := newRoot()
			trackers := []*lifecycleTracker{{}, {}, {}}
			child := func(i int) View { return &lifecycleView{tracker: trackers[i]} }
			o := r.update(Overlay(child(0), OverlayItem(
				Overlay(child(1), OverlayItem(child(2))),
			))).(*gui.Overlay)
			outerState := r.root.state.(*overlayChildren)
			innerState := r.root.children[1].nodes[0].state.(*overlayChildren)
			var probes []*overlayDisconnectProbe
			for _, state := range []*overlayChildren{outerState, innerState} {
				entry := state.items[state.overlay.Children()[1]]
				probe := &overlayDisconnectProbe{Handle: entry.position}
				entry.position = probe
				probes = append(probes, probe)
			}
			outerChildren := o.Children()
			inner := outerChildren[1].(*gui.Overlay)
			innerChildren := inner.Children()
			if destroying {
				r.unmountForWindowDestroy()
				if !slices.Equal(o.Children(), outerChildren) || !slices.Equal(inner.Children(), innerChildren) {
					t.Fatal("UI detached widgets before Window could destroy them")
				}
			} else {
				r.update(Label("replacement"))
				if len(o.Children()) != 0 || len(inner.Children()) != 0 {
					t.Fatal("unmounted overlay retained its main/floating children")
				}
				for _, widget := range append(outerChildren, innerChildren...) {
					if widget.Parent() != nil {
						t.Fatal("released child still has a parent")
					}
				}
			}
			r.unmountWindow()
			for _, tracker := range trackers {
				if tracker.mounts != 1 || tracker.unmounts != 1 {
					t.Fatalf("child was not released exactly once: %+v", tracker)
				}
			}
			for _, probe := range probes {
				if probe.calls != 1 {
					t.Fatalf("position connection disconnected %d times", probe.calls)
				}
			}
			if outerState.items != nil || innerState.items != nil {
				t.Fatal("unmount retained slot state")
			}
		})
	}
}

// Headless host: inputs still enter Window.DispatchEvent and the real GUI
// dispatcher/controllers; never invoke a button callback directly.
type overlayInputWindow struct {
	*testWindow
	dispatcher gui.EventDispatcher
}

func (w *overlayInputWindow) DispatchEvent(event events.Event) error {
	return w.dispatcher.DispatchEvent(w, event)
}

func TestOverlayBindingSnapshotAndPointerRouting(t *testing.T) {
	r := newRoot()
	t.Cleanup(r.unmountWindow)
	win := &overlayInputWindow{testWindow: newTestWindow()}
	background, floating, panel := 0, 0, 0
	build := func(show bool) *OverlayView {
		return Overlay(
			Button().OnClick(func() { background++ }),
			OverlayItem(Button().MinSize(40, 20).ID("find").OnClick(func() { floating++ })).
				OnPosition(func(_, _ geometry.Size, p *geometry.Point) { p.X, p.Y = 150, 10 }),
			OverlayItem(Overlay(
				HBox().ID("mask"),
				OverlayItem(Button().MinSize(80, 40).ID("panel").OnClick(func() { panel++ })).
					OnPosition(func(available, size geometry.Size, p *geometry.Point) {
						p.X, p.Y = (available.Width-size.Width)/2, (available.Height-size.Height)/2
					}),
			).Visible(show)).Fill(true),
		)
	}
	o := r.updateWindow(win, build(false)).(*gui.Overlay)
	o.Arrange(geometry.Rect(0, 0, 300, 200))
	info := o.Snapshot()
	if info.Role != gui.RoleBox || len(info.Children) != 3 || info.Children[1].Role != gui.RoleButton || info.Children[2].Visible {
		t.Fatalf("binding added wrappers or changed GUI semantics: %+v", info)
	}
	click := func(point geometry.Point) {
		t.Helper()
		for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
			if err := win.DispatchEvent(events.PointerEvent{EventType: kind, Button: events.PointerButtonLeft, Position: point}); err != nil {
				t.Fatal(err)
			}
		}
	}
	click(geometry.Point{X: 10, Y: 10})
	click(info.Children[1].Bounds.Center())
	if background != 1 || floating != 1 {
		t.Fatal("floating content/outside input routing failed")
	}
	r.updateWindow(win, build(true))
	o.Arrange(geometry.Rect(0, 0, 300, 200))
	info = o.Snapshot()
	confirm := info.Children[2]
	if confirm.Role != gui.RoleBox || len(confirm.Children) != 2 || confirm.Bounds != geometry.Rect(0, 0, 300, 200) {
		t.Fatalf("nested full-size confirmation lost its tree: %+v", confirm)
	}
	click(geometry.Point{X: 10, Y: 10})
	click(confirm.Children[1].Bounds.Center())
	if background != 1 || floating != 1 || panel != 1 {
		t.Fatal("mask/panel did not block underlying siblings")
	}
	r.updateWindow(win, build(false))
	click(geometry.Point{X: 10, Y: 10})
	if background != 2 {
		t.Fatal("hiding confirmation did not restore background input")
	}
}

// 未 Show 的独立 Popover 是真实 GUI Root，不创建原生窗口或依赖桌面。
func TestOverlayModalLifecycleAndFocus(t *testing.T) {
	r, host := newRoot(), gui.NewPopover(nil, nil)
	t.Cleanup(func() { r.unmountWindow(); host.Destroy() })
	focus := host.(gui.EventTarget)
	build := func(show bool, item func(*OverlayItemView) *OverlayItemView) View {
		floating := OverlayItem(VBox(Button().ID("first"), Button().ID("second")).ID("layer").Visible(show))
		return Overlay(TextInput().ID("background"), item(floating))
	}
	modal := func(v *OverlayItemView) *OverlayItemView { return v.Modal(true) }
	o := r.update(build(false, modal)).(*gui.Overlay)
	host.SetWidget(o)
	back, layer := o.Child(), o.Children()[1]
	state := r.root.state.(*overlayChildren)
	entry := state.items[layer]
	scope := entry.scope
	if scope == nil || scope.Active() || !focus.SetFocusedWidget(back) {
		t.Fatal("隐藏项不应接管输入，但应保留声明关联")
	}
	r.update(build(true, modal))
	first, second := layer.Children()[0], layer.Children()[1]
	if !scope.Active() || focus.FocusedWidget() != first || focus.SetFocusedWidget(back) {
		t.Fatal("显示覆盖项后未建立真实 GUI 模态与初始焦点")
	}
	focus.SetFocusedWidget(second)
	r.update(build(true, modal))
	if state.items[layer] != entry || entry.scope != scope || focus.FocusedWidget() != second {
		t.Fatal("普通重建重新创建 Scope 或重置了内部焦点")
	}
	r.update(build(false, modal))
	if scope.Active() || entry.scope != scope || focus.FocusedWidget() != back {
		t.Fatal("隐藏应撤销参与并恢复后台焦点，不销毁关联")
	}
	r.update(build(true, modal))
	if !scope.Active() || focus.FocusedWidget() != second {
		t.Fatal("再次显示没有保留边界内部的最近焦点")
	}
	r.update(build(true, func(v *OverlayItemView) *OverlayItemView { return v.Modal(true).Modal(false) }))
	if entry.scope != nil || scope.Active() || !layer.Visible() || focus.FocusedWidget() != back {
		t.Fatal("Modal(false) 应仅移除输入边界并保留可见内容")
	}
	if _, exists := layer.Snapshot().Attributes[gui.ModalInfoKey]; exists {
		t.Fatal("撤销后仍残留模态语义")
	}
	r.update(build(true, modal))
	scope = entry.scope
	r.update(build(true, func(v *OverlayItemView) *OverlayItemView { return v }))
	if entry.scope != nil || scope.Active() {
		t.Fatal("省略 Modal 未恢复默认非模态")
	}
	r.update(build(true, modal))
	scope = entry.scope
	r.update(Overlay(TextInput().ID("background")))
	if entry.scope != nil || scope.Active() || layer.Parent() != nil {
		t.Fatal("移除覆盖项未释放边界")
	}
}

func TestOverlayModalInputWithoutFill(t *testing.T) {
	r, host := newRoot(), gui.NewPopover(nil, nil)
	t.Cleanup(func() { r.unmountWindow(); host.Destroy() })
	focus := host.(gui.EventTarget)
	outside, inside, keys := 0, 0, 0
	build := func(modal bool) View {
		return Overlay(Button().OnClick(func() { outside++ }),
			OverlayItem(VBox(
				Button().MinSize(60, 30).OnClick(func() { inside++ }),
				Button().MinSize(60, 30),
			).Spacing(10).MinSize(100, 90)).Modal(modal).
				OnPosition(func(_, _ geometry.Size, p *geometry.Point) { p.X, p.Y = 100, 20 }))
	}
	o := r.update(build(true)).(*gui.Overlay)
	host.SetWidget(o)
	o.Measure(layout.Tight(geometry.Size{Width: 300, Height: 200}))
	o.Arrange(geometry.Rect(0, 0, 300, 200))
	layer := o.Children()[1]
	controller := gui.NewKeyEventController()
	controller.SetPhase(gui.PhaseCapture)
	controller.ConnectKeyDown(func(gui.EventContext, events.KeyEvent) { keys++ })
	o.AddEventController(controller)
	click := func(p geometry.Point) {
		t.Helper()
		for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
			if err := host.DispatchEvent(events.PointerEvent{EventType: kind, Position: p, Button: events.PointerButtonLeft}); err != nil {
				t.Fatal(err)
			}
		}
	}
	if o.OverlayFill(layer) || layer.Rect().Width >= o.Rect().Width || len(o.Snapshot().Children) != 2 {
		t.Fatal("Modal 不应强制 Fill 或生成额外 Widget")
	}
	click(geometry.Point{X: 10, Y: 10})
	click(layer.Children()[0].Snapshot().Bounds.Center())
	if outside != 0 || inside != 1 {
		t.Fatal("非铺满模态项没有隔离后台点击或阻挡了内部点击")
	}
	for i := range 7 {
		modifiers := events.Modifiers(0)
		if i > 3 {
			modifiers = events.ModifierShift
		}
		handled := false
		if err := host.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: events.KeyTab, Modifiers: modifiers, Handled: &handled}); err != nil {
			t.Fatal(err)
		}
		focused := focus.FocusedWidget()
		if !handled || focused != layer.Children()[0] && focused != layer.Children()[1] {
			t.Fatal("Tab 或 Shift+Tab 逃离了模态覆盖项")
		}
	}
	if keys != 0 {
		t.Fatal("按键传播越过模态项进入外部祖先")
	}
	r.update(build(false))
	click(geometry.Point{X: 10, Y: 10})
	if outside != 1 {
		t.Fatal("撤销模态没有恢复后台交互")
	}
}

func TestOverlayModalLocalOutputReplacement(t *testing.T) {
	mode := MakeState(0)
	_, fake := bindingApp(t, func() RootView {
		return Window("main").Content(Overlay(Label("main"), OverlayItem(Build(func() View {
			switch mode.Get() {
			case 0:
				return Label("floating")
			case 1:
				return Button("replacement")
			default:
				return nil
			}
		}).Observe(&mode)).Modal(true)))
	})
	o := fake.windows[0].Widget().(*gui.Overlay)
	host := gui.NewPopover(nil, nil)
	t.Cleanup(host.Destroy)
	host.SetWidget(o)
	first := o.Children()[1]
	for _, value := range []int{1, 2, 0} {
		old := first
		mode.Set(value)
		fake.runPosted()
		if old != nil {
			if _, exists := old.Snapshot().Attributes[gui.ModalInfoKey]; exists {
				t.Fatal("局部组合替换输出后保留了旧边界")
			}
		}
		first = nil
		if value != 2 {
			first = o.Children()[1]
			info, ok := first.Snapshot().Attributes[gui.ModalInfoKey].(gui.ModalInfo)
			if !ok || !info.Active {
				t.Fatal("局部组合的新输出没有重新应用 Modal")
			}
		} else if len(o.Children()) != 1 {
			t.Fatal("空输出仍保留覆盖项")
		}
	}
}

func TestOverlayDoesNotReleaseWidgetOwnedModal(t *testing.T) {
	r := newRoot()
	o := r.update(Overlay(nil, OverlayItem(Label("owned")))).(*gui.Overlay)
	child := o.Children()[0]
	scope := gui.NewModalScope(child)
	t.Cleanup(scope.Destroy)
	r.update(Overlay(nil, OverlayItem(Label("updated"))))
	r.unmountWindow()
	if _, exists := child.Snapshot().Attributes[gui.ModalInfoKey]; !exists {
		t.Fatal("未声明 Modal 的覆盖项释放了控件自有的边界")
	}
}

func TestOverlayModalReorderAndWindowRelease(t *testing.T) {
	r, host := newRoot(), gui.NewPopover(nil, nil)
	t.Cleanup(func() { r.unmountWindow(); host.Destroy() })
	item := func(id string) *OverlayItemView { return OverlayItem(Button().ID(id)).Modal(true) }
	o := r.update(Overlay(nil, item("a"), item("b"))).(*gui.Overlay)
	host.SetWidget(o)
	state := r.root.state.(*overlayChildren)
	a, b := o.Children()[0], o.Children()[1]
	sa, sb := state.items[a].scope, state.items[b].scope
	if sa.Active() || !sb.Active() {
		t.Fatal("初始模态未遵循覆盖项顺序")
	}
	r.update(Overlay(nil, item("b"), item("a")))
	if state.items[a].scope != sa || state.items[b].scope != sb || !sa.Active() || sb.Active() {
		t.Fatal("声明重排重新创建了 Scope 或未改变前台模态")
	}
	r.unmountForWindowDestroy()
	if sa.Active() || sb.Active() || state.items != nil || !slices.Equal(o.Children(), []gui.Widget{b, a}) {
		t.Fatal("窗口释放未清理 UI 关联，或提前拆走了 GUI 控件树")
	}
}
