package gui

import "github.com/golang-gui/goui/core/geometry"

type ApplicationInfo struct {
	Windows []WindowInfo `json:"windows"`
}

type WindowInfo struct {
	ID     string             `json:"id"`
	Title  string             `json:"title"`
	Bounds geometry.Rectangle `json:"bounds"`
	Widget WidgetInfo         `json:"widget"`
}

type WidgetInfo struct {
	ID            string             `json:"id"`
	Role          Role               `json:"role"`
	Text          string             `json:"text"`
	Bounds        geometry.Rectangle `json:"bounds"`
	Visible       bool               `json:"visible"`
	Enabled       bool               `json:"enabled"`
	Focusable     bool               `json:"focusable"`
	Focused       bool               `json:"focused"`
	ContainsFocus bool               `json:"containsFocus"`
	Actions       []Action           `json:"actions"`
	Children      []WidgetInfo       `json:"children"`

	// Scroll state (omitempty: absent on non-scrolling widgets).
	ScrollY      float32 `json:"scrollY,omitempty"`      // current scroll offset
	MaxScrollY   float32 `json:"maxScrollY,omitempty"`   // scrollable range (contentH - viewportH, >= 0)
	ScrollX      float32 `json:"scrollX,omitempty"`      // horizontal scroll offset
	MaxScrollX   float32 `json:"maxScrollX,omitempty"`   // horizontal scrollable range (contentW - viewportW, >= 0)
	ItemCount    int     `json:"itemCount,omitempty"`    // ListView: total items (virtualized)
	VisibleStart int     `json:"visibleStart,omitempty"` // ListView: first visible index
	VisibleEnd   int     `json:"visibleEnd,omitempty"`   // ListView: last visible index
}

type Role string

const (
	RoleWidget     Role = "widget"
	RoleBox        Role = "box"
	RoleHBox       Role = "hbox"
	RoleVBox       Role = "vbox"
	RoleLabel      Role = "label"
	RoleButton     Role = "button"
	RoleImage      Role = "image"
	RoleTextInput  Role = "textinput"
	RoleScrollView Role = "scrollview" // scrollable container (WAI-ARIA: scrollbar host)
	RoleScrollBar  Role = "scrollbar"  // scrollbar control (WAI-ARIA: scrollbar)
	RoleList       Role = "list"       // virtualized list (WAI-ARIA: list)
	RoleListItem   Role = "listitem"   // list row (WAI-ARIA: listitem)
)

type Action string

const (
	ActionClick Action = "click"
	ActionFocus Action = "focus"
)
