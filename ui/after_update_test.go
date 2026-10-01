package ui

import (
	"testing"

	"github.com/golang-gui/goui/gui"
)

type afterUpdateProbe struct {
	ViewBase[afterUpdateProbe]
	child    View
	callback func(gui.Widget)
}

func (v *afterUpdateProbe) Build() View                   { return v }
func (v *afterUpdateProbe) Mount(BuildContext) gui.Widget { return gui.NewButton() }
func (v *afterUpdateProbe) Update(ctx BuildContext, widget gui.Widget) {
	ctx.AfterUpdate(func() { v.callback(widget) })
	ctx.UpdateChild(widget.(gui.Bin), v.child)
}
func (v *afterUpdateProbe) Unmount(BuildContext, gui.Widget) {}

func TestAfterUpdateSeesCompletedChildAndWindowMount(t *testing.T) {
	r := newRoot()
	win := newTestWindow()
	called := 0
	probe := &afterUpdateProbe{child: Label("ready").ID("child")}
	probe.Self = probe
	probe.callback = func(widget gui.Widget) {
		called++
		if win.Widget() != widget {
			t.Fatal("callback ran before SetWidget")
		}
		child := widget.(gui.Bin).Child()
		if child == nil || child.ID() != "child" {
			t.Fatalf("callback observed incomplete child: %v", child)
		}
	}
	r.updateWindow(win, probe)
	if called != 1 {
		t.Fatalf("callback count = %d", called)
	}
}

type supersedingProbe struct {
	ViewBase[supersedingProbe]
	called *int
}

func (v *supersedingProbe) Build() View                   { return v }
func (v *supersedingProbe) Mount(BuildContext) gui.Widget { return gui.NewButton() }
func (v *supersedingProbe) Update(ctx BuildContext, widget gui.Widget) {
	ctx.UpdateChild(widget.(gui.Bin), &afterUpdateProbe{callback: func(gui.Widget) { *v.called += 100 }})
	ctx.UpdateChild(widget.(gui.Bin), &afterUpdateProbe{callback: func(gui.Widget) { *v.called++ }})
}
func (v *supersedingProbe) Unmount(BuildContext, gui.Widget) {}

func TestAfterUpdateSkipsSupersededOwner(t *testing.T) {
	r := newRoot()
	count := 0
	probe := &supersedingProbe{called: &count}
	probe.Self = probe
	r.update(probe)
	if count != 1 {
		t.Fatalf("superseded callback ran: %d", count)
	}
}
