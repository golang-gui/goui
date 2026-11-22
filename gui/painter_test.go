package gui

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/style"
)

func TestPainterUsesWidgetLocalCoordinates(t *testing.T) {
	backend := new(recordingPainterBackend)
	bounds := geometry.Rect(0, 0, 200, 100)
	widget := newPainterTestWidget(func(p Painter) {
		p.SetClipRect(geometry.Rect(4, 6, 20, 10))
		p.FillRect(geometry.Rect(1, 2, 3, 4), graphics.RGB(1, 2, 3))
	})
	widget.Arrange(geometry.Rect(10, 5, 100, 80))

	paintWidget(widget, newPainter(backend, bounds, 1))

	if len(backend.fills) != 1 {
		t.Fatalf("expected one fill, got %d", len(backend.fills))
	}
	fill := backend.fills[0]
	if fill.rect != geometry.Rect(1, 2, 3, 4) {
		t.Fatalf("draw coordinates must remain Widget-local, got %+v", fill.rect)
	}
	if fill.clip != geometry.Rect(14, 11, 20, 10) {
		t.Fatalf("unexpected translated clip: %+v", fill.clip)
	}
	if fill.transform != geometry.Translate(10, 5) {
		t.Fatalf("unexpected Widget transform: %+v", fill.transform)
	}
	if backend.clip != (geometry.Rectangle{}) {
		t.Fatalf("subtree exit must restore the full target clip, got %+v", backend.clip)
	}
	if backend.transform != geometry.Identity() {
		t.Fatalf("subtree exit must restore the identity transform, got %+v", backend.transform)
	}
}

func TestPainterAutomaticallyPaintsChildrenInTreeOrder(t *testing.T) {
	backend := new(recordingPainterBackend)
	bounds := geometry.Rect(0, 0, 200, 100)
	parent := newPainterTestWidget(fillAt(1))
	first := newPainterTestWidget(fillAt(2))
	hidden := newPainterTestWidget(fillAt(3))
	last := newPainterTestWidget(fillAt(4))
	parent.Arrange(bounds)
	first.Arrange(geometry.Rect(0, 0, 20, 20))
	hidden.Arrange(geometry.Rect(20, 0, 20, 20))
	last.Arrange(geometry.Rect(40, 0, 20, 20))
	hidden.SetVisible(false)
	parent.AddChild(first)
	parent.AddChild(hidden)
	parent.AddChild(last)

	paintWidget(parent, newPainter(backend, bounds, 1))

	if parent.paints != 1 || first.paints != 1 || hidden.paints != 0 || last.paints != 1 {
		t.Fatalf("unexpected paint counts: parent=%d first=%d hidden=%d last=%d",
			parent.paints, first.paints, hidden.paints, last.paints)
	}
	if len(backend.fills) != 3 {
		t.Fatalf("expected three visible fills, got %d", len(backend.fills))
	}
	for i, wantX := range []float32{1, 2, 4} {
		if backend.fills[i].rect.X != wantX {
			t.Fatalf("fill %d has X=%v, want %v", i, backend.fills[i].rect.X, wantX)
		}
	}
}

func TestWidgetBaseDefaultPaintStillTraversesChildren(t *testing.T) {
	backend := new(recordingPainterBackend)
	bounds := geometry.Rect(0, 0, 100, 100)
	parent := new(painterTestContainer)
	child := newPainterTestWidget(fillAt(7))
	parent.Arrange(bounds)
	child.Arrange(geometry.Rect(5, 6, 20, 20))
	parent.AddChild(child)

	paintWidget(parent, newPainter(backend, bounds, 1))

	if child.paints != 1 || len(backend.fills) != 1 {
		t.Fatalf("WidgetBase.Paint should be a no-op without suppressing descendants: paints=%d fills=%d",
			child.paints, len(backend.fills))
	}
}

func TestPainterRestoresParentStateBeforePaintingChild(t *testing.T) {
	backend := new(recordingPainterBackend)
	bounds := geometry.Rect(0, 0, 200, 100)
	parent := newPainterTestWidget(func(p Painter) {
		p.SetClipRect(geometry.Rect(1, 1, 5, 5))
		p.SetTransform(geometry.Scale(2, 2))
	})
	child := newPainterTestWidget(func(p Painter) {
		p.FillRect(geometry.Rect(0, 0, 30, 20), graphics.RGB(1, 2, 3))
	})
	parent.Arrange(geometry.Rect(5, 5, 100, 80))
	child.Arrange(geometry.Rect(20, 10, 30, 20))
	parent.AddChild(child)

	paintWidget(parent, newPainter(backend, bounds, 1))

	if len(backend.fills) != 1 {
		t.Fatalf("expected child fill, got %d", len(backend.fills))
	}
	fill := backend.fills[0]
	if fill.clip != geometry.Rect(25, 15, 30, 20) {
		t.Fatalf("parent explicit clip leaked to child: %+v", fill.clip)
	}
	if fill.transform != geometry.Translate(25, 15) {
		t.Fatalf("parent transform leaked to child: %+v", fill.transform)
	}
}

func TestPainterStructuralViewportClipDoesNotMoveWithContent(t *testing.T) {
	backend := new(recordingPainterBackend)
	bounds := geometry.Rect(0, 0, 200, 200)
	root := newPainterTestWidget(nil)
	viewport := newPainterTestWidget(nil)
	content := newPainterTestWidget(func(p Painter) {
		p.FillRect(geometry.Rect(0, 0, 100, 300), graphics.RGB(1, 2, 3))
	})
	root.Arrange(bounds)
	viewport.Arrange(geometry.Rect(20, 20, 100, 100))
	content.Arrange(geometry.Rect(0, -50, 100, 300))
	root.AddChild(viewport)
	viewport.AddChild(content)

	paintWidget(root, newPainter(backend, bounds, 1))

	if len(backend.fills) != 1 {
		t.Fatalf("expected one fill, got %d", len(backend.fills))
	}
	fill := backend.fills[0]
	if fill.clip != geometry.Rect(20, 20, 100, 100) {
		t.Fatalf("viewport clip moved with content: %+v", fill.clip)
	}
	if fill.transform != geometry.Translate(20, -30) {
		t.Fatalf("unexpected scrolled content transform: %+v", fill.transform)
	}
}

// 先求完整逻辑交集；各层分别扩大到像素会让本已分离的子项重新变成可绘制。
func TestPainterKeepsFractionalIntersectionsUntilBackend(t *testing.T) {
	backend := new(recordingPainterBackend)
	parent := newPainterTestWidget(nil)
	child := newPainterTestWidget(func(p Painter) {
		p.SetClipRect(geometry.Rect(.75, 0, 5, 8))
		p.FillRect(geometry.Rect(0, 0, 20, 20), graphics.RGB(255, 0, 0))
	})
	outside := newPainterTestWidget(fillAt(2))
	parent.Arrange(geometry.Rect(10.25, 20.25, 10.5, 10.5))
	child.Arrange(geometry.Rect(-.5, .5, 11.5, 11.5))
	outside.Arrange(geometry.Rect(10.625, 0, .125, 1))
	parent.AddChild(child)
	parent.AddChild(outside)

	paintWidget(parent, newPainter(backend, geometry.Rect(0, 0, 100, 100), 1.25))
	if len(backend.fills) != 1 || outside.paints != 0 {
		t.Fatalf("逻辑分离的子项不应绘制：fills=%d outside=%d", len(backend.fills), outside.paints)
	}
	if got, want := backend.fills[0].clip, geometry.Rect(10.5, 20.75, 5, 8); got != want {
		t.Fatalf("后端应接收未取整的最终交集：%v，预期%v", got, want)
	}
}

func TestPainterSuppressesEmptyClipAndCanSetAnotherClip(t *testing.T) {
	backend := new(recordingPainterBackend)
	bounds := geometry.Rect(0, 0, 100, 100)
	widget := newPainterTestWidget(func(p Painter) {
		p.SetClipRect(geometry.Rect(200, 200, 10, 10))
		p.FillRect(bounds, graphics.RGB(1, 2, 3))

		// SetClipRect replaces the explicit clip within this Widget's structural
		// boundary; an empty preceding clip does not make later clips empty.
		p.SetClipRect(geometry.Rect(10, 10, 20, 20))
		p.FillRect(bounds, graphics.RGB(4, 5, 6))
	})
	widget.Arrange(bounds)

	paintWidget(widget, newPainter(backend, bounds, 1))

	if len(backend.fills) != 1 {
		t.Fatalf("expected drawing to resume with a non-empty clip, got %d fills", len(backend.fills))
	}
	if backend.fills[0].clip != geometry.Rect(10, 10, 20, 20) {
		t.Fatalf("unexpected replacement clip: %+v", backend.fills[0].clip)
	}
}

func TestPainterSaveRestoreRestoresClipAndTransform(t *testing.T) {
	backend := new(recordingPainterBackend)
	bounds := geometry.Rect(0, 0, 100, 100)
	widget := newPainterTestWidget(func(p Painter) {
		p.FillRect(geometry.Rect(1, 0, 1, 1), graphics.RGB(1, 2, 3))
		p.Save()
		p.SetClipRect(geometry.Rect(2, 3, 10, 11))
		p.FillRect(geometry.Rect(2, 0, 1, 1), graphics.RGB(1, 2, 3))
		p.Save()
		p.SetTransform(geometry.Scale(2, 3))
		p.FillRect(geometry.Rect(3, 0, 1, 1), graphics.RGB(1, 2, 3))
		p.Restore()
		p.FillRect(geometry.Rect(4, 0, 1, 1), graphics.RGB(1, 2, 3))
		p.Restore()
		p.FillRect(geometry.Rect(5, 0, 1, 1), graphics.RGB(1, 2, 3))
	})
	widget.Arrange(geometry.Rect(10, 5, 80, 70))

	paintWidget(widget, newPainter(backend, bounds, 1))

	if len(backend.fills) != 5 {
		t.Fatalf("expected five fills, got %d", len(backend.fills))
	}
	defaultClip := geometry.Rect(10, 5, 80, 70)
	defaultTransform := geometry.Translate(10, 5)
	if backend.fills[0].clip != defaultClip || backend.fills[0].transform != defaultTransform {
		t.Fatalf("unexpected initial state: %+v", backend.fills[0])
	}
	narrowClip := geometry.Rect(12, 8, 10, 11)
	if backend.fills[1].clip != narrowClip || backend.fills[1].transform != defaultTransform {
		t.Fatalf("unexpected outer saved state: %+v", backend.fills[1])
	}
	wantTransform := defaultTransform.Multiply(geometry.Scale(2, 3))
	if backend.fills[2].clip != narrowClip || backend.fills[2].transform != wantTransform {
		t.Fatalf("unexpected nested saved state: %+v", backend.fills[2])
	}
	if backend.fills[3].clip != narrowClip || backend.fills[3].transform != defaultTransform {
		t.Fatalf("inner Restore did not restore the outer state: %+v", backend.fills[3])
	}
	if backend.fills[4].clip != defaultClip || backend.fills[4].transform != defaultTransform {
		t.Fatalf("outer Restore did not restore initial state: %+v", backend.fills[4])
	}
}

func TestPainterRejectsRestoreAcrossWidgetBoundary(t *testing.T) {
	backend := new(recordingPainterBackend)
	bounds := geometry.Rect(0, 0, 100, 100)
	widget := newPainterTestWidget(func(p Painter) { p.Restore() })
	widget.SetID("broken")
	widget.Arrange(bounds)

	message := recoverMessage(func() {
		paintWidget(widget, newPainter(backend, bounds, 1))
	})
	if !strings.Contains(message, "without matching Save") || !strings.Contains(message, `id "broken"`) {
		t.Fatalf("unexpected panic: %q", message)
	}
	if backend.clip != (geometry.Rectangle{}) || backend.transform != geometry.Identity() {
		t.Fatalf("failed Paint must restore backend state: clip=%+v transform=%+v", backend.clip, backend.transform)
	}
}

func TestPainterReportsUnbalancedSaveAndRestoresState(t *testing.T) {
	backend := new(recordingPainterBackend)
	bounds := geometry.Rect(0, 0, 100, 100)
	p := newPainter(backend, bounds, 1)
	broken := newPainterTestWidget(func(p Painter) {
		p.Save()
		p.SetClipRect(geometry.Rect(1, 2, 3, 4))
	})
	broken.SetID("broken")
	broken.Arrange(bounds)

	message := recoverMessage(func() { paintWidget(broken, p) })
	if !strings.Contains(message, "1 unbalanced Painter.Save") || !strings.Contains(message, `id "broken"`) {
		t.Fatalf("unexpected panic: %q", message)
	}

	healthy := newPainterTestWidget(func(p Painter) {
		p.FillRect(bounds, graphics.RGB(1, 2, 3))
	})
	healthy.Arrange(bounds)
	paintWidget(healthy, p)
	if len(backend.fills) != 1 || backend.fills[0].clip != (geometry.Rectangle{}) {
		t.Fatalf("painter was not reusable after an unbalanced Save: %+v", backend.fills)
	}
}

func TestPainterRestoresStateAfterWidgetPanic(t *testing.T) {
	backend := new(recordingPainterBackend)
	bounds := geometry.Rect(0, 0, 100, 100)
	p := newPainter(backend, bounds, 1)
	broken := newPainterTestWidget(func(p Painter) {
		p.Save()
		p.SetClipRect(geometry.Rect(1, 2, 3, 4))
		p.SetTransform(geometry.Scale(2, 2))
		panic("paint failed")
	})
	broken.Arrange(bounds)

	message := recoverMessage(func() { paintWidget(broken, p) })
	if message != "paint failed" {
		t.Fatalf("original Paint panic was not preserved: %q", message)
	}
	if backend.clip != (geometry.Rectangle{}) || backend.transform != geometry.Identity() {
		t.Fatalf("panic did not restore backend state: clip=%+v transform=%+v", backend.clip, backend.transform)
	}

	healthy := newPainterTestWidget(fillAt(9))
	healthy.Arrange(bounds)
	paintWidget(healthy, p)
	if len(backend.fills) != 1 || backend.fills[0].rect.X != 9 {
		t.Fatalf("painter was not reusable after panic: %+v", backend.fills)
	}
}

func recoverMessage(fn func()) (message string) {
	defer func() {
		if value := recover(); value != nil {
			message = fmt.Sprint(value)
		}
	}()
	fn()
	return ""
}

func fillAt(x float32) func(Painter) {
	return func(p Painter) {
		p.FillRect(geometry.Rect(x, 0, 1, 1), graphics.RGB(1, 2, 3))
	}
}

type painterTestWidget struct {
	WidgetBase
	paint  func(Painter)
	paints int
}

type painterTestContainer struct {
	WidgetBase
}

func (w *painterTestContainer) AddChild(child Widget) {
	w.WidgetBase.AddChild(w, child)
}

func newPainterTestWidget(paint func(Painter)) *painterTestWidget {
	return &painterTestWidget{paint: paint}
}

func (w *painterTestWidget) AddChild(child Widget) {
	w.WidgetBase.AddChild(w, child)
}

func (w *painterTestWidget) Paint(p Painter) {
	w.paints++
	if w.paint != nil {
		w.paint(p)
	}
}

type recordedFill struct {
	rect      geometry.Rectangle
	clip      geometry.Rectangle
	transform geometry.Transform
}

type recordingPainterBackend struct {
	clip      geometry.Rectangle
	transform geometry.Transform
	fills     []recordedFill
	shadows   []recordedShadow
}

func (p *recordingPainterBackend) Name() string                                     { return "recording" }
func (p *recordingPainterBackend) SnapRect(r geometry.Rectangle) geometry.Rectangle { return r }
func (p *recordingPainterBackend) Destroy()                                         {}
func (p *recordingPainterBackend) RenderImage(int, int, float32, func()) (image.Image, error) {
	return nil, fmt.Errorf("recording painter does not rasterize")
}
func (p *recordingPainterBackend) NewImage(src image.Image) (graphics.Image, error) {
	return newTestNativeImage(src), nil
}
func (p *recordingPainterBackend) Begin(width, height, scale float32) {}
func (p *recordingPainterBackend) End()                               {}
func (p *recordingPainterBackend) SetClipRect(rect graphics.Rectangle) {
	p.clip = rect
}
func (p *recordingPainterBackend) Clear(color graphics.Color) {}
func (p *recordingPainterBackend) DrawBoxShadow(rect graphics.Rectangle, radius geometry.CornerRadius, shadow graphics.BoxShadow) {
	p.shadows = append(p.shadows, recordedShadow{rect, radius, shadow, p.clip, p.transform})
}
func (p *recordingPainterBackend) FillRect(rect graphics.Rectangle, brush graphics.Brush) {
	p.fills = append(p.fills, recordedFill{rect: rect, clip: p.clip, transform: p.transform})
}
func (p *recordingPainterBackend) FillRoundRect(rect graphics.Rectangle, radius float32, brush graphics.Brush) {
}
func (p *recordingPainterBackend) FillEllipse(center graphics.Point, xRadius, yRadius float32, brush graphics.Brush) {
}
func (p *recordingPainterBackend) FillPath(path graphics.Path, brush graphics.Brush) {}
func (p *recordingPainterBackend) DrawLine(p0, p1 graphics.Point, strokeWidth float32, brush graphics.Brush) {
}
func (p *recordingPainterBackend) DrawRect(rect graphics.Rectangle, strokeWidth float32, brush graphics.Brush) {
}
func (p *recordingPainterBackend) DrawRoundRect(rect graphics.Rectangle, radius, strokeWidth float32, brush graphics.Brush) {
}
func (p *recordingPainterBackend) DrawEllipse(center graphics.Point, xRadius, yRadius, strokeWidth float32, brush graphics.Brush) {
}
func (p *recordingPainterBackend) DrawPath(path graphics.Path, strokeWidth float32, brush graphics.Brush) {
}
func (p *recordingPainterBackend) DrawTextLayout(origin graphics.Point, layout typography.TextLayout) {
}
func (p *recordingPainterBackend) DrawImage(rect graphics.Rectangle, img graphics.Image) {}
func (p *recordingPainterBackend) SetTransform(matrix geometry.Transform) {
	p.transform = matrix
}

type recordedShadow struct {
	rect      geometry.Rectangle
	radius    geometry.CornerRadius
	shadow    graphics.BoxShadow
	clip      geometry.Rectangle
	transform geometry.Transform
}

func TestPainterBoxShadowUsesLocalStateAndSuppressesEmptyClip(t *testing.T) {
	backend := new(recordingPainterBackend)
	shadow := graphics.BoxShadow{Color: graphics.RGB(10, 20, 30), Offset: geometry.Point{Y: 2}, BlurRadius: 6, SpreadRadius: 1}
	widget := newPainterTestWidget(func(p Painter) {
		p.DrawBoxShadow(geometry.Rect(4, 5, 20, 10), geometry.UniformCornerRadius(3), shadow)
		p.SetClipRect(geometry.Rect(200, 200, 1, 1))
		p.DrawBoxShadow(geometry.Rect(4, 5, 20, 10), geometry.UniformCornerRadius(3), shadow)
	})
	widget.Arrange(geometry.Rect(10, 15, 80, 60))
	paintWidget(widget, newPainter(backend, geometry.Rect(0, 0, 200, 200), 2))
	if len(backend.shadows) != 1 {
		t.Fatalf("shadow count=%d", len(backend.shadows))
	}
	got := backend.shadows[0]
	if got.rect != geometry.Rect(4, 5, 20, 10) || got.radius != geometry.UniformCornerRadius(3) || got.shadow != shadow || got.clip != geometry.Rect(10, 15, 80, 60) || got.transform != geometry.Translate(10, 15) {
		t.Fatalf("shadow state: %+v", got)
	}
}

func (p *recordingPainterBackend) ClearPath(path graphics.Path) {}

func TestPainterSnapRectTracksWidgetScope(t *testing.T) {
	// 使用真实 Software Painter，验证 GUI 局部坐标到后端像素网格的完整边界。
	for _, scale := range []float32{1, 1.25, 2} {
		for _, transform := range []geometry.Transform{
			geometry.Identity(), geometry.Translate(.25, -.5),
			geometry.Rotate(90), geometry.Scale(1.2, .8),
			{A11: 1, A12: .2, A22: 1},
		} {
			t.Run(fmt.Sprintf("scale=%g/transform=%v", scale, transform), func(t *testing.T) {
				backend, err := software.NewPainter(&widgetRenderSurface{})
				if err != nil {
					t.Fatal(err)
				}
				defer backend.Destroy()
				backend.Begin(120*scale, 100*scale, scale)
				defer backend.End()
				r := geometry.Rect(.3, .7, 36.2, 20.4)
				want := r
				if transform.A11 == 1 && transform.A22 == 1 && transform.A12 == 0 && transform.A21 == 0 {
					// 父 (4.25,8.5) + 子 (10.25,4.5) + 子自身的用户平移。
					edge := func(v, offset float32) float32 {
						return float32(math.Floor(float64((v+offset)*scale)+.5))/scale - offset
					}
					x, y := float32(14.5)+transform.TX, float32(13)+transform.TY
					left, top := edge(r.X, x), edge(r.Y, y)
					want = geometry.Rect(left, top, edge(r.X+r.Width, x)-left, edge(r.Y+r.Height, y)-top)
				}
				painted := false
				child := newPainterTestWidget(func(p Painter) {
					painted = true
					p.SetTransform(transform)
					check := func() {
						t.Helper()
						if got := p.SnapRect(r); got != want || p.SnapRect(got) != got {
							t.Fatalf("局部对齐/幂等错误：got=%v want=%v", got, want)
						}
					}
					check()
					p.Save()
					p.SetClipRect(geometry.Rect(1000, 1000, 1, 1))
					check() // 空 clip 不应改变纯几何查询或变换。
					p.SetTransform(geometry.Rotate(15))
					if p.SnapRect(r) != r {
						t.Fatal("用户旋转被量化")
					}
					p.Restore()
					check()
				})
				parent := newPainterTestWidget(nil)
				parent.Arrange(geometry.Rect(4.25, 8.5, 100, 80))
				child.Arrange(geometry.Rect(10.25, 4.5, 64, 48))
				parent.AddChild(child)
				paintWidget(parent, newPainter(backend, geometry.Rect(0, 0, 120, 100), scale))
				if !painted {
					t.Fatal("没有经过实际控件绘制作用域")
				}
			})
		}
	}
}

// --- 绘制可见性与绘制请求 ---

func TestDrawableMatchesStructuralPaintWithoutContainerTypes(t *testing.T) {
	w := testWindowWithSurface(&window{surfaceBase: surfaceBase{rootBase: rootBase{width: 80, height: 80, painter: &recordingPainterBackend{}}}})
	root := &painterTestContainer{}
	viewport := newPainterTestWidget(nil)
	content := &painterTestContainer{}
	inside, partial, outside := newPainterTestWidget(nil), newPainterTestWidget(nil), newPainterTestWidget(nil)
	escape := newPainterTestWidget(nil)
	root.AddChild(viewport)
	viewport.AddChild(content)
	content.AddChild(inside)
	content.AddChild(partial)
	content.AddChild(outside)
	outside.AddChild(escape)
	w.SetWidget(root)
	t.Cleanup(func() { w.SetWidget(nil) })
	root.Arrange(geometry.Rect(0, 0, 80, 80))
	viewport.Arrange(geometry.Rect(10, 5, 50, 40))
	content.Arrange(geometry.Rect(0, -100, 50, 300))
	inside.Arrange(geometry.Rect(0, 110, 20, 20))
	partial.Arrange(geometry.Rect(0, 130, 20, 20))
	outside.Arrange(geometry.Rect(0, 140, 20, 20))
	escape.Arrange(geometry.Rect(0, -130, 20, 20))
	if IsDrawable(inside) {
		t.Fatal("未准备宿主绘制区域时报告可绘制")
	}
	w.drawFrame(root, style.Style{})
	for _, tc := range []struct {
		name string
		w    *painterTestWidget
		want bool
	}{{"完全可见", inside, true}, {"部分可见", partial, true}, {"完全裁出", outside, false}, {"不能逃出父裁剪", escape, false}} {
		if IsDrawable(tc.w) != tc.want || (tc.w.paints > 0) != tc.want {
			t.Fatalf("%s：查询=%v Paint次数=%d", tc.name, IsDrawable(tc.w), tc.w.paints)
		}
	}
	if allocations := testing.AllocsPerRun(100, func() { _ = IsDrawable(inside) }); allocations != 0 {
		t.Fatal("父链查询发生堆分配", allocations)
	}
	if !outside.Snapshot().Visible {
		t.Fatal("裁出改变了语义 Visible")
	}
	viewport.SetVisible(false)
	if IsDrawable(inside) {
		t.Fatal("忽略隐藏祖先")
	}
	viewport.SetVisible(true)
	w.suspendFrames()
	if IsDrawable(inside) {
		t.Fatal("宿主暂停后仍可绘制")
	}
	w.frameSuspended = false
	if IsDrawable(inside) {
		t.Fatal("恢复后复用了旧根裁剪")
	}
	w.drawDecoratedFrame(root, style.Style{}, style.Style{}, 30)
	if IsDrawable(inside) {
		t.Fatal("查询忽略宿主安全裁剪")
	}
	inside.Arrange(geometry.Rectangle{})
	if IsDrawable(inside) || IsDrawable(nil) || IsDrawable(newPainterTestWidget(nil)) {
		t.Fatal("空尺寸或未挂载控件可绘制")
	}
}

func TestDrawableFiltersOnlyStableWidgetPaintRequests(t *testing.T) {
	d, app := startTestDiagnostics(t, true)
	w := testWindowWithSurface(&window{surfaceBase: surfaceBase{rootBase: rootBase{app: app, width: 100, height: 80, painter: &recordingPainterBackend{}}}})
	root := &painterTestContainer{}
	child := newPainterTestWidget(nil)
	root.AddChild(child)
	w.SetWidget(root)
	t.Cleanup(func() { w.SetWidget(nil) })
	root.Arrange(geometry.Rect(0, 0, 100, 80))
	child.Arrange(geometry.Rect(0, 100, 20, 20))
	w.drawFrame(root, style.Style{})
	w.layoutDirty, w.paintDirty, w.framePending = false, false, false
	w.stateStyles = nil
	w.platformWindow = &showTestWindow{}
	r := diagnosticRoot(&w.rootBase)
	before := r.PaintRequests
	for range 100 {
		child.RequestPaint()
	}
	if r.PaintRequests != before || w.paintDirty {
		t.Fatal("离屏请求唤醒了宿主")
	}
	var intents uint64
	for _, stat := range d.Snapshot().Widgets {
		if stat.Type == "*gui.painterTestWidget" {
			intents += stat.PaintRequests
		}
	}
	if intents != 100 {
		t.Fatal("过滤丢失了请求意图计数", intents)
	}
	w.layoutDirty = true
	child.RequestPaint()
	if r.PaintRequests != before+1 {
		t.Fatal("使用旧矩形丢弃了待布局请求")
	}
	w.layoutDirty = false
	w.stateStyles = map[Widget]stateMeasurement{child: {}}
	child.RequestPaint()
	if r.PaintRequests != before+2 {
		t.Fatal("丢弃了状态样式可能引起的布局变化")
	}
	w.stateStyles = nil
	w.screenClipValid = false
	child.RequestPaint()
	if r.PaintRequests != before+3 {
		t.Fatal("未知根裁剪没有保守转发")
	}
	w.screenClipValid = true
	child.Arrange(geometry.Rect(0, 70, 20, 20))
	child.RequestPaint()
	if r.PaintRequests != before+4 {
		t.Fatal("部分可见请求没有转发")
	}
	w.suspendFrames()
	child.RequestPaint()
	if r.PaintRequests != before+4 {
		t.Fatal("暂停宿主仍转发 Widget 绘制请求")
	}
	child.RequestLayout()
	if !w.layoutDirty || r.PaintRequests != before+5 {
		t.Fatal("暂停或离屏导致布局失效丢失")
	}
}

func TestDrawableDoesNotRestrictRenderWidgetTarget(t *testing.T) {
	for _, scale := range []float32{1, 2} {
		t.Run(map[float32]string{1: "1x", 2: "2x"}[scale], func(t *testing.T) {
			output := &transparentFrame{}
			backend, err := software.NewPainter(output)
			if err != nil {
				t.Fatal(err)
			}
			defer backend.Destroy()
			w := testWindowWithSurface(&window{surfaceBase: surfaceBase{rootBase: rootBase{width: 30, height: 20, pixelWidth: 30 * scale, pixelHeight: 20 * scale, transparent: true, painter: backend}}})
			root := &painterTestContainer{}
			value := graphics.RGBA(60, 100, 180, 128)
			child := newPainterTestWidget(func(p Painter) {
				p.SetTransform(geometry.Translate(1, 1))
				p.FillRect(geometry.Rect(0, 0, 8, 6), value)
			})
			root.AddChild(child)
			w.SetWidget(root)
			defer w.SetWidget(nil)
			root.Arrange(geometry.Rect(0, 0, 30, 20))
			child.Arrange(geometry.Rect(2, 40, 10, 8))
			w.drawFrame(root, style.Style{})
			if IsDrawable(child) || child.paints != 0 {
				t.Fatal("屏幕裁出区域仍调用 Paint")
			}
			frames, after := 0, 0
			w.ConnectFrame(func(_ time.Time) { frames++ })
			w.ConnectAfterPaint(func(Painter, geometry.Rectangle) { after++ })
			clip := w.screenClip
			img, err := RenderWidget(child, scale)
			if err != nil {
				t.Fatal(err)
			}
			// 采样填充内部与透明边缘，预乘 RGBA8：分量乘以 128/255 并取整。
			inside := color.RGBAModel.Convert(img.At(3*int(scale), 3*int(scale))).(color.RGBA)
			outside := color.RGBAModel.Convert(img.At(0, 0)).(color.RGBA)
			if inside != (color.RGBA{R: 30, G: 50, B: 90, A: 128}) || outside != (color.RGBA{}) {
				t.Fatal("离屏裁剪、变换或预乘像素不正确", inside, outside)
			}
			if frames != 0 || after != 0 || w.screenClip != clip || IsDrawable(child) || child.paints != 1 {
				t.Fatal("离屏绘制影响了屏幕续帧或根裁剪")
			}
		})
	}
}
