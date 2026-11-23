package gui

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"log"
	"strings"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/style"
)

type popoverMeasureWidget struct {
	WidgetBase
	size      geometry.Size
	onMeasure func()
}

func TestWindowPopoversFollowVisibleLifetime(t *testing.T) {
	p, native, _, app := placementFixture(t, geometry.Rect(0, 0, 1000, 1000))
	w := app.windows[0]
	if len(w.Popovers()) != 0 {
		t.Fatal("unshown popover was enumerated")
	}
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	if got := w.Popovers(); len(got) != 1 || got[0].Widget() != p.Widget() {
		t.Fatalf("shown popovers=%v", got)
	} else {
		got[0] = nil
		if w.Popovers()[0].Widget() != p.Widget() {
			t.Fatal("Popovers exposed the internal slice")
		}
	}
	if err := p.Show(); err != nil || len(w.Popovers()) != 1 {
		t.Fatal("repeated Show registered the same object twice")
	}
	p.Hide()
	if len(w.Popovers()) != 0 {
		t.Fatal("hidden popover retained by window")
	}
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	other := testWindowWithSurface(&window{surfaceBase: surfaceBase{platformWindow: native}})
	other.SetWidget(p.Anchor()) // unmount releases the old native surface
	if len(w.Popovers()) != 0 {
		t.Fatal("anchor migration left an old window entry")
	}
	if err := p.Show(); err != nil || len(other.Popovers()) != 1 {
		t.Fatalf("new owner did not register popover: %v", err)
	}
	p.Destroy()
	if len(other.Popovers()) != 0 {
		t.Fatal("destroyed popover retained by window")
	}
}

func TestWindowPopoversRemoveBeforeClosedCallbacks(t *testing.T) {
	p, _, _, app := placementFixture(t, geometry.Rect(0, 0, 1000, 1000))
	w := app.windows[0]
	p.ConnectClosed(func() {
		if len(w.Popovers()) != 0 {
			t.Fatal("closed callback observed stale visible popovers")
		}
	})
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	p.Anchor().base().detachRoot(p.Anchor())
	if len(w.Popovers()) != 0 {
		t.Fatal("anchor unmount retained popup")
	}
}

type failedShowPopup struct{ platform.Popup }

func (*failedShowPopup) Show() error { return errors.New("native Show failed") }

type failedShowPlatform struct{ *placementPlatform }

func (p *failedShowPlatform) NewPopup(owner platform.Window, size geometry.Size, handler platform.EventHandler, options platform.PopupOptions) (platform.Popup, error) {
	native, err := p.placementPlatform.NewPopup(owner, size, handler, options)
	if err != nil {
		return nil, err
	}
	return &failedShowPopup{native}, nil
}

func TestWindowPopoversExcludeFailedShowAndOwnerDestruction(t *testing.T) {
	p, _, plat, app := placementFixture(t, geometry.Rect(0, 0, 1000, 1000))
	w := app.windows[0]
	app.platform = &failedShowPlatform{plat}
	if err := p.Show(); err == nil || len(w.Popovers()) != 0 || p.Visible() {
		t.Fatal("failed native Show registered a visible popup")
	}
	p.releaseNative()
	app.platform = plat
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	w.Destroy()
	if len(w.Popovers()) != 0 || p.Visible() {
		t.Fatal("owner destruction retained a shown popup")
	}
}

func TestPopoverTabFocusAndModalIsolation(t *testing.T) {
	ownerRoot := newTestWidget()
	ownerRoot.SetFocusable(true)
	win := testWindowWithSurface(&window{})
	win.SetWidget(ownerRoot)
	win.SetFocusedWidget(ownerRoot)
	root, first, second := newTestWidget(), newTestWidget(), newTestWidget()
	first.SetFocusable(true)
	second.SetFocusable(true)
	root.AddChild(first)
	root.AddChild(second)
	p := &popover{modal: true, autoDismiss: true, visible: true}
	p.SetWidget(root)
	win.SetModalTarget(p)
	var changes []bool
	first.ConnectFocused(func(focused bool) { changes = append(changes, focused) })
	for _, want := range []Widget{first, second, first} {
		handled := false
		e := shortcutPress(events.KeyTab, 0)
		e.Handled = &handled
		_ = win.DispatchEvent(e)
		if !handled || p.FocusedWidget() != want || !want.Focused() || !root.ContainsFocus() || win.FocusedWidget() != ownerRoot {
			t.Fatalf("modal Tab: focus=%p want=%p handled=%v", p.FocusedWidget(), want, handled)
		}
	}
	root.RemoveChild(first)
	if p.FocusedWidget() != nil || first.Focused() || root.ContainsFocus() {
		t.Fatal("popover detach retained focus")
	}
	if len(changes) != 4 || !changes[0] || changes[1] || !changes[2] || changes[3] {
		t.Fatalf("focus signals=%v, want [true false true false]", changes)
	}
	win.SetModalTarget(nil)
	p.SetFocusedWidget(second)
	p.Hide()
	if p.FocusedWidget() != nil || second.Focused() || root.ContainsFocus() {
		t.Fatal("hiding popover retained logical focus")
	}
}

func TestPopoverFocusVisibleIsolationAndReset(t *testing.T) {
	ownerRoot, popupRoot := newTestWidget(), newTestWidget()
	for _, widget := range []*testWidget{ownerRoot, popupRoot} {
		widget.SetFocusable(true)
		widget.Arrange(geometry.Rect(0, 0, 100, 100))
	}
	win := testWindowWithSurface(&window{})
	win.SetWidget(ownerRoot)
	p := &popover{rootBase: rootBase{width: 100, height: 100}, modal: true, autoDismiss: true, visible: true}
	p.SetWidget(popupRoot)
	t.Cleanup(func() { p.SetWidget(nil); win.SetWidget(nil) })
	_ = win.DispatchEvent(shortcutPress(events.KeyTab, 0))
	win.SetModalTarget(p)
	_ = win.DispatchEvent(shortcutPress(events.KeyTab, 0))
	if !ownerRoot.FocusVisible() || !popupRoot.FocusVisible() {
		t.Fatal("modal navigation changed the owner's input mode")
	}
	_ = p.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Position: geometry.Point{X: 5, Y: 5}, Button: events.PointerButtonLeft})
	if !ownerRoot.FocusVisible() || popupRoot.FocusVisible() || !popupRoot.Focused() {
		t.Fatal("popup pointer mode leaked to owner or changed logical focus")
	}
	_ = win.DispatchEvent(shortcutPress(events.KeyTab, 0))
	// Even a click in the transparent popup corner clears its own hint before
	// requesting dismissal; applications need not hide it synchronously.
	_ = p.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Position: geometry.Point{X: -1, Y: -1}})
	if popupRoot.FocusVisible() || !ownerRoot.FocusVisible() {
		t.Fatal("popup outside press did not independently clear hints")
	}
	_ = win.DispatchEvent(shortcutPress(events.KeyTab, 0))
	p.ConnectDismissRequest(func() { p.Hide(); win.SetModalTarget(nil) })
	_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerDown})
	if ownerRoot.FocusVisible() || !ownerRoot.Focused() || popupRoot.FocusVisible() || p.FocusedWidget() != nil {
		t.Fatal("owner outside press or popup Hide retained keyboard hints")
	}
	// Reopening and programmatically focusing starts without the old popup mode.
	p.visible = true
	p.SetFocusedWidget(popupRoot)
	if popupRoot.FocusVisible() {
		t.Fatal("reopened popup inherited the hidden session's keyboard mode")
	}
}

func (w *popoverMeasureWidget) Measure(layout.Constraint) layout.Measurement {
	if w.onMeasure != nil {
		w.onMeasure()
	}
	return layout.Measured(w.size)
}

type recordingPlatformPopup struct {
	width, height float32
	observed      *geometry.Size
}

func (*recordingPlatformPopup) NativeHandle() uintptr           { return 1 }
func (*recordingPlatformPopup) Transparent() bool               { return false }
func (*recordingPlatformPopup) Draw(image.Image) error          { return nil }
func (*recordingPlatformPopup) RequestPaint() error             { return nil }
func (*recordingPlatformPopup) Destroy()                        {}
func (*recordingPlatformPopup) SetPosition(float32, float32)    {}
func (p *recordingPlatformPopup) SetSize(width, height float32) { p.width, p.height = width, height }
func (*recordingPlatformPopup) Show() error                     { return nil }
func (*recordingPlatformPopup) Hide() error                     { return nil }
func (p *recordingPlatformPopup) ClientSize() (geometry.Size, geometry.Size, error) {
	size := geometry.Size{Width: p.width, Height: p.height}
	if p.observed != nil {
		size = *p.observed
	}
	return size, size, nil
}
func (*recordingPlatformPopup) Repaint() error { return nil }

// The window forwards its own input to an open popover (§7): keyboard is
// forwarded, an outside click / Esc / focus loss requests dismissal, and the
// window's own widget tree is not reached while a popover is open.
func TestWindowForwardsToPopover(t *testing.T) {
	winRoot := newTestWidget()
	winRoot.Arrange(geometry.Rect(0, 0, 100, 100))
	var winCalls []string
	winRoot.AddEventController(newRecordingController("win", PhaseTarget, &winCalls, nil))
	win := testWindowWithSurface(&window{surfaceBase: surfaceBase{root: winRoot}})

	content := newTestWidget()
	content.Arrange(geometry.Rect(0, 0, 60, 40))
	var popCalls []string
	content.AddEventController(newRecordingController("pop", PhaseTarget, &popCalls, nil))

	p := &popover{modal: true, autoDismiss: true} // 明确启用菜单式关闭，Modal 只指定输入独占。
	p.SetWidget(content)
	p.visible = true
	win.SetModalTarget(p) // what Show() does for a modal popover

	dismisses := 0
	p.dismissRequest.Connect(func() { dismisses++ })

	// Outside click (the owner only ever receives clicks outside the popover).
	win.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Position: geometry.Point{X: 5, Y: 5}})
	if dismisses != 1 {
		t.Fatalf("PointerDown: dismisses=%d, want 1", dismisses)
	}
	if len(winCalls) != 0 {
		t.Fatalf("PointerDown should be swallowed, winCalls=%v", winCalls)
	}

	// Esc requests dismissal.
	win.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: events.KeyEscape})
	if dismisses != 2 {
		t.Fatalf("Esc: dismisses=%d, want 2", dismisses)
	}

	// A non-Esc key is forwarded to the popover's content, not the window's tree.
	win.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: events.KeyEnter})
	if dismisses != 2 {
		t.Fatalf("non-Esc key must not dismiss, dismisses=%d", dismisses)
	}
	if len(popCalls) == 0 {
		t.Fatalf("non-Esc key should reach popover content, popCalls empty")
	}
	if len(winCalls) != 0 {
		t.Fatalf("non-Esc key should not reach window tree, winCalls=%v", winCalls)
	}

	// Focus loss requests dismissal.
	win.DispatchEvent(events.FocusEvent{Focused: false})
	if dismisses != 3 {
		t.Fatalf("FocusEvent{false}: dismisses=%d, want 3", dismisses)
	}
}

// A widget hosted in a popover reaches the popover as its host for repaint and
// layout requests — Window() is nil there, so those must go through Root.
func TestPopoverHostsWidgetForRepaintAndLayout(t *testing.T) {
	p := &popover{}
	content := newTestWidget()
	p.SetWidget(content)

	if content.Root() != Root(p) {
		t.Fatalf("content.Root() should be the popover")
	}
	if content.Window() != nil {
		t.Fatalf("content.Window() should be nil for a popover-hosted widget")
	}

	p.layoutDirty = false
	content.RequestLayout()
	if !p.layoutDirty {
		t.Fatalf("RequestLayout on popover content should reach the popover (mark it dirty)")
	}
}

func TestPopoverResizeKeepsObservedNativeSizeUntilSizeEvent(t *testing.T) {
	// 可见实例提交尺寸：原生按整数像素向外取整，查询和通知仍报告实际 50 DIP。
	native := &recordingPlatformPopup{observed: &geometry.Size{Width: 120, Height: 50}}
	p := &popover{rootBase: rootBase{width: 120, height: 50, pixelWidth: 120, pixelHeight: 50},
		platformPopup: native,
		visible:       true,
		widget:        &popoverMeasureWidget{size: geometry.Size{Width: 120, Height: 49.65625}}}

	p.measureAndSize()
	if native.width != 120 || native.height != 49.65625 {
		t.Fatalf("native size request = %gx%g, want 120x49.65625", native.width, native.height)
	}
	if p.width != 120 || p.height != 50 {
		t.Fatalf("host size changed before SizeEvent: %gx%g, want authoritative 120x50", p.width, p.height)
	}

	p.onEvent(events.SizeEvent{Width: 120, Height: 51, PixelWidth: 120, PixelHeight: 51})
	if p.width != 120 || p.height != 51 {
		t.Fatalf("host size after SizeEvent = %gx%g, want 120x51", p.width, p.height)
	}
}

// A modal popover registers itself as the window's modal target and resigns when
// it stops being modal — through the public Window.SetModalTarget API only.
func TestPopoverModalTogglesWindowModalTarget(t *testing.T) {
	win := testWindowWithSurface(&window{surfaceBase: surfaceBase{root: newTestWidget()}})
	p := &popover{owner: win, visible: true}

	p.becomeModalTarget()
	if win.modalTarget == nil {
		t.Fatalf("a modal popover should register itself as the owner's modal target")
	}
	p.resignModalTarget()
	if win.modalTarget != nil {
		t.Fatalf("resigning should clear the window's modal target")
	}
}

func TestPopoverSetWidgetMigratesFromWindow(t *testing.T) {
	win := testWindowWithSurface(&window{})
	content := newTestWidget()
	win.SetWidget(content)

	unmounted := false
	content.ConnectUnmount(func() { unmounted = true })

	// Migrating the widget from the window into the popover must emit the
	// unmount notification (adoptWidget semantics) and re-mount under the
	// popover.
	p := &popover{}
	p.SetWidget(content)
	if !unmounted {
		t.Fatal("migration to popover should emit unmount on the old root")
	}
	if content.Root() != p {
		t.Fatalf("content should be mounted under the popover, got %v", content.Root())
	}
	if p.Widget() != content {
		t.Fatal("popover content should be set")
	}
}

func TestPopoverConnectClosedFiresOnHideOnce(t *testing.T) {
	p := NewPopover(newTestWidget(), nil)
	closed := 0
	p.ConnectClosed(func() { closed++ })

	p.Hide() // not visible yet → no fire
	if closed != 0 {
		t.Fatalf("Hide on a hidden popover must not fire closed, got %d", closed)
	}
}

func TestPopoverSurfacePreservesBorderSides(t *testing.T) {
	p, _, _, app := placementFixture(t, geometry.Rect(0, 0, 200, 150))
	for _, sides := range []style.BorderSides{0, style.BorderTop | style.BorderBottom, style.BorderAll} {
		app.style = style.Sheet(style.Name("popover").BorderWidth(2).BorderColor(color.Black).BorderSides(sides))
		_, border, _ := p.surfaceStyle()
		if got, set := border.BorderSides(); !set || got != sides {
			t.Fatalf("宿主重建边框样式丢失边集合：%v/%v，期望%v", got, set, sides)
		}
	}
}

// 非模态自动关闭不抢键盘，外部按下关闭后仍沿原窗口事件路径完成一次点击。
func TestPopoverModelessDismissalAndExplicitKeys(t *testing.T) {
	p, _, _, app := placementFixture(t, geometry.Rect(0, 0, 1000, 800))
	w := app.windows[0]
	anchor := p.anchor
	w.SetWidget(nil)
	root := newTestWidget()
	root.AddChild(anchor)
	other := NewButton()
	root.AddChild(other)
	w.SetWidget(root)
	root.Arrange(geometry.Rect(0, 0, 400, 300))
	anchor.Arrange(geometry.Rect(20, 20, 100, 40))
	other.Arrange(geometry.Rect(200, 20, 100, 40))
	t.Cleanup(w.Destroy)
	keys, ownerKeys, clicks, dismissed := 0, 0, 0, 0
	key := NewKeyEventController()
	key.ConnectKeyDown(func(EventContext, events.KeyEvent) { keys++ })
	p.Widget().AddEventController(key)
	p.Widget().SetFocusable(true)
	ownerKey := NewKeyEventController()
	ownerKey.ConnectKeyDown(func(EventContext, events.KeyEvent) { ownerKeys++ })
	anchor.AddEventController(ownerKey)
	other.ConnectClicked(func() { clicks++ })
	p.ConnectDismissRequest(func() { dismissed++; p.Hide() })
	show := func() {
		t.Helper()
		if err := p.Show(); err != nil {
			t.Fatal(err)
		}
		p.SetFocusedWidget(p.Widget())
	}
	if p.AutoDismiss() || p.Modal() {
		t.Fatal("默认策略不是被动非模态面")
	}
	p.SetAutoDismiss(true)
	show()
	w.SetFocusedWidget(anchor)
	if err := w.DispatchEvent(shortcutPress(events.KeyQ, 0)); err != nil {
		t.Fatal(err)
	}
	if ownerKeys != 1 || keys != 0 {
		t.Fatal("非模态面自动抢走了键盘")
	}
	if err := p.DispatchEvent(shortcutPress(events.KeyQ, 0)); err != nil {
		t.Fatal(err)
	}
	if keys != 1 {
		t.Fatal("显式键盘转发没有进入普通控制器")
	}
	press := func(x float32) {
		t.Helper()
		for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
			if err := w.DispatchEvent(events.PointerEvent{EventType: kind, Position: geometry.Point{X: x, Y: 30}, Button: events.PointerButtonLeft}); err != nil {
				t.Fatal(err)
			}
		}
	}
	press(40)
	if !p.Visible() || dismissed != 0 {
		t.Fatal("锚点被当作外部点击")
	}
	press(220)
	if p.Visible() || clicks != 1 || dismissed != 1 {
		t.Fatal("外部关闭吞掉、重复或阻止了原点击", clicks, dismissed)
	}
	if err := p.DispatchEvent(shortcutPress(events.KeyQ, 0)); err == nil {
		t.Fatal("隐藏面仍接受键盘")
	}
	show()
	handled := false
	e := shortcutPress(events.KeyEscape, 0)
	e.Handled = &handled
	if err := w.DispatchEvent(e); err != nil {
		t.Fatal(err)
	}
	// AutoDismiss 的 Esc 现在先交给内容，未处理才请求关闭。
	if p.Visible() || dismissed != 2 || !handled || keys != 2 {
		t.Fatal("Esc 没有关闭并消费")
	}
	p.SetAutoDismiss(false)
	p.SetModal(true)
	show()
	if err := w.DispatchEvent(shortcutPress(events.KeyEscape, 0)); err != nil {
		t.Fatal(err)
	}
	if !p.Visible() || dismissed != 2 || keys != 3 {
		t.Fatal("持久模态面的 Esc 没有交给内容", keys, dismissed)
	}
	p.SetAutoDismiss(true)
	if err := w.DispatchEvent(events.FocusEvent{Focused: false}); err != nil {
		t.Fatal(err)
	}
	if p.Visible() || dismissed != 3 {
		t.Fatal("失焦没有请求关闭")
	}
}

// 原生面被替换后，旧面的延迟释放不能撤销新的模态登记。
func TestPopoverResignKeepsReplacementTarget(t *testing.T) {
	w := testWindowWithSurface(&window{})
	old, next := &popover{owner: w}, &popover{owner: w}
	w.SetModalTarget(next)
	old.resignModalTarget()
	if w.modalTarget != next {
		t.Fatal("旧面清除了替换目标")
	}
	next.resignModalTarget()
	if w.modalTarget != nil {
		t.Fatal("当前目标没有正确撤销")
	}
}

func TestPopoverModelessDismissalReentry(t *testing.T) {
	for _, action := range []string{"new surface", "new root", "destroy"} {
		t.Run(action, func(t *testing.T) {
			p, _, _, app := placementFixture(t, geometry.Rect(0, 0, 1000, 800))
			w := app.windows[0]
			w.SetWidget(nil)
			root := newTestWidget()
			root.AddChild(p.anchor)
			button := NewButton()
			root.AddChild(button)
			w.SetWidget(root)
			root.Arrange(geometry.Rect(0, 0, 400, 300))
			p.anchor.Arrange(geometry.Rect(0, 0, 80, 40))
			button.Arrange(geometry.Rect(150, 0, 100, 40))
			t.Cleanup(w.Destroy)
			p.SetAutoDismiss(true)
			var replacement Popover
			p.ConnectDismissRequest(func() {
				p.Hide()
				switch action {
				case "new surface":
					replacement = NewPopover(p.anchor, nil)
					replacement.SetWidget(&popoverMeasureWidget{size: geometry.Size{Width: 20, Height: 20}})
					if err := replacement.Show(); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(replacement.Destroy)
				case "new root":
					w.SetWidget(button)
					button.Arrange(geometry.Rect(0, 0, 400, 300))
				case "destroy":
					w.Destroy()
				}
			})
			if err := p.Show(); err != nil {
				t.Fatal(err)
			}
			if err := w.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Position: geometry.Point{X: 180, Y: 20}, Button: events.PointerButtonLeft}); err != nil {
				t.Fatal(err)
			}
			if button.State().Has(style.Pressed) {
				t.Fatal("旧事件泄漏给关闭回调改变后的界面")
			}
			if replacement != nil && !replacement.Visible() {
				t.Fatal("新面被原始关闭事件再次关闭")
			}
		})
	}
}

func TestPopoverReopenedDuringFocusReset(t *testing.T) {
	p, _, _, app := placementFixture(t, geometry.Rect(0, 0, 1000, 800))
	p.SetModal(true)
	p.SetAutoDismiss(true)
	p.Widget().SetFocusable(true)
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	p.SetFocusedWidget(p.Widget())
	reopen, closed := true, 0
	p.ConnectClosed(func() { closed++ })
	p.Widget().ConnectFocused(func(focused bool) {
		if !focused && reopen {
			reopen = false
			if err := p.Show(); err != nil {
				t.Fatal(err)
			}
		}
	})
	p.Hide()
	w := app.windows[0]
	if !p.Visible() || w.modalTarget != p || len(w.Popovers()) != 1 || closed != 1 {
		t.Fatal("旧 Hide 清理了回调重新显示的面", p.Visible(), closed)
	}
	p.Hide()
	if p.Visible() || w.modalTarget != nil || closed != 2 {
		t.Fatal("重新显示后的正常关闭没有完成")
	}
}

// 使用原生浮窗替身，所有鼠标输入仍经过 Popover.DispatchEvent。
func popoverHoverFixture(t *testing.T) (*popover, *testWidget, *testWidget) {
	t.Helper()
	p, _, _, _ := placementFixture(t, geometry.Rect(0, 0, 1000, 800))
	root, first, second := newTestWidget(), newTestWidget(), newTestWidget()
	root.SetMinSize(geometry.Size{Width: 100, Height: 100})
	root.AddChild(first)
	root.AddChild(second)
	for _, row := range []*testWidget{first, second} {
		row.AddEventController(NewMotionEventController())
	}
	p.SetWidget(root)
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	first.Arrange(geometry.Rect(5, 5, 90, 35))
	second.Arrange(geometry.Rect(5, 50, 90, 35))
	return p, first, second
}

func TestPopoverCloseClearsHover(t *testing.T) {
	for _, release := range []bool{false, true} {
		t.Run(fmt.Sprintf("release=%t", release), func(t *testing.T) {
			p, first, second := popoverHoverFixture(t)
			motion := first.EventControllers()[0].(*MotionEventController)
			second.SetState(style.Selected, true)
			leaves := 0
			motion.ConnectContainsHover(func(inside bool) {
				if !inside {
					leaves++
				}
			})
			for range 3 {
				if err := p.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: first.windowRect().Center()}); err != nil {
					t.Fatal(err)
				}
				if !first.State().Has(style.Hovered) || !motion.Hover() || !motion.ContainsHover() {
					t.Fatal("没有建立初始悬停")
				}
				before := leaves
				if release {
					p.releaseNative()
				} else {
					p.Hide()
				}
				if p.Widget().State().Has(style.Hovered) || first.Snapshot().Hovered || second.Snapshot().Hovered ||
					motion.Hover() || motion.ContainsHover() || leaves != before+1 || !second.State().Has(style.Selected) {
					t.Fatal("关闭后控件、快照、控制器的悬停状态不一致，或错误清除了选中状态")
				}
				if err := p.Show(); err != nil {
					t.Fatal(err)
				}
				if err := p.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: second.windowRect().Center()}); err != nil {
					t.Fatal(err)
				}
				if first.Snapshot().Hovered || !second.Snapshot().Hovered || !second.Snapshot().Selected {
					t.Fatal("重新显示后保留旧行悬停，或丢失当前行的正常悬停／选中")
				}
			}
		})
	}
}

func TestPopoverHoverLeaveReentry(t *testing.T) {
	for _, action := range []string{"reopen", "replace", "destroy"} {
		t.Run(action, func(t *testing.T) {
			p, first, second := popoverHoverFixture(t)
			motion := first.EventControllers()[0].(*MotionEventController)
			armed, closed := true, 0
			p.ConnectClosed(func() { closed++ })
			motion.ConnectHover(func(inside bool) {
				if inside || !armed {
					return
				}
				armed = false
				if action == "destroy" {
					p.Destroy()
					return
				}
				if action == "replace" {
					second = newTestWidget()
					second.SetMinSize(geometry.Size{Width: 100, Height: 100})
					second.AddEventController(NewMotionEventController())
					p.SetWidget(second)
				}
				if err := p.Show(); err != nil {
					t.Fatal(err)
				}
				if action == "replace" {
					second.Arrange(geometry.Rect(0, 0, 100, 100))
				}
				if err := p.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: second.windowRect().Center()}); err != nil {
					t.Fatal(err)
				}
			})
			if err := p.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: first.windowRect().Center()}); err != nil {
				t.Fatal(err)
			}
			p.Hide()
			if action == "destroy" {
				if !p.destroyed || p.Visible() || first.State().Has(style.Hovered) {
					t.Fatal("离开回调销毁后仍保留悬停或浮窗")
				}
				return
			}
			currentMotion := second.EventControllers()[0].(*MotionEventController)
			if !p.Visible() || closed != 1 || !second.State().Has(style.Hovered) || !currentMotion.ContainsHover() || len(p.dispatcher.hoverPath) == 0 {
				t.Fatal("旧的关闭清理覆盖了离开回调中新显示表面的输入状态")
			}
			p.Hide()
			if p.Visible() || second.State().Has(style.Hovered) || currentMotion.ContainsHover() {
				t.Fatal("重新显示后的正常关闭未清理悬停")
			}
		})
	}
}

// 登记模态输入会取消 owner 手势，取消回调可能打开另一表面。
func TestPopoverModalShowReentryDoesNotOwnReplacement(t *testing.T) {
	p, _, _, app := placementFixture(t, geometry.Rect(0, 0, 1000, 800))
	w := app.windows[0]
	w.focused = true
	w.root.Arrange(geometry.Rect(0, 0, 100, 60))
	replacement := NewPopover(p.anchor, nil).(*popover)
	replacement.SetModal(true)
	replacement.SetAutoDismiss(true)
	replacement.SetWidget(&popoverMeasureWidget{size: geometry.Size{Width: 20, Height: 20}})
	t.Cleanup(replacement.Destroy)
	var click *ClickEventController
	for _, controller := range p.anchor.EventControllers() {
		if c, ok := controller.(*ClickEventController); ok {
			click = c
			break
		}
	}
	if click == nil {
		t.Fatal("anchor controller missing")
	}
	reentered := false
	click.ConnectPressedChanged(func(pressed bool) {
		if !pressed && !reentered {
			reentered = true
			if err := replacement.Show(); err != nil {
				t.Fatal(err)
			}
		}
	})
	if err := w.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Position: geometry.Point{X: 20, Y: 20}, Button: events.PointerButtonLeft}); err != nil {
		t.Fatal(err)
	}
	if !click.Pressed() || reentered || w.dispatcher.gesture == nil {
		t.Fatalf("无效的按下初态：pressed=%v reentered=%v", click.Pressed(), reentered)
	}
	p.SetModal(true)
	p.SetAutoDismiss(true)
	if err := p.Show(); err == nil {
		t.Fatalf("Show 在登记回调替换输入目标后仍报告成功：reentered=%v current=%p expected=%p", reentered, w.modalTarget, replacement)
	}
	if !reentered || !replacement.Visible() || w.modalTarget != replacement || p.Visible() {
		t.Fatal("失效的 Show 覆盖或保留了旧表面")
	}
}

func TestPopoverDismissalSkipsReopenedPresentation(t *testing.T) {
	p, _, _, app := placementFixture(t, geometry.Rect(0, 0, 1000, 800))
	p.SetAutoDismiss(true)
	p.ConnectDismissRequest(p.Hide)
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	next := NewPopover(p.anchor, nil)
	next.SetAutoDismiss(true)
	next.SetWidget(&popoverMeasureWidget{size: geometry.Size{Width: 20, Height: 20}})
	t.Cleanup(next.Destroy)
	next.ConnectDismissRequest(func() {
		next.Hide()
		p.Hide()
		if err := p.Show(); err != nil {
			t.Fatal(err)
		}
	})
	if err := next.Show(); err != nil {
		t.Fatal(err)
	}
	if err := app.windows[0].DispatchEvent(events.FocusEvent{Focused: false}); err != nil {
		t.Fatal(err)
	}
	if !p.Visible() || next.Visible() {
		t.Fatal("原事件关闭了回调重新显示的面")
	}
}

func TestPopoverCannotOpenFromOwnerDestroy(t *testing.T) {
	p, _, _, app := placementFixture(t, geometry.Rect(0, 0, 1000, 800))
	called := false
	app.windows[0].ConnectDestroy(func() {
		called = true
		if err := p.Show(); err == nil {
			t.Fatal("销毁中的 owner 仍创建了原生面")
		}
	})
	app.windows[0].Destroy()
	if !called || p.Visible() || p.platformPopup != nil {
		t.Fatal("销毁期间重入 Show 遗留资源")
	}
}

// 原生透明面仍接收阴影区域内的指针；非模态 Bottom 不得挡住锚点按钮。
func TestPopoverModelessAnchorExcludesShadow(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		anchorY                    float32
		constrained                bool
		wantTop, wantBottom, wantY float32
	}{
		{"below", 40, true, 0, 24, 72},
		{"above", 160, true, 16, 0, 104},
		{"unconstrained", 40, false, 0, 24, 72},
		{"overlapping", 80, true, 16, 24, 64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			height := float32(40)
			if tc.name == "overlapping" {
				height = 180
			}
			snapshot := popoverPlacementSnapshot{anchor: geometry.Rect(30, tc.anchorY, 80, 32), placement: PopoverPlacementBottom, workArea: geometry.Rect(0, 0, 300, 220), constrained: tc.constrained}
			i := snapshot.trimAnchorShadow(geometry.Size{Width: 100, Height: height}, popoverInsets{20, 16, 20, 24})
			if i.top != tc.wantTop || i.bottom != tc.wantBottom {
				t.Fatal("阴影裁剪错误", i)
			}
			pos := snapshot.position(geometry.Size{Width: 140, Height: height + i.top + i.bottom}, i)
			if tc.name == "overlapping" {
				if snapshot.anchorSide != 0 {
					t.Fatal("超高内容改变原滑移规则")
				}
				return
			}
			if pos.Y != tc.wantY {
				t.Fatal("主体没有贴住锚点", pos)
			}
			if snapshot.anchorSide > 0 && pos.Y < tc.anchorY+32 || snapshot.anchorSide < 0 && pos.Y+height+i.top+i.bottom > tc.anchorY {
				t.Fatal("原生表面挡住锚点", pos, i)
			}
			// 向外取整多半像素后仍不侵入锚点。
			rounded := snapshot.position(geometry.Size{Width: 140.5, Height: height + i.top + i.bottom + .5}, i)
			if snapshot.anchorSide > 0 && rounded.Y < tc.anchorY+32 || snapshot.anchorSide < 0 && rounded.Y+height+i.top+i.bottom+.5 > tc.anchorY+.5 {
				t.Fatal("原生取整改变锚点关系", rounded)
			}
		})
	}
}

func TestPopoverModelessNativeShadowDoesNotCoverAnchor(t *testing.T) {
	for _, above := range []bool{false, true} {
		t.Run(fmt.Sprint(above), func(t *testing.T) {
			p, _, _, app := placementFixture(t, geometry.Rect(0, 0, 400, 300))
			p.transparent = true
			app.style = style.Sheet(style.Name("popover").Shadow(style.Shadow{Color: color.Black, BlurRadius: 16, Offset: geometry.Point{Y: 4}}))
			anchor := geometry.Rect(80, 40, 60, 32)
			if above {
				anchor.Y = 260 // 主体本身放不下时才翻转，不能再依赖阴影触发。
			}
			p.anchor.Arrange(anchor)
			p.SetPlacement(PopoverPlacementBottom)
			if err := p.Show(); err != nil {
				t.Fatal(err)
			}
			settlePlacement(t, p)
			if above {
				if p.insets.bottom != 0 || p.requestedPosition.Y+p.height > anchor.Y {
					t.Fatal("原生阴影挡住上方菜单的锚点", p.requestedPosition, p.insets, p.height)
				}
			} else if p.insets.top != 0 || p.requestedPosition.Y < anchor.Y+anchor.Height {
				t.Fatal("原生阴影挡住下方菜单的锚点", p.requestedPosition, p.insets)
			}
		})
	}
}

func TestPopoverNativeTerminationPreservesContent(t *testing.T) {
	for _, inFrame := range []bool{false, true} {
		t.Run(fmt.Sprint(inFrame), func(t *testing.T) {
			p, _, plat, _ := placementFixture(t, geometry.Rect(0, 0, 1000, 1000))
			p.SetModal(true)
			content := p.Widget()
			if err := p.Show(); err != nil {
				t.Fatal(err)
			}
			settlePlacement(t, p)
			old := plat.natives[0]
			backend := &lifecyclePainter{}
			p.painter = backend
			closed := 0
			p.ConnectClosed(func() { closed++ })
			terminate := func() { old.handler(events.DestroyEvent{}) }
			if inFrame {
				backend.onBegin = terminate
				p.paint()
			} else {
				terminate()
			}
			old.handler(events.DestroyEvent{})
			old.handler(events.PaintEvent{})
			if p.Visible() || p.platformPopup != nil || p.painter != nil || p.destroyed || content.base().destroyed || p.Widget() != content || closed != 1 || !old.destroyed {
				t.Fatalf("终止后状态错误: visible=%v destroyed=%v closed=%d", p.Visible(), p.destroyed, closed)
			}
			want := []string{"destroy"}
			if inFrame {
				want = []string{"begin", "end", "destroy"}
			}
			if fmt.Sprint(backend.calls) != fmt.Sprint(want) {
				t.Fatalf("calls=%v want=%v", backend.calls, want)
			}
			if err := p.Show(); err != nil {
				t.Fatal(err)
			}
			if len(plat.natives) != 2 || p.Widget() != content || !p.Visible() {
				t.Fatal("内容没有在新原生表面中复用")
			}
			old.handler(events.DestroyEvent{})
			old.handler(events.PaintEvent{})
			if !p.Visible() || closed != 1 || p.platformPopup != plat.natives[1] {
				t.Fatal("旧表面的事件污染了重新打开的 Popover")
			}
			p.Destroy()
			if p.Widget() != nil || content.Root() != nil || closed != 2 {
				t.Fatal("最终 Destroy 没有解除内容的挂载")
			}
		})
	}
}

func (*recordingPlatformPopup) SurfaceState() platform.SurfaceState {
	return platform.SurfaceState{Ready: true, Generation: 1}
}

func TestPopoverFocusPolicyDoesNotInheritFromAnchor(t *testing.T) {
	root, anchor := newTestWidget(), newTestWidget()
	root.AddChild(anchor)
	root.SetCanContainFocus(false)
	win := testWindowWithSurface(&window{})
	win.SetWidget(root)
	p := &popover{anchor: anchor}
	popupRoot, child := newTestWidget(), newTestWidget()
	child.SetFocusable(true)
	popupRoot.AddChild(child)
	p.SetWidget(popupRoot)
	t.Cleanup(p.Destroy)
	t.Cleanup(win.Destroy)
	if !p.SetFocusedWidget(child) {
		t.Fatal("Popover从锚点错误继承了准入限制")
	}
	popupRoot.SetCanContainFocus(false)
	if p.FocusedWidget() != nil || p.SetFocusedWidget(child) || child.Focused() || popupRoot.ContainsFocus() {
		t.Fatal("Popover自身焦点准入没有生效")
	}
}

// --- Popover 锚点切换与资源复用 ---

func ordinaryReuseFixture(t *testing.T) (*popover, *window, *transientTestPlatform, *testWidget, *testWidget) {
	t.Helper()
	w, plat, makeClient := transientFixture(t)
	a, b := newTestWidget(), newTestWidget()
	tree := newTestWidget()
	tree.AddChild(a)
	tree.AddChild(b)
	w.SetWidget(tree)
	tree.Arrange(geometry.Rect(0, 0, 400, 300))
	a.Arrange(geometry.Rect(10, 20, 80, 30))
	b.Arrange(geometry.Rect(210, 40, 90, 30))
	p := NewPopover(a, nil).(*popover)
	p.SetWidget(makeClient(nil).Widget())
	t.Cleanup(p.Destroy)
	return p, w, plat, a, b
}

func TestPopoverSetAnchorReusesNativeWithinSurface(t *testing.T) {
	p, w, plat, a, b := ordinaryReuseFixture(t)
	mounts, unmounts, closed := 0, 0, 0
	content := newTestWidget()
	content.SetMinSize(geometry.Size{Width: 60, Height: 30})
	content.ConnectMount(func() { mounts++ })
	content.ConnectUnmount(func() { unmounts++ })
	p.SetWidget(content)
	p.ConnectClosed(func() { closed++ })
	p.SetPlacement(PopoverPlacementBottom)
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	native, painter := p.platformPopup, p.painter
	if len(w.ownedPopovers) != 1 {
		t.Fatal("可见实例没有登记资源归属")
	}
	p.SetAnchor(a)
	if !p.Visible() || closed != 0 {
		t.Fatal("相同锚点改变了显示")
	}
	p.SetAnchor(b)
	if p.Visible() || p.Anchor() != b || closed != 1 || len(plat.natives) != 1 {
		t.Fatal("SetAnchor 没有只结束旧显示并保留资源")
	}
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	if p.platformPopup != native || p.painter != painter || content.Root() != p || mounts != 1 || unmounts != 0 {
		t.Fatal("同 Surface 换锚点重建资源或重复挂载")
	}
	if p.requestedPosition != (geometry.Point{X: 210, Y: 70}) {
		t.Fatal("没有使用新锚点定位", p.requestedPosition)
	}
	w.Widget().(*testWidget).RemoveChild(a)
	if !p.Visible() {
		t.Fatal("旧锚点卸载关闭了新显示")
	}
	p.Hide()
	if len(w.Popovers()) != 0 || len(w.ownedPopovers) != 1 {
		t.Fatal("可见枚举与隐藏资源归属混淆")
	}
	w.Destroy()
	if !plat.natives[0].destroyed || p.painter != nil || content.Root() != nil || len(w.ownedPopovers) != 0 {
		t.Fatal("Surface 未释放隐藏普通 Popover")
	}
}

func TestPopoverSetAnchorRebuildsAcrossSurfaces(t *testing.T) {
	p, old, plat, _, _ := ordinaryReuseFixture(t)
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	native, painter := p.platformPopup, p.painter
	other := testWindowWithSurface(&window{surfaceBase: surfaceBase{platformWindow: &placementWindow{area: geometry.Rect(0, 0, 1000, 800)}}})
	t.Cleanup(other.Destroy)
	anchor := newTestWidget()
	other.SetWidget(anchor)
	anchor.Arrange(geometry.Rect(50, 60, 40, 30))
	p.SetAnchor(anchor)
	if p.platformPopup != native || len(plat.natives) != 1 {
		t.Fatal("SetAnchor 提前重建了原生窗口")
	}
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	if p.platformPopup == native || p.painter == painter || !plat.natives[0].destroyed || len(plat.natives) != 2 || p.Widget().Root() != p {
		t.Fatal("跨 Surface 未释放重建资源并保留 Go Root")
	}
	if len(old.ownedPopovers) != 0 || len(other.ownedPopovers) != 1 {
		t.Fatal("资源仍登记在旧 Surface")
	}
	current := p.platformPopup
	old.Destroy()
	if p.platformPopup != current || !p.Visible() || plat.natives[1].destroyed {
		t.Fatal("旧 Surface 销毁影响了新归属")
	}
}

func TestPopoverAnchorUnmountRetainsUntilOwnerEnds(t *testing.T) {
	p, w, plat, a, _ := ordinaryReuseFixture(t)
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	native, painter := p.platformPopup, p.painter
	tree := w.Widget().(*testWidget)
	tree.RemoveChild(a)
	if p.Visible() || p.platformPopup != native || p.painter != painter || plat.natives[0].destroyed {
		t.Fatal("锚点卸载销毁了仍有效的资源")
	}
	tree.AddChild(a)
	a.Arrange(geometry.Rect(10, 20, 80, 30))
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	if p.platformPopup != native || p.painter != painter {
		t.Fatal("重新挂载后没有复用")
	}
	w.Destroy()
	other := testWindowWithSurface(&window{surfaceBase: surfaceBase{platformWindow: &placementWindow{area: geometry.Rect(0, 0, 1000, 800)}}})
	t.Cleanup(other.Destroy)
	anchor := newTestWidget()
	other.SetWidget(anchor)
	anchor.Arrange(geometry.Rect(10, 10, 50, 30))
	p.SetAnchor(anchor)
	if err := p.Show(); err != nil || len(plat.natives) != 2 {
		t.Fatal("原 Surface 结束后 Popover 对象不能迁移", err)
	}
}

func TestPopoverSetAnchorClosedReentryKeepsLaterRequest(t *testing.T) {
	p, _, _, _, b := ordinaryReuseFixture(t)
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	handle := p.ConnectClosed(func() {
		if p.Anchor() != b {
			t.Fatal("Closed 看不到新锚点请求")
		}
		if err := p.Show(); err != nil {
			t.Fatal(err)
		}
	})
	t.Cleanup(handle.Disconnect)
	p.SetAnchor(b)
	if !p.Visible() || p.Anchor() != b {
		t.Fatal("SetAnchor 的旧清理覆盖了回调重开")
	}
}

func TestPopoverPreparationFailureRetainsValidNative(t *testing.T) {
	p, w, plat, _, _ := ordinaryReuseFixture(t)
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	native, painter := p.platformPopup, p.painter
	p.Hide()
	w.platformWindow.(*placementWindow).err = platform.ErrUnavailable
	if err := p.Show(); !errors.Is(err, platform.ErrUnavailable) || p.Visible() {
		t.Fatal("查询失败仍显示或错误丢失", err)
	}
	if p.platformPopup != native || p.painter != painter || plat.natives[0].destroyed {
		t.Fatal("普通准备失败销毁了有效资源")
	}
}

func TestPopoverContentUnmountCanDestroyOwner(t *testing.T) {
	p, w, plat, _, _ := ordinaryReuseFixture(t)
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	p.Widget().ConnectUnmount(func() {
		w.Destroy()
		if p.painter == nil || plat.natives[0].destroyed {
			t.Fatal("内容卸载回调尚未退出就释放了原生资源")
		}
	})
	p.SetWidget(newTestWidget())
	if w.platformWindow != nil || !plat.natives[0].destroyed || p.platformPopup != nil {
		t.Fatal("普通弹层内容卸载后未安全完成资源清理")
	}
}

func TestPopoverRejectsAnchorCycle(t *testing.T) {
	p, _, _, _, _ := ordinaryReuseFixture(t)
	p.SetAnchor(p.Widget())
	if err := p.Show(); err == nil {
		t.Fatal("接受了以自身内容为锚点的循环")
	}
}

func TestSharedSetAnchorSelectsExistingDestinationCache(t *testing.T) {
	w, plat, makeClient := transientFixture(t)
	a := makeClient(nil)
	if err := a.Show(); err != nil {
		t.Fatal(err)
	}
	source := w.shared.popup
	nativeSource := source.platformPopup
	other := testWindowWithSurface(&window{surfaceBase: surfaceBase{platformWindow: &placementWindow{area: geometry.Rect(0, 0, 1000, 800)}}})
	t.Cleanup(other.Destroy)
	anchor := newTestWidget()
	other.SetWidget(anchor)
	anchor.Arrange(geometry.Rect(10, 10, 50, 30))
	b := makeClient(anchor)
	if err := b.Show(); err != nil {
		t.Fatal(err)
	}
	destination := other.shared.popup
	nativeDestination := destination.platformPopup
	a.SetAnchor(anchor)
	if a.Visible() || source.platformPopup != nativeSource || plat.natives[0].destroyed {
		t.Fatal("共享客户端换锚点销毁或迁移了源缓存")
	}
	if err := a.Show(); err != nil {
		t.Fatal(err)
	}
	if b.Visible() || a.Widget().Root() != destination || destination.platformPopup != nativeDestination || len(plat.natives) != 2 {
		t.Fatal("未选择目标 Surface 已存在的缓存")
	}
	c := makeClient(nil)
	if err := c.Show(); err != nil {
		t.Fatal(err)
	}
	if c.Widget().Root() != source || source.platformPopup != nativeSource || len(plat.natives) != 2 {
		t.Fatal("源 Surface 不能继续复用缓存")
	}
}

// --- Popover 布局、样式与隐藏状态失效 ---

type popoverStyleMeasureWidget struct {
	WidgetBase
	measures int
}

func (w *popoverStyleMeasureWidget) Measure(c layout.Constraint) layout.Measurement {
	w.measures++
	size, _ := WidgetStyle(w, "popover-invalidation-content", style.PartDefault).FontSize()
	return layout.Measured(c.Clamp(geometry.Size{Width: 4 * size, Height: 2 * size}))
}

func popoverInvalidationSheet(size float32) style.StyleSheet {
	return style.Sheet(style.Name("popover-invalidation-content").FontSize(size))
}

func capturePopoverLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(previous) })
	return &output
}

func TestHiddenPopoverDefersStyleLayout(t *testing.T) {
	for _, state := range []string{
		"ordinary-retained", "ordinary-detached", "ordinary-no-anchor", "ordinary-other-surface",
		"shared-retained", "shared-evicted", "shared-detached",
	} {
		t.Run(state, func(t *testing.T) {
			w, plat, makeClient := transientFixture(t)
			App.SetStyleSheet(popoverInvalidationSheet(10))
			anchor := w.Widget()
			anchorRect := anchor.Rect()
			content := new(popoverStyleMeasureWidget)
			var client *sharedPopover
			var p *popover
			shared := strings.HasPrefix(state, "shared-")
			if shared {
				client = makeClient(anchor)
				client.SetWidget(content)
				if err := client.Show(); err != nil {
					t.Fatal(err)
				}
				p = w.shared.popup
			} else {
				p = NewPopover(anchor, nil).(*popover)
				t.Cleanup(p.Destroy)
				p.SetWidget(content)
				if err := p.Show(); err != nil {
					t.Fatal(err)
				}
			}
			p.Hide()
			restore := func() {}
			queries := []*placementWindow{w.platformWindow.(*placementWindow)}
			migrated := false
			switch state {
			case "ordinary-detached", "shared-detached":
				w.SetWidget(nil)
				restore = func() { w.SetWidget(anchor); anchor.Arrange(anchorRect) }
			case "ordinary-no-anchor":
				p.SetAnchor(nil)
				restore = func() { p.SetAnchor(anchor) }
			case "ordinary-other-surface":
				native := &placementWindow{area: geometry.Rect(0, 0, 1000, 800)}
				other := testWindowWithSurface(&window{surfaceBase: surfaceBase{platformWindow: native}})
				t.Cleanup(other.Destroy)
				otherAnchor := newTestWidget()
				other.SetWidget(otherAnchor)
				otherAnchor.Arrange(geometry.Rect(40, 50, 80, 30))
				p.SetAnchor(otherAnchor)
				queries = append(queries, native)
				migrated = true
			case "shared-evicted":
				client.Destroy()
			}
			native, painter := p.platformPopup, p.painter
			beforeMeasures := content.measures
			beforeQueries := 0
			for _, host := range queries {
				beforeQueries += len(host.points)
			}
			beforeSizes, beforePositions, beforePaints := len(plat.natives[0].sizes), len(plat.natives[0].positions), len(plat.painters[0].calls)
			output := capturePopoverLog(t)
			App.SetStyleSheet(popoverInvalidationSheet(20))
			afterQueries := 0
			for _, host := range queries {
				afterQueries += len(host.points)
			}
			if output.Len() != 0 || content.measures != beforeMeasures || afterQueries != beforeQueries {
				t.Fatalf("隐藏样式更新不应定位或测量：queries=%d→%d measures=%d→%d log=%s", beforeQueries, afterQueries, beforeMeasures, content.measures, output)
			}
			if p.Visible() || !p.layoutDirty || p.platformPopup != native || p.painter != painter || plat.natives[0].destroyed ||
				len(plat.natives[0].sizes) != beforeSizes || len(plat.natives[0].positions) != beforePositions || len(plat.painters[0].calls) != beforePaints {
				t.Fatal("隐藏样式更新操作了原生资源，或未保留待布局状态")
			}
			restore()
			if shared {
				if client.destroyed {
					client = makeClient(anchor)
					client.SetWidget(content)
				}
				if err := client.Show(); err != nil {
					t.Fatal(err)
				}
			} else if err := p.Show(); err != nil {
				t.Fatal(err)
			}
			if !p.Visible() || p.requestedSize != (geometry.Size{Width: 80, Height: 40}) || content.Rect().Size != p.bodyRect().Size {
				t.Fatalf("重开未采用新样式布局：requested=%v content=%v", p.requestedSize, content.Rect())
			}
			if migrated {
				if len(plat.natives) != 2 || !plat.natives[0].destroyed || p.platformPopup == native || p.painter == painter {
					t.Fatal("跨 Surface 重开未正确迁移资源")
				}
			} else if len(plat.natives) != 1 || p.platformPopup != native || p.painter != painter {
				t.Fatal("同 Surface 重开未复用原生资源")
			}
		})
	}
}

func TestHiddenPopoverSizeEventDefersPosition(t *testing.T) {
	p, _, plat, _, _ := ordinaryReuseFixture(t)
	p.SetPosition(geometry.Point{X: 900, Y: 650})
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	p.Hide()
	p.SetPosition(geometry.Point{X: 20, Y: 30})
	native := plat.natives[0]
	beforePositions, beforePaints := len(native.positions), native.paints
	native.handler(events.SizeEvent{Width: 200, Height: 100, PixelWidth: 300, PixelHeight: 150})
	if p.width != 200 || p.height != 100 || p.pixelWidth != 300 || p.pixelHeight != 150 || !p.layoutDirty {
		t.Fatal("隐藏时没有记录原生尺寸和待布局状态")
	}
	if len(native.positions) != beforePositions || native.paints != beforePaints {
		t.Fatal("隐藏尺寸通知使用旧定位快照移动或绘制了原生窗口")
	}
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	if p.requestedPosition != (geometry.Point{X: 30, Y: 50}) {
		t.Fatal("重开未按当前锚点和请求位置定位", p.requestedPosition)
	}
}

func TestVisiblePopoverStyleLayoutPreservesQueryErrors(t *testing.T) {
	p, w, _, _, _ := ordinaryReuseFixture(t)
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	query := w.platformWindow.(*placementWindow)
	query.err = platform.ErrUnavailable
	before := len(query.points)
	output := capturePopoverLog(t)
	App.SetStyleSheet(popoverInvalidationSheet(20))
	if !p.Visible() || len(query.points) <= before || !strings.Contains(output.String(), "goui: popup layout: popover work area:") {
		t.Fatalf("可见状态未保留真实工作区查询失败：visible=%t queries=%d→%d log=%s", p.Visible(), before, len(query.points), output)
	}
}

func TestHiddenPopoverConfigurationDefersNativeWork(t *testing.T) {
	p, w, plat, a, _ := ordinaryReuseFixture(t)
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	w.Widget().(*testWidget).RemoveChild(a)
	native, painter := p.platformPopup, p.painter
	query := w.platformWindow.(*placementWindow)
	beforeQueries, beforePaints := len(query.points), len(plat.painters[0].calls)
	beforeSizes, beforePositions := len(plat.natives[0].sizes), len(plat.natives[0].positions)
	output := capturePopoverLog(t)
	content := new(popoverStyleMeasureWidget)
	p.SetWidget(content)
	p.SetStyleName("updated-popover")
	p.SetPosition(geometry.Point{X: 50, Y: 60})
	p.SetPlacement(PopoverPlacementBottom)
	content.RequestLayout()
	content.RequestPaint()
	App.SetStyleSheet(popoverInvalidationSheet(20))
	if output.Len() != 0 || content.measures != 0 || len(query.points) != beforeQueries || len(plat.painters[0].calls) != beforePaints ||
		len(plat.natives[0].sizes) != beforeSizes || len(plat.natives[0].positions) != beforePositions {
		t.Fatal("隐藏配置或内容失效提前执行了原生工作", output)
	}
	w.Widget().(*testWidget).AddChild(a)
	a.Arrange(geometry.Rect(70, 80, 50, 30))
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	if p.platformPopup != native || p.painter != painter || content.Rect().Size != (geometry.Size{Width: 80, Height: 40}) ||
		p.requestedPosition != (geometry.Point{X: 70, Y: 110}) || p.StyleName() != "updated-popover" {
		t.Fatal("重新挂载未应用隐藏期间的内容和配置")
	}
}

func TestVisiblePopoverStyleUpdateResizesRetainedResources(t *testing.T) {
	w, plat, makeClient := transientFixture(t)
	App.SetStyleSheet(popoverInvalidationSheet(10))
	client := makeClient(nil)
	content := new(popoverStyleMeasureWidget)
	client.SetWidget(content)
	if err := client.Show(); err != nil {
		t.Fatal(err)
	}
	p := w.shared.popup
	native, painter := p.platformPopup, p.painter
	output := capturePopoverLog(t)
	App.SetStyleSheet(popoverInvalidationSheet(20))
	p.paint()
	if output.Len() != 0 || !client.Visible() || p.platformPopup != native || p.painter != painter || len(plat.natives) != 1 ||
		p.requestedSize != (geometry.Size{Width: 80, Height: 40}) || content.Rect().Size != p.bodyRect().Size {
		t.Fatal("可见样式更新未重新布局，或改变了资源归属", output)
	}
}

func TestPopoverEscapeContentPrecedesDismissal(t *testing.T) {
	p, _, _, _ := placementFixture(t, geometry.Rect(0, 0, 1000, 800))
	p.SetModal(true)
	p.SetAutoDismiss(true)
	controller := NewKeyEventController()
	controller.ConnectKeyDown(func(ctx EventContext, e events.KeyEvent) {
		if e.Key == events.KeyEscape {
			e.PreventDefault()
			ctx.StopPropagation()
		}
	})
	p.Widget().AddEventController(controller)
	dismissals := 0
	p.ConnectDismissRequest(func() { dismissals++; p.Hide() })
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	if err := p.DispatchEvent(shortcutPress(events.KeyEscape, 0)); err != nil {
		t.Fatal(err)
	}
	if !p.Visible() || dismissals != 0 {
		t.Fatal("内容已处理 Esc，仍触发默认关闭")
	}
}
