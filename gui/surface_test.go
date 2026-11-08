package gui

import (
	"errors"
	"fmt"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
)

func TestSurfaceIdentityAndTextLifetime(t *testing.T) {
	host := newSystemSurface(nil)
	host.claimed = true
	host.frameSuspended = false
	var public Surface = host
	if _, managed := public.(Window); managed {
		t.Fatal("系统 Surface 暴露了窗口管理")
	}
	input := NewTextInput()
	input.SetText("before")
	mounts, unmounts := 0, 0
	input.ConnectMount(func() { mounts++ })
	input.ConnectUnmount(func() { unmounts++ })
	public.SetWidget(input)
	if input.Root() != public || input.Surface() != public || input.Window() != nil {
		t.Fatal("Widget 宿主身份不正确")
	}
	if err := public.DispatchEvent(events.FocusEvent{Focused: true}); err != nil {
		t.Fatal(err)
	}
	if !public.SetFocusedWidget(input) {
		t.Fatal("无法设置逻辑焦点")
	}
	input.SetSelection(TextSelection{Anchor: 6, Caret: 6})
	if err := public.CommitText(" after"); err != nil {
		t.Fatal(err)
	}
	if input.Text() != "before after" {
		t.Fatal(input.Text())
	}
	for generation := uint64(1); generation <= 3; generation++ {
		if err := public.DispatchEvent(events.SurfaceEvent{State: platform.SurfaceState{Ready: false, Generation: generation}}); err != nil {
			t.Fatal(err)
		}
		if err := public.DispatchEvent(events.SizeEvent{Width: 320, Height: 480, PixelWidth: 640, PixelHeight: 960}); err != nil {
			t.Fatal(err)
		}
		if err := public.DispatchEvent(events.SurfaceEvent{State: platform.SurfaceState{Ready: true, Generation: generation + 1}}); err != nil {
			t.Fatal(err)
		}
	}
	if public.Widget() != input || public.FocusedWidget() != input || input.Text() != "before after" || mounts != 1 || unmounts != 0 {
		t.Fatalf("目标替换破坏逻辑树：mount=%d unmount=%d", mounts, unmounts)
	}
	info := public.Snapshot()
	if info.Bounds.Size != (geometry.Size{Width: 320, Height: 480}) || info.Controls != nil || info.Title != "" || info.Widget.Text != "before after" {
		t.Fatalf("Surface 快照错误：%+v", info)
	}
	destroyed := 0
	public.ConnectDestroy(func() { destroyed++; public.Destroy(); public.SetWidget(NewLabel("resurrect")) })
	public.Destroy()
	public.Destroy()
	if destroyed != 1 || unmounts != 1 || !input.Destroyed() || public.Widget() != nil {
		t.Fatalf("销毁重入/释放错误：destroy=%d unmount=%d", destroyed, unmounts)
	}
}

func TestSurfaceApplicationEnumeration(t *testing.T) {
	app := &application{servicesReady: true}
	host := newSystemSurface(app)
	host.claimed = true
	host.SetID("content")
	app.systemSurface = host
	managed := testWindowWithSurface(&window{surfaceBase: surfaceBase{rootBase: rootBase{app: app}, id: "managed"}})
	app.windows = []*window{managed}
	if len(app.Surfaces()) != 2 || len(app.Windows()) != 1 || len(app.Snapshot().Surfaces) != 2 {
		t.Fatal("未统一枚举 Surface 或混淆窗口管理子集")
	}
	app.quitOnLastWindowClosed = true
	app.removeWindow(managed)
	if app.quitting.Load() || len(app.Surfaces()) != 1 {
		t.Fatal("仍有系统 Surface 时关闭管理窗口触发退出")
	}
	host.Destroy()
	if app.quitting.Load() || len(app.Surfaces()) != 0 {
		t.Fatal("Surface 释放被当作退出应用")
	}
}

func TestSurfaceClaimAndShutdown(t *testing.T) {
	app := &application{servicesReady: true, windowSource: &activationTestSource{}}
	app.systemSurface = newSystemSurface(app)
	first, err := app.NewSurface()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = app.NewSurface(); err != platform.ErrUnsupported {
		t.Fatalf("重复领取没有返回不支持：%v", err)
	}
	first.Destroy()
	second, err := app.NewSurface()
	if err != nil || second == first {
		t.Fatalf("显式销毁后未重新创建逻辑宿主：%v", err)
	}
	root := NewLabel("owned")
	second.SetWidget(root)
	app.shutdown()
	if !root.Destroyed() || len(app.Surfaces()) != 0 {
		t.Fatal("Shutdown 没有最终释放系统 Surface")
	}
}

func TestSurfacePopoverUsesCommonOwner(t *testing.T) {
	p, native, _, app := placementFixture(t, geometry.Rect(0, 0, 300, 200))
	host := newSystemSurface(app)
	host.claimed = true
	// 只通过基础 Window 借用事实，不令逻辑 Surface 获得管理方法。
	host.platformWindow = struct{ platform.Window }{native}
	anchor := NewButton()
	host.SetWidget(anchor)
	anchor.Arrange(geometry.Rect(20, 20, 40, 30))
	app.systemSurface = host
	p.anchor = anchor
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	if p.owner != host || len(host.Popovers()) != 1 || anchor.Surface() != host {
		t.Fatal("Popover 仍依赖管理窗口")
	}
	p.Hide()
	if len(host.Popovers()) != 0 {
		t.Fatal("隐藏后未从 Surface 注销")
	}
	host.Destroy()
}

type surfaceErrorLoop struct{ quitCountLoop }

func (*surfaceErrorLoop) Run() {}

type surfaceErrorNative struct {
	platform.Window
	ready     bool
	destroyed int
}

func (n *surfaceErrorNative) SurfaceState() platform.SurfaceState {
	return platform.SurfaceState{Ready: n.ready, Generation: 1}
}
func (n *surfaceErrorNative) RequestPaint() error { return nil }
func (n *surfaceErrorNative) Destroy()            { n.destroyed++; n.ready = false }

type surfaceErrorPlatform struct {
	platform.Platform
	failure error
}

func (p *surfaceErrorPlatform) NewPainter(platform.Surface) (graphics.Painter, error) {
	return nil, p.failure
}

func TestSurfacePainterFailure(t *testing.T) {
	for _, immediate := range []bool{true, false} {
		t.Run(fmt.Sprintf("ready-on-claim=%t", immediate), func(t *testing.T) {
			failure := errors.New("无法创建测试 Painter")
			native := &surfaceErrorNative{ready: immediate}
			app := &application{servicesReady: true, windowSource: &activationTestSource{}, loop: &surfaceErrorLoop{}, platform: &surfaceErrorPlatform{failure: failure}}
			core := newSystemSurface(app)
			app.systemSurface = core
			core.platformWindow, core.surface = native, native
			host, err := app.NewSurface()
			if immediate {
				if !errors.Is(err, failure) || app.quitting.Load() || native.destroyed != 1 {
					t.Fatalf("同步失败未返回或错误退出：%v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			widget := newTestWidget()
			host.SetWidget(widget)
			native.ready = true
			_ = host.DispatchEvent(events.SurfaceEvent{State: native.SurfaceState()})
			_ = host.DispatchEvent(events.VisibilityEvent{Visible: true})
			_ = host.DispatchEvent(events.PaintEvent{})
			if !errors.Is(app.Run(), failure) || native.destroyed != 1 || !widget.Destroyed() {
				t.Fatal("异步创建失败未经 Run 返回或没有释放树")
			}
		})
	}
}
