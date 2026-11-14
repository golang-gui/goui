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

type transientTestPlatform struct {
	*placementPlatform
	painters []*lifecyclePainter
}

func (p *transientTestPlatform) NewPainter(platform.Surface) (graphics.Painter, error) {
	backend := &lifecyclePainter{}
	p.painters = append(p.painters, backend)
	return backend, nil
}

func transientFixture(t *testing.T) (*window, *transientTestPlatform, func(Widget) *popover) {
	t.Helper()
	base, _, native, app := placementFixture(t, geometry.Rect(0, 0, 1000, 800))
	plat := &transientTestPlatform{placementPlatform: native}
	app.platform = plat
	w := app.windows[0]
	t.Cleanup(w.Destroy)
	return w, plat, func(anchor Widget) *popover {
		if anchor == nil {
			anchor = base.anchor
		}
		p := NewTransientPopover(anchor).(*popover)
		p.SetWidget(&popoverMeasureWidget{size: geometry.Size{Width: 100, Height: 40}})
		t.Cleanup(p.Destroy)
		return p
	}
}

func TestTransientPopupRetainsNativeAndPainterAcrossPresenters(t *testing.T) {
	w, plat, makePopup := transientFixture(t)
	var native platform.Popup
	var painter graphics.Painter
	for range 30 {
		p := makePopup(nil)
		for range 3 {
			if err := p.Show(); err != nil {
				t.Fatal(err)
			}
			if native == nil {
				native, painter = p.platformPopup, p.painter
			}
			if p.platformPopup != native || p.painter != painter || len(plat.natives) != 1 || len(plat.painters) != 1 {
				t.Fatal("切换或重新打开创建了新的 Popup/Painter")
			}
			p.Hide()
			if p.Visible() || p.platformPopup != nil || p.painter != nil || len(w.Popovers()) != 0 || w.transient.active != nil || plat.natives[0].destroyed || len(plat.painters[0].calls) != 0 {
				t.Fatal("关闭未归还宿主或销毁了缓存资源")
			}
		}
		p.Destroy()
	}
	w.Destroy()
	if !plat.natives[0].destroyed || len(plat.painters[0].calls) != 1 || plat.painters[0].calls[0] != "destroy" {
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
	for _, presenter := range []*popover{ordinary, p, q} {
		if err := presenter.Show(); err != nil {
			t.Fatal(err)
		}
	}
	if len(plat.natives) != 3 || len(plat.painters) != 3 || len(w.Popovers()) != 2 || len(other.Popovers()) != 1 || p.platformPopup == q.platformPopup || p.painter == q.painter {
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
			if err := second.Show(); err == nil || second.Visible() || !latest.Visible() || w.transient.active != latest {
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
	native, painter := p.platformPopup, p.painter
	p.Hide()
	// 不改变原生尺寸、也不再发送 SizeEvent；新 presenter 必须继承实际像素尺寸。
	plat.natives[0].syncSize = false
	q := makePopup(nil)
	if err := q.Show(); err != nil || q.width != 100 || q.height != 40 || q.pixelWidth != 100 || q.pixelHeight != 40 {
		t.Fatalf("复用未继承实际尺寸：%v DIP=(%g,%g) pixels=(%g,%g)", err, q.width, q.height, q.pixelWidth, q.pixelHeight)
	}
	anchor := p.anchor
	w.SetWidget(nil)
	if q.Visible() || plat.natives[0].destroyed || w.transient.active != nil {
		t.Fatal("源控件卸载没有归还宿主或销毁了缓存")
	}
	w.SetWidget(anchor)
	if err := p.Show(); err != nil || p.platformPopup != native || p.painter != painter {
		t.Fatalf("同窗重新挂载没有复用：%v", err)
	}
	other := testWindowWithSurface(&window{surfaceBase: surfaceBase{platformWindow: &placementWindow{area: geometry.Rect(0, 0, 1000, 800)}}})
	t.Cleanup(other.Destroy)
	other.SetWidget(anchor)
	if err := p.Show(); err != nil || p.platformPopup == native || p.painter == painter || len(plat.natives) != 2 || plat.natives[0].destroyed {
		t.Fatalf("迁窗没有隔离资源：%v", err)
	}
}

func TestTransientPopupRoutesEventsToCurrentContent(t *testing.T) {
	_, plat, makePopup := transientFixture(t)
	first, second := makePopup(nil), makePopup(nil)
	calls := [2][]string{}
	for i, p := range []*popover{first, second} {
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
	if !second.Visible() || second.platformPopup == nil {
		t.Fatal("旧 presenter 销毁影响新内容")
	}
}

func TestTransientPopupDefersOwnerDestroyUntilFrameEnds(t *testing.T) {
	w, plat, makePopup := transientFixture(t)
	p := makePopup(nil)
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	plat.painters[0].onBegin = func() {
		w.Destroy()
		if plat.natives[0].destroyed || w.platformWindow == nil || len(plat.painters[0].calls) != 1 {
			t.Fatal("活动帧中提前销毁共享资源或 owner")
		}
	}
	p.paint()
	if !plat.natives[0].destroyed || w.platformWindow != nil || p.Visible() || w.transient.active != nil {
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
					if err != nil || p.Transparent() || len(plat.options) != 2 || len(plat.painters) != 1 {
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
	if !menu.Visible() || w.tooltip.target != nil || w.tooltip.popup != nil || len(plat.natives) != 1 || w.transient.active != menu.popover {
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
	if err := p.Show(); err != nil || p.Transparent() || len(plat.natives) != 2 || len(plat.painters) != 1 || !plat.natives[0].destroyed {
		t.Fatalf("Painter fallback 没有清理失败候选：%v", err)
	}
	// 失败候选的迟到尺寸和销毁通知不能落到当前 presenter。
	plat.natives[0].handler(events.SizeEvent{Width: 888, Height: 888})
	plat.natives[0].handler(events.DestroyEvent{})
	if !p.Visible() || p.width != 100 || p.height != 40 || w.transient.active != p {
		t.Fatal("失败候选的旧回调污染了已选中的共享宿主")
	}
	p.Hide()
	q := makePopup(nil)
	if err := q.Show(); err != nil || creation.attempts != 2 || q.painter != plat.painters[0] {
		t.Fatalf("后续 presenter 再次尝试或创建 Painter：%v", err)
	}
}

func TestTransientPopupOwnerDestroyedDuringCreation(t *testing.T) {
	w, plat, makePopup := transientFixture(t)
	creation := &transientCreationPlatform{transientTestPlatform: plat, onPainter: w.Destroy}
	App.(*application).platform = creation
	p := makePopup(nil)
	if err := p.Show(); err == nil || p.Visible() || p.platformPopup != nil || p.painter != nil || w.platformWindow != nil {
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
	if !plat.natives[0].destroyed || w.platformWindow != nil || w.transient.active != nil || p.FocusedWidget() != nil {
		t.Fatal("焦点回调结束后未完成窗口清理")
	}
}
