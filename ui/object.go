package ui

// FindAncestor borrows the object exposed by the nearest ancestor declaration
// with id. Missing declarations, non-object declarations and incompatible types
// return false; a matching ID shadows outer declarations regardless of type.
// Object receives its own node's context, never the caller's private State.
// Lookup has no creation, subscription or update side effects and belongs to
// the GUI thread. Released contexts cannot resolve objects.
func FindAncestor[T any](ctx BuildContext, id string) (T, bool) {
	var zero T
	c := ctx.Coordinator()
	if id == "" || c == nil || c.owner == nil || c.owner.released || c.owner.root == nil {
		return zero, false
	}
	owner, r := c.owner, c.owner.root
	for n := owner.parent; n != nil; n = n.parent {
		if n.released || n.root != r {
			return zero, false
		}
		if n.id != id {
			continue
		}
		view, ok := n.view.(ObjectView)
		if !ok {
			return zero, false
		}
		version, callerVersion := n.version, owner.version
		object := view.Object(&buildContext{node: n})
		if owner.released || owner.root != r || owner.version != callerVersion || n.released || n.root != r || n.version != version {
			return zero, false
		}
		value, ok := object.(T)
		return value, ok
	}
	return zero, false
}
