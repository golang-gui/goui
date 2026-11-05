package ui

import (
	"fmt"
	"slices"

	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
)

// CommandView declares a nonvisual operation anywhere below a CommandScope, or
// in its Commands shorthand. The nearest scope owns the operation and its ID
// namespace, independently of Widget IDs. Construct fresh values each build.
type CommandView struct {
	ComposeBase[CommandView]
	id            string
	enabled       bool
	enabledSource ReadState[bool]
	onActivate    func()
}

// Command declares a scope-local operation without creating a runtime object.
func Command(id string) *CommandView {
	if id == "" {
		panic("ui: empty command ID")
	}
	v := &CommandView{id: id, enabled: true}
	v.Self = v
	return v
}

// Enabled declares operation availability, independently of individual entries.
func (v *CommandView) Enabled(enabled bool) *CommandView {
	v.enabled, v.enabledSource = enabled, nil
	return v
}

// BindEnabled follows availability without rebuilding the surrounding views.
// A later Enabled replaces the binding. nil is a no-op.
func (v *CommandView) BindEnabled(source ReadState[bool]) *CommandView {
	if source != nil {
		validateObservable(source)
		v.enabledSource = source
	}
	return v
}

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

// Commands replaces definitions attached directly to this scope. Nil entries
// are ignored. IDs must be unique across this list and all descendant Command
// declarations in the same scope. Nested scopes may shadow the same IDs.
func (v *CommandScopeView) Commands(commands ...*CommandView) *CommandScopeView {
	v.commands = slices.Clone(commands)
	return v
}

type scopedCommand struct {
	command    *gui.Command
	activate   signal.Handle
	definition *commandDefinition
}
type commandScope struct {
	ctx          BuildContext
	commands     map[string]*scopedCommand
	definitions  map[*commandDefinition]struct{}
	shorthand    map[string]*commandDefinition
	dirty        bool
	parent       *commandScope
	parentHandle signal.Handle
	changed      signal.Signal0
	released     bool
}

// A definition belongs to its declaration node; the scope owns the operation.
// Shorthand definitions use the same registration and binding paths.
type commandDefinition struct {
	scope    *commandScope
	id       string
	enabled  bool
	callback func()
}

func (v *CommandView) Build(ctx BuildContext) View {
	if ctx.State() == nil {
		ctx.SetState(new(commandDefinition))
	}
	d := ctx.State().(*commandDefinition)
	scope, ok := FindAncestor[*commandScope](ctx)
	if !ok {
		// Ancestor lookup can release the caller through a custom ObjectView.
		if ctx.State() == d {
			panic("ui: Command requires an ancestor CommandScope")
		}
		return nil
	}
	if ctx.State() == d {
		d.update(ctx, scope, v, "enabled")
	}
	return nil
}
func (*CommandView) Object(ctx BuildContext) any { return ctx.State() }
func (*CommandView) Release(ctx BuildContext) {
	if d, ok := ctx.State().(*commandDefinition); ok {
		d.detach()
	}
}
func (*CommandView) Reparented(ctx BuildContext) {
	d, ok := ctx.State().(*commandDefinition)
	if !ok {
		return
	}
	scope, _ := FindAncestor[*commandScope](ctx)
	if ctx.State() != d {
		return
	}
	old := d.scope
	d.attach(scope)
	// A transfer installs all ancestry before these callbacks run. Publishing
	// here also covers transfers outside an application update batch.
	ctx.Coordinator().AfterTransfer(func() {
		if old != nil {
			old.commit()
		}
		if d.scope != nil {
			d.scope.commit()
		}
	})
}

func (d *commandDefinition) attach(scope *commandScope) {
	if d.scope != scope {
		d.detach()
		d.scope = scope
		if scope != nil {
			scope.definitions[d] = struct{}{}
		}
	}
	if scope != nil {
		scope.invalidate()
	}
}
func (d *commandDefinition) detach() {
	if s := d.scope; s != nil {
		d.scope = nil
		delete(s.definitions, d)
		s.invalidate()
	}
}
func (d *commandDefinition) update(ctx BuildContext, s *commandScope, v *CommandView, property string) {
	if v.id == "" {
		panic("ui: empty command ID")
	}
	d.id, d.callback = v.id, v.onActivate
	d.attach(s)
	if v.enabledSource != nil {
		BindValue(ctx, property, v.enabledSource, d.setEnabled)
	} else {
		d.setEnabled(v.enabled)
	}
}
func (d *commandDefinition) setEnabled(enabled bool) {
	d.enabled = enabled
	if d.scope != nil {
		d.scope.invalidate()
	}
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
		ctx.SetState(&commandScope{
			commands:    make(map[string]*scopedCommand),
			definitions: make(map[*commandDefinition]struct{}),
			shorthand:   make(map[string]*commandDefinition),
		})
	}
	s := ctx.State().(*commandScope)
	s.ctx = ctx
	s.reparent(ctx)
	if s.released {
		return nil
	}
	for id, definition := range s.shorthand {
		if !seen[id] {
			delete(s.shorthand, id)
			definition.detach()
		}
	}
	for _, view := range v.commands {
		if view == nil {
			continue
		}
		d := s.shorthand[view.id]
		if d == nil {
			d = new(commandDefinition)
			s.shorthand[view.id] = d
		}
		d.update(ctx, s, view, "command-enabled:"+view.id)
		if s.released {
			return nil
		}
	}
	// Always queue against the current scope version: an earlier queued hook
	// can have been superseded while its descendants remain mounted.
	s.invalidate()
	return v.child
}
func (*CommandScopeView) Object(ctx BuildContext) any { return ctx.State() }
func (*CommandScopeView) Release(ctx BuildContext) {
	s, ok := ctx.State().(*commandScope)
	if !ok || s.released {
		return
	}
	s.released = true
	s.ctx = nil
	if s.parentHandle != nil {
		s.parentHandle.Disconnect()
		s.parentHandle = nil
	}
	s.parent = nil
	for d := range s.definitions {
		d.scope = nil
		d.callback = nil
	}
	s.definitions, s.shorthand = nil, nil
	for id, entry := range s.commands {
		delete(s.commands, id)
		entry.definition = nil
		entry.activate.Disconnect()
		entry.command.SetEnabled(false)
	}
	s.changed.Emit()
}
func (*CommandScopeView) Reparented(ctx BuildContext) {
	if s, ok := ctx.State().(*commandScope); ok {
		s.ctx = ctx
		s.reparent(ctx)
		s.invalidate()
		ctx.Coordinator().AfterTransfer(s.commit)
	}
}

func (s *commandScope) invalidate() {
	if !s.released && s.ctx != nil {
		s.dirty = true
		s.ctx.AfterUpdate(s.commit)
	}
}

func (s *commandScope) commit() {
	if s.released || !s.dirty {
		return
	}
	definitions := make(map[string]*commandDefinition, len(s.definitions))
	for d := range s.definitions {
		if definitions[d.id] != nil {
			panic(fmt.Sprintf("ui: duplicate command ID %q", d.id))
		}
		definitions[d.id] = d
	}
	// Validate before mutating any operation. Replacing or moving a declaration
	// in this batch retains the scope's existing object for the same ID.
	previous := s.commands
	next := make(map[string]*scopedCommand, len(definitions))
	changed := len(previous) != len(definitions)
	for id, d := range definitions {
		entry := previous[id]
		if entry == nil {
			entry = &scopedCommand{command: gui.NewCommand()}
			entry.activate = entry.command.ConnectActivate(func() {
				if definition := entry.definition; !s.released && definition != nil && definition.scope == s && definition.callback != nil {
					definition.callback()
				}
			})
			changed = true
		}
		entry.definition = d
		next[id] = entry
	}
	s.commands, s.dirty = next, false
	for id, entry := range previous {
		if next[id] == nil {
			entry.definition = nil
			entry.activate.Disconnect()
			entry.command.SetEnabled(false)
		}
	}
	for _, entry := range next {
		if s.released {
			return
		}
		entry.command.SetEnabled(entry.definition.enabled)
	}
	if changed && !s.released {
		s.changed.Emit()
	}
}
func (s *commandScope) reparent(ctx BuildContext) {
	parent, _ := FindAncestor[*commandScope](ctx)
	if s.released {
		return
	}
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

// FindCommand borrows an ancestor scope's last published operation. Definitions
// are published after reconciliation, before layout/paint; the initial Build
// may not resolve a definition yet. Missing or expired references return
// (nil, false), never an operation from another root.
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
