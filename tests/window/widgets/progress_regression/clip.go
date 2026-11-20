package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"slices"
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
// -case=arc复用场景，改为7个旋转角度的四分之一圆弧叠加轨道。
// 每个相交像素按64x64独立面积预期检查预乘RGBA，不只检查四个轴向位置。
// 覆盖0/22.5/45/73/135/223/315度，1/1.25/1.5/2x、亮暗轨道、128/255前景alpha。
// 旋转使用普通Painter变换，保持结构clip；-hold最终保留73度圆弧供桌面检查。
// 此模式的质量预算已在Linux OpenGL／Software通过；Direct2D的既有差异见
// doc/PlanFractionalClipping.md，失败仍正常报告，不据此放宽断言。
// arc-motion遍历整周并在44..46、72..74度逐.25度采样，保留原圆面积断言。
// coverage额外绘制三个半透明凹多边形，要求原始浮点轮廓的矩形像素面积积分，
// 容差2级；相邻直边样本的差值误差不超过两次量化容差之和4级。
// 此专项验证OpenGL的新覆盖率算法；Software的26.6及扫描转换精度使用arc-motion
// 单独对照，不将其计为2级面积专项通过。实际动画另由all模式观察。
func runClip(chrome gui.WindowChromeMode, hold, arcs, motion, coverage bool) {
	app, err := gui.NewApplication("org.golang-gui.ClipRegression")
	must(err)
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 820, Height: 500}, Chrome: chrome})
	must(err)
	defer win.Destroy()
	must(win.SetTitle("GOUI fractional clipping regression"))
	scene := newClipScene(arcs, coverage)
	win.SetWidget(scene)
	stage, painted := 0, -1
	var nativeScale float32
	scales := []float32{1, 1.25, 1.5, 2}
	values := []float32{1, .25, .75}
	angles := []float64{0, 22.5, 45, 73, 135, 223, 315}
	if motion {
		for a := 0; a < 360; a += 15 {
			angles = append(angles, float64(a))
		}
		for i := 0; i <= 8; i++ {
			angles = append(angles, 44+float64(i)/4, 72+float64(i)/4)
		}
		slices.Sort(angles)
		angles = slices.Compact(angles)
	}
	stages := 24
	if arcs {
		stages = len(angles) * 8
	}
	configure := func() {
		s := scales[stage%4]
		dark := stage/4%2 == 1
		value := float32(1)
		if arcs {
			scene.angle = angles[stage/8]
		} else {
			value = values[stage/8]
		}
		scene.dark = dark
		scene.scale = s
		scene.revision = stage
		for _, r := range scene.rings {
			if r.bar != nil {
				r.bar.SetValue(value)
			}
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
		if arcs {
			err := scene.checkArcs(img)
			fmt.Printf("ARC CASE PASS scale=%g dark=%t angle=%g max-error=%.3f", scene.scale, scene.dark, scene.angle, err)
			if len(scene.edges) > 0 {
				fmt.Printf(" edge-error=%.3f motion-error=%.3f", scene.edgeError, scene.motionError)
			}
			fmt.Println()
		} else {
			scene.check(img, values[stage/8])
			fmt.Printf("CLIP CASE PASS scale=%g dark=%t value=%g\n", scene.scale, scene.dark, values[stage/8])
		}
		stage++
		if stage == stages {
			timer.Stop()
			if arcs {
				fmt.Println("ARC REGRESSION PASS")
			} else {
				fmt.Println("CLIP REGRESSION PASS")
			}
			if !hold {
				win.Destroy()
			} else {
				// 保留真实窗口倍率的完整环或73度圆弧，检查半像素位置的桌面呈现。
				scene.scale, scene.revision = nativeScale, stage
				scene.angle = 73
				for _, r := range scene.rings {
					if r.bar != nil {
						r.bar.SetValue(1)
					}
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
	widget      gui.Widget
	bar         *widgets.ProgressBar
	size, phase float32
	alpha       uint8
}
type clipScene struct {
	gui.WidgetBase
	rings                  []clipRing
	scale                  float32
	arcs, dark             bool
	angle                  float64
	revision, arranged     int
	probes                 []*clipProbe
	scroll                 *gui.ScrollView
	viewport               *gui.WidgetBase
	edges                  []*clipEdge
	edgeError, motionError float64
}

func newClipScene(arcs, coverage bool) *clipScene {
	s := &clipScene{scale: 1, arranged: -1, arcs: arcs}
	for _, size := range []float32{48, 64} {
		for _, phase := range []float32{0, .25, .5} {
			for _, alpha := range []uint8{255, 128} {
				r := clipRing{size: size, phase: phase, alpha: alpha}
				if arcs {
					r.widget = &clipArc{scene: s, alpha: alpha}
				} else {
					p := widgets.NewProgressBar()
					p.SetShape(widgets.ProgressCircular)
					p.SetThickness(4)
					p.SetStyleName("clip-ring")
					if alpha == 128 {
						p.SetStyleName("clip-alpha")
					}
					r.widget, r.bar = p, p
				}
				s.rings = append(s.rings, r)
				s.WidgetBase.AddChild(s, r.widget)
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
	if coverage {
		for _, angle := range []float64{22.5, 45, 67.5} {
			e := &clipEdge{scene: s, offset: angle}
			s.edges = append(s.edges, e)
			s.WidgetBase.AddChild(s, e)
		}
	}
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
	for i := range s.edges {
		place(n+6+i, geometry.Rect(24+float32(i)*180+.5/s.scale, 400+.25/s.scale, 96, 72))
	}
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
			// 此模式检查clip是否保留轴向半覆盖，保留25%覆盖差及2级量化预算。
			// 旧裁剪将半覆盖变成0；完整轮廓的更严格质量断言由arc模式执行。
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
	s.checkProbes(img)
}

func (s *clipScene) checkProbes(img image.Image) {
	scale := s.scale
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

type clipArc struct {
	gui.WidgetBase
	scene *clipScene
	alpha uint8
}

func (a *clipArc) Paint(p gui.Painter) {
	radius := a.Rect().Width / 2
	track := a.scene.trackColor()
	p.FillPath(qualityRingPath(radius, radius-4, true), graphics.RGB(track.R, track.G, track.B))
	// 面积参考从顶部向右递增，显式按仿射矩阵的旋转方向换算。
	p.SetTransform(geometry.Translate(radius, radius).Multiply(geometry.Rotate(-float32(a.scene.angle))).Multiply(geometry.Translate(-radius, -radius)))
	p.FillPath(qualityRingPath(radius, radius-4, false), graphics.RGBA(69, 128, 217, a.alpha))
}

func (s *clipScene) trackColor() color.RGBA {
	if s.dark {
		return color.RGBA{R: 70, G: 70, B: 74, A: 255}
	}
	return color.RGBA{R: 225, G: 225, B: 227, A: 255}
}

// 通过公开Path表达标准圆与圆端帽；预期用下面的独立圆面积积分计算。
func qualityRingPath(outer, inner float32, full bool) graphics.Path {
	p := graphics.MoveTo(outer, 0)
	if full {
		return p.ArcTo(outer, outer, 0, 0, 1, outer, 2*outer).
			ArcTo(outer, outer, 0, 0, 1, outer, 0).Close().
			MoveTo(outer, outer-inner).Hole().
			ArcTo(inner, inner, 0, 0, 0, outer, outer+inner).
			ArcTo(inner, inner, 0, 0, 0, outer, outer-inner).Close()
	}
	half := (outer - inner) / 2
	return p.ArcTo(outer, outer, 0, 0, 1, 2*outer, outer).
		ArcTo(half, half, 0, 0, 1, outer+inner, outer).
		ArcTo(inner, inner, 0, 0, 0, outer, outer-inner).
		ArcTo(half, half, 0, 0, 1, outer, 0).Close()
}

func (s *clipScene) checkArcs(img image.Image) float64 {
	track := s.trackColor()
	maxError := float64(0)
	for _, ring := range s.rings {
		r := ring.widget.Rect().Scale(s.scale)
		cx, cy, outer := float64(r.X+r.Width/2), float64(r.Y+r.Height/2), float64(r.Width/2)
		bounds := pixelEnvelope(r.Inset(-1), 1)
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				t, f := arcPixelAreas(image.Pt(x, y), cx, cy, outer, outer-float64(4*s.scale), s.angle*math.Pi/180)
				f *= float64(ring.alpha) / 255
				want := [4]float64{69*f + float64(track.R)*t*(1-f), 128*f + float64(track.G)*t*(1-f), 217*f + float64(track.B)*t*(1-f), 255 * (f + t*(1-f))}
				got := rgbaAt(img, x, y)
				for i, value := range [4]uint8{got.R, got.G, got.B, got.A} {
					err := math.Abs(float64(value) - want[i])
					maxError = max(maxError, err)
					// 1/16物理像素的曲线误差、线性AA与面积覆盖的差别及2级量化。
					// 此边缘用例保留12%覆盖差预算；精确clip／边框仍使用原1级容差。
					if err > 255*.12+2 {
						panic(fmt.Sprintf("圆弧 size=%g phase=%g alpha=%d scale=%g angle=%g (%d,%d)=%v want=%v", ring.size, ring.phase, ring.alpha, s.scale, s.angle, x, y, got, want))
					}
				}
			}
		}
	}
	s.checkProbes(img)
	s.edgeError, s.motionError = 0, 0
	for _, e := range s.edges {
		static, motion := e.check(img)
		s.edgeError, s.motionError = max(s.edgeError, static), max(s.motionError, motion)
	}
	return maxError
}

type edgePoint struct{ x, y float64 }

type edgeKey struct {
	scale float32
	dark  bool
}

type edgeSample struct {
	angle     float64
	pixels    []color.RGBA
	reference [][4]float32
}

type clipEdge struct {
	gui.WidgetBase
	scene    *clipScene
	offset   float64
	previous map[edgeKey]edgeSample
}

func (e *clipEdge) Paint(p gui.Painter) {
	// 凹轮廓进入通用填充路径；用普通Painter变换旋转，保持结构裁剪不变。
	p.SetTransform(geometry.Translate(44, 36).Multiply(geometry.Rotate(-float32(e.scene.angle + e.offset))).Multiply(geometry.Translate(-44, -36)))
	path := graphics.MoveTo(24, 20).LineTo(64, 20).LineTo(64, 52).LineTo(48, 52).LineTo(48, 36).LineTo(24, 36).Close()
	p.FillPath(path, graphics.RGBA(180, 60, 220, 128))
}

func (e *clipEdge) check(img image.Image) (float64, float64) {
	scale, r := float64(e.scene.scale), e.Rect()
	angle := (e.scene.angle + e.offset) * math.Pi / 180
	sn, cs := math.Sin(angle), math.Cos(angle)
	polygon := [6]edgePoint{{24, 20}, {64, 20}, {64, 52}, {48, 52}, {48, 36}, {24, 36}}
	for i, p := range polygon {
		x, y := p.x-44, p.y-36
		polygon[i] = edgePoint{(float64(r.X) + 44 + x*cs - y*sn) * scale, (float64(r.Y) + 36 + x*sn + y*cs) * scale}
	}
	bounds := pixelEnvelope(r, e.scene.scale)
	key := edgeKey{e.scene.scale, e.scene.dark}
	if e.previous == nil {
		e.previous = make(map[edgeKey]edgeSample)
	}
	previous, exists := e.previous[key]
	count := bounds.Dx() * bounds.Dy()
	if !exists {
		previous.pixels = make([]color.RGBA, count)
		previous.reference = make([][4]float32, count)
	}
	adjacent := exists && e.scene.angle > previous.angle && e.scene.angle-previous.angle <= .251
	maxError, maxMotion := float64(0), float64(0)
	i := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			got := rgbaAt(img, x, y)
			values := [4]uint8{got.R, got.G, got.B, got.A}
			old := previous.pixels[i]
			oldValues := [4]uint8{old.R, old.G, old.B, old.A}
			area := polygonPixelArea(polygon, x, y)
			want := [4]float64{180 * 128 / 255.0 * area, 60 * 128 / 255.0 * area, 220 * 128 / 255.0 * area, 128 * area}
			for j, v := range values {
				err := math.Abs(float64(v) - want[j])
				maxError = max(maxError, err)
				if err > 2 {
					panic(fmt.Sprintf("直边 scale=%g angle=%g offset=%g pixel(%d,%d)=%v want=%v", scale, e.scene.angle, e.offset, x, y, got, want))
				}
				if adjacent {
					delta := math.Abs(float64(int(v)-int(oldValues[j])) - (want[j] - float64(previous.reference[i][j])))
					maxMotion = max(maxMotion, delta)
					if delta > 4.0001 {
						panic(fmt.Sprintf("相邻角度覆盖不连续：scale=%g %g→%g pixel(%d,%d) channel=%d error=%g", scale, previous.angle, e.scene.angle, x, y, j, delta))
					}
				}
				previous.reference[i][j] = float32(want[j])
			}
			previous.pixels[i] = got
			i++
		}
	}
	previous.angle = e.scene.angle
	e.previous[key] = previous
	return maxError, maxMotion
}

// 用Sutherland-Hodgman裁切到像素方形，再以鞋带公式求面积；不复用GPU逐边积分。
func polygonPixelArea(polygon [6]edgePoint, x, y int) float64 {
	var a, b [24]edgePoint
	copy(a[:], polygon[:])
	count := len(polygon)
	for side := 0; side < 4; side++ {
		limit := float64(x + side%2)
		if side >= 2 {
			limit = float64(y + side%2)
		}
		coordinate := func(p edgePoint) float64 {
			if side < 2 {
				return p.x
			}
			return p.y
		}
		inside := func(v float64) bool {
			if side%2 == 0 {
				return v >= limit
			}
			return v <= limit
		}
		if count == 0 {
			return 0
		}
		previous, next := a[count-1], 0
		for _, current := range a[:count] {
			pv, cv := coordinate(previous), coordinate(current)
			if inside(pv) != inside(cv) {
				t := (limit - pv) / (cv - pv)
				b[next] = edgePoint{previous.x + t*(current.x-previous.x), previous.y + t*(current.y-previous.y)}
				next++
			}
			if inside(cv) {
				b[next] = current
				next++
			}
			previous = current
		}
		a, b, count = b, a, next
	}
	area := float64(0)
	if count > 0 {
		previous := a[count-1]
		for _, current := range a[:count] {
			area += previous.x*current.y - previous.y*current.x
			previous = current
		}
	}
	return min(1, math.Abs(area)/2)
}

// 点到单位像素方形的最近／最远平方距离，可直接判定完全内／外区域。
func diskPixelBounds(pt image.Point, cx, cy float64) (float64, float64) {
	dx, dy := math.Abs(float64(pt.X)+.5-cx), math.Abs(float64(pt.Y)+.5-cy)
	return math.Pow(max(0, dx-.5), 2) + math.Pow(max(0, dy-.5), 2), (dx+.5)*(dx+.5) + (dy+.5)*(dy+.5)
}

func arcPixelAreas(pt image.Point, cx, cy, outer, inner, angle float64) (float64, float64) {
	sn, cs := math.Sin(angle), math.Cos(angle)
	middle, half := (outer+inner)/2, (outer-inner)/2
	sx, sy, ex, ey := cx+middle*sn, cy-middle*cs, cx+middle*cs, cy+middle*sn
	near, far := diskPixelBounds(pt, cx, cy)
	t, f := float64(-1), float64(-1)
	if near >= outer*outer || far <= inner*inner {
		t = 0
	} else if near >= inner*inner && far <= outer*outer {
		t = 1
	}
	ns, fs := diskPixelBounds(pt, sx, sy)
	ne, fe := diskPixelBounds(pt, ex, ey)
	dx, dy := float64(pt.X)+.5-cx, float64(pt.Y)+.5-cy
	dot1, dot2, reach := dx*sn-dy*cs, dx*cs+dy*sn, .5*(math.Abs(sn)+math.Abs(cs))
	if t == 1 && dot1 >= reach && dot2 >= reach || fs <= half*half || fe <= half*half {
		f = 1
	} else if (t == 0 || dot1+reach <= 0 || dot2+reach <= 0) && ns >= half*half && ne >= half*half {
		f = 0
	}
	if t >= 0 && f >= 0 {
		return t, f
	}
	const samples = 64
	trackCount, arcCount := 0, 0
	for y := 0; y < samples; y++ {
		for x := 0; x < samples; x++ {
			px, py := float64(pt.X)+(float64(x)+.5)/samples, float64(pt.Y)+(float64(y)+.5)/samples
			dx, dy := px-cx, py-cy
			r2 := dx*dx + dy*dy
			inRing := r2 >= inner*inner && r2 <= outer*outer
			if inRing {
				trackCount++
			}
			if inRing && dx*sn-dy*cs >= 0 && dx*cs+dy*sn >= 0 ||
				(px-sx)*(px-sx)+(py-sy)*(py-sy) <= half*half || (px-ex)*(px-ex)+(py-ey)*(py-ey) <= half*half {
				arcCount++
			}
		}
	}
	if t < 0 {
		t = float64(trackCount) / (samples * samples)
	}
	if f < 0 {
		f = float64(arcCount) / (samples * samples)
	}
	return t, f
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
