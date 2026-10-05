package widgets

import "github.com/golang-gui/goui/core/signal"

// TableColumn is mutable configuration owned by at most one TableView. Widths
// are DIP, including cell padding. It owns no native resources.
type TableColumn struct {
	id, title                 string
	width, minWidth, maxWidth float32
	resizable, sortable       bool
	delegate                  TableCellDelegate
	owner                     *TableView
	resize                    signal.Signal1[tableColumnResize]
}

type tableColumnResize struct {
	width    float32
	owner    *TableView
	revision uint64
}

func NewTableColumn(id, title string) *TableColumn {
	if id == "" {
		panic("widgets: empty table column ID")
	}
	return &TableColumn{id: id, title: title, width: 160, minWidth: 40, resizable: true}
}
func (c *TableColumn) ID() string    { return c.id }
func (c *TableColumn) Title() string { return c.title }
func (c *TableColumn) SetTitle(title string) {
	if c.title == title {
		return
	}
	c.title = title
	if c.owner != nil {
		c.owner.updateHeaders()
	}
}
func (c *TableColumn) Width() float32    { return c.width }
func (c *TableColumn) MinWidth() float32 { return c.minWidth }
func (c *TableColumn) MaxWidth() float32 { return c.maxWidth }
func (c *TableColumn) clamp(width float32) float32 {
	if !splitFinite(width) {
		width = 0
	}
	width = max(c.minWidth, width)
	if c.maxWidth > 0 {
		width = min(width, max(c.minWidth, c.maxWidth))
	}
	return width
}
func (c *TableColumn) SetWidth(width float32) {
	width = c.clamp(width)
	if c.width == width {
		return
	}
	c.width = width
	if c.owner != nil {
		c.owner.columnsChanged()
	}
}
func (c *TableColumn) SetMinWidth(width float32) {
	if !splitFinite(width) {
		width = 0
	}
	c.minWidth = max(0, width)
	c.SetWidth(c.width)
	if c.owner != nil {
		c.owner.updateHeaders()
	}
}
func (c *TableColumn) SetMaxWidth(width float32) {
	if !splitFinite(width) {
		width = 0
	}
	c.maxWidth = max(0, width)
	c.SetWidth(c.width)
	if c.owner != nil {
		c.owner.updateHeaders()
	}
}
func (c *TableColumn) Resizable() bool { return c.resizable }
func (c *TableColumn) SetResizable(enabled bool) {
	if c.resizable == enabled {
		return
	}
	c.resizable = enabled
	if c.owner != nil {
		c.owner.updateHeaders()
	}
}
func (c *TableColumn) Sortable() bool { return c.sortable }
func (c *TableColumn) SetSortable(enabled bool) {
	if c.sortable == enabled {
		return
	}
	c.sortable = enabled
	if c.owner != nil {
		if !enabled && c.owner.sortColumn == c.id {
			c.owner.SetSort("", SortNone)
		}
		c.owner.updateHeaders()
	}
}
func (c *TableColumn) Delegate() TableCellDelegate { return c.delegate }

// SetDelegate explicitly reinstalls cell content, including the same object.
func (c *TableColumn) SetDelegate(delegate TableCellDelegate) {
	if c.owner == nil {
		c.delegate = delegate
		return
	}
	v := c.owner
	// Existing shells capture the previous delegate until unbound.
	c.delegate = delegate
	v.reinstallRows()
}

// ConnectResize observes user resizing and cancellation rollback. SetWidth is
// silent; declarative controlled widths can write application state here.
func (c *TableColumn) ConnectResize(fn func(float32)) signal.Handle {
	return c.resize.Connect(func(event tableColumnResize) {
		if c.width == event.width && c.owner == event.owner && event.owner.valid(event.revision) {
			fn(event.width)
		}
	})
}
func (c *TableColumn) emitResize() {
	if c.owner != nil {
		c.resize.Emit(tableColumnResize{c.width, c.owner, c.owner.revision})
	}
}
