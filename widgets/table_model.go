package widgets

import "github.com/golang-gui/goui/gui"

// TableModel supplies records with nonempty, unique, stable model-local IDs.
// Queries are synchronous and must not emit Items. Publish complete changes
// on the GUI thread; sorting and storage belong to the application.
type TableModel interface {
	gui.ListModel
	RowID(index int) string
}

// TableData adds typed record access for declarative cell builders.
type TableData[T any] interface {
	TableModel
	ItemAt(index int) T
}

// TableRow is the snapshot of one cell binding. Index is zero-based and may
// change after sorting. Save business state by ID, never by pooled Widget.
type TableRow struct {
	ID                string
	Index             int
	Selected, Current bool
}

// TableCellDelegate creates cell content for one column. Setup returns a live,
// detached Widget (nil means empty). Bind can repeat without Unbind. Unbind
// receives the last binding, not a fresh lookup into a possibly reordered model.
// Content fills the cell after grid lines are reserved. Return a container when
// the content needs padding, alignment or more than one Widget.
type TableCellDelegate interface {
	Setup() gui.Widget
	Bind(TableRow, gui.Widget)
	Unbind(TableRow, gui.Widget)
}

// SortOrder describes the application's confirmed single-column sort state.
type SortOrder uint8

const (
	SortNone SortOrder = iota
	SortAscending
	SortDescending
)
