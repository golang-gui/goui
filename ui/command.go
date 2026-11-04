package ui

import (
	"fmt"
	"slices"

	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
)

// CommandView describes an operation owned by a CommandScope. Its ID is local
// to that scope, independent of Widget IDs. Construct fresh values each build.
type CommandView struct {
	id         string
	enabled    bool
	onActivate func()
}

// Command declares a scope-local operation without creating a runtime object.
func Command(id string) *CommandView {
	if id == "" {
		panic("ui: empty command ID")
	}
	return &CommandView{id: id, enabled: true}
}

// Enabled declares operation availability, independently of individual entries.
func (v *CommandView) Enabled(enabled bool) *CommandView { v.enabled = enabled; return v }

// OnActivate supplies the latest handler without invoking it during a build.
func (v *CommandView) OnActivate(fn func()) *CommandView { v.onActivate = fn; return v }

// CommandScopeView owns operations without creating a widget or layout boundary.
// Descendants resolve the nearest matching command ID, falling back to outer scopes.
type CommandScopeView struct {
	ComposeBase[CommandScopeView]
	child    View
	commands []*CommandView
}

// CommandScope exposes commands to its content without adding layout or paint.
func CommandScope(child View) *CommandScopeView {
	v := &CommandScopeView{child: child}
	v.Self = v
	return v
}
func (v *CommandScopeView) Child(child View) *CommandScopeView { v.child = child; return v }

// Commands replaces this scope's definitions. Nil entries are ignored. IDs
// must be nonempty and unique here; same IDs in nested scopes shadow outer ones.
func (v *CommandScopeView) Commands(commands ...*CommandView) *CommandScopeView {
	v.commands = slices.Clone(commands)
	return v
}

type scopedCommand struct {
	command  *gui.Command
	activate signal.Handle
	callback func()
}
type commandScope struct {
	commands     map[string]*scopedCommand
	parent       *commandScope
	parentHandle signal.Handle
	changed      signal.Signal0
	released     bool
}

func (v *CommandScopeView) Build(ctx BuildContext) View {
	seen := make(map[string]bool)
	for _, view := range v.commands {
		if view == nil {
			continue
		}
		if view.id == "" || seen[view.id] {
			panic(fmt.Sprintf("ui: duplicate or empty command ID %q", view.id))
		}
		seen[view.id] = true
	}
	if ctx.State() == nil {
		ctx.SetState(&commandScope{commands: make(map[string]*scopedCommand)})
	}
	s := ctx.State().(*commandScope)
	s.reparent(ctx)
	changed := false
	for id, entry := range s.commands {
		if !seen[id] {
			delete(s.commands, id)
			entry.callback = nil
			entry.activate.Disconnect()
			entry.command.SetEnabled(false)
			changed = true
			if s.released {
				return nil
			}
		}
	}
	for _, view := range v.commands {
		if view == nil {
			continue
		}
		entry := s.commands[view.id]
		if entry == nil {
			entry = &scopedCommand{command: gui.NewCommand()}
			entry.activate = entry.command.ConnectActivate(func() {
				if !s.released && entry.callback != nil {
					entry.callback()
				}
			})
			s.commands[view.id] = entry
			changed = true
		}
		entry.callback = view.onActivate
		entry.command.SetEnabled(view.enabled)
		if s.released {
			return nil
		}
	}
	if changed {
		s.changed.Emit()
	}
	return v.child
}
func (*CommandScopeView) Object(ctx BuildContext) any { return ctx.State() }
func (*CommandScopeView) Release(ctx BuildContext) {
	s, ok := ctx.State().(*commandScope)
	if !ok || s.released {
		return
	}
	s.released = true
	if s.parentHandle != nil {
		s.parentHandle.Disconnect()
		s.parentHandle = nil
	}
	s.parent = nil
	for id, entry := range s.commands {
		delete(s.commands, id)
		entry.callback = nil
		entry.activate.Disconnect()
		entry.command.SetEnabled(false)
	}
	s.changed.Emit()
}
func (*CommandScopeView) Reparented(ctx BuildContext) {
	if s, ok := ctx.State().(*commandScope); ok {
		s.reparent(ctx)
	}
}
func (s *commandScope) reparent(ctx BuildContext) {
	parent, _ := FindAncestor[*commandScope](ctx)
	if s.parent == parent {
		return
	}
	if s.parentHandle != nil {
		s.parentHandle.Disconnect()
		s.parentHandle = nil
	}
	s.parent = parent
	if parent != nil {
		s.parentHandle = parent.changed.Connect(func() {
			if !s.released {
				s.changed.Emit()
			}
		})
	}
	s.changed.Emit()
}
func (s *commandScope) find(id string) *gui.Command {
	for current := s; current != nil && !current.released; current = current.parent {
		if entry := current.commands[id]; entry != nil {
			return entry.command
		}
	}
	return nil
}

// FindCommand borrows an ancestor scope's operation. Missing or expired
// references return (nil, false), never an operation from another root.
func FindCommand(ctx BuildContext, id string) (*gui.Command, bool) {
	if ctx == nil || id == "" {
		return nil, false
	}
	scope, ok := FindAncestor[*commandScope](ctx)
	if !ok {
		return nil, false
	}
	command := scope.find(id)
	return command, command != nil
}

// commandReference follows membership and ancestry, not enabled-state changes.
// Its owner supplies the target setter and releases the reference on unmount.
type commandReference struct {
	ctx     BuildContext
	scope   *commandScope
	handle  signal.Handle
	id      string
	apply   func(*gui.Command)
	missing *gui.Command
	version uint64
}

func (r *commandReference) update(ctx BuildContext, id string, apply func(*gui.Command)) {
	r.version++
	r.ctx, r.id, r.apply = ctx, id, apply
	r.reparent()
}
func (r *commandReference) reparent() {
	version := r.version
	var scope *commandScope
	if r.ctx != nil && r.id != "" {
		scope, _ = FindAncestor[*commandScope](r.ctx)
	}
	if r.version != version {
		return
	}
	if scope != r.scope {
		if r.handle != nil {
			r.handle.Disconnect()
			r.handle = nil
		}
		r.scope = scope
		if scope != nil {
			r.handle = scope.changed.Connect(r.resolve)
		}
	}
	r.resolve()
}
func (r *commandReference) resolve() {
	if r.apply == nil {
		return
	}
	if r.id == "" {
		r.apply(nil)
		return
	}
	// ObjectView lookup may run user code and release or replace this reference.
	version := r.version
	command, ok := FindCommand(r.ctx, r.id)
	if r.version != version || r.apply == nil {
		return
	}
	if ok {
		r.apply(command)
		return
	}
	// An unresolved operation must remain disabled even after a previous target
	// is removed. The sentinel owns no handlers, registration or scope reference.
	if r.missing == nil {
		r.missing = gui.NewCommand()
		r.missing.SetEnabled(false)
	}
	r.apply(r.missing)
}
func (r *commandReference) clear() {
	r.version++
	if r.handle != nil {
		r.handle.Disconnect()
	}
	r.ctx, r.scope, r.handle, r.apply, r.missing = nil, nil, nil, nil, nil
	r.id = ""
}
