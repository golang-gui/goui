package ui

// FindAncestor borrows the nearest compatible ancestor object. With an ID it
// instead looks up that exact declaration: a matching ID shadows outer objects
// even when it exposes no compatible value. An explicitly empty ID finds nothing.
// Object receives its own node's context, never the caller's private State.
// Lookup has no creation, subscription or update side effects and belongs to
// the GUI thread. Released contexts cannot resolve objects.
func FindAncestor[T any](ctx BuildContext, ids ...string) (T, bool) {
	if len(ids) > 1 {
		panic("ui: FindAncestor accepts at most one ID")
	}
	named := len(ids) == 1
	id := ""
	if named {
		id = ids[0]
	}
	var zero T
	c := ctx.Coordinator()
	if named && id == "" || c == nil || c.owner == nil || c.owner.released || c.owner.root == nil {
		return zero, false
	}
	owner, r := c.owner, c.owner.root
	for n := owner.parent; n != nil; n = n.parent {
		if n.released || n.root != r {
			return zero, false
		}
		if named && n.id != id {
			continue
		}
		view, ok := n.view.(ObjectView)
		if !ok {
			if named {
				return zero, false
			}
			continue
		}
		version, callerVersion := n.version, owner.version
		object := view.Object(&buildContext{node: n})
		if owner.released || owner.root != r || owner.version != callerVersion || n.released || n.root != r || n.version != version {
			return zero, false
		}
		value, ok := object.(T)
		if named || ok {
			return value, ok
		}
	}
	return zero, false
}
