package gui

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
	"testing"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/software"
)

// 原生尺寸立即生效，通知可同步、延迟或完全不再发送；查询独立读取实际值。
type presentationPopup struct {
	recordingPlatformPopup
	handler    platform.EventHandler
	actual     events.SizeEvent
	scale      float32
	syncSize   bool
	position   geometry.Point
	positions  []geometry.Point
	image      image.Image
	queries    int
	shows      int
	repaints   int
	queryError error
	onQuery    func()
	onRepaint  func()
	onPosition func()
	onShow     func()
}

func (p *presentationPopup) SetSize(w, h float32) {
	p.actual.PixelWidth = float32(math.Ceil(float64(w * p.scale)))
	p.actual.PixelHeight = float32(math.Ceil(float64(h * p.scale)))
	p.actual.Width, p.actual.Height = p.actual.PixelWidth/p.scale, p.actual.PixelHeight/p.scale
	if p.syncSize {
		p.handler(p.actual)
	}
}

func (p *presentationPopup) ClientSize() (geometry.Size, geometry.Size, error) {
	p.queries++
	if p.onQuery != nil {
		p.onQuery()
	}
	return geometry.Size{Width: p.actual.Width, Height: p.actual.Height},
		geometry.Size{Width: p.actual.PixelWidth, Height: p.actual.PixelHeight}, p.queryError
}

func (p *presentationPopup) SetPosition(x, y float32) {
	p.position = geometry.Point{X: x, Y: y}
	p.positions = append(p.positions, p.position)
	if p.onPosition != nil {
		p.onPosition()
	}
}

func (p *presentationPopup) Show() error {
	p.shows++
	if p.onShow != nil {
		p.onShow()
	}
	// 对应 Windows Show 内部的同步 WM_PAINT，提交前仍应暂停。
	p.handler(events.PaintEvent{})
	return nil
}

func (p *presentationPopup) Repaint() error {
	p.repaints++
	if p.onRepaint != nil {
		p.onRepaint()
	}
	p.handler(events.PaintEvent{})
	return nil
}

func (p *presentationPopup) Draw(src image.Image) error {
	p.image = graphics.CopyToBitmap(src, graphics.PixelFormatRGBA, nil)
	return nil
}

type presentationPlatform struct {
	*placementPlatform
	native   *presentationPopup
	scale    float32
	syncSize bool
	onShow   func()
}

func (p *presentationPlatform) NewPopup(_ platform.Window, size geometry.Size, handler platform.EventHandler, _ platform.PopupOptions) (platform.Popup, error) {
	p.native = &presentationPopup{handler: handler, scale: p.scale, syncSize: p.syncSize, onShow: p.onShow}
	p.native.SetSize(size.Width, size.Height)
	return p.native, nil
}

func (*presentationPlatform) NewPainter(surface platform.Surface) (graphics.Painter, error) {
	return software.NewPainter(surface)
}

type presentationWidget struct {
	WidgetBase
	size      geometry.Size
	color     color.RGBA
	onArrange func()
}

func (w *presentationWidget) Measure(c layout.Constraint) layout.Measurement {
	return layout.Measured(c.Clamp(w.size))
}

func (w *presentationWidget) Arrange(r geometry.Rectangle) {
	w.WidgetBase.Arrange(r)
	if w.onArrange != nil {
		w.onArrange()
	}
}

func (w *presentationWidget) Paint(p Painter) {
	p.FillRect(geometry.Rectangle{Size: w.Rect().Size}, graphics.ColorOf(w.color))
}

func TestTransientPresentationGeometryAndFirstFrame(t *testing.T) {
	for _, synchronous := range []bool{false, true} {
		for _, scale := range []float32{1, 1.5, 2} {
			t.Run(fmt.Sprintf("sync=%t/scale=%g", synchronous, scale), func(t *testing.T) {
				base, _, plat, app := placementFixture(t, geometry.Rect(0, 0, 500, 400))
				w := app.windows[0]
				t.Cleanup(w.Destroy)
				base.anchor.Arrange(geometry.Rect(460, 340, 30, 20))
				backend := &presentationPlatform{placementPlatform: plat, scale: scale, syncSize: synchronous}
				app.platform = backend
				var current *sharedPopover
				var content *presentationWidget
				var wantPosition geometry.Point
				backend.onShow = func() {
					n := backend.native
					if n.image == nil || color.RGBAModel.Convert(n.image.At(n.image.Bounds().Dx()/2, n.image.Bounds().Dy()/2)).(color.RGBA) != (color.RGBA{}) {
						t.Fatal("原生显示前没有丢弃旧内容；慢绘制会暴露上次像素")
					}
					if testSharedCore(current).requestedPosition != wantPosition || n.position != wantPosition || content.Rect().Size != (geometry.Size{Width: n.actual.Width, Height: n.actual.Height}) {
						t.Fatalf("显示前仍有旧几何：position=%v native=%v want=%v content=%v actual=%v", testSharedCore(current).requestedPosition, n.position, wantPosition, content.Rect(), n.actual)
					}
				}
				// 独立预期：小面可在下方右对齐，大面只能在上方右对齐。
				for i, size := range []geometry.Size{{Width: 100, Height: 30}, {Width: 220, Height: 150}, {Width: 100, Height: 30}, {Width: 100, Height: 30}} {
					current = NewSharedPopover(base.anchor).(*sharedPopover)
					t.Cleanup(current.Destroy)
					content = &presentationWidget{size: size, color: color.RGBA{R: uint8(30 + i*40), G: 80, B: 160, A: 255}}
					current.SetWidget(content)
					current.SetModal(true)
					current.SetPlacement(PopoverPlacementBottom)
					wantPosition = geometry.Point{X: 490 - size.Width, Y: 360}
					if i == 1 {
						wantPosition.Y = 340 - size.Height
					}
					if err := current.Show(); err != nil {
						t.Fatal(err)
					}
					n := backend.native
					if n.queries == 0 || n.repaints != (i+1)*2 || n.image == nil {
						t.Fatal("Show 返回时没有查询实际尺寸并完成当前内容首帧")
					}
					// 软件像素采用预乘 RGBA，采样主体中心；不涉及边缘抗锯齿，无容差。
					got := color.RGBAModel.Convert(n.image.At(n.image.Bounds().Dx()/2, n.image.Bounds().Dy()/2)).(color.RGBA)
					if got != content.color || current.Widget().Snapshot().Bounds.Size != content.Rect().Size {
						t.Fatalf("首帧或语义仍属旧控件：pixel=%v want=%v snapshot=%v", got, content.color, current.Widget().Snapshot().Bounds)
					}
					current.Hide()
				}
			})
		}
	}
}

func TestTransientPresentationRefinesLayoutBeforeShow(t *testing.T) {
	base, _, plat, app := placementFixture(t, geometry.Rect(0, 0, 500, 400))
	t.Cleanup(app.windows[0].Destroy)
	backend := &presentationPlatform{placementPlatform: plat, scale: 1.5}
	app.platform = backend
	p := NewSharedPopover(base.anchor).(*sharedPopover)
	t.Cleanup(p.Destroy)
	content := &presentationWidget{size: geometry.Size{Width: 100, Height: 30}}
	content.onArrange = func() {
		content.onArrange = nil
		content.size = geometry.Size{Width: 180.25, Height: 90.25}
		content.RequestLayout()
	}
	p.SetWidget(content)
	backend.onShow = func() {
		// 原生 ceil(180.25*1.5)=271，ceil(90.25*1.5)=136。
		if content.Rect().Size != (geometry.Size{Width: 271 / float32(1.5), Height: 136 / float32(1.5)}) || testSharedCore(p).requestedSize != content.size {
			t.Fatalf("显示后才细化布局：requested=%v content=%v", testSharedCore(p).requestedSize, content.Rect())
		}
	}
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	backend.native.queryError = platform.ErrUnavailable
	p.Hide()
	shows := backend.native.shows
	if err := p.Show(); !errors.Is(err, platform.ErrUnavailable) || p.Visible() || backend.native.shows != shows {
		t.Fatalf("尺寸查询失败仍显示或丢失原错误：%v", err)
	}
}

func TestTransientPresentationPreparationCancellation(t *testing.T) {
	for _, action := range []string{"unmount", "destroy", "hide", "nonconverging"} {
		t.Run(action, func(t *testing.T) {
			base, _, plat, app := placementFixture(t, geometry.Rect(0, 0, 500, 400))
			w := app.windows[0]
			t.Cleanup(w.Destroy)
			backend := &presentationPlatform{placementPlatform: plat, scale: 1}
			app.platform = backend
			p := NewSharedPopover(base.anchor).(*sharedPopover)
			t.Cleanup(p.Destroy)
			content := &presentationWidget{size: geometry.Size{Width: 100, Height: 30}}
			content.onArrange = func() {
				switch action {
				case "unmount":
					w.SetWidget(nil)
				case "destroy":
					w.Destroy()
				case "hide":
					p.Hide()
				case "nonconverging":
					content.RequestLayout()
				}
			}
			p.SetWidget(content)
			if err := p.Show(); err == nil || p.Visible() || backend.native.shows != 0 || p.Visible() {
				t.Fatalf("取消或不收敛后仍提交显示：%v", err)
			}
		})
	}
}

func TestTransientPresentationFirstFrameCancellation(t *testing.T) {
	base, _, plat, app := placementFixture(t, geometry.Rect(0, 0, 500, 400))
	w := app.windows[0]
	t.Cleanup(w.Destroy)
	backend := &presentationPlatform{placementPlatform: plat, scale: 1}
	app.platform = backend
	p := NewSharedPopover(base.anchor).(*sharedPopover)
	t.Cleanup(p.Destroy)
	p.SetWidget(&presentationWidget{size: geometry.Size{Width: 100, Height: 30}})
	p.Widget().ConnectMount(func() {
		p.Widget().Root().ConnectFrame(func(_ time.Time) {
			if !p.Visible() || len(w.Popovers()) != 1 || w.Popovers()[0].Widget() != p.Widget() {
				t.Fatal("首帧回调观察到未提交的可见状态")
			}
			p.Hide()
		})
	})
	if err := p.Show(); err == nil || p.Visible() || w.shared.active != nil || len(w.Popovers()) != 0 {
		t.Fatalf("首帧取消后仍报告显示成功或保留借用：%v", err)
	}
}

func TestTransientPresentationMoveChangesNativeScale(t *testing.T) {
	base, _, plat, app := placementFixture(t, geometry.Rect(0, 0, 500, 400))
	t.Cleanup(app.windows[0].Destroy)
	backend := &presentationPlatform{placementPlatform: plat, scale: 1}
	app.platform = backend
	p := NewSharedPopover(base.anchor).(*sharedPopover)
	t.Cleanup(p.Destroy)
	p.SetWidget(&presentationWidget{size: geometry.Size{Width: 100, Height: 30}})
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	p.Hide()
	base.anchor.Arrange(geometry.Rect(460, 340, 30, 20))
	p.SetModal(true)
	p.SetPlacement(PopoverPlacementBottom)
	n := backend.native
	n.onPosition = func() {
		n.onPosition = nil
		// 移到另一显示器后，原生 backing 保持原像素，逻辑尺寸先按新 scale 报告；
		// 没有额外尺寸通知，呈现器必须查询后按新 scale 重新提交请求。
		n.scale = 2
		n.actual.Width, n.actual.Height = n.actual.PixelWidth/2, n.actual.PixelHeight/2
	}
	n.onShow = func() {
		if testSharedCore(p).width != 100 || testSharedCore(p).height != 30 || testSharedCore(p).pixelWidth != 200 || testSharedCore(p).pixelHeight != 60 || n.position != (geometry.Point{X: 390, Y: 360}) {
			t.Fatalf("换屏后仍使用旧 backing 或旧位置：logical=(%g,%g) pixels=(%g,%g) position=%v", testSharedCore(p).width, testSharedCore(p).height, testSharedCore(p).pixelWidth, testSharedCore(p).pixelHeight, n.position)
		}
	}
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
}

func TestTransientPresentationClosedCallbackCannotNestNativePaint(t *testing.T) {
	_, _, makePopup := transientFixture(t)
	first, next := makePopup(nil), makePopup(nil)
	first.Widget().ConnectMount(func() { first.Widget().Root().ConnectFrame(func(time.Time) { first.Hide() }) })
	called := false
	first.ConnectClosed(func() {
		called = true
		if err := next.Show(); err == nil || next.Visible() {
			t.Fatal("原生绘制分发尚未退出，就将同一个绘制上下文借给了另一个 presenter")
		}
	})
	if err := first.Show(); err == nil || !called {
		t.Fatal("没有完成首帧取消及关闭通知")
	}
	if err := next.Show(); err != nil {
		t.Fatalf("原生分发退出后仍不能借用：%v", err)
	}
}

func TestSharedPresentationCancelWhileClearing(t *testing.T) {
	base, _, plat, app := placementFixture(t, geometry.Rect(0, 0, 500, 400))
	t.Cleanup(app.windows[0].Destroy)
	backend := &presentationPlatform{placementPlatform: plat, scale: 1}
	app.platform = backend
	p := NewSharedPopover(base.anchor).(*sharedPopover)
	t.Cleanup(p.Destroy)
	content := &presentationWidget{size: geometry.Size{Width: 100, Height: 30}}
	// 初次 Arrange 时 native 已创建；在隐藏清除回调取消显示。
	content.onArrange = func() { backend.native.onRepaint = func() { p.Hide() } }
	p.SetWidget(content)
	if err := p.Show(); !errors.Is(err, ErrPopoverSuperseded) || backend.native.shows != 0 || p.Visible() || len(app.windows[0].Popovers()) != 0 {
		t.Fatalf("清除期间取消后仍 Map 或留下失活显示：%v", err)
	}
}

type invalidatingPresentationPainter struct {
	graphics.Painter
	backend *invalidatingPresentationPlatform
}

func (p *invalidatingPresentationPainter) InvalidatePresentation() error {
	p.backend.invalidations++
	return p.backend.failure
}

type invalidatingPresentationPlatform struct {
	*presentationPlatform
	failure       error
	invalidations int
}

func (p *invalidatingPresentationPlatform) NewPainter(surface platform.Surface) (graphics.Painter, error) {
	painter, err := p.presentationPlatform.NewPainter(surface)
	return &invalidatingPresentationPainter{Painter: painter, backend: p}, err
}
func TestSharedPresentationInvalidationFailure(t *testing.T) {
	base, _, plat, app := placementFixture(t, geometry.Rect(0, 0, 500, 400))
	t.Cleanup(app.windows[0].Destroy)
	failure := errors.New("discard failed")
	backend := &invalidatingPresentationPlatform{presentationPlatform: &presentationPlatform{placementPlatform: plat, scale: 1}, failure: failure}
	app.platform = backend
	p := NewSharedPopover(base.anchor)
	t.Cleanup(p.Destroy)
	p.SetWidget(&presentationWidget{size: geometry.Size{Width: 100, Height: 30}})
	closed := 0
	p.ConnectClosed(func() { closed++ })
	if err := p.Show(); !errors.Is(err, failure) || backend.native.shows != 0 || p.Visible() || closed != 0 {
		t.Fatalf("失效失败仍显示或丢失错误：%v", err)
	}
	native := backend.native
	backend.failure = nil
	if err := p.Show(); err != nil || backend.native != native || !p.Visible() || backend.invalidations != 2 {
		t.Fatalf("失效失败后的重开重建了资源或未重新失效：%v", err)
	}
}
