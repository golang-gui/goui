package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/widgets"
)

// -case=clip：不消除子控件偏移的父容器绘制回归。记录实际后端、缩放及OS/WM。
// 自动验证48/64 DIP、4 DIP环宽、0/0.25/0.5物理像素偏移、128/255透明度，
// 1/1.25/1.5/2x及亮暗、1/0.25/0.75进度；圆边缘按独立圆面积采样，不互比后端。
// 另验证显式小数／空裁剪、Save/Restore、旋转、嵌套裁剪、ScrollView、边框和相邻半透明填充。
// 等待真实布局／绘制后在普通Timer回调中RenderWidget父容器，不在Paint中嵌套绘制。
// 任一断言失败panic；完成输出CLIP REGRESSION PASS并退出。-hold改为真实窗口倍率的
// 完整圆环并保留窗口；按48/64 DIP、0/.25/.5相位、255/128透明度顺序每行排列四个。
// 可截图检查顶部／左侧圆弧与滚动视口；关闭退出。副作用仅本例窗口和日志，不改系统设置。
func runClip(chrome gui.WindowChromeMode, hold bool) {
	app, err := gui.NewApplication("org.golang-gui.ClipRegression")
	must(err)
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 820, Height: 500}, Chrome: chrome})
	must(err)
	defer win.Destroy()
	must(win.SetTitle("GOUI fractional clipping regression"))
	scene := newClipScene()
	win.SetWidget(scene)
	stage, painted := 0, -1
	var nativeScale float32
	scales := []float32{1, 1.25, 1.5, 2}
	values := []float32{1, .25, .75}
	configure := func() {
		s := scales[stage%4]
		dark := stage/4%2 == 1
		value := values[stage/8]
		scene.scale = s
		scene.revision = stage
		for _, r := range scene.rings {
			r.bar.SetValue(value)
		}
		foreground := color.Black
		background := color.White
		if dark {
			foreground = color.White
			background = color.Black
		}
		app.SetStyleSheet(style.Sheet(
			style.Name("window").BackgroundColor(background), style.Name("label").ForegroundColor(foreground),
			style.Name("clip-ring").BackgroundColor(color.Transparent).ForegroundColor(color.NRGBA{R: 255, A: 255}),
			style.Name("clip-alpha").BackgroundColor(color.Transparent).ForegroundColor(color.NRGBA{R: 255, A: 128}),
		))
		scene.RequestLayout()
	}
	configure()
	win.ConnectAfterPaint(func(p gui.Painter, _ geometry.Rectangle) {
		nativeScale = p.PixelScale()
		painted = scene.arranged
		if stage == 0 {
			native, err := p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
			must(err)
			fmt.Printf("CLIP BACKEND %T scale=%g\n", native, p.PixelScale())
			native.Destroy()
		}
	})
	timer := app.NewTimer()
	defer timer.Stop()
	deadline := time.Now().Add(5 * time.Second)
	timer.ConnectTimeout(func() {
		if painted != stage || scene.Rect().Width == 0 {
			if time.Now().After(deadline) {
				panic("裁剪场景未得到真实布局／绘制")
			}
			return
		}
		img, err := gui.RenderWidget(scene, scene.scale)
		must(err)
		scene.check(img, values[stage/8])
		fmt.Printf("CLIP CASE PASS scale=%g dark=%t value=%g\n", scene.scale, stage/4%2 == 1, values[stage/8])
		stage++
		if stage == 24 {
			timer.Stop()
			fmt.Println("CLIP REGRESSION PASS")
			if !hold {
				win.Destroy()
			} else {
				// 保留真实窗口倍率下的完整圆环，便于检查半像素位置的桌面呈现。
				scene.scale, scene.revision = nativeScale, stage
				for _, r := range scene.rings {
					r.bar.SetValue(1)
				}
				scene.RequestLayout()
			}
			return
		}
		configure()
		deadline = time.Now().Add(5 * time.Second)
	})
	win.ConnectDestroy(func() { timer.Stop(); app.Quit() })
	must(win.Show())
	must(timer.Start(40 * time.Millisecond))
	must(app.Run())
}

type clipRing struct {
	bar         *widgets.ProgressBar
	size, phase float32
	alpha       uint8
}
type clipScene struct {
	gui.WidgetBase
	rings              []clipRing
	scale              float32
	revision, arranged int
	probes             []*clipProbe
	scroll             *gui.ScrollView
	viewport           *gui.WidgetBase
}

func newClipScene() *clipScene {
	s := &clipScene{scale: 1, arranged: -1}
	for _, size := range []float32{48, 64} {
		for _, phase := range []float32{0, .25, .5} {
			for _, alpha := range []uint8{255, 128} {
				p := widgets.NewProgressBar()
				p.SetShape(widgets.ProgressCircular)
				p.SetThickness(4)
				p.SetStyleName("clip-ring")
				if alpha == 128 {
					p.SetStyleName("clip-alpha")
				}
				s.rings = append(s.rings, clipRing{p, size, phase, alpha})
				s.WidgetBase.AddChild(s, p)
			}
		}
	}
	for kind := 0; kind < 4; kind++ {
		p := &clipProbe{kind: kind, scene: s}
		s.probes = append(s.probes, p)
		s.WidgetBase.AddChild(s, p)
	}
	// 嵌套裁剪：子项超出父区域，逻辑交集必须先完成，再换算一次物理包络。
	parent := new(gui.WidgetBase)
	parent.SetLayoutManager(clipOverflowLayout{})
	p := &clipProbe{kind: 4, scene: s}
	parent.AddChild(parent, p)
	s.viewport = parent
	s.WidgetBase.AddChild(s, parent)
	scroll := gui.NewScrollView()
	p = &clipProbe{kind: 4, scene: s}
	p.SetMinSize(geometry.Size{Width: 200, Height: 200})
	scroll.SetChild(p)
	scroll.SetDirection(gui.ScrollBoth)
	s.scroll = scroll
	s.WidgetBase.AddChild(s, scroll)
	s.SetLayoutManager(&clipSceneLayout{scene: s})
	return s
}

type clipSceneLayout struct{ scene *clipScene }

func (l *clipSceneLayout) Measure(_ []layout.Child, c layout.Constraint) layout.Measurement {
	return layout.Measured(c.Clamp(geometry.Size{Width: 820, Height: 500}))
}
func (l *clipSceneLayout) Arrange(children []layout.Child, _ geometry.Rectangle) {
	s := l.scene
	place := func(i int, r geometry.Rectangle) {
		children[i].Measure(layout.Tight(r.Size))
		children[i].Arrange(r)
	}
	for i, v := range s.rings {
		place(i, geometry.Rect(24+float32(i%4)*140+v.phase/s.scale, 24+float32(i/4)*128+v.phase/s.scale, v.size, v.size))
	}
	n := len(s.rings)
	for i := 0; i < 4; i++ {
		place(n+i, geometry.Rect(620, 24+float32(i)*85, 64, 64))
	}
	place(n+4, geometry.Rect(620+.5/s.scale, 365+.5/s.scale, 24, 24))
	place(n+5, geometry.Rect(680+.5/s.scale, 365+.5/s.scale, 64, 64))
	s.arranged = s.revision
}

type clipOverflowLayout struct{}

func (clipOverflowLayout) Measure(_ []layout.Child, c layout.Constraint) layout.Measurement {
	return layout.Measured(c.Clamp(geometry.Size{Width: 24, Height: 24}))
}
func (clipOverflowLayout) Arrange(children []layout.Child, _ geometry.Rectangle) {
	children[0].Measure(layout.Tight(geometry.Size{Width: 80, Height: 80}))
	children[0].Arrange(geometry.Rect(-5, -7, 80, 80))
}

type clipProbe struct {
	gui.WidgetBase
	kind  int
	scene *clipScene
}

func (p *clipProbe) Paint(q gui.Painter) {
	s := p.scene.scale
	ink := graphics.RGBA(255, 0, 0, 128)
	switch p.kind {
	case 0:
		q.SetClipRect(geometry.Rect(2.5/s, 3.25/s, 3/s, 2.5/s))
		q.SetTransform(geometry.Rotate(11))
		q.FillRect(geometry.Rect(-100, -100, 300, 300), ink)
	case 1:
		q.Save()
		q.SetClipRect(geometry.Rect(.5/s, .5/s, 0, 4))
		q.FillRect(geometry.Rect(0, 0, 64, 64), ink)
		q.Restore()
		q.FillRect(geometry.Rect(16/s, 16/s, 4/s, 4/s), graphics.RGB(0, 255, 0))
	case 2:
		gui.PaintStyledBox(q, geometry.Rect(2.5/s, 3.5/s, 12/s, 12/s), style.Default().BorderWidth(1/s).BorderColor(color.RGBA{B: 255, A: 255}).Style)
	case 3:
		q.Save()
		for _, x := range []float32{2 / s, 12.5 / s} {
			r := geometry.Rect(x, 2/s, 10.5/s, 6/s)
			q.SetClipRect(r)
			q.FillRect(r, ink)
		}
		q.Restore()
	default:
		q.FillRect(geometry.Rect(-400, -400, 800, 800), ink)
	}
}

func (s *clipScene) check(img image.Image, value float32) {
	scale := s.scale
	for _, v := range s.rings {
		r := v.bar.Rect().Scale(scale)
		cx, cy, rad := float64(r.X+r.Width/2), float64(r.Y+r.Height/2), float64(r.Width/2)
		samples := []image.Point{{int(cx), int(math.Floor(float64(r.Y)))}, {int(cx), int(math.Ceil(float64(r.Y+r.Height))) - 1}, {int(math.Floor(float64(r.X))), int(cy)}, {int(math.Ceil(float64(r.X+r.Width))) - 1, int(cy)}}
		for _, pt := range samples {
			coverage := ringPixelArea(pt, cx, cy, rad, rad-float64(4*scale), float64(value))
			got := rgbaAt(img, pt.X, pt.Y)
			// NanoVG默认曲线离散精度0.25物理像素；允许相应覆盖率差，加2级量化。
			// 半像素处完整圆环应约半覆盖，旧裁剪将其变成0，仍明确超出此误差界限。
			want := coverage * float64(v.alpha)
			tolerance := float64(v.alpha)*.25 + 2
			if math.Abs(float64(got.A)-want) > tolerance || absByte(got.R, got.A) > 1 || got.G != 0 || got.B != 0 {
				panic(fmt.Sprintf("圆环边缘 size=%g phase=%g alpha=%d scale=%g value=%g %v=%v area=%g", v.size, v.phase, v.alpha, scale, value, pt, got, coverage))
			}
		}
		for y := int(math.Floor(float64(r.Y))) - 1; y <= int(math.Ceil(float64(r.Y+r.Height))); y++ {
			for x := int(math.Floor(float64(r.X))) - 1; x <= int(math.Ceil(float64(r.X+r.Width))); x++ {
				got := rgbaAt(img, x, y)
				if int(got.A) > int(v.alpha)+1 {
					panic("圆环半透明边缘重复合成")
				}
			}
		}
	}
	// 显式裁剪、空裁剪／恢复：独立逻辑区域再求物理包络。
	r := s.probes[0].Rect()
	c := geometry.Rect(r.X+2.5/scale, r.Y+3.25/scale, 3/scale, 2.5/scale)
	checkClipPixels(img, r, c, scale, color.RGBA{R: 128, A: 128})
	r = s.probes[1].Rect()
	checkPixelRegion(img, pixelEnvelope(r, scale), image.Rect(roundPixel(r.X+16/scale, scale), roundPixel(r.Y+16/scale, scale), roundPixel(r.X+20/scale, scale), roundPixel(r.Y+20/scale, scale)), color.RGBA{G: 255, A: 255})
	// 普通1物理像素边框不能因裁剪额外衰减。
	r = s.probes[2].Rect()
	x0, y0 := roundPixel(r.X+2.5/scale, scale), roundPixel(r.Y+3.5/scale, scale)
	x1, y1 := roundPixel(r.X+14.5/scale, scale), roundPixel(r.Y+15.5/scale, scale)
	for _, pt := range []image.Point{{(x0 + x1) / 2, y0}, {(x0 + x1) / 2, y1 - 1}, {x0, (y0 + y1) / 2}, {x1 - 1, (y0 + y1) / 2}} {
		checkRGBA(img, pt.X, pt.Y, color.RGBA{B: 255, A: 255})
	}
	r = s.probes[3].Rect()
	checkPixelRegion(img, pixelEnvelope(r, scale), image.Rect(roundPixel(r.X+2/scale, scale), roundPixel(r.Y+2/scale, scale), roundPixel(r.X+23/scale, scale), roundPixel(r.Y+8/scale, scale)), color.RGBA{R: 128, A: 128})
	r = s.viewport.Rect()
	checkClipPixels(img, r.Inset(-2/scale), r, scale, color.RGBA{R: 128, A: 128})
	// ScrollView的滚动条可以绘制；红色内容不得落到视口物理包络之外。
	r = s.scroll.ScrollGeometry().Viewport.Translate(s.scroll.Rect().Pos)
	viewport := pixelEnvelope(r, scale)
	whole := pixelEnvelope(s.scroll.Rect().Inset(-2/scale), scale)
	for y := whole.Min.Y; y < whole.Max.Y; y++ {
		for x := whole.Min.X; x < whole.Max.X; x++ {
			got := rgbaAt(img, x, y)
			if !image.Pt(x, y).In(viewport) && got.R > 0 && got.G == 0 && got.B == 0 {
				panic("滚动内容越过视口包络")
			}
		}
	}
}
func ringPixelArea(pt image.Point, cx, cy, outer, inner, value float64) float64 {
	const n = 64
	middle, half := (outer+inner)/2, (outer-inner)/2
	endX, endY := middle*math.Sin(2*math.Pi*value), -middle*math.Cos(2*math.Pi*value)
	count := 0
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			dx, dy := float64(pt.X)+(float64(x)+.5)/n-cx, float64(pt.Y)+(float64(y)+.5)/n-cy
			rr := dx*dx + dy*dy
			if rr > outer*outer || rr < inner*inner {
				continue
			}
			angle := math.Atan2(dx, -dy)
			if angle < 0 {
				angle += 2 * math.Pi
			}
			// 不完整圆弧带圆端帽，按扇环与两个端帽圆的并集计算。
			if value == 1 || angle <= 2*math.Pi*value ||
				dx*dx+(dy+middle)*(dy+middle) <= half*half ||
				(dx-endX)*(dx-endX)+(dy-endY)*(dy-endY) <= half*half {
				count++
			}
		}
	}
	return float64(count) / (n * n)
}
func roundPixel(v, scale float32) int { return int(math.Floor(float64(v*scale) + .5)) }
func pixelEnvelope(r geometry.Rectangle, scale float32) image.Rectangle {
	return image.Rect(int(math.Floor(float64(r.X*scale))), int(math.Floor(float64(r.Y*scale))), int(math.Ceil(float64((r.X+r.Width)*scale))), int(math.Ceil(float64((r.Y+r.Height)*scale))))
}
func rgbaAt(img image.Image, x, y int) color.RGBA {
	return color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
}
func absByte(a, b uint8) int { return int(math.Abs(float64(a) - float64(b))) }
func checkRGBA(img image.Image, x, y int, want color.RGBA) {
	got := rgbaAt(img, x, y)
	if absByte(got.R, want.R) > 1 || absByte(got.G, want.G) > 1 || absByte(got.B, want.B) > 1 || absByte(got.A, want.A) > 1 {
		panic(fmt.Sprintf("裁剪像素(%d,%d)=%v want=%v", x, y, got, want))
	}
}
func checkClipPixels(img image.Image, region, clip geometry.Rectangle, scale float32, ink color.RGBA) {
	checkPixelRegion(img, pixelEnvelope(region, scale), pixelEnvelope(clip, scale), ink)
}
func checkPixelRegion(img image.Image, area, wantRect image.Rectangle, ink color.RGBA) {
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			want := color.RGBA{}
			if image.Pt(x, y).In(wantRect) {
				want = ink
			}
			checkRGBA(img, x, y, want)
		}
	}
}
