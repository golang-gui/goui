package widgets

import (
	"image/color"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
)

type treeAppearanceAPI interface {
	SetRowStyle(func(TreeRow) string)
	SetRowContentDepth(func(TreeRow) (int, bool))
}

func treeAppearance(t *testing.T, tree *TreeView) treeAppearanceAPI {
	t.Helper()
	api, ok := any(tree).(treeAppearanceAPI)
	if !ok {
		t.Fatal("TreeView缺少按行样式及content绘制深度入口")
	}
	return api
}

type treeStylePixelPainter struct{ *tablePixelPainter }

func (p *treeStylePixelPainter) FillPath(path graphics.Path, brush graphics.Brush) {
	p.native.FillPath(path, brush)
}

// 采样为sRGB预乘RGBA；1x/2x整数色带内采样，半透明合成只容许1级8位舍入。
func TestTreeRowContentPartUsesIndentedGeometry(t *testing.T) {
	old := gui.App
	t.Cleanup(func() { gui.App = old })
	gui.App = &separatorApp{sheet: style.Sheet(
		style.Name("custom-row").BackgroundColor(color.NRGBA{R: 255, A: 128}).BorderWidth(0).Radius(0),
		style.Name("custom-row").Part("selected").BackgroundColor(color.NRGBA{G: 255, A: 128}),
		style.Name("custom-row").Part("content").BackgroundColor(color.Transparent).
			BorderColor(color.NRGBA{B: 255, A: 128}).BorderWidth(2).BorderSides(style.BorderTop).Radius(0),
	)}
	v, _ := treeFixture()
	defer v.SetModel(nil)
	v.SetExpanded("root", true)
	v.SetSelection([]string{"a"})
	treeLayout(v, 140, 0)
	r := v.realized["a"]
	r.SetStyleName("custom-row") // 旧源码也可编译；直接验证新Part的实际绘制，不能仅查接口存在。
	for _, scale := range []float32{1, 2} {
		pixels := tablePixels(t, r.Rect().Size, scale, func(p *tablePixelPainter) {
			r.Paint(&treeStylePixelPainter{p})
		})
		for _, sample := range []struct {
			x, y int
			want color.RGBA
		}{
			{39, 1, color.RGBA{G: 128, A: 128}},
			{41, 1, color.RGBA{G: 64, B: 128, A: 192}},
			{100, 14, color.RGBA{G: 128, A: 128}},
		} {
			checkTableEdgePixel(t, pixels.RGBAAt(sample.x*int(scale), sample.y*int(scale)), sample.want)
		}
	}
}

func TestTreeRowContentDepthOnlyChangesPaint(t *testing.T) {
	old := gui.App
	t.Cleanup(func() { gui.App = old })
	gui.App = &separatorApp{sheet: style.Sheet(
		style.Name("custom-row").BackgroundColor(color.White).BorderWidth(0),
		style.Name("custom-row").Part("selected").BackgroundColor(color.White),
		style.Name("custom-row").Part("current").BorderWidth(0),
		style.Name("custom-row").Part("content").BackgroundColor(color.Transparent).
			BorderColor(color.Black).BorderWidth(2).BorderSides(style.BorderTop).Radius(0),
	)}
	v, _ := treeFixture()
	defer v.SetModel(nil)
	v.SetExpanded("root", true)
	api := treeAppearance(t, v)
	api.SetRowStyle(func(TreeRow) string { return "custom-row" })
	treeLayout(v, 140, 0)
	r := v.realized["a"]
	rowRect, contentRect := r.Rect(), r.content.(*TreeExpander).Child().Rect()
	for _, tc := range []struct {
		name     string
		depth    int
		override bool
		start    int
	}{
		{"默认深度", 7, false, 40}, {"根级覆盖", 0, true, 24}, {"更深覆盖", 2, true, 56},
		{"负值归零", -2, true, 24}, {"窄行裁剪", 1000, true, 300},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api.SetRowContentDepth(func(TreeRow) (int, bool) { return tc.depth, tc.override })
			treeLayout(v, 140, 0)
			if r != v.realized["a"] || r.Rect() != rowRect || r.content.(*TreeExpander).Child().Rect() != contentRect {
				t.Fatal("装饰深度改变了行壳或实际内容布局")
			}
			pixels := tablePixels(t, r.Rect().Size, 1, func(p *tablePixelPainter) { r.Paint(&treeStylePixelPainter{p}) })
			for x := 0; x < 300; x++ {
				want := color.RGBA{255, 255, 255, 255}
				if x >= tc.start {
					want = color.RGBA{A: 255}
				}
				if got := pixels.RGBAAt(x, 1); got != want {
					t.Fatalf("x=%d像素=%v，预期%v", x, got, want)
				}
			}
		})
	}
	api.SetRowContentDepth(nil)
	v.SetIndentation(20)
	treeLayout(v, 140, 0)
	pixels := tablePixels(t, r.Rect().Size, 1, func(p *tablePixelPainter) { r.Paint(&treeStylePixelPainter{p}) })
	if got := pixels.RGBAAt(43, 1); got != (color.RGBA{255, 255, 255, 255}) || pixels.RGBAAt(44, 1) != (color.RGBA{A: 255}) {
		t.Fatal("恢复默认深度后没有跟随统一Indentation")
	}
}

type treeStyleUnbindDelegate struct {
	*treeTestDelegate
	check func(gui.Widget)
}

func (d *treeStyleUnbindDelegate) Unbind(row TreeRow, w gui.Widget) {
	if d.check != nil {
		d.check(w.Parent())
	}
	d.treeTestDelegate.Unbind(row, w)
}

func TestTreeRowStyleBindingStateAndRecycling(t *testing.T) {
	v, m := treeFixture()
	defer v.SetModel(nil)
	v.SetExpandedIDs([]string{"root", "a"})
	d := &treeStyleUnbindDelegate{treeTestDelegate: &treeTestDelegate{view: v}, check: func(w gui.Widget) {
		if w != nil && w.StyleName() != "" {
			t.Error("解绑回调仍可观察到旧行样式", w.StyleName())
		}
	}}
	v.SetDelegate(d)
	api := treeAppearance(t, v)
	choose := func(row TreeRow) string {
		if row.Selected {
			return "selected-" + row.ID
		}
		if row.Current {
			return "current-" + row.ID
		}
		return "row-" + row.ID
	}
	api.SetRowStyle(choose)
	treeLayout(v, 56, 0)
	root := v.realized["root"]
	content := root.content
	if root.StyleName() != "row-root" {
		t.Fatal("首绑没有应用样式")
	}
	v.SetSelection([]string{"root"})
	treeLayout(v, 56, 0)
	if v.realized["root"] != root || root.content != content || root.StyleName() != "selected-root" {
		t.Fatal("选中变化丢失样式更新或替换了行壳／内容")
	}
	v.SetSelection(nil)
	v.SetCurrent("root")
	treeLayout(v, 56, 0)
	if root.StyleName() != "current-root" {
		t.Fatal("Current变化没有更新回调输入")
	}
	m.Set("root", "Renamed")
	treeLayout(v, 56, 0)
	if v.realized["root"] != root || root.content != content {
		t.Fatal("同ID刷新丢失行身份")
	}
	api.SetRowStyle(func(row TreeRow) string {
		if row.ID == "root" {
			return "target"
		}
		return ""
	})
	for _, offset := range []float32{0, 56, 84, 0} {
		treeLayout(v, 56, offset)
		for id, row := range v.realized {
			want := ""
			if id == "root" {
				want = "target"
			}
			if row.StyleName() != want {
				t.Fatalf("虚拟复用串样式：%s=%q，预期%q", id, row.StyleName(), want)
			}
		}
	}
	api.SetRowStyle(nil)
	treeLayout(v, 56, 0)
	if v.realized["root"].StyleName() != "" || v.Current() != "root" || len(v.Selection()) != 0 {
		t.Fatal("省略样式没有恢复默认，或改变了交互状态")
	}
}

func TestTreeRowStyleCallbackInvalidation(t *testing.T) {
	for _, action := range []string{"换模型", "换委托", "销毁", "回调替换"} {
		t.Run(action, func(t *testing.T) {
			v, model := treeFixture()
			model.Remove("other") // 单次绑定回调的失效保护，不让后续独立行绑定混入断言。
			owner := gui.NewPopover(nil, nil)
			owner.SetWidget(v)
			defer owner.Destroy()
			defer v.SetModel(nil)
			api := treeAppearance(t, v)
			calls := 0
			api.SetRowStyle(func(TreeRow) string {
				calls++
				switch action {
				case "换模型":
					v.SetModel(NewTreeStore[string]())
				case "换委托":
					v.SetDelegate(nil)
				case "销毁":
					owner.Destroy()
				case "回调替换":
					api.SetRowStyle(nil)
				}
				return "stale-style"
			})
			treeLayout(v, 56, 0)
			if calls != 1 {
				t.Fatal("回调失效后重复调用旧样式构建器", calls)
			}
			for _, row := range v.realized {
				if row.StyleName() == "stale-style" {
					t.Fatal("回调失效后提交了旧样式")
				}
			}
		})
	}
}

func TestTreeRowStyleFocusRemainsOnTreeOrChild(t *testing.T) {
	old := gui.App
	t.Cleanup(func() { gui.App = old })
	gui.App = &separatorApp{sheet: style.Sheet(
		style.Name("styled-row").BackgroundColor(color.White).BorderWidth(0),
		style.Name("styled-row").Part("selected").BackgroundColor(color.White),
		style.Name("styled-row").Part("current").BorderColor(color.Black).BorderWidth(1).Radius(0),
		style.Name("styled-row").Part("content").BackgroundColor(color.Transparent).BorderWidth(2).
			BorderSides(style.BorderTop).BorderColor(color.RGBA{B: 255, A: 255}).Radius(0),
		style.Name("styled-row").Part("content").State(style.FocusVisible).BorderColor(color.RGBA{R: 255, A: 255}),
	)}
	v, _ := treeFixture()
	owner := gui.NewPopover(nil, nil) // 惰性宿主，不创建原生表面或桌面连接。
	owner.SetWidget(v)
	defer owner.Destroy()
	input := owner.(gui.ModalTarget)
	focus := owner.(gui.EventTarget)
	api := treeAppearance(t, v)
	api.SetRowStyle(func(TreeRow) string { return "styled-row" })
	treeLayout(v, 100, 0)
	row := v.realized["root"]
	dispatch := func(event events.Event) {
		t.Helper()
		if err := input.DispatchEvent(event); err != nil {
			t.Fatal(err)
		}
	}
	for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
		dispatch(events.PointerEvent{EventType: kind, Button: events.PointerButtonLeft, Position: geometry.Point{X: 60, Y: 14}})
	}
	dispatch(events.KeyEvent{EventType: events.KeyDown, Key: events.KeyTab})
	if !v.Focused() || !v.FocusVisible() || row.Focused() || row.Snapshot().Focused {
		t.Fatal("外观把当前项伪装成真实行焦点")
	}
	check := func(want color.RGBA) {
		t.Helper()
		pixels := tablePixels(t, row.Rect().Size, 1, func(p *tablePixelPainter) { row.Paint(&treeStylePixelPainter{p}) })
		if pixels.RGBAAt(30, 1) != want || pixels.RGBAAt(0, 14) != (color.RGBA{A: 255}) {
			t.Fatal("内容装饰没有获得键盘状态，或当前框未保持整行范围")
		}
	}
	check(color.RGBA{R: 255, A: 255})
	api.SetRowStyle(func(TreeRow) string { return "styled-row" })
	treeLayout(v, 100, 0)
	if focus.FocusedWidget() != v || v.realized["root"] != row {
		t.Fatal("换样式改变了真实焦点或行身份")
	}
	child := row.content.(*TreeExpander).Child()
	child.SetFocusable(true)
	focus.SetFocusedWidget(child)
	if focus.FocusedWidget() != child || v.Focused() || !row.ContainsFocus() {
		t.Fatal("行内控件没有保有真实焦点")
	}
	pixels := tablePixels(t, row.Rect().Size, 1, func(p *tablePixelPainter) { row.Paint(&treeStylePixelPainter{p}) })
	if got := pixels.RGBAAt(30, 1); got != (color.RGBA{B: 255, A: 255}) {
		t.Fatalf("子控件焦点被误当成树的键盘焦点：%v", got)
	}
}
