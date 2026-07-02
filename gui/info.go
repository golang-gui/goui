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
}

type Role string

const (
	RoleWidget    Role = "widget"
	RoleBox       Role = "box"
	RoleHBox      Role = "hbox"
	RoleVBox      Role = "vbox"
	RoleLabel     Role = "label"
	RoleButton    Role = "button"
	RoleImage     Role = "image"
	RoleTextInput Role = "textinput"
)

type Action string

const (
	ActionClick Action = "click"
	ActionFocus Action = "focus"
)
