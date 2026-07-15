package ui

import "github.com/golang-gui/goui/style"

// RootView is the root declarative view.
type RootView interface {
	rootView()
}

// RootNode is the optional application-level declarative node. It carries app-wide
// declarations (currently the style sheet) plus the set of windows. Return it from
// the Run build closure when you need app-level configuration; otherwise return a
// WindowView directly.
type RootNode struct {
	styleSheet style.StyleSheet
	windows    []WindowView
}

// Root creates an application-level root node.
func Root() RootNode {
	return RootNode{}
}

func (r RootNode) rootView() {}

// StyleSheet sets the application style sheet. It is reconciled onto the running
// application on each rebuild, so swapping it re-skins the app at runtime; a nil
// sheet reverts to the built-in default.
func (r RootNode) StyleSheet(sheet style.StyleSheet) RootNode {
	r.styleSheet = sheet
	return r
}

// Windows sets the top-level windows of the application.
func (r RootNode) Windows(windows ...WindowView) RootNode {
	r.windows = windows
	return r
}

// WindowView describes one top-level window.
type WindowView struct {
	id             string
	title          string
	content        View
	onCloseRequest func(*bool)
	onDestroy      func()
}

// Window creates a top-level window view with a stable identity.
func Window(id string) WindowView {
	return WindowView{
		id:    id,
		title: id,
	}
}

func (w WindowView) rootView() {}

// ID returns the stable identity used by the window reconciler.
func (w WindowView) ID() string {
	return w.id
}

// Title sets the native window title.
func (w WindowView) Title(title string) WindowView {
	w.title = title
	return w
}

// Content sets the root widget view for the window.
func (w WindowView) Content(content View) WindowView {
	w.content = content
	return w
}

// OnCloseRequest sets the handler called before a close request is accepted.
func (w WindowView) OnCloseRequest(fn func(allow *bool)) WindowView {
	w.onCloseRequest = fn
	return w
}

// OnDestroy sets the handler called when the underlying window is destroyed.
func (w WindowView) OnDestroy(fn func()) WindowView {
	w.onDestroy = fn
	return w
}
