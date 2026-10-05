package widgets

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
)

const (
	tableMinHeight float32 = 32
	tablePaddingX  float32 = 8
	tablePaddingY  float32 = 4
)

type tableLayout struct{ view *TableView }

func (l *tableLayout) Measure(children []layout.Child, c layout.Constraint) layout.Measurement {
	h, w := l.view.borderWidths()
	width, height := max(0, c.Max.Width-2*w), max(0, c.Max.Height-2*h)
	header := children[0].Measure(layout.Loose(geometry.Size{Width: width, Height: layout.Inf}))
	body := children[1].Measure(layout.Loose(geometry.Size{Width: width, Height: max(0, height-header.Height)}))
	// The header's summed column width is scrollable content, not a minimum
	// viewport width. Use the same viewport policy as the internal ScrollView.
	return layout.Measured(c.Clamp(geometry.Size{Width: body.Width + 2*w, Height: header.Height + body.Height + 2*h}))
}
func (l *tableLayout) Arrange(children []layout.Child, rect geometry.Rectangle) {
	v := l.view
	h, w := v.borderWidths()
	h, w = min(h, rect.Height/2), min(w, rect.Width/2)
	width, height := max(0, rect.Width-2*w), max(0, rect.Height-2*h)
	header := children[0].Measure(layout.Loose(geometry.Size{Width: width, Height: layout.Inf}))
	headerHeight := min(height, header.Height)
	v.headerViewport.Arrange(geometry.Rect(w, h, width, headerHeight))
	children[1].Measure(layout.Tight(geometry.Size{Width: width, Height: max(0, height-headerHeight)}))
	children[1].Arrange(geometry.Rect(w, h+headerHeight, width, max(0, height-headerHeight)))
}

type tableHeaderLayout struct{ view *TableView }

func (l *tableHeaderLayout) Measure(children []layout.Child, c layout.Constraint) layout.Measurement {
	height := tableMinHeight
	for i, child := range children {
		m := child.Measure(layout.Loose(geometry.Size{Width: l.view.columns[i].width, Height: layout.Inf}))
		height = max(height, m.Height)
	}
	return layout.Measured(c.Clamp(geometry.Size{Width: l.view.columnWidth, Height: height}))
}
func (l *tableHeaderLayout) Arrange(children []layout.Child, rect geometry.Rectangle) {
	for i, child := range children {
		size := geometry.Size{Width: l.view.columns[i].width, Height: rect.Height}
		child.Measure(layout.Tight(size))
		child.Arrange(geometry.Rectangle{Pos: geometry.Point{X: l.view.columnX[i]}, Size: size})
	}
}

type tableBody struct {
	gui.WidgetBase
	view *TableView
}

func (b *tableBody) ContentSize() geometry.Size {
	size := b.view.list.ContentSize()
	size.Width = b.view.columnWidth
	return size
}
func (b *tableBody) ConnectScrollIntoView(fn func(geometry.Rectangle)) signal.Handle {
	return b.view.list.ConnectScrollIntoView(fn)
}
func (b *tableBody) LayoutVisible(viewport geometry.Size, offset geometry.Point) {
	v := b.view
	if v.Destroyed() {
		return
	}
	headerRect := v.headerViewport.Rect()
	headerRect.Width = max(0, viewport.Width)
	v.headerViewport.Arrange(headerRect)
	v.headerRow.Arrange(geometry.Rect(-offset.X, 0, max(viewport.Width, v.columnWidth), v.headerViewport.Rect().Height))
	v.list.Arrange(geometry.Rect(0, 0, max(0, viewport.Width), max(0, viewport.Height)))
	version := v.revision
	for _, index := range v.list.VisibleIndexes() {
		if index < len(v.rowIDs) {
			if row := v.realized[v.rowIDs[index]]; row != nil && row.bound != v.rowState(index) {
				v.list.Delegate().Bind(index, row)
				if !v.valid(version) {
					return
				}
			}
		}
	}
	v.list.LayoutVisible(viewport, offset)
	if !v.valid(version) {
		return
	}
	if v.menuID != "" && v.realized[v.menuID] == nil {
		v.closeMenu()
	}
	if id := v.menuPending; id != "" && v.revealID == "" {
		if row := v.realized[id]; row != nil {
			v.menuPending = ""
			v.showMenu(id, "", row.Rect().Pos.Add(geometry.Point{Y: row.Rect().Height}))
		}
	}
}

type tableRowLayout struct{ row *tableRow }

func (l *tableRowLayout) Measure(children []layout.Child, c layout.Constraint) layout.Measurement {
	height := tableMinHeight
	for i, child := range children {
		m := child.Measure(layout.Loose(geometry.Size{Width: l.row.cells[i].column.width, Height: layout.Inf}))
		height = max(height, m.Height)
	}
	return layout.Measured(c.Clamp(geometry.Size{Width: l.row.view.columnWidth, Height: height + l.row.lineHeight()}))
}
func (l *tableRowLayout) Arrange(children []layout.Child, rect geometry.Rectangle) {
	height := max(0, rect.Height-l.row.lineHeight())
	for i, child := range children {
		size := geometry.Size{Width: l.row.cells[i].column.width, Height: height}
		child.Measure(layout.Tight(size))
		child.Arrange(geometry.Rectangle{Pos: geometry.Point{X: l.row.view.columnX[i]}, Size: size})
	}
}

// tableCellLayout measures content within its column, not the list's unbounded
// width. layout.Child keeps GUI's measurement/style cache in the normal path.
type tableCellLayout struct{ cell *tableCell }

func (l *tableCellLayout) Measure(children []layout.Child, c layout.Constraint) layout.Measurement {
	line := l.cell.row.view.columnLineWidth(l.cell.column)
	size := geometry.Size{Width: 2*tablePaddingX + line, Height: 2 * tablePaddingY}
	if len(children) > 0 {
		m := children[0].Measure(layout.Loose(geometry.Size{Width: max(0, c.Max.Width-size.Width), Height: layout.Inf}))
		size.Width += m.Width
		size.Height += m.Height
	}
	return layout.Measured(c.Clamp(size))
}
func (l *tableCellLayout) Arrange(children []layout.Child, rect geometry.Rectangle) {
	if len(children) == 0 {
		return
	}
	available := max(0, rect.Width-l.cell.row.view.columnLineWidth(l.cell.column))
	width := max(0, available-2*tablePaddingX)
	m := children[0].Measure(layout.Loose(geometry.Size{Width: width, Height: max(0, rect.Height-2*tablePaddingY)}))
	children[0].Arrange(geometry.Rect(min(tablePaddingX, available), max(0, (rect.Height-m.Height)/2), min(width, m.Width), m.Height))
}
