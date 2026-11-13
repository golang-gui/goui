package widgets

import (
	"math"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
)

// Line widths reserve layout space even when their color is transparent. Color
// changes therefore do not change geometry; use width zero to remove a line.
func tableLineWidth(s style.Style) float32 {
	w, _ := s.BorderWidth()
	if w <= 0 || math.IsNaN(float64(w)) || math.IsInf(float64(w), 0) {
		return 0
	}
	return w
}

func tableGridStyle(v *TableView, part string) style.Style {
	return v.partStyle("grid-"+part, style.Normal)
}

func (v *TableView) partStyle(part string, state style.State) style.Style {
	return gui.ResolveWidgetStyle(v, v.styleName(), part, state)
}

func (v *TableView) styleName() string {
	if name := v.StyleName(); name != "" {
		return name
	}
	return "table-view"
}

func (v *TableView) borderWidths() (horizontal, vertical float32) {
	name := v.styleName()
	return tableLineWidth(gui.ResolveWidgetStyle(v, name, "outer-horizontal", style.Normal)),
		tableLineWidth(gui.ResolveWidgetStyle(v, name, "outer-vertical", style.Normal))
}

// Internal separators and the trailing data outline have independent styles.
// Both reserve space inside the owning column/row before content is arranged.
func (v *TableView) columnLineStyle(c *TableColumn) style.Style {
	if len(v.visibleColumns) > 0 && v.visibleColumns[len(v.visibleColumns)-1] == c {
		return tableGridStyle(v, "right")
	}
	return tableGridStyle(v, "vertical")
}

func (v *TableView) columnLineWidth(c *TableColumn) float32 {
	if len(v.visibleColumns) == 0 {
		return 0
	}
	return min(c.allocatedWidth, tableLineWidth(v.columnLineStyle(c)))
}

func (v *TableView) columnLeadingWidth(c *TableColumn) float32 {
	if len(v.visibleColumns) == 0 || v.visibleColumns[0] != c {
		return 0
	}
	return min(max(0, c.allocatedWidth-v.columnLineWidth(c)), tableLineWidth(tableGridStyle(v, "left")))
}

func (r *tableRow) lineHeight() float32 {
	if r.bound.ID == "" {
		return 0
	}
	return tableLineWidth(r.lineStyle())
}

func (r *tableRow) lineStyle() style.Style {
	if r.bound.Index == len(r.view.rowIDs)-1 {
		return tableGridStyle(r.view, "bottom")
	}
	return tableGridStyle(r.view, "horizontal")
}

func tablePaintLine(p gui.Painter, rect geometry.Rectangle, s style.Style) {
	if rect.Width <= 0 || rect.Height <= 0 {
		return
	}
	if ink, ok := s.BorderColor(); ok && ink != nil {
		p.FillRect(rect, graphics.ColorOf(ink))
	}
}

func (r *tableRow) paintGrid(p gui.Painter, rect geometry.Rectangle) {
	h := min(rect.Height, r.lineHeight())
	if len(r.view.visibleColumns) > 0 {
		left := r.view.columnLeadingWidth(r.view.visibleColumns[0])
		tablePaintLine(p, geometry.Rect(0, 0, left, max(0, rect.Height-h)), tableGridStyle(r.view, "left"))
	}
	for i, c := range r.view.visibleColumns {
		w := r.view.columnLineWidth(c)
		// Stop above the horizontal band: translucent intersections must not
		// be painted twice. Content uses the same non-overlapping geometry.
		tablePaintLine(p, geometry.Rect(r.view.columnX[i]+c.allocatedWidth-w, 0, w, max(0, rect.Height-h)), r.view.columnLineStyle(c))
	}
	tablePaintLine(p, geometry.Rect(0, rect.Height-h, min(rect.Width, r.view.columnWidth), h), r.lineStyle())
}

func tableHeaderLineWidth(v *TableView) float32 {
	if len(v.rowIDs) == 0 {
		return tableLineWidth(tableGridStyle(v, "bottom"))
	}
	return tableLineWidth(v.partStyle("header-separator", style.Normal))
}

func tablePaintHeaderLine(p gui.Painter, rect geometry.Rectangle, v *TableView, state style.State) {
	if len(v.rowIDs) == 0 {
		tablePaintLine(p, rect, tableGridStyle(v, "bottom"))
		return
	}
	if ink, ok := v.partStyle("header-separator", state).ForegroundColor(); ok && ink != nil {
		p.FillRect(rect, graphics.ColorOf(ink))
	}
}

func (v *TableView) outlineWidths() (horizontal, vertical float32) {
	return tableLineWidth(v.partStyle("outline-horizontal", style.Normal)),
		tableLineWidth(v.partStyle("outline-vertical", style.Normal))
}

// layoutInsets reserves both independently styled frames. The overall frame
// owns the outside band; the tight outline owns the next band inside it.
func (v *TableView) layoutInsets(size geometry.Size) (horizontal, vertical float32) {
	h, w := v.borderWidths()
	oh, ow := v.outlineWidths()
	return min(size.Height/2, h+oh), min(size.Width/2, w+ow)
}

// The tight outline encloses visible data together with the fixed scroll slots.
// A visible bar spans an allocated viewport axis, so their rectangular envelope
// fills the inner allocation. With neither bar it hugs the measured data only.
func (v *TableView) paintOutline(p gui.Painter) {
	oh, ow := v.outlineWidths()
	if oh == 0 && ow == 0 || len(v.visibleColumns) == 0 {
		return
	}
	h, w := v.borderWidths()
	size := v.Rect().Size
	h, w = min(h, size.Height/2), min(w, size.Width/2)
	r := geometry.Rect(w, h, max(0, size.Width-2*w), max(0, size.Height-2*h))
	oh, ow = min(oh, r.Height/2), min(ow, r.Width/2)
	if !v.scroll.Scrollable() && v.columnWidth <= v.body.Rect().Width {
		r.Width = min(r.Width, min(v.headerViewport.Rect().Width, v.columnWidth)+2*ow)
		r.Height = min(r.Height, v.headerViewport.Rect().Height+min(v.body.contentHeight, v.body.Rect().Height)+2*oh)
	}
	tablePaintLine(p, geometry.Rect(r.X, r.Y, r.Width, oh), v.partStyle("outline-horizontal", style.Normal))
	tablePaintLine(p, geometry.Rect(r.X, r.Y+r.Height-oh, r.Width, oh), v.partStyle("outline-horizontal", style.Normal))
	tablePaintLine(p, geometry.Rect(r.X, r.Y+oh, ow, max(0, r.Height-2*oh)), v.partStyle("outline-vertical", style.Normal))
	tablePaintLine(p, geometry.Rect(r.X+r.Width-ow, r.Y+oh, ow, max(0, r.Height-2*oh)), v.partStyle("outline-vertical", style.Normal))
}

// The vertical scrollbar leaves a non-interactive gap beside the fixed header.
// Paint only that gap: existing headers own their backgrounds and cell edges.
func (v *TableView) paintHeaderScrollGap(p gui.Painter) {
	header := v.headerViewport.Rect()
	right := max(0, v.scroll.Rect().X+v.scroll.Rect().Width)
	x := min(right, max(0, header.X+header.Width))
	gap := geometry.Rect(x, header.Y, right-x, header.Height)
	if gap.Width <= 0 || gap.Height <= 0 {
		return
	}
	if bg, ok := v.partStyle("header", style.Normal).BackgroundColor(); ok && bg != nil {
		p.FillRect(gap, graphics.ColorOf(bg))
	}
	line := min(gap.Height, tableHeaderLineWidth(v))
	if line > 0 {
		tablePaintHeaderLine(p, geometry.Rect(gap.X, gap.Y+gap.Height-line, gap.Width, line), v, style.Normal)
	}
}

// The junction belongs to the fixed scroll slots, never to an extra data cell.
func (v *TableView) paintScrollCorner(p gui.Painter) {
	if !v.scroll.Scrollable() || v.columnWidth <= v.body.Rect().Width {
		return
	}
	scroll, body := v.scroll.Rect(), v.body.Rect()
	corner := geometry.Rect(scroll.X+body.Width, scroll.Y+body.Height,
		max(0, scroll.Width-body.Width), max(0, scroll.Height-body.Height))
	if ink, ok := v.partStyle("trough", style.Normal).BackgroundColor(); ok && ink != nil {
		p.FillRect(corner, graphics.ColorOf(ink))
	}
}
