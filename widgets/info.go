package widgets

import "github.com/golang-gui/goui/gui"

// Semantic roles supplied by this widget package. Values remain stable across
// implementation changes; generic GUI snapshot consumers need no tab knowledge.
const (
	RoleTabView      gui.Role = "tabview"
	RoleTabBar       gui.Role = "tabbar"
	RoleTab          gui.Role = "tab"
	RoleTabPanel     gui.Role = "tabpanel"
	RoleSplitView    gui.Role = "splitview"
	RoleSeparator    gui.Role = "separator"
	RoleProgressBar  gui.Role = "progressbar"
	RoleCheckBox     gui.Role = "checkbox"
	RoleRadioButton  gui.Role = "radiobutton"
	RoleToggleButton gui.Role = "togglebutton"
)

// Snapshot attribute keys describe semantic capabilities, not concrete widget
// types. Their data is package-owned and needs no registration in gui.
const (
	TableInfoKey     = "goui.table"
	HierarchyInfoKey = "goui.hierarchy"
)

// TableInfo is stored as a value at TableInfoKey in gui.WidgetInfo.Attributes.
// It uses one-based row/column positions. Zero means the table itself or a
// header rather than a data row. IDs are model-local, not Widget IDs.
// Sort is "ascending", "descending" or empty; Current is not keyboard focus.
type TableInfo struct {
	RowCount    int    `json:"rowCount"`
	ColumnCount int    `json:"columnCount"`
	RowID       string `json:"rowID,omitempty"`
	ColumnID    string `json:"columnID,omitempty"`
	RowIndex    int    `json:"rowIndex,omitempty"`
	ColumnIndex int    `json:"columnIndex,omitempty"`
	Current     bool   `json:"current,omitempty"`
	Sort        string `json:"sort,omitempty"`
}

// HierarchyInfo is stored as a value at HierarchyInfoKey in
// gui.WidgetInfo.Attributes. It uses one-based levels and sibling positions.
// SetSize counts currently known siblings; Current is not keyboard focus.
type HierarchyInfo struct {
	NodeID        string `json:"nodeID"`
	ParentID      string `json:"parentID"`
	Level         int    `json:"level"`
	PositionInSet int    `json:"positionInSet"`
	SetSize       int    `json:"setSize"`
	Expandable    bool   `json:"expandable"`
	Expanded      bool   `json:"expanded"`
	Current       bool   `json:"current"`
}
