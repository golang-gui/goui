package gui

import (
	"fmt"
	"image/color"
	"math"
	"testing"

	"github.com/goexlib/mathx"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
)

func TestScrollViewCoupledBarsAndResize(t *testing.T) {
	for _, size := range []geometry.Size{{Width: 96, Height: 200}, {Width: 200, Height: 96}} {
		sv := NewScrollView()
		sv.SetChild(&mockWidget{size: size})
		sv.Measure(layout.Tight(geometry.Size{Width: 100, Height: 100}))
		sv.Arrange(geometry.Rect(0, 0, 100, 100))
		if !sv.vbar.Visible() || !sv.hbar.Visible() || sv.viewport.Rect() != geometry.Rect(0, 0, 92, 92) {
			t.Fatalf("bar-induced overflow not resolved: size=%v viewport=%v", size, sv.viewport.Rect())
		}
		info := sv.Snapshot()
		if info.MaxScrollX != size.Width-92 || info.MaxScrollY != size.Height-92 {
			t.Fatalf("snapshot ranges disagree with viewport: %+v", info)
		}
		sv.SetScrollX(1000)
		sv.SetScrollY(1000)
		sv.Arrange(geometry.Rect(0, 0, 220, 220))
		if sv.hbar.Visible() || sv.vbar.Visible() || sv.ScrollX() != 0 || sv.ScrollY() != 0 || sv.viewport.Rect() != geometry.Rect(0, 0, 220, 220) {
			t.Fatal("old bars or offsets survived a larger allocation")
		}
	}
}

// 独立列出 DIP 几何，不用被测占位函数生成预期；Measure 不能发布新几何。
func TestScrollViewBarOptionsGeometry(t *testing.T) {
	customH := ScrollBarOptions{Thickness: 10, ThumbThickness: 6, Spacing: 3}
	customV := ScrollBarOptions{Thickness: 20, ThumbThickness: 8, Spacing: 4}
	for _, tc := range []struct {
		name      string
		h, v      ScrollBarOptions
		direction ScrollDirection
		content   geometry.Size
		want      ScrollGeometry
		maximum   geometry.Point
	}{
		{"默认", ScrollBarOptions{}, ScrollBarOptions{}, ScrollBoth, geometry.Size{Width: 200, Height: 300},
			ScrollGeometry{geometry.Rect(0, 0, 92, 92), geometry.Rect(0, 92, 92, 8), geometry.Rect(92, 0, 8, 92), geometry.Rect(92, 92, 8, 8)}, geometry.Point{X: 108, Y: 208}},
		{"两轴独立厚度和间距", customH, customV, ScrollBoth, geometry.Size{Width: 200, Height: 300},
			ScrollGeometry{geometry.Rect(0, 0, 76, 87), geometry.Rect(0, 90, 76, 10), geometry.Rect(80, 0, 20, 87), geometry.Rect(80, 90, 20, 10)}, geometry.Point{X: 124, Y: 213}},
		{"隐藏不禁止滚动", ScrollBarOptions{Policy: ScrollBarHidden, Spacing: 20}, ScrollBarOptions{Policy: ScrollBarHidden, Spacing: 20}, ScrollBoth, geometry.Size{Width: 200, Height: 300},
			ScrollGeometry{Viewport: geometry.Rect(0, 0, 100, 100)}, geometry.Point{X: 100, Y: 200}},
		{"无范围仍占位", ScrollBarOptions{Policy: ScrollBarAlways}, ScrollBarOptions{Policy: ScrollBarAlways}, ScrollBoth, geometry.Size{Width: 20, Height: 20},
			ScrollGeometry{geometry.Rect(0, 0, 92, 92), geometry.Rect(0, 92, 92, 8), geometry.Rect(92, 0, 8, 92), geometry.Rect(92, 92, 8, 8)}, geometry.Point{}},
		{"Always迫使另一轴溢出", ScrollBarOptions{Policy: ScrollBarAlways}, ScrollBarOptions{}, ScrollBoth, geometry.Size{Width: 80, Height: 95},
			ScrollGeometry{geometry.Rect(0, 0, 92, 92), geometry.Rect(0, 92, 92, 8), geometry.Rect(92, 0, 8, 92), geometry.Rect(92, 92, 8, 8)}, geometry.Point{Y: 3}},
		{"禁止轴优先于Always", ScrollBarOptions{Policy: ScrollBarAlways}, ScrollBarOptions{}, ScrollVertical, geometry.Size{Width: 200, Height: 300},
			ScrollGeometry{Viewport: geometry.Rect(0, 0, 92, 100), VScrollBar: geometry.Rect(92, 0, 8, 100)}, geometry.Point{Y: 200}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sv := NewScrollView()
			sv.SetHScrollBarOptions(tc.h)
			sv.SetVScrollBarOptions(tc.v)
			sv.SetDirection(tc.direction)
			sv.SetChild(&mockWidget{size: tc.content})
			sv.Measure(layout.Tight(geometry.Size{Width: 100, Height: 100}))
			sv.Arrange(geometry.Rect(7, 9, 100, 100))
			info := sv.Snapshot()
			if got := sv.ScrollGeometry(); got != tc.want || sv.viewport.Rect() != tc.want.Viewport || sv.hbar.Rect() != tc.want.HScrollBar || sv.vbar.Rect() != tc.want.VScrollBar {
				t.Fatalf("布局=%+v，预期=%+v", got, tc.want)
			}
			if info.MaxScrollX != tc.maximum.X || info.MaxScrollY != tc.maximum.Y || len(info.Children) != 1 {
				t.Fatalf("范围或业务子树错误：%+v", info)
			}
			sv.Measure(layout.Loose(geometry.Size{Width: 25, Height: 31}))
			if sv.ScrollGeometry() != tc.want || sv.Snapshot().MaxScrollX != tc.maximum.X || sv.Snapshot().MaxScrollY != tc.maximum.Y {
				t.Fatal("独立测量覆盖了排列后的几何或范围")
			}
		})
	}
}

func TestScrollViewBarOptionsNormalizeAndClamp(t *testing.T) {
	sv := NewScrollView()
	sv.SetHScrollBarOptions(ScrollBarOptions{Policy: 255, Thickness: float32(math.NaN()), ThumbThickness: float32(math.Inf(1)), Spacing: -3})
	want := ScrollBarOptions{Thickness: 8, ThumbThickness: 8}
	if sv.HScrollBarOptions() != want || sv.VScrollBarOptions() != want {
		t.Fatalf("默认或非法输入未归一化：%+v / %+v", sv.HScrollBarOptions(), sv.VScrollBarOptions())
	}
	options := ScrollBarOptions{Policy: ScrollBarAlways, Thickness: 20, ThumbThickness: 30, Spacing: 40}
	sv.SetHScrollBarOptions(options)
	sv.SetVScrollBarOptions(options)
	for _, size := range []geometry.Size{{}, {Width: 5, Height: 4}, {Width: 50, Height: 60}} {
		sv.Arrange(geometry.Rectangle{Size: size})
		g := sv.ScrollGeometry()
		for _, r := range []geometry.Rectangle{g.Viewport, g.HScrollBar, g.VScrollBar, g.Corner} {
			if r.Width < 0 || r.Height < 0 || r.X < 0 || r.Y < 0 || r.X+r.Width > size.Width || r.Y+r.Height > size.Height {
				t.Fatalf("极小布局越界：size=%v geometry=%+v", size, g)
			}
		}
	}
	if sv.HScrollBarOptions() != options || sv.VScrollBarOptions() != options {
		t.Fatal("父布局钳制改写了期望配置")
	}
	sv.Arrange(geometry.Rect(0, 0, 5, 4))
	if sv.ScrollGeometry().Corner != geometry.Rect(0, 0, 5, 4) {
		t.Fatal("极小分配没有先保留条厚，再钳制间距")
	}
}

func TestScrollViewHiddenBarsKeepInputAndReveal(t *testing.T) {
	sv := NewScrollView()
	content := &mockWidget{size: geometry.Size{Width: 500, Height: 500}}
	field := newTestWidget()
	field.SetFocusable(true)
	content.WidgetBase.AddChild(content, field)
	field.Arrange(geometry.Rect(480, 480, 8, 8))
	sv.SetChild(content)
	sv.SetHScrollBarOptions(ScrollBarOptions{Policy: ScrollBarHidden})
	sv.SetVScrollBarOptions(ScrollBarOptions{Policy: ScrollBarHidden})
	win := testWindowWithSurface(&window{})
	win.SetWidget(sv)
	defer win.Destroy()
	sv.Arrange(geometry.Rect(0, 0, 100, 100))
	if err := win.DispatchEvent(events.WheelEvent{Position: geometry.Point{X: 10, Y: 10}, DeltaX: 20, DeltaY: 30, Mode: events.WheelDeltaPixel}); err != nil {
		t.Fatal(err)
	}
	if sv.ScrollX() != 20 || sv.ScrollY() != 30 {
		t.Fatal("隐藏条禁止了滚轮")
	}
	if err := win.DispatchEvent(shortcutPress(events.KeyTab, 0)); err != nil {
		t.Fatal(err)
	}
	if win.FocusedWidget() != field || sv.ScrollX() != 388 || sv.ScrollY() != 388 {
		t.Fatal("隐藏条阻止了键盘焦点的可见请求")
	}
	sv.ScrollIntoView(geometry.Rect(499, 499, 1, 1))
	if sv.ScrollX() != 400 || sv.ScrollY() != 400 || sv.hbar.Visible() || sv.vbar.Visible() {
		t.Fatal("隐藏条影响了末端 Reveal 或重新占位")
	}
}

func TestScrollViewOptionsCancelActiveBarDrag(t *testing.T) {
	sv := NewScrollView()
	// 显式留出两侧各2 DIP，继续验证滑块外的轨道带可以抓取并取消。
	sv.SetVScrollBarOptions(ScrollBarOptions{Thickness: 12})
	sv.SetChild(&mockWidget{size: geometry.Size{Width: 500, Height: 500}})
	win := testWindowWithSurface(&window{})
	win.SetWidget(sv)
	defer win.Destroy()
	sv.Arrange(geometry.Rect(0, 0, 100, 100))
	editorPointer(t, win, events.PointerDown, geometry.Point{X: 89, Y: 5}, 0)
	if !sv.vbar.State().Has(style.Pressed) {
		t.Fatal("未在滑块留白带开始拖动")
	}
	sv.SetVScrollBarOptions(ScrollBarOptions{Thickness: 20, Spacing: 4})
	if sv.vbar.State().Has(style.Pressed) {
		t.Fatal("配置变化保留了旧拖动")
	}
	before := sv.ScrollY()
	sv.Arrange(sv.Rect())
	editorPointer(t, win, events.PointerMove, geometry.Point{X: 89, Y: 60}, 0)
	editorPointer(t, win, events.PointerUp, geometry.Point{X: 89, Y: 60}, 0)
	if sv.ScrollY() != before {
		t.Fatal("已取消手势继续修改滚动位置")
	}
}

// Always 预留的条宽必须参与自然高度测量，不能先按整宽换行再挤出新溢出。
func TestScrollViewAlwaysBarMeasuresReducedCrossAxis(t *testing.T) {
	sv := NewScrollView()
	sv.SetDirection(ScrollVertical)
	sv.SetVScrollBarOptions(ScrollBarOptions{Policy: ScrollBarAlways, Thickness: 16, Spacing: 4})
	sv.SetChild(&responsiveScrollWidget{length: 400})
	if got := sv.Measure(layout.Loose(geometry.Size{Width: 100, Height: 300})).Size; got != (geometry.Size{Width: 100, Height: 100}) {
		t.Fatalf("预留20 DIP后应按80 DIP宽度测量五行，得到%v", got)
	}
	content := &responsiveScrollWidget{length: 400}
	sv.SetChild(content)
	sv.SetVScrollBarOptions(ScrollBarOptions{Policy: ScrollBarAlways, Thickness: 96, Spacing: 128})
	if got := sv.Measure(layout.Unbounded()).Size; got != (geometry.Size{Width: 624, Height: 20}) {
		t.Fatalf("无界自然宽度应包含224 DIP槽位，得到%v", got)
	}
	for _, c := range content.constraints {
		if c.Max.Width != layout.Inf {
			t.Fatalf("槽位扣减破坏了无界约束：%+v", c)
		}
	}
}

// responsiveScrollWidget models wrapping without fonts or a desktop. Its
// natural text extent is 400 DIP; each wrapped line/column takes 20 DIP.
type responsiveScrollWidget struct {
	WidgetBase
	horizontal  bool
	length      float32
	constraints []layout.Constraint
}

func (w *responsiveScrollWidget) Measure(c layout.Constraint) layout.Measurement {
	w.constraints = append(w.constraints, c)
	cross := c.Max.Width
	if w.horizontal {
		cross = c.Max.Height
	}
	cross = min(w.length, cross)
	lines := float32(1)
	if cross > 0 {
		lines = float32(math.Ceil(float64(w.length / cross)))
	}
	size := geometry.Size{Width: cross, Height: 20 * lines}
	if w.horizontal {
		size.Width, size.Height = size.Height, size.Width
	}
	return layout.Measured(size).Constrain(c)
}

func TestScrollViewSingleAxisMeasuresActualViewport(t *testing.T) {
	for _, direction := range []ScrollDirection{ScrollVertical, ScrollHorizontal} {
		horizontal := direction == ScrollHorizontal
		sv := NewScrollView()
		sv.SetDirection(direction)
		content := &responsiveScrollWidget{length: 400, horizontal: horizontal}
		sv.SetChild(content)
		size := geometry.Size{Width: 100, Height: 60}
		if horizontal {
			size.Width, size.Height = size.Height, size.Width
		}
		sv.Measure(layout.Tight(size))
		sv.Arrange(geometry.Rectangle{Size: size})
		wantContent, wantArea := geometry.Size{Width: 92, Height: 100}, geometry.Size{Width: 92, Height: 60}
		if horizontal {
			wantContent.Width, wantContent.Height = wantContent.Height, wantContent.Width
			wantArea.Width, wantArea.Height = wantArea.Height, wantArea.Width
		}
		if content.Rect().Size != wantContent || sv.viewport.Rect().Size != wantArea || sv.vbar.Visible() == horizontal || sv.hbar.Visible() != horizontal {
			t.Fatalf("single-axis layout: direction=%v content=%v viewport=%v", direction, content.Rect(), sv.viewport.Rect())
		}
		last := content.constraints[len(content.constraints)-1]
		if !horizontal && (last.Min.Width != 92 || last.Max.Width != 92 || last.Max.Height != layout.Inf) ||
			horizontal && (last.Min.Height != 92 || last.Max.Height != 92 || last.Max.Width != layout.Inf) {
			t.Fatalf("not measured at bar-reduced cross extent: %+v", last)
		}
		// Reveal requests, including an overflowing cross-axis rectangle, may
		// only move the configured axis. The visible end is 100 - 60 = 40.
		sv.ScrollIntoView(geometry.Rect(200, 200, 1, 1))
		if !horizontal && (sv.ScrollX() != 0 || sv.ScrollY() != 40) || horizontal && (sv.ScrollX() != 40 || sv.ScrollY() != 0) {
			t.Fatalf("reveal ignored direction: %g/%g", sv.ScrollX(), sv.ScrollY())
		}
		content.length = 60
		sv.Arrange(geometry.Rectangle{Size: size})
		if sv.hbar.Visible() || sv.vbar.Visible() || sv.ScrollX() != 0 || sv.ScrollY() != 0 {
			t.Fatal("content shrink retained scrollbar or offset")
		}
		// Short content fills only the cross axis, not the scrolling axis.
		want := geometry.Size{Width: 100, Height: 20}
		if horizontal {
			want.Width, want.Height = want.Height, want.Width
		}
		if content.Rect().Size != want {
			t.Fatalf("short content stretched along the scrolling axis: %v", content.Rect())
		}
	}
}

// Measure 可以被父布局或状态样式比较单独调用，不能覆盖已排列的范围。
// 400 DIP 内容每行/列 20 DIP；长内容的交叉轴须始终扣除 8 DIP 滚动条。
func TestScrollViewRemeasurePreservesArrangedGeometry(t *testing.T) {
	for _, direction := range []ScrollDirection{ScrollVertical, ScrollHorizontal} {
		t.Run(map[ScrollDirection]string{ScrollVertical: "vertical", ScrollHorizontal: "horizontal"}[direction], func(t *testing.T) {
			horizontal := direction == ScrollHorizontal
			sv := NewScrollView()
			sv.SetDirection(direction)
			content := &responsiveScrollWidget{length: 400, horizontal: horizontal}
			sv.SetChild(content)
			for _, cross := range []float32{100, 120, 80, 400, 100} {
				size := geometry.Size{Width: cross, Height: 60}
				area := size
				if cross < 400 {
					area.Width -= 8
				}
				extent := geometry.Size{Width: area.Width, Height: 20 * float32(math.Ceil(400/float64(area.Width)))}
				if horizontal {
					size.Width, size.Height = size.Height, size.Width
					area.Width, area.Height = area.Height, area.Width
					extent.Width, extent.Height = extent.Height, extent.Width
				}
				sv.Measure(layout.Tight(size))
				sv.Arrange(geometry.Rectangle{Size: size})
				if content.Rect().Size != extent || sv.viewport.Rect().Size != area {
					t.Fatalf("初次排列尺寸错误：content=%v viewport=%v，预期 %v / %v", content.Rect(), sv.viewport.Rect(), extent, area)
				}
				before := sv.Snapshot()
				viewport, vbar, hbar := sv.viewport.Rect(), sv.vbar.Rect(), sv.hbar.Rect()
				for _, c := range []layout.Constraint{layout.Tight(size), layout.Loose(geometry.Size{Width: 800, Height: 500}), layout.Unbounded()} {
					sv.Measure(c)
					if sv.contentWidth != extent.Width || sv.contentHeight != extent.Height {
						t.Fatalf("重测覆盖了已排列内容尺寸：%g×%g，预期 %v，约束 %v", sv.contentWidth, sv.contentHeight, extent, c)
					}
					if after := sv.Snapshot(); after.MaxScrollX != before.MaxScrollX || after.MaxScrollY != before.MaxScrollY {
						t.Fatalf("重测改变了快照滚动范围：%g/%g -> %g/%g", before.MaxScrollX, before.MaxScrollY, after.MaxScrollX, after.MaxScrollY)
					}
					if horizontal {
						sv.SetScrollX(sv.ScrollX() + 10)
					} else {
						sv.SetScrollY(sv.ScrollY() + 10)
					}
					if content.Rect().Size != extent || sv.viewport.Rect() != viewport || sv.vbar.Rect() != vbar || sv.hbar.Rect() != hbar {
						t.Fatalf("重测后滚动改变了尺寸或滚动条占位：content=%v viewport=%v", content.Rect(), sv.viewport.Rect())
					}
				}
			}
		})
	}
}

func TestScrollViewDirectionOffsetsWheelAndSnapshot(t *testing.T) {
	sv := NewScrollView()
	sv.SetChild(&mockWidget{size: geometry.Size{Width: 500, Height: 500}})
	sv.Arrange(geometry.Rect(0, 0, 100, 100))
	sv.SetScrollX(20)
	sv.SetScrollY(30)
	sv.SetDirection(ScrollVertical)
	if sv.ScrollX() != 0 || sv.ScrollY() != 30 {
		t.Fatal("direction change did not reset just the disabled offset")
	}
	sv.Arrange(sv.Rect())
	sv.SetScrollX(50)
	win := testWindowWithSurface(&window{})
	win.SetWidget(sv)
	defer win.Destroy()
	_ = win.DispatchEvent(events.WheelEvent{Position: geometry.Point{X: 20, Y: 20}, DeltaX: 40, DeltaY: 10, Mode: events.WheelDeltaPixel})
	info := sv.Snapshot()
	if sv.ScrollX() != 0 || sv.ScrollY() != 40 || info.MaxScrollX != 0 || sv.hbar.Visible() {
		t.Fatal("vertical mode accepted horizontal input or exposed a disabled range")
	}
	sv.SetDirection(ScrollHorizontal)
	sv.Arrange(sv.Rect())
	sv.SetScrollY(50)
	_ = win.DispatchEvent(events.WheelEvent{Position: geometry.Point{X: 20, Y: 20}, DeltaY: 10, Modifiers: events.ModifierShift, Mode: events.WheelDeltaPixel})
	info = sv.Snapshot()
	if sv.ScrollX() != 10 || sv.ScrollY() != 0 || info.MaxScrollY != 0 || sv.vbar.Visible() {
		t.Fatal("horizontal mode did not honor Shift-wheel or exposed a disabled range")
	}
	sv.SetDirection(255)
	if sv.Direction() != ScrollBoth {
		t.Fatal("invalid direction was not normalized")
	}
}

func TestScrollViewUnboundedCrossAxisAndOwnMaximum(t *testing.T) {
	sv := NewScrollView()
	sv.SetDirection(ScrollVertical)
	content := &responsiveScrollWidget{length: 400}
	sv.SetChild(content)
	if got := sv.Measure(layout.Unbounded()).Size; got != (geometry.Size{Width: 400, Height: 20}) {
		t.Fatalf("unbounded viewport invented a cross size: %v", got)
	}
	sv.SetMaxSize(geometry.Size{Width: 100})
	// 最大外宽100，Auto纵条占8，最后一次换行测量必须使用92。
	content.constraints = nil
	if got := sv.Measure(layout.Loose(geometry.Size{Width: 200, Height: 60})).Width; got != 100 || content.constraints[0].Max.Width != 100 || content.constraints[len(content.constraints)-1].Max.Width != 92 {
		t.Fatalf("own maximum did not reach content measurement: width=%g", got)
	}
}

func TestScrollViewTinyAllocationAndClear(t *testing.T) {
	sv := NewScrollView()
	sv.SetChild(&mockWidget{size: geometry.Size{Width: 200, Height: 200}})
	for _, size := range []geometry.Size{{}, {Width: 3, Height: 4}, {Width: 8, Height: 8}} {
		sv.Arrange(geometry.Rectangle{Size: size})
		for _, r := range []geometry.Rectangle{sv.viewport.Rect(), sv.vbar.Rect(), sv.hbar.Rect()} {
			if r.X < 0 || r.Y < 0 || r.Width < 0 || r.Height < 0 || r.X+r.Width > size.Width || r.Y+r.Height > size.Height {
				t.Fatalf("structural allocation outside tiny viewport %v: %v", size, r)
			}
		}
	}
	sv.SetChild(nil)
	sv.Arrange(sv.Rect())
	info := sv.Snapshot()
	if info.MaxScrollX != 0 || info.MaxScrollY != 0 || sv.hbar.Visible() || sv.vbar.Visible() {
		t.Fatal("cleared child retained its extent")
	}
}

type extentScrollContent struct {
	WidgetBase
	size      geometry.Size
	discover  geometry.Size
	viewports []geometry.Size
}

func (w *extentScrollContent) ContentSize() geometry.Size { return w.size }

func (w *extentScrollContent) LayoutVisible(viewport geometry.Size, offset geometry.Point) {
	w.viewports = append(w.viewports, viewport)
	if w.discover != (geometry.Size{}) {
		w.size, w.discover = w.discover, geometry.Size{}
	}
}

func TestScrollViewVirtualExtentAndDisabledAxis(t *testing.T) {
	sv := NewScrollView()
	content := &extentScrollContent{discover: geometry.Size{Width: 96, Height: 200}}
	sv.SetChild(content)
	sv.Measure(layout.Tight(geometry.Size{Width: 100, Height: 100}))
	sv.Arrange(geometry.Rect(0, 0, 100, 100))
	if len(content.viewports) != 2 || content.Rect() != geometry.Rect(0, 0, 92, 92) || content.viewports[1] != (geometry.Size{Width: 92, Height: 92}) {
		t.Fatalf("lazy extent did not update the actual viewport: %v / %v", content.viewports, content.Rect())
	}
	sv.SetDirection(ScrollVertical)
	sv.Arrange(sv.Rect())
	if content.ContentSize().Width != 96 || sv.Snapshot().MaxScrollX != 0 || sv.hbar.Visible() || content.Rect() != geometry.Rect(0, 0, 92, 100) {
		t.Fatal("host changed virtual content policy instead of disabling its scroll axis")
	}
}

// 虚拟内容也只在排列时发布新范围；Measure 不应让滚动范围先于视口变化。
func TestScrollViewVirtualRemeasurePreservesArrangedExtent(t *testing.T) {
	sv := NewScrollView()
	content := &extentScrollContent{size: geometry.Size{Width: 200, Height: 300}}
	sv.SetChild(content)
	sv.Measure(layout.Tight(geometry.Size{Width: 100, Height: 100}))
	sv.Arrange(geometry.Rect(0, 0, 100, 100))
	content.size = geometry.Size{Width: 400, Height: 500}
	sv.Measure(layout.Tight(geometry.Size{Width: 100, Height: 100}))
	if info := sv.Snapshot(); info.MaxScrollX != 108 || info.MaxScrollY != 208 {
		t.Fatalf("重测提前发布了虚拟内容范围：%g/%g", info.MaxScrollX, info.MaxScrollY)
	}
	sv.Arrange(sv.Rect())
	if info := sv.Snapshot(); info.MaxScrollX != 308 || info.MaxScrollY != 408 || content.Rect().Size != (geometry.Size{Width: 92, Height: 92}) {
		t.Fatalf("重新排列没有发布新范围：%g/%g，content=%v", info.MaxScrollX, info.MaxScrollY, content.Rect())
	}
	content.size = geometry.Size{Width: 80, Height: 80}
	sv.Arrange(sv.Rect())
	if info := sv.Snapshot(); info.MaxScrollX != 0 || info.MaxScrollY != 0 || sv.vbar.Visible() || sv.hbar.Visible() || content.Rect().Size != (geometry.Size{Width: 100, Height: 100}) {
		t.Fatalf("虚拟内容缩短后仍保留旧范围或滚动条：%g/%g，content=%v", info.MaxScrollX, info.MaxScrollY, content.Rect())
	}
}

// --- ScrollView (viewport mode) ---

// mockWidget is a plain widget with a fixed measure size (no typography
// dependency), standing in for a real content tree in tests.
type mockWidget struct {
	WidgetBase
	size geometry.Size
}

type mockPaintWidget struct {
	mockWidget
}

func (m *mockPaintWidget) Paint(p Painter) {
	p.FillRect(geometry.Rect(0, 0, m.Rect().Width, m.Rect().Height), graphics.RGB(1, 2, 3))
}

func (m *mockWidget) Measure(c layout.Constraint) layout.Measurement {
	return layout.Measured(m.size)
}

func (m *mockWidget) Arrange(rect geometry.Rectangle) {
	m.WidgetBase.Arrange(rect)
}

func TestScrollViewClamp(t *testing.T) {
	sv := NewScrollView()
	sv.SetChild(&mockWidget{size: geometry.Size{Width: 100, Height: 500}})

	// Measure gives the viewport the parent constraint; content is unbounded.
	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 100, Height: 200}})
	sv.Arrange(geometry.Rect(0, 0, 100, 200))
	if sv.contentHeight != 500 {
		t.Fatalf("content height should be 500, got %v", sv.contentHeight)
	}

	sv.SetScrollY(1000)
	// The vertical bar reduces width below 100, requiring a horizontal bar too.
	if got := sv.ScrollY(); got != 308 {
		t.Fatalf("SetScrollY(1000) should clamp to 308, got %v", got)
	}

	sv.SetScrollY(-50)
	if got := sv.ScrollY(); got != 0 {
		t.Fatalf("SetScrollY(-50) should clamp to 0, got %v", got)
	}
}

func TestScrollViewContentOffset(t *testing.T) {
	sv := NewScrollView()
	content := &mockWidget{size: geometry.Size{Width: 100, Height: 500}}
	sv.SetChild(content)

	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 100, Height: 100}})
	sv.Arrange(geometry.Rect(0, 0, 100, 100))

	sv.SetScrollY(40)
	// Content should be arranged at -scrollY.
	if got := content.Rect().Y; got != -40 {
		t.Fatalf("content Y should be -40 after scrolling, got %v", got)
	}
}

func TestScrollViewSetChildReplaces(t *testing.T) {
	sv := NewScrollView()
	first := &mockWidget{size: geometry.Size{Width: 100, Height: 500}}
	second := &mockWidget{size: geometry.Size{Width: 100, Height: 500}}

	// SetChild is the Bin content API; it replaces the content inside the
	// structural viewport. The viewport and scrollbars remain regardless.
	sv.SetChild(first)
	if sv.Child() != first {
		t.Fatal("SetChild should set the content")
	}
	if first.Parent() != sv.viewport || !sliceContains(sv.viewport.Children(), first) {
		t.Fatalf("content should be mounted under the viewport, parent=%v children=%v", first.Parent(), sv.viewport.Children())
	}

	sv.SetChild(second)
	if sv.Child() != second {
		t.Fatal("SetChild should replace the previous content")
	}
	if second.Parent() != sv.viewport || !sliceContains(sv.viewport.Children(), second) {
		t.Fatalf("new content should be mounted under the viewport, parent=%v children=%v", second.Parent(), sv.viewport.Children())
	}
	if first.Parent() != nil || sliceContains(sv.viewport.Children(), first) {
		t.Fatal("previous content should be removed after replace")
	}

	// Clear via SetChild(nil): content slot is nil, scrollbars remain.
	sv.SetChild(nil)
	if sv.Child() != nil {
		t.Fatal("SetChild(nil) should clear the content")
	}
	if len(sv.viewport.Children()) != 0 || second.Parent() != nil {
		t.Fatal("viewport should not retain content after clear")
	}
}

func sliceContains(ws []Widget, w Widget) bool {
	for _, c := range ws {
		if c == w {
			return true
		}
	}
	return false
}

func TestScrollViewChildrenContainStructuralViewport(t *testing.T) {
	sv := NewScrollView()
	content := &mockWidget{size: geometry.Size{Width: 100, Height: 500}}
	sv.SetChild(content)

	// The real tree includes a viewport between ScrollView and content so paint
	// clipping and hit testing use the same structural boundary.
	children := sv.Children()
	if !sliceContains(children, sv.viewport) || sliceContains(children, content) {
		t.Fatalf("ScrollView children should contain the viewport, not content directly: %v", children)
	}
	if content.Parent() != sv.viewport || !sliceContains(sv.viewport.Children(), content) {
		t.Fatalf("viewport should contain content, parent=%v children=%v", content.Parent(), sv.viewport.Children())
	}
}

func TestScrollViewViewportDrivesPaintClipAndHitTesting(t *testing.T) {
	sv := NewScrollView()
	// 本组继续检查角区几何与正文裁剪，显式选择有底色的corner Part。
	scope := NewStyleScope()
	scope.SetStyleSheet(style.Sheet(style.Name("scroll-view").Part("corner").BackgroundColor(color.Black)))
	scope.SetChild(sv)
	defer scope.destroy(scope)
	content := &mockPaintWidget{mockWidget: mockWidget{
		size: geometry.Size{Width: 100, Height: 500},
	}}
	sv.SetChild(content)
	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 100, Height: 100}})
	sv.Arrange(geometry.Rect(10, 20, 100, 100))
	sv.SetScrollY(40)

	wantViewport := geometry.Rect(0, 0, 100-scrollbarWidth, 100-scrollbarWidth)
	if sv.viewport.Rect() != wantViewport {
		t.Fatalf("unexpected viewport rect: got %v want %v", sv.viewport.Rect(), wantViewport)
	}

	backend := new(recordingPainterBackend)
	paintWidget(sv, newPainter(backend, geometry.Rect(0, 0, 200, 200), 1))
	if len(backend.fills) != 2 {
		t.Fatalf("期望角区与正文各一次填充，实际 %d 次", len(backend.fills))
	}
	if backend.fills[0].rect != geometry.Rect(92, 92, 8, 8) {
		t.Fatalf("角区未使用实际槽位：%v", backend.fills[0].rect)
	}
	fill := backend.fills[1]
	if fill.clip != geometry.Rect(10, 20, 100-scrollbarWidth, 100-scrollbarWidth) {
		t.Fatalf("content must be clipped by the viewport, got %v", fill.clip)
	}
	if fill.transform != geometry.Translate(10, -20) {
		t.Fatalf("unexpected scrolled content transform: %v", fill.transform)
	}

	// Scrollbars are later siblings than the viewport, so reverse-order hit
	// testing reaches the vertical bar before any content beneath it.
	if target := hitTest(sv, geometry.Point{X: 109, Y: 25}); target != sv.vbar {
		t.Fatalf("point over the scrollbar should hit vbar, got %T", target)
	}
}

func TestWheelEventControllerPublic(t *testing.T) {
	// WheelEventController is a public, reusable controller (like
	// MotionEventController): connect via signal, drive via HandleEvent.
	controller := NewWheelEventController()
	var got WheelInfo
	controller.ConnectScroll(func(_ EventContext, e WheelInfo) {
		got = e
	})
	ctx := &eventContext{event: events.WheelEvent{DeltaY: 5, Mode: events.WheelDeltaPixel}}
	controller.HandleEvent(ctx)
	if got.DeltaY != 5 {
		t.Fatalf("ConnectScroll should receive the wheel event, got %+v", got)
	}
	if ctx.PropagationStopped() {
		t.Fatal("a reusable wheel controller must leave consumption to its listener")
	}
}

// 通过窗口正常派发路径验证嵌套滚动：消费依据是请求轴的范围，不是本次偏移
// 是否改变。内层已到边界或小于整数 DIP 的输入也不能意外移动外层。
func TestScrollViewNestedWheelPropagation(t *testing.T) {
	for _, tc := range []struct {
		name      string
		direction ScrollDirection
		content   geometry.Size
		initial   geometry.Point
		delta     geometry.Point
		modifiers events.Modifiers
		wantInner geometry.Point
		wantOuter geometry.Point
		propagate bool
	}{
		{name: "vertical", content: geometry.Size{Width: 50, Height: 500}, delta: geometry.Point{Y: 12}, wantInner: geometry.Point{Y: 12}},
		{name: "bottom", content: geometry.Size{Width: 50, Height: 500}, initial: geometry.Point{Y: 400}, delta: geometry.Point{Y: 12}, wantInner: geometry.Point{Y: 400}},
		{name: "top", content: geometry.Size{Width: 50, Height: 500}, delta: geometry.Point{Y: -12}},
		{name: "fractional", content: geometry.Size{Width: 50, Height: 500}, delta: geometry.Point{Y: 0.2}},
		{name: "horizontal", content: geometry.Size{Width: 500, Height: 50}, delta: geometry.Point{X: 12}, wantInner: geometry.Point{X: 12}},
		{name: "right", content: geometry.Size{Width: 500, Height: 50}, initial: geometry.Point{X: 400}, delta: geometry.Point{X: 12}, wantInner: geometry.Point{X: 400}},
		{name: "left", content: geometry.Size{Width: 500, Height: 50}, delta: geometry.Point{X: -12}},
		{name: "shift", content: geometry.Size{Width: 500, Height: 50}, delta: geometry.Point{Y: 12}, modifiers: events.ModifierShift, wantInner: geometry.Point{X: 12}},
		{name: "both axes", content: geometry.Size{Width: 500, Height: 500}, delta: geometry.Point{X: 8, Y: 12}, wantInner: geometry.Point{X: 8, Y: 12}},
		{name: "one matching axis consumes the event", direction: ScrollVertical, content: geometry.Size{Width: 500, Height: 500}, delta: geometry.Point{X: 8, Y: 12}, wantInner: geometry.Point{Y: 12}},
		{name: "short", content: geometry.Size{Width: 50, Height: 50}, delta: geometry.Point{Y: 12}, wantOuter: geometry.Point{Y: 12}, propagate: true},
		{name: "empty", delta: geometry.Point{Y: 12}, wantOuter: geometry.Point{Y: 12}, propagate: true},
		{name: "no vertical range", content: geometry.Size{Width: 500, Height: 50}, delta: geometry.Point{Y: 12}, wantOuter: geometry.Point{Y: 12}, propagate: true},
		{name: "disabled vertical axis", direction: ScrollHorizontal, content: geometry.Size{Width: 500, Height: 500}, delta: geometry.Point{Y: 12}, wantOuter: geometry.Point{Y: 12}, propagate: true},
		{name: "disabled horizontal axis", direction: ScrollVertical, content: geometry.Size{Width: 500, Height: 500}, delta: geometry.Point{Y: 12}, modifiers: events.ModifierShift, wantOuter: geometry.Point{X: 12}, propagate: true},
		{name: "zero", content: geometry.Size{Width: 500, Height: 500}, propagate: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inner := NewScrollView()
			inner.SetDirection(tc.direction)
			if tc.content != (geometry.Size{}) {
				inner.SetChild(&mockWidget{size: tc.content})
			}
			content := &mockWidget{size: geometry.Size{Width: 600, Height: 600}}
			content.AddChild(content, inner)
			outer := NewScrollView()
			outer.SetChild(content)
			win := testWindowWithSurface(&window{})
			win.SetWidget(outer)
			defer win.Destroy()
			outer.Arrange(geometry.Rect(0, 0, 300, 300))
			outer.SetScrollX(20)
			outer.SetScrollY(20)
			inner.Arrange(geometry.Rect(40, 40, 100, 100))
			inner.SetScrollX(tc.initial.X)
			inner.SetScrollY(tc.initial.Y)
			outerReceived := false
			outer.wheel.ConnectScroll(func(EventContext, WheelInfo) {
				outerReceived = true
			})
			if err := win.DispatchEvent(events.WheelEvent{
				Position: geometry.Point{X: 50, Y: 50}, DeltaX: tc.delta.X, DeltaY: tc.delta.Y,
				Mode: events.WheelDeltaPixel, Modifiers: tc.modifiers,
			}); err != nil {
				t.Fatal(err)
			}
			if got := (geometry.Point{X: inner.ScrollX(), Y: inner.ScrollY()}); got != tc.wantInner {
				t.Fatalf("inner offset = %v, want %v", got, tc.wantInner)
			}
			wantOuter := geometry.Point{X: 20 + tc.wantOuter.X, Y: 20 + tc.wantOuter.Y}
			if got := (geometry.Point{X: outer.ScrollX(), Y: outer.ScrollY()}); got != wantOuter || outerReceived != tc.propagate {
				t.Fatalf("outer offset = %v, want %v; received = %v, want %v", got, wantOuter, outerReceived, tc.propagate)
			}
		})
	}
}

func TestScrollViewScrollable(t *testing.T) {
	sv := NewScrollView()
	sv.SetChild(&mockWidget{size: geometry.Size{Width: 100, Height: 50}})

	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 100, Height: 100}})
	sv.Arrange(geometry.Rect(0, 0, 100, 100))
	if sv.Scrollable() {
		t.Fatal("short content should not be scrollable")
	}

	sv.SetChild(&mockWidget{size: geometry.Size{Width: 100, Height: 500}})
	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 100, Height: 100}})
	sv.Arrange(geometry.Rect(0, 0, 100, 100))
	if !sv.Scrollable() {
		t.Fatal("tall content should be scrollable")
	}
}

func TestScrollViewWheel(t *testing.T) {
	sv := NewScrollView()
	sv.SetChild(&mockWidget{size: geometry.Size{Width: 100, Height: 500}})
	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 100, Height: 100}})
	sv.Arrange(geometry.Rect(0, 0, 100, 100))

	// Pixel mode: DeltaY directly translates to scroll.
	sv.wheel.HandleEvent(&eventContext{event: events.WheelEvent{DeltaY: 30, Mode: events.WheelDeltaPixel}})
	if got := sv.ScrollY(); got != 30 {
		t.Fatalf("pixel wheel should scroll 30, got %v", got)
	}

	// Line mode: DeltaY * lineHeight, rounded to whole pixels.
	before := sv.ScrollY()
	sv.wheel.HandleEvent(&eventContext{event: events.WheelEvent{DeltaY: 2, Mode: events.WheelDeltaLine}})
	want := mathx.Round(before + 2*sv.lineHeightPx())
	if got := sv.ScrollY(); got != want {
		t.Fatalf("line wheel should scroll to %v, got %v", want, got)
	}
	// Scroll offsets are always whole pixels (text bitmaps stay crisp).
	if sv.ScrollY() != mathx.Round(sv.ScrollY()) {
		t.Fatalf("scrollY should be a whole pixel, got %v", sv.ScrollY())
	}

	// Clamp at the end.
	sv.wheel.HandleEvent(&eventContext{event: events.WheelEvent{DeltaY: 10000, Mode: events.WheelDeltaPixel}})
	if got := sv.ScrollY(); got != 408 {
		t.Fatalf("wheel should clamp to 408 with both bars, got %v", got)
	}
}

// --- ScrollView (Scrollable mode) ---

// mockScrollContent is a minimal Scrollable for tests.
type mockScrollContent struct {
	WidgetBase
	height      float32
	layoutCalls []geometry.Point
}

// 回调只用于在正常布局边界重入，不直接修改 ScrollView 私有状态。
type intentScrollContent struct {
	WidgetBase
	onLayout func()
	onSize   func()
}

func (c *intentScrollContent) ContentSize() geometry.Size {
	if fn := c.onSize; fn != nil {
		c.onSize = nil
		fn()
	}
	return geometry.Size{Width: 1000, Height: 1000}
}
func (c *intentScrollContent) LayoutVisible(geometry.Size, geometry.Point) {
	if fn := c.onLayout; fn != nil {
		c.onLayout = nil
		fn()
	}
}

func TestScrollViewLatestPositionIntent(t *testing.T) {
	for _, axis := range []string{"X", "Y"} {
		for _, value := range []float32{0, -10, 20, 9999} {
			t.Run(fmt.Sprintf("%s/%g", axis, value), func(t *testing.T) {
				sv := NewScrollView()
				content := &intentScrollContent{}
				sv.SetChild(content)
				content.onLayout = func() {
					sv.ScrollIntoView(geometry.Rect(700, 700, 1, 1))
					if axis == "X" {
						sv.SetScrollX(value)
					} else {
						sv.SetScrollY(value)
					}
				}
				sv.Arrange(geometry.Rect(0, 0, 100, 100))
				want := min(float32(908), max(float32(0), value))
				x, y := want, float32(0)
				if axis == "Y" {
					x, y = y, x
				}
				if sv.ScrollX() != x || sv.ScrollY() != y {
					t.Fatalf("旧 Reveal 覆盖新偏移：(%g,%g)，预期 (%g,%g)", sv.ScrollX(), sv.ScrollY(), x, y)
				}
			})
		}
	}
}

func TestScrollViewRevealOrdering(t *testing.T) {
	for _, tc := range []struct {
		name   string
		action func(*ScrollView)
		want   float32
	}{
		{"新Reveal", func(s *ScrollView) { s.SetScrollY(0); s.ScrollIntoView(geometry.Rect(0, 400, 1, 1)) }, 309},
		{"NaN", func(s *ScrollView) { s.SetScrollY(float32(math.NaN())) }, 609},
		{"Inf", func(s *ScrollView) { s.SetScrollY(float32(math.Inf(1))) }, 609},
		{"禁用轴", func(s *ScrollView) { s.SetDirection(ScrollVertical); s.SetScrollX(0) }, 609},
		{"布局钳制", func(s *ScrollView) { s.clampScroll() }, 609},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sv := NewScrollView()
			content := &intentScrollContent{}
			sv.SetChild(content)
			content.onLayout = func() {
				sv.ScrollIntoView(geometry.Rect(0, 700, 1, 1))
				tc.action(sv)
			}
			sv.Arrange(geometry.Rect(0, 0, 100, 100))
			want := tc.want
			if tc.name == "禁用轴" {
				want = 601
			}
			if sv.ScrollY() != want {
				t.Fatalf("偏移=%g，预期 %g", sv.ScrollY(), want)
			}
		})
	}
}

func TestScrollViewRevealReentrantContentSize(t *testing.T) {
	for _, action := range []string{"position", "reveal", "replace", "destroy"} {
		t.Run(action, func(t *testing.T) {
			sv := NewScrollView()
			content := &intentScrollContent{}
			sv.SetChild(content)
			sv.Arrange(geometry.Rect(0, 0, 100, 100))
			content.onSize = func() {
				switch action {
				case "position":
					sv.SetScrollY(0)
				case "reveal":
					sv.ScrollIntoView(geometry.Rect(0, 400, 1, 1))
				case "replace":
					sv.SetChild(nil)
				case "destroy":
					sv.base().destroy(sv)
				}
			}
			sv.ScrollIntoView(geometry.Rect(0, 700, 1, 1))
			if action == "reveal" {
				// 替代请求留到下次布局，不递归用户回调。
				sv.Arrange(sv.Rect())
				if sv.ScrollY() != 309 {
					t.Fatalf("新 Reveal 丢失：%g", sv.ScrollY())
				}
			} else if sv.ScrollY() != 0 {
				t.Fatalf("过期 Reveal 被应用：%g", sv.ScrollY())
			}
		})
	}
}

func (m *mockScrollContent) Measure(c layout.Constraint) layout.Measurement {
	return layout.Measured(c.Clamp(geometry.Size{Width: 100, Height: m.height}))
}

func (m *mockScrollContent) ContentSize() geometry.Size {
	return geometry.Size{Width: 100, Height: m.height}
}

func (m *mockScrollContent) LayoutVisible(viewport geometry.Size, offset geometry.Point) {
	m.layoutCalls = append(m.layoutCalls, offset)
}

func TestScrollViewScrollableContentMode(t *testing.T) {
	sv := NewScrollView()
	content := &mockScrollContent{height: 500}
	sv.SetChild(content)

	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 100, Height: 100}})
	sv.Arrange(geometry.Rect(0, 0, 100, 100))

	// The vertical bar reduces width below the content's fixed 100, so both
	// bars occupy space. Scrollable still controls its own width/wrapping policy.
	areaW := float32(100 - scrollbarWidth)

	if len(content.layoutCalls) != 1 {
		t.Fatalf("LayoutVisible should be called once on Arrange, got %d", len(content.layoutCalls))
	}
	if content.layoutCalls[0].Y != 0 {
		t.Fatalf("initial offset should be 0, got %v", content.layoutCalls[0])
	}
	// The content's own rect is the structural viewport area; its descendants
	// are laid out virtually for the requested offset.
	if got := content.Rect(); got != (geometry.Rect(0, 0, areaW, areaW)) {
		t.Fatalf("content rect should be the content area, got %v", got)
	}

	sv.SetScrollY(100)
	if len(content.layoutCalls) != 2 {
		t.Fatalf("LayoutVisible should be called again on SetScrollY, got %d calls", len(content.layoutCalls))
	}
	if content.layoutCalls[1].Y != 100 {
		t.Fatalf("offset should be 100, got %v", content.layoutCalls[1])
	}

	// 钳制：内容高500、视口高92，最大偏移408。
	sv.SetScrollY(999)
	if got := content.layoutCalls[len(content.layoutCalls)-1].Y; got != 408 {
		t.Fatalf("offset should clamp to 408, got %v", got)
	}
}

func TestScrollViewSnapshot(t *testing.T) {
	sv := NewScrollView()
	sv.SetChild(newTestWidget())
	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 100, Height: 100}})
	sv.Arrange(geometry.Rect(0, 0, 100, 100))

	info := sv.Snapshot()
	if info.Role != RoleScrollView {
		t.Fatalf("role should be scrollview, got %q", info.Role)
	}
	if info.ScrollY != 0 {
		t.Fatalf("scrollY should be 0, got %v", info.ScrollY)
	}
	if len(info.Children) != 1 {
		t.Fatalf("viewport content should appear in the snapshot, got %d children", len(info.Children))
	}

	// Scrollable content: MaxScrollY reflects the range.
	content := &mockWidget{size: geometry.Size{Width: 100, Height: 400}}
	sv.SetChild(content)
	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 100, Height: 100}})
	sv.Arrange(geometry.Rect(0, 0, 100, 100))
	sv.SetScrollY(50)
	info = sv.Snapshot()
	if info.ScrollY != 50 {
		t.Fatalf("scrollY should be 50, got %v", info.ScrollY)
	}
	if info.MaxScrollY != 308 {
		t.Fatalf("maxScrollY should be 308 with both bars, got %v", info.MaxScrollY)
	}
}

func TestScrollViewElasticMeasure(t *testing.T) {
	sv := NewScrollView()
	sv.SetChild(&mockScrollContent{height: 500})

	// A finite parent height is the viewport height; content height remains
	// cached separately for scrolling.
	if got := sv.Measure(layout.Constraint{Max: geometry.Size{Width: 100, Height: 100}}); got.Size != (geometry.Size{Width: 100, Height: 100}) {
		t.Fatalf("bounded measure should use the viewport height, got %v", got)
	}
	if sv.MainWeight() != 1 {
		t.Fatalf("ScrollView should default to MainWeight(1), got %v", sv.MainWeight())
	}
	// An unbounded height now follows content through the same Measure path.
	if got := sv.Measure(layout.Constraint{Max: geometry.Size{Width: 100, Height: layout.Inf}}); got.Size != (geometry.Size{Width: 100, Height: 500}) {
		t.Fatalf("unbounded measure should follow content height, got %v", got)
	}
	// Tight constraint (window root): c.Min == c.Max -> fills.
	if got := sv.Measure(layout.Tight(geometry.Size{Width: 800, Height: 600})); got.Size != (geometry.Size{Width: 800, Height: 600}) {
		t.Fatalf("elastic measure under Tight should fill, got %v", got)
	}
}

func TestScrollViewMeasureCombinesChildSizeWithParentConstraint(t *testing.T) {
	sv := NewScrollView()
	sv.SetChild(&mockScrollContent{height: 500})

	got := sv.Measure(layout.Constraint{
		Min: geometry.Size{Width: 120, Height: 20},
		Max: geometry.Size{Width: 200, Height: 100},
	})
	// Scrollable content fills the cross axis, so width is the parent's max
	// width, clamped to the constraint bounds.
	if got.Size != (geometry.Size{Width: 200, Height: 100}) {
		t.Fatalf("Measure should fill the cross axis and apply both parent bounds, got %v", got)
	}
}

func TestScrollViewFillsStretchedHBox(t *testing.T) {
	box := NewLinearBox(layout.DirectionHorizontal)
	box.SetCrossAlign(layout.CrossStretch)
	sv := NewScrollView()
	sv.SetChild(&mockScrollContent{height: 500})
	box.AddChild(sv)

	box.Arrange(geometry.Rect(0, 0, 800, 600))
	if got := sv.Rect(); got != geometry.Rect(0, 0, 800, 600) {
		t.Fatalf("single ScrollView should fill the viewport by default, got %v", got)
	}
}

func TestScrollViewFillsCrossAxisInVerticalLinearBox(t *testing.T) {
	box := NewLinearBox(layout.DirectionVertical)
	box.SetCrossAlign(layout.CrossStretch)
	sv := NewScrollView()
	sv.SetChild(&mockScrollContent{height: 500})
	box.AddChild(sv)

	box.Arrange(geometry.Rect(0, 0, 800, 600))
	if got := sv.Rect(); got != geometry.Rect(0, 0, 800, 600) {
		t.Fatalf("ScrollView should fill the cross axis in a VBox, got %v", got)
	}
}

func TestScrollViewCanHugContentWidthInVerticalLinearBox(t *testing.T) {
	box := NewLinearBox(layout.DirectionVertical)
	sv := NewScrollView()
	sv.SetChild(&mockScrollContent{height: 500})
	box.AddChild(sv)

	box.Arrange(geometry.Rect(0, 0, 800, 600))
	// Scrollable content fills the cross axis regardless of CrossAlign, so the
	// viewport spans the full 800 width.
	if got := sv.Rect(); got != geometry.Rect(0, 0, 800, 600) {
		t.Fatalf("ScrollView should fill the cross axis with Scrollable content, got %v", got)
	}
}

func TestScrollViewViewportModeUsesParentHeight(t *testing.T) {
	box := NewLinearBox(layout.DirectionVertical)
	sv := NewScrollView()
	content := &mockWidget{size: geometry.Size{Width: 100, Height: 1000}}
	sv.SetChild(content)
	box.AddChild(sv)

	box.Arrange(geometry.Rect(0, 0, 800, 600))
	if got := sv.Rect(); got != geometry.Rect(0, 0, 100, 600) {
		t.Fatalf("ScrollView should preserve content width and use the parent's height, got %v", got)
	}
	if got := content.Rect(); got != geometry.Rect(0, 0, 100, 1000) {
		t.Fatalf("content should retain its full scrollable height, got %v", got)
	}
}

func TestScrollViewSideBySideInLinearBox(t *testing.T) {
	// Regression: two ScrollViews in an HBox previously each reported c.Max
	// (the whole available width), overflowing the linear sum and pushing the
	// second list out of the container. ScrollView's MainWeight(1) splits the
	// main axis; the parent explicitly stretches the cross axis.
	box := NewLinearBox(layout.DirectionHorizontal)
	box.SetCrossAlign(layout.CrossStretch)
	left := NewScrollView()
	left.SetChild(&mockScrollContent{height: 500})
	right := NewScrollView()
	right.SetChild(&mockScrollContent{height: 500})
	box.AddChild(left)
	box.AddChild(right)

	box.Arrange(geometry.Rect(0, 0, 800, 600))
	if left.Rect() != geometry.Rect(0, 0, 400, 600) {
		t.Fatalf("left ScrollView should take half, got %v", left.Rect())
	}
	if right.Rect() != geometry.Rect(400, 0, 400, 600) {
		t.Fatalf("right ScrollView should take half, got %v", right.Rect())
	}
}

func TestScrollViewStackedInVerticalLinearBox(t *testing.T) {
	box := NewLinearBox(layout.DirectionVertical)
	top := NewScrollView()
	top.SetChild(&mockScrollContent{height: 500})
	bottom := NewScrollView()
	bottom.SetChild(&mockScrollContent{height: 500})
	box.AddChild(top)
	box.AddChild(bottom)

	box.Arrange(geometry.Rect(0, 0, 800, 600))
	// Scrollable content fills the cross axis, so the stacked viewports span
	// the full 800 width and split the height via main-axis weight.
	if top.Rect() != geometry.Rect(0, 0, 800, 300) {
		t.Fatalf("top ScrollView should fill the width and take half the height, got %v", top.Rect())
	}
	if bottom.Rect() != geometry.Rect(0, 300, 800, 300) {
		t.Fatalf("bottom ScrollView should fill the width and take half the height, got %v", bottom.Rect())
	}
}

// --- ScrollView slice 2: horizontal scroll + scrollbar interaction ---

func TestScrollViewScrollXClamp(t *testing.T) {
	sv := NewScrollView()
	sv.SetChild(&mockWidget{size: geometry.Size{Width: 500, Height: 100}})

	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 200, Height: 100}})
	sv.Arrange(geometry.Rect(0, 0, 200, 100))

	// The horizontal bar reduces height below 100, requiring a vertical bar.
	sv.SetScrollX(1000)
	if got := sv.ScrollX(); got != 308 {
		t.Fatalf("SetScrollX(1000) should clamp to 308, got %v", got)
	}
	sv.SetScrollX(-50)
	if got := sv.ScrollX(); got != 0 {
		t.Fatalf("SetScrollX(-50) should clamp to 0, got %v", got)
	}
}

func TestScrollViewContentOffsetX(t *testing.T) {
	sv := NewScrollView()
	content := &mockWidget{size: geometry.Size{Width: 500, Height: 100}}
	sv.SetChild(content)

	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 200, Height: 100}})
	sv.Arrange(geometry.Rect(0, 0, 200, 100))
	sv.SetScrollX(50)

	if got := content.Rect().X; got != -50 {
		t.Fatalf("content X should be -50 after horizontal scroll, got %v", got)
	}
}

func TestScrollViewHorizontalWheel(t *testing.T) {
	sv := NewScrollView()
	sv.SetChild(&mockWidget{size: geometry.Size{Width: 500, Height: 100}})

	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 200, Height: 100}})
	sv.Arrange(geometry.Rect(0, 0, 200, 100))

	// Native horizontal wheel (DeltaX).
	sv.wheel.HandleEvent(&eventContext{event: events.WheelEvent{DeltaX: 30, Mode: events.WheelDeltaPixel}})
	if got := sv.ScrollX(); got != 30 {
		t.Fatalf("horizontal wheel should scroll X by 30, got %v", got)
	}

	// Shift+vertical → horizontal.
	sv.wheel.HandleEvent(&eventContext{event: events.WheelEvent{DeltaY: 20, Mode: events.WheelDeltaPixel, Modifiers: events.ModifierShift}})
	if got := sv.ScrollX(); got != 50 {
		t.Fatalf("Shift+wheel should scroll X by 20 more, got %v", got)
	}
}

func TestScrollViewScrollBarVisibility(t *testing.T) {
	sv := NewScrollView()
	content := &mockWidget{size: geometry.Size{Width: 500, Height: 500}}
	sv.SetChild(content)

	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 200, Height: 200}})
	sv.Arrange(geometry.Rect(0, 0, 200, 200))

	// Both axes scrollable → both bars visible.
	if !sv.vbar.Visible() {
		t.Fatal("vertical bar should be visible when content is taller")
	}
	if !sv.hbar.Visible() {
		t.Fatal("horizontal bar should be visible when content is wider")
	}

	// Short content that fits → no bars.
	sv.SetChild(&mockWidget{size: geometry.Size{Width: 100, Height: 100}})
	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 200, Height: 200}})
	sv.Arrange(geometry.Rect(0, 0, 200, 200))
	if sv.vbar.Visible() {
		t.Fatal("vertical bar should be hidden when content fits")
	}
	if sv.hbar.Visible() {
		t.Fatal("horizontal bar should be hidden when content fits")
	}
}

func TestScrollViewScrollBarDragUpdatesScroll(t *testing.T) {
	sv := NewScrollView()
	sv.SetChild(&mockWidget{size: geometry.Size{Width: 100, Height: 500}})

	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 100, Height: 200}})
	sv.Arrange(geometry.Rect(0, 0, 100, 200))

	// maxY = 500 - 200 = 300. The vbar is arranged in the right column.
	// Press on the thumb and drag down; the scroll offset should follow.
	before := sv.ScrollY()
	bar := sv.vbar

	down := &eventContext{
		current: bar,
		event: events.PointerEvent{
			EventType: events.PointerDown,
			Button:    events.PointerButtonLeft,
			Position:  geometry.Point{X: bar.Rect().X, Y: bar.Rect().Y + 5},
		},
	}
	bar.drag.HandleEvent(down)

	move := &eventContext{
		current: bar,
		event: events.PointerEvent{
			EventType: events.PointerMove,
			Button:    events.PointerButtonLeft,
			Position:  geometry.Point{X: bar.Rect().X, Y: bar.Rect().Y + 80},
		},
	}
	bar.drag.HandleEvent(move)

	if sv.ScrollY() <= before {
		t.Fatalf("dragging the thumb down should increase scrollY, before=%v after=%v", before, sv.ScrollY())
	}

	up := &eventContext{
		current: bar,
		event: events.PointerEvent{
			EventType: events.PointerUp,
			Button:    events.PointerButtonLeft,
			Position:  geometry.Point{X: bar.Rect().X, Y: bar.Rect().Y + 80},
		},
	}
	bar.drag.HandleEvent(up)
	if bar.drag.Dragging() {
		t.Fatal("drag should end after pointer up")
	}
}

func TestScrollViewStyleNamePushDown(t *testing.T) {
	sv := NewScrollView()
	sv.SetChild(&mockWidget{size: geometry.Size{Width: 100, Height: 500}})
	sv.SetStyleName("my-scroll")

	if sv.vbar.StyleName() != "my-scroll" {
		t.Fatalf("vbar should inherit ScrollView StyleName, got %q", sv.vbar.StyleName())
	}
	if sv.hbar.StyleName() != "my-scroll" {
		t.Fatalf("hbar should inherit ScrollView StyleName, got %q", sv.hbar.StyleName())
	}
}

func TestScrollViewSnapshotScrollX(t *testing.T) {
	sv := NewScrollView()
	sv.SetChild(&mockWidget{size: geometry.Size{Width: 500, Height: 100}})

	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 200, Height: 100}})
	sv.Arrange(geometry.Rect(0, 0, 200, 100))
	sv.SetScrollX(50)

	info := sv.Snapshot()
	if info.ScrollX != 50 {
		t.Fatalf("scrollX should be 50, got %v", info.ScrollX)
	}
	if info.MaxScrollX != 308 {
		t.Fatalf("maxScrollX should be 308 with both bars, got %v", info.MaxScrollX)
	}
	// Snapshot children filter out scrollbars.
	for _, c := range info.Children {
		if c.Role == RoleScrollBar {
			t.Fatal("snapshot should not include scrollbar children")
		}
	}
}

func TestScrollViewSnapshotFiltersScrollBars(t *testing.T) {
	sv := NewScrollView()
	sv.SetChild(&mockWidget{size: geometry.Size{Width: 100, Height: 500}})
	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 100, Height: 100}})
	sv.Arrange(geometry.Rect(0, 0, 100, 100))

	info := sv.Snapshot()
	// Only the content child, not the scrollbars.
	if len(info.Children) != 1 {
		t.Fatalf("snapshot should have 1 child (content), got %d", len(info.Children))
	}
}

type replacingMeasuredContent struct {
	mockScrollContent
	replace func()
}

func (c *replacingMeasuredContent) Measure(cs layout.Constraint) layout.Measurement {
	c.replace()
	return c.mockScrollContent.Measure(cs)
}

func TestScrollViewStopsWhenMeasurementReplacesContent(t *testing.T) {
	scroll := NewScrollView()
	content := &replacingMeasuredContent{replace: func() { scroll.SetChild(nil) }}
	scroll.SetChild(content)
	if got := scroll.Measure(layout.Loose(geometry.Size{Width: 100, Height: 100})); got != (layout.Measurement{}) || scroll.Child() != nil {
		t.Fatalf("measurement retained replaced content: %+v", got)
	}
}

// 目标末端不能先按 float32 相加；最终滚动值向可见方向取可表示的边界。
func TestRevealOffsetPreservesLargeTargetEdges(t *testing.T) {
	for _, tc := range []struct{ start, size, viewport float32 }{
		{270000032, 71, 400}, {1000000000, 67, 981.65686}, {268435456, 1, 401}, {20.25, 1, 100}, {20, 200, 100},
	} {
		got := revealOffset(0, tc.viewport, tc.start, tc.size)
		if tc.size > tc.viewport {
			if got > tc.start || float64(tc.start)-float64(got) >= 1 {
				t.Fatal("超宽目标没有对齐首端", tc, got)
			}
		} else if float64(got)+float64(tc.viewport) < float64(tc.start)+float64(tc.size) || got > tc.start {
			t.Fatal("目标末端仍在视口之外", tc, got)
		}
	}
}

// 最大偏移也必须允许显示内容末端，两个轴均按高精度差值向外取整。
func TestScrollViewLargeRangeContainsContentEnd(t *testing.T) {
	sv := NewScrollView()
	content := &extentScrollContent{size: geometry.Size{Width: 270000064, Height: 1000000000}}
	sv.SetChild(content)
	sv.Arrange(geometry.Rect(0, 0, 409, 219))
	area := sv.contentAreaSize()
	sv.SetScrollX(math.MaxFloat32)
	sv.SetScrollY(math.MaxFloat32)
	if float64(sv.ScrollX())+float64(area.Width) < float64(content.size.Width) || float64(sv.ScrollY())+float64(area.Height) < float64(content.size.Height) {
		t.Fatal("范围钳制隐藏内容末端", sv.ScrollX(), sv.ScrollY(), area, content.size)
	}
}

// 非 Widget 客户端保留实际状态，也能在同步中触发外部回调。
type scrollClientProbe struct {
	maximum, page, value float32
	change               signal.Signal1[float32]
	onSet                func(string)
	values               int
	onConnect            func()
}

func (p *scrollClientProbe) SetMax(v float32) {
	p.maximum = v
	if p.onSet != nil {
		p.onSet("max")
	}
}
func (p *scrollClientProbe) SetPageSize(v float32) {
	p.page = v
	if p.onSet != nil {
		p.onSet("page")
	}
}
func (p *scrollClientProbe) SetValue(v float32) {
	p.value = v
	p.values++
	if p.onSet != nil {
		p.onSet("value")
	}
}
func (p *scrollClientProbe) ConnectChange(fn func(float32)) signal.Handle {
	if p.onConnect != nil {
		p.onConnect()
	}
	return p.change.Connect(fn)
}

func TestScrollViewExternalClients(t *testing.T) {
	sv := NewScrollView()
	sv.SetChild(&mockWidget{size: geometry.Size{Width: 500, Height: 1000}})
	sv.SetHScrollBarOptions(ScrollBarOptions{Policy: ScrollBarHidden})
	sv.SetVScrollBarOptions(ScrollBarOptions{Policy: ScrollBarHidden})
	sv.Arrange(geometry.Rect(0, 0, 100, 100))
	x, y, other := new(scrollClientProbe), new(scrollClientProbe), new(scrollClientProbe)
	hx := sv.BindScroll(layout.DirectionHorizontal, x)
	hy := sv.BindScroll(layout.DirectionVertical, y)
	ho := sv.BindScroll(layout.DirectionVertical, other)
	defer signal.Handles{hx, hy, ho}.Disconnect()
	if x.maximum != 400 || x.page != 100 || y.maximum != 900 || y.page != 100 {
		t.Fatal("外部状态没有使用内容视口", x, y)
	}
	y.value = 10.4 // 模拟先改变自身显示值的数值控件。
	y.change.Emit(10.4)
	if sv.ScrollY() != 10 || y.value != 10 || other.value != 10 || x.value != 0 {
		t.Fatal("请求没有回传实际值或串扰其他轴")
	}
	y.value = 10.2
	y.change.Emit(10.2)
	if y.value != 10 {
		t.Fatal("位置未改变时没有纠正客户端")
	}
	y.change.Emit(float32(math.NaN()))
	if y.value != 10 {
		t.Fatal("非有限请求污染实际状态")
	}
	x.change.Emit(9999)
	if x.value != 400 || sv.ScrollX() != 400 {
		t.Fatal("没有钳制到末端")
	}
	sv.ScrollIntoView(geometry.Rect(0, 999, 1, 1))
	if y.value != 900 || other.value != 900 {
		t.Fatal("Reveal 没有同步外部条")
	}
	sv.Arrange(geometry.Rect(0, 0, 200, 300))
	if y.page != 300 || y.maximum != 700 || y.value != 700 || x.page != 200 || x.maximum != 300 {
		t.Fatal("重布局没有更新范围、页面或末端位置", x, y)
	}
	sv.SetDirection(ScrollHorizontal)
	y.change.Emit(70)
	if y.value != 0 || y.maximum != 0 || sv.ScrollY() != 0 {
		t.Fatal("禁止轴仍接受外部请求")
	}
	sv.SetChild(nil)
	if x.maximum != 0 || x.value != 0 || y.maximum != 0 {
		t.Fatal("清除内容后保留旧状态")
	}
}

func TestScrollViewExternalBindingLifetime(t *testing.T) {
	sv := NewScrollView()
	sv.SetChild(&mockWidget{size: geometry.Size{Width: 100, Height: 1000}})
	win := testWindowWithSurface(&window{})
	win.SetWidget(sv)
	defer win.Destroy()
	sv.Arrange(geometry.Rect(0, 0, 100, 100))
	client := new(scrollClientProbe)
	h := sv.BindScroll(layout.DirectionVertical, client)
	h.Block()
	h.Block()
	sv.SetScrollY(50)
	client.change.Emit(80)
	if sv.ScrollY() != 50 || client.value != 0 {
		t.Fatal("Block没有暂停双向关联")
	}
	h.Unblock()
	if client.value != 0 {
		t.Fatal("嵌套Block提前恢复")
	}
	h.Unblock()
	if client.value != 50 {
		t.Fatal("恢复关联时没有补发实际状态")
	}
	h.Disconnect()
	h.Disconnect()
	client.change.Emit(90)
	sv.SetScrollY(60)
	if client.value != 50 || sv.ScrollY() != 60 || len(sv.clientBindings) != 0 {
		t.Fatal("解绑后仍有连接")
	}
	h = sv.BindScroll(layout.DirectionVertical, client)
	win.Destroy()
	if len(sv.clientBindings) != 0 {
		t.Fatal("宿主销毁后仍保留连接，不能依赖调用方再次解绑")
	}
	client.change.Emit(100)
	h.Unblock()
	h.Disconnect()
	if len(sv.clientBindings) != 0 {
		t.Fatal("宿主销毁未清除连接")
	}
}

func TestScrollViewExternalSyncReentrancy(t *testing.T) {
	sv := NewScrollView()
	sv.SetChild(&mockWidget{size: geometry.Size{Width: 100, Height: 1000}})
	sv.Arrange(geometry.Rect(0, 0, 100, 100))
	first, second := new(scrollClientProbe), new(scrollClientProbe)
	h := sv.BindScroll(layout.DirectionVertical, first)
	defer h.Disconnect()
	sv.BindScroll(layout.DirectionVertical, second)
	once := false
	first.onSet = func(stage string) {
		if stage == "max" && !once {
			once = true
			sv.SetScrollY(80)
		}
		// 即使错误的适配器在接收状态时回发请求，也不能产生回声。
		first.change.Emit(500)
	}
	sv.SetScrollY(40)
	if !once || sv.ScrollY() != 80 || first.value != 80 || second.value != 80 {
		t.Fatal("旧同步覆盖重入后的状态", sv.ScrollY(), first, second)
	}
	first.onSet = func(stage string) {
		if stage == "max" {
			h.Disconnect()
		}
	}
	before := first.values
	sv.SetScrollY(90)
	if first.values != before || second.value != 90 {
		t.Fatal("解绑回调后仍调用客户端")
	}
}

func TestScrollViewExternalSyncUnchangedReentrantLayout(t *testing.T) {
	sv := NewScrollView()
	sv.SetChild(&mockWidget{size: geometry.Size{Width: 100, Height: 1000}})
	sv.Arrange(geometry.Rect(0, 0, 100, 100))
	client := new(scrollClientProbe)
	h := sv.BindScroll(layout.DirectionVertical, client)
	defer h.Disconnect()
	client.onSet = func(stage string) {
		if stage == "max" {
			sv.Arrange(sv.Rect())
		}
	}
	sv.SetScrollY(40)
	if client.value != 40 || sv.ScrollY() != 40 {
		t.Fatal("同值重入布局阻止提交实际偏移")
	}
}

func TestScrollViewExternalCallbackDestroysHost(t *testing.T) {
	for _, phase := range []string{"connect", "page", "deferred-page"} {
		t.Run(phase, func(t *testing.T) {
			sv := NewScrollView()
			sv.SetChild(&mockWidget{size: geometry.Size{Width: 100, Height: 1000}})
			win := testWindowWithSurface(&window{})
			win.SetWidget(sv)
			defer win.Destroy()
			sv.Arrange(geometry.Rect(0, 0, 100, 100))
			client := new(scrollClientProbe)
			if phase == "deferred-page" {
				win.framePainting = true
			}
			if phase == "connect" {
				client.onConnect = win.Destroy
			} else {
				client.onSet = func(stage string) {
					if stage == "page" {
						win.Destroy()
					}
				}
			}
			if h := sv.BindScroll(layout.DirectionVertical, client); h != nil || len(sv.clientBindings) != 0 || client.values != 0 {
				t.Fatalf("宿主在外部回调中销毁后仍完成了绑定或写入值：handle=%v bindings=%d values=%d destroyed=%t", h, len(sv.clientBindings), client.values, sv.Destroyed())
			}
			win.framePainting = false
			win.finishDestroy()
			client.change.Emit(90)
			if sv.ScrollY() != 0 {
				t.Fatal("销毁后的客户端仍请求滚动")
			}
		})
	}
}

// 小数内容/视口的末端必须可达；快照、滚动条、setter、Reveal和滚轮共享同一范围。
func TestScrollViewFractionalEndIsReachable(t *testing.T) {
	for _, virtual := range []bool{false, true} {
		for _, direction := range []ScrollDirection{ScrollBoth, ScrollVertical, ScrollHorizontal} {
			for _, method := range []string{"setter", "reveal", "wheel"} {
				t.Run(fmt.Sprintf("virtual=%t/direction=%v/%s", virtual, direction, method), func(t *testing.T) {
					size := geometry.Size{Width: 200.5, Height: 300.25}
					sv := NewScrollView()
					sv.SetDirection(direction)
					if virtual {
						sv.SetChild(&extentScrollContent{size: size})
					} else {
						sv.SetChild(&mockWidget{size: size})
					}
					win := testWindowWithSurface(&window{})
					win.width, win.height = 99.25, 100.75
					win.SetWidget(sv)
					defer win.SetWidget(nil)
					win.layoutContent()
					switch method {
					case "setter":
						sv.SetScrollX(1000)
						sv.SetScrollY(1000)
					case "reveal":
						sv.ScrollIntoView(geometry.Rect(size.Width-1, size.Height-1, 1, 1))
					case "wheel":
						if err := win.DispatchEvent(events.WheelEvent{Position: geometry.Point{X: 10, Y: 10}, DeltaX: 1000, DeltaY: 1000, Mode: events.WheelDeltaPixel}); err != nil {
							t.Fatal(err)
						}
					}
					area, info := sv.contentAreaSize(), sv.Snapshot()
					for _, axis := range []struct {
						enabled                                               bool
						content, viewport, offset, reported, barMax, barValue float32
					}{
						{sv.canScrollX(), sv.contentWidth, area.Width, sv.ScrollX(), info.MaxScrollX, sv.hbar.max, sv.hbar.value},
						{sv.canScrollY(), sv.contentHeight, area.Height, sv.ScrollY(), info.MaxScrollY, sv.vbar.max, sv.vbar.value},
					} {
						if !axis.enabled {
							if axis.offset != 0 || axis.reported != 0 {
								t.Fatal("禁用轴仍可滚动")
							}
							continue
						}
						end := float64(axis.offset) + float64(axis.viewport)
						if end < float64(axis.content) || end-float64(axis.content) >= 1 || axis.offset != float32(math.Trunc(float64(axis.offset))) || axis.offset != axis.reported || axis.offset != axis.barMax || axis.offset != axis.barValue {
							t.Fatalf("末端被裁切、过量留白或范围不一致：%+v", axis)
						}
					}
				})
			}
		}
	}
}

func TestScrollViewSubDIPOverflowRemainsReachable(t *testing.T) {
	sv := NewScrollView()
	sv.SetDirection(ScrollVertical)
	sv.SetChild(&mockWidget{size: geometry.Size{Width: 20, Height: 100.25}})
	sv.Arrange(geometry.Rect(0, 0, 80, 100))
	sv.SetScrollY(1000)
	if sv.ScrollY() != 1 || sv.Snapshot().MaxScrollY != 1 || sv.vbar.max != 1 {
		t.Fatal("不足1 DIP的溢出仍被向下取整成不可滚动")
	}
}
