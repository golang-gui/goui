package gui

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
)

type ApplicationInfo struct {
	Windows []WindowInfo `json:"windows"`
}

type WindowInfo struct {
	ID     string             `json:"id"`
	Title  string             `json:"title"`
	Bounds geometry.Rectangle `json:"bounds"`
	Widget WidgetInfo         `json:"widget"`
	// Controls is the window-owned custom decoration tree, not an application
	// child. Native buttons keep their OS semantics and are not duplicated.
	Controls *WidgetInfo `json:"controls,omitempty"`
}

// WidgetInfo describes semantic content and state for inspection and input
// planning, not an exhaustive layout or hit-test tree. Widgets may omit internal
// structure (for example, ScrollView's viewport and scrollbars).
// Operations must still be dispatched as input events through Window.DispatchEvent.
type WidgetInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role Role   `json:"role"`
	Text string `json:"text"`
	// Bounds is the layout rectangle in window-local DIP, before clipping or
	// occlusion. It does not guarantee that a point can receive pointer input.
	Bounds geometry.Rectangle `json:"bounds"`
	// Visible is the widget's own visibility flag, not effective visibility
	// through ancestors or occlusion. Hidden nodes may remain in the snapshot.
	Visible       bool `json:"visible"`
	Enabled       bool `json:"enabled"`
	Focusable     bool `json:"focusable"`
	Focused       bool `json:"focused"`
	Selected      bool `json:"selected,omitempty"`
	ContainsFocus bool `json:"containsFocus"`
	// Actions describes supported actions, not guaranteed input reachability.
	Actions []Action `json:"actions"`
	// Children preserves back-to-front order for retained widget siblings.
	// Semantic filtering means this is not a complete pointer-picking tree.
	Children []WidgetInfo `json:"children"`
	// TextEditing is range-labelled editor state, not an input shortcut.
	TextEditing *TextEditingInfo `json:"textEditing,omitempty"`
	// DragDrop describes capabilities and transient state, never transferred data.
	DragDrop *DragDropInfo `json:"dragDrop,omitempty"`
	// Range describes an adjustable value; it is not a semantic input shortcut.
	Range *RangeInfo `json:"range,omitempty"`
	// Hierarchy describes model identity and logical sibling position without
	// materializing nodes outside a virtualized viewport.
	Hierarchy *HierarchyInfo `json:"hierarchy,omitempty"`

	// Scroll state (omitempty: absent on non-scrolling widgets).
	ScrollY      float32 `json:"scrollY,omitempty"`      // current scroll offset
	MaxScrollY   float32 `json:"maxScrollY,omitempty"`   // scrollable range (contentH - viewportH, >= 0)
	ScrollX      float32 `json:"scrollX,omitempty"`      // horizontal scroll offset
	MaxScrollX   float32 `json:"maxScrollX,omitempty"`   // horizontal scrollable range (contentW - viewportW, >= 0)
	ItemCount    int     `json:"itemCount,omitempty"`    // ListView: total items (virtualized)
	VisibleStart int     `json:"visibleStart,omitempty"` // ListView: first visible index
	VisibleEnd   int     `json:"visibleEnd,omitempty"`   // ListView: last visible index
}

// HierarchyInfo uses one-based levels and sibling positions. SetSize counts
// currently known siblings; Current is distinct from actual keyboard focus.
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

// RangeInfo uses DIP for spatial controls. Direction is the axis along which
// Value increases, not the orientation of a separator's visible line.
type RangeInfo struct {
	Value     float32          `json:"value"`
	Min       float32          `json:"min"`
	Max       float32          `json:"max"`
	Direction layout.Direction `json:"direction"`
}

type DragDropInfo struct {
	SourceActions DragAction   `json:"sourceActions,omitempty"`
	TargetActions DragAction   `json:"targetActions,omitempty"`
	TargetFormats []DragFormat `json:"targetFormats,omitempty"`
	Dragging      bool         `json:"dragging,omitempty"`
	DropActive    bool         `json:"dropActive,omitempty"`
}

// TextEditingInfo describes an editor without materializing an entire large
// document. VisibleText corresponds exactly to VisibleRange (UTF-8 bytes) and
// may be truncated; Length is the size of the complete document.
type TextEditingInfo struct {
	ReadOnly     bool          `json:"readOnly"`
	Selection    TextSelection `json:"selection"`
	Length       int           `json:"length"`
	VisibleRange TextRange     `json:"visibleRange"`
	VisibleText  string        `json:"visibleText"`
	// Preedit describes temporary, uncommitted text separately. VisibleText
	// and its range always refer to the committed model, even during IME.
	Preedit *TextPreeditInfo `json:"preedit,omitempty"`
}

type TextPreeditInfo struct {
	Replacement TextRange `json:"replacement"`
	Text        string    `json:"text"`
	Caret       int       `json:"caret"`  // UTF-8 bytes within the complete preedit text
	Length      int       `json:"length"` // Text may be a bounded prefix
}

// Role is an open semantic identifier. Widget packages may define their own
// typed constants; GUI consumers must not assume this list is exhaustive.
type Role string

const (
	RoleWidget        Role = "widget"
	RoleBox           Role = "box"
	RoleHBox          Role = "hbox"
	RoleVBox          Role = "vbox"
	RoleWrapBox       Role = "wrapbox" // ordered wrapping container, not a selectable grid
	RoleLabel         Role = "label"
	RoleButton        Role = "button"
	RoleImage         Role = "image"
	RoleTextInput     Role = "textinput"
	RoleTextView      Role = "textview"
	RoleScrollView    Role = "scrollview" // scrollable container (WAI-ARIA: scrollbar host)
	RoleScrollBar     Role = "scrollbar"  // scrollbar control (WAI-ARIA: scrollbar)
	RoleList          Role = "list"       // virtualized list (WAI-ARIA: list)
	RoleListItem      Role = "listitem"   // list row (WAI-ARIA: listitem)
	RoleMenu          Role = "menu"
	RoleMenuItem      Role = "menuitem"
	RoleMenuSeparator Role = "menuseparator"
)

type Action string

const (
	ActionClick Action = "click"
	ActionFocus Action = "focus"
)
