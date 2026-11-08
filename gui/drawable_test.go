package gui

import (
	"image/color"
	"testing"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/style"
)

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
