package gui

import (
	"slices"

	"github.com/golang-gui/goui/platform/events"
)

// ModalInfoKey identifies a Widget subtree's modal input boundary.
const ModalInfoKey = "goui.modal"

// ModalInfo describes input policy, independently of appearance and Enabled.
type ModalInfo struct {
	Active bool `json:"active"`
}

// ModalScope restricts its Root's content input to one Widget subtree. It does
// not draw, arrange, close or disable Widgets. Visible, mounted scopes compete
// in ordinary paint order; only the frontmost scope is active. Window-owned
// decorations retain their window operations. All methods belong to the GUI
// thread. A scope must not be copied after construction.
type ModalScope struct {
	widget    Widget
	root      Root
	lastFocus Widget
}

// NewModalScope attaches a boundary, including before mounting. A nil or
// destroyed Widget, or a second scope on the same Widget, is a programming error.
// Hiding/unmounting withdraws the boundary; destroying the Widget releases it.
func NewModalScope(widget Widget) *ModalScope {
	if widget == nil || widget.base().destroyed || widget.base().modal != nil {
		panic("gui: ModalScope requires a live widget without a modal scope")
	}
	s := &ModalScope{widget: widget}
	widget.base().modal = s
	s.mount()
	return s
}

// Active reports whether this scope currently limits its Root's content input.
// Querying it does not deliver focus callbacks.
func (s *ModalScope) Active() bool {
	if s == nil || s.root == nil {
		return false
	}
	host, ok := s.root.(modalHost)
	return ok && modalScopeOf(host) == s
}

// Destroy removes the boundary without destroying its Widget. It is idempotent.
func (s *ModalScope) Destroy() {
	if s == nil || s.widget == nil {
		return
	}
	root := s.root
	s.unmount()
	s.widget.base().modal = nil
	s.widget, s.lastFocus = nil, nil
	if host, ok := root.(modalHost); ok {
		ensureModal(host)
	}
}

type modalHost interface {
	Root
	EventTarget
	rootState() *rootBase
}

type modalState struct {
	host       modalHost
	scopes     map[*ModalScope]struct{}
	active     *ModalScope
	background Widget
	revision   uint64
	dirty      bool
	pending    bool
	engaged    bool
	updating   bool
}

func (s *ModalScope) mount() {
	if s.widget == nil || s.widget.base().self == nil {
		return
	}
	root := s.widget.Root()
	host, ok := root.(modalHost)
	if !ok || root == s.root {
		return
	}
	s.unmount()
	b := host.rootState()
	if b.modal == nil {
		b.modal = &modalState{host: host, scopes: make(map[*ModalScope]struct{})}
	}
	s.root = root
	b.modal.scopes[s] = struct{}{}
	modalChanged(root)
}

func (s *ModalScope) unmount() {
	if host, ok := s.root.(modalHost); ok && host.rootState().modal != nil {
		delete(host.rootState().modal.scopes, s)
		modalChanged(s.root)
	}
	s.root = nil
}

func modalChanged(root Root) {
	if host, ok := root.(modalHost); ok && host.rootState().modal != nil {
		host.rootState().modal.dirty = true
		_ = root.RequestPaint()
	}
}

func ensureModalRoot(root Root) {
	if host, ok := root.(modalHost); ok {
		ensureModal(host)
	}
}

// A lifecycle change can restore focus while withdrawing a modal subtree.
// Recheck containment afterwards rather than clearing that restored focus.
func clearSubtreeFocus(host EventTarget, subtree *WidgetBase) {
	ensureModal(host)
	if focusWithin(host, subtree) {
		host.SetFocusedWidget(nil)
	}
}

func modalScopeOf(host EventTarget) *ModalScope {
	h, ok := host.(modalHost)
	if !ok || h.rootState().modal == nil {
		return nil
	}
	s := h.rootState().modal
	s.selectScope()
	return s.active
}

func modalRevision(host EventTarget) uint64 {
	modalScopeOf(host)
	if h, ok := host.(modalHost); ok && h.rootState().modal != nil {
		return h.rootState().modal.revision
	}
	return 0
}

func (s *modalState) selectScope() {
	if !s.dirty {
		return
	}
	s.dirty = false
	var next *ModalScope
	for candidate := range s.scopes {
		w := candidate.widget
		if w == nil || w.base().destroyed || !visibleInTree(w) || !modalMounted(w) ||
			!w.base().isDescendant(w, s.host.Widget()) {
			continue
		}
		if next == nil || modalInFront(w, next.widget) {
			next = candidate
		}
	}
	if next == s.active {
		if next != nil {
			focus := s.host.FocusedWidget()
			if focus != nil && (!modalFocusable(s.host, focus)) {
				s.pending = true
			}
		}
		return
	}
	focus := s.host.FocusedWidget()
	if old := s.active; old != nil && old.widget != nil && focus != nil && focus.base().isDescendant(focus, old.widget) {
		old.lastFocus = focus
	}
	if next != nil && !s.engaged {
		s.background, s.engaged = focus, true
	}
	s.active = next
	s.revision++
	s.pending = true
}

func modalMounted(w Widget) bool {
	for ; w != nil; w = w.Parent() {
		if w.base().unmounting || w.base().self == nil {
			return false
		}
	}
	return true
}

func modalInFront(a, b Widget) bool {
	var pa, pb []Widget
	for w := a; w != nil; w = w.Parent() {
		pa = append(pa, w)
	}
	for w := b; w != nil; w = w.Parent() {
		pb = append(pb, w)
	}
	i, j := len(pa)-1, len(pb)-1
	for i >= 0 && j >= 0 && pa[i] == pb[j] {
		i--
		j--
	}
	if j < 0 {
		return i >= 0
	}
	if i < 0 {
		return false
	}
	parent := pa[i].Parent()
	return parent != nil && slices.Index(parent.base().children, pa[i]) > slices.Index(parent.base().children, pb[j])
}

func ensureModal(host EventTarget) {
	h, ok := host.(modalHost)
	if !ok || h.rootState().modal == nil {
		return
	}
	s, b := h.rootState().modal, h.rootState()
	s.selectScope()
	if s.updating || !s.pending || b.frameDestroyed {
		return
	}
	s.updating, s.pending = true, false
	defer func() { s.updating = false }()
	revision := s.revision
	b.cancelInput(GestureInterrupted)
	if app := b.app; app != nil && app.dragSession != nil && app.dragSession.host == h && !modalWidgetAllowed(app.dragSession.widget) {
		app.dragSession.cancel()
	}
	b.dispatcher.updatePointerHoverPath(nil, b.dispatcher.pointerEvent)
	if tips := tooltipsFor(host); tips != nil {
		tips.dismiss(false)
	}
	if w, ok := host.(*window); ok {
		for _, p := range slices.Clone(w.popovers) {
			if p.visible && !modalWidgetAllowed(p.anchor) {
				p.Hide()
			}
		}
		if target := w.modalTarget; target != nil && !modalTargetAllowed(w, target) {
			w.SetModalTarget(nil)
			target.RequestDismiss()
		}
	}
	s.selectScope()
	if b.frameDestroyed || s.revision != revision {
		return
	}
	var focus Widget
	if s.active != nil {
		if modalFocusable(host, host.FocusedWidget()) {
			return
		}
		if modalFocusable(host, s.active.lastFocus) {
			focus = s.active.lastFocus
		}
		if focus == nil {
			focus = firstModalFocus(s.active.widget)
		}
	} else if s.engaged {
		if modalFocusable(host, s.background) {
			focus = s.background
		}
		if focus == nil {
			focus = firstModalFocus(host.Widget())
			if !modalFocusable(host, focus) {
				focus = nil
			}
		}
		s.background, s.engaged = nil, false
	}
	host.SetFocusedWidget(focus)
	if !b.frameDestroyed && modalRevision(host) == revision {
		if b.dispatcher.pointerInside {
			b.dispatcher.updateHover(host, host.Widget(), b.dispatcher.pointerEvent)
		}
		if window, ok := host.(*window); ok {
			window.applyCursor()
		}
	}
}

func firstModalFocus(w Widget) Widget {
	if w == nil || w.base().destroyed || !w.Visible() || !IsEnabled(w) {
		return nil
	}
	if w.Focusable() {
		return w
	}
	for _, child := range w.base().children {
		if found := firstModalFocus(child); found != nil {
			return found
		}
	}
	return nil
}

func modalFocusable(host EventTarget, w Widget) bool {
	root, ok := host.(Root)
	return ok && w != nil && !w.base().destroyed && w.Root() == root && w.Focusable() &&
		IsEnabled(w) && visibleInTree(w) && modalWidgetAllowed(w)
}

func modalWidgetAllowed(w Widget) bool {
	var storage [8]Root
	seen := storage[:0]
	for w != nil {
		root := w.Root()
		if w.base().destroyed {
			return false
		}
		if root == nil {
			return true
		}
		if slices.Contains(seen, root) {
			return false
		}
		seen = append(seen, root)
		host, ok := root.(EventTarget)
		if !ok {
			return true
		}
		if scope := modalScopeOf(host); scope != nil && !w.base().isDescendant(w, scope.widget) {
			return false
		}
		p, popup := root.(*popover)
		if !popup || p.anchor == nil {
			return true
		}
		w = p.anchor
	}
	return false
}

func modalInputAllowed(host EventTarget, target Widget) bool {
	if target == nil {
		return false
	}
	// Window-owned decorations are outside ordinary content modality.
	if h, ok := host.(modalHost); ok {
		decoration := h.rootState().dispatcher.decoration
		if decoration != nil && target.base().isDescendant(target, decoration) {
			return true
		}
	}
	return modalWidgetAllowed(target)
}

func modalPath(host EventTarget, path []Widget) []Widget {
	if len(path) == 0 || !modalInputAllowed(host, path[len(path)-1]) {
		return nil
	}
	if scope := modalScopeOf(host); scope != nil {
		if index := slices.Index(path, scope.widget); index >= 0 {
			return path[index:]
		}
	}
	return path
}

// AnchoredModalTarget reports the Widget that owns a modal input target. A
// target needs this capability to receive input while a Widget modal scope is
// active. Targets without a scope retain the original ModalTarget contract.
type AnchoredModalTarget interface {
	ModalTarget
	Anchor() Widget
}

func modalTargetAllowed(host EventTarget, target ModalTarget) bool {
	if modalScopeOf(host) == nil {
		return true
	}
	anchored, ok := target.(AnchoredModalTarget)
	return ok && anchored.Anchor() != nil && modalWidgetAllowed(anchored.Anchor())
}

func modalPreventDefault(event events.Event) {
	if key, ok := event.(events.KeyEvent); ok {
		key.PreventDefault()
	}
}
