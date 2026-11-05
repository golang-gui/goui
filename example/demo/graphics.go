package main

import (
	"fmt"
	"image"
	"math"
	"time"

	"github.com/golang-gui/goui/animation"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/example/demo/resource"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/icon/svg"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	wui "github.com/golang-gui/goui/widgets/ui"
)

// graphicsDemo owns this feature for the lifetime of the gallery.
// host is borrowed for shared assets and application operations.
type graphicsDemo struct {
	host              *demo
	busy              ui.State[bool]
	motion            *animation.Animation[float32]
	motionValue       ui.State[float32]
	capture           ui.State[image.Image]
	captureSize       geometry.Size
	drawingGeneration ui.State[int]
	drawingFrames     ui.State[bool]
	drawingDirty      bool
	frameConnection   signal.Handle
	drawingStarted    time.Time
}

func (d *graphicsDemo) View() ui.View { return ui.Build(d.view) }

func (d *graphicsDemo) view() ui.View {
	icons := []ui.View{}
	for _, source := range d.host.icons {
		icons = append(icons, ui.Icon(source).Size(36).Style("demo-swatch"))
	}
	return page("graphics", resource.Graphics(), resource.GraphicsHint(),
		section(resource.ImagesTitle(), newWordmark(d.host.wordmark).ID("wordmark"), ui.Image(d.host.bitmap).MaxWidth(280).MaxHeight(110), ui.HBox(icons...).Spacing(24), ui.Label(resource.IconsHint())),
		section(resource.CornerTitle(),
			ui.Label(resource.CornerHint()).WrapMode(gui.WrapWordChar).Style(modern.MutedText),
			ui.HWrap(
				ui.VBox(newCornerSample().ID("corner-first").Style("demo-corner-first"), ui.Label("24 / 4 / 24 / 4 DIP")).CrossAlign(layout.CrossCenter),
				ui.VBox(newCornerSample().ID("corner-second").Style("demo-corner-second"), ui.Label("4 / 24 / 4 / 24 DIP")).CrossAlign(layout.CrossCenter),
			).Spacing(20).LineSpacing(16)),
		section(resource.DrawingTitle(), newCanvas().ID("canvas").MinHeight(190).MaxHeight(190),
			ui.Button(resource.Capture()).ID("capture-canvas").OnClick(func() {
				w := d.host.app.FindWidget("main", "canvas")
				if w == nil {
					return
				}
				img, err := gui.RenderWidget(w, 0)
				if err == nil {
					d.capture.Set(img)
					d.captureSize = w.Rect().Size
				}
				d.host.result(resource.Offscreen(), err)
			}), ui.Image(d.capture.Get()).ID("capture-preview").ContentSize(d.captureSize.Width, d.captureSize.Height).Visible(d.capture.Get() != nil).MaxHeight(190)),
		d.cachedDrawing(),
		section(resource.AnimationTitle(),
			ui.HWrap(ui.Button(resource.Animate()).ID("animate").OnClick(d.animate), ui.Button(resource.Stop()).ID("stop-animation").OnClick(func() { d.motion.Stop(); d.busy.Set(false); d.host.report(resource.AnimationStopped()) }),
				ui.Button(resource.Finish()).OnClick(d.motion.Finish),
				wui.Switch(resource.Busy()).Checked(d.busy.Get()).OnChange(func(v bool) { d.busy.Set(v) })).Spacing(12).LineSpacing(8),
			ui.Build(func() ui.View {
				return ui.VBox(
					ui.HBox(ui.Icon(d.host.logo).ID("animated-icon").Size(d.motionValue.Get())).
						MainAlign(layout.MainCenter).CrossAlign(layout.CrossCenter).MinHeight(88).MaxHeight(88),
					wui.ProgressBar((d.motionValue.Get()-32)/48).Indeterminate(d.busy.Get()),
					wui.ProgressBar((d.motionValue.Get()-32)/48).Shape(wui.ProgressCircular).Indeterminate(d.busy.Get()).MinSize(64, 64).MaxSize(64, 64),
					ui.Label(fmt.Sprintf(resource.AnimationValue(), d.motionValue.Get()))).Spacing(12).CrossAlign(layout.CrossStretch)
			})),
	)
}

// Shadow space is owned by this sample, inside its normal rectangular clip.
// Both shadow and body use the same resolved style and native Painter backend.
type cornerSampleWidget struct{ gui.WidgetBase }

func (w *cornerSampleWidget) Measure(c layout.Constraint) layout.Measurement {
	if !w.Visible() {
		return layout.Measurement{}
	}
	return layout.Measured(c.Clamp(geometry.Size{Width: 240, Height: 136}))
}

func (w *cornerSampleWidget) Paint(p gui.Painter) {
	s := gui.WidgetStyle(w, "widget", "")
	rect := geometry.Rectangle{Size: w.Rect().Size}.Inset(20)
	if rect.Width <= 0 || rect.Height <= 0 {
		return
	}
	radius, _ := s.CornerRadius()
	if shadow, ok := s.Shadow(); ok && shadow.Color != nil {
		p.DrawBoxShadow(rect, radius, graphics.BoxShadow{Color: graphics.ColorOf(shadow.Color), Offset: shadow.Offset, BlurRadius: shadow.BlurRadius, SpreadRadius: shadow.SpreadRadius})
	}
	gui.PaintStyledBox(p, rect, s)
}

func (w *cornerSampleWidget) Snapshot() gui.WidgetInfo {
	info := w.WidgetBase.Snapshot()
	info.Role = gui.RoleImage
	info.Text = resource.CornerTitle()
	return info
}

type cornerSampleView struct{ ui.ViewBase[cornerSampleView] }

func newCornerSample() *cornerSampleView                      { v := new(cornerSampleView); v.Self = v; return v }
func (*cornerSampleView) Mount(ui.BuildContext) gui.Widget    { return new(cornerSampleWidget) }
func (*cornerSampleView) Update(ui.BuildContext, gui.Widget)  {}
func (*cornerSampleView) Unmount(ui.BuildContext, gui.Widget) {}

const wordmarkWidth, wordmarkHeight float32 = 305, 128

// The horizontal SVG uses a rectangular drawing area rather than Icon's square
// allocation. Parsing is done once; pixels follow the current paint scale.
type wordmarkWidget struct {
	gui.WidgetBase
	source    *svg.Source
	native    graphics.Image
	pixelSize image.Point
}

func (w *wordmarkWidget) Measure(c layout.Constraint) layout.Measurement {
	if !w.Visible() || w.source == nil {
		return layout.Measurement{}
	}
	return layout.Measured(c.Clamp(geometry.Size{Width: wordmarkWidth, Height: wordmarkHeight}))
}

func (w *wordmarkWidget) Paint(p gui.Painter) {
	if !w.Visible() || w.source == nil {
		return
	}
	bounds := w.Rect()
	fit := min(float32(1), bounds.Width/wordmarkWidth, bounds.Height/wordmarkHeight)
	if fit <= 0 {
		return
	}
	width, height := wordmarkWidth*fit, wordmarkHeight*fit
	scale := p.PixelScale()
	pw, ph := math.Ceil(float64(width)*float64(scale)), math.Ceil(float64(height)*float64(scale))
	if pw < 1 || ph < 1 || pw > 4096 || ph > 4096 || math.IsNaN(pw) || math.IsNaN(ph) {
		return
	}
	size := image.Pt(int(pw), int(ph))
	if w.native == nil || w.pixelSize != size {
		pixels := w.source.Image(size.X, size.Y, graphics.Color{})
		if pixels == nil || pixels.Bounds().Empty() {
			return
		}
		native, err := p.NewImage(pixels)
		if err != nil {
			return
		}
		w.releaseImage()
		w.native, w.pixelSize = native, size
	}
	p.DrawImage(geometry.Rect(0, 0, width, height), w.native)
}

func (w *wordmarkWidget) releaseImage() {
	if w.native != nil {
		w.native.Destroy()
		w.native = nil
	}
	w.pixelSize = image.Point{}
}

func (w *wordmarkWidget) Snapshot() gui.WidgetInfo {
	info := w.WidgetBase.Snapshot()
	info.Role = gui.RoleImage
	info.Text = "GOUI"
	return info
}

type wordmarkView struct {
	ui.ViewBase[wordmarkView]
	source *svg.Source
}

func newWordmark(source *svg.Source) *wordmarkView {
	v := &wordmarkView{source: source}
	v.Self = v
	return v
}

func (v *wordmarkView) Mount(ui.BuildContext) gui.Widget {
	w := &wordmarkWidget{}
	w.ConnectUnmount(w.releaseImage)
	return w
}
func (v *wordmarkView) Update(_ ui.BuildContext, widget gui.Widget) {
	w := widget.(*wordmarkWidget)
	if w.source != v.source {
		w.releaseImage()
		w.source = v.source
		w.RequestLayout()
	}
}
func (*wordmarkView) Unmount(ui.BuildContext, gui.Widget) {}

func (d *graphicsDemo) animate() {
	target := float32(80)
	if d.motionValue.Get() > 55 {
		target = 32
	}
	d.host.result(resource.StartAnimation(), d.motion.AnimateTo(d.host.app.FindWindow("main"), target, 650*time.Millisecond, animation.EaseOutCubic))
}

type canvasWidget struct{ gui.WidgetBase }

func (w *canvasWidget) Snapshot() gui.WidgetInfo {
	i := w.WidgetBase.Snapshot()
	i.Role = gui.RoleImage
	i.Text = resource.CanvasDescription()
	return i
}
func (w *canvasWidget) Paint(p gui.Painter) {
	s := gui.WidgetStyle(w, "demo-swatch", style.PartDefault)
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
func (*canvasView) Mount(ui.BuildContext) gui.Widget    { return new(canvasWidget) }
func (*canvasView) Update(ui.BuildContext, gui.Widget)  {}
func (*canvasView) Unmount(ui.BuildContext, gui.Widget) {}

func newGraphicsDemo(host *demo) *graphicsDemo {
	d := &graphicsDemo{host: host, motionValue: ui.MakeState(float32(32))}
	d.motion = animation.New(float32(32), animation.Float32)
	d.motion.ConnectUpdate(d.motionValue.Set)

	return d
}

func (d *graphicsDemo) cachedDrawing() ui.View {
	generation := d.drawingGeneration.Get()
	return section(resource.CachedDrawing(),
		ui.Label(resource.CachedDrawingHint()).WrapMode(gui.WrapWordChar).Style(modern.MutedText),
		ui.DrawingArea(func(p gui.Painter, size geometry.Size) {
			d.drawingDirty = false
			// Paint may precede the queued declaration update.
			phase := float32(d.drawingGeneration.Peek()) * .5
			if d.drawingFrames.Peek() {
				phase += float32(time.Since(d.drawingStarted).Seconds()) * 2
			}
			p.FillRoundRect(geometry.Rectangle{Size: size}.Inset(1), 12, graphics.RGBA(40, 115, 190, 150))
			x := size.Width/2 + float32(math.Sin(float64(phase)))*max(0, size.Width/2-50)
			p.FillEllipse(geometry.Point{X: x, Y: size.Height / 2}, 30, 30, graphics.RGBA(245, 170, 55, 220))
			p.Save()
			p.SetClipRect(geometry.Rect(16, 16, max(0, size.Width-32), max(0, size.Height-32)))
			p.SetTransform(geometry.Translate(size.Width*.65, size.Height/2).Rotate(phase * 30))
			p.DrawRect(geometry.Rect(-25, -25, 50, 50), 3, graphics.RGB(240, 245, 255))
			p.Restore()
			p.ClearPath(graphics.MoveTo(size.Width-45, 16).LineTo(size.Width-16, 16).LineTo(size.Width-16, 45).Close())
		}).ID("cached-drawing").MinHeight(150).MaxHeight(150).OnShouldDraw(func(redraw *bool) { *redraw = *redraw || d.drawingDirty || d.drawingFrames.Peek() }),
		ui.HWrap(ui.Button(resource.RegenerateDrawing()).ID("drawing-regenerate").OnClick(d.regenerateDrawing),
			wui.Switch(resource.DrawingFrames()).ID("drawing-frames").Checked(d.drawingFrames.Get()).OnChange(d.setDrawingFrames),
			ui.Label(fmt.Sprintf(resource.DrawingGeneration(), generation)),
		).Spacing(12).LineSpacing(8),
	)
}
func (d *graphicsDemo) regenerateDrawing() {
	d.drawingDirty = true
	d.drawingGeneration.Set(d.drawingGeneration.Peek() + 1)
}
func (d *graphicsDemo) setDrawingFrames(on bool) {
	d.drawingFrames.Set(on)
	if d.frameConnection != nil {
		d.frameConnection.Disconnect()
		d.frameConnection = nil
	}
	if !on {
		return
	}
	w := d.host.app.FindWindow("main")
	if w == nil {
		d.drawingFrames.Set(false)
		return
	}
	d.drawingStarted = time.Now()
	area := d.host.app.FindWidget("main", "cached-drawing")
	d.frameConnection = w.ConnectAfterPaint(func(gui.Painter, geometry.Rectangle) {
		// Reacquire only if reconciliation replaced the borrowed widget.
		if area == nil || area.Root() != w {
			area = d.host.app.FindWidget("main", "cached-drawing")
		}
		if gui.IsDrawable(area) {
			_ = w.RequestPaint()
		}
	})
	_ = w.RequestPaint()
}
func (d *graphicsDemo) stop() {
	d.motion.Stop()
	d.busy.Set(false)
	d.setDrawingFrames(false)
}
