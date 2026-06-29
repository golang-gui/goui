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
	ID            string
	Role          Role
	Text          string
	Bounds        geometry.Rectangle
	Visible       bool
	Enabled       bool
	Focusable     bool
	Focused       bool
	ContainsFocus bool
	Actions       []Action
	Children      []WidgetInfo
}

type Role string

const (
	RoleWidget Role = "widget"
	RoleBox    Role = "box"
	RoleHBox   Role = "hbox"
	RoleVBox   Role = "vbox"
	RoleLabel  Role = "label"
	RoleButton Role = "button"
	RoleImage  Role = "image"
)

type Action string

const (
	ActionClick Action = "click"
	ActionFocus Action = "focus"
)
