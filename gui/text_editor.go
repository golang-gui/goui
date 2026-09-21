package gui

import (
	"strings"
	"time"

	"github.com/goexlib/mathx"
	"github.com/golang-gui/goui/core/colors"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui/textedit"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/style"
)

type textParagraph struct {
	index          int
	textBytes      int
	previous, next *textParagraph
	layout         typography.TextLayout
	measured       bool
	geometry       textedit.Geometry
	geometryValid  bool
	width          float32
	height         float32
	textHeight     float32
}

// editGeometry is independent of size measurement: displaying a paragraph
// does not require its cluster hit tests and visual caret/selection index.
// Editing and viewport anchoring retain the same exact geometry, built once
// on demand for each measured layout.
func (p *textParagraph) editGeometry(emptyHeight float32) *textedit.Geometry {
	if !p.geometryValid {
		var lines []typography.TextLine
		var clusters []typography.TextCluster
		length := 0
		if p.layout != nil {
			lines, clusters = p.layout.MeasureMetrics()
			length = len(p.layout.Text())
		}
		p.geometry = textedit.NewGeometry(lines, clusters, length, emptyHeight)
		p.geometryValid = true
	}
	return &p.geometry
}

// Retain recently used native layouts per editor, not per document or backend.
// The byte budget counts source text, not native memory (which is opaque).
// Both limits apply to retained resources; the active layout range is pinned
// even if a large viewport or a single long paragraph exceeds either budget.
const (
	textParagraphCacheLimit = 512
	textParagraphCacheBytes = 1 << 20
)

type textParagraphCache struct {
	paragraphs     map[int]*textParagraph
	oldest, newest *textParagraph
	textBytes      int
}

func (c *textParagraphCache) touch(p *textParagraph) {
	if c.newest == p {
		return
	}
	c.unlink(p)
	p.previous = c.newest
	if c.newest != nil {
		c.newest.next = p
	} else {
		c.oldest = p
	}
	c.newest = p
}

func (c *textParagraphCache) unlink(p *textParagraph) {
	if p.previous != nil {
		p.previous.next = p.next
	} else if c.oldest == p {
		c.oldest = p.next
	}
	if p.next != nil {
		p.next.previous = p.previous
	} else if c.newest == p {
		c.newest = p.previous
	}
	p.previous, p.next = nil, nil
}

func (c *textParagraphCache) add(index, textBytes int, p *textParagraph) {
	if c.paragraphs == nil {
		c.paragraphs = make(map[int]*textParagraph)
	}
	p.index, p.textBytes = index, textBytes
	c.paragraphs[index] = p
	c.textBytes += textBytes
	c.touch(p)
}

func (c *textParagraphCache) remove(p *textParagraph) {
	c.unlink(p)
	delete(c.paragraphs, p.index)
	c.textBytes -= p.textBytes
	if p.layout != nil {
		p.layout.Destroy()
	}
}

func (c *textParagraphCache) clear() {
	for c.oldest != nil {
		c.remove(c.oldest)
	}
	c.paragraphs = nil
}

func (c *textParagraphCache) trim(start, end int) {
	for p := c.oldest; p != nil && (len(c.paragraphs) > textParagraphCacheLimit || c.textBytes > textParagraphCacheBytes); {
		next := p.next
		if p.index < start || p.index >= end {
			c.remove(p)
		}
		p = next
	}
}

// An edit invalidates only replaced paragraphs. Keep both native resources
// and recency order for unaffected paragraphs, remapping their logical index.
func (c *textParagraphCache) splice(start, removed, inserted int) {
	for p := c.oldest; p != nil; {
		next := p.next
		if p.index >= start && p.index < start+removed {
			c.remove(p)
		}
		p = next
	}
	next := make(map[int]*textParagraph, len(c.paragraphs))
	for index, p := range c.paragraphs {
		if index >= start+removed {
			index += inserted - removed
		}
		p.index = index
		next[index] = p
	}
	c.paragraphs = next
}

// textEditor is private state and operations, not a Widget. Its owner is the
// sole tree node and input target. Each control has independent resources.
type textEditor struct {
	textParagraphCache
	owner             Widget
	singleLine        bool
	model             *TextModel
	modelHandle       signal.Handle
	seenRevision      uint64
	suspended         bool
	selection         TextSelection
	caretUpstream     bool
	readOnly          bool
	acceptsTab        bool
	wrap              WrapMode
	padding           float32
	viewport          geometry.Size
	offset            geometry.Point
	format            typography.TextFormat
	formatValid       bool
	lineHeight        float32
	baseline          float32
	hasBaseline       bool
	layoutWidth       float32
	observedWidth     float32
	heights           textedit.HeightIndex
	changeSignal      signal.Signal1[TextChange]
	submitSignal      signal.Signal0
	selectSignal      signal.Signal1[TextSelection]
	revealSignal      signal.Signal1[geometry.Rectangle]
	revealCaret       bool
	im                IMContext
	input             *textEditController
	desiredX          float32
	hasDesiredX       bool
	mountEpoch        uint64
	editOwner         *textedit.EditSource
	preedit           *textedit.Projection
	resettingIME      bool
	nextCommitAtomic  bool
	inputRevision     uint64
	blinkTimer        *Timer
	scrollTimer       *Timer
	windowFocusHandle signal.Handle
	caretVisible      bool
	anchor            textViewportAnchor
	anchorPending     bool
}

func newTextEditor(owner Widget, singleLine bool) *textEditor {
	t := &textEditor{owner: owner, singleLine: singleLine, wrap: WrapWordChar, padding: 4, lineHeight: 20, editOwner: &textedit.EditSource{}, caretVisible: true}
	if singleLine {
		t.wrap = WrapNone
	}
	t.owner.SetFocusable(true)
	t.owner.SetCursor(CursorText)
	t.setModel(NewTextModel(""))
	t.im = NewIMContext()
	t.im.ConnectCommit(t.onCommit)
	t.im.ConnectPreedit(t.onPreedit)
	t.input = &textEditController{EventControllerBase: NewEventControllerBase(PhaseTarget), view: t}
	t.owner.AddEventController(t.input)
	t.owner.ConnectMount(func() {
		t.mountEpoch++
		t.suspended = false
		t.connectModel()
		t.connectInteractionTimers()
	})
	t.owner.ConnectUnmount(func() {
		t.mountEpoch++
		t.suspended = true
		t.cancelPreedit(true)
		t.input.Reset()
		t.disconnectModel()
		t.releaseParagraphs()
		t.disconnectInteractionTimers()
	})
	t.owner.ConnectFocused(func(focused bool) {
		t.model.BreakUndoGroup()
		if !focused {
			t.cancelPreedit(false)
			t.input.Reset()
		}
		t.syncBlink(true)
	})
	return t
}

// SetModel replaces the document, resets selection and scrolling, and leaves
// the previous model and its history intact. nil creates an empty model.
func (t *textEditor) setModel(model *TextModel) {
	if t.model == model && model != nil {
		return
	}
	if model == nil {
		model = NewTextModel("")
	}
	previous, epoch := t.model, t.mountEpoch
	t.cancelPreedit(true)
	if t.owner.base().destroyed || t.model != previous || t.mountEpoch != epoch {
		return
	}
	oldSelection := t.selection
	if t.model != nil {
		t.model.BreakUndoGroup()
	}
	t.disconnectModel()
	t.model = model
	t.selection = TextSelection{}
	t.caretUpstream = false
	t.nextCommitAtomic = false
	t.hasDesiredX = false
	t.revealCaret = true
	t.offset = geometry.Point{}
	t.anchor, t.anchorPending = textViewportAnchor{}, false
	t.seenRevision = model.Revision()
	t.resetParagraphs()
	t.connectModel()
	t.owner.RequestLayout()
	t.owner.base().requestSemanticUpdate()
	if oldSelection != t.selection {
		t.selectSignal.Emit(t.selection)
	}
}

func (t *textEditor) connectModel() {
	if t.owner.base().destroyed || t.suspended || t.modelHandle != nil {
		return
	}
	if t.seenRevision != t.model.Revision() {
		t.anchor, t.anchorPending = textViewportAnchor{}, false
		t.selection.Anchor = t.clampPosition(t.selection.Anchor)
		t.selection.Caret = t.clampPosition(t.selection.Caret)
		t.resetParagraphs()
		t.seenRevision = t.model.Revision()
	}
	model := t.model
	t.modelHandle = model.ConnectChange(func(change TextChange) {
		if !t.owner.base().destroyed && !t.suspended && t.model == model {
			t.modelChanged(change)
		}
	})
}

func (t *textEditor) disconnectModel() {
	if t.modelHandle != nil {
		t.modelHandle.Disconnect()
		t.modelHandle = nil
	}
}

// SetSelection clamps endpoints to the document and to whole Cluster boundaries
// when typography is available. It does not change shared model content.
func (t *textEditor) setSelectionValue(selection TextSelection) {
	if t.preedit != nil && selection == t.selection {
		return
	}
	model, epoch := t.model, t.mountEpoch
	t.cancelPreedit(true)
	if t.owner.base().destroyed || t.model != model || t.mountEpoch != epoch {
		return
	}
	t.hasDesiredX = false
	selection.Anchor = t.snapPosition(selection.Anchor)
	selection.Caret = t.snapPosition(selection.Caret)
	t.setSelection(selection, false, true)
}

func (t *textEditor) setSelection(selection TextSelection, upstream, reveal bool) {
	changed := t.selection != selection
	if changed || t.caretUpstream != upstream {
		t.inputRevision++
		t.model.BreakUndoGroup()
		t.selection, t.caretUpstream = selection, upstream
		t.owner.RequestPaint()
		t.owner.base().requestSemanticUpdate()
		t.syncBlink(true)
	}
	if reveal {
		t.revealCaret = true
		t.owner.RequestLayout()
	}
	if changed {
		t.selectSignal.Emit(selection)
	}
}

func (t *textEditor) setReadOnly(value bool) {
	if t.readOnly == value {
		return
	}
	t.readOnly = value
	model, epoch := t.model, t.mountEpoch
	t.cancelPreedit(true)
	if t.owner.base().destroyed || t.model != model || t.mountEpoch != epoch {
		return
	}
	if t.owner.Focused() {
		if w, ok := t.owner.Window().(*window); ok {
			w.updateInputMethod(t.owner)
		}
	}
	t.model.BreakUndoGroup()
	t.owner.RequestPaint()
	t.owner.base().requestSemanticUpdate()
}
func (t *textEditor) setAcceptsTab(value bool) { t.acceptsTab = value }
func (t *textEditor) setWrapMode(value WrapMode) {
	if value != WrapNone && value != WrapChar && value != WrapWordChar {
		return
	}
	if t.wrap == value {
		return
	}
	t.wrap = value
	t.retainViewportAnchor()
	t.formatValid = false
	t.resetParagraphs()
	t.owner.RequestLayout()
}
func (t *textEditor) setPadding(value float32) {
	value = normalizeLayoutValue(value)
	if t.padding == value {
		return
	}
	t.padding = value
	t.owner.RequestLayout()
}
func (t *textEditor) connectChange(fn func(TextChange)) signal.Handle {
	return t.changeSignal.Connect(fn)
}
func (t *textEditor) connectSelection(fn func(TextSelection)) signal.Handle {
	return t.selectSignal.Connect(fn)
}
func (t *textEditor) connectScrollIntoView(fn func(geometry.Rectangle)) signal.Handle {
	return t.revealSignal.Connect(fn)
}

func (t *textEditor) modelChanged(change TextChange) {
	// Restore the old paragraph index before applying the model change. A
	// shared model edit invalidates the replacement range of a composition.
	hadPreedit := t.preedit != nil
	t.cancelPreedit(false)
	t.hasDesiredX = false
	oldSelection := t.selection
	if change.Reset {
		t.anchor, t.anchorPending = textViewportAnchor{}, false
		t.selection = TextSelection{}
		t.nextCommitAtomic = false
		t.revealCaret = true
		if t.singleLine {
			t.selection = TextSelection{t.model.Len(), t.model.Len()}
			t.revealCaret = true
		}
		t.offset = geometry.Point{}
		t.resetParagraphs()
	} else {
		for _, edit := range change.Edits {
			if t.anchor.valid {
				t.anchor.offset = textedit.MapOffset(t.anchor.offset, edit)
				t.anchorPending = !t.revealCaret
			}
			t.selection.Anchor = textedit.MapOffset(t.selection.Anchor, edit)
			t.selection.Caret = textedit.MapOffset(t.selection.Caret, edit)
		}
		if change.Revision != t.model.Revision() || t.seenRevision+1 != change.Revision || len(change.Edits) != 1 {
			t.resetParagraphs()
		} else {
			edit := change.Edits[0]
			start := t.model.LineAt(edit.Range.Start)
			added := strings.Count(edit.Text, "\n")
			removed := t.heights.Count() + added - t.model.LineCount()
			if removed < 0 || start+removed >= t.heights.Count() {
				t.resetParagraphs()
			} else {
				t.spliceParagraphs(start, removed+1, added+1)
			}
		}
	}
	t.seenRevision = change.Revision
	if change.Revision == t.model.Revision() {
		t.selection.Anchor = t.clampPosition(t.selection.Anchor)
		t.selection.Caret = t.clampPosition(t.selection.Caret)
	}
	t.caretUpstream = false
	t.syncBlink(true)
	t.owner.RequestLayout()
	t.owner.base().requestSemanticUpdate()
	model := t.model
	epoch := t.mountEpoch
	if hadPreedit {
		t.resetInputMethod()
	}
	if t.owner.base().destroyed || t.model != model || t.mountEpoch != epoch {
		return
	}
	if oldSelection != t.selection {
		t.selectSignal.Emit(t.selection)
	}
	if !t.owner.base().destroyed && !t.suspended && t.model == model && t.mountEpoch == epoch {
		t.changeSignal.Emit(change)
	}
}

func (t *textEditor) clampPosition(offset int) int {
	offset = min(max(0, offset), t.model.Len())
	for offset > 0 && !t.model.IsRuneBoundary(offset) {
		offset--
	}
	return offset
}

func (t *textEditor) snapPosition(offset int) int {
	offset = t.clampPosition(offset)
	t.ensureFormat()
	i := t.model.LineAt(offset)
	rng, _ := t.model.LineRange(i)
	if p := t.paragraph(i); p != nil {
		return rng.Start + p.editGeometry(t.lineHeight).Snap(offset-rng.Start, false)
	}
	return offset
}

// A font change or an edit in another view can change shaping boundaries.
// Correct endpoints before the next input command, without forcing distant
// caret paragraphs to be shaped during a viewport-only style/layout pass.
func (t *textEditor) normalizeEditingSelection() bool {
	model, epoch, revision := t.model, t.mountEpoch, t.model.Revision()
	s := TextSelection{t.snapPosition(t.selection.Anchor), t.snapPosition(t.selection.Caret)}
	if s != t.selection {
		t.setSelection(s, false, false)
	}
	return !t.owner.base().destroyed && !t.suspended && t.model == model && t.mountEpoch == epoch && model.Revision() == revision
}

func (t *textEditor) resolvedStyle(part string) style.Style {
	name := t.owner.StyleName()
	if name == "" {
		name = styleNameTextView
		if t.singleLine {
			name = styleNameTextInput
		}
	}
	state := style.Normal
	if t.owner.Focused() {
		state = style.Focused
	}
	return ResolveStyle(name, part, state)
}

func (t *textEditor) styleChanged() { t.ensureFormat() }

func (t *textEditor) ensureFormat() {
	format := textFormatFromStyle(t.resolvedStyle(style.PartDefault), t.wrap, TextAlignBegin)
	if !t.formatValid || t.format.Font != format.Font || t.format.WrapMode != format.WrapMode {
		t.retainViewportAnchor()
		t.format = format
		t.formatValid = true
		t.lineHeight = max(1, textLineHeight(format.Font.Size))
		t.baseline, t.hasBaseline = 0, false
		if App != nil && App.Typography() != nil {
			if sample, err := App.Typography().NewTextLayout(textInputHeightSample, format, textInputMeasureExtent, textInputMeasureExtent); err == nil {
				_, height := sample.MeasureSize()
				t.lineHeight = max(t.lineHeight, height)
				if t.singleLine && height > 0 {
					t.lineHeight = height
				}
				lines, _ := sample.MeasureMetrics()
				if len(lines) > 0 && height > 0 {
					t.baseline, t.hasBaseline = lines[0].Baseline, true
				}
				sample.Destroy()
			}
		}
		t.resetParagraphs()
	} else if !colors.Equal(t.format.TextColor, format.TextColor) {
		t.format = format
		color := format.TextColor
		if color == nil {
			color = typography.DefaultTextColor()
		}
		for _, p := range t.paragraphs {
			if p.layout != nil && len(p.layout.Text()) > 0 {
				p.layout.SetTextColor(0, len(p.layout.Text()), color)
			}
		}
	}
}

func (t *textEditor) releaseParagraphs() {
	t.textParagraphCache.clear()
}

func (t *textEditor) resetParagraphs() {
	t.releaseParagraphs()
	t.heights.Reset(t.displayLineCount(), t.lineHeight)
	t.observedWidth = 0
}

func (t *textEditor) spliceParagraphs(start, removed, inserted int) {
	t.textParagraphCache.splice(start, removed, inserted)
	t.heights.Splice(start, removed, inserted, t.lineHeight)
}

func (t *textEditor) paragraph(index int) *textParagraph {
	p := t.paragraphs[index]
	if p != nil {
		t.textParagraphCache.touch(p)
	}
	if p != nil && p.measured {
		return p
	}
	if index < 0 || index >= t.displayLineCount() || App == nil || App.Typography() == nil {
		return nil
	}
	width := t.layoutWidth
	if t.wrap == WrapNone || width <= 0 {
		width = textInputMeasureExtent
	}
	if p == nil {
		text := t.displayParagraph(index)
		p = &textParagraph{}
		if text != "" || !t.singleLine {
			var err error
			p.layout, err = App.Typography().NewTextLayout(text, t.format, width, textInputMeasureExtent)
			if err != nil {
				return nil
			}
		}
		t.textParagraphCache.add(index, len(text), p)
	} else if p.layout != nil {
		p.layout.SetSize(width, textInputMeasureExtent)
	}
	var w, h float32
	if p.layout != nil {
		w, h = p.layout.MeasureSize()
	}
	p.geometry, p.geometryValid = textedit.Geometry{}, false
	p.width, p.height, p.textHeight = w, max(t.lineHeight, h), h
	p.measured = true
	t.heights.Set(index, p.height)
	t.observedWidth = max(t.observedWidth, w)
	return p
}

func (t *textEditor) contentSize() geometry.Size {
	return geometry.Size{Width: max(t.viewport.Width, t.observedWidth+2*t.padding+1), Height: t.heights.Total() + 2*t.padding}
}

func (t *textEditor) measure(c layout.Constraint) layout.Measurement {
	if !t.owner.Visible() {
		return layout.Measurement{}
	}
	t.ensureFormat()
	return layout.Measured(t.owner.base().constrain(c, geometry.Size{Width: 320, Height: 6*t.lineHeight + 2*t.padding}))
}

func (t *textEditor) arrange(rect geometry.Rectangle) {
	t.owner.base().Arrange(rect)
	// ScrollView calls LayoutVisible itself with its current offset. Outside a
	// ScrollView the editor still lays out the allocated area, without bars.
	if _, scrolling := t.owner.Parent().(*scrollViewport); !scrolling {
		t.layoutVisible(rect.Size, geometry.Point{})
	}
}

func (t *textEditor) layoutVisible(viewport geometry.Size, offset geometry.Point) {
	if t.owner.base().destroyed {
		return
	}
	t.syncBlink(false)
	t.ensureFormat()
	t.viewport, t.offset = viewport, offset
	width := max(1, viewport.Width-2*t.padding-1)
	if width != t.layoutWidth && t.wrap != WrapNone {
		t.retainViewportAnchor()
		// A new width invalidates measurements, not the immutable paragraph text.
		// Retain native layouts and resize only the paragraphs actually visited.
		t.heights.Reset(t.displayLineCount(), t.lineHeight)
		t.observedWidth = 0
		for _, p := range t.paragraphs {
			p.measured = false
		}
	}
	t.layoutWidth = width
	if viewport.Height <= 0 {
		return
	}
	if t.revealCaret {
		// Explicit navigation wins over a retained browsing/reflow anchor.
		// Otherwise refinement can restore an old top line after the host
		// has already applied the caret request, leaving host and content
		// offsets different with no further scroll left for the host to do.
		t.anchorPending = false
	} else {
		t.restoreViewportAnchor()
	}
	originalOffset := offset
	offset = t.offset
	anchor := t.heights.At(max(0, offset.Y))
	anchorTop := t.heights.Top(anchor)
	start := t.heights.At(max(0, offset.Y-t.padding-viewport.Height))
	end := start
	// Above-viewport measurement participates in the retained text anchor.
	// Preserve that range, but do not synchronously shape another full screen
	// below the viewport on every scrollbar jump.
	const trailingOverscanParagraphs = 4
	trailing := 0
	for end < t.displayLineCount() {
		top := t.heights.Top(end)
		if top > offset.Y-t.padding+2*viewport.Height && end > anchor {
			break
		}
		if top > offset.Y-t.padding+viewport.Height && end > anchor {
			if trailing == trailingOverscanParagraphs {
				break
			}
			trailing++
		}
		t.paragraph(end)
		end++
	}
	t.textParagraphCache.trim(start, end)
	// Measuring the overscan above the viewport must not move its top text.
	if delta := t.heights.Top(anchor) - anchorTop; delta != 0 {
		t.offset.Y = max(0, offset.Y+delta)
	}
	if t.offset.Y != originalOffset.Y {
		// Match SetScrollY/clampScroll independently of the previous offset.
		// Directional floor/ceil would alternate forever around fractional
		// native line metrics; the maximum offset must also be integral.
		t.offset.Y = min(mathx.Floor(max(0, t.contentSize().Height-viewport.Height)),
			mathx.Round(t.offset.Y))
	}
	if t.offset == originalOffset {
		t.anchorPending = false
		t.saveViewportAnchor()
	} else if !t.anchorPending {
		// Overscan measurement can also revise the top paragraph's position,
		// without a model edit or reflow having retained an anchor beforehand.
		t.saveViewportAnchor()
		t.retainViewportAnchor()
	}
	if t.offset != originalOffset {
		model, epoch := t.model, t.mountEpoch
		t.emitReveal(geometry.Rect(t.offset.X, t.offset.Y, viewport.Width, viewport.Height))
		if t.owner.base().destroyed || t.suspended || t.model != model || t.mountEpoch != epoch {
			return
		}
	}
	if t.revealCaret {
		if rect, ok := t.caretRect(); ok {
			// A distant navigation initially targets estimated paragraph
			// heights. Measuring the new viewport can move that target again;
			// retain the request until the actual caret fits, rather than
			// consuming it after the first approximate scroll.
			target := geometry.Rect(rect.X-t.padding, rect.Y-t.padding,
				rect.Width+2*t.padding, rect.Height+2*t.padding)
			extent := t.contentSize()
			// ScrollView clamps to an integer DIP range. A fractional maximum
			// would leave the request pending at an unreachable offset even
			// after the host has scrolled as far as its contract permits.
			x := min(max(0, revealOffset(t.offset.X, viewport.Width, target.X, target.Width)), mathx.Floor(max(0, extent.Width-viewport.Width)))
			y := min(max(0, revealOffset(t.offset.Y, viewport.Height, target.Y, target.Height)), mathx.Floor(max(0, extent.Height-viewport.Height)))
			visible := x == t.offset.X && y == t.offset.Y
			t.revealCaret = !visible
			if !visible {
				t.emitReveal(target)
			}
		}
	}
}

func (t *textEditor) caretRect() (geometry.Rectangle, bool) {
	index := t.displayLineAt(t.displayCaret().Offset)
	p := t.paragraph(index)
	return t.paragraphCaret(index, p)
}

func (t *textEditor) paragraphCaret(index int, p *textParagraph) (geometry.Rectangle, bool) {
	if p == nil {
		return geometry.Rectangle{}, false
	}
	rng := t.displayLineRange(index)
	pos := t.displayCaret()
	pos.Offset -= rng.Start
	return p.editGeometry(t.lineHeight).Caret(pos).Translate(geometry.Point{X: t.padding, Y: t.padding + t.heights.Top(index)}), true
}

func (t *textEditor) paint(p Painter) {
	if !t.owner.Visible() || t.owner.base().destroyed {
		return
	}
	paintStyledBox(p, geometry.Rect(0, 0, t.owner.Rect().Width, t.owner.Rect().Height), t.resolvedStyle(style.PartDefault))
	p.SetClipRect(geometry.Rect(t.padding, t.padding, max(0, t.owner.Rect().Width-2*t.padding), max(0, t.owner.Rect().Height-2*t.padding)))
	selected := t.selection.Range()
	if t.preedit != nil {
		selected = TextRange{}
	}
	selectionColor, hasSelection := t.resolvedStyle("selection").BackgroundColor()
	start := t.heights.At(max(0, t.offset.Y-t.padding))
	for i := start; i < t.displayLineCount(); i++ {
		y := t.padding + t.heights.Top(i) - t.offset.Y
		if y >= t.viewport.Height {
			break
		}
		paragraph := t.paragraphs[i]
		if paragraph == nil {
			continue
		}
		origin := geometry.Point{X: t.padding - t.offset.X, Y: y}
		rng := t.displayLineRange(i)
		if hasSelection && selected.Start < selected.End && selected.Start <= rng.End && selected.End > rng.Start {
			for _, rect := range paragraph.editGeometry(t.lineHeight).Selection(TextRange{selected.Start - rng.Start, selected.End - rng.Start}) {
				p.FillRect(rect.Translate(origin), graphics.ColorOf(selectionColor))
			}
			if selected.Start <= rng.End && selected.End > rng.End && i+1 < t.displayLineCount() {
				rect := paragraph.editGeometry(t.lineHeight).Caret(textedit.Position{Offset: rng.End - rng.Start, Upstream: true})
				rect.Width = max(1, t.lineHeight/3)
				p.FillRect(rect.Translate(origin), graphics.ColorOf(selectionColor))
			}
		}
		if paragraph.layout != nil && paragraph.layout.Text() != "" {
			p.DrawTextLayout(origin, paragraph.layout)
		}
		if composition := t.preedit; composition != nil {
			if color, ok := t.resolvedStyle("caret").ForegroundColor(); ok {
				rangeInLine := TextRange{composition.Replacement().Start - rng.Start, composition.Replacement().Start + len(composition.Text()) - rng.Start}
				for _, rect := range paragraph.editGeometry(t.lineHeight).Selection(rangeInLine) {
					rect.Y += rect.Height - 1
					rect.Height = 1
					p.FillRect(rect.Translate(origin), graphics.ColorOf(color))
				}
			}
		}
	}
	if t.owner.Focused() {
		index := t.displayLineAt(t.displayCaret().Offset)
		if rect, ok := t.paragraphCaret(index, t.paragraphs[index]); ok {
			t.im.SetCaretRect(rect.Translate(geometry.Point{X: -t.offset.X, Y: -t.offset.Y}))
			if c, has := t.resolvedStyle("caret").ForegroundColor(); has && t.caretVisible {
				p.FillRect(rect.Translate(geometry.Point{X: -t.offset.X, Y: -t.offset.Y}), graphics.ColorOf(c))
			}
		}
	}
}

func (t *textEditor) snapshot() WidgetInfo {
	info := t.owner.base().Snapshot()
	info.Role = RoleTextView
	state := &TextEditingInfo{ReadOnly: t.readOnly, Selection: t.selection, Length: t.model.Len()}
	// Bounded, range-labelled visible content, never a copy of the full model.
	start := t.heights.At(max(0, t.offset.Y-t.padding))
	end := t.heights.At(max(0, t.offset.Y+t.viewport.Height-t.padding))
	first := t.displayLineRange(start)
	last := t.displayLineRange(end)
	if p := t.preedit; p != nil {
		first.Start = p.ModelOffset(first.Start, false)
		last.End = p.ModelOffset(last.End, true)
		limit := textedit.ClampOffset(p.Text(), min(len(p.Text()), 4096))
		state.Preedit = &TextPreeditInfo{Replacement: p.Replacement(), Text: strings.Clone(p.Text()[:limit]), Caret: p.Caret(), Length: len(p.Text())}
	}
	limit := min(last.End, first.Start+4096)
	for limit > first.Start && !t.model.IsRuneBoundary(limit) {
		limit--
	}
	state.VisibleRange = TextRange{first.Start, limit}
	state.VisibleText, _ = t.model.Slice(state.VisibleRange)
	info.TextEditing = state
	return info
}

// A viewport anchor records model text, not an absolute Y or a native layout
// object. Its intra-line offset preserves partial clipping of the top line.
// Reflow measures just this paragraph and the new viewport/overscan.
type textViewportAnchor struct {
	offset int
	within float32
	valid  bool
}

func (t *textEditor) retainViewportAnchor() {
	if t.anchor.valid && t.preedit == nil {
		t.anchorPending = true
	}
}

func (t *textEditor) restoreViewportAnchor() {
	if !t.anchorPending || !t.anchor.valid || t.preedit != nil {
		return
	}
	offset := t.clampPosition(t.anchor.offset)
	index := t.model.LineAt(offset)
	p := t.paragraph(index)
	if p == nil {
		return
	}
	rng, _ := t.model.LineRange(index)
	rect := p.editGeometry(t.lineHeight).Caret(textedit.Position{Offset: offset - rng.Start, Upstream: false})
	t.offset.Y = max(0, t.heights.Top(index)+rect.Y+t.anchor.within)
	// ScrollView may lay out twice before applying its queued scroll request.
	// Keep the text anchor until LayoutVisible receives the requested offset.
}

func (t *textEditor) saveViewportAnchor() {
	if t.preedit != nil || t.anchorPending {
		return
	}
	// Text is translated by padding-offset and clipped at padding, so the
	// first visible content coordinate is offset. Sampling offset-padding
	// can retain a preceding line hidden entirely by the content clip.
	index := t.heights.At(max(0, t.offset.Y))
	p := t.paragraphs[index]
	if p == nil {
		return
	}
	y := t.offset.Y - t.heights.Top(index)
	g := p.editGeometry(t.lineHeight)
	if t.anchor.valid && t.model.LineAt(t.anchor.offset) == index {
		rng, _ := t.model.LineRange(index)
		rect := g.Caret(textedit.Position{Offset: t.anchor.offset - rng.Start, Upstream: false})
		// Reflow may put the retained character in the middle of a new line.
		// Do not replace it with that line's start: repeated width changes
		// would then walk the anchor backwards through the paragraph. Keep
		// it while it still explains the viewport, including DIP rounding.
		// Actual scrolling or extent clamping establishes a new anchor below.
		if mathx.Round(t.heights.Top(index)+rect.Y+t.anchor.within) == t.offset.Y {
			return
		}
	}
	line := 0
	for line+1 < g.LineCount() && y >= g.LineMetrics(line).Y+g.LineMetrics(line).Height {
		line++
	}
	metrics := g.LineMetrics(line)
	rng, _ := t.model.LineRange(index)
	t.anchor = textViewportAnchor{offset: rng.Start + metrics.Start, within: y - metrics.Y, valid: true}
}

func (t *textEditor) normalizeInput(text string) string {
	if t.singleLine {
		return textedit.NormalizeSingleLine(text)
	}
	return textedit.Normalize(text)
}

func (t *textEditor) layoutSingleLine(size geometry.Size) {
	if t.owner.base().destroyed || t.suspended {
		return
	}
	t.ensureFormat()
	t.viewport = size
	t.layoutWidth = textInputMeasureExtent
	p := t.paragraph(0)
	if p != nil {
		t.observedWidth = p.width
	}
	center := max(0, (size.Height-2*t.padding-t.lineHeight)/2)
	t.offset.Y = -center
	if p != nil && p.textHeight > 0 {
		if t.hasBaseline {
			// Align the actual glyph baseline to the stable sample baseline;
			// centering each string by its own height moves Latin/CJK text.
			t.offset.Y += p.editGeometry(t.lineHeight).LineMetrics(0).Baseline - t.baseline
		} else {
			t.offset.Y = -max(0, (size.Height-2*t.padding-p.textHeight)/2)
		}
	}
	t.offset.X = min(t.offset.X, max(0, t.contentSize().Width-size.Width))
	if t.revealCaret && size.Width > 0 {
		if rect, ok := t.caretRect(); ok {
			t.emitReveal(geometry.Rect(rect.X-t.padding, rect.Y-t.padding,
				rect.Width+2*t.padding, rect.Height+2*t.padding))
		}
	}
	t.syncBlink(false)
}

// A multiline view asks its ScrollView host; a single-line field owns only
// a horizontal offset. Both paths receive the same caret/drag requests.
func (t *textEditor) emitReveal(rect geometry.Rectangle) {
	if !t.singleLine {
		t.revealSignal.Emit(rect)
		return
	}
	x := revealOffset(t.offset.X, t.viewport.Width, rect.X, rect.Width)
	x = min(max(0, x), max(0, t.contentSize().Width-t.viewport.Width))
	if x != t.offset.X {
		t.offset.X = x
		t.owner.RequestPaint()
	}
	t.revealCaret = false
}

const (
	textCaretInterval  = 500 * time.Millisecond
	textScrollInterval = 40 * time.Millisecond
)

func (t *textEditor) connectInteractionTimers() {
	if w := t.owner.Window(); w != nil && t.windowFocusHandle == nil {
		t.windowFocusHandle = w.ConnectFocus(func(focused bool) {
			if !focused {
				t.input.Reset()
				t.cancelPreedit(true)
			}
			if !t.owner.base().destroyed && !t.suspended {
				t.syncBlink(true)
			}
		})
	}
	t.syncBlink(true)
}

func (t *textEditor) disconnectInteractionTimers() {
	if t.windowFocusHandle != nil {
		t.windowFocusHandle.Disconnect()
		t.windowFocusHandle = nil
	}
	if t.blinkTimer != nil {
		t.blinkTimer.Stop()
		t.blinkTimer = nil
	}
	if t.scrollTimer != nil {
		t.scrollTimer.Stop()
		t.scrollTimer = nil
	}
	t.caretVisible = true
}

func (t *textEditor) interactive() bool {
	if t.owner.base().destroyed || t.suspended || !t.owner.Focused() || t.owner.Root() == nil || !visibleInTree(t.owner) {
		return false
	}
	if w := t.owner.Window(); w != nil {
		return w.Focused()
	}
	return true
}

func (t *textEditor) syncBlink(restart bool) {
	if restart {
		t.caretVisible = true
		t.owner.RequestPaint()
	}
	if !t.interactive() || t.preedit != nil || App == nil {
		if t.blinkTimer != nil {
			t.blinkTimer.Stop()
		}
		return
	}
	if t.blinkTimer == nil {
		t.blinkTimer = App.NewTimer()
		t.blinkTimer.ConnectTimeout(func() {
			if !t.interactive() || t.preedit != nil {
				t.syncBlink(true)
				return
			}
			t.caretVisible = !t.caretVisible
			t.owner.RequestPaint() // No layout, metrics, model change or snapshot invalidation.
		})
	}
	if restart || !t.blinkTimer.Active() {
		// Blink is best effort. After application shutdown a timer cannot start;
		// the editor remains readable with its steady caret.
		if t.blinkTimer.Start(textCaretInterval) != nil {
			t.caretVisible = true
		}
	}
}

func (t *textEditor) dragScrollOffset() geometry.Point {
	step := func(point, extent float32) float32 {
		var distance float32
		if point < t.padding {
			distance = point - t.padding
		} else if point > extent-t.padding {
			distance = point - (extent - t.padding)
		}
		if distance == 0 {
			return 0
		}
		amount := min(max(2, abs32(distance)/4), max(2, t.lineHeight*3))
		if distance < 0 {
			amount = -amount
		}
		return amount
	}
	extent := t.contentSize()
	point := t.input.dragPoint
	next := geometry.Point{
		X: min(max(0, t.offset.X+step(point.X, t.viewport.Width)), max(0, extent.Width-t.viewport.Width)),
		Y: min(max(0, t.offset.Y+step(point.Y, t.viewport.Height)), max(0, extent.Height-t.viewport.Height)),
	}
	if t.singleLine {
		next.Y = t.offset.Y // vertical centering is not a scroll range
	}
	return next
}

func (t *textEditor) syncAutoScroll() {
	if !t.input.dragging || !t.interactive() || t.preedit != nil || App == nil || t.dragScrollOffset() == t.offset {
		if t.scrollTimer != nil {
			t.scrollTimer.Stop()
		}
		return
	}
	if t.scrollTimer == nil {
		t.scrollTimer = App.NewTimer()
		t.scrollTimer.ConnectTimeout(t.autoScroll)
	}
	if !t.scrollTimer.Active() {
		_ = t.scrollTimer.Start(textScrollInterval)
	}
}

func (t *textEditor) autoScroll() {
	if !t.input.dragging || !t.interactive() || t.preedit != nil {
		t.syncAutoScroll()
		return
	}
	model, epoch := t.model, t.mountEpoch
	next := t.dragScrollOffset()
	t.revealCaret = false // Do not reveal the old selection during this scroll.
	t.emitReveal(geometry.Rect(next.X, next.Y, t.viewport.Width, t.viewport.Height))
	if t.owner.base().destroyed || t.suspended || t.model != model || t.mountEpoch != epoch {
		return
	}
	t.moveTo(t.hitText(t.dragSelectionPoint()), true)
	if !t.owner.base().destroyed && !t.suspended && t.model == model && t.mountEpoch == epoch {
		t.syncAutoScroll()
	}
}

func (t *textEditor) dragSelectionPoint() geometry.Point {
	point := t.input.dragPoint
	point.X = min(max(t.padding, point.X), max(t.padding, t.viewport.Width-t.padding-1))
	point.Y = min(max(t.padding, point.Y), max(t.padding, t.viewport.Height-t.padding-1))
	return point
}

func (t *textEditor) displayLineCount() int {
	if t.preedit != nil {
		return t.preedit.LineCount(t.model)
	}
	return t.model.LineCount()
}
func (t *textEditor) displayLineRange(index int) TextRange {
	if t.preedit != nil {
		return t.preedit.LineRange(t.model, index)
	}
	rng, _ := t.model.LineRange(index)
	return rng
}
func (t *textEditor) displayParagraph(index int) string {
	if t.preedit != nil {
		return t.preedit.Paragraph(t.model, index)
	}
	rng, _ := t.model.LineRange(index)
	text, _ := t.model.Slice(rng)
	return text
}
func (t *textEditor) displayLineAt(offset int) int {
	if t.preedit != nil {
		return t.preedit.LineAt(t.model, offset)
	}
	return t.model.LineAt(offset)
}

func (t *textEditor) displayCaret() textedit.Position {
	if p := t.preedit; p != nil {
		return textedit.Position{Offset: p.Replacement().Start + p.Caret(), Upstream: false}
	}
	return textedit.Position{Offset: t.selection.Caret, Upstream: t.caretUpstream}
}

func (t *textEditor) onPreedit(text string, caret int) {
	if t.owner.base().destroyed || t.suspended || t.readOnly || t.resettingIME {
		return
	}
	if text == "" {
		t.cancelPreedit(false)
		return
	}
	if t.preedit == nil && !t.normalizeEditingSelection() {
		return
	}
	// Native caret positions refer to the supplied text, before CRLF/invalid
	// UTF-8 normalization. Convert the prefix along with the text itself.
	caret = len(t.normalizeInput(text[:textedit.ClampOffset(text, caret)]))
	text = t.normalizeInput(text)
	if p := t.preedit; p != nil && p.Text() == text {
		if p.Caret() == caret {
			return
		}
		p.SetCaret(caret)
	} else {
		p := t.preedit
		removed := 0
		if p == nil {
			p = textedit.NewProjection(t.model, t.selection)
			removed = p.LastLine() - p.FirstLine() + 1
			t.model.BreakUndoGroup()
		} else {
			removed = p.ParagraphCount()
		}
		p.Update(text, caret)
		t.preedit = p
		t.spliceParagraphs(p.FirstLine(), removed, p.ParagraphCount())
	}
	t.inputRevision++
	t.nextCommitAtomic = true
	t.syncBlink(true)
	t.revealCaret = true
	t.hasDesiredX = false
	t.owner.RequestLayout()
	t.owner.base().requestSemanticUpdate()
}

func (t *textEditor) onCommit(commit IMCommit) {
	if t.owner.base().destroyed || t.suspended || t.readOnly || t.resettingIME {
		return
	}
	atomic := commit.Composed || t.preedit != nil || t.nextCommitAtomic
	if p := t.preedit; p != nil {
		t.selection = p.Selection()
		t.cancelPreedit(false)
	}
	t.nextCommitAtomic = false
	t.replaceSelection(commit.Text, atomic)
}

// cancelPreedit restores committed content without modifying model/history.
// resetNative can synchronously invoke native callbacks; callers that continue
// working must check their model/mount epoch after this call.
func (t *textEditor) cancelPreedit(resetNative bool) {
	p := t.preedit
	if p == nil {
		return
	}
	t.preedit = nil
	t.inputRevision++
	t.spliceParagraphs(p.FirstLine(), p.ParagraphCount(), p.LastLine()-p.FirstLine()+1)
	t.syncBlink(true)
	t.owner.RequestLayout()
	t.owner.base().requestSemanticUpdate()
	if resetNative {
		t.resetInputMethod()
	}
}

func (t *textEditor) resetInputMethod() {
	if t.im == nil || t.resettingIME {
		return
	}
	t.resettingIME = true
	defer func() { t.resettingIME = false }()
	t.im.Reset()
}
