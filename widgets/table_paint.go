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

func tableGridStyle(part string) style.Style {
	return gui.ResolveStyle("table-grid", part, style.Normal)
}

func (v *TableView) styleName() string {
	if name := v.StyleName(); name != "" {
		return name
	}
	return "table-view"
}

func (v *TableView) borderWidths() (horizontal, vertical float32) {
	name := v.styleName()
	return tableLineWidth(gui.ResolveStyle(name, "outer-horizontal", style.Normal)),
		tableLineWidth(gui.ResolveStyle(name, "outer-vertical", style.Normal))
}

// Each interior boundary belongs to the preceding column/row. The last column
// and record have no interior separator; the outer frame belongs to the view.
func (v *TableView) columnLineWidth(c *TableColumn) float32 {
	if len(v.columns) == 0 || v.columns[len(v.columns)-1] == c {
		return 0
	}
	return min(c.width, tableLineWidth(tableGridStyle("vertical")))
}

func (r *tableRow) lineHeight() float32 {
	if r.bound.ID == "" || r.bound.Index >= len(r.view.rowIDs)-1 {
		return 0
	}
	return tableLineWidth(tableGridStyle("horizontal"))
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
	vertical := tableGridStyle("vertical")
	for i, c := range r.view.columns {
		w := r.view.columnLineWidth(c)
		// Stop above the horizontal band: translucent intersections must not
		// be painted twice. Content uses the same non-overlapping geometry.
		tablePaintLine(p, geometry.Rect(r.view.columnX[i]+c.width-w, 0, w, max(0, rect.Height-h)), vertical)
	}
	tablePaintLine(p, geometry.Rect(0, rect.Height-h, min(rect.Width, r.view.columnWidth), h), tableGridStyle("horizontal"))
}

func tableHeaderLineWidth() float32 {
	return tableLineWidth(gui.ResolveStyle("table-header", "separator", style.Normal))
}
