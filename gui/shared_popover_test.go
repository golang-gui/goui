package gui

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
)

type popoverPressController struct {
	EventControllerBase
	pressed bool
}

func (c *popoverPressController) HandleEvent(ctx EventContext) {
	if e, ok := ctx.Event().(events.PointerEvent); ok && e.EventType == events.PointerDown {
		c.pressed = true
	}
}

func (c *popoverPressController) Reset() { c.pressed = false }

func TestPopoverHideResetsNestedControllers(t *testing.T) {
	for _, kind := range []string{"ordinary", "shared"} {
		t.Run(kind, func(t *testing.T) {
			_, _, newPopup := transientFixture(t)
			client := newPopup(nil)
			var p interface {
				SetWidget(Widget)
				Show() error
				Hide()
				Destroy()
				DispatchEvent(events.Event) error
			} = client
			if kind == "ordinary" {
				p = NewPopover(client.Anchor(), nil)
				t.Cleanup(p.Destroy)
			}
			root, child := newTestWidget(), newTestWidget()
			root.SetMinSize(geometry.Size{Width: 100, Height: 60})
			root.AddChild(child)
			capture := &popoverPressController{EventControllerBase: NewEventControllerBase(PhaseCapture)}
			target := &popoverPressController{EventControllerBase: NewEventControllerBase(PhaseTarget)}
			root.AddEventController(capture)
			child.AddEventController(target)
			p.SetWidget(root)
			if err := p.Show(); err != nil {
				t.Fatal(err)
			}
			root.Arrange(geometry.Rect(0, 0, 100, 60))
			child.Arrange(geometry.Rect(5, 5, 90, 50))
			if err := p.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Position: child.windowRect().Center(), Button: events.PointerButtonLeft}); err != nil {
				t.Fatal(err)
			}
			if !capture.pressed || !target.pressed {
				t.Fatal("正常输入未到达父控件和子控件的控制器")
			}
			p.Hide()
			if capture.pressed || target.pressed {
				t.Fatal("关闭后父控件或子控件的控制器仍保留按压状态")
			}
		})
	}
}

func TestSharedPopoverOwnsOnlyClientState(t *testing.T) {
	w, plat, newPopup := transientFixture(t)
	p, q := newPopup(nil), newPopup(nil)
	a, b := p.Widget(), q.Widget()
	if _, ok := any(p).(Root); ok {
		t.Fatal("SharedPopover 意外实现 Root")
	}
	if a.Root() != nil || b.Root() != nil || w.shared != nil || len(plat.natives) != 0 {
		t.Fatal("构造或 SetWidget 提前挂载/分配宿主")
	}
	mounts, unmounts := 0, 0
	a.ConnectMount(func() { mounts++ })
	a.ConnectUnmount(func() { unmounts++ })
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	root := a.Root()
	if actual, ok := root.(*popover); !ok || actual != w.shared.popup {
		t.Fatal("共享内容没有使用 Surface 缓存的普通 Popover")
	}
	native, painter := testSharedCore(p).platformPopup, testSharedCore(p).painter
	p.Hide()
	if a.Root() != root || mounts != 1 || unmounts != 0 {
		t.Fatal("Hide 没有保留唯一缓存树")
	}
	before := plat.natives[0].paints
	a.RequestLayout()
	a.RequestPaint()
	if plat.natives[0].paints != before {
		t.Fatal("隐藏内容仍调度原生绘制")
	}
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	if a.Root() != root || mounts != 1 || unmounts != 0 {
		t.Fatal("同一内容重开重新挂载")
	}
	if err := q.Show(); err != nil {
		t.Fatal(err)
	}
	if a.Root() != nil || b.Root() != root || unmounts != 1 || testSharedCore(q).platformPopup != native || testSharedCore(q).painter != painter {
		t.Fatal("换内容没有复用唯一 GUI/原生宿主")
	}
	p.SetStyleName("old")
	p.SetModal(true)
	p.SetAutoDismiss(true)
	p.SetPosition(geometry.Point{X: 90, Y: 90})
	p.SetPlacement(PopoverPlacementBottom)
	if !q.Visible() || testSharedCore(q).styleName != "" || q.slot.popup.modal || q.slot.popup.position != (geometry.Point{}) {
		t.Fatal("失活客户端的配置污染当前显示")
	}
	p.Hide()
	p.Destroy()
	if !q.Visible() || len(plat.natives) != 1 || len(plat.painters) != 1 {
		t.Fatal("旧客户端销毁干扰当前客户端或重建资源")
	}
}

func TestPopoverTargetsExpirePerShow(t *testing.T) {
	w, _, newPopup := transientFixture(t)
	p := newPopup(nil)
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	first := w.Popovers()[0]
	root := p.Widget().Root()
	closed := 0
	first.ConnectClosed(func() { closed++ })
	p.Hide()
	if first.Visible() || first.Widget() != nil || closed != 1 {
		t.Fatal("关闭没有作废当次目标")
	}
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	second := w.Popovers()[0]
	if second == first || p.Widget().Root() != root {
		t.Fatal("重开未保留 Root 或复用了过期输入身份")
	}
	for _, e := range []events.Event{events.KeyEvent{EventType: events.KeyDown, Key: events.KeyEscape}, events.PointerEvent{EventType: events.PointerDown}} {
		if err := first.DispatchEvent(e); err == nil {
			t.Fatal("旧目标向重新显示的内容投递了输入")
		}
	}
	if err := first.CommitText("old"); err == nil || !second.Visible() || closed != 1 {
		t.Fatal("旧目标提交或关闭影响重新显示")
	}
}

func TestSharedPopoverInactiveFocusAndInput(t *testing.T) {
	_, _, newPopup := transientFixture(t)
	p, q := newPopup(nil), newPopup(nil)
	a := newTestWidget()
	a.SetFocusable(true)
	p.SetWidget(a)
	b := newTestWidget()
	b.SetFocusable(true)
	q.SetWidget(b)
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	if !p.SetFocusedWidget(a) || p.SetFocusedWidget(b) {
		t.Fatal("焦点未限制到当前子树")
	}
	if err := q.Show(); err != nil {
		t.Fatal(err)
	}
	if p.FocusedWidget() != nil || p.SetFocusedWidget(a) || p.SetFocusedWidget(b) {
		t.Fatal("旧客户端仍拥有焦点操作权限")
	}
	if err := p.DispatchEvent(events.PointerEvent{EventType: events.PointerDown}); err == nil {
		t.Fatal("旧客户端仍可投递指针输入")
	}
	if !q.SetFocusedWidget(b) || q.FocusedWidget() != b {
		t.Fatal("当前客户端无法设置自己的焦点")
	}
	root := b.Root()
	q.Hide()
	if root.(EventTarget).SetFocusedWidget(b) || b.Focused() {
		t.Fatal("隐藏的实际宿主仍可获取逻辑焦点")
	}
	if err := q.DispatchEvent(events.PointerEvent{EventType: events.PointerDown}); err == nil {
		t.Fatal("隐藏缓存仍接收指针输入")
	}
}

func TestSharedPopoverTargetClosedReentry(t *testing.T) {
	w, _, newPopup := transientFixture(t)
	p, next := newPopup(nil), newPopup(nil)
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	ref := w.Popovers()[0]
	ref.ConnectClosed(func() {
		if err := next.Show(); err != nil {
			t.Fatal(err)
		}
	})
	p.Hide()
	if !next.Visible() || ref.Visible() {
		t.Fatal("目标关闭通知未在完成清理后允许重入")
	}
}

func TestSharedPopoverRejectsAnchorInReplacedTree(t *testing.T) {
	_, _, newPopup := transientFixture(t)
	p := newPopup(nil)
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	nested := NewSharedPopover(p.Widget())
	if err := nested.Show(); err == nil || !p.Visible() {
		t.Fatal("替换卸载了新弹层自己的 anchor")
	}
}

func TestSharedPopoverSupersededIsRecognizable(t *testing.T) {
	_, _, newPopup := transientFixture(t)
	a, b, c := newPopup(nil), newPopup(nil), newPopup(nil)
	if err := a.Show(); err != nil {
		t.Fatal(err)
	}
	a.ConnectClosed(func() {
		if err := c.Show(); err != nil {
			t.Fatal(err)
		}
	})
	if err := b.Show(); !errors.Is(err, ErrPopoverSuperseded) || b.Visible() || !c.Visible() {
		t.Fatalf("显示取消未保留较晚操作或不能识别：%v", err)
	}
}

func TestSharedPopoverUnmountCallbackDestroysOwner(t *testing.T) {
	for _, replaceVisible := range []bool{false, true} {
		t.Run(map[bool]string{false: "switch-hidden", true: "replace-visible"}[replaceVisible], func(t *testing.T) {
			w, plat, newPopup := transientFixture(t)
			p := newPopup(nil)
			if err := p.Show(); err != nil {
				t.Fatal(err)
			}
			content := p.Widget()
			content.ConnectUnmount(w.Destroy)
			if replaceVisible {
				p.SetWidget(newTestWidget())
			} else {
				p.Hide()
				q := newPopup(nil)
				if err := q.Show(); err == nil {
					t.Fatal("卸载回调结束 owner 后仍显示成功")
				}
			}
			if w.platformWindow != nil || !plat.natives[0].destroyed || w.shared.retained != nil || w.shared.current != nil {
				t.Fatal("卸载回调结束后遗留窗口或共享宿主")
			}
		})
	}
}

// --- 共享资源、抢占与宿主生命周期 ---

type transientTestPlatform struct {
	*placementPlatform
	painters []*lifecyclePainter
}

func (p *transientTestPlatform) NewPainter(platform.Surface) (graphics.Painter, error) {
	backend := &lifecyclePainter{}
	p.painters = append(p.painters, backend)
	return backend, nil
}

func transientFixture(t *testing.T) (*window, *transientTestPlatform, func(Widget) *sharedPopover) {
	t.Helper()
	base, _, native, app := placementFixture(t, geometry.Rect(0, 0, 1000, 800))
	plat := &transientTestPlatform{placementPlatform: native}
	app.platform = plat
	w := app.windows[0]
	t.Cleanup(w.Destroy)
	return w, plat, func(anchor Widget) *sharedPopover {
		if anchor == nil {
			anchor = base.anchor
		}
		p := NewSharedPopover(anchor).(*sharedPopover)
		p.SetWidget(&popoverMeasureWidget{size: geometry.Size{Width: 100, Height: 40}})
		t.Cleanup(p.Destroy)
		return p
	}
}

func TestTransientPopupRetainsNativeAndPainterAcrossPresenters(t *testing.T) {
	w, plat, makePopup := transientFixture(t)
	var native platform.Popup
	var painter graphics.Painter
	frames := 0
	for range 30 {
		p := makePopup(nil)
		for range 3 {
			if err := p.Show(); err != nil {
				t.Fatal(err)
			}
			if native == nil {
				native, painter = testSharedCore(p).platformPopup, testSharedCore(p).painter
			}
			if testSharedCore(p).platformPopup != native || testSharedCore(p).painter != painter || len(plat.natives) != 1 || len(plat.painters) != 1 {
				t.Fatal("切换或重新打开创建了新的 Popup/Painter")
			}
			frames++ // 每次 Show 现在同步完成首帧，Hide 不应新增绘制或销毁。
			calls := len(plat.painters[0].calls)
			p.Hide()
			if p.Visible() || len(w.Popovers()) != 0 || w.shared.current != nil || plat.natives[0].destroyed || len(plat.painters[0].calls) != calls {
				t.Fatal("关闭未归还宿主或销毁了缓存资源")
			}
		}
		p.Destroy()
	}
	w.Destroy()
	if !plat.natives[0].destroyed || len(plat.painters[0].calls) != frames*4+1 || plat.painters[0].calls[frames*4] != "destroy" {
		t.Fatal("窗口销毁没有恰好释放一次共享资源")
	}
}

func TestTransientPopupOrdinaryAndWindowIsolation(t *testing.T) {
	w, plat, makePopup := transientFixture(t)
	p := makePopup(nil)
	ordinary := NewPopover(p.anchor, nil).(*popover)
	ordinary.SetWidget(newTestWidget())
	t.Cleanup(ordinary.Destroy)
	other := testWindowWithSurface(&window{surfaceBase: surfaceBase{platformWindow: &placementWindow{area: geometry.Rect(0, 0, 1000, 800)}}})
	anchor := newTestWidget()
	other.SetWidget(anchor)
	t.Cleanup(other.Destroy)
	q := makePopup(anchor)
	for _, presenter := range []interface{ Show() error }{ordinary, p, q} {
		if err := presenter.Show(); err != nil {
			t.Fatal(err)
		}
	}
	if len(plat.natives) != 3 || len(plat.painters) != 3 || len(w.Popovers()) != 2 || len(other.Popovers()) != 1 || testSharedCore(p).platformPopup == testSharedCore(q).platformPopup || testSharedCore(p).painter == testSharedCore(q).painter {
		t.Fatal("普通 Popover 或不同窗口错误共用宿主")
	}
	p.Destroy()
	if !ordinary.Visible() || !q.Visible() || plat.natives[1].destroyed {
		t.Fatal("销毁逻辑 presenter 影响独立浮窗或共享缓存")
	}
}

func TestTransientPopupClosedReentryKeepsLatestPresentation(t *testing.T) {
	for _, reopen := range []bool{false, true} {
		t.Run(map[bool]string{false: "third", true: "same"}[reopen], func(t *testing.T) {
			w, plat, makePopup := transientFixture(t)
			first, second, latest := makePopup(nil), makePopup(nil), makePopup(nil)
			if reopen {
				latest = first
			}
			if err := first.Show(); err != nil {
				t.Fatal(err)
			}
			closedHandle := first.ConnectClosed(func() {
				if len(w.Popovers()) != 0 {
					t.Fatal("Closed 仍观察到旧浮窗")
				}
				if err := latest.Show(); err != nil {
					t.Fatal(err)
				}
			})
			if err := second.Show(); err == nil || second.Visible() || !latest.Visible() || w.shared.current != latest {
				t.Fatal("旧 Show 覆盖了关闭回调打开的新浮窗")
			}
			second.Destroy()
			if !latest.Visible() || len(plat.natives) != 1 || len(plat.painters) != 1 {
				t.Fatal("失败 presenter 的清理干扰了新浮窗")
			}
			// 避免清理测试窗口时再次打开。
			closedHandle.Disconnect()
		})
	}
}

func TestTransientPopupUnmountMigrationAndGeometry(t *testing.T) {
	w, plat, makePopup := transientFixture(t)
	p := makePopup(nil)
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	native, painter := testSharedCore(p).platformPopup, testSharedCore(p).painter
	p.Hide()
	// 不改变原生尺寸、也不再发送 SizeEvent；新 presenter 必须继承实际像素尺寸。
	plat.natives[0].syncSize = false
	q := makePopup(nil)
	if err := q.Show(); err != nil || testSharedCore(q).width != 100 || testSharedCore(q).height != 40 || testSharedCore(q).pixelWidth != 100 || testSharedCore(q).pixelHeight != 40 {
		t.Fatalf("复用未继承实际尺寸：%v DIP=(%g,%g) pixels=(%g,%g)", err, testSharedCore(q).width, testSharedCore(q).height, testSharedCore(q).pixelWidth, testSharedCore(q).pixelHeight)
	}
	anchor := p.anchor
	w.SetWidget(nil)
	if q.Visible() || plat.natives[0].destroyed || w.shared.current != nil {
		t.Fatal("源控件卸载没有归还宿主或销毁了缓存")
	}
	w.SetWidget(anchor)
	if err := p.Show(); err != nil || testSharedCore(p).platformPopup != native || testSharedCore(p).painter != painter {
		t.Fatalf("同窗重新挂载没有复用：%v", err)
	}
	other := testWindowWithSurface(&window{surfaceBase: surfaceBase{platformWindow: &placementWindow{area: geometry.Rect(0, 0, 1000, 800)}}})
	t.Cleanup(other.Destroy)
	other.SetWidget(anchor)
	if err := p.Show(); err != nil || testSharedCore(p).platformPopup == native || testSharedCore(p).painter == painter || len(plat.natives) != 2 || plat.natives[0].destroyed {
		t.Fatalf("迁窗没有隔离资源：%v", err)
	}
}

func TestTransientPopupRoutesEventsToCurrentContent(t *testing.T) {
	_, plat, makePopup := transientFixture(t)
	first, second := makePopup(nil), makePopup(nil)
	calls := [2][]string{}
	for i, p := range []*sharedPopover{first, second} {
		content := newTestWidget()
		content.AddEventController(newRecordingController("pointer", PhaseTarget, &calls[i], nil))
		p.SetWidget(content)
		if err := p.Show(); err != nil {
			t.Fatal(err)
		}
		content.Arrange(geometry.Rect(0, 0, 100, 100))
	}
	plat.natives[0].handler(events.PointerEvent{EventType: events.PointerDown, Position: geometry.Point{X: 1, Y: 1}, Button: events.PointerButtonLeft})
	if len(calls[0]) != 0 || len(calls[1]) != 1 {
		t.Fatalf("复用后输入误投旧内容：%v", calls)
	}
	first.Destroy()
	if !second.Visible() || testSharedCore(second).platformPopup == nil {
		t.Fatal("旧 presenter 销毁影响新内容")
	}
}

func TestTransientPopupDefersOwnerDestroyUntilFrameEnds(t *testing.T) {
	w, plat, makePopup := transientFixture(t)
	p := makePopup(nil)
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	calls := len(plat.painters[0].calls)
	plat.painters[0].onBegin = func() {
		w.Destroy()
		if plat.natives[0].destroyed || w.platformWindow == nil || len(plat.painters[0].calls) != calls+1 {
			t.Fatal("活动帧中提前销毁共享资源或 owner")
		}
	}
	testSharedCore(p).paint()
	if !plat.natives[0].destroyed || w.platformWindow != nil || p.Visible() || w.shared.current != nil {
		t.Fatal("帧结束后未完成 owner 与宿主销毁")
	}
}

func TestTransientPopupFallbackOnceAndUnknownFailure(t *testing.T) {
	for _, failure := range []error{platform.ErrUnsupported, platform.ErrUnavailable, errors.New("fatal")} {
		t.Run(failure.Error(), func(t *testing.T) {
			_, plat, makePopup := transientFixture(t)
			plat.transparentErr = failure
			for range 3 {
				p := makePopup(nil)
				err := p.Show()
				if errors.Is(failure, platform.ErrUnsupported) || errors.Is(failure, platform.ErrUnavailable) {
					if err != nil || testSharedCore(p).Transparent() || len(plat.options) != 2 || len(plat.painters) != 1 {
						t.Fatalf("没有稳定复用不透明 fallback：%v", err)
					}
				} else if !errors.Is(err, failure) || len(plat.natives) != 0 || len(plat.painters) != 0 {
					t.Fatal("未知故障被忽略或残留资源")
				}
				p.Destroy()
			}
		})
	}
}

func TestTransientPopupTooltipNeverPreemptsMenu(t *testing.T) {
	w, _, a, _, q, plat := tooltipFixture(t)
	tooltipMove(t, w, 40, 40)
	q.advance(tooltipDelay) // 已排队的超时不能覆盖之后打开的菜单。
	menu := NewPopoverMenu(a)
	menu.SetMenu(NewSliceListModel([]*MenuItem{NewMenuItem("action", nil)}))
	if err := menu.ShowAt(geometry.Point{}); err != nil {
		t.Fatal(err)
	}
	for len(q.posts) > 0 {
		q.dispatch(t)
	}
	tooltipAdvance(t, q, 2*time.Second)
	if !menu.Visible() || w.tooltip.target != nil || w.tooltip.popup != nil || len(plat.natives) != 1 || w.shared.current != menu.popover {
		t.Fatal("过期 Tooltip 抢占了菜单")
	}
	menu.Hide()
	tooltipMove(t, w, 300, 200)
	tooltipMove(t, w, 40, 40)
	tooltipAdvance(t, q, tooltipDelay)
	if w.tooltip.popup == nil || len(plat.natives) != 1 {
		t.Fatal("关闭菜单后的 Tooltip 没有复用同一宿主")
	}
	if err := menu.ShowAt(geometry.Point{}); err != nil || w.tooltip.popup != nil || !menu.Visible() || len(plat.natives) != 1 {
		t.Fatalf("菜单没有接管 Tooltip 宿主：%v", err)
	}
}

type transientCreationPlatform struct {
	*transientTestPlatform
	onPainter func()
	failFirst bool
	attempts  int
}

func (p *transientCreationPlatform) NewPainter(surface platform.Surface) (graphics.Painter, error) {
	p.attempts++
	if p.onPainter != nil {
		p.onPainter()
	}
	if p.failFirst && p.attempts == 1 {
		// 模拟已创建 Popup 的 Painter 不支持；销毁该失败 Popup 的终态
		// 不能结束下一次 opaque 创建。真正的 DragSource 终态仍单独转交 app。
		p.natives[0].handler(events.SizeEvent{Width: 999, Height: 999})
		return nil, platform.ErrUnsupported
	}
	return p.transientTestPlatform.NewPainter(surface)
}

func TestTransientPopupPainterFallbackRetainsOnlySuccessfulResources(t *testing.T) {
	w, plat, makePopup := transientFixture(t)
	creation := &transientCreationPlatform{transientTestPlatform: plat, failFirst: true}
	App.(*application).platform = creation
	p := makePopup(nil)
	if err := p.Show(); err != nil || testSharedCore(p).Transparent() || len(plat.natives) != 2 || len(plat.painters) != 1 || !plat.natives[0].destroyed {
		t.Fatalf("Painter fallback 没有清理失败候选：%v", err)
	}
	// 失败候选的迟到尺寸和销毁通知不能落到当前 presenter。
	plat.natives[0].handler(events.SizeEvent{Width: 888, Height: 888})
	plat.natives[0].handler(events.DestroyEvent{})
	if !p.Visible() || testSharedCore(p).width != 100 || testSharedCore(p).height != 40 || w.shared.current != p {
		t.Fatal("失败候选的旧回调污染了已选中的共享宿主")
	}
	p.Hide()
	q := makePopup(nil)
	if err := q.Show(); err != nil || creation.attempts != 2 || testSharedCore(q).painter != plat.painters[0] {
		t.Fatalf("后续 presenter 再次尝试或创建 Painter：%v", err)
	}
}

func TestTransientPopupOwnerDestroyedDuringCreation(t *testing.T) {
	w, plat, makePopup := transientFixture(t)
	creation := &transientCreationPlatform{transientTestPlatform: plat, onPainter: w.Destroy}
	App.(*application).platform = creation
	p := makePopup(nil)
	if err := p.Show(); err == nil || p.Visible() || testSharedCore(p).platformPopup != nil || testSharedCore(p).painter != nil || w.platformWindow != nil {
		t.Fatal("创建回调销毁 owner 后仍成功显示或残留借用资源")
	}
	if len(plat.natives) != 1 || !plat.natives[0].destroyed || len(plat.painters) != 1 || len(plat.painters[0].calls) != 1 || plat.painters[0].calls[0] != "destroy" {
		t.Fatal("取消创建后没有完整释放部分资源")
	}
}

func TestTransientPopupOwnerDestroyedDuringFocusCleanup(t *testing.T) {
	w, plat, makePopup := transientFixture(t)
	p := makePopup(nil)
	content := newTestWidget()
	content.SetFocusable(true)
	p.SetWidget(content)
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	p.SetFocusedWidget(content)
	content.ConnectFocused(func(focused bool) {
		if !focused {
			w.Destroy()
			if plat.natives[0].destroyed {
				t.Fatal("归还过程的输入回调提前销毁原生宿主")
			}
		}
	})
	p.Hide()
	if !plat.natives[0].destroyed || w.platformWindow != nil || w.shared.current != nil || p.FocusedWidget() != nil {
		t.Fatal("焦点回调结束后未完成窗口清理")
	}
}

// 只供断言读取当前实际宿主；未取得宿主的轻量对象没有 GUI 运行状态。
func testSharedCore(p SharedPopover) *popover {
	if p := p.(*sharedPopover); p.slot != nil {
		return p.slot.popup
	}
	return &popover{}
}
