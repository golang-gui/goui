package widgets

import (
	"encoding/json"
	"errors"
	"image/color"
	"math"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/style"
)

// 不创建原生窗口。Popover 替身只替代原生显示，输入仍走 EventDispatcher。
type dropDownTestPopup struct {
	gui.Popover
	widget, focus gui.Widget
	visible       bool
	err           error
	dispatcher    gui.EventDispatcher
}

func (p *dropDownTestPopup) Widget() gui.Widget { return p.widget }
func (p *dropDownTestPopup) Visible() bool      { return p.visible }
func (p *dropDownTestPopup) Show() error {
	if p.Placement() != gui.PopoverPlacementBottom {
		return errors.New("DropDown did not request Bottom placement")
	}
	if p.err == nil {
		p.visible = true
	}
	return p.err
}
func (p *dropDownTestPopup) Hide()                              { p.visible = false }
func (p *dropDownTestPopup) Destroy()                           { p.visible = false }
func (p *dropDownTestPopup) FocusedWidget() gui.Widget          { return p.focus }
func (p *dropDownTestPopup) SetFocusedWidget(w gui.Widget) bool { p.focus = w; return true }
func (p *dropDownTestPopup) key(t *testing.T, key events.Key, mods events.Modifiers) {
	t.Helper()
	handled := false
	if err := p.dispatcher.DispatchEvent(p, events.KeyEvent{EventType: events.KeyDown, Key: key, Modifiers: mods, Handled: &handled}); err != nil {
		t.Fatal(err)
	}
	if !handled {
		t.Fatal("popup key was not consumed")
	}
}

type dropDownFixedContent struct {
	gui.WidgetBase
	height float32
	width  float32
}

func (w *dropDownFixedContent) Measure(c layout.Constraint) layout.Measurement {
	width := w.width
	if width == 0 {
		width = 70
	}
	return layout.Measurement{Size: c.Clamp(geometry.Size{Width: width, Height: w.height}), HasBaseline: true, Baseline: 10}
}

type dropDownTestDelegate struct {
	setups, binds, unbinds int
	width                  float32
}

func (d *dropDownTestDelegate) Setup() gui.Widget {
	d.setups++
	return &dropDownFixedContent{height: 16, width: d.width}
}
func (d *dropDownTestDelegate) Bind(_ int, w gui.Widget) { d.binds++; w.RequestLayout() }
func (d *dropDownTestDelegate) Unbind(int, gui.Widget)   { d.unbinds++ }

type dropDownReentrantDelegate struct {
	dropDownTestDelegate
	onUnbind func()
}

func (d *dropDownReentrantDelegate) Unbind(int, gui.Widget) {
	fn := d.onUnbind
	d.onUnbind = nil
	if fn != nil {
		fn()
	}
}

func TestDropDownDelegateReentryDoesNotOverwriteNewerSelection(t *testing.T) {
	d, _, h, dispatcher := dropDownInput()
	defer d.SetModel(nil)
	delegate := new(dropDownReentrantDelegate)
	d.SetDelegate(delegate)
	d.SetSelected(0)
	delegate.onUnbind = func() { d.SetSelected(3) }
	notified := 0
	d.ConnectSelected(func(int) { notified++ })
	switchKey(t, dispatcher, h, events.KeyArrowDown, false)
	if d.Selected() != 3 || notified != 0 {
		t.Fatal("older transition overwrote reentrant setter")
	}
}

type dropDownButtonDelegate struct{ clicks *int }

func (d dropDownButtonDelegate) Setup() gui.Widget {
	b := gui.NewButton()
	b.SetChild(&dropDownFixedContent{height: 16})
	b.ConnectClicked(func() { *d.clicks++ })
	return b
}
func (dropDownButtonDelegate) Bind(int, gui.Widget)   {}
func (dropDownButtonDelegate) Unbind(int, gui.Widget) {}

func TestDropDownDisabledOptionBlocksCustomActions(t *testing.T) {
	d, _, _, _ := dropDownInput()
	defer d.SetModel(nil)
	clicks := 0
	d.SetDelegate(dropDownButtonDelegate{&clicks})
	popup := dropDownPopup(d)
	if err := d.Open(); err != nil {
		t.Fatal(err)
	}
	measured := d.content.Measure(layout.Unbounded())
	d.content.Arrange(geometry.Rectangle{Size: measured.Size})
	row := d.content.list.Children()[1]
	// Sample the arranged child center, independent of font metrics.
	point := row.Children()[0].Rect().Center()
	point.X += row.Rect().X + d.content.scroll.Rect().X
	point.Y += row.Rect().Y + d.content.scroll.Rect().Y
	h := &tabInputHost{root: d.content, focus: d.content}
	dispatcher := new(gui.EventDispatcher)
	dispatchTabPointer(t, dispatcher, h, events.PointerDown, point.X, point.Y)
	dispatchTabPointer(t, dispatcher, h, events.PointerUp, point.X, point.Y)
	if clicks != 0 || d.Selected() != -1 || !popup.Visible() {
		t.Fatal("disabled custom action fired")
	}
}

func dropDownInput() (*DropDown, *gui.SliceListModel[DropDownItem], *tabInputHost, *gui.EventDispatcher) {
	d := NewDropDown()
	m := gui.NewSliceListModel([]DropDownItem{{Text: "A"}, {Text: "Disabled", Disabled: true}, {Text: "C"}, {Text: "D"}})
	d.SetModel(m)
	d.SetDelegate(new(dropDownTestDelegate))
	d.Measure(layout.Tight(geometry.Size{Width: 180, Height: 32}))
	d.Arrange(geometry.Rect(0, 0, 180, 32))
	return d, m, &tabInputHost{root: d, focus: d}, new(gui.EventDispatcher)
}
func dropDownPopup(d *DropDown) *dropDownTestPopup {
	d.content = newDropDownContent(d)
	p := &dropDownTestPopup{Popover: gui.NewPopover(d, nil), widget: d.content}
	d.popup = p
	return p
}

func TestDropDownSelectionAndModel(t *testing.T) {
	d, m, h, dispatch := dropDownInput()
	if d.Selected() != -1 || !d.Enabled() || !d.Focusable() || d.Padding() != 6 || d.PopupMaxHeight() != 320 || d.Opened() {
		t.Fatal("defaults")
	}
	calls := 0
	d.ConnectSelected(func(index int) {
		calls++
		if index != d.Selected() || d.Opened() {
			t.Fatal("signal before commit/close")
		}
	})
	d.SetSelected(1) // 程序可以选择禁用项。
	d.SetSelected(999)
	d.SetSelected(-10)
	if calls != 0 || d.Selected() != -1 {
		t.Fatal("setter signaled / failed to normalize")
	}
	switchKey(t, dispatch, h, events.KeyArrowDown, false)
	switchKey(t, dispatch, h, events.KeyArrowDown, false)
	if calls != 2 || d.Selected() != 2 {
		t.Fatal("Down did not skip disabled option")
	}
	switchKey(t, dispatch, h, events.KeyArrowUp, false)
	switchKey(t, dispatch, h, events.KeyArrowUp, false)
	if calls != 3 || d.Selected() != 0 {
		t.Fatal("Up wrapped or did not skip")
	}
	d.SetSelected(3)
	m.Set(3, DropDownItem{Text: "changed"})
	if d.Selected() != 3 || d.Snapshot().Text != "changed" || calls != 3 {
		t.Fatal("model notification lost position")
	}
	m.Remove(3)
	if d.Selected() != -1 || calls != 3 {
		t.Fatal("model shrink not cleared silently")
	}
	d.SetSelected(2)
	d.SetModel(m)
	if d.Selected() != -1 {
		t.Fatal("explicit SetModel did not reinstall")
	}
	d.SetEnabled(false)
	switchKey(t, dispatch, h, events.KeyArrowDown, false)
	if d.Selected() != -1 || d.Focusable() || calls != 3 {
		t.Fatal("disabled input")
	}
	d.SetFocusable(false)
	d.SetEnabled(true)
	if d.Focusable() {
		t.Fatal("enabled lost focusable preference")
	}
	for _, value := range []float32{-1, float32(math.NaN()), float32(math.Inf(1))} {
		d.SetPadding(value)
		d.SetPopupMaxHeight(value)
		if d.Padding() != 0 || d.PopupMaxHeight() != 0 {
			t.Fatal("nonfinite layout value")
		}
	}
	d.SetModel(nil)
}

func TestDropDownBrowseConfirmCancel(t *testing.T) {
	d, m, _, _ := dropDownInput()
	defer d.SetModel(nil)
	p := dropDownPopup(d)
	d.SetSelected(0)
	calls := 0
	d.ConnectSelected(func(int) { calls++ })
	if err := d.Open(); err != nil {
		t.Fatal(err)
	}
	p.key(t, events.KeyArrowDown, 0)
	if d.current != 2 || d.Selected() != 0 || calls != 0 {
		t.Fatal("browse committed selection")
	}
	d.SetSelected(0)
	d.Refresh()
	if !d.Opened() || d.current != 2 {
		t.Fatal("rebuild interrupted browse")
	}
	p.key(t, events.KeyEscape, 0)
	if d.Opened() || d.Selected() != 0 || d.current != -1 {
		t.Fatal("cancel changed selection")
	}
	_ = d.Open()
	p.key(t, events.KeyEnd, 0)
	p.key(t, events.KeyEnter, 0)
	if d.Opened() || d.Selected() != 3 || calls != 1 {
		t.Fatal("confirm did not commit once")
	}
	_ = d.Open()
	p.key(t, events.KeyEnter, 0)
	if calls != 1 {
		t.Fatal("same item signaled")
	}
	_ = d.Open()
	p.key(t, events.KeyHome, 0)
	if d.current != 0 || d.Selected() != 3 {
		t.Fatal("Home committed")
	}
	m.Set(0, DropDownItem{Text: "A", Disabled: true})
	if d.Opened() || d.Selected() != 3 {
		t.Fatal("model change kept stale popup")
	}
	d.SetSelected(-1)
	_ = d.Open()
	if d.current != 2 {
		t.Fatal("initial browse did not skip disabled")
	}
	d.SetEnabled(false)
	if d.Opened() || d.current != -1 {
		t.Fatal("disable kept popup")
	}
}

func TestDropDownClickAndOpenErrors(t *testing.T) {
	d, _, h, dispatch := dropDownInput()
	defer d.SetModel(nil)
	errorsSeen := 0
	d.ConnectOpenError(func(error) { errorsSeen++ })
	// 未挂载：主动 Open 返回错误，输入触发才发送 OpenError。
	if d.Open() == nil || errorsSeen != 0 {
		t.Fatal("explicit Open error contract")
	}
	dispatchTabPointer(t, dispatch, h, events.PointerDown, 80, 16)
	dispatchTabPointer(t, dispatch, h, events.PointerMove, 210, 16)
	dispatchTabPointer(t, dispatch, h, events.PointerUp, 210, 16)
	if errorsSeen != 0 {
		t.Fatal("outside release opened")
	}
	dispatchTabPointer(t, dispatch, h, events.PointerDown, 80, 16)
	dispatchTabPointer(t, dispatch, h, events.PointerUp, 80, 16)
	if errorsSeen != 1 {
		t.Fatal("click failure not delivered once")
	}
	p := dropDownPopup(d)
	p.err = errors.New("native failure")
	if d.Open() == nil || errorsSeen != 1 || d.Opened() {
		t.Fatal("Show failure reported twice")
	}
	d.SetModel(gui.NewSliceListModel([]DropDownItem{{Disabled: true}}))
	if err := d.Open(); err != nil || d.Opened() {
		t.Fatal("fully disabled popup attempted Show")
	}
	d.SetModel(nil)
	if err := d.Open(); err != nil {
		t.Fatal("empty popup attempted Show")
	}
}

func TestDropDownDelegateVirtualizationAndGeometry(t *testing.T) {
	d, _, _, _ := dropDownInput()
	defer d.SetModel(nil)
	delegate := new(dropDownTestDelegate)
	d.SetDelegate(delegate)
	d.SetSelected(0)
	d.content = newDropDownContent(d)
	c := d.content
	got := c.Measure(layout.Unbounded())
	if got.Width != 180 || got.Height != 124 {
		t.Fatalf("short content should fit: %v", got)
	}
	c.Arrange(geometry.Rectangle{Size: got.Size})
	row := c.list.Children()[0].(*dropDownRow)
	if row.content == d.display || d.display.Parent() != d || row.content.Parent() != row {
		t.Fatal("content reused across slots")
	}
	display := d.display
	d.Refresh()
	if d.display != display {
		t.Fatal("refresh replaced collapsed content")
	}
	selected := new(dropDownTestDelegate)
	d.SetSelectedDelegate(selected)
	if d.display == display || row.delegate != delegate || selected.setups != 1 {
		t.Fatal("selected override changed list delegate")
	}
	items := make([]DropDownItem, 10000)
	for i := range items {
		items[i].Text = "option"
	}
	d.SetModel(gui.NewSliceListModel(items))
	before := delegate.setups
	got = c.Measure(layout.Unbounded())
	c.Arrange(geometry.Rectangle{Size: got.Size})
	if got.Height != 320 || delegate.setups-before > 20 || len(c.list.Children()) > 20 {
		t.Fatal("popup measured/created all options")
	}
	before = delegate.setups
	c.scroll.SetScrollY(20000)
	c.Arrange(geometry.Rectangle{Size: got.Size})
	if delegate.setups-before > 20 {
		t.Fatal("large scroll did not recycle")
	}
	d.SetPopupMaxHeight(0)
	got = c.Measure(layout.Loose(geometry.Size{Width: 100, Height: 210}))
	if got.Width != 100 || got.Height != 210 {
		t.Fatal("workarea constraint ignored")
	}
	d.SetSelected(2)
	d.Measure(layout.Tight(geometry.Size{Width: 180, Height: 32}))
	measured := d.Measure(layout.Loose(geometry.Size{Width: 180, Height: 32}))
	if !measured.HasBaseline || measured.Baseline != 16 {
		t.Fatalf("baseline %v", measured)
	}
}

func dropDownVerticalBar(t *testing.T, c *dropDownContent) *gui.ScrollBar {
	t.Helper()
	for _, child := range c.scroll.Children() {
		if bar, ok := child.(*gui.ScrollBar); ok && bar.Orientation() == layout.DirectionVertical {
			return bar
		}
	}
	t.Fatal("vertical scrollbar missing")
	return nil
}

func TestDropDownPopupGutters(t *testing.T) {
	d, _, _, _ := dropDownInput()
	defer d.SetModel(nil)
	popup := dropDownPopup(d)
	if err := d.Open(); err != nil {
		t.Fatal(err)
	}
	c := d.content
	layoutContent := func() {
		size := c.Measure(layout.Unbounded()).Size
		c.Arrange(geometry.Rectangle{Size: size})
	}
	checkGutters := func(vertical bool) {
		t.Helper()
		row := c.list.Children()[0]
		bar := dropDownVerticalBar(t, c)
		left := c.scroll.Rect().X + row.Rect().X
		if left != 6 || bar.Visible() != vertical {
			t.Fatal("leading padding or scrollbar visibility", left, bar.Visible())
		}
		right := c.Rect().Width - c.scroll.Rect().X - row.Rect().X - row.Rect().Width
		if vertical {
			right = bar.Rect().X - row.Rect().X - row.Rect().Width
			if c.Rect().Width-c.scroll.Rect().X-bar.Rect().X-bar.Rect().Width != 6 {
				t.Fatal("scrollbar outer padding")
			}
		}
		if right != left || row.Snapshot().Bounds.Width != row.Rect().Width {
			t.Fatal("asymmetric visual/pickable bounds", left, right, row.Snapshot())
		}
	}
	layoutContent()
	checkGutters(false)
	d.SetPopupMaxHeight(96)
	layoutContent()
	checkGutters(true)
	row := c.list.Children()[0]
	bar := dropDownVerticalBar(t, c)
	point := geometry.Point{X: c.scroll.Rect().X + bar.Rect().X - 3, Y: c.scroll.Rect().Y + row.Rect().Y + row.Rect().Height/2}
	// The blank gutter is outside the option's hit area: no selection or close.
	h := &tabInputHost{root: c, focus: c}
	dispatch := new(gui.EventDispatcher)
	dispatchTabPointer(t, dispatch, h, events.PointerDown, point.X, point.Y)
	dispatchTabPointer(t, dispatch, h, events.PointerUp, point.X, point.Y)
	if d.Selected() != -1 || !popup.Visible() {
		t.Fatal("gutter click activated an option")
	}
	d.Close()
	d.SetModel(gui.NewSliceListModel([]DropDownItem{{Text: "only"}}))
	layoutContent()
	checkGutters(false)
	d.SetModel(gui.NewSliceListModel([]DropDownItem{{Text: "A"}, {Text: "B"}, {Text: "C"}, {Text: "D"}}))
	layoutContent()
	checkGutters(true)
}

func TestDropDownPopupGutterPreservesWideContent(t *testing.T) {
	d, _, _, _ := dropDownInput()
	defer d.SetModel(nil)
	d.SetDelegate(&dropDownTestDelegate{width: 400})
	d.SetPopupMaxHeight(96)
	c := newDropDownContent(d)
	d.content = c
	size := c.Measure(layout.Unbounded()).Size
	c.Arrange(geometry.Rectangle{Size: size})
	if !dropDownVerticalBar(t, c).Visible() || c.scroll.Snapshot().MaxScrollX <= 0 {
		t.Fatal("wide scrolling fixture")
	}
	c.scroll.SetScrollX(c.scroll.Snapshot().MaxScrollX)
	row := c.list.Children()[0].(*dropDownRow)
	bar := dropDownVerticalBar(t, c)
	if row.Rect().Width != 430 || row.content.Rect().Width != 400 || bar.Rect().X-row.Rect().X-row.Rect().Width != 6 {
		t.Fatal("gutter clipped wide content or disappeared at scroll end", row.Rect(), row.content.Rect(), bar.Rect())
	}
	if row.Rect().X+row.content.Rect().X+row.content.Rect().Width > bar.Rect().X-6 {
		t.Fatal("wide content right edge remains outside viewport")
	}
}

func TestDropDownPopupGutterPixels(t *testing.T) {
	// Premultiplied sRGB RGBA8, opaque blue option on white. Integer geometry
	// and Radius(0) allow exact samples: body x=[6,160), gutter x=[160,166).
	useProgressApplication(t, &progressTestApplication{sheet: style.Sheet(
		style.Name("drop-down-item").BackgroundColor(color.RGBA{B: 255, A: 255}).Radius(0).BorderWidth(0),
		style.Name("drop-down-item").Part("check").ForegroundColor(color.RGBA{R: 255, A: 255}),
	)})
	for _, scale := range []float32{1, 2} {
		d := NewDropDown()
		d.SetDelegate(new(dropDownTestDelegate))
		d.SetModel(gui.NewSliceListModel([]DropDownItem{{Text: "A"}, {Text: "B"}, {Text: "C"}, {Text: "D"}}))
		d.SetSelected(0)
		d.Arrange(geometry.Rect(0, 0, 180, 32))
		d.SetPopupMaxHeight(96)
		c := newDropDownContent(d)
		d.content = c
		size := c.Measure(layout.Unbounded()).Size
		c.Arrange(geometry.Rectangle{Size: size})
		row := c.list.Children()[0].(*dropDownRow)
		out := new(progressPixelSurface)
		painter, err := software.NewPainter(out)
		if err != nil {
			t.Fatal(err)
		}
		painter.Begin(180*scale, 96*scale, scale)
		painter.Clear(graphics.ColorOf(color.White))
		painter.SetTransform(geometry.Translate(6, 6))
		row.Paint(dropDownPixelPainter{checkPixelPainter{native: painter}})
		painter.End()
		for _, x := range []int{3, 163} {
			if got := out.pixels.RGBAAt(x*int(scale), 20*int(scale)); got != (color.RGBA{255, 255, 255, 255}) {
				t.Fatalf("gutter painted at scale=%g x=%d: %v", scale, x, got)
			}
		}
		if got := out.pixels.RGBAAt(159*int(scale), 20*int(scale)); got != (color.RGBA{0, 0, 255, 255}) {
			t.Fatalf("option background edge scale=%g: %v", scale, got)
		}
		painter.Destroy()
		d.SetModel(nil)
	}
}

func TestDropDownOptionInputSnapshotAndReentry(t *testing.T) {
	d, _, _, _ := dropDownInput()
	defer d.SetModel(nil)
	p := dropDownPopup(d)
	_ = d.Open()
	d.content.Measure(layout.Unbounded())
	d.content.Arrange(geometry.Rect(0, 0, 180, 120))
	rows := d.content.list.Children()
	if len(rows) != 4 {
		t.Fatal("realized rows")
	}
	h := &tabInputHost{root: d.content, focus: d.content}
	dispatch := new(gui.EventDispatcher)
	click := func(y float32) {
		dispatchTabPointer(t, dispatch, h, events.PointerDown, 20, y)
		dispatchTabPointer(t, dispatch, h, events.PointerUp, 20, y)
	}
	click(44) // disabled index 1, between popup padding and scroll offset
	if d.Selected() != -1 || !d.Opened() {
		t.Fatal("disabled row accepted click")
	}
	click(76)
	if d.Selected() != 2 || d.Opened() || p.visible {
		t.Fatal("row click did not confirm")
	}
	info := d.Snapshot()
	attr := info.Attributes[DropDownInfoKey].(DropDownInfo)
	if info.Role != RoleComboBox || info.Text != "C" || attr.Index != 2 || attr.Count != 4 || attr.Expanded {
		t.Fatal("collapsed snapshot")
	}
	if _, err := json.Marshal(info); err != nil {
		t.Fatal(err)
	}
	_ = d.Open()
	p.key(t, events.KeyEnd, 0)
	option := rows[3].Snapshot()
	attr = option.Attributes[DropDownInfoKey].(DropDownInfo)
	if option.Role != RoleOption || option.Selected || !attr.Current || d.content.Snapshot().Role != RoleListBox {
		t.Fatal("browsing snapshot masquerades as selection")
	}
	late := 0
	d.ConnectSelected(func(int) { d.SetSelected(0) })
	d.ConnectSelected(func(int) { late++ })
	p.key(t, events.KeyEnter, 0)
	if late != 0 || d.Selected() != 0 {
		t.Fatal("stale notification survived reentry")
	}
}

func TestDropDownUnmountReleasesModelAndPopup(t *testing.T) {
	d, m, _, _ := dropDownInput()
	h := newProgressTestHost()
	h.AddChild(h, d)
	p := dropDownPopup(d)
	_ = d.Open()
	h.RemoveChild(d)
	if p.visible || d.popup != nil || d.modelHandle != nil || d.content.list.Model() != nil || d.bound != -1 {
		t.Fatal("unmount retained resources")
	}
	m.SetItems([]DropDownItem{{Text: "replaced"}})
	h.AddChild(h, d)
	if d.modelHandle == nil {
		t.Fatal("remount lost model subscription")
	}
	h.RemoveChild(d)
	d.SetModel(nil)
}

type dropDownPixelPainter struct{ checkPixelPainter }

func (p dropDownPixelPainter) DrawLine(a, b geometry.Point, width float32, brush graphics.Brush) {
	p.native.DrawLine(a, b, width, brush)
}

func TestDropDownSoftwarePixels(t *testing.T) {
	// Premultiplied sRGB RGBA8. Interior red at 50% alpha is 128/0/0/128;
	// allow one byte rounding. Samples avoid AA boundaries and arrows/checks.
	useProgressApplication(t, &progressTestApplication{sheet: style.Sheet(
		style.Name("drop-down").BackgroundColor(color.NRGBA{R: 255, A: 128}).BorderWidth(0).Radius(6),
		style.Name("drop-down-item").BackgroundColor(color.RGBA{B: 255, A: 255}).Radius(4),
	)})
	for _, scale := range []float32{1, 2} {
		d := NewDropDown()
		d.placeholderLabel.SetVisible(false) // This test paints the surface, not text.
		d.Arrange(geometry.Rect(0, 0, 180, 32))
		out := new(progressPixelSurface)
		painter, err := software.NewPainter(out)
		if err != nil {
			t.Fatal(err)
		}
		painter.Begin(220*scale, 80*scale, scale)
		painter.Clear(graphics.Color{})
		painter.SetTransform(geometry.Translate(10, 8))
		painter.SetClipRect(geometry.Rect(10, 8, 90, 32))
		d.Paint(dropDownPixelPainter{checkPixelPainter{native: painter}})
		painter.End()
		pixel := out.pixels.RGBAAt(30*int(scale), 24*int(scale))
		if math.Abs(float64(pixel.R)-128) > 1 || math.Abs(float64(pixel.A)-128) > 1 || pixel.G != 0 || pixel.B != 0 {
			t.Fatalf("premultiplied pixel %v", pixel)
		}
		if out.pixels.RGBAAt(110*int(scale), 24*int(scale)).A != 0 {
			t.Fatal("clip exceeded")
		}
		painter.Destroy()
	}
}
