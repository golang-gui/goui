package gui

import (
	"image/color"
	"slices"
	"testing"

	"github.com/golang-gui/goui/core/colors"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/style"
)

type styleProbe struct {
	WidgetBase
	events   []string
	notified int
	size     float32
	onStyle  func()
}

func (w *styleProbe) StyleChanged() {
	w.events = append(w.events, "style")
	w.notified++
	w.size, _ = ResolveStyle(w.StyleName(), "", style.Normal).FontSize()
	if w.onStyle != nil {
		w.onStyle()
	}
}

func (w *styleProbe) Measure(c layout.Constraint) layout.Measurement {
	w.events = append(w.events, "measure")
	return layout.Measured(c.Clamp(geometry.Size{Width: w.size, Height: w.size}))
}

func (w *styleProbe) Paint(Painter) { w.events = append(w.events, "paint") }

func paintStyleTestWidget(w Widget) {
	paintWidget(w, newPainter(&recordingPainterBackend{}, geometry.Rect(0, 0, 400, 400), 1))
}

func TestStyleChangedCoalescesBeforeMeasureAndPaint(t *testing.T) {
	app := &application{}
	useTestApplication(t, app)
	win := &window{}
	app.windows = []*window{win}
	w := &styleProbe{}
	w.SetStyleName("probe")
	win.SetWidget(w)
	c := layout.Unbounded()
	app.SetStyleSheet(style.Sheet(style.Name("probe").FontSize(10)))
	app.SetStyleSheet(style.Sheet(style.Name("probe").FontSize(20)))
	if w.notified != 0 {
		t.Fatal("setters must defer the hook until framework use")
	}
	if got := measureWidget(w, c); got.Width != 20 {
		t.Fatalf("measurement used stale style: %+v", got)
	}
	measureWidget(w, c)
	paintStyleTestWidget(w)
	paintStyleTestWidget(w)
	if !slices.Equal(w.events, []string{"style", "measure", "paint", "paint"}) {
		t.Fatalf("unexpected hook/cache order: %v", w.events)
	}
	win.layoutDirty, win.paintDirty = false, false
	w.SetStyleName("probe")
	if win.layoutDirty || win.paintDirty {
		t.Fatal("unchanged style name scheduled work")
	}
	w.SetStyleName("other")
	if !win.layoutDirty || !win.paintDirty || w.measureValid {
		t.Fatal("style name must invalidate layout and paint without a Base hook call")
	}
	paintStyleTestWidget(w) // also works for paint-only framework entry
	if w.notified != 2 || w.measureValid {
		t.Fatal("paint did not deliver the hook or incorrectly validated measurement")
	}
}

func TestStyleChangedLocalNameDoesNotPropagate(t *testing.T) {
	parent, child := &styleProbe{}, &styleProbe{}
	parent.WidgetBase.AddChild(parent, child)
	measureWidget(parent, layout.Unbounded())
	measureWidget(child, layout.Unbounded())
	parent.SetStyleName("parent-only")
	measureWidget(parent, layout.Unbounded())
	measureWidget(child, layout.Unbounded())
	if parent.notified != 2 || child.notified != 1 || child.StyleName() != "" {
		t.Fatal("local style name leaked into the child")
	}
}

func TestStyleChangedReattachHiddenAndDestroyed(t *testing.T) {
	app := &application{}
	useTestApplication(t, app)
	win := &window{}
	app.windows = []*window{win}
	w := &styleProbe{}
	win.SetWidget(w)
	measureWidget(w, layout.Unbounded())
	win.SetWidget(nil)
	app.SetStyleSheet(style.Sheet()) // detached widgets are not globally registered
	win.SetWidget(w)
	measureWidget(w, layout.Unbounded())
	if w.notified != 2 {
		t.Fatal("reattachment reused the previous style environment")
	}
	w.SetVisible(false)
	app.SetStyleSheet(style.Sheet())
	paintStyleTestWidget(w)
	if w.notified != 2 {
		t.Fatal("hidden paint should not eagerly rebuild resources")
	}
	w.SetVisible(true)
	paintStyleTestWidget(w)
	if w.notified != 3 {
		t.Fatal("showing a hidden widget missed the pending change")
	}
	w.WidgetBase.destroy(w)
	measureWidget(w, layout.Unbounded())
	paintStyleTestWidget(w)
	if w.notified != 3 {
		t.Fatal("destroyed widget received a hook")
	}

	// Defensive check: even if a hook destroys its widget, do not call Measure
	// or Paint afterwards. Tree mutations are not part of the hook contract.
	victim := &styleProbe{}
	victim.onStyle = func() { victim.WidgetBase.destroy(victim) }
	measureWidget(victim, layout.Unbounded())
	paintStyleTestWidget(victim)
	if !slices.Equal(victim.events, []string{"style"}) {
		t.Fatalf("used a widget after destruction: %v", victim.events)
	}
}

func TestStyleChangedIncludesWindowControls(t *testing.T) {
	win := &window{}
	win.controls = newWindowControls(win)
	defer win.controls.release()
	probe := &styleProbe{}
	win.controls.WidgetBase.AddChild(win.controls, probe)
	measureWidget(probe, layout.Unbounded())
	app := &application{windows: []*window{win}}
	useTestApplication(t, app)
	app.SetStyleSheet(style.Sheet())
	measureWidget(probe, layout.Unbounded())
	if probe.notified != 2 {
		t.Fatal("the window-owned decoration tree was not invalidated")
	}
}

type styleTypography struct{ testTypography }

func (c *styleTypography) NewTextLayout(text string, f typography.TextFormat, width, height float32) (typography.TextLayout, error) {
	c.measureSize = geometry.Size{Width: float32(len(text)) * f.Font.Size, Height: f.Font.Size}
	c.lines = []typography.TextLine{{Height: f.Font.Size, Baseline: f.Font.Size * .8}}
	return c.testTypography.NewTextLayout(text, f, width, height)
}

func textStyleSheet(size float32, ink color.Color) style.StyleSheet {
	return style.Sheet(
		style.Name("label").FontFamily("Test Sans").FontSize(size).ForegroundColor(ink),
		style.Name("text-input").FontFamily("Test Sans").FontSize(size).ForegroundColor(ink),
		style.Name("menu-item-text").FontFamily("Test Sans").FontSize(size).ForegroundColor(ink),
	)
}

func TestStyleChangedRefreshesLabelAndTextInputResources(t *testing.T) {
	for _, kind := range []string{"label", "text-input"} {
		t.Run(kind, func(t *testing.T) {
			typo := &styleTypography{}
			app := &application{typo: typo, style: textStyleSheet(10, color.Black)}
			useTestApplication(t, app)
			win := &window{}
			app.windows = []*window{win}
			var w Widget
			var cached func() typography.TextLayout
			if kind == "label" {
				label := NewLabel("hello")
				w, cached = label, func() typography.TextLayout { return label.cachedLayout }
			} else {
				input := NewTextInput()
				input.SetText("hello")
				input.caret = 2
				input.setPreedit("x", 1)
				w, cached = input, func() typography.TextLayout { return input.cachedLayout }
			}
			win.SetWidget(w)
			defer win.SetWidget(nil)
			before := measureWidget(w, layout.Unbounded())
			w.Arrange(geometry.Rect(0, 0, 200, 40))
			paintStyleTestWidget(w)
			old := cached().(*testTextLayout)
			calls := len(typo.calls)
			for range 5 {
				measureWidget(w, layout.Unbounded())
				paintStyleTestWidget(w)
			}
			if len(typo.calls) != calls || cached() != old {
				t.Fatal("stable frames recreated text layouts")
			}
			destroyed := 0
			old.ConnectDestroy(func() { destroyed++ })
			ink := color.RGBA{R: 40, G: 80, B: 160, A: 255}
			app.SetStyleSheet(textStyleSheet(20, ink))
			after := measureWidget(w, layout.Unbounded())
			paintStyleTestWidget(w)
			if after.Height <= before.Height || after.Baseline <= before.Baseline {
				t.Fatalf("old font metrics survived: before=%+v after=%+v", before, after)
			}
			if destroyed != 1 || cached() == old || !colors.Equal(cached().Format().TextColor, ink) {
				t.Fatal("new style did not release old resources and use the new color")
			}
			old = cached().(*testTextLayout)
			app.SetStyleSheet(textStyleSheet(20, color.White))
			paintStyleTestWidget(w)
			if !old.destroyed || !colors.Equal(cached().Format().TextColor, color.White) {
				t.Fatal("color-only change left stale text resources")
			}
			if input, ok := w.(*TextInput); ok && (input.Text() != "hello" || input.caret != 2 || input.preedit != "x" || input.preeditCaret != 1) {
				t.Fatal("style change altered editing state")
			}
		})
	}
}

type stylePopupPlatform struct {
	platform.Platform
	popup *recordingPlatformPopup
}

func (p *stylePopupPlatform) NewPopup(platform.Window, geometry.Size, platform.EventHandler, platform.PopupOptions) (platform.Popup, error) {
	p.popup = &recordingPlatformPopup{}
	return p.popup, nil
}

func (*stylePopupPlatform) NewPainter(platform.Surface) (graphics.Painter, error) {
	return &recordingPainterBackend{}, nil
}

func TestStyleChangedPopoverSizingAndRecreation(t *testing.T) {
	plat := &stylePopupPlatform{}
	app := &application{platform: plat, typo: &styleTypography{}, style: textStyleSheet(10, color.Black)}
	useTestApplication(t, app)
	win := &window{}
	anchor := NewButton()
	win.SetWidget(anchor)
	p := &popover{anchor: anchor}
	p.SetWidget(NewLabel("hello"))
	defer p.Destroy()
	p.measureAndSize()
	app.SetStyleSheet(textStyleSheet(20, color.White))
	if err := p.createNative(win); err != nil {
		t.Fatal(err)
	}
	if p.width != 100 || p.height != 20 {
		t.Fatalf("native creation used the old unattached style: %gx%g", p.width, p.height)
	}
	app.SetStyleSheet(textStyleSheet(30, color.Black))
	if plat.popup.width != 150 || plat.popup.height != 30 {
		t.Fatalf("popover did not resize with font: %+v", plat.popup)
	}
	p.releaseNative()
	app.SetStyleSheet(textStyleSheet(40, color.White))
	if err := p.createNative(win); err != nil {
		t.Fatal(err)
	}
	if p.width != 200 || p.height != 40 {
		t.Fatalf("recreated popover retained old style: %gx%g", p.width, p.height)
	}
}

func TestStyleChangedMenuNaturalSizeReleasesTemporaryText(t *testing.T) {
	typo := &styleTypography{}
	app := &application{typo: typo, style: textStyleSheet(10, color.Black)}
	useTestApplication(t, app)
	model := NewSliceListModel([]*MenuItem{NewMenuItem("long menu label", nil)})
	content := newMenuContent(model, 480, nil)
	win := &window{}
	win.SetWidget(content)
	defer win.SetWidget(nil)
	app.windows = []*window{win}
	before := measureWidget(content, layout.Unbounded())
	app.SetStyleSheet(textStyleSheet(30, color.White))
	after := measureWidget(content, layout.Unbounded())
	if after.Width <= before.Width || after.Height <= before.Height {
		t.Fatalf("menu natural size remained cached: before=%+v after=%+v", before, after)
	}
	for _, text := range typo.layouts {
		if !text.destroyed {
			t.Fatal("temporary menu measurement leaked a TextLayout")
		}
	}
}

type styleListRow struct{ styleProbe }

func (w *styleListRow) Measure(layout.Constraint) layout.Measurement {
	return layout.Measured(geometry.Size{Width: w.size * 20, Height: w.size})
}

type styleListDelegate struct{ setups, binds, unbinds int }

func (d *styleListDelegate) Setup() Widget {
	d.setups++
	w := &styleListRow{}
	w.SetStyleName("row")
	return w
}
func (d *styleListDelegate) Bind(int, Widget)   { d.binds++ }
func (d *styleListDelegate) Unbind(int, Widget) { d.unbinds++ }

func TestStyleChangedScrollableInvalidatesSizesWithoutReload(t *testing.T) {
	app := &application{style: style.Sheet(style.Name("row").FontSize(10))}
	useTestApplication(t, app)
	d := &styleListDelegate{}
	lv := NewListView()
	lv.SetDelegate(d)
	lv.SetModel(NewSliceListModel(make([]int, 100)))
	defer lv.modelHandle.Disconnect()
	sv := NewScrollView()
	sv.SetChild(lv)
	win := &window{rootBase: rootBase{width: 120, height: 60}}
	win.SetWidget(sv)
	defer win.SetWidget(nil)
	app.windows = []*window{win}
	for range 3 {
		win.layoutFrame(sv)
	}
	sv.SetScrollY(120)
	sv.SetScrollY(20)
	sv.SetScrollX(20)
	for range 3 {
		win.layoutFrame(sv)
	}
	if len(lv.pool) == 0 || sv.ScrollY() != 20 || sv.ScrollX() != 20 {
		t.Fatalf("test setup must have pooled rows and a scrolled viewport: pool=%d offset=%g,%g", len(lv.pool), sv.ScrollX(), sv.ScrollY())
	}
	pool := slices.Clone(lv.pool)
	first := lv.items[lv.VisibleIndexes()[0]]
	counts := *d
	app.SetStyleSheet(style.Sheet(style.Name("row").FontSize(10).ForegroundColor(color.White)))
	// ScrollView calls ContentSize instead of measuring its Scrollable child.
	measureWidget(sv, layout.Tight(geometry.Size{Width: 120, Height: 60}))
	if len(lv.heights) != 0 || len(lv.widths) != 0 || lv.first != 0 || lv.firstY != 0 {
		t.Fatal("Scrollable retained its own size/position caches")
	}
	if !slices.Equal(lv.pool, pool) || *d != counts || sv.ScrollY() != 20 || sv.ScrollX() != 20 {
		t.Fatal("style invalidation reloaded rows, cleared the pool or reset scrolling")
	}
	for range 3 {
		win.layoutFrame(sv)
	}
	if lv.items[lv.VisibleIndexes()[0]] != first || *d != counts {
		t.Fatal("same-size restyling replaced or rebound visible rows")
	}
	app.SetStyleSheet(style.Sheet(style.Name("row").FontSize(20)))
	for range 3 {
		win.layoutFrame(sv)
	}
	for _, i := range lv.VisibleIndexes() {
		row := lv.items[i].(*styleListRow)
		if lv.heights[i] != 20 || row.Rect().Height != 20 || row.size != 20 {
			t.Fatalf("row %d retained stale geometry: height=%g rect=%v style=%g", i, lv.heights[i], row.Rect(), row.size)
		}
	}
	if sv.ScrollY() != 20 || sv.ScrollX() != 20 {
		t.Fatal("font growth reset an offset that is still in range")
	}
	counts = *d
	for range 5 {
		win.layoutFrame(sv)
		paintStyleTestWidget(sv)
	}
	if *d != counts {
		t.Fatal("stable frames rebuilt list rows")
	}
}

// Use deterministic glyph pixels to exercise the real Painter ImageCache,
// independently of installed fonts or native typography rasterization.
type stylePixelLayout struct {
	*testTextLayout
	rasters int
}

func TestStyleChangedLocalListRowRemeasures(t *testing.T) {
	app := &application{style: style.Sheet(
		style.Name("row").FontSize(10),
		style.Name("large-row").FontSize(25),
	)}
	useTestApplication(t, app)
	lv := NewListView()
	lv.SetDelegate(&styleListDelegate{})
	lv.SetModel(NewSliceListModel(make([]int, 100)))
	defer lv.modelHandle.Disconnect()
	sv := NewScrollView()
	sv.SetChild(lv)
	win := &window{rootBase: rootBase{width: 120, height: 60}}
	win.SetWidget(sv)
	defer win.SetWidget(nil)
	for range 3 {
		win.layoutFrame(sv)
	}
	row := lv.items[0]
	row.SetStyleName("large-row")
	for range 3 {
		win.layoutFrame(sv)
	}
	if lv.items[0] != row || row.Rect().Height != 25 || lv.heights[0] != 25 {
		t.Fatalf("local style was hidden by row index cache: rect=%v cached=%g", row.Rect(), lv.heights[0])
	}
	row.SetStyleName("row")
	for range 3 {
		win.layoutFrame(sv)
	}
	if row.Rect().Height != 10 || lv.contentWidth != 200 || sv.contentWidth != 200 {
		t.Fatalf("shrinking a styled row retained old extent: rect=%v list=%g scroll=%g", row.Rect(), lv.contentWidth, sv.contentWidth)
	}
}

func (l *stylePixelLayout) Rasterize(float32, []byte) (typography.TextBitmap, error) {
	l.rasters++
	r, g, b, a := l.format.TextColor.RGBA()
	pixels := make([]byte, 4*4*4)
	for i := 0; i < len(pixels); i += 4 {
		pixels[i], pixels[i+1], pixels[i+2], pixels[i+3] = byte(r>>8), byte(g>>8), byte(b>>8), byte(a>>8)
	}
	return typography.TextBitmap{Width: 4, Height: 4, Stride: 16, Pixels: pixels}, nil
}

type stylePixelTypography struct {
	styleTypography
	pixels []*stylePixelLayout
}

func (c *stylePixelTypography) NewTextLayout(text string, f typography.TextFormat, width, height float32) (typography.TextLayout, error) {
	textLayout, err := c.styleTypography.NewTextLayout(text, f, width, height)
	if err != nil {
		return nil, err
	}
	pixels := &stylePixelLayout{testTextLayout: textLayout.(*testTextLayout)}
	c.pixels = append(c.pixels, pixels)
	return pixels, nil
}

func TestStyleChangedUpdatesPixelsAndReusesImageCache(t *testing.T) {
	typo := &stylePixelTypography{}
	red, blue := color.RGBA{R: 255, A: 255}, color.RGBA{B: 255, A: 255}
	app := &application{typo: typo, style: textStyleSheet(10, red)}
	useTestApplication(t, app)
	output := &transparentFrame{}
	backend, err := software.NewPainter(output)
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Destroy()
	win := &window{rootBase: rootBase{
		painter: backend, transparent: true, width: 40, height: 20, pixelWidth: 40, pixelHeight: 20,
	}}
	label := NewLabel("text")
	win.SetWidget(label)
	defer win.SetWidget(nil)
	app.windows = []*window{win}
	for range 4 {
		win.paint()
	}
	if !colors.Equal(output.image.At(1, 1), red) || len(typo.pixels) != 1 || typo.pixels[0].rasters != 1 {
		t.Fatal("stable frames failed to render/cache the original glyph pixels")
	}
	app.SetStyleSheet(textStyleSheet(10, blue))
	for range 4 {
		win.paint()
	}
	if !colors.Equal(output.image.At(1, 1), blue) || len(typo.pixels) != 2 || typo.pixels[1].rasters != 1 || !typo.pixels[0].destroyed {
		t.Fatal("style change retained old image pixels or broke stable-frame caching")
	}
}
