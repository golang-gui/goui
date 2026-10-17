package gui

import (
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
)

// StyleScope explicitly selects a complete stylesheet for its subtree. It does
// not inherit individual visual properties or transfer styles to native popups.
// A nil sheet delegates to the enclosing scope or application's sheet.
// Sheets are immutable after installation; replace one to invalidate resources.
type StyleScope struct {
	WidgetBase
	child Widget
	sheet style.StyleSheet
}

func NewStyleScope() *StyleScope {
	s := new(StyleScope)
	s.SetLayoutManager(layout.NewFillLayout())
	return s
}
func (s *StyleScope) Child() Widget {
	if s.child != nil && s.child.Parent() == s && !s.child.base().destroyed {
		return s.child
	}
	return nil
}
func (s *StyleScope) SetChild(child Widget) {
	if s.destroyed || child != nil && (child.base().destroyed || s.isDescendant(s, child)) || s.child == child && (child == nil || child.Parent() == s) {
		return
	}
	s.RemoveChild(s.Child())
	s.child = child
	if child != nil {
		child.base().setParent(child, s)
	}
	s.RequestLayout()
}
func (s *StyleScope) StyleSheet() style.StyleSheet { return s.sheet }

// SetStyleSheet also invalidates nested scopes. Notifications are coalesced by
// the existing StyleChanged mechanism; no callbacks run from this setter.
func (s *StyleScope) SetStyleSheet(sheet style.StyleSheet) {
	if s.destroyed {
		return
	}
	s.sheet = sheet
	invalidateStyleSubtree(s)
	s.RequestLayout()
}
func (s *StyleScope) Snapshot() WidgetInfo {
	info := s.WidgetBase.Snapshot()
	info.Role = RoleBox
	return info
}

// ResolveWidgetStyle resolves the widget's semantic name, part and state using
// the nearest explicit StyleScope, otherwise ResolveStyle. Custom widgets should
// use this entry point; ResolveStyle intentionally remains application-wide.
func ResolveWidgetStyle(widget Widget, name, part string, state style.State) style.Style {
	for w := widget; w != nil; w = w.Parent() {
		if scope, ok := w.(*StyleScope); ok && scope.sheet != nil {
			return scope.sheet.Resolve(style.Sel{Name: name, Part: part, State: state})
		}
	}
	return ResolveStyle(name, part, state)
}

var _ Bin = (*StyleScope)(nil)
