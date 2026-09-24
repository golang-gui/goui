package ui

import (
	"testing"

	"github.com/golang-gui/goui/gui"
)

func TestDragDropDescriptorsReconcileWithoutDuplicatingControllers(t *testing.T) {
	root := newRoot()
	widget := root.update(Label("drag").DragSource(DragSource().Actions(DragCopy)).DropTarget(
		DropTarget(DragFormatText).Actions(DragCopy | DragLink),
	))
	if got := widget.Snapshot().DragDrop; got == nil || got.SourceActions != DragCopy ||
		got.TargetActions != DragCopy|DragLink || len(got.TargetFormats) != 1 || got.TargetFormats[0] != DragFormatText {
		t.Fatalf("initial drag/drop snapshot: %+v", got)
	}
	if got := len(widget.EventControllers()); got != 2 {
		t.Fatalf("initial controllers = %d, want 2", got)
	}
	updated := root.update(Label("dragged").DragSource(DragSource().Actions(DragMove)).DropTarget(
		DropTarget(DragFormatFiles, DragFormatURLs).Actions(DragCopy),
	))
	if updated != widget || len(widget.EventControllers()) != 2 {
		t.Fatal("rebuild replaced the widget or duplicated controllers")
	}
	if got := widget.Snapshot().DragDrop; got == nil || got.SourceActions != DragMove ||
		got.TargetActions != DragCopy || len(got.TargetFormats) != 2 {
		t.Fatalf("updated drag/drop snapshot: %+v", got)
	}
	root.update(Label("plain"))
	if got := widget.Snapshot().DragDrop; got != nil {
		t.Fatalf("removed descriptors still advertised: %+v", got)
	}
	for _, controller := range widget.EventControllers() {
		if _, source := controller.(*gui.DragSource); source {
			t.Fatal("source controller retained after descriptor removal")
		}
		if _, target := controller.(*gui.DropTarget); target {
			t.Fatal("target controller retained after descriptor removal")
		}
	}
}

func TestDragSourceDataPrecedesPrepareAndCanBeReplaced(t *testing.T) {
	initial := new(DragData)
	initial.SetText("initial")
	replacement := new(DragData)
	replacement.SetText("replacement")
	sawInitial := false

	for _, test := range []struct {
		name string
		view *DragSourceView
		want *DragData
	}{
		{"fixed data", DragSource().Data(initial), initial},
		{"supplement", DragSource().Data(initial).OnPrepare(func(r *DragPrepare) {
			sawInitial = r.Data == initial
		}), initial},
		{"replace", DragSource().Data(initial).OnPrepare(func(r *DragPrepare) {
			r.Data = replacement
		}), replacement},
		{"decline", DragSource().Data(initial).OnPrepare(func(r *DragPrepare) {
			r.Data = nil
		}), nil},
		{"dynamic data", DragSource().OnPrepare(func(r *DragPrepare) {
			r.Data = replacement
		}), replacement},
		{"reverse chain order", DragSource().OnPrepare(func(r *DragPrepare) {
			r.Data = replacement
		}).Data(initial), replacement},
		{"cleared data", DragSource().Data(initial).Data(nil), nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := &DragPrepare{Data: replacement}
			test.view.prepareRequest(request)
			if request.Data != test.want {
				t.Fatalf("prepared data=%p, want %p", request.Data, test.want)
			}
		})
	}
	if !sawInitial {
		t.Fatal("OnPrepare did not see declared data")
	}
}

func TestDragSourceDataReconcilesWithoutReplacingController(t *testing.T) {
	root := newRoot()
	first := new(DragData)
	first.SetText("first")
	second := new(DragData)
	second.SetText("second")
	widget := root.update(Label("source").DragSource(DragSource().Data(first)))
	binding := &root.root.baseCtx.dragSource
	controller := binding.controller
	if controller == nil || len(widget.EventControllers()) != 1 || binding.callbacks.data != first {
		t.Fatal("initial data source was not bound")
	}
	if got := root.update(Label("source").DragSource(DragSource().Data(second))); got != widget ||
		binding.controller != controller || len(widget.EventControllers()) != 1 || binding.callbacks.data != second {
		t.Fatal("data update replaced controller or retained stale data")
	}
	root.update(Label("source").DragSource(DragSource().Data(nil)))
	if binding.controller != controller || binding.callbacks.data != nil {
		t.Fatal("Data(nil) removed the source or retained old data")
	}
	root.update(Label("source"))
	if binding.controller != nil || len(widget.EventControllers()) != 0 || binding.callbacks.data != nil {
		t.Fatal("omitting source did not release the binding")
	}
}
