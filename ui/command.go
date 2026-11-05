package ui

import (
	"fmt"
	"slices"

	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
)

// CommandView describes an operation supplied to CommandGroup.Commands or an
// entry's DefineCommand. It is a declaration value, not a View. The nearest
// scope owns the operation and its ID namespace, independently of Widget IDs.
// Construct fresh values each build.
type CommandView struct {
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
	return &CommandView{id: id, enabled: true}
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

// CommandGroupView owns operations without creating a widget or layout boundary.
// Descendants resolve the nearest matching command ID, falling back to outer scopes.
type CommandGroupView struct {
	ComposeBase[CommandGroupView]
	child    View
	commands []*CommandView
}

// CommandGroup exposes commands to its content without adding layout or paint.
func CommandGroup(child View) *CommandGroupView {
	v := &CommandGroupView{child: child}
	v.Self = v
	return v
}
func (v *CommandGroupView) Child(child View) *CommandGroupView { v.child = child; return v }

// Commands replaces definitions attached directly to this scope. Nil entries
// are ignored. IDs must be unique across this list and descendant menu/shortcut
// definitions in the same scope. Nested scopes may shadow the same IDs.
func (v *CommandGroupView) Commands(commands ...*CommandView) *CommandGroupView {
	v.commands = slices.Clone(commands)
	return v
}

type scopedCommand struct {
	command    *gui.Command
	activate   signal.Handle
	definition *commandDefinition
}
type commandGroup struct {
	ctx          BuildContext
	commands     map[string]*scopedCommand
	definitions  map[*commandDefinition]struct{}
	shorthand    commandDefinitions
	dirty        bool
	parent       *commandGroup
	parentHandle signal.Handle
	changed      signal.Signal0
	released     bool
}

// A definition belongs to its declaring menu, shortcut list or scope. All
// hosts use the same registration and property binding paths.
type commandDefinition struct {
	scope    *commandGroup
	id       string
	enabled  bool
	callback func()
}

func (d *commandDefinition) attach(scope *commandGroup) {
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
func (d *commandDefinition) update(ctx BuildContext, s *commandGroup, v *CommandView, property string) {
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

// commandDefinitions tracks declarations owned by one existing UI host. It
// creates no View nodes; bindings belong to that host's normal update cycle.
type commandDefinitions struct {
	ctx     BuildContext
	entries map[string]*commandDefinition
	version uint64
}

func (d *commandDefinitions) update(ctx BuildContext, scope *commandGroup, views []*CommandView, property string) {
	seen := make(map[string]bool, len(views))
	for _, view := range views {
		if view == nil {
			continue
		}
		if view.id == "" || seen[view.id] {
			panic(fmt.Sprintf("ui: duplicate or empty command ID %q", view.id))
		}
		seen[view.id] = true
	}
	if len(seen) == 0 {
		d.clear()
		return
	}
	if ctx == nil {
		panic("ui: DefineCommand requires a mounted command group")
	}
	d.version++
	version := d.version
	coordinator := ctx.Coordinator()
	if coordinator == nil || coordinator.owner == nil || coordinator.owner.released || coordinator.owner.root == nil {
		return
	}
	owner := coordinator.owner
	root, nodeVersion := owner.root, owner.version
	epoch := root.epoch
	live := func() bool {
		return d.version == version && !owner.released && owner.root == root && root.epoch == epoch && owner.version == nodeVersion
	}
	if scope == nil {
		scope, _ = FindAncestor[*commandGroup](ctx)
	}
	if !live() {
		return
	}
	if scope == nil || scope.released {
		panic("ui: DefineCommand requires an ancestor CommandGroup")
	}
	d.ctx = ctx
	if d.entries == nil {
		d.entries = make(map[string]*commandDefinition, len(seen))
	}
	for id, entry := range d.entries {
		if !seen[id] {
			delete(d.entries, id)
			entry.detach()
			entry.callback = nil
		}
	}
	for _, view := range views {
		if view == nil {
			continue
		}
		entry := d.entries[view.id]
		if entry == nil {
			entry = new(commandDefinition)
			d.entries[view.id] = entry
		}
		entry.update(ctx, scope, view, property+view.id)
		if !live() {
			return
		}
	}
}

func (d *commandDefinitions) clear() {
	entries := d.entries
	d.version++
	d.ctx, d.entries = nil, nil
	for _, entry := range entries {
		entry.detach()
		entry.callback = nil
	}
}

func (d *commandDefinitions) reparented() {
	if d.ctx == nil || len(d.entries) == 0 {
		return
	}
	d.version++
	version := d.version
	ctx := d.ctx
	scope, _ := FindAncestor[*commandGroup](ctx)
	if d.version != version {
		return
	}
	var previous *commandGroup
	for _, entry := range d.entries {
		previous = entry.scope
		entry.attach(scope)
	}
	// Transfer callbacks run after every descendant has its final ancestry.
	ctx.Coordinator().AfterTransfer(func() {
		if previous != nil {
			previous.commit()
		}
		if scope != nil {
			scope.commit()
		}
	})
}

func (v *CommandGroupView) Build(ctx BuildContext) View {
	if ctx.State() == nil {
		ctx.SetState(&commandGroup{
			commands:    make(map[string]*scopedCommand),
			definitions: make(map[*commandDefinition]struct{}),
		})
	}
	s := ctx.State().(*commandGroup)
	s.ctx = ctx
	s.reparent(ctx)
	if s.released {
		return nil
	}
	s.shorthand.update(ctx, s, v.commands, "scope-command:")
	if s.released {
		return nil
	}
	// Always queue against the current scope version: an earlier queued hook
	// can have been superseded while its descendants remain mounted.
	s.invalidate()
	return v.child
}
func (*CommandGroupView) Object(ctx BuildContext) any { return ctx.State() }
func (*CommandGroupView) Release(ctx BuildContext) {
	s, ok := ctx.State().(*commandGroup)
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
	s.definitions = nil
	s.shorthand.clear()
	for id, entry := range s.commands {
		delete(s.commands, id)
		entry.definition = nil
		entry.activate.Disconnect()
		entry.command.SetEnabled(false)
	}
	s.changed.Emit()
}
func (*CommandGroupView) Reparented(ctx BuildContext) {
	if s, ok := ctx.State().(*commandGroup); ok {
		s.ctx = ctx
		s.reparent(ctx)
		s.invalidate()
		ctx.Coordinator().AfterTransfer(s.commit)
	}
}

func (s *commandGroup) invalidate() {
	if !s.released && s.ctx != nil {
		s.dirty = true
		s.ctx.AfterUpdate(s.commit)
	}
}

func (s *commandGroup) live() bool {
	if s.released || s.ctx == nil {
		return false
	}
	coordinator := s.ctx.Coordinator()
	return coordinator != nil && coordinator.owner != nil && !coordinator.owner.released && coordinator.owner.root != nil
}

func (s *commandGroup) commit() {
	if !s.live() || !s.dirty {
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
				if definition := entry.definition; s.live() && definition != nil && definition.scope == s && definition.callback != nil {
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
		if !s.live() {
			return
		}
		entry.command.SetEnabled(entry.definition.enabled)
	}
	if changed && s.live() {
		s.changed.Emit()
	}
}
func (s *commandGroup) reparent(ctx BuildContext) {
	parent, _ := FindAncestor[*commandGroup](ctx)
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
func (s *commandGroup) find(id string) *gui.Command {
	for current := s; current != nil && current.live(); current = current.parent {
		if entry := current.commands[id]; entry != nil {
			return entry.command
		}
	}
	return nil
}

// findCommand borrows an ancestor group's last published operation. Definitions
// are published after reconciliation, before layout/paint; the initial Build
// may not resolve a definition yet. Missing or expired references return
// (nil, false), never an operation from another root.
func findCommand(ctx BuildContext, id string) (*gui.Command, bool) {
	if ctx == nil || id == "" {
		return nil, false
	}
	scope, ok := FindAncestor[*commandGroup](ctx)
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
	scope   *commandGroup
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
	var scope *commandGroup
	if r.ctx != nil && r.id != "" {
		scope, _ = FindAncestor[*commandGroup](r.ctx)
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
	command, ok := findCommand(r.ctx, r.id)
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
