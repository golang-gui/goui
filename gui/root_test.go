package gui

import (
	"errors"
	"image"
	"reflect"
	"testing"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/style"
)

type changingSurface struct {
	state platform.SurfaceState
}

func (s *changingSurface) SurfaceState() platform.SurfaceState { return s.state }
func (*changingSurface) Transparent() bool                     { return false }
func (*changingSurface) NativeHandle() uintptr                 { return 1 }
func (*changingSurface) Draw(image.Image) error                { return nil }
func (*changingSurface) RequestPaint() error                   { return nil }

func TestRootReplaceableSurface(t *testing.T) {
	// 逻辑窗口先于 drawable；就绪、隐藏、丢失、恢复不卸载控件或丢弃脏状态。
	surface := new(changingSurface)
	native := new(showTestWindow)
	w := &window{rootBase: rootBase{surface: surface}, platformWindow: native}
	child := newTestWidget()
	w.SetWidget(child)
	frames := 0
	w.ConnectFrame(func(time.Time) { frames++ })
	if native.paints != 0 || w.beginFrame() || !w.paintDirty || w.framePending {
		t.Fatal("原生目标出现前启动了帧")
	}
	surface.state = platform.SurfaceState{Ready: true, Generation: 1}
	_ = w.DispatchEvent(events.SurfaceEvent{State: surface.state})
	if native.paints != 1 || !w.layoutDirty {
		t.Fatal("首次表面未安排布局/绘制")
	}
	if !w.beginFrame() {
		t.Fatal("表面就绪后未恢复帧")
	}
	w.endFrame()
	_ = w.DispatchEvent(events.VisibilityEvent{Visible: false})
	_ = w.RequestPaint()
	if native.paints != 1 || w.beginFrame() || frames != 1 {
		t.Fatal("后台窗口仍然推进帧")
	}
	surface.state.Ready = false
	_ = w.DispatchEvent(events.SurfaceEvent{State: surface.state})
	_ = w.DispatchEvent(events.VisibilityEvent{Visible: true})
	if native.paints != 1 || w.framePending || w.beginFrame() {
		t.Fatal("尚未恢复 drawable 就提交绘制")
	}
	surface.state = platform.SurfaceState{Ready: true, Generation: 2}
	_ = w.DispatchEvent(events.SurfaceEvent{State: surface.state})
	if native.paints != 2 || !w.beginFrame() || frames != 2 {
		t.Fatal("替换表面未恢复帧")
	}
	w.endFrame()
	if w.Widget() != child || child.Root() != w || w.destroyed {
		t.Fatal("表面替换改变了逻辑控件树")
	}
}

func TestRootSurfaceRevokedBeforeNotification(t *testing.T) {
	// 模拟原生线程先撤销许可、SurfaceEvent 尚未被 GOUI 线程处理。
	surface := &changingSurface{state: platform.SurfaceState{Ready: true, Generation: 1}}
	backend := new(testGraphicsPainter)
	b := &rootBase{surface: surface, painter: backend}
	later := false
	b.ConnectFrame(func(time.Time) { surface.state.Ready = false })
	b.ConnectFrame(func(time.Time) { later = true })
	if b.beginFrame() || later || b.framePainting {
		t.Fatal("撤销后继续帧回调或遗留绘制状态")
	}
	b.drawFrame(nil, style.Style{})
	if backend.begins != 0 || backend.ends != 0 {
		t.Fatal("撤销后仍然调用绘制后端")
	}
	requester := new(frameRequesterTest)
	_ = b.requestPaint(requester)
	if requester.requests != 0 || !b.paintDirty {
		t.Fatal("撤销后仍请求原生绘制或丢失脏状态")
	}
}

type frameRequesterTest struct {
	requests int
	err      error
}

func (r *frameRequesterTest) RequestPaint() error { r.requests++; return r.err }

func TestRootFrameScheduling(t *testing.T) {
	now := time.Unix(100, 0)
	var posts []func()
	scheduler := newTimerScheduler(func(fn func()) { posts = append(posts, fn) }, func() time.Time { return now })
	app := &application{timers: scheduler}
	b := &rootBase{app: app}
	r := new(frameRequesterTest)
	frames := 0
	b.ConnectFrame(func(at time.Time) {
		frames++
		if at != now {
			t.Fatal("frame timestamp")
		}
	})
	if frames != 0 || len(scheduler.queue) != 0 || r.requests != 0 {
		t.Fatal("passive connection started drawing")
	}
	for range 10 {
		if err := b.requestPaint(r); err != nil {
			t.Fatal(err)
		}
	}
	if r.requests != 1 {
		t.Fatal("requests not coalesced")
	}
	if !b.beginFrame() {
		t.Fatal("frame rejected")
	}
	for range 10 {
		_ = b.requestPaint(r)
	}
	if r.requests != 1 || len(scheduler.queue) != 0 {
		t.Fatal("requested platform during frame")
	}
	b.endFrame()
	if len(scheduler.queue) != 1 || r.requests != 1 {
		t.Fatal("next frame not armed once")
	}
	_ = b.requestPaint(r)
	if len(scheduler.queue) != 1 || r.requests != 1 {
		t.Fatal("async UI invalidation bypassed pacing")
	}
	now = now.Add(frameInterval)
	scheduler.postDue()
	if len(posts) != 1 {
		t.Fatal("deadline did not dispatch")
	}
	posts[0]()
	if r.requests != 2 {
		t.Fatal("timer did not request paint")
	}
	if !b.beginFrame() {
		t.Fatal("second frame")
	}
	b.endFrame()
	if len(scheduler.queue) != 0 || b.framePending || frames != 2 {
		t.Fatal("idle retained work")
	}
}

func TestRootFrameEarlyPaintAndLifecycle(t *testing.T) {
	now := time.Unix(100, 0)
	s := newTimerScheduler(func(func()) {}, func() time.Time { return now })
	b := &rootBase{app: &application{timers: s}}
	r := new(frameRequesterTest)
	_ = b.requestPaint(r)
	b.beginFrame()
	_ = b.requestPaint(r)
	b.endFrame()
	if len(s.queue) != 1 {
		t.Fatal("timer missing")
	}
	now = now.Add(time.Millisecond)
	b.beginFrame() // Native expose satisfies a pending GUI request early.
	b.endFrame()
	if len(s.queue) != 0 || r.requests != 1 {
		t.Fatal("early native paint left stale timer")
	}
	_ = b.requestPaint(r)
	b.suspendFrames()
	now = now.Add(time.Second)
	_ = b.requestPaint(r)
	if len(s.queue) != 0 || r.requests != 1 || b.beginFrame() {
		t.Fatal("suspended root woke")
	}
	b.resumeFrames(r)
	if r.requests != 2 {
		t.Fatal("resume did not request frame")
	}
	called := false
	b.ConnectFrame(func(time.Time) { b.destroyFrames() })
	b.ConnectFrame(func(time.Time) { called = true })
	if b.beginFrame() || called {
		t.Fatal("destroyed root continued callbacks")
	}
	_ = b.requestPaint(r)
	if r.requests != 2 || len(s.queue) != 0 {
		t.Fatal("destroy retained work")
	}
}

func TestRootFrameReentrancyAndFailure(t *testing.T) {
	b := new(rootBase)
	r := &frameRequesterTest{err: errors.New("paint")}
	if b.requestPaint(r) != r.err || b.framePending {
		t.Fatal("submission failure lost")
	}
	r.err = nil
	if err := b.requestPaint(r); err != nil || r.requests != 2 {
		t.Fatal("explicit retry failed")
	}
	b.ConnectFrame(func(time.Time) {
		if b.beginFrame() {
			t.Fatal("reentered frame")
		}
	})
	if !b.beginFrame() {
		t.Fatal("outer frame")
	}
	b.endFrame()
	if r.requests != 3 {
		t.Fatal("reentrant request not preserved")
	}
}

type frameOrderWidget struct {
	WidgetBase
	trace *[]string
}

func (w *frameOrderWidget) Measure(c layout.Constraint) layout.Measurement {
	*w.trace = append(*w.trace, "measure")
	return layout.Measured(c.Min)
}
func (w *frameOrderWidget) Arrange(r geometry.Rectangle) {
	*w.trace = append(*w.trace, "arrange")
	w.WidgetBase.Arrange(r)
}
func (w *frameOrderWidget) Paint(Painter) { *w.trace = append(*w.trace, "paint") }

func TestWindowFrameUpdatesBeforeLayoutAndReplacesTree(t *testing.T) {
	var trace []string
	w := &window{rootBase: rootBase{painter: new(testGraphicsPainter), width: 100, height: 100}}
	old := newTestWidget()
	w.SetWidget(old)
	newContent := &frameOrderWidget{trace: &trace}
	w.ConnectFrame(func(time.Time) { trace = append(trace, "frame"); w.SetWidget(newContent) })
	w.paint()
	if !reflect.DeepEqual(trace, []string{"frame", "measure", "arrange", "paint"}) || newContent.Root() != w {
		t.Fatalf("frame order: %v", trace)
	}
}

type frameShowPopup struct {
	platform.Popup
	handler platform.EventHandler
}

func (p *frameShowPopup) Show() error {
	p.handler(events.PaintEvent{}) // Windows UpdateWindow during native Show.
	return p.Popup.Show()
}

type frameShowPlatform struct{ *placementPlatform }

func (p *frameShowPlatform) NewPopup(w platform.Window, size geometry.Size, handler platform.EventHandler, options platform.PopupOptions) (platform.Popup, error) {
	native, err := p.placementPlatform.NewPopup(w, size, handler, options)
	return &frameShowPopup{Popup: native, handler: handler}, err
}

func TestPopoverFirstShowPausesSynchronousPaint(t *testing.T) {
	p, _, plat, app := placementFixture(t, geometry.Rect(0, 0, 1000, 1000))
	app.platform = &frameShowPlatform{plat}
	p.anchor.Root().(*window).app = app
	frames := 0
	p.ConnectFrame(func(time.Time) {
		if !p.Visible() {
			t.Fatal("frame before Show committed visibility")
		}
		frames++
	})
	for range 2 { // First presentation and reopening must have identical order.
		before := frames
		if err := p.Show(); err != nil {
			t.Fatal(err)
		}
		if frames != before || !p.layoutDirty || p.app != app {
			t.Fatal("native Show consumed initial layout/frame or lost owner application")
		}
		p.paint()
		if frames != before+1 || p.layoutDirty {
			t.Fatal("visible frame did not lay out")
		}
		p.Hide()
	}
}
