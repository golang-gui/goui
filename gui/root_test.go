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
	"github.com/golang-gui/goui/platform/graphics/software"
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
	w := testWindowWithSurface(&window{surfaceBase: surfaceBase{rootBase: rootBase{surface: surface}, platformWindow: native}})
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
	w := testWindowWithSurface(&window{surfaceBase: surfaceBase{rootBase: rootBase{painter: new(testGraphicsPainter), width: 100, height: 100}}})
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
		// 原生 Show 内的绘制仍暂停；提交可见性后，同步 Repaint 完成一个用户帧。
		if frames != before+1 || p.layoutDirty || p.app != app {
			t.Fatal("Show 未在提交可见性后完成首帧，或原生 Show 提前消耗了帧")
		}
		p.RequestPaint()
		p.paint()
		if frames != before+2 || p.layoutDirty {
			t.Fatal("后续可见帧未正常执行")
		}
		p.Hide()
	}
}

// 三种宿主共用绘制入口；测试替身不连接桌面或原生 GPU。
type preparationHost interface {
	Root
	SetWidget(Widget)
	Destroy()
}

func preparationFixture(t *testing.T, kind string) (preparationHost, *rootBase, func()) {
	t.Helper()
	switch kind {
	case "window":
		w := testWindowWithSurface(&window{surfaceBase: surfaceBase{rootBase: rootBase{width: 120, height: 80}}})
		t.Cleanup(w.Destroy)
		return w, &w.rootBase, w.paint
	case "surface":
		s := newSystemSurface(nil)
		s.claimed, s.frameSuspended = true, false
		s.width, s.height = 120, 80
		t.Cleanup(s.Destroy)
		return s, &s.rootBase, s.paint
	case "popover":
		p, _, _, _ := placementFixture(t, geometry.Rect(0, 0, 1000, 1000))
		if err := p.Show(); err != nil {
			t.Fatal(err)
		}
		return p, &p.rootBase, p.paint
	default:
		t.Fatalf("未知宿主 %q", kind)
		return nil, nil, nil
	}
}

type preparationWidget struct{ frameOrderWidget }

func (w *preparationWidget) Measure(c layout.Constraint) layout.Measurement {
	*w.trace = append(*w.trace, "measure")
	return layout.Measured(c.Clamp(geometry.Size{Width: 80, Height: 60}))
}

type preparationPainter struct {
	testGraphicsPainter
	trace *[]string
}

func (p *preparationPainter) Begin(width, height, scale float32) {
	*p.trace = append(*p.trace, "begin")
	p.testGraphicsPainter.Begin(width, height, scale)
}

func (p *preparationPainter) End() {
	*p.trace = append(*p.trace, "end")
	p.testGraphicsPainter.End()
}

func TestRootBeforePaintOrderAcrossHosts(t *testing.T) {
	for _, kind := range []string{"window", "surface", "popover"} {
		t.Run(kind, func(t *testing.T) {
			host, base, paint := preparationFixture(t, kind)
			var trace []string
			content := &preparationWidget{frameOrderWidget{trace: &trace}}
			host.SetWidget(content)
			backend := &preparationPainter{trace: &trace}
			base.painter = backend
			content.RequestLayout()
			trace = nil // Popup 的内容测量可能在设置时发生，下面只记录完整帧。
			host.ConnectFrame(func(time.Time) { trace = append(trace, "frame") })
			host.ConnectBeforePaint(func() {
				if content.Rect().Width <= 0 || content.Rect().Height <= 0 || base.layoutDirty || backend.begins != backend.ends {
					t.Fatal("准备先于布局完成，或发生在 Painter.Begin 之后")
				}
				trace = append(trace, "prepare")
			})
			host.ConnectAfterPaint(func(Painter, geometry.Rectangle) { trace = append(trace, "after") })
			paint()
			want := []string{"frame", "measure", "arrange", "prepare", "begin", "paint", "after", "end"}
			if kind == "popover" {
				// Popup 先按内容计算自然尺寸，再按最终窗口尺寸测量分配。
				want = []string{"frame", "measure", "measure", "arrange", "prepare", "begin", "paint", "after", "end"}
			}
			if !reflect.DeepEqual(trace, want) {
				t.Fatalf("布局与绘制顺序：%v，期望 %v", trace, want)
			}
			trace = nil
			paint()
			want = []string{"frame", "prepare", "begin", "paint", "after", "end"}
			if !reflect.DeepEqual(trace, want) {
				t.Fatalf("没有重新布局的帧仍应准备一次：%v", trace)
			}
		})
	}
}

func TestRootBeforePaintRequestsWaitForNextFrame(t *testing.T) {
	now := time.Unix(100, 0)
	var posts []func()
	scheduler := newTimerScheduler(func(fn func()) { posts = append(posts, fn) }, func() time.Time { return now })
	base := &rootBase{app: &application{timers: scheduler}, painter: new(testGraphicsPainter), width: 100, height: 80, layoutDirty: true}
	requester := new(frameRequesterTest)
	content := newLayoutPassWidget()
	called := 0
	handle := base.ConnectBeforePaint(func() {
		called++
		base.requestLayout(requester)
		_ = base.requestPaint(requester)
		if requester.requests != 1 || content.arranges != 1 {
			t.Fatal("准备回调同步重入布局或原生请求")
		}
	})
	base.ConnectBeforePaint(nil).Disconnect()
	if requester.requests != 0 || base.paintDirty || len(scheduler.queue) != 0 {
		t.Fatal("连接准备信号主动安排了帧")
	}
	_ = base.requestPaint(requester)
	base.paintFrame(content, style.Style{})
	if called != 1 || content.arranges != 1 || content.paints != 1 || !base.layoutDirty || !base.paintDirty || len(scheduler.queue) != 1 {
		t.Fatal("准备中的失效丢失，或在同帧反复布局")
	}
	handle.Disconnect()
	now = now.Add(frameInterval)
	scheduler.postDue()
	if len(posts) != 1 {
		t.Fatal("没有合并为一次后续帧请求")
	}
	posts[0]()
	base.paintFrame(content, style.Style{})
	if called != 1 || requester.requests != 2 || content.arranges != 2 || content.paints != 2 || base.layoutDirty || base.paintDirty || len(scheduler.queue) != 0 {
		t.Fatal("后续帧没有完成失效或空闲后仍继续续帧")
	}
}

func TestRootBeforePaintWaitsForChromeReservation(t *testing.T) {
	w, _ := chromeFixture(t, false, true)
	backend := new(testGraphicsPainter)
	w.painter = backend
	content := newLayoutPassWidget()
	w.SetWidget(content)
	// 模拟 HeaderBar 根据新的按钮占位请求重新分配，触发一次有界追加布局。
	w.Chrome().ConnectInfo(func(ChromeInfo) { content.RequestLayout() })
	w.Chrome().ConnectQueryControls(func(result *ChromeControls) { result.Height = 64 })
	calls := 0
	w.ConnectBeforePaint(func() {
		calls++
		if content.arranges != 2 || w.controls == nil || w.controls.Rect() != w.chrome.info.ControlsBounds || w.controls.Rect().Height != 64 || backend.begins != 0 {
			t.Fatalf("准备先于追加布局或装饰分配：arranges=%d controls=%v", content.arranges, w.controls)
		}
	})
	w.paint()
	if calls != 1 || backend.begins != 1 || backend.ends != 1 {
		t.Fatal("追加布局导致重复准备或跳过绘制")
	}
}

func TestRootBeforePaintHandlesAndTargetValidity(t *testing.T) {
	for _, reason := range []string{"suspend", "revoke", "replace", "destroy"} {
		t.Run(reason, func(t *testing.T) {
			surface := &changingSurface{state: platform.SurfaceState{Ready: true, Generation: 1}}
			backend := new(testGraphicsPainter)
			base := &rootBase{surface: surface, painter: backend, width: 20, height: 20}
			first, later := 0, 0
			handle := base.ConnectBeforePaint(func() {
				first++
				switch reason {
				case "suspend":
					base.suspendFrames()
				case "revoke":
					surface.state.Ready = false // 原生目标先撤销，事件尚未到达。
				case "replace":
					base.surfaceChanged()
				case "destroy":
					base.destroyFrames()
				}
			})
			base.ConnectBeforePaint(func() { later++ })
			handle.Block()
			base.paintFrame(nil, style.Style{})
			if first != 0 || later != 1 || backend.begins != 1 || backend.ends != 1 {
				t.Fatal("Block 阻止了其它回调或正常绘制")
			}
			handle.Unblock()
			base.paintFrame(nil, style.Style{})
			if first != 1 || later != 1 || backend.begins != 1 || backend.ends != 1 || base.framePainting {
				t.Fatal("失效后执行了后续准备/绘制，或没有完成帧清理")
			}
			handle.Disconnect()
		})
	}
}

func TestRootBeforePaintCanDestroyHosts(t *testing.T) {
	for _, kind := range []string{"window", "surface", "popover"} {
		t.Run(kind, func(t *testing.T) {
			host, base, paint := preparationFixture(t, kind)
			backend := new(testGraphicsPainter)
			base.painter = backend
			calls := 0
			host.ConnectBeforePaint(func() { calls++; host.Destroy() })
			host.ConnectBeforePaint(func() { t.Fatal("宿主销毁后执行了后续准备回调") })
			paint()
			if calls != 1 || !base.frameDestroyed || base.framePainting || backend.begins != 0 || backend.ends != 0 || base.painter != nil {
				t.Fatal("准备回调销毁宿主后仍访问 Painter，或延迟清理未完成")
			}
			host.ConnectBeforePaint(func() { t.Fatal("已销毁宿主接受了新连接") }).Disconnect()
			paint()
		})
	}
}

func TestRootBeforePaintExcludesOffscreenRendering(t *testing.T) {
	output := new(transparentFrame)
	backend, err := software.NewPainter(output)
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Destroy()
	base := &rootBase{painter: backend, width: 20, height: 20}
	content := newTestWidget()
	content.Arrange(geometry.Rect(0, 0, 20, 20))
	calls := 0
	base.ConnectBeforePaint(func() { calls++ })
	if _, err := base.renderWidgetImage(content, 1); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatal("RenderWidget 误触发 Root 的准备信号")
	}
	base.paintFrame(content, style.Style{})
	if calls != 1 {
		t.Fatal("实际绘制未执行准备回调")
	}
}
