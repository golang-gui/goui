package gui

import (
	"errors"
	"image/color"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui/textedit"
	"github.com/golang-gui/goui/mlang"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
)

// labelSelection is allocated only for selection APIs. Ordinary labels keep
// their existing layout and paint path, without controllers or geometry.
type labelSelection struct {
	label             *Label
	enabled           bool
	value             TextSelection
	upstream          bool
	geometry          *textedit.Geometry
	controller        *labelSelectionController
	context           *ContextMenuEventController
	menu              *PopoverMenu
	language          signal.Handle
	previousCursor    Cursor
	previousFocusable bool
	revision          uint64
	changed           signal.Signal2[TextSelection, uint64]
	copyError         signal.Signal2[error, uint64]
}

func (l *Label) selectionState() *labelSelection {
	if l.selectState == nil {
		l.selectState = &labelSelection{label: l}
		l.ConnectFocused(func(bool) { l.RequestPaint() })
	}
	return l.selectState
}

// Selectable reports whether users can select and copy the displayed text.
func (l *Label) Selectable() bool { return l.selectState != nil && l.selectState.enabled }

// SetSelectable enables selection, keyboard focus and a text cursor. Disabling
// clears selection and restores the previous focusability/cursor if unchanged
// since enabling. Applications can override these defaults with Widget setters.
func (l *Label) SetSelectable(value bool) {
	if l.Destroyed() || l.Selectable() == value {
		return
	}
	s := l.selectionState()
	s.enabled = value
	s.revision++
	if value {
		s.previousFocusable, s.previousCursor = l.Focusable(), l.Cursor()
		l.SetFocusable(true)
		l.SetCursor(CursorText)
		if s.controller == nil {
			s.controller = &labelSelectionController{EventControllerBase: NewEventControllerBase(PhaseBubble), state: s, now: time.Now}
			s.context = NewContextMenuEventController()
			s.context.ConnectRequest(func(ctx EventContext) {
				if !s.interactive() {
					return
				}
				point, pointer := ctx.Position()
				if !pointer {
					if g := s.metrics(); g != nil {
						r := g.Caret(textedit.Position{Offset: s.value.Caret, Upstream: s.upstream})
						point = geometry.Point{X: r.X, Y: r.Y + r.Height}
					}
				}
				ctx.StopPropagation()
				s.showMenu(point)
			})
		}
		l.AddEventController(s.controller)
		l.AddEventController(s.context)
	} else {
		s.cancel()
		if l.Destroyed() || s.enabled {
			return
		}
		l.RemoveEventController(s.controller)
		l.RemoveEventController(s.context)
		if l.Focusable() {
			l.SetFocusable(s.previousFocusable)
		}
		if l.Destroyed() || s.enabled {
			return
		}
		if l.Cursor() == CursorText {
			l.SetCursor(s.previousCursor)
		}
		if l.Destroyed() || s.enabled {
			return
		}
		s.geometry = nil
		s.set(TextSelection{}, false)
	}
	l.RequestPaint()
	l.requestSemanticUpdate()
}

// Selection returns UTF-8 byte endpoints in PlainText, preserving direction.
func (l *Label) Selection() TextSelection {
	if l.selectState != nil {
		return l.selectState.value
	}
	return TextSelection{}
}

// SetSelection clamps endpoints to the displayed text and available shaping
// Clusters. Before layout is available, only UTF-8 boundaries can be enforced.
// It is a no-op when selection is disabled. Layout/paint never emit Selection.
func (l *Label) SetSelection(value TextSelection) {
	if !l.Selectable() || l.Destroyed() {
		return
	}
	s := l.selectState
	value.Anchor, value.Caret = s.snap(value.Anchor), s.snap(value.Caret)
	s.set(value, false)
}

// SelectedText returns the logical plain-text range, without format markers or
// extra line breaks at soft wraps. It does not create native resources.
func (l *Label) SelectedText() string {
	r := l.Selection().Range()
	return l.PlainText()[r.Start:r.End]
}

// SelectAll selects the entire PlainText when selection is enabled.
func (l *Label) SelectAll() { l.SetSelection(TextSelection{Caret: len(l.PlainText())}) }

// Copy writes the selected plain text to the system clipboard. Empty selection
// is a no-op. Direct calls return errors without emitting CopyError.
func (l *Label) Copy() error {
	if l.Destroyed() {
		return errors.New("label: destroyed")
	}
	if !l.Selectable() {
		return nil
	}
	s := l.selectState
	if !s.normalize() {
		return nil
	}
	return s.copyText()
}

func (s *labelSelection) copyText() error {
	text := s.label.SelectedText()
	if text == "" {
		return nil
	}
	if App == nil || App.Clipboard() == nil {
		return errors.New("label: clipboard unavailable")
	}
	return App.Clipboard().SetText(text)
}

// ConnectSelection observes changed endpoints, including explicit setters.
// Stale notifications stop after a callback changes or releases the label.
func (l *Label) ConnectSelection(fn func(TextSelection)) signal.Handle {
	s := l.selectionState()
	return s.changed.Connect(func(value TextSelection, revision uint64) {
		if !l.Destroyed() && s.revision == revision {
			fn(value)
		}
	})
}

// ConnectCopyError reports clipboard failures from built-in keyboard/menu
// actions. Copy returns its error directly instead.
func (l *Label) ConnectCopyError(fn func(error)) signal.Handle {
	s := l.selectionState()
	return s.copyError.Connect(func(err error, revision uint64) {
		if !l.Destroyed() && s.revision == revision {
			fn(err)
		}
	})
}

func (l *Label) invalidateSelectionGeometry() {
	if s := l.selectState; s != nil {
		s.geometry = nil
	}
}

func (s *labelSelection) interactive() bool {
	l := s.label
	return s.enabled && !l.Destroyed() && IsEnabled(l) && visibleInTree(l) && l.Root() != nil
}

func (s *labelSelection) metrics() *textedit.Geometry {
	l := s.label
	if s.geometry == nil && l.Rect().Width > 0 && l.Rect().Height > 0 {
		if native := l.ensureLayout(l.Rect().Size); native != nil {
			lines, clusters := native.MeasureMetrics()
			g := textedit.NewGeometry(lines, clusters, len(l.PlainText()), textLineHeight(l.resolvedTextFormat().Font.Size))
			s.geometry = &g
		}
	}
	return s.geometry
}

func (s *labelSelection) snap(offset int) int {
	offset = textedit.ClampOffset(s.label.PlainText(), offset)
	if g := s.metrics(); g != nil {
		offset = g.Snap(offset, false)
	}
	return offset
}

func (s *labelSelection) set(value TextSelection, upstream bool) {
	l := s.label
	affinityChanged := s.upstream != upstream
	s.upstream = upstream
	if s.value == value {
		if affinityChanged {
			l.RequestPaint()
		}
		return
	}
	s.value = value
	s.revision++
	l.RequestPaint()
	l.requestSemanticUpdate()
	s.changed.Emit(value, s.revision)
}

// Normalize before interaction/copy, never from Measure or Paint. A callback
// may replace text, disable, unmount or destroy the label while being notified.
func (s *labelSelection) normalize() bool {
	l := s.label
	text, root, enabled := l.Text(), l.Root(), l.enabledRevision
	next := TextSelection{Anchor: s.snap(s.value.Anchor), Caret: s.snap(s.value.Caret)}
	revision := s.revision
	if next != s.value {
		revision++
	}
	s.set(next, s.upstream)
	return !l.Destroyed() && s.enabled && s.revision == revision && l.Text() == text && l.Root() == root && l.enabledRevision == enabled
}

func (s *labelSelection) move(pos textedit.Position, extend bool) {
	value := s.value
	value.Caret = pos.Offset
	if !extend {
		value.Anchor = pos.Offset
	}
	s.set(value, pos.Upstream)
}

func (s *labelSelection) cancel() {
	s.revision++
	if s.controller != nil {
		s.controller.resetGesture()
	}
	if s.context != nil {
		s.context.Reset()
	}
	if s.language != nil {
		s.language.Disconnect()
		s.language = nil
	}
	menu := s.menu
	s.menu = nil
	if menu != nil && menu.popover != nil {
		menu.popover.Destroy()
	}
}

func (s *labelSelection) copyFromInput() {
	if !s.interactive() || !s.normalize() {
		return
	}
	revision := s.revision
	err := s.copyText()
	if err != nil && s.interactive() && s.revision == revision {
		s.copyError.Emit(err, revision)
	}
}

func (s *labelSelection) showMenu(point geometry.Point) {
	if !s.interactive() || !s.normalize() {
		return
	}
	s.cancel()
	if !s.interactive() {
		return
	}
	revision := s.revision
	menu := NewPopoverMenu(s.label)
	s.menu = menu
	items := NewMenu()
	copyItem := items.Append(textCopy.menuLabel(), func() {
		if s.menu == menu && s.revision == revision {
			s.copyFromInput()
		}
	})
	copyItem.SetEnabled(s.value.Anchor != s.value.Caret)
	items.Append(textSelectAll.menuLabel(), func() {
		if s.menu == menu && s.revision == revision && s.interactive() {
			s.label.SelectAll()
		}
	}).SetEnabled(s.label.PlainText() != "")
	menu.SetMenu(items)
	s.language = mlang.ConnectChanged(func() {
		// Language notifications can arrive outside the GUI thread.
		if App != nil {
			App.Post(func() {
				if s.menu == menu {
					s.cancel()
				}
			})
		}
	})
	// Showing a menu is best effort; clipboard errors have a dedicated signal.
	_ = menu.ShowAt(point)
}

func (s *labelSelection) ink(part string) color.Color {
	l := s.label
	state := l.State()
	field := func(v style.Style) (color.Color, bool) {
		if part == "selection" {
			return v.BackgroundColor()
		}
		return v.ForegroundColor()
	}
	if name := l.StyleName(); name != "" && name != styleNameLabel {
		if ink, ok := field(ResolveWidgetStyle(l, name, part, state)); ok {
			return ink
		}
	}
	ink, _ := field(ResolveWidgetStyle(l, styleNameLabel, part, state))
	return ink
}

func (s *labelSelection) paint(p Painter) {
	g := s.metrics()
	if g == nil {
		return
	}
	if s.value.Anchor != s.value.Caret {
		if bg := s.ink("selection"); bg != nil {
			for _, rect := range g.Selection(s.value.Range()) {
				p.FillRect(rect, graphics.ColorOf(bg))
			}
		}
	} else if s.label.Focused() && IsEnabled(s.label) {
		if ink := s.ink("caret"); ink != nil {
			r := g.Caret(textedit.Position{Offset: g.Snap(s.value.Caret, false), Upstream: s.upstream})
			r.Width = 1
			p.FillRect(r, graphics.ColorOf(ink))
		}
	}
}

type labelSelectionController struct {
	EventControllerBase
	state     *labelSelection
	gesture   *GestureParticipation
	dragging  bool
	lastDown  time.Time
	lastPoint geometry.Point
	now       func() time.Time
	desiredX  float32
	vertical  bool
}

func (c *labelSelectionController) resetGesture() {
	gesture := c.gesture
	c.gesture, c.dragging, c.lastDown = nil, false, time.Time{}
	c.vertical = false
	gesture.Reject()
}
func (c *labelSelectionController) Reset()                 { c.state.cancel() }
func (c *labelSelectionController) CapturingPointer() bool { return c.dragging }
func (c *labelSelectionController) GestureCanceled(GestureCancelReason) {
	c.gesture, c.dragging = nil, false
	c.lastDown = time.Time{}
}
func (c *labelSelectionController) GestureAccepted(ctx EventContext) {
	s := c.state
	if !s.interactive() || !s.normalize() {
		return
	}
	e := ctx.Event().(events.PointerEvent)
	point, ok := ctx.Position()
	g := s.metrics()
	if !ok || g == nil {
		return
	}
	pos := g.Hit(point)
	now := c.now()
	double := !c.lastDown.IsZero() && now.Sub(c.lastDown) <= gestureClickInterval && !gestureMoved(point, c.lastPoint, gestureDefaultDistance)
	c.lastDown, c.lastPoint, c.dragging, c.vertical = now, point, true, false
	ctx.StopPropagation()
	if double {
		c.lastDown = time.Time{}
		r := textedit.WordAt(s.label.PlainText(), pos.Offset)
		s.set(TextSelection{Anchor: g.Snap(r.Start, false), Caret: g.Snap(r.End, true)}, true)
	} else {
		s.move(pos, e.Modifiers&events.ModifierShift != 0)
	}
}

func (c *labelSelectionController) HandleEvent(ctx EventContext) {
	s := c.state
	if !s.interactive() {
		return
	}
	switch e := ctx.Event().(type) {
	case events.PointerEvent:
		switch e.EventType {
		case events.PointerDown:
			if primaryPointer(e) {
				if p := JoinGesture(ctx); p != nil {
					c.gesture = p
					p.Claim()
				}
			}
		case events.PointerMove:
			if c.dragging {
				if point, ok := ctx.Position(); ok && s.normalize() {
					if gestureMoved(point, c.lastPoint, gestureDefaultDistance) {
						c.lastDown = time.Time{}
					}
					if g := s.metrics(); g != nil {
						ctx.StopPropagation()
						s.move(g.Hit(point), true)
					}
				}
			}
		case events.PointerUp:
			if e.Button == events.PointerButtonLeft && c.dragging {
				c.gesture, c.dragging = nil, false
				ctx.StopPropagation()
			}
		}
	case events.KeyEvent:
		if e.EventType == events.KeyDown && c.key(e) {
			e.PreventDefault()
			ctx.StopPropagation()
		}
	}
}

func (c *labelSelectionController) key(e events.KeyEvent) bool {
	s := c.state
	shift := e.Modifiers&events.ModifierShift != 0
	command := textCommand(e.Modifiers)
	if command && e.Key == events.KeyC {
		s.copyFromInput()
		return true
	}
	if command && e.Key == events.KeyA {
		s.label.SelectAll()
		return true
	}
	if e.Modifiers&^events.ModifierShift != 0 && !(command && (e.Key == events.KeyHome || e.Key == events.KeyEnd)) {
		return false
	}
	switch e.Key {
	case events.KeyEscape, events.KeyArrowLeft, events.KeyArrowRight, events.KeyArrowUp, events.KeyArrowDown, events.KeyHome, events.KeyEnd:
	default:
		return false
	}
	if !s.normalize() {
		return true
	}
	g := s.metrics()
	if g == nil {
		return false
	}
	pos := textedit.Position{Offset: s.value.Caret, Upstream: s.upstream}
	if e.Key != events.KeyArrowUp && e.Key != events.KeyArrowDown {
		c.vertical = false
	}
	switch e.Key {
	case events.KeyEscape:
		c.resetGesture()
		s.move(pos, false)
	case events.KeyArrowLeft, events.KeyArrowRight:
		right := e.Key == events.KeyArrowRight
		if !shift && s.value.Anchor != s.value.Caret {
			// Collapse toward the visual edge, including RTL selections.
			a := textedit.Position{Offset: s.value.Anchor}
			ar, cr := g.Caret(a), g.Caret(pos)
			if (ar.Y < cr.Y || ar.Y == cr.Y && ar.X < cr.X) != right {
				pos = a
			}
		} else if next, ok := g.Visual(pos, right); ok {
			pos = next
		}
		s.move(pos, shift)
	case events.KeyArrowUp, events.KeyArrowDown:
		if !c.vertical {
			c.desiredX = g.Caret(pos).X
			c.vertical = true
		}
		line := g.LineIndex(pos)
		if e.Key == events.KeyArrowUp {
			line--
		} else {
			line++
		}
		if line >= 0 && line < g.LineCount() {
			s.move(g.HitLine(line, c.desiredX), shift)
		}
	case events.KeyHome, events.KeyEnd:
		if command {
			pos = textedit.Position{}
			if e.Key == events.KeyEnd {
				pos = textedit.Position{Offset: len(s.label.PlainText()), Upstream: true}
			}
		} else {
			pos = g.LineEdge(g.LineIndex(pos), e.Key == events.KeyEnd)
		}
		s.move(pos, shift)
	}
	return true
}
