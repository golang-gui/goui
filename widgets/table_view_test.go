package widgets

import (
	"encoding/json"
	"fmt"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/style"
	"image"
	"image/color"
	"image/draw"
	"math"
	"slices"
	"testing"
)

// 表格逻辑、无字体测量及正常输入分发回归；不创建原生窗口。
type tableRecord struct {
	id            string
	units, height float32
}
type tableTestModel struct {
	*gui.SliceListModel[tableRecord]
	connections, deliveries int
}

func (m *tableTestModel) RowID(index int) string { return m.ItemAt(index).id }
func (m *tableTestModel) ConnectItems(fn func()) signal.Handle {
	m.connections++
	return m.SliceListModel.ConnectItems(func() { m.deliveries++; fn() })
}

type tableTestContent struct {
	gui.WidgetBase
	units, height float32
	measures      int
	ink           graphics.Color
}

func (w *tableTestContent) Paint(p gui.Painter) {
	p.FillRect(geometry.Rectangle{Size: w.Rect().Size}, w.ink)
}

func (w *tableTestContent) Measure(c layout.Constraint) layout.Measurement {
	w.measures++
	width := min(w.units, c.Max.Width)
	lines := float32(1)
	if width > 0 {
		lines = max(1, float32(math.Ceil(float64(w.units/width))))
	}
	return layout.Measured(c.Clamp(geometry.Size{Width: width, Height: lines * w.height}))
}

type tableTestDelegate struct {
	model         *tableTestModel
	setups, binds int
	unbound       []TableRow
	setup         func() gui.Widget
	onUnbind      func()
}

func (d *tableTestDelegate) Setup() gui.Widget {
	d.setups++
	if d.setup != nil {
		return d.setup()
	}
	return &tableTestContent{}
}
func (d *tableTestDelegate) Bind(row TableRow, w gui.Widget) {
	d.binds++
	if content, ok := w.(*tableTestContent); ok {
		record := d.model.ItemAt(row.Index)
		content.units, content.height = record.units, record.height
		content.RequestLayout()
	}
}
func (d *tableTestDelegate) Unbind(row TableRow, _ gui.Widget) {
	d.unbound = append(d.unbound, row)
	if d.onUnbind != nil {
		d.onUnbind()
	}
}
func tableFixture(n int) (*TableView, *tableTestModel, *TableColumn, *tableTestDelegate) {
	data := make([]tableRecord, n)
	for i := range data {
		data[i] = tableRecord{fmt.Sprintf("r%d", i), 60, 20}
	}
	m := &tableTestModel{SliceListModel: gui.NewSliceListModel(data)}
	v := NewTableView()
	c := NewTableColumn("name", "Name")
	d := &tableTestDelegate{model: m}
	c.SetDelegate(d)
	v.SetColumns(c)
	v.SetModel(m)
	return v, m, c, d
}
func tableArrange(v *TableView, width, height float32) {
	size := geometry.Size{Width: width, Height: height}
	v.Measure(layout.Tight(size))
	v.Arrange(geometry.Rectangle{Size: size})
}
func tableKey(t *testing.T, d *gui.EventDispatcher, host gui.EventTarget, key events.Key, modifiers events.Modifiers) {
	t.Helper()
	if err := d.DispatchEvent(host, events.KeyEvent{EventType: events.KeyDown, Key: key, Modifiers: modifiers}); err != nil {
		t.Fatal(err)
	}
}
func tablePanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected model/column contract panic")
		}
	}()
	fn()
}

// 边框、内容几何和斑马色；不创建桌面窗口或字体资源。
func TestTableViewCellContentFillsAllocation(t *testing.T) {
	old := gui.App
	gui.App = &separatorApp{sheet: style.Sheet(
		style.Name("table-grid").Part("horizontal").BorderWidth(5),
		style.Name("table-grid").Part("vertical").BorderWidth(7),
	)}
	t.Cleanup(func() { gui.App = old })
	v, m, column, _ := tableFixture(2)
	defer v.SetModel(nil)
	column.SetWidth(100)
	column.SetDelegate(&tableTestDelegate{model: m, setup: func() gui.Widget {
		return splitTestChild(20, 12)
	}})
	other := NewTableColumn("tall", "Tall")
	other.SetWidth(70)
	other.SetDelegate(&tableTestDelegate{model: m, setup: func() gui.Widget {
		return splitTestChild(30, 64)
	}})
	v.SetColumns(column, other)
	tableArrange(v, 220, 180)
	row := v.realized["r0"]
	if row.Rect().Height != 69 || row.cells[0].Rect().Height != 64 {
		t.Fatalf("row must use the tallest content plus one grid band: %v", row.Rect())
	}
	for i, want := range []geometry.Rectangle{geometry.Rect(0, 0, 93, 64), geometry.Rect(0, 0, 70, 64)} {
		if got := row.cells[i].content.Rect(); got != want {
			t.Fatalf("cell %d content=%v, want full allocation %v", i, got, want)
		}
	}
	// Last records do not reserve another horizontal grid band.
	if last := v.realized["r1"]; last.Rect().Height != 64 || last.cells[0].content.Rect().Height != 64 {
		t.Fatal("last record lost content height or duplicated a grid band")
	}
	column.SetMinWidth(0)
	column.SetWidth(3)
	tableArrange(v, 220, 180)
	if got := row.cells[0].content.Rect(); got != geometry.Rect(0, 0, 0, 64) {
		t.Fatalf("grid wider than the column produced an invalid content allocation: %v", got)
	}
}

func TestTableViewDecorationLayout(t *testing.T) {
	old := gui.App
	app := &separatorApp{sheet: style.Sheet(
		style.Name("table-view").BorderWidth(2).BorderColor(color.Black),
		style.Name("table-view").Part("outer-horizontal").BorderWidth(3),
		style.Name("table-view").Part("outer-vertical").BorderWidth(5),
		style.Name("table-grid").Part("horizontal").BorderWidth(6).BorderColor(color.Black),
		style.Name("table-grid").Part("vertical").BorderWidth(12).BorderColor(color.Black),
		style.Name("table-header").Part("separator").BorderWidth(3).ForegroundColor(color.Black),
	)}
	gui.App = app
	t.Cleanup(func() { gui.App = old })
	v, _, c, _ := tableFixture(2)
	c.SetWidth(80)
	other := NewTableColumn("other", "Other")
	other.SetWidth(80)
	c.SetResizable(false) // grid must not depend on the resize handle
	v.SetColumns(c, other)
	tableArrange(v, 200, 200)
	if v.headerViewport.Rect().Pos != (geometry.Point{X: 5, Y: 3}) || v.headerViewport.Rect().Height != 35 || v.scroll.Rect().X != 5 || v.scroll.Rect().Y != 38 || v.scroll.Rect().Width != 190 || v.scroll.Rect().Height != 159 {
		t.Fatalf("outer frame not reserved: header=%v scroll=%v", v.headerViewport.Rect(), v.scroll.Rect())
	}
	row := v.realized["r0"]
	content := row.cells[0].content.(*tableTestContent)
	if content.Rect() != geometry.Rect(0, 0, 68, 32) || row.Rect().Height != 38 || row.cells[0].Rect().Height != 32 {
		t.Fatalf("content squeezed by grid: content=%v row=%v cell=%v", content.Rect(), row.Rect(), row.cells[0].Rect())
	}
	if v.realized["r1"].Rect().Height != 32 || v.columnLineWidth(other) != 0 || row.cells[1].Rect().X != 80 {
		t.Fatal("last boundary duplicated or column allocation shifted")
	}
	for _, cell := range row.cells {
		h := v.headers[cell.column]
		if h.Rect().X != cell.Rect().X || h.Rect().Width != cell.Rect().Width {
			t.Fatal("header/cell allocation differs")
		}
	}
	if v.headers[c].handle.Visible() || len(v.Snapshot().Children) != 4 {
		t.Fatal("decoration changed handles or semantic child count")
	}
	t.Run("scrolled header retains outer frame offset", func(t *testing.T) {
		other.SetWidth(180)
		tableArrange(v, 200, 200)
		v.scroll.SetScrollX(32)
		tableArrange(v, 200, 200)
		if v.headerViewport.Rect().X != 5 || v.headerViewport.Rect().Y != 3 || v.headerRow.Rect().X != -32 || row.Rect().X != -32 {
			t.Fatal("horizontal synchronization discarded the frame inset")
		}
		if cell, header := row.cells[0].Snapshot().Bounds, v.headers[c].Snapshot().Bounds; cell.X != header.X || cell.Width != header.Width {
			t.Fatalf("scrolled cell/header geometry: %v %v", cell, header)
		}
	})

	t.Run("width changes remeasure but transparent color retains geometry", func(t *testing.T) {
		app.sheet = style.Sheet(style.Name("table-grid").Part("vertical").BorderWidth(12).BorderColor(color.Transparent))
		// Production SetStyleSheet uses the existing style invalidation path.
		// In this window-free fixture Refresh invalidates the bound cell layout.
		v.Refresh()
		tableArrange(v, 200, 200)
		if content.Rect().Width != 68 || row.Rect().Height != 32 {
			t.Fatal("transparent color unexpectedly released line space")
		}
		app.sheet = style.Sheet(style.Name("table-grid").Part("vertical").BorderWidth(0))
		v.Refresh()
		tableArrange(v, 200, 200)
		if content.Rect().Width != 80 || row.Rect().Height != 32 {
			t.Fatalf("grid removal retained stale measurement: %v %v", content.Rect(), row.Rect())
		}
	})
	t.Run("extreme widths and tiny viewport", func(t *testing.T) {
		app.sheet = style.Sheet(style.Name("table-view").BorderWidth(400), style.Name("table-grid").Part("vertical").BorderWidth(400))
		v.Refresh()
		tableArrange(v, 1, 1)
		if v.scroll.Rect().Width != 0 || v.scroll.Rect().Height != 0 || v.headerViewport.Rect().Pos != (geometry.Point{X: .5, Y: .5}) {
			t.Fatal("oversized frame produced negative geometry")
		}
		for _, width := range []float32{-1, float32(math.NaN()), float32(math.Inf(1))} {
			app.sheet = style.Sheet(style.Name("table-view").BorderWidth(width))
			if h, w := v.borderWidths(); h != 0 || w != 0 {
				t.Fatal("invalid line width entered geometry")
			}
		}
	})
}

type tablePixelSurface struct{ pixels *image.RGBA }

func (*tablePixelSurface) Transparent() bool { return true }
func (s *tablePixelSurface) Draw(img image.Image) error {
	s.pixels = image.NewRGBA(img.Bounds())
	draw.Draw(s.pixels, s.pixels.Bounds(), img, img.Bounds().Min, draw.Src)
	return nil
}

// Only adapt the operation used by these Paint methods, not GUI traversal.
type tablePixelPainter struct {
	gui.Painter
	native graphics.Painter
}

func (p *tablePixelPainter) FillRect(r geometry.Rectangle, ink graphics.Brush) {
	p.native.FillRect(r, ink)
}
func tablePixels(t *testing.T, size geometry.Size, scale float32, paint func(*tablePixelPainter)) *image.RGBA {
	t.Helper()
	surface := new(tablePixelSurface)
	native, err := software.NewPainter(surface)
	if err != nil {
		t.Fatal(err)
	}
	defer native.Destroy()
	native.Begin(size.Width*scale, size.Height*scale, scale)
	native.Clear(graphics.Color{})
	func() {
		defer native.End()
		paint(&tablePixelPainter{native: native})
	}()
	return surface.pixels
}

func TestTableViewDecorationPixels(t *testing.T) {
	old := gui.App
	app := &separatorApp{}
	gui.App = app
	t.Cleanup(func() { gui.App = old })
	for _, scale := range []float32{1, 2} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			app.sheet = style.Sheet(
				style.Name("table-row").BackgroundColor(color.Transparent),
				style.Name("table-grid").Part("vertical").BorderWidth(12).BorderColor(color.NRGBA{R: 255, A: 128}),
				style.Name("table-grid").Part("horizontal").BorderWidth(6).BorderColor(color.NRGBA{R: 255, A: 128}),
			)
			v, _, c, _ := tableFixture(2)
			c.SetWidth(80)
			other := NewTableColumn("other", "Other")
			other.SetWidth(80)
			v.SetColumns(c, other)
			tableArrange(v, 200, 200)
			row := v.realized["r0"]
			content := row.cells[0].content.(*tableTestContent)
			content.ink = graphics.ColorOf(color.RGBA{G: 255, A: 255})
			pixels := tablePixels(t, row.Rect().Size, scale, func(p *tablePixelPainter) {
				row.Paint(p)
				p.native.SetTransform(geometry.Translate(content.Rect().X, content.Rect().Y))
				content.Paint(p) // fills every allocated content pixel, drawn after the lines
			})
			// Integer DIP bands have full physical-pixel coverage at 1x/2x.
			// Compare premultiplied RGBA, with 1 channel step for 8-bit rounding.
			for _, sample := range []struct {
				x, y int
				want color.RGBA
			}{{69, 20, color.RGBA{R: 128, A: 128}}, {79, 37, color.RGBA{R: 128, A: 128}}, {100, 33, color.RGBA{R: 128, A: 128}}, {0, 0, color.RGBA{G: 255, A: 255}}, {67, 31, color.RGBA{G: 255, A: 255}}, {67, 20, color.RGBA{G: 255, A: 255}}, {159, 20, color.RGBA{}}} {
				got := pixels.RGBAAt(sample.x*int(scale), sample.y*int(scale))
				for i, channel := range []uint8{got.R, got.G, got.B, got.A} {
					want := []uint8{sample.want.R, sample.want.G, sample.want.B, sample.want.A}[i]
					if abs := math.Abs(float64(channel) - float64(want)); abs > 1 {
						t.Fatalf("sample (%d,%d): %v want %v", sample.x, sample.y, got, sample.want)
					}
				}
			}
			// Outer corner belongs to the horizontal side exactly once.
			app.sheet = style.Sheet(style.Name("table-view").BackgroundColor(color.Transparent).BorderWidth(4).BorderColor(color.NRGBA{R: 255, A: 128}))
			pixels = tablePixels(t, v.Rect().Size, scale, func(p *tablePixelPainter) { v.Paint(p) })
			if got := pixels.RGBAAt(int(scale), int(scale)); got != (color.RGBA{R: 128, A: 128}) {
				t.Fatalf("outer corner painted more than once: %v", got)
			}
			// Header vertical lines remain when the column is not resizable.
			app.sheet = style.Sheet(style.Name("table-grid").Part("vertical").BorderWidth(12).BorderColor(color.Black))
			c.SetResizable(false)
			h := v.headers[c]
			pixels = tablePixels(t, h.Rect().Size, scale, func(p *tablePixelPainter) { h.Paint(p) })
			if got := pixels.RGBAAt(79*int(scale), 4*int(scale)); got != (color.RGBA{A: 255}) {
				t.Fatalf("nonresizable header lost its grid: %v", got)
			}
		})
	}
}

func TestTableViewAlternateRowsAndStatePriority(t *testing.T) {
	old := gui.App
	app := &separatorApp{sheet: style.Sheet(
		style.Name("table-row").BackgroundColor(color.White),
		style.Name("table-row").Part("alternate").BackgroundColor(color.RGBA{R: 220, G: 220, B: 220, A: 255}),
		style.Name("table-row").State(style.Hovered).BackgroundColor(color.RGBA{B: 255, A: 255}),
		style.Name("table-row").State(style.Pressed).BackgroundColor(color.RGBA{G: 255, A: 255}),
		style.Name("table-row").Part("selected").BackgroundColor(color.RGBA{R: 255, A: 255}),
	)}
	gui.App = app
	t.Cleanup(func() { gui.App = old })
	v, m, _, _ := tableFixture(1000)
	tableArrange(v, 200, 160)
	check := func(row *tableRow, want color.RGBA) {
		t.Helper()
		pixels := tablePixels(t, row.Rect().Size, 1, func(p *tablePixelPainter) { row.Paint(p) })
		if got := pixels.RGBAAt(0, 10); got != want {
			t.Fatalf("row=%s index=%d background=%v want=%v", row.bound.ID, row.bound.Index, got, want)
		}
	}
	check(v.realized["r0"], color.RGBA{R: 255, G: 255, B: 255, A: 255})
	row := v.realized["r1"]
	check(row, color.RGBA{R: 220, G: 220, B: 220, A: 255})
	row.hover = true
	check(row, color.RGBA{B: 255, A: 255})
	row.pressed = true
	check(row, color.RGBA{G: 255, A: 255})
	v.SetSelection([]string{"r1"})
	check(row, color.RGBA{R: 255, A: 255})
	v.SetSelection(nil)
	row.hover, row.pressed = false, false
	m.Modify(func(rows []tableRecord) []tableRecord { rows[0], rows[1] = rows[1], rows[0]; return rows })
	tableArrange(v, 200, 160)
	check(v.realized["r1"], color.RGBA{R: 255, G: 255, B: 255, A: 255})
	v.Reveal("r999")
	tableArrange(v, 200, 160)
	check(v.realized["r999"], color.RGBA{R: 220, G: 220, B: 220, A: 255})
}
func TestTableViewColumnsAndIdentity(t *testing.T) {
	t.Run("configuration and ownership", func(t *testing.T) {
		v, _, c, _ := tableFixture(3)
		if c.Width() != 160 || c.MinWidth() != 40 || c.MaxWidth() != 0 || !c.Resizable() || c.Sortable() || !v.Focusable() || v.MainWeight() != 1 {
			t.Fatal("wrong defaults")
		}
		calls := 0
		c.ConnectResize(func(float32) { calls++ })
		c.SetWidth(-1)
		if c.Width() != 40 {
			t.Fatal("minimum not honored")
		}
		c.SetMinWidth(80)
		c.SetMaxWidth(60)
		c.SetWidth(100)
		if c.Width() != 80 {
			t.Fatal("inverted range not normalized")
		}
		c.SetMinWidth(float32(math.NaN()))
		c.SetMaxWidth(float32(math.Inf(1)))
		c.SetWidth(120)
		if c.Width() != 120 || calls != 0 {
			t.Fatal("programmatic width emitted user Resize")
		}
		copy := v.Columns()
		copy[0] = nil
		if v.Columns()[0] != c {
			t.Fatal("Columns leaked backing slice")
		}
		other := NewTableView()
		tablePanic(t, func() { other.SetColumns(c) })
		tablePanic(t, func() { v.SetColumns(c, NewTableColumn(c.ID(), "duplicate")) })
		if c.owner != v || len(v.Columns()) != 1 {
			t.Fatal("invalid collection partially mutated ownership")
		}
		v.SetColumns()
		other.SetColumns(c)
		if c.owner != other {
			t.Fatal("removed column not reusable")
		}
	})
	t.Run("invalid model IDs", func(t *testing.T) {
		for _, ids := range [][]string{{""}, {"x", "x"}} {
			tablePanic(t, func() {
				v, m, _, _ := tableFixture(0)
				data := make([]tableRecord, len(ids))
				for i, id := range ids {
					data[i].id = id
				}
				m.SetItems(data)
				v.SetModel(m)
			})
		}
	})
	t.Run("zero columns and empty model", func(t *testing.T) {
		v, m, c, d := tableFixture(0)
		c.SetWidth(500)
		tableArrange(v, 200, 150)
		if v.scroll.Snapshot().MaxScrollX <= 0 || d.setups != 0 {
			t.Fatal("empty model lost column extent or realized rows")
		}
		m.Append(tableRecord{"one", 60, 20})
		v.SetColumns()
		tableArrange(v, 200, 150)
		if len(v.rowIDs) != 1 || len(v.realized) != 0 {
			t.Fatal("zero columns lost data or realized records")
		}
	})
}
func TestTableViewStateModelAndSignals(t *testing.T) {
	v, m, c, _ := tableFixture(5)
	defer v.SetModel(nil)
	v.SetSelectionMode(SelectionMultiple)
	v.SetSelection([]string{"r3", "r1", "missing", "r1"})
	v.SetCurrent("r2")
	if !slices.Equal(v.Selection(), []string{"r1", "r3"}) || v.Current() != "r2" {
		t.Fatal("state normalization")
	}
	changes := 0
	v.ConnectSelection(func(ids []string) {
		changes++
		if len(ids) > 0 {
			ids[0] = "mutated"
		}
	})
	v.ConnectSelection(func(ids []string) {
		if len(ids) > 0 && ids[0] == "mutated" {
			t.Fatal("signal receivers share a slice")
		}
	})
	m.Modify(func(rows []tableRecord) []tableRecord { slices.Reverse(rows); return rows })
	if changes != 0 || v.Current() != "r2" || !slices.Equal(v.Selection(), []string{"r3", "r1"}) {
		t.Fatal("reorder changed logical selection")
	}
	v.SetCurrent("r3")
	m.Remove(1) // r3 at index 1; next record r2
	if v.Current() != "r2" || !slices.Equal(v.Selection(), []string{"r2", "r1"}) {
		t.Fatalf("delete normalization: %s %v", v.Current(), v.Selection())
	}
	v.SetSelection(nil)
	v.SetCurrent("r0")
	m.Remove(m.ItemsCount() - 1)
	if len(v.Selection()) != 0 || v.Current() != "r1" {
		t.Fatal("deleting unselected Current introduced selection")
	}
	c.SetSortable(true)
	v.SetSort(c.ID(), SortDescending)
	tableArrange(v, 200, 120)
	v.scroll.SetScrollY(20)
	v.SetModel(m)
	if m.connections != 2 || v.Current() != "" || len(v.Selection()) != 0 || v.sortOrder != SortNone || v.scroll.ScrollY() != 0 {
		t.Fatal("same model installation did not reset state")
	}
	m.Append(tableRecord{"new", 60, 20})
	if m.deliveries != 4 {
		t.Fatalf("subscription leak: %d", m.deliveries)
	}
	v.SetSelectionMode(SelectionSingle)
	v.SetSelection([]string{"new", "r1"})
	if !slices.Equal(v.Selection(), []string{"new"}) {
		t.Fatal("single selection input order")
	}
	other := NewTableView()
	other.SetModel(m)
	defer other.SetModel(nil)
	if len(other.Selection()) != 0 {
		t.Fatal("shared model shared view state")
	}
}
func TestTableViewLayoutVirtualizationAndRefresh(t *testing.T) {
	v, m, c, d := tableFixture(10000)
	defer v.SetModel(nil)
	second := NewTableColumn("other", "Other")
	second.SetWidth(120)
	v.SetColumns(c, second)
	m.Set(0, tableRecord{"r0", 360, 20})
	tableArrange(v, 240, 180)
	row := v.realized["r0"]
	if row.Rect().Height != 60 || row.cells[0].Rect().Width != 160 || row.cells[1].Rect().X != 160 {
		t.Fatalf("fixed column wrapping: row=%v cells=%v", row.Rect(), row.cells[0].Rect())
	}
	preferred := v.Measure(layout.Unbounded())
	if len(v.realized) > 8 || d.setups > 8 || preferred.Height > 1000 || preferred.Width != 0 {
		t.Fatal("table measured/realized all records")
	}
	v.scroll.SetScrollX(32)
	v.scroll.SetScrollY(1500)
	if v.headerRow.Rect().X != -32 || v.headerViewport.Rect().Y != 0 || v.headerViewport.Rect().Width != v.body.Rect().Width {
		t.Fatal("header not synchronized to actual body viewport")
	}
	for _, r := range v.realized {
		if r.Rect().X != -32 || r.cells[1].Rect().X != 160 {
			t.Fatal("header/body column geometry diverged")
		}
	}
	for i := 0; i < 100; i++ {
		v.scroll.SetScrollY(float32(i * 1000))
	}
	if d.setups > 20 {
		t.Fatalf("unbounded row pool: setups=%d", d.setups)
	}
	v.scroll.SetScrollY(0)
	tableArrange(v, 240, 180)
	row = v.realized["r0"]
	shell := row.cells[0].content
	c.SetWidth(80)
	tableArrange(v, 240, 180)
	if row.cells[0].content != shell || row.Rect().Height != 100 || row.cells[1].Rect().X != 80 {
		t.Fatalf("resize lost content or cached old height: %v", row.Rect())
	}
	content := shell.(*tableTestContent)
	content.height = 30
	content.RequestLayout()
	tableArrange(v, 240, 180)
	if row.Rect().Height != 150 {
		t.Fatalf("local layout invalidation ignored: %v", row.Rect())
	}
	v.Refresh()
	tableArrange(v, 240, 180)
	if row.Rect().Height != 100 {
		t.Fatal("Refresh did not rebind content")
	}
	bindings := d.binds
	tableArrange(v, 240, 180)
	if d.binds != bindings {
		t.Fatal("ordinary layout rebinds every row")
	}
	v.SetColumns(second, c)
	tableArrange(v, 240, 180)
	if row.cells[1].content != shell || row.cells[1].Rect().X != 120 {
		t.Fatal("column reorder recreated retained content")
	}
}
func TestTableViewDelegateLifecycleAndFocus(t *testing.T) {
	v, m, c, d := tableFixture(50)
	host := gui.NewPopover(nil, nil)
	host.SetWidget(v)
	defer host.Destroy()
	tableArrange(v, 240, 160)
	old := len(v.realized)
	replacement := &tableTestDelegate{model: m}
	c.SetDelegate(replacement)
	if len(d.unbound) != old || len(replacement.unbound) != 0 {
		t.Fatal("replacement unbound with new delegate")
	}
	tableArrange(v, 240, 160)
	before := replacement.setups
	c.SetDelegate(replacement)
	tableArrange(v, 240, 160)
	if replacement.setups <= before {
		t.Fatal("explicit same delegate reinstall retained its shells")
	}
	row := v.realized["r0"]
	content := row.cells[0].content.(*tableTestContent)
	content.SetFocusable(true)
	focusHost := host.(gui.EventTarget)
	focusHost.SetFocusedWidget(content)
	v.scroll.SetScrollY(1000)
	if focusHost.FocusedWidget() != v {
		t.Fatal("recycled cell carried focus to another record")
	}
	count := len(replacement.unbound)
	host.SetWidget(nil)
	if len(replacement.unbound) <= count {
		t.Fatal("unmount failed to unbind visible cells")
	}
	count = len(replacement.unbound)
	host.SetWidget(v)
	tableArrange(v, 240, 160)
	if len(replacement.unbound) != count {
		t.Fatal("remount duplicated Unbind")
	}
	count = len(replacement.unbound)
	visible := len(v.list.VisibleIndexes())
	v.SetColumns()
	if len(replacement.unbound)-count != visible {
		t.Fatal("column removal failed to unbind each cell exactly once")
	}
}
func TestTableViewInputSortResizeAndContextMenu(t *testing.T) {
	v, _, c, _ := tableFixture(8)
	defer v.SetModel(nil)
	v.SetSelectionMode(SelectionMultiple)
	c.SetSortable(true)
	tableArrange(v, 300, 180)
	host, d := &tabInputHost{root: v}, new(gui.EventDispatcher)
	click := func(y float32, mods events.Modifiers) {
		for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
			if err := d.DispatchEvent(host, events.PointerEvent{EventType: kind, Position: geometry.Point{X: 30, Y: y}, Button: events.PointerButtonLeft, Modifiers: mods}); err != nil {
				t.Fatal(err)
			}
		}
	}
	primary, _ := gui.ModPrimary.Resolve()
	click(48, 0)
	click(80, primary)
	click(112, events.ModifierShift)
	if !slices.Equal(v.Selection(), []string{"r1", "r2"}) || v.Current() != "r2" {
		t.Fatalf("click/range: %v %s", v.Selection(), v.Current())
	}
	host.focus = v
	tableKey(t, d, host, events.KeyArrowDown, primary)
	if v.Current() != "r3" || !slices.Equal(v.Selection(), []string{"r1", "r2"}) {
		t.Fatal("Primary navigation changed selection")
	}
	tableKey(t, d, host, events.KeyA, primary)
	if len(v.Selection()) != 8 {
		t.Fatal("Primary+A")
	}
	activated := ""
	v.ConnectActivate(func(id string) { activated = id })
	tableKey(t, d, host, events.KeyEnter, 0)
	if activated != "r3" {
		t.Fatal("Enter activate")
	}
	sorted := 0
	v.ConnectSortRequest(func(id string, order SortOrder) {
		sorted++
		if id != "name" || order != SortAscending {
			t.Fatal("wrong sort request")
		}
	})
	dispatchTabPointer(t, d, host, events.PointerDown, 30, 16)
	dispatchTabPointer(t, d, host, events.PointerUp, 30, 16)
	if sorted != 1 || v.sortOrder != SortNone || len(v.Selection()) != 8 {
		t.Fatal("header mutates sort/selection before confirmation")
	}
	widths := []float32{}
	c.ConnectResize(func(width float32) { widths = append(widths, width) })
	dispatchTabPointer(t, d, host, events.PointerDown, 156, 16)
	dispatchTabPointer(t, d, host, events.PointerMove, 196, 16)
	if c.Width() != 200 || sorted != 1 {
		t.Fatalf("resize competed with sort: width=%g sort=%d", c.Width(), sorted)
	}
	tableArrange(v, 300, 180)
	tableKey(t, d, host, events.KeyEscape, 0)
	if c.Width() != 160 || !slices.Equal(widths, []float32{200, 160}) {
		t.Fatalf("resize cancellation: %g %v", c.Width(), widths)
	}
	dispatchTabPointer(t, d, host, events.PointerUp, 196, 16)
	queryRow, queryCol := "", ""
	queries := 0
	v.ConnectContextMenu(func(row, col string, _ *gui.MenuModel) { queryRow, queryCol = row, col; queries++ })
	tableArrange(v, 300, 180)
	for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
		d.DispatchEvent(host, events.PointerEvent{EventType: kind, Position: geometry.Point{X: 30, Y: 48}, Button: events.PointerButtonRight})
	}
	if queries != 1 || queryRow != "r0" || queryCol != "name" || len(v.Selection()) != 8 {
		t.Fatalf("row context menu %s/%s selected=%v queries=%d", queryRow, queryCol, v.Selection(), queries)
	}
	host.focus = v
	tableKey(t, d, host, events.KeyF10, events.ModifierShift)
	tableArrange(v, 300, 180)
	if queries != 2 || queryRow != v.Current() || queryCol != "" {
		t.Fatal("keyboard menu identity")
	}
	for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
		d.DispatchEvent(host, events.PointerEvent{EventType: kind, Position: geometry.Point{X: 30, Y: 16}, Button: events.PointerButtonRight})
	}
	if queries != 2 {
		t.Fatal("header queried body menu")
	}
}
func TestTableViewInteractiveCellAndSnapshot(t *testing.T) {
	v, m, c, _ := tableFixture(20)
	defer v.SetModel(nil)
	clicks := 0
	c.SetDelegate(&tableTestDelegate{model: m, setup: func() gui.Widget {
		b := gui.NewButton()
		b.SetChild(splitTestChild(60, 20))
		b.ConnectClicked(func() { clicks++ })
		return b
	}})
	tableArrange(v, 220, 180)
	host, d := &tabInputHost{root: v}, new(gui.EventDispatcher)
	dispatchTabPointer(t, d, host, events.PointerDown, 30, 52)
	dispatchTabPointer(t, d, host, events.PointerUp, 30, 52)
	if clicks != 1 || v.Current() != "" || len(v.Selection()) != 0 {
		t.Fatal("interactive cell selected its row")
	}
	tableKey(t, d, host, events.KeyArrowDown, 0)
	if v.Current() != "" {
		t.Fatal("table stole child keyboard input")
	}
	v.SetCurrent("r0")
	v.SetSelection([]string{"r0"})
	tableArrange(v, 220, 180)
	info := v.Snapshot()
	table, ok := info.Attributes[TableInfoKey].(TableInfo)
	if info.Role != RoleTable || !ok || table.RowCount != 20 || table.ColumnCount != 1 || len(info.Children) > 8 {
		t.Fatal("table snapshot extent/virtualization")
	}
	header, ok := info.Children[0].Attributes[TableInfoKey].(TableInfo)
	if info.Children[0].Role != RoleColumnHeader || !ok || header.ColumnID != "name" {
		t.Fatal("header metadata")
	}
	row := info.Children[1]
	cell := row.Children[0]
	rowInfo, rowOK := row.Attributes[TableInfoKey].(TableInfo)
	cellInfo, cellOK := cell.Attributes[TableInfoKey].(TableInfo)
	if row.Role != RoleTableRow || !rowOK || rowInfo.RowID != "r0" || rowInfo.RowIndex != 1 || !row.Selected || !rowInfo.Current || cell.Role != RoleTableCell || !cellOK || cellInfo.ColumnIndex != 1 || cell.ID == "r0" {
		t.Fatal("row/cell semantic identity")
	}
}

func TestTableViewSnapshotAttributesIsolation(t *testing.T) {
	v, _, column, _ := tableFixture(1000)
	defer v.SetModel(nil)
	column.SetSortable(true)
	v.AddEventController(gui.NewDragSource())
	v.SetCurrent("r0")
	v.SetSelection([]string{"r0"})
	v.SetSort("name", SortAscending)
	tableArrange(v, 220, 180)
	old := v.Snapshot()
	oldTable := old.Attributes[TableInfoKey].(TableInfo)
	oldHeader := old.Children[0].Attributes[TableInfoKey].(TableInfo)
	oldRow := old.Children[1].Attributes[TableInfoKey].(TableInfo)
	oldCell := old.Children[1].Children[0].Attributes[TableInfoKey].(TableInfo)
	if old.DragDrop == nil || old.DragDrop.SourceActions != gui.DragCopy || oldHeader.Sort != "ascending" || !oldRow.Current || !oldCell.Current {
		t.Fatal("table attributes lost another capability, sort or current state")
	}
	v.SetCurrent("r1")
	v.SetSelection([]string{"r1"})
	v.SetSort("name", SortDescending)
	now := v.Snapshot()
	if now.Children[0].Attributes[TableInfoKey].(TableInfo).Sort != "descending" || now.Children[1].Attributes[TableInfoKey].(TableInfo).Current {
		t.Fatal("new snapshot did not capture changed state")
	}
	now.SetAttribute(TableInfoKey, TableInfo{})
	now.Children[0].SetAttribute(TableInfoKey, TableInfo{})
	now.Children[1].SetAttribute(TableInfoKey, TableInfo{})
	now.Children[1].Children[0].SetAttribute(TableInfoKey, TableInfo{})
	if old.Attributes[TableInfoKey].(TableInfo) != oldTable || old.Children[0].Attributes[TableInfoKey].(TableInfo) != oldHeader || old.Children[1].Attributes[TableInfoKey].(TableInfo) != oldRow || old.Children[1].Children[0].Attributes[TableInfoKey].(TableInfo) != oldCell || !old.Children[1].Selected {
		t.Fatal("previous table snapshot shares mutable state with a later snapshot")
	}
	data, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if _, legacy := fields["table"]; legacy {
		t.Fatal("snapshot still emits the retired table field")
	}
	var attributes map[string]json.RawMessage
	if err := json.Unmarshal(fields["attributes"], &attributes); err != nil {
		t.Fatal(err)
	}
	var decoded TableInfo
	if err := json.Unmarshal(attributes[TableInfoKey], &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded != (TableInfo{RowCount: 1000, ColumnCount: 1}) || len(old.Children) > 8 {
		t.Fatalf("JSON changed the table schema or virtualized children: %+v / %d", decoded, len(old.Children))
	}
}
func TestTableViewRevealAndReentrantCallbacks(t *testing.T) {
	v, m, _, _ := tableFixture(1000)
	defer v.SetModel(nil)
	tableArrange(v, 220, 180)
	v.Reveal("r999")
	for i := 0; i < 12; i++ {
		tableArrange(v, 220, 180)
	}
	if v.realized["r999"] == nil || v.revealID != "" || v.scroll.ScrollY() == 0 {
		t.Fatal("Reveal did not converge")
	}
	v.Reveal("r500")
	m.Modify(func(rows []tableRecord) []tableRecord { slices.Reverse(rows); return rows })
	for i := 0; i < 12; i++ {
		tableArrange(v, 220, 180)
	}
	if v.realized["r500"] == nil {
		t.Fatal("model reorder lost pending ID reveal")
	}
	calls := 0
	v.ConnectSelection(func([]string) { v.SetModel(nil) })
	v.ConnectSelection(func([]string) { calls++ })
	v.SetSelection([]string{"r1"})
	if calls != 0 || v.Model() != nil {
		t.Fatal("stale signal receiver after model replacement")
	}
}

func TestTableViewBalancedColumnWidthsAndNarrowViewport(t *testing.T) {
	v, m, c, _ := tableFixture(100)
	defer v.SetModel(nil)
	other := NewTableColumn("other", "Other")
	other.SetWidth(120)
	v.SetColumns(c, other)
	m.Set(0, tableRecord{"r0", 360, 20})
	tableArrange(v, 280, 180)
	if v.realized["r0"].Rect().Height != 60 {
		t.Fatal("initial wrap")
	}
	c.SetWidth(100)
	other.SetWidth(180)
	tableArrange(v, 280, 180)
	if v.columnWidth != 280 || v.realized["r0"].Rect().Height != 80 || v.realized["r0"].cells[1].Rect().X != 100 {
		t.Fatal("same total width concealed per-column wrapping change")
	}
	v.scroll.SetScrollY(1500)
	c.SetWidth(80)
	other.SetWidth(200)
	tableArrange(v, 280, 180)
	v.scroll.SetScrollY(0)
	tableArrange(v, 280, 180)
	if v.realized["r0"].Rect().Height != 100 {
		t.Fatal("offscreen row retained old column measurements")
	}
	tableArrange(v, 1, 1)
	tableArrange(v, 280, 180)
	if len(v.realized) == 0 {
		t.Fatal("narrow viewport did not recover")
	}
}
func TestTableViewColumnCallbacksCanChangeModelAndOwnership(t *testing.T) {
	t.Run("cancellation cannot partially remove columns", func(t *testing.T) {
		v, m, c, _ := tableFixture(10)
		defer v.SetModel(nil)
		other := NewTableColumn("other", "Other")
		v.SetColumns(c, other)
		tableArrange(v, 400, 180)
		host, d := &tabInputHost{root: v}, new(gui.EventDispatcher)
		dispatchTabPointer(t, d, host, events.PointerDown, 316, 16)
		dispatchTabPointer(t, d, host, events.PointerMove, 356, 16)
		other.ConnectResize(func(float32) { v.SetModel(m) })
		v.SetColumns()
		if len(v.Columns()) != 2 || c.owner != v || v.headers[c] == nil {
			t.Fatal("reentrant cancellation partially removed column ownership")
		}
		tableArrange(v, 400, 180)
		v.Snapshot()
	})
	t.Run("Unbind replaces model", func(t *testing.T) {
		v, m, c, d := tableFixture(10)
		defer v.SetModel(nil)
		other := NewTableColumn("other", "Other")
		v.SetColumns(c, other)
		tableArrange(v, 300, 180)
		d.onUnbind = func() { d.onUnbind = nil; v.SetModel(m) }
		v.SetColumns(other)
		tableArrange(v, 300, 180)
		for _, r := range v.realized {
			if len(r.cells) != 1 || len(r.Children()) != 1 || r.cells[0].column != other {
				t.Fatal("reentrant Unbind left orphan cells")
			}
		}
	})
	t.Run("resize removes column", func(t *testing.T) {
		v, _, c, _ := tableFixture(10)
		defer v.SetModel(nil)
		tableArrange(v, 300, 180)
		stale := 0
		c.ConnectResize(func(float32) { v.SetColumns() })
		c.ConnectResize(func(float32) { stale++ })
		host, d := &tabInputHost{root: v}, new(gui.EventDispatcher)
		dispatchTabPointer(t, d, host, events.PointerDown, 156, 16)
		dispatchTabPointer(t, d, host, events.PointerMove, 196, 16)
		dispatchTabPointer(t, d, host, events.PointerUp, 196, 16)
		if stale != 0 || len(v.Columns()) != 0 {
			t.Fatal("removed column delivered stale Resize")
		}
	})
	t.Run("selection destroys host", func(t *testing.T) {
		v, _, _, _ := tableFixture(10)
		host := gui.NewPopover(nil, nil)
		host.SetWidget(v)
		tableArrange(v, 300, 180)
		stale := 0
		v.ConnectSelection(func([]string) { host.Destroy() })
		v.ConnectCurrent(func(string) { stale++ })
		v.choose("r0", false, false)
		// A Popover releases its borrowed subtree rather than destroying it;
		// Unmount must still invalidate the in-flight notification generation.
		if v.Root() != nil || stale != 0 {
			t.Fatal("closed host delivered stale Current")
		}
	})
}
