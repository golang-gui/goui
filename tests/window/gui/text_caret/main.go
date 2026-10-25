// 超长行补充：加 -file /outside/repo/long.json -width 800，验证只读文件的冷 End
// 后直接正向拖选。每个事件循环推进一次鼠标，校验水平偏移、EOF 选区及光标像素。
// 不访问系统剪贴板，不保存原文件；全程需保持窗口焦点，提前关闭或超时失败。
// 加 -scrollbar 时不先按 End，通过正常指针事件从首屏拖动水平滑块到最右端。
// 等待测量收敛，检查 EOF 在可见范围、末行真实文字像素及末端点击定位；
// 在 Software/OpenGL、1x/1.5x/2x 重复，不能将暂时空白当作验证通过。
// Cached long-paragraph caret regression. Requires a desktop and native fonts.
// Run: GOUI_PLAT_PAINTER=software GOUI_PLAT_SCALE=1 go run ./tests/window/gui/text_caret
// Also run at scale 2; use software on all platforms for the typography wrapper.
// Initial state: Modern, integrated window, empty TextView in ScrollView.
// Automatic steps use Window.DispatchEvent:
//  1. Load 32 KiB of an original ASCII word and wait for its wrapped layout.
//  2. Scroll, then click three distinct native cluster interiors. Each caret
//     must equal that cluster's leading UTF-8 offset.
//  3. Navigate to document end/start. Selection and scrolling must follow.
//
// After each navigation, a frame and a subsequent timer must complete without
// creating, resizing or exporting metrics for the long paragraph. This is a
// deterministic reuse assertion, not a hardware-specific latency threshold.
// Logged latency spans event dispatch through the first timer after a frame
// (20 ms timer resolution), not display/compositor presentation. Initial layout
// is excluded and can still block. The native metrics supply click expectations;
// byte/cell geometry and reentrant callbacks have separate package tests.
// Platform font differences can change wrapping. The command modifier is
// resolved by GOUI; no native IME or physical mouse acceptance is claimed.
// No clipboard/settings/files are modified. PASS closes the window; closing
// early fails. Restart resets the sample.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func check(ok bool, message string) {
	if !ok {
		panic(message)
	}
}

type observedApp struct {
	gui.Application
	context typography.Context
}

func (a observedApp) Typography() typography.Context { return a.context }

type counters struct{ created, resized, metrics int }

type observedContext struct {
	typography.Context
	counters
	lines         []typography.TextLine
	clusters      []typography.TextCluster
	fieldBaseline float32
}

func (c *observedContext) NewTextLayout(s string, f typography.TextFormat, w, h float32) (typography.TextLayout, error) {
	l, err := c.Context.NewTextLayout(s, f, w, h)
	if err != nil || len(s) < 1024 && s != "Ag中" {
		return l, err
	}
	if len(s) >= 1024 {
		c.created++
	}
	return &observedLayout{TextLayout: l, context: c}, nil
}

type observedLayout struct {
	typography.TextLayout
	context *observedContext
}

func (l *observedLayout) SetSize(w, h float32) {
	oldW, oldH := l.Size()
	if len(l.Text()) >= 1024 && (w != oldW || h != oldH) {
		l.context.resized++
	}
	l.TextLayout.SetSize(w, h)
}

func (l *observedLayout) MeasureMetrics() ([]typography.TextLine, []typography.TextCluster) {
	lines, clusters := l.TextLayout.MeasureMetrics()
	if len(l.Text()) >= 1024 {
		l.context.metrics++
		l.context.lines, l.context.clusters = lines, clusters
	} else if len(lines) > 0 {
		l.context.fieldBaseline = lines[0].Baseline
	}
	return lines, clusters
}

func main() {
	runtime.LockOSThread()
	file := flag.String("file", "", "existing single-line UTF-8 file to verify without writing")
	scrollbar := flag.Bool("scrollbar", false, "drag the horizontal scrollbar to EOF instead of pressing End")
	width := flag.Float64("width", 760, "window width in DIP")
	flag.Parse()
	check(!*scrollbar || *file != "", "-scrollbar requires -file")
	app, err := gui.NewApplication("org.golanggui.TextCaretRegression")
	must(err)
	ctx := &observedContext{Context: app.Typography()}
	gui.App = observedApp{app, ctx}
	sheet := modern.Sheet(modern.Options{})
	if *file != "" {
		sheet = caretProbeSheet{modern.FromSettings(app.Settings())}
	}
	app.SetStyleSheet(sheet)
	chrome := gui.WindowChromeIntegrated
	if *file != "" {
		chrome = gui.WindowChromeNative
	}
	win, err := app.NewWindow(&gui.WindowOptions{
		Size:   geometry.Size{Width: float32(*width), Height: 360},
		Chrome: chrome,
	})
	must(err)
	defer win.Destroy()
	must(win.SetTitle("Wrapped text caret reuse"))
	v := gui.NewTextView()
	scroll := gui.NewScrollView()
	if *file == "" {
		scroll.SetStyleName("text-view")
	}
	scroll.SetChild(v)
	probeRoot := gui.NewOverlay()
	probeRoot.SetChild(scroll)
	probeRoot.AddOverlay(&painterProbe{})
	win.SetWidget(probeRoot)
	must(win.Show())
	win.SetFocusedWidget(v)

	if *file != "" {
		if *scrollbar {
			verifyHorizontalScrollEnd(app, win, v, scroll, *file)
		} else {
			verifyLongHorizontal(app, win, v, scroll, *file)
		}
		return
	}

	const length = 32768
	primary, ok := gui.ModPrimary.Resolve()
	check(ok, "primary modifier unavailable")
	frame, actionFrame := 0, 0
	frameHandle := win.ConnectFrame(func(time.Time) { frame++ })
	defer frameHandle.Disconnect()
	var baseline counters
	var started time.Time
	stage, action, expected := 0, 0, 0
	complete := false
	deadline := time.Now().Add(time.Minute)
	timer := app.NewTimer()
	defer timer.Stop()
	timer.ConnectTimeout(func() {
		check(time.Now().Before(deadline), "caret verification timed out")
		if v.Rect().Width <= 0 || !win.Focused() {
			return
		}
		switch stage {
		case 0:
			v.Model().SetText(strings.Repeat("a", length))
			actionFrame, stage = frame, 1
		case 1:
			if frame <= actionFrame || len(ctx.lines) < 20 {
				return
			}
			scroll.SetScrollY(300)
			actionFrame, stage = frame, 2
		case 2:
			if frame <= actionFrame {
				return
			}
			baseline, started, actionFrame = ctx.counters, time.Now(), frame
			if action < 3 {
				line := 0
				for line+1 < len(ctx.lines) && ctx.lines[line].Y < scroll.ScrollY()+40 {
					line++
				}
				var target typography.TextCluster
				found, cell := false, 0
				for _, c := range ctx.clusters {
					if c.LineIndex != line || c.Width <= 0 {
						continue
					}
					if cell == 10+6*action {
						target, found = c, true
						break
					}
					cell++
				}
				check(found, "missing native click target")
				expected = target.Start
				bounds := v.Snapshot().Bounds
				point := geometry.Point{
					X: bounds.X + v.Padding() + target.X + target.Width/4,
					Y: bounds.Y + v.Padding() + max(0, ctx.fieldBaseline-ctx.lines[0].Baseline) +
						target.Y + target.Height/2 - scroll.ScrollY(),
				}
				for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
					must(win.DispatchEvent(events.PointerEvent{
						EventType: kind, Position: point, Button: events.PointerButtonLeft,
					}))
				}
			} else {
				key := events.KeyEnd
				expected = length
				if action == 4 {
					key, expected = events.KeyHome, 0
				}
				for _, kind := range []events.EventType{events.KeyDown, events.KeyUp} {
					must(win.DispatchEvent(events.KeyEvent{EventType: kind, Key: key, Modifiers: primary}))
				}
			}
			check(v.Selection() == (gui.TextSelection{Anchor: expected, Caret: expected}),
				fmt.Sprintf("action %d: caret=%+v want=%d", action, v.Selection(), expected))
			stage = 3
		case 3:
			if frame <= actionFrame {
				return
			}
			log.Printf("action=%d event-to-post-frame-timer=%v scrollY=%g native-before=%+v native-after=%+v",
				action, time.Since(started), scroll.ScrollY(), baseline, ctx.counters)
			check(ctx.counters == baseline, "caret navigation repeated whole-paragraph layout")
			if action == 3 {
				check(scroll.ScrollY() > 300, "document end did not scroll")
			}
			if action == 4 {
				// The mixed-script field baseline can place ASCII below the
				// native layout origin (4 DIP on the Linux test font). Reveal
				// aligns that actual caret with the padded viewport, not Y=0.
				top := float32(math.Floor(float64(max(0, ctx.fieldBaseline-ctx.lines[0].Baseline))))
				check(scroll.ScrollY() == top, fmt.Sprintf("document start: scroll=%g want=%g", scroll.ScrollY(), top))
				complete = true
				log.Printf("PASS cached text caret bytes=%d typography=%s", length, ctx.Name())
				win.Destroy()
				return
			}
			action++
			stage = 2
		}
	})
	must(timer.Start(20 * time.Millisecond))
	app.Run()
	check(complete, "caret verification aborted")
}

func verifyHorizontalScrollEnd(app gui.Application, win gui.Window, v *gui.TextView, scroll *gui.ScrollView, file string) {
	data, err := os.ReadFile(file)
	must(err)
	v.SetWrapMode(gui.WrapNone)
	v.Model().SetText(string(data))
	length := len(data)
	data = nil
	check(length > 64*1024 && v.Model().LineCount() == 1, "-file expects a long single-line document")
	complete := false
	deadline := time.Now().Add(45 * time.Second)
	stage, step := 0, 0
	var track geometry.Rectangle
	var pressed time.Time
	var priorSize geometry.Size
	var priorOffset float32
	stable := 0
	pointer := func(kind events.EventType, at geometry.Point) {
		e := events.PointerEvent{EventType: kind, Position: at, Button: events.PointerButtonLeft}
		if kind == events.PointerMove {
			e.Button, e.Buttons = 0, events.PointerButtonLeftDown
		}
		must(win.DispatchEvent(e))
	}
	timer := app.NewTimer()
	defer timer.Stop()
	timer.ConnectTimeout(func() {
		check(time.Now().Before(deadline), fmt.Sprintf("scrollbar end timed out: stage=%d offset=%g extent=%v", stage, scroll.ScrollX(), v.ContentSize()))
		if !win.Focused() || v.Rect().Width <= 0 {
			return
		}
		switch stage {
		case 0:
			for _, child := range scroll.Children() {
				if bar, ok := child.(*gui.ScrollBar); ok && bar.Visible() && bar.Orientation() == layout.DirectionHorizontal {
					track = bar.Snapshot().Bounds
				}
			}
			if track.Width <= 0 || v.Snapshot().TextEditing.VisibleRange.End == 0 {
				return
			}
			pointer(events.PointerDown, geometry.Point{X: track.X + 1, Y: track.Y + track.Height/2})
			stage = 1
		case 1:
			step++
			pointer(events.PointerMove, geometry.Point{X: track.X + track.Width*float32(step)/16, Y: track.Y + track.Height/2})
			if step < 17 {
				return
			}
			pressed, stage = time.Now(), 2
		case 2:
			if priorSize == v.ContentSize() && priorOffset == scroll.ScrollX() {
				stable++
			} else {
				stable = 0
			}
			priorSize, priorOffset = v.ContentSize(), scroll.ScrollX()
			// 稳定尺寸不代表远端的前缀上下文已确认；等待同一截止时间内的末块就绪。
			if stable < 5 || time.Since(pressed) < 200*time.Millisecond || v.Snapshot().TextEditing.VisibleRange.End != length {
				return
			}
			pointer(events.PointerUp, geometry.Point{X: track.X + track.Width + 20, Y: track.Y + track.Height/2})
			check(v.Snapshot().TextEditing.VisibleRange.End == length, "scrollbar end did not display EOF")
			img, err := gui.RenderWidget(v, 0)
			must(err)
			scale := float64(img.Bounds().Dx()) / float64(v.Rect().Width)
			ink := 0
			// Modern 浅色主题 sRGB 输出；首行排除边缘、光标和滚动条，只计暗色字形核心。
			for y := int(float64(v.Padding()) * scale); y < min(img.Bounds().Dy(), int(40*scale)); y++ {
				for x := int(float64(v.Padding()) * scale); x < img.Bounds().Dx()-int(float64(v.Padding())*scale); x++ {
					r, g, b, a := img.At(x, y).RGBA()
					if a == 65535 && r < 30000 && g < 30000 && b < 30000 {
						ink++
					}
				}
			}
			check(ink > 0, "scrollbar end is an entirely blank text row")
			bounds := v.Snapshot().Bounds
			point := geometry.Point{X: bounds.X + bounds.Width - v.Padding() - 1, Y: bounds.Y + v.Padding() + 8}
			pointer(events.PointerDown, point)
			pointer(events.PointerUp, point)
			check(v.Selection().Caret == length, fmt.Sprintf("scrollbar-end click missed EOF: %v want=%d", v.Selection(), length))
			stage = 3
		case 3:
			check(scroll.ScrollX() == priorOffset, "clicking EOF moved the corrected viewport")
			complete = true
			log.Printf("PASS scrollbar EOF bytes=%d typography=%s offset=%g extent=%v selection=%v", length, app.Typography().Name(), scroll.ScrollX(), v.ContentSize(), v.Selection())
			win.Destroy()
		}
	})
	must(timer.Start(20 * time.Millisecond))
	app.Run()
	check(complete, "scrollbar end verification aborted")
}

// 只给光标标记专用颜色；其余名称与 part 保留完整的 Modern 主题。
type caretProbeSheet struct{ style.StyleSheet }

func (s caretProbeSheet) Resolve(sel style.Sel) style.Style {
	if sel.Name == "text-view" && sel.Part == "caret" {
		return style.Sheet(style.Name("text-view").Part("caret").ForegroundColor(color.NRGBA{R: 255, B: 255, A: 255})).Resolve(sel)
	}
	return s.StyleSheet.Resolve(sel)
}

func verifyLongHorizontal(app gui.Application, win gui.Window, v *gui.TextView, scroll *gui.ScrollView, file string) {
	data, err := os.ReadFile(file)
	must(err)
	v.SetWrapMode(gui.WrapNone)
	v.Model().SetText(string(data))
	data = nil
	check(v.Model().LineCount() == 1, "-file expects one logical line")
	length := v.Model().Len()
	check(length > 64*1024, "-file expects a long single-line document")
	stage, anchor := 0, 0
	var end, point geometry.Point
	var offset float32
	var held time.Time
	complete := false
	deadline := time.Now().Add(45 * time.Second)
	pointer := func(kind events.EventType, p geometry.Point) {
		r := v.Snapshot().Bounds
		event := events.PointerEvent{EventType: kind, Position: geometry.Point{X: r.X + p.X, Y: r.Y + p.Y}}
		if kind == events.PointerMove {
			event.Buttons = events.PointerButtonLeftDown
		} else {
			event.Button = events.PointerButtonLeft
		}
		must(win.DispatchEvent(event))
	}
	caret := func() (geometry.Point, bool) {
		img, err := gui.RenderWidget(v, 0)
		must(err)
		scale := float64(img.Bounds().Dx()) / float64(v.Rect().Width)
		left, top, bottom := img.Bounds().Dx(), img.Bounds().Dy(), -1
		// sRGB 不透明输出，按洋红核心像素识别光标，允许字体抗锯齿差异。
		for y := 0; y < min(img.Bounds().Dy(), int(64*scale)); y++ {
			for x := 0; x < img.Bounds().Dx(); x++ {
				r, g, b, a := img.At(x, y).RGBA()
				if a == 65535 && r > 64000 && b > 64000 && g < 58000 {
					left = min(left, x)
					top = min(top, y)
					bottom = max(bottom, y)
				}
			}
		}
		if bottom < 0 {
			return geometry.Point{}, false
		}
		return geometry.Point{X: float32(float64(left) / scale), Y: float32(float64(top+bottom) / 2 / scale)}, true
	}
	timer := app.NewTimer()
	defer timer.Stop()
	timer.ConnectTimeout(func() {
		check(time.Now().Before(deadline), fmt.Sprintf("long horizontal caret timed out: stage=%d selection=%v", stage, v.Selection()))
		if !win.Focused() || v.Rect().Width <= 0 {
			return
		}
		if stage >= 2 {
			check(scroll.ScrollX() == offset, fmt.Sprintf("drag moved text: before=%.0f after=%.0f selection=%v", offset, scroll.ScrollX(), v.Selection()))
		}
		switch stage {
		case 0:
			for _, kind := range []events.EventType{events.KeyDown, events.KeyUp} {
				must(win.DispatchEvent(events.KeyEvent{EventType: kind, Key: events.KeyEnd}))
			}
			stage = 1
		case 1:
			p, ok := caret()
			if !ok {
				return
			}
			end = p
			offset = scroll.ScrollX()
			point = geometry.Point{X: end.X - 100, Y: end.Y}
			pointer(events.PointerDown, point)
			anchor = v.Selection().Anchor
			check(anchor < length && anchor >= length-256, "tail drag did not start in the final window")
			log.Printf("BEGIN tail drag bytes=%d offset=%.0f end=%v anchor=%d", length, offset, end, anchor)
			stage = 2
		case 2:
			point.X += 4
			pointer(events.PointerMove, point)
			check(v.Selection().Anchor == anchor && v.Selection().Caret >= anchor && v.Model().Len() == length, "tail drag changed anchor or document")
			if point.X < end.X+16 {
				return
			}
			check(v.Selection().Caret == length, fmt.Sprintf("tail drag missed EOF: %v want %d", v.Selection(), length))
			held = time.Now()
			stage = 3
		case 3:
			// 留出至少两个自动滚动时钟，不能在错误定时器触发前关闭窗口。
			if time.Since(held) < 100*time.Millisecond {
				return
			}
			check(v.Selection() == (gui.TextSelection{Anchor: anchor, Caret: length}), "held drag lost EOF")
			actual, ok := caret()
			check(ok && math.Abs(float64(actual.X-end.X)) <= 1, "final caret pixels moved")
			pointer(events.PointerUp, point)
			complete = true
			log.Printf("PASS long horizontal forward drag typography=%s offset=%.0f selection=%v", app.Typography().Name(), offset, v.Selection())
			win.Destroy()
		}
	})
	must(timer.Start(20 * time.Millisecond))
	app.Run()
	check(complete, "long horizontal verification aborted")
}

// 验证实际窗口的 painter，而非仅根据环境变量推断绘图后端。
type painterProbe struct {
	gui.WidgetBase
	reported bool
}

func (p *painterProbe) Measure(c layout.Constraint) layout.Measurement {
	return layout.Measured(c.Clamp(geometry.Size{Width: 1, Height: 1}))
}
func (p *painterProbe) Paint(draw gui.Painter) {
	if p.reported {
		return
	}
	p.reported = true
	img, err := draw.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
	must(err)
	name := fmt.Sprintf("%T", img)
	img.Destroy()
	prefer := os.Getenv("GOUI_PLAT_PAINTER")
	if prefer == "software" || prefer == "opengl" {
		check(strings.Contains(name, prefer+"."), "requested painter unavailable: "+name)
	}
	log.Printf("actual painter image=%s scale=%g", name, draw.PixelScale())
}
