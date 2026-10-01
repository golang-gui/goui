package ui

import (
	"fmt"
	"reflect"
	"slices"

	"github.com/golang-gui/goui/gui"
)

// Coordinator is a borrowed, GUI-thread handle for explicit node transfers.
// It follows its node when transferred and expires on release. It does not
// expose another View's State or establish global identity.
type Coordinator struct{ owner *node }

func (c *Coordinator) find(widget gui.Widget) *node {
	if c == nil || c.owner == nil || c.owner.released || c.owner.root == nil || nilWidget(widget) {
		return nil
	}
	r := c.owner.root
	if n := findNode(r.root, widget); n != nil {
		return n
	}
	if r.runtime != nil {
		for _, mount := range r.runtime.windows {
			if !mount.destroying && mount.root != r {
				if n := findNode(mount.root.root, widget); n != nil {
					return n
				}
			}
		}
	}
	return nil
}

func findNode(n *node, widget gui.Widget) *node {
	if n == nil || n.released {
		return nil
	}
	if n.widget == widget {
		return n
	}
	for _, target := range n.children {
		for _, child := range target.nodes {
			if found := findNode(child, widget); found != nil {
				return found
			}
		}
	}
	return nil
}

func nilWidget(widget gui.Widget) bool {
	return widget == nil || (reflect.ValueOf(widget).Kind() == reflect.Pointer && reflect.ValueOf(widget).IsNil())
}

func destroyedWidget(widget gui.Widget) bool {
	state, ok := widget.(interface{ Destroyed() bool })
	return ok && state.Destroyed()
}

func containerWidget(target Container) gui.Widget {
	if adapter, ok := target.(TransferContainer); ok {
		return adapter.Widget()
	}
	widget, _ := target.(gui.Widget)
	return widget
}

func transferTarget(owner *node) *childTarget {
	var found *childTarget
	for _, target := range owner.children {
		if target.container != nil && containerWidget(target.container) == owner.widget {
			if found != nil {
				return nil
			}
			found = target
		}
	}
	return found
}

// AfterTransfer delays a binding notification until the active transfer and its
// committed callback finish. Outside a transfer it runs immediately. Released
// owners are skipped. This is for adapter signals, not UI update scheduling.
func (c *Coordinator) AfterTransfer(fn func()) {
	if c == nil || c.owner == nil || c.owner.released || c.owner.root == nil || fn == nil {
		return
	}
	r := c.owner.root
	if r.transferring {
		r.transferAfter = append(r.transferAfter, afterUpdateCall{owner: c.owner, version: c.owner.version, fn: fn})
	} else {
		fn()
	}
}

// TransferChild transfers one ordinary, directly managed list node to the View
// owning to. Both lists must be registered by UpdateChildren and identify their
// owning Widget (directly or through TransferContainer). move performs the GUI
// transaction; ordinary failure must leave the original GUI parent intact.
// committed updates application declarations after successful adoption and
// before deferred binding signals. It may be nil. RequestUpdate, not a nested
// reconciliation, is the normal next step.
//
// No View Mount/Unmount or signal reconnection occurs on success. GUI lifecycle
// remains move's responsibility. The actual parent/root are checked, including
// callback panic and destruction: an orphan is released, never resurrected.
func (c *Coordinator) TransferChild(child, to gui.Widget, move func() error, committed func()) (err error) {
	destination := c.find(to)
	if destination == nil || nilWidget(child) || destroyedWidget(child) || destroyedWidget(destination.widget) || destroyedWidget(c.owner.widget) || move == nil {
		return fmt.Errorf("ui: transfer requires live coordinated nodes")
	}
	source := c.owner
	fromRoot, toRoot := source.root, destination.root
	if source == destination || fromRoot.transferring || toRoot.transferring || fromRoot.reconciling || toRoot.reconciling {
		return fmt.Errorf("ui: child transfer is unavailable")
	}
	from, target := transferTarget(source), transferTarget(destination)
	if from == nil || target == nil || child.Parent() != source.widget {
		return fmt.Errorf("ui: transfer requires registered owning containers")
	}
	index := slices.IndexFunc(from.nodes, func(n *node) bool { return n.widget == child })
	if index < 0 {
		return fmt.Errorf("ui: child is not a coordinated source node")
	}
	n := from.nodes[index]
	if n.id != "" && slices.ContainsFunc(target.nodes, func(other *node) bool { return other.id == n.id }) {
		return fmt.Errorf("ui: target already declares child ID %q", n.id)
	}
	if findNode(n, to) != nil {
		return fmt.Errorf("ui: cannot transfer into own subtree")
	}
	if err := checkTransferIDs(child, toRoot.widget()); err != nil {
		return err
	}
	fromRoot.transferring, toRoot.transferring = true, true
	from.nodes = slices.Delete(from.nodes, index, index+1)
	rehomeNode(n, nil)
	adopted := false
	restore := func() {
		if n.released {
			return
		}
		if !destroyedWidget(child) && !destination.released && !destroyedWidget(destination.widget) && destination.root == toRoot && child.Parent() == destination.widget && child.Root() == destination.widget.Root() {
			insert := 0
			for _, widget := range destination.widget.Children() {
				if widget == child {
					break
				}
				if slices.ContainsFunc(target.nodes, func(node *node) bool { return node.widget == widget }) {
					insert++
				}
			}
			target.nodes = slices.Insert(target.nodes, insert, n)
			rehomeNode(n, toRoot)
			adopted = true
		} else if !destroyedWidget(child) && !source.released && !destroyedWidget(source.widget) && source.root == fromRoot && child.Parent() == source.widget && child.Root() == source.widget.Root() {
			from.nodes = slices.Insert(from.nodes, min(index, len(from.nodes)), n)
			rehomeNode(n, fromRoot)
		} else {
			fromRoot.release(n, false)
		}
	}
	settled := false
	defer func() {
		failure := recover()
		if !settled {
			restore()
		}
		fromRoot.transferring, toRoot.transferring = false, false
		calls := append(fromRoot.transferAfter, toRoot.transferAfter...)
		if fromRoot == toRoot {
			calls = fromRoot.transferAfter
		}
		fromRoot.transferAfter, toRoot.transferAfter = nil, nil
		if failure != nil {
			panic(failure)
		}
		if settled {
			for _, call := range calls {
				if !call.owner.released && call.owner.root != nil && call.owner.version == call.version {
					call.fn()
				}
			}
		}
	}()
	err = move()
	restore()
	settled = true
	if !adopted {
		if err == nil {
			err = fmt.Errorf("ui: GUI transaction did not establish destination ownership")
		}
		return err
	}
	if err != nil {
		return err
	}
	if committed != nil {
		committed()
	}
	return nil
}

func rehomeNode(n *node, owner *root) {
	n.root = owner
	n.version++
	for _, target := range n.children {
		for _, child := range target.nodes {
			rehomeNode(child, owner)
		}
	}
}

func checkTransferIDs(child, destination gui.Widget) error {
	moving := make(map[gui.Widget]bool)
	ids := make(map[string]bool)
	var add func(gui.Widget) error
	add = func(widget gui.Widget) error {
		moving[widget] = true
		if id := widget.ID(); id != "" {
			if ids[id] {
				return fmt.Errorf("ui: duplicate moving widget ID %q", id)
			}
			ids[id] = true
		}
		for _, child := range widget.Children() {
			if err := add(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := add(child); err != nil {
		return err
	}
	var check func(gui.Widget) error
	check = func(widget gui.Widget) error {
		if widget == nil || moving[widget] {
			return nil
		}
		if id := widget.ID(); id != "" && ids[id] {
			return fmt.Errorf("ui: target already contains widget ID %q", id)
		}
		for _, child := range widget.Children() {
			if err := check(child); err != nil {
				return err
			}
		}
		return nil
	}
	return check(destination)
}
