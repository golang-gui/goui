package ui

import (
	"errors"
	"testing"

	"github.com/golang-gui/goui/gui"
)

// Ordinary nodes and mounting targets, without a display or native window.
func transferRoots() (*root, *root) {
	a, b := newRoot(), newRoot()
	runtime := &app{windows: map[string]*windowMount{"a": {root: a}, "b": {root: b}}}
	a.runtime, b.runtime = runtime, runtime
	return a, b
}

func transferView(tracker *lifecycleTracker, text string) *lifecycleView {
	v := &lifecycleView{tracker: tracker, text: text}
	v.id = "page"
	v.fields.Set(viewID, true)
	return v
}

func TestCoordinatorTransferRetainsStateAndSurvivesBothUpdateOrders(t *testing.T) {
	for _, targetFirst := range []bool{false, true} {
		a, b := transferRoots()
		tracker := new(lifecycleTracker)
		source := a.update(VBox(transferView(tracker, "before"))).(*gui.LinearBox)
		target := b.update(VBox()).(*gui.LinearBox)
		n := a.root.children[0].nodes[0]
		original, state, base := n.widget, n.state, n.baseCtx
		owner, moved := &Coordinator{owner: a.root}, &Coordinator{owner: n}
		stale := 0
		a.afterUpdate = append(a.afterUpdate, afterUpdateCall{owner: n, version: n.version, fn: func() { stale++ }})
		if err := owner.TransferChild(original, target, func() error {
			source.RemoveChild(original)
			target.AddChild(original)
			return nil
		}, nil); err != nil {
			t.Fatal(err)
		}
		a.flushAfterUpdate()
		if stale != 0 || len(a.root.children[0].nodes) != 0 || b.root.children[0].nodes[0] != n || n.root != b || n.state != state || n.baseCtx != base || moved.find(original) != n {
			t.Fatal("transfer replaced state, kept source ownership or ran a stale callback")
		}
		updateSource := func() { a.update(VBox()) }
		updateTarget := func() { b.update(VBox(transferView(tracker, "after"))) }
		if targetFirst {
			updateTarget()
			updateSource()
		} else {
			updateSource()
			updateTarget()
		}
		a.unmountWindow()
		if target.Children()[0] != original || tracker.mounts != 1 || tracker.unmounts != 0 || tracker.updates != 2 || original.(*gui.Label).Text() != "after" || owner.find(original) != nil {
			t.Fatalf("node did not survive update/source release: %+v", tracker)
		}
		b.unmountWindow()
		if tracker.unmounts != 1 || moved.find(original) != nil {
			t.Fatal("destination did not release node exactly once")
		}
	}
}

func TestCoordinatorTransferFailureAndPanicRetainRecord(t *testing.T) {
	for _, panicMove := range []bool{false, true} {
		a, b := transferRoots()
		tracker := new(lifecycleTracker)
		a.update(VBox(transferView(tracker, "held")))
		target := b.update(VBox())
		n := a.root.children[0].nodes[0]
		failure := errors.New("move failed")
		func() {
			defer func() {
				got := recover()
				if panicMove && got != failure || !panicMove && got != nil {
					t.Fatalf("panic changed: %v", got)
				}
			}()
			err := (&Coordinator{owner: a.root}).TransferChild(n.widget, target, func() error {
				if !a.transferring || !b.transferring {
					t.Fatal("roots not protected")
				}
				if panicMove {
					panic(failure)
				}
				return failure
			}, nil)
			if err != failure {
				t.Fatalf("failure = %v", err)
			}
		}()
		if a.transferring || b.transferring || a.root.children[0].nodes[0] != n || n.root != a || len(b.root.children[0].nodes) != 0 || tracker.unmounts != 0 {
			t.Fatal("failure lost ownership or left roots locked")
		}
		a.unmountWindow()
		b.unmountWindow()
	}
}

func TestCoordinatorTransferValidatesBeforeMove(t *testing.T) {
	a, b := transferRoots()
	a.update(VBox(Label("moving").ID("duplicate")))
	target := b.update(VBox(Label("existing").ID("duplicate")))
	child := a.widget().Children()[0]
	called := false
	move := func() error { called = true; return nil }
	c := &Coordinator{owner: a.root}
	var typedNil *gui.Button
	for _, test := range []struct{ child, target gui.Widget }{
		{child, target}, {child, child}, {gui.NewButton(), target},
		{child, gui.NewLabel("foreign")}, {nil, target}, {typedNil, target},
	} {
		if c.TransferChild(test.child, test.target, move, nil) == nil || called {
			t.Fatal("invalid transfer invoked move")
		}
	}
	b.update(VBox())
	b.reconciling = true
	if c.TransferChild(child, b.widget(), move, nil) == nil || called {
		t.Fatal("transfer while reconciling accepted")
	}
	b.reconciling = false
	if c.TransferChild(child, b.widget(), nil, nil) == nil {
		t.Fatal("nil move accepted")
	}
	a.unmountWindow()
	b.unmountWindow()
}

func TestCoordinatorEmptyPageIsStillAnOrdinaryNode(t *testing.T) {
	a, b := transferRoots()
	source := a.update(VBox(Button().ID("empty"))).(*gui.LinearBox)
	target := b.update(VBox()).(*gui.LinearBox)
	child := source.Children()[0]
	if err := (&Coordinator{owner: a.root}).TransferChild(child, target, func() error {
		source.RemoveChild(child)
		target.AddChild(child)
		return nil
	}, nil); err != nil {
		t.Fatal(err)
	}
	b.update(VBox(Button("created later").ID("empty")))
	if target.Children()[0] != child {
		t.Fatal("empty page had no retained node")
	}
	a.unmountWindow()
	b.unmountWindow()
}

func TestCoordinatorSameRootAndSourceReleaseDuringMove(t *testing.T) {
	for _, sameRoot := range []bool{false, true} {
		a, b := transferRoots()
		tracker := new(lifecycleTracker)
		var sourceNode, targetNode *node
		if sameRoot {
			a.update(VBox(VBox(transferView(tracker, "held")), VBox()))
			sourceNode, targetNode = a.root.children[0].nodes[0], a.root.children[0].nodes[1]
		} else {
			a.update(VBox(transferView(tracker, "held")))
			b.update(VBox())
			sourceNode, targetNode = a.root, b.root
		}
		source, target := sourceNode.widget.(*gui.LinearBox), targetNode.widget.(*gui.LinearBox)
		n := sourceNode.children[0].nodes[0]
		if err := (&Coordinator{owner: sourceNode}).TransferChild(n.widget, target, func() error {
			source.RemoveChild(n.widget)
			target.AddChild(n.widget)
			if !sameRoot {
				a.unmountWindow()
			}
			return nil
		}, nil); err != nil {
			t.Fatal(err)
		}
		if targetNode.children[0].nodes[0] != n || tracker.mounts != 1 || tracker.unmounts != 0 {
			t.Fatal("source release lost held node")
		}
		a.unmountWindow()
		b.unmountWindow()
		if tracker.unmounts != 1 {
			t.Fatal("node not released exactly once")
		}
	}
}

func TestCoordinatorReleasesHeldRecordWhenBothRootsDie(t *testing.T) {
	a, b := transferRoots()
	tracker := new(lifecycleTracker)
	a.update(VBox(transferView(tracker, "held")))
	target := b.update(VBox())
	child := a.widget().Children()[0]
	if err := (&Coordinator{owner: a.root}).TransferChild(child, target, func() error {
		a.unmountWindow()
		b.unmountWindow()
		return errors.New("both hosts destroyed")
	}, nil); err == nil {
		t.Fatal("destruction reported success")
	}
	if tracker.unmounts != 1 || a.transferring || b.transferring {
		t.Fatal("held node leaked or roots locked")
	}
}

func TestCoordinatorTransferRejectsNestedReconciliation(t *testing.T) {
	a, b := transferRoots()
	a.update(VBox(Label("held")))
	target := b.update(VBox())
	child := a.widget().Children()[0]
	var got any
	func() {
		defer func() { got = recover() }()
		_ = (&Coordinator{owner: a.root}).TransferChild(child, target, func() error { b.update(Label("invalid")); return nil }, nil)
	}()
	if got == nil || a.transferring || b.transferring || b.widget() != target || len(a.root.children[0].nodes) != 1 {
		t.Fatal("nested reconciliation damaged ownership")
	}
	a.unmountWindow()
	b.unmountWindow()
}

func TestCoordinatorTransferChecksActualGUIOwnership(t *testing.T) {
	for _, behavior := range []string{"no-move", "detached", "destroyed", "panic-after-move"} {
		t.Run(behavior, func(t *testing.T) {
			a, b := transferRoots()
			tracker := new(lifecycleTracker)
			source := a.update(VBox(transferView(tracker, "held"))).(*gui.LinearBox)
			target := b.update(VBox()).(*gui.LinearBox)
			child := source.Children()[0]
			var err error
			var got any
			func() {
				defer func() { got = recover() }()
				err = (&Coordinator{owner: a.root}).TransferChild(child, target, func() error {
					if behavior == "no-move" {
						return nil
					}
					source.RemoveChild(child)
					if behavior == "panic-after-move" {
						target.AddChild(child)
						panic("original panic")
					}
					if behavior == "destroyed" {
						a.unmountWindow()
						b.unmountWindow()
					}
					return nil
				}, nil)
			}()
			if behavior == "panic-after-move" {
				if got != "original panic" || len(b.root.children[0].nodes) != 1 {
					t.Fatal("panic did not preserve destination ownership")
				}
			} else if err == nil {
				t.Fatal("false GUI success accepted")
			}
			if behavior == "detached" || behavior == "destroyed" {
				if tracker.unmounts != 1 {
					t.Fatal("orphan not released")
				}
			}
			a.unmountWindow()
			b.unmountWindow()
		})
	}
}

func TestCoordinatorTransferPublishesAfterModelCommit(t *testing.T) {
	a, b := transferRoots()
	source := a.update(VBox(Label("page").ID("page"))).(*gui.LinearBox)
	target := b.update(VBox()).(*gui.LinearBox)
	child := source.Children()[0]
	modelChanged, published := false, false
	targetCoordinator := &Coordinator{owner: b.root}
	if err := (&Coordinator{owner: a.root}).TransferChild(child, target, func() error {
		source.RemoveChild(child)
		target.AddChild(child)
		targetCoordinator.AfterTransfer(func() {
			if !modelChanged {
				t.Fatal("notification preceded declaration update")
			}
			published = true
		})
		return nil
	}, func() { modelChanged = true }); err != nil {
		t.Fatal(err)
	}
	if !published {
		t.Fatal("deferred notification was lost")
	}
	a.unmountWindow()
	b.unmountWindow()
}
