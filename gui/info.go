package gui

import "github.com/golang-gui/goui/core/geometry"

type ApplicationInfo struct {
	Windows []WindowInfo
}

type WindowInfo struct {
	ID     string
	Title  string
	Bounds geometry.Rectangle
	Widget WidgetInfo
}

type WidgetInfo struct {
	ID       string
	Role     Role
	Text     string
	Bounds   geometry.Rectangle
	Visible  bool
	Enabled  bool
	Actions  []Action
	Children []WidgetInfo
}

type Role string

const (
	RoleWidget Role = "widget"
	RoleBox    Role = "box"
	RoleLabel  Role = "label"
	RoleButton Role = "button"
)

type Action string

const (
	ActionClick Action = "click"
	ActionFocus Action = "focus"
)
