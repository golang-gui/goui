package gui

import (
	"fmt"
	"image/color"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
)

// 使用真实 GUI 分发路径，不创建原生窗口；父容器悬停、按钮按下与焦点可同时存在。
func TestWidgetCombinedStateLifecycle(t *testing.T) {
	win := &window{}
	parent := NewLinearBox(layout.DirectionVertical)
	button := NewButton()
	parent.AddChild(button)
	win.SetWidget(parent)
	t.Cleanup(func() { win.SetWidget(nil) })
	parent.Arrange(geometry.Rect(0, 0, 160, 80))
	button.Arrange(geometry.Rect(8, 8, 100, 40))
	dispatch := func(kind events.EventType) {
		t.Helper()
		if err := win.DispatchEvent(events.PointerEvent{EventType: kind, Button: events.PointerButtonLeft, Position: geometry.Point{X: 20, Y: 20}}); err != nil {
			t.Fatal(err)
		}
	}
	dispatch(events.PointerDown)
	if !parent.State().Has(style.Hovered|style.ContainsFocus) || !button.State().Has(style.Hovered|style.Pressed|style.Focused) {
		t.Fatalf("组合状态丢失: parent=%v button=%v", parent.State(), button.State())
	}
	snap := button.Snapshot()
	if !snap.Hovered || !snap.Pressed || !snap.Focused {
		t.Fatal("快照没有投影同一份状态")
	}
	setFocusVisible(win, true)
	if !button.State().Has(style.Focused | style.FocusVisible | style.Pressed) {
		t.Fatal("键盘提示覆盖了按下状态")
	}
	button.SetState(style.Checked|style.Selected, true)
	parent.SetEnabled(false)
	if button.State() != style.Disabled|style.Checked|style.Selected || button.Snapshot().Pressed {
		t.Fatalf("禁用保留了交互状态: %v", button.State())
	}
	parent.SetEnabled(true)
	dispatch(events.PointerUp)
	button.SetState(style.Pressed|style.DropActive, true)
	parent.RemoveChild(button)
	if button.State() != style.Checked|style.Selected {
		t.Fatalf("卸载未清理临时状态: %v", button.State())
	}
	parent.AddChild(button)
	if button.State().Has(style.Pressed) {
		t.Fatal("重挂恢复了过期按下")
	}
}

func TestWidgetStateOwnershipAndInvalidation(t *testing.T) {
	w := newTestWidget()
	measureWidget(w, layout.Unbounded())
	w.SetState(style.Checked|style.Mixed, true)
	if w.styleValid || w.measureValid {
		t.Fatal("状态变化未失效样式与测量")
	}
	measureWidget(w, layout.Unbounded())
	w.SetState(style.Checked|style.Mixed, true)
	if !w.styleValid || !w.measureValid {
		t.Fatal("相同状态重复失效")
	}
	for _, flags := range []style.State{style.Hovered, style.Focused, style.Disabled, style.ContainsFocus, style.FocusVisible, 1 << 31} {
		t.Run(fmt.Sprintf("%x", flags), func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("保留位必须拒绝外部写入")
				}
			}()
			w.SetState(flags, true)
		})
	}
}

// 仅记录绘制指令；透明默认值不能产生额外填充或描边。
type styledCommandPainter struct {
	testButtonBackgroundPainter
	fills, borders int
}

func (p *styledCommandPainter) FillRect(r geometry.Rectangle, b graphics.Brush) {
	p.fills++
	p.testButtonBackgroundPainter.FillRect(r, b)
}
func (p *styledCommandPainter) FillRoundRect(r geometry.Rectangle, a float32, b graphics.Brush) {
	p.fills++
	p.testButtonBackgroundPainter.FillRoundRect(r, a, b)
}
func (p *styledCommandPainter) DrawRect(r geometry.Rectangle, w float32, b graphics.Brush) {
	p.borders++
	p.testButtonBackgroundPainter.DrawRect(r, w, b)
}
func (p *styledCommandPainter) DrawRoundRect(r geometry.Rectangle, a, w float32, b graphics.Brush) {
	p.borders++
	p.testButtonBackgroundPainter.DrawRoundRect(r, a, w, b)
}

func TestStyledPaintingHelpersAndBoxes(t *testing.T) {
	for _, box := range []Widget{NewLinearBox(layout.DirectionVertical), NewWrapBox(layout.DirectionHorizontal)} {
		box.Arrange(geometry.Rect(30, 40, 100, 60))
		p := new(styledCommandPainter)
		box.Paint(p)
		if p.fills != 0 || p.borders != 0 {
			t.Fatal("未配置外观的 Box 应不产生指令")
		}
		scope := NewStyleScope()
		scope.SetChild(box)
		scope.SetStyleSheet(style.Sheet(style.Name("card").BackgroundColor(color.NRGBA{R: 200, A: 128}).Radius(8).BorderWidth(2).BorderColor(color.White), style.Name("card").State(style.Checked|style.Selected).Radius(12)))
		box.SetStyleName("card")
		box.base().SetState(style.Checked|style.Selected, true)
		p = new(styledCommandPainter)
		box.Paint(p)
		if p.fills != 1 || p.borders != 1 || p.rect != geometry.Rect(0, 0, 100, 60) || p.radius != 12 || p.drawRect != geometry.Rect(1, 1, 98, 58) || p.drawRadius != 11 || p.brush != graphics.ColorOf(color.NRGBA{R: 200, A: 128}) {
			t.Fatalf("局部坐标/组合规则/预乘色错误: %+v", p)
		}
	}
	p := new(styledCommandPainter)
	invisible := style.Default().BackgroundColor(color.Transparent).BorderColor(color.Transparent).BorderWidth(2).Style
	PaintStyledBox(p, geometry.Rect(0, 0, 40, 30), invisible)
	PaintStyledBox(p, geometry.Rect(0, 0, 0, 30), style.Default().BackgroundColor(color.Black).BorderColor(color.White).BorderWidth(2).Style)
	if p.fills != 0 || p.borders != 0 {
		t.Fatal("透明或空矩形产生了无效绘制")
	}
	p = new(styledCommandPainter)
	s := style.Default().BackgroundColor(color.Black).BorderColor(color.White).BorderWidth(2).Style
	PaintStyledBorder(p, geometry.Rect(0, 0, 40, 30), s)
	if p.fills != 0 || p.borders != 1 {
		t.Fatal("仅描边接口重复绘制了背景")
	}
}

// 状态通知合并到帧，只有实际测量变化才安排布局；卸载移除宿主引用。
type stateStyleProbe struct {
	WidgetBase
	notified int
	callback func()
}

func (w *stateStyleProbe) Measure(c layout.Constraint) layout.Measurement {
	size, _ := WidgetStyle(w, "state-probe", "").FontSize()
	return layout.Measured(c.Clamp(geometry.Size{Width: size * 4, Height: 20}))
}
func (w *stateStyleProbe) StyleChanged() {
	w.notified++
	if w.callback != nil {
		w.callback()
	}
}
func TestWidgetStateDeferredGeometryAndLifecycle(t *testing.T) {
	useTestApplication(t, &application{style: style.Sheet(
		style.Name("state-probe").FontSize(10),
		style.Name("state-probe").State(style.Checked).FontSize(20),
	)})
	win := &window{}
	w := new(stateStyleProbe)
	win.SetWidget(w)
	t.Cleanup(func() { win.SetWidget(nil) })
	measureWidget(w, layout.Unbounded())
	win.layoutDirty = false
	w.SetState(style.Pressed, true)
	w.SetState(style.Pressed, false)
	if win.layoutDirty || w.notified != 1 || len(win.stateStyles) != 1 {
		t.Fatal("状态通知未延迟/合并")
	}
	win.updateStateStyles()
	if win.layoutDirty || w.notified != 2 {
		t.Fatal("纯视觉状态错误地改变了布局")
	}
	w.SetState(style.Checked, true)
	if win.layoutDirty {
		t.Fatal("输入分发中立即布局会使 chrome 命中失效")
	}
	win.updateStateStyles()
	if !win.layoutDirty || w.measureResult.Width != 80 {
		t.Fatal("改变字体尺寸后未更新布局")
	}
	win.layoutDirty = false
	w.SetState(style.Selected, true)
	w.callback = func() { win.SetWidget(nil) }
	win.updateStateStyles()
	if w.Root() != nil || len(win.stateStyles) != 0 {
		t.Fatal("通知中卸载后残留引用")
	}
	win.SetWidget(w)
	w.callback = nil
	measureWidget(w, layout.Unbounded())
	w.SetState(style.Selected, false)
	if len(win.stateStyles) != 1 {
		t.Fatal("重挂后未接收通知")
	}
	win.SetWidget(nil)
	if len(win.stateStyles) != 0 {
		t.Fatal("卸载没有删除待通知引用")
	}
}

// 超厚描边内轮廓为空时仍绘制完整外轮廓，不能因退化路径丢失边框。
func TestStyledBorderCollapsedInnerContour(t *testing.T) {
	p := new(styledCommandPainter)
	rect := geometry.Rect(2, 3, 10, 8)
	PaintStyledBorder(p, rect, style.Default().BorderWidth(12).BorderColor(color.White).Radius(3).Style)
	if p.fills != 1 || p.borders != 0 || p.rect != rect || p.radius != 3 || p.brush != graphics.ColorOf(color.White) {
		t.Fatal("退化边框未覆盖外轮廓")
	}
}

func TestButtonCombinedFocusPart(t *testing.T) {
	useTestApplication(t, &application{style: style.Sheet(
		style.Name("button").Part("focus").State(style.FocusVisible).BorderWidth(2).BorderColor(color.Black),
		style.Name("button").Part("focus").State(style.Hovered|style.FocusVisible).BorderColor(color.White),
	)})
	win := &window{}
	b := NewButton()
	win.SetWidget(b)
	t.Cleanup(func() { win.SetWidget(nil) })
	b.Arrange(geometry.Rect(0, 0, 80, 30))
	win.SetFocusedWidget(b)
	setFocusVisible(win, true)
	if err := win.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: geometry.Point{X: 10, Y: 10}}); err != nil {
		t.Fatal(err)
	}
	p := new(styledCommandPainter)
	b.Paint(p)
	if p.borders != 1 || p.drawBrush != graphics.ColorOf(color.White) {
		t.Fatal("focus Part 没有收到悬停与键盘焦点组合")
	}
}
