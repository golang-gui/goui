package main

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"time"

	"github.com/golang-gui/goui/animation"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/example/demo/resource"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/icon/draw"
	"github.com/golang-gui/goui/icon/svg"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/ui"
	wui "github.com/golang-gui/goui/widgets/ui"
)

func (d *demo) loadAssets() error {
	logo, err := svg.Parse(strings.NewReader(resource.LogoMarkSVG()))
	if err != nil {
		return err
	}
	// Branding always keeps its original colors, independently of icon styles.
	// Never add a tintable copy of the logo to the ordinary icon samples.
	d.logo = logo.WithRawColors()
	paths := []string{
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M6 3H14L19 8V21H6Z M14 3V8H19 M9 12H16 M9 16H16" fill="none" stroke="#303840" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M2 5H10L12 7H22V20H2Z" fill="#e0ac42"/><path d="M3 9H21V19H3Z" fill="#f4cd6b"/></svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><circle cx="12" cy="12" r="10" fill="none" stroke="#30a878" stroke-width="2"/><path d="M6 12L10 16L18 7" fill="none" stroke="#30a878" stroke-width="2"/></svg>`,
	}
	for _, text := range paths {
		s, err := svg.Parse(strings.NewReader(text))
		if err != nil {
			return err
		}
		d.icons = append(d.icons, s)
	}
	wordmark, err := svg.Parse(strings.NewReader(resource.LogoSVG()))
	if err != nil {
		return err
	}
	// Fixed Image sample at its native aspect ratio (1220:512).
	d.wordmark = wordmark.WithRawColors().Image(305, 128, graphics.Color{})
	d.icons = append(d.icons, &draw.Source{Draw: func(p graphics.Painter, s graphics.Size, c graphics.Color) {
		p.FillEllipse(graphics.Point{X: s.Width / 2, Y: s.Height / 2}, s.Width*.45, s.Height*.45, c)
		p.DrawLine(graphics.Point{X: s.Width * .3, Y: s.Height * .5}, graphics.Point{X: s.Width * .7, Y: s.Height * .5}, max(1, s.Width*.06), graphics.RGBA(255, 255, 255, 230))
	}})
	colored, err := svg.Parse(strings.NewReader(paths[1]))
	if err != nil {
		return err
	}
	d.icons = append(d.icons, colored.WithRawColors())
	// 固定像素资源：只创建一次，不在 Build/Paint 中反复生成。
	img := image.NewRGBA(image.Rect(0, 0, 280, 110))
	for y := 0; y < 110; y++ {
		for x := 0; x < 280; x++ {
			img.SetRGBA(x, y, color.RGBA{R: byte(40 + x*150/280), G: byte(70 + y*100/110), B: 180, A: 255})
		}
	}
	d.bitmap = img
	return nil
}
func (d *demo) graphicsPage() ui.View {
	icons := []ui.View{}
	for _, source := range d.icons {
		icons = append(icons, ui.Icon(source).Size(36).Style("demo-swatch"))
	}
	return page("graphics", resource.Graphics(), resource.GraphicsHint(),
		section(resource.ImagesTitle(), ui.Image(d.wordmark).MaxWidth(305).MaxHeight(128), ui.Image(d.bitmap).MaxWidth(280).MaxHeight(110), ui.HBox(icons...).Spacing(24), ui.Label(resource.IconsHint())),
		section(resource.DrawingTitle(), newCanvas().ID("canvas").MinHeight(190).MaxHeight(190),
			ui.Button(resource.Capture()).ID("capture-canvas").OnClick(func() {
				w := d.app.FindWidget("main", "canvas")
				if w == nil {
					return
				}
				img, err := gui.RenderWidget(w, 0)
				if err == nil {
					d.capture = img
				}
				d.result(resource.Offscreen(), err)
			}), ui.Image(d.capture).Visible(d.capture != nil).MaxHeight(190)),
		section(resource.AnimationTitle(),
			ui.HWrap(ui.Button(resource.Animate()).ID("animate").OnClick(d.animate), ui.Button(resource.Stop()).ID("stop-animation").OnClick(func() { d.motion.Stop(); d.busy = false; d.report(resource.AnimationStopped()) }),
				ui.Button(resource.Finish()).OnClick(d.motion.Finish),
				wui.Switch(resource.Busy()).Checked(d.busy).OnChange(func(v bool) { d.busy = v; d.app.RequestUpdate() })).Spacing(12).LineSpacing(8),
			ui.HBox(ui.Icon(d.logo).ID("animated-icon").Size(d.motionValue.Get())).
				MainAlign(layout.MainCenter).CrossAlign(layout.CrossCenter).MinHeight(88).MaxHeight(88),
			wui.ProgressBar((d.motionValue.Get()-32)/48).Indeterminate(d.busy),
			wui.ProgressBar((d.motionValue.Get()-32)/48).Shape(wui.ProgressCircular).Indeterminate(d.busy).MinSize(64, 64).MaxSize(64, 64),
			ui.Label(fmt.Sprintf(resource.AnimationValue(), d.motionValue.Get()))),
	)
}
func (d *demo) animate() {
	target := float32(80)
	if d.motionValue.Get() > 55 {
		target = 32
	}
	d.result(resource.StartAnimation(), d.motion.AnimateTo(d.app.FindWindow("main"), target, 650*time.Millisecond, animation.EaseOutCubic))
}

type canvasWidget struct{ gui.WidgetBase }

func (w *canvasWidget) Snapshot() gui.WidgetInfo {
	i := w.WidgetBase.Snapshot()
	i.Role = gui.RoleImage
	i.Text = resource.CanvasDescription()
	return i
}
func (w *canvasWidget) Paint(p gui.Painter) {
	s := gui.ResolveStyle("demo-swatch", style.PartDefault, style.Normal)
	c, _ := s.ForegroundColor()
	fg := graphics.ColorOf(c)
	width := w.Rect().Width
	p.FillRoundRect(geometry.Rect(4, 8, min(240, width*.38), 150), 18, graphics.LinearGradient{Start: graphics.Point{X: 0, Y: 0}, End: graphics.Point{X: 240, Y: 150}, StartColor: fg, EndColor: graphics.RGBA(25, 190, 165, 180)})
	p.FillEllipse(geometry.Point{X: min(190, width*.3), Y: 110}, 55, 45, graphics.RGBA(240, 140, 40, 140))
	p.Save()
	p.SetClipRect(geometry.Rect(width*.45, 8, width*.5, 160))
	p.SetTransform(geometry.Translate(width*.7, 85).Rotate(22))
	p.FillRoundRect(geometry.Rect(-70, -45, 140, 90), 12, fg)
	p.DrawRect(geometry.Rect(-85, -58, 170, 116), 2, graphics.RGBA(40, 190, 135, 255))
	p.Restore()
	path := graphics.MoveTo(width*.45, 165).BezierTo(width*.6, 110, width*.75, 215, width*.92, 150)
	p.DrawPath(path, 3, fg)
}

type canvasView struct{ ui.ViewBase[canvasView] }

func newCanvas() *canvasView                            { v := new(canvasView); v.Self = v; return v }
func (v *canvasView) Build() ui.View                    { return v }
func (*canvasView) Mount(ui.BuildContext) gui.Widget    { return new(canvasWidget) }
func (*canvasView) Update(ui.BuildContext, gui.Widget)  {}
func (*canvasView) Unmount(ui.BuildContext, gui.Widget) {}
