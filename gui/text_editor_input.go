package gui

import (
	"runtime"
	"strings"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui/textedit"
	"github.com/golang-gui/goui/platform/events"
)

func (t *textEditor) imContext() IMContext {
	if t.readOnly {
		return nil
	}
	return t.im
}

// textEditController only interprets events delivered by EventDispatcher; it
// does not own hit testing, focus routing or a separate input dispatch path.
type textEditController struct {
	EventControllerBase
	view      *textEditor
	dragging  bool
	lastDown  time.Time
	lastPoint geometry.Point
	dragPoint geometry.Point
}

func (c *textEditController) CapturingPointer() bool { return c.dragging }
func (c *textEditController) Reset() {
	c.dragging = false
	c.lastDown = time.Time{}
	if c.view.scrollTimer != nil {
		c.view.scrollTimer.Stop()
	}
}

func (c *textEditController) HandleEvent(ctx EventContext) {
	t := c.view
	if t.owner.base().destroyed {
		return
	}
	switch event := ctx.Event().(type) {
	case events.WheelEvent:
		// User browsing supersedes a pending keyboard reveal; the parent
		// ScrollView still receives and applies the wheel event normally.
		t.revealCaret = false
		t.anchorPending = false
	case events.KeyEvent:
		if event.EventType == events.KeyDown && t.editingKey(event) {
			event.PreventDefault()
			ctx.StopPropagation()
		} else if event.EventType == events.KeyDown && textInputKey(event) {
			// Reserve text for the focused editor, but allow the native text/IME
			// path to commit it. Capture handlers can explicitly override this.
			ctx.StopPropagation()
		}
	case events.PointerEvent:
		point, ok := ctx.Position()
		if !ok {
			return
		}
		switch event.EventType {
		case events.PointerDown:
			if event.Button != events.PointerButtonLeft {
				return
			}
			t.hasDesiredX = false
			position := t.hitText(point)
			model, epoch := t.model, t.mountEpoch
			t.cancelPreedit(true)
			if t.owner.base().destroyed || t.model != model || t.mountEpoch != epoch {
				return
			}
			now := time.Now()
			double := !c.lastDown.IsZero() && now.Sub(c.lastDown) <= 500*time.Millisecond &&
				abs32(point.X-c.lastPoint.X) <= 4 && abs32(point.Y-c.lastPoint.Y) <= 4
			c.lastDown, c.lastPoint, c.dragging = now, point, true
			c.dragPoint = point
			ctx.StopPropagation()
			if double {
				c.lastDown = time.Time{}
				rng := t.wordAt(position.Offset)
				t.setSelection(TextSelection{rng.Start, rng.End}, false, true)
			} else {
				t.moveTo(position, event.Modifiers&events.ModifierShift != 0)
			}
		case events.PointerMove:
			if c.dragging {
				c.dragPoint = point
				ctx.StopPropagation()
				t.moveTo(t.hitText(t.dragSelectionPoint()), true)
				if !t.owner.base().destroyed {
					t.syncAutoScroll()
				}
			}
		case events.PointerUp:
			if event.Button == events.PointerButtonLeft && c.dragging {
				c.dragging = false
				t.syncAutoScroll()
				ctx.StopPropagation()
			}
		}
	}
}

func textInputKey(event events.KeyEvent) bool {
	key := event.Key
	if key != events.KeySpace && !(key >= events.KeyA && key <= events.KeyBackquote) &&
		!(key >= events.KeyNumpad0 && key <= events.KeyNumpadDecimal) {
		return false
	}
	mods := event.Modifiers &^ events.ModifierShift
	// Ctrl+Alt can be native AltGr text. Preserve it at the text target rather
	// than guessing a keyboard layout or manufacturing a platform AltGr flag.
	return mods == 0 || mods == events.ModifierOption || mods == events.ModifierAltGraph || mods == events.ModifierControl|events.ModifierAlt
}

func textCommand(modifiers events.Modifiers) bool {
	return modifiers&^events.ModifierShift == textCommandModifier()
}

func abs32(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

func (t *textEditor) hitText(point geometry.Point) textedit.Position {
	point.X += t.offset.X - t.padding
	point.Y += t.offset.Y - t.padding
	index := t.heights.At(point.Y)
	if p := t.paragraph(index); p != nil {
		rng := t.displayLineRange(index)
		point.Y -= t.heights.Top(index)
		pos := p.editGeometry(t.lineHeight).Hit(point)
		pos.Offset += rng.Start
		if composition := t.preedit; composition != nil {
			pos.Offset = composition.ModelOffset(pos.Offset, pos.Upstream)
		}
		return pos
	}
	return textedit.Position{Offset: t.selection.Caret, Upstream: t.caretUpstream}
}

func (t *textEditor) moveTo(pos textedit.Position, extend bool) {
	selection := t.selection
	selection.Caret = pos.Offset
	if !extend {
		selection.Anchor = pos.Offset
	}
	t.setSelection(selection, pos.Upstream, true)
}

func (t *textEditor) replaceSelection(text string, atomic bool) {
	if t.owner.base().destroyed || t.readOnly || t.suspended {
		return
	}
	model, epoch := t.model, t.mountEpoch
	t.cancelPreedit(true)
	if t.owner.base().destroyed || t.model != model || t.mountEpoch != epoch {
		return
	}
	if !t.normalizeEditingSelection() {
		return
	}
	rng := t.selection.Range()
	before := t.selection
	t.revealCaret = true
	text = t.normalizeInput(text)
	_ = model.Apply(TextEdit{Range: rng, Text: text}, textedit.EditOptions{Atomic: atomic, Source: t.editOwner, Selection: &before})
	// Signals may edit again, replace the model, unmount, or destroy the view.
	// The model change handler has already mapped the selection in order.
	if t.owner.base().destroyed || t.model != model || t.mountEpoch != epoch {
		return
	}
	t.caretUpstream = false
	t.hasDesiredX = false
	t.revealCaret = true
	t.owner.RequestLayout()
}

func (t *textEditor) logicalAdjacent(offset int, forward bool) int {
	index := t.model.LineAt(offset)
	rng, _ := t.model.LineRange(index)
	if forward && offset == rng.End && offset < t.model.Len() {
		return offset + 1
	}
	if !forward && offset == rng.Start && offset > 0 {
		return offset - 1
	}
	if p := t.paragraph(index); p != nil {
		return rng.Start + p.editGeometry(t.lineHeight).Adjacent(offset-rng.Start, forward)
	}
	return offset // Never guess a cluster boundary from rune counts.
}

func (t *textEditor) horizontal(right bool) textedit.Position {
	index := t.model.LineAt(t.selection.Caret)
	rng, _ := t.model.LineRange(index)
	pos := textedit.Position{Offset: t.selection.Caret - rng.Start, Upstream: t.caretUpstream}
	if p := t.paragraph(index); p != nil {
		if next, ok := p.editGeometry(t.lineHeight).Visual(pos, right); ok {
			next.Offset += rng.Start
			return next
		}
	}
	if right && index+1 < t.model.LineCount() {
		return textedit.Position{Offset: rng.End + 1, Upstream: false}
	}
	if !right && index > 0 {
		return textedit.Position{Offset: rng.Start - 1, Upstream: true}
	}
	pos.Offset += rng.Start
	return pos
}

// Collapse toward the visual endpoint, not the smaller/larger byte offset.
// Endpoint affinity faces the selected text, so an embedded RTL run collapses
// onto that run's edge rather than the coincident boundary of surrounding LTR.
// Different paragraphs are ordered vertically; only a shared endpoint paragraph
// needs layout. Never walk or shape the intervening selected document.
func (t *textEditor) selectionEdge(right, word bool) textedit.Position {
	rng := t.selection.Range()
	first, last := textedit.Position{Offset: rng.Start, Upstream: false}, textedit.Position{Offset: rng.End, Upstream: true}
	index := t.model.LineAt(rng.Start)
	if !word && index == t.model.LineAt(rng.End) {
		if p := t.paragraph(index); p != nil {
			line, _ := t.model.LineRange(index)
			a := p.editGeometry(t.lineHeight).Caret(textedit.Position{Offset: first.Offset - line.Start, Upstream: first.Upstream})
			b := p.editGeometry(t.lineHeight).Caret(textedit.Position{Offset: last.Offset - line.Start, Upstream: last.Upstream})
			if a.Y > b.Y || a.Y == b.Y && a.X > b.X {
				first, last = last, first
			}
		}
	}
	if right {
		return last
	}
	return first
}

func (t *textEditor) vertical(down bool, page bool) textedit.Position {
	pos := textedit.Position{Offset: t.selection.Caret, Upstream: t.caretUpstream}
	rect, ok := t.caretRect()
	if !ok {
		return pos
	}
	if !t.hasDesiredX {
		t.desiredX, t.hasDesiredX = rect.X-t.padding, true
	}
	if page {
		delta := max(rect.Height, t.viewport.Height)
		if !down {
			delta = -delta
		}
		return t.hitText(geometry.Point{X: t.desiredX + t.padding - t.offset.X, Y: rect.Y - t.offset.Y + delta + rect.Height/2})
	}
	index := t.model.LineAt(pos.Offset)
	rng, _ := t.model.LineRange(index)
	p := t.paragraph(index)
	line := p.editGeometry(t.lineHeight).LineIndex(textedit.Position{Offset: pos.Offset - rng.Start, Upstream: pos.Upstream})
	if down {
		line++
	} else {
		line--
	}
	if line < 0 {
		if index == 0 {
			return pos
		}
		index--
		p = t.paragraph(index)
		if p == nil {
			return pos
		}
		line = p.editGeometry(t.lineHeight).LineCount() - 1
	} else if line >= p.editGeometry(t.lineHeight).LineCount() {
		if index+1 == t.model.LineCount() {
			return pos
		}
		index++
		p = t.paragraph(index)
		if p == nil {
			return pos
		}
		line = 0
	}
	rng, _ = t.model.LineRange(index)
	pos = p.editGeometry(t.lineHeight).HitLine(line, t.desiredX)
	pos.Offset += rng.Start
	return pos
}

func (t *textEditor) lineEdge(end, document bool) textedit.Position {
	if document {
		if end {
			return textedit.Position{Offset: t.model.Len(), Upstream: true}
		}
		return textedit.Position{}
	}
	index := t.model.LineAt(t.selection.Caret)
	rng, _ := t.model.LineRange(index)
	p := t.paragraph(index)
	if p == nil {
		return textedit.Position{Offset: rng.Start, Upstream: false}
	}
	i := p.editGeometry(t.lineHeight).LineIndex(textedit.Position{Offset: t.selection.Caret - rng.Start, Upstream: t.caretUpstream})
	pos := p.editGeometry(t.lineHeight).LineEdge(i, end)
	pos.Offset += rng.Start
	return pos
}

func textCommandModifier() events.Modifiers {
	modifier, _ := ModPrimary.Resolve()
	return modifier
}

func (t *textEditor) editingKey(event events.KeyEvent) bool {
	if t.preedit != nil && cancelsTextPreedit(event) {
		model, epoch := t.model, t.mountEpoch
		t.cancelPreedit(true)
		if t.owner.base().destroyed || t.model != model || t.mountEpoch != epoch || event.Key == events.KeyEscape {
			return true
		}
	}
	if t.preedit == nil && (cancelsTextPreedit(event) || event.Key == events.KeyC && textCommand(event.Modifiers)) && !t.normalizeEditingSelection() {
		return true
	}
	shift := event.Modifiers&events.ModifierShift != 0
	command := textCommand(event.Modifiers)
	word := event.Modifiers&events.ModifierControl != 0
	if runtime.GOOS == "darwin" {
		modifier, _ := ModAlt.Resolve()
		word = event.Modifiers&modifier != 0
	}
	if command {
		switch event.Key {
		case events.KeyA:
			t.setSelectionValue(TextSelection{0, t.model.Len()})
			return true
		case events.KeyC:
			t.copySelection(false)
			return true
		case events.KeyX:
			t.copySelection(true)
			return true
		case events.KeyV:
			t.paste()
			return true
		case events.KeyZ:
			if !t.readOnly {
				t.undo(shift)
			}
			return true
		case events.KeyY:
			if !t.readOnly {
				t.undo(true)
			}
			return true
		}
	}
	if event.Key != events.KeyArrowUp && event.Key != events.KeyArrowDown && event.Key != events.KeyPageUp && event.Key != events.KeyPageDown {
		t.hasDesiredX = false
	}
	switch event.Key {
	case events.KeyEnter, events.KeyNumpadEnter:
		if t.singleLine {
			t.submitSignal.Emit()
			return true
		}
		t.replaceSelection("\n", true)
		return true
	case events.KeyTab:
		if t.acceptsTab && !shift && !command {
			t.replaceSelection("\t", false)
			return true
		}
		return false
	case events.KeyBackspace, events.KeyDelete:
		if t.readOnly {
			return true
		}
		rng := t.selection.Range()
		if rng.Start == rng.End {
			forward := event.Key == events.KeyDelete
			next := t.logicalAdjacent(rng.Start, forward)
			if word {
				next = t.wordAdjacent(rng.Start, forward)
			}
			if forward {
				rng.End = next
			} else {
				rng.Start = next
			}
		}
		if rng.Start != rng.End {
			before := t.selection
			model, epoch := t.model, t.mountEpoch
			t.revealCaret = true
			_ = model.Apply(TextEdit{Range: rng}, textedit.EditOptions{Source: t.editOwner, Selection: &before})
			if !t.owner.base().destroyed && !t.suspended && t.model == model && t.mountEpoch == epoch {
				t.revealCaret = true
				t.owner.RequestLayout()
			}
		}
		return true
	case events.KeyArrowLeft, events.KeyArrowRight:
		right := event.Key == events.KeyArrowRight
		var pos textedit.Position
		if !shift && t.selection.Anchor != t.selection.Caret {
			pos = t.selectionEdge(right, word)
		} else if word {
			pos = textedit.Position{Offset: t.wordAdjacent(t.selection.Caret, right), Upstream: false}
		} else {
			pos = t.horizontal(right)
		}
		t.moveTo(pos, shift)
		return true
	case events.KeyArrowUp, events.KeyArrowDown, events.KeyPageUp, events.KeyPageDown:
		if t.singleLine {
			return false
		}
		t.moveTo(t.vertical(event.Key == events.KeyArrowDown || event.Key == events.KeyPageDown, event.Key == events.KeyPageUp || event.Key == events.KeyPageDown), shift)
		return true
	case events.KeyHome, events.KeyEnd:
		t.moveTo(t.lineEdge(event.Key == events.KeyEnd, command), shift)
		return true
	}
	return false
}

// These are commands the platform input method did not consume, not guesses
// about which physical keys an IME should handle. Modifier-only keys preserve
// composition; an explicit editor command finishes its temporary projection.
func cancelsTextPreedit(event events.KeyEvent) bool {
	switch event.Key {
	case events.KeyEscape, events.KeyArrowLeft, events.KeyArrowRight, events.KeyArrowUp, events.KeyArrowDown,
		events.KeyHome, events.KeyEnd, events.KeyPageUp, events.KeyPageDown, events.KeyBackspace, events.KeyDelete,
		events.KeyEnter, events.KeyNumpadEnter, events.KeyTab:
		return true
	case events.KeyA, events.KeyX, events.KeyV, events.KeyZ, events.KeyY:
		return textCommand(event.Modifiers)
	}
	return false
}

func (t *textEditor) undo(redo bool) {
	model, epoch := t.model, t.mountEpoch
	if redo && !model.CanRedo() || !redo && !model.CanUndo() {
		return
	}
	t.revealCaret = true
	var result textedit.HistoryResult
	if redo {
		result = model.RedoWithSelection()
	} else {
		result = model.UndoWithSelection()
	}
	if t.owner.base().destroyed || t.model != model || t.mountEpoch != epoch || !result.Applied || model.Revision() != result.Revision {
		return
	}
	if result.Selection != nil {
		// Font/width may have changed since this history entry was created.
		t.setSelectionValue(*result.Selection)
	} else {
		t.revealCaret = true
		t.owner.RequestLayout()
	}
}

func (t *textEditor) copySelection(cut bool) {
	if App == nil || App.Clipboard() == nil {
		return
	}
	rng := t.selection.Range()
	if rng.Start == rng.End {
		return
	}
	text, err := t.model.Slice(rng)
	if err != nil {
		return
	}
	App.Clipboard().SetText(text)
	if cut && !t.readOnly {
		t.replaceSelection("", true)
	}
}

func (t *textEditor) paste() {
	if t.readOnly || App == nil || App.Clipboard() == nil {
		return
	}
	model, revision, selection, epoch := t.model, t.model.Revision(), t.selection, t.mountEpoch
	inputRevision := t.inputRevision
	App.Clipboard().RequestText(func(text string, ok bool) {
		if !ok || t.owner.base().destroyed || t.suspended || t.readOnly || t.model != model || model.Revision() != revision || t.selection != selection || t.mountEpoch != epoch || t.inputRevision != inputRevision {
			return
		}
		t.replaceSelection(text, true)
	})
}

func (t *textEditor) wordAt(offset int) TextRange {
	index := t.model.LineAt(offset)
	line, _ := t.model.LineRange(index)
	rng := t.model.WordAt(offset)
	if p := t.paragraph(index); p != nil {
		return TextRange{line.Start + p.editGeometry(t.lineHeight).Snap(rng.Start-line.Start, false), line.Start + p.editGeometry(t.lineHeight).Snap(rng.End-line.Start, true)}
	}
	return rng
}

func (t *textEditor) wordAdjacent(offset int, forward bool) int {
	if !forward {
		offset = t.logicalAdjacent(offset, false)
		return t.wordAt(offset).Start
	}
	rng := t.wordAt(offset)
	if rng.End == offset {
		return t.logicalAdjacent(offset, true)
	}
	// Skip adjacent whitespace, but not LF (paragraph navigation is separate).
	index := t.model.LineAt(offset)
	line, _ := t.model.LineRange(index)
	rest, _ := t.model.Slice(TextRange{rng.End, line.End})
	end := rng.End + len(rest) - len(strings.TrimLeft(rest, " \t"))
	if p := t.paragraph(index); p != nil {
		end = line.Start + p.editGeometry(t.lineHeight).Snap(end-line.Start, true)
	}
	return end
}
