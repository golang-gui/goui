package ui

import "github.com/golang-gui/goui/gui"

type View interface {
	Mount(ctx BuildContext) gui.Widget
	Update(ctx BuildContext, widget gui.Widget)
	Unmount(ctx BuildContext, widget gui.Widget)
}

type BuildContext interface {
	State() any
	SetState(any)
	UpdateChildren(container gui.Container, children []View)
}
