package widgets

import "slices"

// A manual allocation survives release and ordinary relayout. A column geometry
// change or a new external layout area discards it and resumes automatic sizing.
type tableColumnResizeState struct {
	column   *TableColumn
	index    int
	widths   []float32
	before   float32
	width    float32
	viewport float32
	previous *tableColumnResizeState
}

func (v *TableView) beginColumnResize(c *TableColumn) *tableColumnResizeState {
	r := &tableColumnResizeState{column: c, index: slices.Index(v.visibleColumns, c),
		before: c.width, width: c.allocatedWidth, viewport: v.columnViewportWidth, previous: v.columnResize}
	for _, column := range v.visibleColumns {
		r.widths = append(r.widths, column.allocatedWidth)
	}
	v.columnResize = r
	return r
}

func (v *TableView) invalidateColumnResize() {
	r := v.columnResize
	if r == nil {
		return
	}
	v.columnResize = nil
	r.previous = nil
	if header := v.headers[r.column]; header != nil && header.handle.resize == r {
		header.handle.discard()
	}
}

func (r *tableColumnResizeState) allocate(v *TableView) {
	total := float64(0)
	for i, c := range v.visibleColumns {
		c.allocatedWidth = r.widths[i]
		total += float64(c.allocatedWidth)
	}
	r.column.allocatedWidth = r.width
	right := v.visibleColumns[r.index+1:]
	delta := float64(r.width) - float64(r.widths[r.index])
	viewport := float64(r.viewport)
	// Scrollbar occupancy can change inside the same external allocation. Keep
	// its adjustment on the right as well, without moving the grabbed edge.
	adjustment := delta + viewport - float64(v.columnViewportWidth)
	if delta == 0 {
		// Returning to the starting edge restores the exact original allocation.
		adjustment = 0
	}
	if adjustment > 0 {
		// Consume existing blank space, then borrow only the new growth from the
		// right. Existing overflow must not make the first motion redistribute it.
		tableShrinkColumns(right, max(0, adjustment-max(0, viewport-total)))
	} else if adjustment < 0 {
		// Reduce existing overflow before restoring compressed right-hand columns.
		remaining := max(0, -adjustment-max(0, total-viewport))
		remaining = tableGrowColumns(right, remaining, true)
		tableGrowColumns(right, remaining, false)
	}
	v.updateColumnPositions()
}

func (v *TableView) columnResizeChanged() {
	v.updateHeaders()
	v.Refresh()
}
