package ui

// RootView is the root declarative view.
type RootView interface {
	rootView()
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
