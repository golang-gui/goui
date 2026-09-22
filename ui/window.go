package ui

import (
	"github.com/golang-gui/goui/core/bits"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/style"
)

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
	shortcuts      []*ShortcutView
	options        gui.WindowOptions
	id             string
	title          string
	content        View
	onCloseRequest func(*bool)
	onDestroy      func()
	minWidth       float32
	minHeight      float32 // window minimum size hint (logical DIP); merged over an unbounded base on apply
	fields         bits.Bitmap[uint64]
}

type (
	WindowChromeMode = gui.WindowChromeMode
)

const (
	WindowChromeNative     = gui.WindowChromeNative
	WindowChromeIntegrated = gui.WindowChromeIntegrated
	WindowChromeNone       = gui.WindowChromeNone
)

// Window creates a top-level window view with a stable identity.
func Window(id string) WindowView {
	return WindowView{
		id:    id,
		title: id,
	}
}

func (w WindowView) rootView() {}

// Title sets the native window title.
func (w WindowView) Title(title string) WindowView {
	w.title = title
	return w
}

// Size sets the preferred initial window size in DIP. Windows None/Integrated
// use outer size; the other current desktop modes request client size. Layout
// uses the actual client size reported by the window. Size and Chrome are
// creation-only; changing them for a mounted window ID returns an error. They
// do not force the window size on ordinary declarative rebuilds.
func (w WindowView) Size(width, height float32) WindowView {
	w.options.Size = geometry.Size{Width: width, Height: height}
	return w
}

// Chrome sets a creation-time GUI decoration preference. It does not insert
// a HeaderBar. Integrated owns its controls independently of application content;
// unsupported Integrated falls back to Native.
func (w WindowView) Chrome(chrome WindowChromeMode) WindowView {
	w.options.Chrome = chrome
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

// MinWidth declares the desktop window's minimum width hint in DIP, using the
// same outer/client convention as Size. This does not replace Widget layout
// constraints or add space for a HeaderBar. Like the widget-level viewBase,
// declared axes are merged on each rebuild; an axis without a declaration
// falls back to the window default, which is no constraint at all.
func (w WindowView) MinWidth(v float32) WindowView {
	w.minWidth = v
	w.fields.Set(viewMinWidth, true)
	return w
}

// MinHeight declares the desktop window's minimum height hint in DIP. See
// MinWidth for the merge semantics.
func (w WindowView) MinHeight(v float32) WindowView {
	w.minHeight = v
	w.fields.Set(viewMinHeight, true)
	return w
}

// MinSize declares both axes of the desktop window's minimum size hint in DIP,
// setting the same bits MinWidth and MinHeight would set individually.
func (w WindowView) MinSize(width, height float32) WindowView {
	w.minWidth = width
	w.minHeight = height
	w.fields.Set(viewMinWidth, true)
	w.fields.Set(viewMinHeight, true)
	return w
}

// mergedMinSize resolves the per-axis declarations into one size. Absent axes
// fall back to zero (unbounded): a window has no private minimum to snapshot,
// so "undeclared" simply means unconstrained on that axis.
func (w WindowView) mergedMinSize() geometry.Size {
	var s geometry.Size
	if w.fields.Check(viewMinWidth) {
		s.Width = w.minWidth
	}
	if w.fields.Check(viewMinHeight) {
		s.Height = w.minHeight
	}
	return s
}
