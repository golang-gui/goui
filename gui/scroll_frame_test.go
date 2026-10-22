package gui

import (
	"fmt"
	"image/color"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/style"
)

func scrollFrameScope(sv *ScrollView, radius float32) *StyleScope {
	sv.SetStyleName("editor")
	scope := NewStyleScope()
	scope.SetStyleSheet(style.Sheet(
		style.Name("editor").BorderWidth(0).Radius(0),
		style.Name("editor").Part("frame").BackgroundColor(color.White).BorderColor(color.RGBA{R: 255, A: 255}).BorderWidth(1).Radius(radius),
		style.Name("editor").Part("frame").State(style.Focused).BorderColor(color.RGBA{B: 255, A: 255}).BorderWidth(2),
		style.Name("editor").Part("frame").State(style.Disabled).BorderWidth(2),
		style.Name("editor").Part("trough").BackgroundColor(color.RGBA{G: 255, A: 255}).Radius(4),
		style.Name("editor").Part("thumb").BackgroundColor(color.RGBA{G: 128, A: 255}).Radius(4),
	))
	scope.SetChild(sv)
	return scope
}

func TestScrollViewFrameContainsViewportAndBars(t *testing.T) {
	sv := NewScrollView()
	content := &mockWidget{size: geometry.Size{Width: 200, Height: 300}}
	sv.SetChild(content)
	scope := scrollFrameScope(sv, 0)
	win := &window{}
	win.SetWidget(scope)
	defer win.Destroy()
	content.SetFocusable(true)
	sv.Measure(layout.Tight(geometry.Size{Width: 100, Height: 100}))
	sv.Arrange(geometry.Rect(10, 20, 100, 100))
	want := geometry.Rect(2, 2, 88, 88)
	if sv.viewport.Rect() != want || sv.vbar.Rect() != geometry.Rect(90, 2, 8, 88) || sv.hbar.Rect() != geometry.Rect(2, 90, 88, 8) {
		t.Fatalf("frame geometry: viewport=%v vbar=%v hbar=%v", sv.viewport.Rect(), sv.vbar.Rect(), sv.hbar.Rect())
	}
	if info := sv.Snapshot(); info.MaxScrollX != 112 || info.MaxScrollY != 212 || len(info.Children) != 1 {
		t.Fatalf("frame range or semantic children: %+v", info)
	}
	win.SetFocusedWidget(content)
	sv.Arrange(sv.Rect())
	if !sv.ContainsFocus() || sv.viewport.Rect() != want {
		t.Fatal("focus moved viewport")
	}
	sv.ScrollIntoView(geometry.Rect(199, 299, 1, 1))
	if sv.ScrollX() != 112 || sv.ScrollY() != 212 || content.Rect().Pos != (geometry.Point{X: -112, Y: -212}) {
		t.Fatal("framed reveal used outer coordinates")
	}
	sv.SetEnabled(false)
	sv.Arrange(sv.Rect())
	if sv.viewport.Rect() != want {
		t.Fatal("disabled frame changed wrapping")
	}
}

func TestScrollViewFrameMeasureAndTinyAllocations(t *testing.T) {
	sv := NewScrollView()
	sv.SetChild(&mockWidget{size: geometry.Size{Width: 30, Height: 20}})
	scope := scrollFrameScope(sv, 0)
	defer scope.destroy(scope)
	if got := sv.Measure(layout.Unbounded()).Size; got != (geometry.Size{Width: 34, Height: 24}) {
		t.Fatalf("unbounded frame measurement: %v", got)
	}
	for _, size := range []geometry.Size{{Width: 100, Height: 100}, {Width: 3, Height: 2}, {}, {Width: 1, Height: 100}} {
		sv.Measure(layout.Tight(size))
		sv.Arrange(geometry.Rectangle{Size: size})
		for _, w := range []Widget{sv.viewport, sv.vbar, sv.hbar} {
			r := w.Rect()
			if r.X < 0 || r.Y < 0 || r.Width < 0 || r.Height < 0 || r.X+r.Width > size.Width || r.Y+r.Height > size.Height {
				t.Fatalf("child outside tiny frame: size=%v rect=%v", size, r)
			}
		}
	}
	sv.Arrange(geometry.Rect(0, 0, 100, 100))
	if sv.vbar.Visible() || sv.hbar.Visible() || sv.viewport.Rect() != geometry.Rect(2, 2, 96, 96) {
		t.Fatal("short content should have a framed viewport without bars")
	}
}

type frameMeasuredContent struct {
	WidgetBase
	constraint layout.Constraint
	viewport   geometry.Size
}

func (c *frameMeasuredContent) Measure(cs layout.Constraint) layout.Measurement {
	c.constraint = cs
	return layout.MeasuredWithBaseline(cs.Clamp(geometry.Size{Width: 30, Height: 20}), 12)
}
func (c *frameMeasuredContent) ContentSize() geometry.Size {
	return geometry.Size{Width: 200, Height: 300}
}
func (c *frameMeasuredContent) LayoutVisible(size geometry.Size, _ geometry.Point) {
	c.viewport = size
}

func TestScrollViewFrameScrollableMeasurement(t *testing.T) {
	sv := NewScrollView()
	content := new(frameMeasuredContent)
	sv.SetChild(content)
	scope := scrollFrameScope(sv, 0)
	defer scope.destroy(scope)
	m := sv.Measure(layout.Loose(geometry.Size{Width: 100, Height: 100}))
	// 子项得到扣除稳定外框后的宽度；横条占高，基线仅增加顶部边距。
	if m.Size != (geometry.Size{Width: 100, Height: 32}) || !m.HasBaseline || m.Baseline != 14 || content.constraint.Min.Width != 96 || content.constraint.Max.Width != 96 {
		t.Fatalf("virtual frame measurement: result=%+v constraint=%+v", m, content.constraint)
	}
	sv.Arrange(geometry.Rect(0, 0, 100, 100))
	if content.viewport != (geometry.Size{Width: 88, Height: 88}) || content.Rect().Pos != (geometry.Point{}) {
		t.Fatalf("virtual content did not retain viewport-local coordinates: area=%v rect=%v", content.viewport, content.Rect())
	}
	m = sv.Measure(layout.Unbounded())
	if content.constraint.Max != (geometry.Size{Width: layout.Inf, Height: layout.Inf}) || m.Size != (geometry.Size{Width: 34, Height: 24}) || m.Baseline != 14 {
		t.Fatalf("unbounded virtual frame: result=%+v constraint=%+v", m, content.constraint)
	}
}

func TestScrollViewRoundedFramePixels(t *testing.T) {
	for _, scale := range []float32{1, 1.5, 2} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			setTestApplication(t, nil)
			backend, err := software.NewPainter(&widgetRenderSurface{})
			if err != nil {
				t.Fatal(err)
			}
			defer backend.Destroy()
			content := newPainterTestWidget(func(p Painter) { p.FillRect(geometry.Rect(0, 0, 200, 300), graphics.RGB(255, 0, 255)) })
			content.SetMinSize(geometry.Size{Width: 200, Height: 300})
			sv := NewScrollView()
			sv.SetChild(content)
			scope := scrollFrameScope(sv, 12)
			win := &window{rootBase: rootBase{painter: backend}}
			win.SetWidget(scope)
			defer win.Destroy()
			// 固定不透明 sRGB 色，采样远离文字和抗锯齿边缘；角落应保持透明。
			// 整条中心直边至少覆盖一物理像素，不采用平台字体金图或放宽容差。
			sv.Measure(layout.Tight(geometry.Size{Width: 100, Height: 100}))
			sv.Arrange(geometry.Rect(0, 0, 100, 100))
			viewport := sv.viewport.Rect()
			for _, focused := range []bool{false, true} {
				if focused {
					content.SetFocusable(true)
					win.SetFocusedWidget(content)
				}
				sv.Arrange(sv.Rect())
				if sv.viewport.Rect() != viewport {
					t.Fatal("rounded focus frame changed viewport")
				}
				img, err := RenderWidget(sv, scale)
				if err != nil {
					t.Fatal(err)
				}
				corner := color.RGBAModel.Convert(img.At(0, 0)).(color.RGBA)
				edge := color.RGBAModel.Convert(img.At(0, int(50*scale))).(color.RGBA)
				want := color.RGBA{R: 255, A: 255}
				if focused {
					want = color.RGBA{B: 255, A: 255}
				}
				if corner.A != 0 || edge != want {
					t.Fatalf("frame overwritten: corner=%v edge=%v want=%v", corner, edge, want)
				}
				// 正文和滚动条都必须留在安全内矩形，不能覆盖外框或圆角。
				for y := 0; y < img.Bounds().Dy(); y++ {
					for x := 0; x < img.Bounds().Dx(); x++ {
						c := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
						if c != (color.RGBA{R: 255, B: 255, A: 255}) && c != (color.RGBA{G: 255, A: 255}) && c != (color.RGBA{G: 128, A: 255}) {
							continue
						}
						px, py := (float32(x)+.5)/scale, (float32(y)+.5)/scale
						inner := sv.innerRect()
						if px < inner.X-.5/scale || py < inner.Y-.5/scale || px > inner.X+inner.Width+.5/scale || py > inner.Y+inner.Height+.5/scale {
							t.Fatalf("child ink escaped frame at %d,%d", x, y)
						}
					}
				}
			}
		})
	}
}
