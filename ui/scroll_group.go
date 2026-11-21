package ui

import (
	"slices"

	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/internal/identity"
	"github.com/golang-gui/goui/layout"
)

// ScrollHost is the capability required of a grouped scrolling widget.
type ScrollHost interface {
	BindScrollBar(layout.Direction, gui.ScrollBarClient) signal.Handle
}

// ScrollGroupView owns associations without introducing a layout widget or
// scroll state. Its child chooses all layout, spacing, style and bar visibility.
type ScrollGroupView struct {
	ComposeBase[ScrollGroupView]
	child View
}

// ScrollGroup associates descendant ScrollBars with one scrolling host in the
// nearest group. Nested groups are independent. No host is a valid empty state;
// more than one host after reconciliation panics. There is no implicit host-wide
// group. Group membership does not hide the host's built-in bars.
func ScrollGroup(child View) *ScrollGroupView {
	v := &ScrollGroupView{child: child}
	v.Self = v
	return v
}

func (v *ScrollGroupView) Build(ctx BuildContext) View {
	g, _ := ctx.State().(*scrollGroupState)
	if g == nil {
		g = &scrollGroupState{ctx: ctx}
		ctx.SetState(g)
	}
	// An older callback can be superseded when this declaration itself updates.
	g.schedule()
	return v.child
}

func (*ScrollGroupView) Object(ctx BuildContext) any { return ctx.State() }

func (*ScrollGroupView) Reparented(ctx BuildContext) {
	ctx.State().(*scrollGroupState).schedule()
}

func (*ScrollGroupView) Release(ctx BuildContext) {
	if g, ok := ctx.State().(*scrollGroupState); ok {
		g.released = true
		for _, member := range slices.Clone(g.members) {
			member.Disconnect()
		}
		g.ctx = nil
	}
}

// ScrollGroupBinding is an adapter helper for a scrolling widget's membership.
// Keep one per mounted widget. Call Update during normal Update and Reparented,
// and Disconnect during Unmount. Hosts must be stable non-nil pointers. A group
// borrows its host and never owns or lays out the host widget.
type ScrollGroupBinding struct{ member *scrollGroupMember }

func (b *ScrollGroupBinding) Update(ctx BuildContext, host ScrollHost) {
	g, _ := FindAncestor[*scrollGroupState](ctx)
	if b.member != nil && b.member.group == g && identity.SamePointer(b.member.host, host) {
		return
	}
	b.Disconnect()
	if g != nil && host != nil {
		b.member = &scrollGroupMember{host: host}
		g.add(b.member)
	}
}

func (b *ScrollGroupBinding) Disconnect() {
	if b.member != nil {
		b.member.Disconnect()
		b.member = nil
	}
}

type scrollGroupState struct {
	ctx            BuildContext
	members        []*scrollGroupMember
	released       bool
	revision       uint64
	pending        bool
	pendingVersion uint64
}

type scrollGroupMember struct {
	group   *scrollGroupState
	host    ScrollHost
	bar     *gui.ScrollBar
	axis    layout.Direction
	bound   ScrollHost
	binding signal.Handle
}

func (g *scrollGroupState) add(m *scrollGroupMember) {
	if g.released {
		return
	}
	m.group = g
	g.members = append(g.members, m)
	g.revision++
	g.schedule()
}

func (g *scrollGroupState) schedule() {
	if !g.released {
		owner := g.ctx.Coordinator().owner
		if g.pending && g.pendingVersion == owner.version {
			return
		}
		g.pending, g.pendingVersion = true, owner.version
		// AfterUpdate is owned by the persistent group, so a locally replaced
		// member cannot invalidate the callback needed to connect its successor.
		g.ctx.AfterUpdate(g.commit)
	}
}

func (m *scrollGroupMember) Disconnect() {
	if g := m.group; g != nil {
		m.group = nil
		g.revision++
		if i := slices.Index(g.members, m); i >= 0 {
			g.members = slices.Delete(g.members, i, i+1)
		}
		if m.host != nil {
			// Prevent an old target from being controlled before commit.
			for _, other := range slices.Clone(g.members) {
				other.unbind()
			}
		}
		m.unbind()
		g.schedule()
	}
}

func (m *scrollGroupMember) unbind() {
	if m.binding != nil {
		m.binding.Disconnect()
		m.binding = nil
	}
	m.bound = nil
	if m.bar != nil && !m.bar.Destroyed() {
		// Losing the scroll range cancels the old thumb gesture in ScrollBar.
		m.bar.SetMax(0)
		m.bar.SetPageSize(0)
		m.bar.SetValue(0)
	}
}

func (g *scrollGroupState) commit() {
	g.pending = false
	if g.released {
		return
	}
	var host ScrollHost
	revision := g.revision
	for _, member := range g.members {
		if member.host != nil {
			if host != nil {
				for _, m := range slices.Clone(g.members) {
					m.unbind()
				}
				panic("ui: ScrollGroup contains more than one scrolling host")
			}
			host = member.host
		}
	}
	for _, member := range slices.Clone(g.members) {
		if g.released || g.revision != revision {
			return
		}
		if member.group != g || member.bar == nil || identity.SamePointer(member.bound, host) {
			continue
		}
		member.unbind()
		if host != nil && member.group == g && !member.bar.Destroyed() {
			member.bound = host
			binding := host.BindScrollBar(member.axis, member.bar)
			if member.group != g || !identity.SamePointer(member.bound, host) {
				if binding != nil {
					binding.Disconnect()
				}
			} else {
				member.binding = binding
			}
		}
	}
}

var _ ObjectView = (*ScrollGroupView)(nil)
