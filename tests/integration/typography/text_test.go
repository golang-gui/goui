//go:build linux || windows || darwin

package typography_test

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/gui/textedit"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/style"
)

// 只记录公开 Paint 的文本原点与排版；表面装饰不参与滚动锚点的独立预期。
func (*paragraphDraw) SnapRect(r geometry.Rectangle) geometry.Rectangle                   { return r }
func (*paragraphDraw) FillRect(geometry.Rectangle, graphics.Brush)                        {}
func (*paragraphDraw) DrawRect(geometry.Rectangle, float32, graphics.Brush)               {}
func (*paragraphDraw) FillRoundRect(geometry.Rectangle, float32, graphics.Brush)          {}
func (*paragraphDraw) DrawRoundRect(geometry.Rectangle, float32, float32, graphics.Brush) {}
func (*paragraphDraw) FillPath(graphics.Path, graphics.Brush)                             {}
func (*paragraphDraw) DrawPath(graphics.Path, float32, graphics.Brush)                    {}

func TestTextViewNativeResizeAnchor(t *testing.T) {
	context := nativeContext(t)
	useTypography(t, context, nil)
	var source strings.Builder
	for row := 0; row < 1000; row++ {
		fmt.Fprintf(&source, "[%06d:start] text editing, viewport layout; [%06d:middle] selection and undo; [%06d:end] 中文测试。\n", row, row, row)
	}
	v := gui.NewTextView()
	v.Model().SetText(source.String())
	scroll := gui.NewScrollView()
	scroll.SetChild(v)
	ownWidget(t, scroll)
	arrange := func(width float32) {
		box := geometry.Rect(0, 0, width, 590)
		scroll.Measure(layout.Tight(box.Size))
		for range 8 {
			scroll.Arrange(box)
		}
	}
	capture := func() *paragraphDraw {
		draw := new(paragraphDraw)
		v.Paint(draw)
		if len(draw.layouts) == 0 {
			t.Fatal("视口没有可观察的实际文本绘制")
		}
		return draw
	}
	arrange(507)
	for offset := float32(890); offset < 918; offset++ {
		arrange(507)
		scroll.SetScrollY(0)
		scroll.SetScrollY(offset)
		before := capture()
		text := before.layouts[0].Text()
		lines, _ := before.layouts[0].MeasureMetrics()
		localY := v.Padding().Top - before.points[0].Y
		line := 0
		for line+1 < len(lines) && lines[line+1].Y <= localY {
			line++
		}
		anchor, within := lines[line].Start, localY-lines[line].Y
		for _, width := range []float32{457, 441, 482, 532, 591, 532, 482, 441, 507} {
			arrange(width)
			after := capture()
			found := false
			for i, l := range after.layouts {
				if l.Text() != text {
					continue
				}
				found = true
				ls, _ := l.MeasureMetrics()
				index := 0
				for index+1 < len(ls) && ls[index+1].Start <= anchor {
					index++
				}
				if delta := after.points[i].Y + ls[index].Y + within - v.Padding().Top; math.Abs(float64(delta)) > 1 {
					t.Fatalf("宽度 %g 改变了原锚定字符的实际绘制位置：偏差 %g DIP，offset=%g", width, delta, offset)
				}
			}
			if !found {
				t.Fatalf("宽度 %g 后原锚定段落离开视口，offset=%g", width, offset)
			}
		}
	}
}

func TestTextViewNativeRevealEnd(t *testing.T) {
	context := nativeContext(t)
	useTypography(t, context, nil)
	var text strings.Builder
	for row := 0; row < 95000; row++ {
		fmt.Fprintf(&text, "[%06d:start] text editing, viewport layout; [%06d:middle] selection and undo; [%06d:end] 中文测试。\n", row, row, row)
	}
	v := gui.NewTextView()
	v.Model().SetText(text.String())
	scroll := gui.NewScrollView()
	scroll.SetChild(v)
	ownWidget(t, scroll)
	arrange := func(width float32) {
		box := geometry.Rect(0, 0, width, 590)
		scroll.Measure(layout.Tight(box.Size))
		for range 100 {
			scroll.Arrange(box)
		}
	}
	arrange(507)
	scroll.SetScrollY(900)
	for _, width := range []float32{441, 591, 507} {
		arrange(width)
	}
	dispatcher, host := new(gui.EventDispatcher), &inputHost{root: scroll, focused: v}
	modifier, ok := gui.ModPrimary.Resolve()
	if !ok {
		t.Fatal("无法解析文档首尾快捷键")
	}
	for _, event := range []events.KeyEvent{
		{EventType: events.KeyDown, Key: events.KeyHome, Modifiers: modifier},
		{EventType: events.KeyDown, Key: events.KeyEnd, Modifiers: modifier | events.ModifierShift},
	} {
		if err := dispatcher.DispatchEvent(host, event); err != nil {
			t.Fatal(err)
		}
	}
	arrange(507)
	info, viewport := v.Snapshot(), scroll.Snapshot()
	if info.TextEditing == nil || info.TextEditing.Selection.Caret != v.Model().Len() || info.TextEditing.VisibleRange.End != v.Model().Len() || math.Abs(float64(viewport.ScrollY-viewport.MaxScrollY)) > 1 {
		t.Fatalf("原生折行重排后没有显示末尾光标：selection=%v，range=%v，scroll=%g/%g", v.Selection(), info.TextEditing, viewport.ScrollY, viewport.MaxScrollY)
	}
}

type inputHost struct{ root, focused gui.Widget }

func (h *inputHost) Widget() gui.Widget        { return h.root }
func (h *inputHost) FocusedWidget() gui.Widget { return h.focused }
func (h *inputHost) SetFocusedWidget(w gui.Widget) bool {
	h.focused = w
	return true
}

// 使用公开 EventDispatcher 和控件状态；不访问编辑器私有选择或原生窗口。
func TestTextEditorNativeNavigation(t *testing.T) {
	for _, single := range []bool{false, true} {
		t.Run(map[bool]string{false: "TextView", true: "TextInput"}[single], func(t *testing.T) {
			context := nativeContext(t)
			useTypography(t, context, nil)
			var widget gui.Widget
			var selectText func(gui.TextSelection)
			var selection func() gui.TextSelection
			var setText func(string)
			var text func() string
			if single {
				v := gui.NewTextInput()
				widget, selectText, selection, setText, text = v, v.SetSelection, v.Selection, v.SetText, v.Text
			} else {
				v := gui.NewTextView()
				widget, selectText, selection, setText, text = v, v.SetSelection, v.Selection, v.Model().SetText, v.Model().Text
			}
			ownWidget(t, widget)
			setText("אבג")
			widget.Arrange(geometry.Rect(0, 0, 200, 80))
			host, dispatcher := &inputHost{root: widget, focused: widget}, new(gui.EventDispatcher)
			key := func(k events.Key, modifiers events.Modifiers) {
				t.Helper()
				if err := dispatcher.DispatchEvent(host, events.KeyEvent{EventType: events.KeyDown, Key: k, Modifiers: modifiers}); err != nil {
					t.Fatal(err)
				}
			}
			selectText(gui.TextSelection{Anchor: 0, Caret: 6})
			key(events.KeyArrowLeft, 0)
			if got := selection(); got != (gui.TextSelection{Anchor: 6, Caret: 6}) {
				t.Fatal("RTL 选区没有向视觉左侧折叠", got)
			}
			key(events.KeyArrowRight, events.ModifierShift)
			if got := selection(); got != (gui.TextSelection{Anchor: 6, Caret: 4}) {
				t.Fatal("RTL 扩选没有按原生 cluster 前进", got)
			}
			key(events.KeyBackspace, 0)
			if text() != "אב" {
				t.Fatal("删除了错误的 RTL cluster", text())
			}
			modifier, ok := gui.ModPrimary.Resolve()
			if !ok {
				t.Fatal("当前平台不支持主快捷键修饰符")
			}
			key(events.KeyZ, modifier)
			if text() != "אבג" || selection() != (gui.TextSelection{Anchor: 6, Caret: 4}) {
				t.Fatal("撤销没有恢复文本和方向选区", text(), selection())
			}
			setText("e\u0301x")
			selectText(gui.TextSelection{})
			key(events.KeyArrowRight, 0)
			if selection().Caret != 3 {
				t.Fatal("原生组合字素被拆开", selection())
			}
			key(events.KeyBackspace, 0)
			if text() != "x" {
				t.Fatal("原生组合字素未整体删除", text())
			}
		})
	}
}

type layoutCalls struct{ created, sizes, metrics, resizes, destroyed int }

type tracedContext struct {
	typography.Context
	calls   layoutCalls
	layouts []*tracedLayout
	regions []image.Rectangle
}

func (c *tracedContext) NewTextLayout(text string, format typography.TextFormat, width, height float32) (typography.TextLayout, error) {
	l, err := c.Context.NewTextLayout(text, format, width, height)
	if err != nil || len(text) <= 1000 {
		return l, err
	}
	c.calls.created++
	traced := &tracedLayout{TextLayout: l, context: c}
	c.layouts = append(c.layouts, traced)
	return traced, nil
}

type tracedLayout struct {
	typography.TextLayout
	context   *tracedContext
	destroyed bool
}

func (l *tracedLayout) MeasureSize() (float32, float32) {
	l.context.calls.sizes++
	return l.TextLayout.MeasureSize()
}
func (l *tracedLayout) MeasureMetrics() ([]typography.TextLine, []typography.TextCluster) {
	l.context.calls.metrics++
	return l.TextLayout.MeasureMetrics()
}
func (l *tracedLayout) SetSize(width, height float32) {
	l.context.calls.resizes++
	l.TextLayout.SetSize(width, height)
}
func (l *tracedLayout) Destroy() {
	if !l.destroyed {
		l.destroyed = true
		l.context.calls.destroyed++
	}
	l.TextLayout.Destroy()
}
func (l *tracedLayout) Rasterize(scale float32, region image.Rectangle, buf []byte) (typography.TextBitmap, error) {
	l.context.regions = append(l.context.regions, region)
	return l.TextLayout.Rasterize(scale, region, buf)
}

func TestTextViewNativeWidthCacheKeepsCompleteContext(t *testing.T) {
	native := nativeContext(t)
	context := &tracedContext{Context: native}
	useTypography(t, context, nil)
	text := "start \u202e" + strings.Repeat("abc 123 العربية עברית e\u0301 ffi 👩‍💻\t ", 600) + "END\u202c"
	if len(text) < 32768 {
		t.Fatal("样本没有覆盖长段落缓存边界")
	}
	v := gui.NewTextView()
	owner := ownWidget(t, v)
	v.SetWrapMode(gui.WrapWordChar)
	v.Model().SetText(text)
	show := func(width, height float32) { v.Arrange(geometry.Rect(0, 0, width, height)) }
	show(240, 200)
	if context.calls.created != 1 {
		t.Fatal("首个宽度没有恰好创建一个完整排版", context.calls)
	}
	a := context.layouts[0]
	lines, clusters := a.MeasureMetrics()
	if len(lines) < 100 {
		t.Fatal("没有完整的原生折行度量")
	}
	w, h := a.Size()
	reference, err := native.NewTextLayout(text, a.Format(), w, h)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(reference.Destroy)
	wantLines, wantClusters := reference.MeasureMetrics()
	if !reflect.DeepEqual(lines, wantLines) || !reflect.DeepEqual(clusters, wantClusters) {
		t.Fatal("缓存排版没有保留完整原生上下文")
	}
	position := strings.Index(text, "END")
	covered := 0
	for _, cluster := range clusters {
		if cluster.Start < position+3 && cluster.Start+cluster.Length > position {
			if cluster.Direction != typography.TextRightToLeft {
				t.Fatal("跨长段落的 RLO 上下文丢失")
			}
			covered += min(cluster.Start+cluster.Length, position+3) - max(cluster.Start, position)
		}
	}
	if covered != 3 {
		t.Fatal("END 没有完整被 cluster 覆盖", covered)
	}
	show(310, 200)
	if context.calls.created != 2 || context.layouts[1] == a || context.layouts[1].TextLayout == a.TextLayout {
		t.Fatal("不同宽度共享了可变原生排版", context.calls)
	}
	before := context.calls
	for range 3 {
		show(240, 280)
		show(310, 320)
		if context.calls != before {
			t.Fatal("恢复宽度或改变高度重复了原生工作", before, context.calls)
		}
	}
	other := gui.NewTextView()
	otherOwner := ownWidget(t, other)
	other.Model().SetText(text)
	other.Arrange(geometry.Rect(0, 0, 240, 200))
	if context.calls.created != 3 || context.layouts[2].TextLayout == a.TextLayout {
		t.Fatal("独立控件共享了原生资源", context.calls)
	}
	fragment, err := textedit.NewFragment(textedit.Range{End: len(text)}, geometry.Size{Width: w, Height: h}, lines, clusters)
	if err != nil {
		t.Fatal(err)
	}
	fragmentGeometry := fragment.Geometry(20)
	beforeCaret := fragmentGeometry.Caret(textedit.Position{Offset: position})
	otherOwner.Destroy()
	owner.Destroy()
	if context.calls.created != context.calls.destroyed {
		t.Fatal("宽度缓存没有配对释放", context.calls)
	}
	fragmentGeometry = fragment.Geometry(20)
	if fragmentGeometry.Caret(textedit.Position{Offset: position}) != beforeCaret {
		t.Fatal("销毁原生排版后普通度量失效")
	}
}

type bitmapSurface struct{ image image.Image }

func (*bitmapSurface) Transparent() bool { return true }
func (s *bitmapSurface) Draw(img image.Image) error {
	s.image = img
	return nil
}

// 只适配 TextInput/TextView 自身 Paint 使用的裁剪栈；不复制 GUI 的控件树遍历或输入路径。
type textPainter struct {
	graphics.Painter
	scale   float32
	clip    geometry.Rectangle
	stack   []geometry.Rectangle
	origins []geometry.Point
}

func (p *textPainter) PixelScale() float32 { return p.scale }
func (p *textPainter) Save()               { p.stack = append(p.stack, p.clip) }
func (p *textPainter) Restore() {
	last := len(p.stack) - 1
	p.SetClipRect(p.stack[last])
	p.stack = p.stack[:last]
}
func (p *textPainter) SetClipRect(r geometry.Rectangle) {
	p.clip = r
	p.Painter.SetClipRect(r)
}
func (p *textPainter) DrawTextLayout(at geometry.Point, l typography.TextLayout) {
	p.origins = append(p.origins, at)
	p.Painter.DrawTextLayout(at, l)
}

func TestNativeTextRasterViewport(t *testing.T) {
	context := &tracedContext{Context: nativeContext(t)}
	useTypography(t, context, style.Sheet(append(gui.DefaultStyleRules(),
		style.Name("text-input").TextColor(color.NRGBA{R: 255, A: 255}),
		style.Name("text-view").TextColor(color.NRGBA{R: 255, A: 255}))...))
	backend, err := software.NewPainter(new(bitmapSurface))
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Destroy()
	for _, mode := range []string{"input", "nowrap", "wrapped"} {
		func() {
			text := strings.Repeat("abcdefghij", 8000)
			width := float32(600)
			var root, editor gui.Widget
			var selectEnd func()
			var selected func() gui.TextSelection
			if mode == "input" {
				v := gui.NewTextInput()
				v.SetText(text)
				v.SetSelection(gui.TextSelection{})
				root, editor, selected = v, v, v.Selection
				selectEnd = func() { v.SetSelection(gui.TextSelection{Anchor: len(text), Caret: len(text)}) }
			} else {
				v := gui.NewTextView()
				if mode == "nowrap" {
					v.SetWrapMode(gui.WrapNone)
				} else {
					width = 48
					text = strings.Repeat("abcdefghij", 1000)
				}
				v.Model().SetText(text)
				scroll := gui.NewScrollView()
				scroll.SetChild(v)
				root, editor, selected = scroll, v, v.Selection
				selectEnd = func() { v.SetSelection(gui.TextSelection{Anchor: len(text), Caret: len(text)}) }
			}
			owner := ownWidget(t, root)
			defer owner.Destroy()
			arrange := func() {
				box := geometry.Rect(0, 0, width, 260)
				root.Measure(layout.Tight(box.Size))
				for range 8 {
					root.Arrange(box)
				}
			}
			arrange()
			for pass := 0; pass < 2; pass++ {
				if pass == 1 {
					selectEnd()
					arrange()
					if selected().Caret != len(text) {
						t.Fatal("末尾选区没有生效", mode, selected())
					}
				}
				for _, scale := range []float32{1, 1.25, 2} {
					for paint := 0; paint < 2; paint++ {
						p := &textPainter{Painter: backend, scale: scale}
						img, err := backend.RenderImage(int(math.Ceil(float64(width*scale))), int(260*scale), scale, func() {
							p.SetClipRect(geometry.Rect(0, 0, width, 260))
							editor.Paint(p)
						})
						if err != nil {
							t.Fatal(mode, err)
						}
						if pass == 1 {
							revealed := false
							if scroll, ok := root.(*gui.ScrollView); ok {
								// 长段落分块后的绘制原点是局部坐标；绝对滚动位置由公开 ScrollView 提供。
								revealed = mode == "wrapped" && scroll.ScrollY() > 10000 || mode == "nowrap" && scroll.ScrollX() > 40000
							} else {
								for _, at := range p.origins {
									revealed = revealed || at.X < -40000
								}
							}
							if !revealed {
								t.Fatal("实际绘制没有滚动到末尾", mode, p.origins)
							}
						}
						red := false
						for y := 0; y < img.Bounds().Dy() && !red; y++ {
							for x := 0; x < img.Bounds().Dx(); x++ {
								r, g, b, a := img.At(x, y).RGBA()
								if r > g+2000 && r > b+2000 && a > 0 {
									red = true
									break
								}
							}
						}
						if !red {
							t.Fatal("可见正文为空白", mode, pass, scale)
						}
					}
				}
			}
		}()
	}
	if len(context.regions) == 0 {
		t.Fatal("没有栅格化长文本")
	}
	for _, r := range context.regions {
		if r.Dx() > 1024 || r.Dy() > 1024 {
			t.Fatal("文字视口请求了无界原生位图", r)
		}
	}
}
