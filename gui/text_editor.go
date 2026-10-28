package gui

import (
	"log"
	"runtime"
	"strings"
	"time"

	"github.com/golang-gui/goui/core/colors"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui/textedit"
	"github.com/golang-gui/goui/internal/resource"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/mlang"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/style"

	"github.com/goexlib/mathx"
)

type textParagraph struct {
	index          int
	textBytes      int
	previous, next *textParagraph
	layout         typography.TextLayout
	paged          *ParagraphLayout
	measured       bool
	geometry       textedit.Geometry
	geometryValid  bool
	fragment       textedit.Fragment
	measuredSize   geometry.Size
	width          float32
	height         float32
	textOffset     float32
	baseline       float32
	lines          []typography.TextLine
	clusters       []typography.TextCluster
}

// editGeometry is independent of size measurement: displaying a paragraph
// does not require its cluster hit tests and visual caret/selection index.
// Editing and viewport anchoring retain the same exact geometry, built once
// on demand for each measured layout.
func (p *textParagraph) editGeometry(emptyHeight float32) *textedit.Geometry {
	if p.paged != nil {
		return &p.paged.Geometry().Geometry
	}
	if !p.geometryValid {
		var lines []typography.TextLine
		var clusters []typography.TextCluster
		length := 0
		if p.layout != nil {
			lines, clusters = p.lines, p.clusters
			length = len(p.layout.Text())
		}
		if p.fragment.LineCount() == 0 && len(lines) > 0 {
			// Copy ordinary values on first geometry demand. Plain display must
			// not pay for a full snapshot that it never queries.
			// Nonpartitioning metrics retain general Geometry semantics below.
			p.fragment, _ = textedit.NewFragment(textedit.Range{End: length}, p.measuredSize, lines, clusters)
		}
		if p.fragment.LineCount() > 0 {
			p.geometry = p.fragment.Geometry(emptyHeight)
		} else {
			p.geometry = textedit.NewGeometry(lines, clusters, length, emptyHeight)
		}
		p.geometryValid = true
	}
	return &p.geometry
}

func (p *textParagraph) geometryAt(emptyHeight float32, offset int) *textedit.Geometry {
	if p.paged != nil {
		if g := p.paged.GeometryAt(offset); g != nil {
			return &g.Geometry
		}
	}
	return p.editGeometry(emptyHeight)
}

// Retain recently used native layouts per editor, not per document or backend.
// The byte budget counts source text, not native memory (which is opaque).
// Both limits apply to retained resources; the active layout range is pinned
// even if a large viewport or a single long paragraph exceeds either budget.
const (
	textParagraphCacheLimit = 512
	textParagraphCacheBytes = 1 << 20
	textReplacedCacheLimit  = 4
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
	if p.paged != nil {
		p.paged.Destroy()
	}
}

func (c *textParagraphCache) clear() {
	old := c.oldest
	*c = textParagraphCache{}
	for old != nil {
		next := old.next
		old.previous, old.next = nil, nil
		if old.layout != nil {
			old.layout.Destroy()
		}
		if old.paged != nil {
			old.paged.Destroy()
		}
		old = next
	}
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
	var released []*textParagraph
	for p := c.oldest; p != nil; {
		next := p.next
		if p.index >= start && p.index < start+removed {
			c.unlink(p)
			delete(c.paragraphs, p.index)
			c.textBytes -= p.textBytes
			released = append(released, p)
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
	for _, p := range released {
		if p.layout != nil {
			p.layout.Destroy()
		}
		if p.paged != nil {
			p.paged.Destroy()
		}
	}
}

// textEditor is private state and operations, not a Widget. Its owner is the
// sole tree node and input target. Each control has independent resources.
type textEditor struct {
	textParagraphCache
	replacedParagraphs []*textParagraph
	replacedBytes      int

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
	padding           geometry.Insets
	viewport          geometry.Size
	offset            geometry.Point
	format            typography.TextFormat
	formatValid       bool
	lineHeight        float32
	baseline          float32
	hasBaseline       bool
	layoutWidth       float32
	observedWidth     float32
	longWidths        map[int]float32
	longWidth         float32
	heightEstimated   bool
	heights           textedit.HeightIndex
	changeSignal      signal.Signal1[TextChange]
	submitSignal      signal.Signal0
	selectSignal      signal.Signal1[TextSelection]
	revealSignal      signal.Signal1[geometry.Rectangle]
	revealCaret       bool
	revealingCaret    bool
	im                IMContext
	input             *textEditController
	contextMenu       *textEditMenu
	desiredX          float32
	hasDesiredX       bool
	mountEpoch        uint64
	editOwner         *textedit.EditSource
	preedit           *textedit.Projection
	placeholder       string
	placeholderLayout *textPlaceholderLayout
	resettingIME      bool
	nextCommitAtomic  bool
	inputRevision     uint64
	blinkTimer        *Timer
	scrollTimer       *Timer
	resizeTimer       *Timer
	resizeWidth       float32
	resizeReady       bool
	windowFocusHandle signal.Handle
	caretVisible      bool
	anchor            textViewportAnchor
	anchorPending     bool
	longFrame         signal.Handle
	longPending       bool
	longBudget        int
}

func newTextEditor(owner Widget, singleLine bool, padding geometry.Insets) *textEditor {
	t := &textEditor{owner: owner, singleLine: singleLine, wrap: WrapWordChar, padding: padding, lineHeight: 20, editOwner: &textedit.EditSource{}, caretVisible: true, longBudget: 1}
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
	menu := NewContextMenuEventController()
	menu.ConnectRequest(func(ctx EventContext) {
		ctx.StopPropagation()
		if point, ok := ctx.Position(); ok {
			t.showContextMenu(&point)
		} else {
			t.showContextMenu(nil)
		}
	})
	t.owner.AddEventController(menu)
	t.owner.ConnectMount(func() {
		t.mountEpoch++
		t.suspended = false
		t.connectModel()
		t.connectInteractionTimers()
		t.connectContextMenuLanguage()
	})
	t.owner.ConnectUnmount(func() {
		if t.longFrame != nil {
			t.longFrame.Disconnect()
			t.longFrame = nil
		}
		t.longPending = false
		t.longBudget = 1
		t.mountEpoch++
		t.suspended = true
		if t.contextMenu != nil && t.contextMenu.language != nil {
			t.contextMenu.language.Disconnect()
			t.contextMenu.language = nil
		}
		t.hideContextMenu()
		t.cancelPreedit(true)
		t.input.Reset()
		t.disconnectModel()
		t.releaseParagraphs()
		t.releasePlaceholder()
		t.disconnectInteractionTimers()
	})
	t.owner.ConnectFocused(func(focused bool) {
		t.model.BreakUndoGroup()
		if !focused {
			t.hideContextMenu()
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
	t.hideContextMenu()
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
		model, epoch, revision, input := t.model, t.mountEpoch, t.model.Revision(), t.inputRevision
		t.requestCaretReveal()
		// A scroll request can synchronously invoke application code. Do not
		// notify the old selection after a replacement, unmount or nested edit.
		if t.owner.base().destroyed || t.model != model ||
			t.mountEpoch != epoch || model.Revision() != revision || t.inputRevision != input {
			return
		}
	}
	if changed {
		t.updateContextMenu()
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
	t.updateContextMenu()
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
func (t *textEditor) setPadding(value geometry.Insets) {
	value = normalizeInsets(value)
	if t.padding == value {
		return
	}
	t.padding = value
	t.owner.RequestLayout()
}

func (t *textEditor) horizontalPadding() float32 { return t.padding.Horizontal() }
func (t *textEditor) verticalPadding() float32   { return t.padding.Vertical() }
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
	model, epoch, revision := t.model, t.mountEpoch, t.model.Revision()
	current := func() bool {
		return !t.owner.base().destroyed && t.model == model && t.mountEpoch == epoch && t.model.Revision() == revision
	}
	// Restore the old paragraph index before applying the model change. A
	// shared model edit invalidates the replacement range of a composition.
	hadPreedit := t.preedit != nil
	t.cancelPreedit(false)
	if !current() {
		return
	}
	t.hasDesiredX = false
	oldSelection := t.selection
	if change.Reset {
		t.anchor, t.anchorPending = textViewportAnchor{}, false
		t.selection = TextSelection{}
		t.nextCommitAtomic = false
		t.revealCaret = true
		if t.singleLine {
			t.selection = TextSelection{Anchor: t.model.Len(), Caret: t.model.Len()}
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
			} else if p := t.paragraphs[start]; p != nil && p.paged != nil && removed == 0 && added == 0 {
				rng := t.displayLineRange(start)
				local := edit
				local.Range.Start -= rng.Start
				local.Range.End -= rng.Start
				if rng.End-rng.Start <= 64*1024 {
					t.spliceParagraphs(start, 1, 1)
				} else {
					p.paged.Change(rng.End-rng.Start, []textedit.Edit{local})
					p.measured = false
					t.heightEstimated = true
					t.queueLongLayout()
				}
			} else {
				t.spliceParagraphs(start, removed+1, added+1)
			}
		}
	}
	if !current() {
		return
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
	if hadPreedit {
		t.resetInputMethod()
	}
	if t.owner.base().destroyed || t.model != model || t.mountEpoch != epoch {
		return
	}
	t.updateContextMenu()
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
		if p.paged != nil {
			if g := p.paged.GeometryAt(offset - rng.Start); g != nil {
				return rng.Start + g.Snap(offset-rng.Start, false)
			}
			t.queueLongLayout()
			return offset
		}
		return rng.Start + p.editGeometry(t.lineHeight).Snap(offset-rng.Start, false)
	}
	return offset
}

// A font change or an edit in another view can change shaping boundaries.
// Correct endpoints before the next input command, without forcing distant
// caret paragraphs to be shaped during a viewport-only style/layout pass.
func (t *textEditor) normalizeEditingSelection() bool {
	model, epoch, revision := t.model, t.mountEpoch, t.model.Revision()
	enabledRevision := t.owner.base().enabledRevision
	s := TextSelection{Anchor: t.snapPosition(t.selection.Anchor), Caret: t.snapPosition(t.selection.Caret)}
	if s != t.selection {
		t.setSelection(s, false, false)
	}
	return !t.owner.base().destroyed && !t.suspended && IsEnabled(t.owner) &&
		t.owner.base().enabledRevision == enabledRevision && t.model == model && t.mountEpoch == epoch && model.Revision() == revision
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
	if !IsEnabled(t.owner) {
		state = style.Disabled
	} else if t.owner.Focused() {
		state = style.Focused
	}
	return ResolveWidgetStyle(t.owner, name, part, state)
}

func (t *textEditor) styleChanged() { t.ensureFormat() }

func (t *textEditor) ensureFormat() {
	format := textFormatFromStyle(t.resolvedStyle(style.PartDefault), t.wrap, TextAlignBegin)
	if !t.formatValid || t.format.Font != format.Font || t.format.WrapMode != format.WrapMode {
		t.retainViewportAnchor()
		t.format = format
		t.formatValid = true
		t.releasePlaceholder()
		t.lineHeight = max(1, textLineHeight(format.Font.Size))
		t.baseline, t.hasBaseline = 0, false
		if App != nil && App.Typography() != nil {
			if sample, err := App.Typography().NewTextLayout(textInputHeightSample, format, textInputMeasureExtent, textInputMeasureExtent); err == nil {
				_, height := sample.MeasureSize()
				t.lineHeight = max(t.lineHeight, height)
				if height > 0 {
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
		model, epoch, revision := t.model, t.mountEpoch, t.model.Revision()
		current := func() bool {
			return !t.owner.base().destroyed && t.model == model && t.mountEpoch == epoch && t.model.Revision() == revision && t.format.Font == format.Font && t.format.WrapMode == format.WrapMode && colors.Equal(t.format.TextColor, format.TextColor)
		}
		t.format = format
		t.releaseReplacedParagraphs()
		if !current() {
			return
		}
		color := format.TextColor
		if color == nil {
			color = typography.DefaultTextColor()
		}
		for _, p := range t.paragraphs {
			if p.paged != nil {
				p.paged.SetColor(color)
				continue
			}
			if p.layout != nil && len(p.layout.Text()) > 0 {
				p.layout.SetTextColor(0, len(p.layout.Text()), color)
				if !current() {
					return
				}
			}
		}
	}
}

func (t *textEditor) releaseParagraphs() {
	t.cancelResize()
	old := t.textParagraphCache
	t.textParagraphCache = textParagraphCache{}
	t.longWidths, t.longWidth = nil, 0
	t.releaseReplacedParagraphs()
	old.clear()
}

func (t *textEditor) releaseReplacedParagraphs() {
	old := t.replacedParagraphs
	t.replacedParagraphs, t.replacedBytes = nil, 0
	for _, p := range old {
		p.layout.Destroy()
	}
}

func (t *textEditor) resetParagraphs() {
	t.releaseParagraphs()
	t.heights.Reset(t.displayLineCount(), t.lineHeight)
	t.observedWidth = 0
	t.heightEstimated = false
}

func (t *textEditor) spliceParagraphs(start, removed, inserted int) {
	t.cancelResize()
	model, epoch, revision := t.model, t.mountEpoch, t.model.Revision()
	// A local edit invalidates text geometry, not the best height estimate.
	// Collapsing a long wrapped paragraph to one line would temporarily hide
	// the scrollbar and measure the replacement at two different widths.
	estimate := t.lineHeight
	if removed == 1 && inserted == 1 {
		estimate = t.heights.Top(start+1) - t.heights.Top(start)
		if p := t.paragraphs[start]; !t.singleLine && p != nil && p.measured && p.layout != nil && p.textBytes > 4096 && p.textBytes <= textParagraphCacheBytes {
			// Retain a few expensive complete inputs for backspace/history.
			// Detach ownership before any native destruction can call back.
			t.retainParagraph(p)
		}
	}
	t.textParagraphCache.splice(start, removed, inserted)
	if t.owner.base().destroyed || t.model != model || t.mountEpoch != epoch || t.model.Revision() != revision {
		return
	}
	t.heights.Splice(start, removed, inserted, estimate)
	if len(t.longWidths) > 0 {
		widths := make(map[int]float32, len(t.longWidths))
		t.longWidth = 0
		for index, width := range t.longWidths {
			if index >= start && index < start+removed {
				continue
			}
			if index >= start+removed {
				index += inserted - removed
			}
			widths[index] = width
			t.longWidth = max(t.longWidth, width)
		}
		t.longWidths = widths
	}
	t.trimReplacedParagraphs()
}

// Recently used complete inputs also include expensive layouts at other
// widths. Retain native resources with their exact metrics: changing the
// native box back would otherwise discard its internal line-breaking result.
func (t *textEditor) retainParagraph(p *textParagraph) {
	t.textParagraphCache.unlink(p)
	delete(t.paragraphs, p.index)
	t.textParagraphCache.textBytes -= p.textBytes
	t.replacedParagraphs = append(t.replacedParagraphs, p)
	t.replacedBytes += p.textBytes
}

func (t *textEditor) trimReplacedParagraphs() {
	model, epoch, revision := t.model, t.mountEpoch, t.model.Revision()
	for len(t.replacedParagraphs) > textReplacedCacheLimit || t.replacedBytes > textParagraphCacheBytes {
		p := t.replacedParagraphs[0]
		t.replacedParagraphs[0] = nil
		t.replacedParagraphs = t.replacedParagraphs[1:]
		t.replacedBytes -= p.textBytes
		p.layout.Destroy()
		if t.owner.base().destroyed || t.model != model || t.mountEpoch != epoch || t.model.Revision() != revision {
			return
		}
	}
}

func (t *textEditor) paragraph(index int) *textParagraph {
	p := t.paragraphs[index]
	if p != nil {
		t.textParagraphCache.touch(p)
	}
	if p != nil && p.measured {
		if p.paged != nil {
			return t.prepareLongParagraph(index, p)
		}
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
		rng := t.displayLineRange(index)
		if !t.singleLine && rng.End-rng.Start > 64*1024 {
			p = &textParagraph{index: index}
			p.paged = NewParagraphLayout(App.Typography(), t.format, width, rng.End-rng.Start, func(local textedit.Range) (string, error) {
				r := t.displayLineRange(p.index)
				local.Start += r.Start
				local.End += r.Start
				if t.preedit != nil {
					return t.preedit.Slice(t.model, local)
				}
				return t.model.Slice(local)
			})
			t.textParagraphCache.add(index, 0, p)
			return t.prepareLongParagraph(index, p)
		}
		text := t.displayParagraph(index)
		for i := len(t.replacedParagraphs) - 1; i >= 0; i-- {
			previous := t.replacedParagraphs[i]
			previousWidth, _ := previous.layout.Size()
			if previousWidth != width || previous.layout.Text() != text {
				continue
			}
			copy(t.replacedParagraphs[i:], t.replacedParagraphs[i+1:])
			last := len(t.replacedParagraphs) - 1
			t.replacedParagraphs[last] = nil
			t.replacedParagraphs = t.replacedParagraphs[:last]
			t.replacedBytes -= previous.textBytes
			t.textParagraphCache.add(index, len(text), previous)
			t.heights.Set(index, previous.height)
			t.observedWidth = max(t.observedWidth, previous.width)
			return previous
		}
		p = &textParagraph{}
		if text != "" || !t.singleLine {
			var err error
			p.layout, err = App.Typography().NewTextLayout(text, t.format, width, textInputMeasureExtent)
			if err != nil {
				return nil
			}
		}
		t.textParagraphCache.add(index, len(text), p)
	} else if p.paged != nil {
		return t.prepareLongParagraph(index, p)
	} else if p.layout != nil {
		p.layout.SetSize(width, textInputMeasureExtent)
	}
	var w, h float32
	if p.layout != nil {
		w, h = p.layout.MeasureSize()
		p.lines, p.clusters = p.layout.MeasureMetrics()
	}
	p.measuredSize = geometry.Size{Width: w, Height: h}
	p.fragment = textedit.Fragment{}
	p.geometry, p.geometryValid = textedit.Geometry{}, false
	p.textOffset, p.baseline = 0, t.baseline
	if len(p.lines) > 0 {
		p.textOffset = max(0, t.baseline-p.lines[0].Baseline)
		p.baseline = p.textOffset + p.lines[0].Baseline
	}
	p.width, p.height = w, max(t.lineHeight, h+p.textOffset)
	p.measured = true
	t.heights.Set(index, p.height)
	t.observedWidth = max(t.observedWidth, w)
	return p
}

func (t *textEditor) queueLongLayout() {
	t.longPending = true
	if root := t.owner.Root(); root != nil {
		if t.longFrame == nil {
			t.longFrame = root.ConnectFrame(func(time.Time) {
				t.longBudget = 1
				if t.longPending {
					t.longPending = false
					t.owner.RequestLayout()
				}
			})
		}
		_ = root.RequestPaint()
	}
}

func (t *textEditor) prepareLongParagraph(index int, p *textParagraph) *textParagraph {
	model, revision, epoch := t.model, t.model.Revision(), t.mountEpoch
	p.paged.SetWidth(t.layoutWidth)
	if t.revealCaret && t.displayLineAt(t.displayCaret().Offset) == index {
		r := t.displayLineRange(index)
		if p.paged.GeometryAt(t.displayCaret().Offset-r.Start) == nil {
			t.queueLongLayout()
		}
	} else if t.anchorPending && t.anchor.valid && t.model.LineAt(t.anchor.offset) == index {
		r := t.displayLineRange(index)
		if p.paged.GeometryAt(t.anchor.offset-r.Start) == nil {
			t.queueLongLayout()
		}
	}
	viewport := geometry.Rect(t.offset.X, max(0, t.offset.Y-t.padding.Top-t.heights.Top(index)), t.viewport.Width, max(t.lineHeight, t.viewport.Height))
	ready := p.paged.Prepare(viewport, t.longBudget > 0 || t.owner.Root() == nil)
	if t.longBudget > 0 {
		t.longBudget--
	}
	if t.owner.base().destroyed || t.model != model || t.model.Revision() != revision || t.mountEpoch != epoch || t.paragraphs[index] != p || p.paged.Err() != nil {
		return nil
	}
	size := p.paged.Size()
	p.width, p.height = size.Width, size.Height
	p.measured = ready
	p.baseline = t.baseline
	t.heights.Set(index, p.height)
	t.setLongWidth(index, p.width)
	if !ready {
		t.queueLongLayout()
	}
	if p.paged.VisibleRange().End == 0 {
		return nil
	}
	return p
}

// Long-paragraph extents include replaceable estimates. Keep scalar widths
// beyond native-cache eviction, but allow a measured correction to shrink the
// current maximum. Ordinary paragraph measurements retain their existing cache.
func (t *textEditor) setLongWidth(index int, width float32) {
	if t.longWidths == nil {
		t.longWidths = make(map[int]float32)
	}
	previous := t.longWidths[index]
	t.longWidths[index] = width
	if previous == t.longWidth && width < previous {
		t.longWidth = 0
		for _, value := range t.longWidths {
			t.longWidth = max(t.longWidth, value)
		}
	} else {
		t.longWidth = max(t.longWidth, width)
	}
}

func (t *textEditor) contentSize() geometry.Size {
	width := max(t.viewport.Width, max(t.observedWidth, t.longWidth)+t.horizontalPadding()+1)
	if t.resizeWidth != 0 {
		// A temporarily retained wider layout must not introduce a horizontal
		// scrollbar merely because its replacement width is still pending.
		width = t.viewport.Width
	}
	return geometry.Size{Width: width, Height: t.heights.Total() + t.verticalPadding()}
}

func (t *textEditor) measure(c layout.Constraint) layout.Measurement {
	if !t.owner.Visible() {
		return layout.Measurement{}
	}
	t.ensureFormat()
	b := t.owner.base()
	width := b.constrain(c, geometry.Size{Width: 320}).Width
	length := t.model.Len()
	if t.preedit != nil {
		r := t.preedit.Replacement()
		length += len(t.preedit.Text()) - (r.End - r.Start)
	}
	if t.displayLineCount() > 64 || length > 4096 {
		t.estimateWrappedHeight(max(1, width-t.horizontalPadding()-1))
		// Natural sizing may be unbounded (for example a LinearBox's first
		// pass), and its cross width may include a ScrollView's scrollbar.
		// Neither query changes the allocated text width or demands exact
		// geometry for the entire document. LayoutVisible owns that work.
		baseline := t.baseline
		if p := t.paragraphs[0]; p != nil && p.measured {
			baseline = p.baseline
		}
		return layout.Measurement{Size: b.constrain(c, geometry.Size{Width: width, Height: t.heights.Total() + t.verticalPadding()}),
			Baseline: t.padding.Top + baseline, HasBaseline: t.hasBaseline}
	}
	if !t.prepareWidth(max(1, width-t.horizontalPadding()-1)) {
		return layout.Measurement{}
	}
	limit := b.layoutConstraint(c).Max.Height
	height := t.verticalPadding()
	baseline := t.baseline
	for i := 0; i < t.displayLineCount(); i++ {
		p := t.paragraph(i)
		if p != nil {
			height += p.height
			if i == 0 {
				baseline = p.baseline
			}
		} else {
			height += t.lineHeight
		}
		t.textParagraphCache.trim(-1, -1)
		if height >= limit {
			break
		}
	}
	return layout.Measurement{Size: b.constrain(c, geometry.Size{Width: width, Height: height}),
		Baseline: t.padding.Top + baseline, HasBaseline: t.hasBaseline}
}

// A bounded sample supplies only a provisional scrolling extent. It never
// supplies editing or rendering geometry. Starting a long wrapped input at one
// line high would first lay out the whole paragraph without a scrollbar, then
// repeat it at the scrollbar-reduced width. Exact paragraph metrics replace
// this estimate during visible layout, including for atypical/zero-width text.
func (t *textEditor) estimateWrappedHeight(width float32) {
	if t.heightEstimated || t.singleLine || t.wrap == WrapNone || t.preedit != nil || t.model.LineCount() != 1 ||
		t.model.Len() <= 4096 || t.heights.Total() != t.lineHeight || App == nil || App.Typography() == nil {
		return
	}
	end := min(512, t.model.Len())
	for !t.model.IsRuneBoundary(end) {
		end--
	}
	text, err := t.model.Slice(TextRange{End: end})
	if err != nil {
		return
	}
	model, epoch, revision := t.model, t.mountEpoch, t.model.Revision()
	sample, err := App.Typography().NewTextLayout(text, t.format, width, textInputMeasureExtent)
	if err != nil {
		return
	}
	w, h := sample.MeasureSize()
	sample.Destroy()
	if t.owner.base().destroyed || t.model != model || t.mountEpoch != epoch || model.Revision() != revision {
		return
	}
	ratio := float32(t.model.Len()) / float32(end)
	height := h * ratio
	if h <= t.lineHeight {
		height = mathx.Ceil(w*ratio/width) * t.lineHeight
	}
	t.heights.Set(0, max(t.lineHeight, height))
	t.heightEstimated = true
}

// Width-sensitive paragraph measurements are shared by Measure and visible
// layout. This never changes the viewport or scroll offset.
func (t *textEditor) prepareWidth(width float32) bool {
	model, epoch, revision := t.model, t.mountEpoch, t.model.Revision()
	if width != t.layoutWidth && t.wrap != WrapNone {
		t.retainViewportAnchor()
		// Keep previous measured heights as estimates until their paragraphs
		// are prepared for the new width. Resetting a wrapped paragraph to one
		// line hides the scrollbar and then brings it back, changing the width
		// a second time during the same allocation.
		t.observedWidth = 0
		t.longWidths, t.longWidth = nil, 0
		for _, p := range t.paragraphs {
			if p.measured && p.layout != nil && p.textBytes > 4096 && p.textBytes <= textParagraphCacheBytes {
				t.retainParagraph(p)
			} else {
				p.measured = false
			}
		}
	}
	t.layoutWidth = width
	t.trimReplacedParagraphs()
	return !t.owner.base().destroyed && t.model == model && t.mountEpoch == epoch && model.Revision() == revision && t.layoutWidth == width
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
	width := max(1, viewport.Width-t.horizontalPadding()-1)
	if t.deferResize(width) {
		width = t.layoutWidth
	}
	if !t.prepareWidth(width) {
		return
	}
	beforeSize := t.contentSize()
	defer func() {
		if !t.owner.base().destroyed && t.contentSize() != beforeSize {
			t.owner.RequestLayout()
		}
	}()
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
	start := t.heights.At(max(0, offset.Y-t.padding.Top-viewport.Height))
	end := start
	// Above-viewport measurement participates in the retained text anchor.
	// Preserve that range, but do not synchronously shape another full screen
	// below the viewport on every scrollbar jump.
	const trailingOverscanParagraphs = 4
	trailing := 0
	for end < t.displayLineCount() {
		top := t.heights.Top(end)
		if top > offset.Y-t.padding.Top+2*viewport.Height && end > anchor {
			break
		}
		if top > offset.Y-t.padding.Top+viewport.Height && end > anchor {
			if trailing == trailingOverscanParagraphs {
				break
			}
			trailing++
		}
		t.paragraph(end)
		end++
	}
	t.textParagraphCache.trim(start, end)
	// A thumb dragged to the current right edge must still reach the text's
	// end when measuring its final window corrects the estimated width.
	// Ordinary browsing retains its offset; this only follows an existing
	// far-edge position during this layout's extent refinement.
	if !t.revealCaret && offset.X > 0 && beforeSize.Width != t.contentSize().Width &&
		offset.X >= mathx.Floor(scrollExtent(beforeSize.Width, viewport.Width)) {
		t.offset.X = mathx.Floor(scrollExtent(t.contentSize().Width, viewport.Width))
	}
	// Measuring the overscan above the viewport must not move its top text.
	if delta := t.heights.Top(anchor) - anchorTop; delta != 0 {
		t.offset.Y = max(0, offset.Y+delta)
	}
	if t.offset.Y != originalOffset.Y {
		// Match SetScrollY/clampScroll independently of the previous offset.
		// Directional floor/ceil would alternate forever around fractional
		// native line metrics; the maximum offset must also be integral.
		t.offset.Y = min(mathx.Floor(scrollExtent(t.contentSize().Height, viewport.Height)),
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
			t.revealCaretRect(rect)
		}
	}
}

// Selection does not change the requested size. Reuse current paragraph
// geometry without invalidating the widget/ancestor measurement caches.
// Missing or width-invalid measurements still use the normal layout path.
func (t *textEditor) requestCaretReveal() {
	t.revealCaret = true
	t.anchorPending = false
	t.ensureFormat()
	p := t.paragraphs[t.displayLineAt(t.displayCaret().Offset)]
	if t.revealingCaret || t.viewport.Width <= 0 || t.viewport.Height <= 0 ||
		p == nil || !p.measured ||
		t.wrap != WrapNone && t.layoutWidth != max(1, t.viewport.Width-t.horizontalPadding()-1) {
		t.owner.RequestLayout()
		return
	}
	if rect, ok := t.paragraphCaret(p.index, p); ok {
		t.revealCaretRect(rect)
	} else if p.paged != nil && p.paged.Err() == nil {
		// A keyboard destination may lie outside the retained working set.
		// Preparing it belongs to later frames, not this input event or Paint.
		t.queueLongLayout()
		t.owner.RequestLayout()
	}
}

func (t *textEditor) revealCaretRect(rect geometry.Rectangle) {
	target := geometry.Rect(rect.X-t.padding.Left, rect.Y-t.padding.Top,
		rect.Width+t.horizontalPadding(), rect.Height+t.verticalPadding())
	// A nested SetSelection from a reveal listener must defer to layout rather
	// than recursively emit another request. LayoutVisible itself can refine
	// the target synchronously while ScrollView processes this request.
	previous := t.revealingCaret
	t.revealingCaret = true
	defer func() { t.revealingCaret = previous }()
	if t.singleLine {
		t.emitReveal(target)
		return
	}
	// A distant navigation initially targets estimated paragraph heights.
	// Retain it until measuring the new viewport puts the actual caret in view.
	// Match ScrollView's integral range, including fractional native metrics.
	extent, viewport := t.contentSize(), t.viewport
	x := min(max(0, revealOffset(t.offset.X, viewport.Width, target.X, target.Width)), mathx.Floor(scrollExtent(extent.Width, viewport.Width)))
	y := min(max(0, revealOffset(t.offset.Y, viewport.Height, target.Y, target.Height)), mathx.Floor(scrollExtent(extent.Height, viewport.Height)))
	visible := x == t.offset.X && y == t.offset.Y
	t.revealCaret = !visible
	if !visible {
		t.emitReveal(target)
	}
}

func (t *textEditor) caretRect() (geometry.Rectangle, bool) {
	index := t.displayLineAt(t.displayCaret().Offset)
	p := t.paragraph(index)
	return t.paragraphCaret(index, p)
}

// paragraphOrigin maps the active local metrics directly into the viewport.
func (t *textEditor) paragraphOrigin(index int, p *textParagraph) geometry.Point {
	var x, y float64
	if p.paged != nil {
		frame := p.paged.Geometry()
		x, y = frame.X, frame.Y
	}
	return geometry.Point{X: t.padding.Left + float32(x-float64(t.offset.X)), Y: float32(float64(t.padding.Top) + t.heights.Top64(index) + float64(p.textOffset) + y - float64(t.offset.Y))}
}

func (t *textEditor) paragraphCaret(index int, p *textParagraph) (geometry.Rectangle, bool) {
	return t.paragraphCaretIn(index, p, false)
}
func (t *textEditor) viewportCaret(index int, p *textParagraph) (geometry.Rectangle, bool) {
	return t.paragraphCaretIn(index, p, true)
}
func (t *textEditor) paragraphCaretIn(index int, p *textParagraph, viewport bool) (geometry.Rectangle, bool) {
	if p == nil {
		return geometry.Rectangle{}, false
	}
	rng := t.displayLineRange(index)
	pos := t.displayCaret()
	pos.Offset -= rng.Start
	g := p.editGeometry(t.lineHeight)
	var x, y float64
	if p.paged != nil {
		frame := p.paged.CachedGeometryAt(pos.Offset)
		if frame == nil {
			return geometry.Rectangle{}, false
		}
		g, x, y = &frame.Geometry, frame.X, frame.Y
	}
	r := g.Caret(pos)
	x += float64(t.padding.Left)
	y += float64(t.padding.Top) + t.heights.Top64(index) + float64(p.textOffset)
	if viewport {
		x -= float64(t.offset.X)
		y -= float64(t.offset.Y)
	}
	r.X = float32(x + float64(r.X))
	r.Y = float32(y + float64(r.Y))
	return r, true
}

func (t *textEditor) paint(p Painter) {
	if !t.owner.Visible() || t.owner.base().destroyed {
		return
	}
	paintStyledBox(p, geometry.Rect(0, 0, t.owner.Rect().Width, t.owner.Rect().Height), t.resolvedStyle(style.PartDefault))
	p.SetClipRect(geometry.Rect(t.padding.Left, t.padding.Top,
		max(0, t.owner.Rect().Width-t.horizontalPadding()), max(0, t.owner.Rect().Height-t.verticalPadding())))
	selected := t.selection.Range()
	if t.preedit != nil {
		selected = TextRange{}
	}
	selectionColor, hasSelection := t.resolvedStyle("selection").BackgroundColor()
	start := t.heights.At(max(0, t.offset.Y-t.padding.Top))
	for i := start; i < t.displayLineCount(); i++ {
		y := t.padding.Top + t.heights.Top(i) - t.offset.Y
		if y >= t.viewport.Height {
			break
		}
		paragraph := t.paragraphs[i]
		if paragraph == nil {
			continue
		}
		origin := t.paragraphOrigin(i, paragraph)
		rng := t.displayLineRange(i)
		if hasSelection && selected.Start < selected.End && selected.Start <= rng.End && selected.End > rng.Start {
			for _, rect := range t.visibleSelection(paragraph, TextRange{Start: selected.Start - rng.Start, End: selected.End - rng.Start}, origin) {
				p.FillRect(rect.Translate(origin), graphics.ColorOf(selectionColor))
			}
			if selected.Start <= rng.End && selected.End > rng.End && i+1 < t.displayLineCount() && (paragraph.paged == nil || paragraph.paged.CachedOffset(rng.End-rng.Start)) {
				rect := paragraph.editGeometry(t.lineHeight).Caret(textedit.Position{Offset: rng.End - rng.Start, Upstream: true})
				rect.Width = max(1, t.lineHeight/3)
				p.FillRect(rect.Translate(origin), graphics.ColorOf(selectionColor))
			}
		}
		if paragraph.paged != nil {
			paragraph.paged.Paint(p, origin)
		} else if paragraph.layout != nil && paragraph.layout.Text() != "" {
			p.DrawTextLayout(origin, paragraph.layout)
		}
		if composition := t.preedit; composition != nil {
			if color, ok := t.resolvedStyle("caret").ForegroundColor(); ok {
				rangeInLine := TextRange{Start: composition.Replacement().Start - rng.Start, End: composition.Replacement().Start + len(composition.Text()) - rng.Start}
				for _, rect := range t.visibleSelection(paragraph, rangeInLine, origin) {
					rect.Y += rect.Height - 1
					rect.Height = 1
					p.FillRect(rect.Translate(origin), graphics.ColorOf(color))
				}
			}
		}
	}
	t.paintPlaceholder(p)
	if t.owner.base().destroyed || t.suspended {
		return
	}
	if t.owner.Focused() {
		index := t.displayLineAt(t.displayCaret().Offset)
		if rect, ok := t.viewportCaret(index, t.paragraphs[index]); ok {
			t.im.SetCaretRect(rect)
			if c, has := t.resolvedStyle("caret").ForegroundColor(); has && t.caretVisible {
				p.FillRect(rect, graphics.ColorOf(c))
			}
		}
	}
}

func (t *textEditor) visibleSelection(p *textParagraph, rng TextRange, origin geometry.Point) []geometry.Rectangle {
	if t.viewport.Height <= t.verticalPadding() {
		return nil
	}
	g := p.editGeometry(t.lineHeight)
	first := g.LineAtY(t.padding.Top - origin.Y)
	end := g.LineAtY(t.viewport.Height-t.padding.Bottom-origin.Y) + 1
	return g.SelectionLines(rng, first, end)
}

func (t *textEditor) snapshot() WidgetInfo {
	info := t.owner.base().Snapshot()
	info.Role = RoleTextView
	state := &TextEditingInfo{ReadOnly: t.readOnly, Selection: t.selection, Length: t.model.Len(), Placeholder: t.placeholder}
	// Bounded, range-labelled visible content, never a copy of the full model.
	start := t.heights.At(max(0, t.offset.Y-t.padding.Top))
	end := t.heights.At(max(0, t.offset.Y+t.viewport.Height-t.padding.Bottom))
	first := t.displayLineRange(start)
	last := t.displayLineRange(end)
	if p := t.paragraphs[start]; p != nil && p.paged != nil {
		r := p.paged.VisibleRange()
		first.Start += r.Start
		last.End = min(last.End, t.displayLineRange(start).Start+r.End)
	}
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
	state.VisibleRange = TextRange{Start: first.Start, End: limit}
	state.VisibleText, _ = t.model.Slice(state.VisibleRange)
	info.TextEditing = state
	if t.resizeWidth != 0 {
		info.SetAttribute("goui.text", map[string]any{"layoutPending": true, "layoutWidth": t.layoutWidth})
	}
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
	g := p.editGeometry(t.lineHeight)
	var baseY float64
	if p.paged != nil {
		frame := p.paged.CachedGeometryAt(offset - rng.Start)
		if frame == nil {
			t.queueLongLayout()
			return
		}
		g, baseY = &frame.Geometry, frame.Y
	}
	rect := g.Caret(textedit.Position{Offset: offset - rng.Start})
	t.offset.Y = max(0, float32(t.heights.Top64(index)+float64(p.textOffset)+baseY+float64(rect.Y)+float64(t.anchor.within)))
	// ScrollView may lay out twice before applying its queued scroll request.
	// Keep the text anchor until LayoutVisible receives the requested offset.
}

func (t *textEditor) saveViewportAnchor() {
	if t.preedit != nil || t.anchorPending {
		return
	}
	if t.anchor.valid {
		index := t.model.LineAt(t.anchor.offset)
		if p := t.paragraphs[index]; p != nil && p.measured {
			rng, _ := t.model.LineRange(index)
			g := p.editGeometry(t.lineHeight)
			var baseY float64
			if p.paged != nil {
				frame := p.paged.CachedGeometryAt(t.anchor.offset - rng.Start)
				if frame == nil {
					return
				}
				g, baseY = &frame.Geometry, frame.Y
			}
			rect := g.Caret(textedit.Position{Offset: t.anchor.offset - rng.Start})
			// Keep the original character while it explains the rounded
			// viewport. Rounding a paragraph top down may put the offset in
			// the preceding paragraph; sampling that paragraph first would
			// replace the anchor and make subsequent width changes drift.
			if mathx.Round(float32(t.heights.Top64(index)+float64(p.textOffset)+baseY+float64(rect.Y)+float64(t.anchor.within))) == t.offset.Y {
				return
			}
		}
	}
	// Text is translated by padding-offset and clipped at padding, so the
	// first visible content coordinate is offset. Sampling offset-padding
	// can retain a preceding line hidden entirely by the content clip.
	index := t.heights.At(max(0, t.offset.Y))
	p := t.paragraphs[index]
	if p == nil || p.paged != nil && p.paged.VisibleRange().End == 0 {
		return
	}
	y := t.padding.Top - t.paragraphOrigin(index, p).Y
	g := p.editGeometry(t.lineHeight)
	line := g.LineAtY(y)
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
	height := t.lineHeight
	if p != nil {
		height = p.height
	}
	center := max(0, (size.Height-t.verticalPadding()-height)/2)
	t.offset.Y = -center
	t.offset.X = min(t.offset.X, scrollExtent(t.contentSize().Width, size.Width))
	if t.revealCaret && size.Width > 0 {
		if rect, ok := t.caretRect(); ok {
			t.emitReveal(geometry.Rect(rect.X-t.padding.Left, rect.Y-t.padding.Top,
				rect.Width+t.horizontalPadding(), rect.Height+t.verticalPadding()))
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
	x = min(max(0, x), scrollExtent(t.contentSize().Width, t.viewport.Width))
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
	t.cancelResize()
	t.resizeTimer = nil
	t.caretVisible = true
}

func (t *textEditor) interactive() bool {
	if t.owner.base().destroyed || t.suspended || !IsEnabled(t.owner) || !t.owner.Focused() || t.owner.Root() == nil || !visibleInTree(t.owner) {
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
	step := func(point, before, after, extent float32) float32 {
		var distance float32
		if point < before {
			distance = point - before
		} else if point > extent-after {
			distance = point - (extent - after)
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
	dx := step(point.X, t.padding.Left, t.padding.Right, t.viewport.Width)
	if t.wrap != WrapNone {
		dx = 0
	} else if dx != 0 {
		// Use already measured visual edges: another row's width is not a reason
		// to scroll beyond this row's selectable text. This never shapes text.
		row := t.heights.At(point.Y + t.offset.Y - t.padding.Top)
		if p := t.paragraphs[row]; p != nil && p.measured && (p.paged == nil || p.paged.VisibleRange().End > 0) {
			g := p.editGeometry(t.lineHeight)
			origin := t.paragraphOrigin(row, p)
			edge := g.LineEdge(g.LineAtY(point.Y-origin.Y), dx > 0)
			rect := g.Caret(edge).Translate(origin)
			if dx < 0 && rect.X >= t.padding.Left || dx > 0 && float64(rect.X)+float64(rect.Width) <= float64(t.viewport.Width-t.padding.Right) {
				dx = 0
			}
		}
	}
	advance := func(offset, delta, content, viewport float32) float32 {
		if delta == 0 {
			return offset
		}
		value := float64(offset) + float64(delta)
		next := ceilScrollCoordinate(value)
		if delta < 0 {
			next = floorScrollCoordinate(value)
		}
		return min(max(0, next), mathx.Floor(scrollExtent(content, viewport)))
	}
	dy := step(point.Y, t.padding.Top, t.padding.Bottom, t.viewport.Height)
	next := geometry.Point{
		X: advance(t.offset.X, dx, extent.Width, t.viewport.Width),
		Y: advance(t.offset.Y, dy, extent.Height, t.viewport.Height),
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
	point.X = min(max(t.padding.Left, point.X), max(t.padding.Left, t.viewport.Width-t.padding.Right-1))
	point.Y = min(max(t.padding.Top, point.Y), max(t.padding.Top, t.viewport.Height-t.padding.Bottom-1))
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
	if t.owner.base().destroyed || t.suspended || !IsEnabled(t.owner) || t.readOnly || t.resettingIME {
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
	if t.owner.base().destroyed || t.suspended || !IsEnabled(t.owner) || t.readOnly || t.resettingIME {
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

// Keyboard bindings and menu items share operations, not synthetic key events.
// This stays private to the editor; it is not an application command registry.
type textEditCommand uint8

const (
	textUndo textEditCommand = iota
	textRedo
	textCut
	textCopy
	textPaste
	textDeleteSelection
	textSelectAll
)

type textEditMenu struct {
	popup    *PopoverMenu
	items    map[textEditCommand]*MenuItem
	language signal.Handle
}

func (command textEditCommand) menuLabel() string {
	switch command {
	case textUndo:
		return resource.Undo()
	case textRedo:
		return resource.Redo()
	case textCut:
		return resource.Cut()
	case textCopy:
		return resource.Copy()
	case textPaste:
		return resource.Paste()
	case textDeleteSelection:
		return resource.Delete()
	case textSelectAll:
		return resource.SelectAll()
	}
	return ""
}

func (t *textEditor) connectContextMenuLanguage() {
	if t.contextMenu == nil || t.contextMenu.language != nil || t.suspended || t.owner.base().destroyed {
		return
	}
	t.contextMenu.language = mlang.ConnectChanged(func() {
		menu := t.contextMenu
		if t.owner.base().destroyed || t.suspended || menu == nil || !menu.popup.Visible() {
			return
		}
		for command, item := range menu.items {
			if t.owner.base().destroyed || t.suspended || t.contextMenu != menu {
				return
			}
			item.SetLabel(command.menuLabel())
		}
	})
}

func (t *textEditor) canEditCommand(command textEditCommand) bool {
	if t.owner.base().destroyed || t.suspended || !IsEnabled(t.owner) {
		return false
	}
	selected := t.selection.Anchor != t.selection.Caret
	switch command {
	case textUndo:
		return !t.readOnly && t.model.CanUndo()
	case textRedo:
		return !t.readOnly && t.model.CanRedo()
	case textCopy:
		return selected && App != nil && App.Clipboard() != nil
	case textCut:
		return !t.readOnly && selected && App != nil && App.Clipboard() != nil
	case textPaste:
		// Clipboard has no format query. Do not fetch potentially sensitive text
		// merely to display a menu; an unsuccessful request is a paste no-op.
		return !t.readOnly && App != nil && App.Clipboard() != nil
	case textDeleteSelection:
		return !t.readOnly && selected
	case textSelectAll:
		return t.model.Len() != 0
	}
	return false
}

func (t *textEditor) runEditCommand(command textEditCommand) {
	if !t.canEditCommand(command) {
		return
	}
	model, epoch := t.model, t.mountEpoch
	if command != textCopy {
		t.cancelPreedit(true)
	}
	if t.owner.base().destroyed || t.model != model || t.mountEpoch != epoch {
		return
	}
	// Copy during IME composition keeps the committed selection. Its offsets
	// cannot be normalized against the shorter/longer temporary projection.
	if t.preedit == nil && !t.normalizeEditingSelection() {
		return
	}
	if !t.canEditCommand(command) {
		return
	}
	switch command {
	case textUndo, textRedo:
		t.undo(command == textRedo)
	case textCut, textCopy:
		t.copySelection(command == textCut)
	case textPaste:
		t.paste()
	case textDeleteSelection:
		t.replaceSelection("", true)
	case textSelectAll:
		t.setSelectionValue(TextSelection{Anchor: 0, Caret: t.model.Len()})
	}
}

// A nil point denotes keyboard invocation, positioned below the caret. Pointer
// coordinates and PopoverMenu.ShowAt both use editor-local DIP.
func (t *textEditor) showContextMenu(point *geometry.Point) {
	if t.owner.base().destroyed || t.suspended || !IsEnabled(t.owner) || t.owner.Window() == nil {
		return
	}
	model, epoch := t.model, t.mountEpoch
	t.cancelPreedit(true)
	if t.owner.base().destroyed || t.model != model || t.mountEpoch != epoch || !t.normalizeEditingSelection() {
		return
	}
	t.input.Reset()
	t.hasDesiredX = false
	position := geometry.Point{X: t.padding.Left, Y: t.padding.Top + t.lineHeight}
	if point != nil {
		position = *point
		if !t.selectionContains(position) {
			t.moveTo(t.hitText(position), false)
		}
	} else if caret, ok := t.viewportCaret(t.displayLineAt(t.displayCaret().Offset), t.paragraphs[t.displayLineAt(t.displayCaret().Offset)]); ok {
		position = geometry.Point{X: caret.X, Y: caret.Y + caret.Height}
	}
	if t.owner.base().destroyed || t.model != model || t.mountEpoch != epoch || t.suspended {
		return
	}
	t.model.BreakUndoGroup()
	if t.contextMenu == nil {
		t.contextMenu = &textEditMenu{popup: NewPopoverMenu(t.owner)}
	}
	t.connectContextMenuLanguage()
	menu := t.contextMenu
	menu.items = make(map[textEditCommand]*MenuItem)
	items := NewMenu()
	for _, command := range []textEditCommand{
		textUndo, textRedo, textCut, textCopy, textPaste, textDeleteSelection, textSelectAll,
	} {
		if command == textCut || command == textSelectAll {
			items.AppendSeparator()
		}
		item := items.Append(command.menuLabel(), func() {
			// PopoverMenu hides before activation. The hide/IME callbacks may
			// unmount us, replace the document, or open another menu.
			if t.model == model && t.mountEpoch == epoch && menu.popup.Menu() == items {
				t.runEditCommand(command)
			}
		})
		item.SetEnabled(t.canEditCommand(command))
		menu.items[command] = item
	}
	menu.popup.SetMenu(items)
	if err := menu.popup.ShowAt(position); err != nil {
		log.Printf("goui: text context menu: %v", err)
	}
}

// Hit the painted selection rather than the nearest caret: the last half of a
// selected Cluster can map to its end boundary. Only shape the hit paragraph,
// even if the selection spans a large document; bidi spans remain disjoint.
func (t *textEditor) selectionContains(point geometry.Point) bool {
	selected := t.selection.Range()
	if selected.Start == selected.End {
		return false
	}
	index := t.heights.At(point.Y + t.offset.Y - t.padding.Top)
	rng := t.displayLineRange(index)
	if selected.Start > rng.End || selected.End <= rng.Start {
		return false
	}
	p := t.paragraph(index)
	if p == nil {
		return false
	}
	origin := t.paragraphOrigin(index, p)
	point.X -= origin.X
	point.Y -= origin.Y
	g := p.editGeometry(t.lineHeight)
	contains := func(rect geometry.Rectangle) bool {
		return point.X >= rect.X && point.X < rect.X+rect.Width && point.Y >= rect.Y && point.Y < rect.Y+rect.Height
	}
	for _, rect := range g.Selection(TextRange{Start: selected.Start - rng.Start, End: selected.End - rng.Start}) {
		if contains(rect) {
			return true
		}
	}
	if selected.End > rng.End && index+1 < t.displayLineCount() {
		rect := g.Caret(textedit.Position{Offset: rng.End - rng.Start, Upstream: true})
		rect.Width = max(1, t.lineHeight/3)
		return contains(rect)
	}
	return false
}

func (t *textEditor) updateContextMenu() {
	if t.contextMenu == nil || !t.contextMenu.popup.Visible() {
		return
	}
	for command, item := range t.contextMenu.items {
		item.SetEnabled(t.canEditCommand(command))
	}
}

func (t *textEditor) hideContextMenu() {
	if t.contextMenu != nil {
		t.contextMenu.popup.Hide()
	}
}

// Coalesce intermediate constraints of a long wrapped paragraph. Existing
// text and hit testing keep the same measured width until the latest width is
// ready. This does not time-slice or preempt a native paragraph measurement.
const textResizeDelay = 150 * time.Millisecond

func (t *textEditor) cancelResize() {
	if t.resizeTimer != nil {
		t.resizeTimer.Stop()
	}
	changed := t.resizeWidth != 0
	t.resizeWidth, t.resizeReady = 0, false
	if changed && !t.owner.base().destroyed {
		t.owner.base().requestSemanticUpdate()
	}
}

func (t *textEditor) deferResize(width float32) bool {
	if width == t.layoutWidth || t.singleLine || t.wrap == WrapNone || t.preedit != nil ||
		t.suspended || t.owner.Root() == nil || App == nil {
		t.cancelResize()
		return false
	}
	if t.resizeWidth == width {
		if t.resizeReady {
			t.cancelResize()
			return false
		}
		if t.resizeTimer != nil && t.resizeTimer.Active() {
			return true
		}
	}
	long := false
	first := t.heights.At(max(0, t.offset.Y-t.padding.Top))
	last := t.heights.At(max(0, t.offset.Y+t.viewport.Height))
	for _, p := range t.paragraphs {
		if p.index < first || p.index > last || !p.measured || p.layout == nil || p.textBytes < 32768 {
			continue
		}
		// Restore an exact retained width immediately. The cache's complete
		// input comparison prevents resurrection of a replaced paragraph.
		cached := false
		for _, previous := range t.replacedParagraphs {
			w, _ := previous.layout.Size()
			if w == width && previous.layout.Text() == p.layout.Text() {
				cached = true
				break
			}
		}
		long = long || !cached
	}
	if !long {
		t.cancelResize()
		return false
	}
	if t.resizeTimer == nil {
		t.resizeTimer = App.NewTimer()
		t.resizeTimer.ConnectTimeout(func() {
			if t.owner.base().destroyed || t.suspended || t.resizeWidth == 0 || !visibleInTree(t.owner) {
				t.cancelResize()
				return
			}
			// Only schedule layout. A newer allocation can still supersede
			// this width before layout actually runs.
			t.resizeReady = true
			t.owner.RequestLayout()
			t.owner.RequestPaint()
		})
	}
	t.resizeWidth, t.resizeReady = width, false
	t.owner.base().requestSemanticUpdate()
	if err := t.resizeTimer.StartOnce(textResizeDelay); err != nil {
		// No usable GUI timer: keep the existing synchronous layout path.
		t.cancelResize()
		return false
	}
	return true
}

func (t *textEditor) imContext() IMContext {
	if t.readOnly || !IsEnabled(t.owner) {
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
	gesture   *GestureParticipation
	lastDown  time.Time
	lastPoint geometry.Point
	dragPoint geometry.Point
}

func (c *textEditController) CapturingPointer() bool { return c.dragging }
func (c *textEditController) Reset() {
	gesture := c.gesture
	c.gesture = nil
	gesture.Reject()
	c.dragging = false
	c.lastDown = time.Time{}
	if c.view.scrollTimer != nil {
		c.view.scrollTimer.Stop()
	}
	if !IsEnabled(c.view.owner) {
		t := c.view
		model, epoch, revision := t.model, t.mountEpoch, t.owner.base().enabledRevision
		current := func() bool {
			return !t.owner.base().destroyed && !IsEnabled(t.owner) && t.model == model &&
				t.mountEpoch == epoch && t.owner.base().enabledRevision == revision
		}
		t.inputRevision++
		t.hideContextMenu()
		if !current() {
			return
		}
		t.cancelPreedit(true)
		if !current() {
			return
		}
		t.model.BreakUndoGroup()
		t.syncBlink(true)
	}
}

func (c *textEditController) GestureAccepted(ctx EventContext) {
	event, ok := ctx.Event().(events.PointerEvent)
	if !ok {
		return
	}
	point, ok := ctx.Position()
	if ok {
		c.beginSelection(ctx, event, point)
	}
}

func (c *textEditController) GestureCanceled(GestureCancelReason) {
	c.gesture = nil
	c.dragging = false
	if c.view.scrollTimer != nil {
		c.view.scrollTimer.Stop()
	}
}

func (c *textEditController) beginSelection(ctx EventContext, event events.PointerEvent, point geometry.Point) {
	t := c.view
	t.hasDesiredX = false
	position := t.hitText(point)
	model, epoch := t.model, t.mountEpoch
	t.cancelPreedit(true)
	if t.owner.base().destroyed || t.model != model || t.mountEpoch != epoch {
		return
	}
	now := time.Now()
	double := !c.lastDown.IsZero() && now.Sub(c.lastDown) <= gestureClickInterval &&
		!gestureMoved(point, c.lastPoint, gestureDefaultDistance)
	c.lastDown, c.lastPoint, c.dragging = now, point, true
	c.dragPoint = point
	ctx.StopPropagation()
	if double {
		c.lastDown = time.Time{}
		rng := t.wordAt(position.Offset)
		t.setSelection(TextSelection{Anchor: rng.Start, Caret: rng.End}, false, true)
	} else {
		t.moveTo(position, event.Modifiers&events.ModifierShift != 0)
	}
}

func (c *textEditController) HandleEvent(ctx EventContext) {
	t := c.view
	if t.owner.base().destroyed || !IsEnabled(t.owner) {
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
			if !primaryPointer(event) {
				return
			}
			if gesture := JoinGesture(ctx); gesture != nil {
				c.gesture = gesture
				gesture.Claim()
				return
			}
			c.beginSelection(ctx, event, point)
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
				c.gesture = nil
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
	index := t.heights.At(point.Y + t.offset.Y - t.padding.Top)
	if p := t.paragraph(index); p != nil {
		rng := t.displayLineRange(index)
		origin := t.paragraphOrigin(index, p)
		pos := p.editGeometry(t.lineHeight).Hit(geometry.Point{X: point.X - origin.X, Y: point.Y - origin.Y})
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
	if t.owner.base().destroyed || !IsEnabled(t.owner) || t.readOnly || t.suspended {
		return
	}
	model, epoch := t.model, t.mountEpoch
	enabledRevision := t.owner.base().enabledRevision
	t.cancelPreedit(true)
	if t.owner.base().destroyed || !IsEnabled(t.owner) || t.owner.base().enabledRevision != enabledRevision || t.model != model || t.mountEpoch != epoch {
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
		return rng.Start + p.geometryAt(t.lineHeight, offset-rng.Start).Adjacent(offset-rng.Start, forward)
	}
	return offset // Never guess a cluster boundary from rune counts.
}

func (t *textEditor) horizontal(right bool) textedit.Position {
	index := t.model.LineAt(t.selection.Caret)
	rng, _ := t.model.LineRange(index)
	pos := textedit.Position{Offset: t.selection.Caret - rng.Start, Upstream: t.caretUpstream}
	if p := t.paragraph(index); p != nil {
		if next, ok := p.geometryAt(t.lineHeight, pos.Offset).Visual(pos, right); ok {
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
			var ax, ay, bx, by float64
			if p.paged != nil {
				ga := p.paged.GeometryAt(first.Offset - line.Start)
				if ga == nil {
					if right {
						return last
					}
					return first
				}
				a := ga.Caret(textedit.Position{Offset: first.Offset - line.Start, Upstream: first.Upstream})
				ax, ay = ga.X+float64(a.X), ga.Y+float64(a.Y)
				gb := p.paged.GeometryAt(last.Offset - line.Start)
				if gb == nil {
					if right {
						return last
					}
					return first
				}
				b := gb.Caret(textedit.Position{Offset: last.Offset - line.Start, Upstream: last.Upstream})
				bx, by = gb.X+float64(b.X), gb.Y+float64(b.Y)
			} else {
				g := p.editGeometry(t.lineHeight)
				a := g.Caret(textedit.Position{Offset: first.Offset - line.Start, Upstream: first.Upstream})
				b := g.Caret(textedit.Position{Offset: last.Offset - line.Start, Upstream: last.Upstream})
				ax, ay, bx, by = float64(a.X), float64(a.Y), float64(b.X), float64(b.Y)
			}
			if ay > by || ay == by && ax > bx {
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
		t.desiredX, t.hasDesiredX = rect.X-t.padding.Left, true
	}
	if page {
		delta := max(rect.Height, t.viewport.Height)
		if !down {
			delta = -delta
		}
		point := geometry.Point{X: t.desiredX + t.padding.Left - t.offset.X, Y: rect.Y - t.offset.Y + delta + rect.Height/2}
		index := t.model.LineAt(pos.Offset)
		if p := t.paragraphs[index]; p != nil && p.paged != nil {
			if local, ok := t.viewportCaret(index, p); ok {
				point = geometry.Point{X: local.X, Y: local.Y + delta + local.Height/2}
			}
		}
		return t.hitText(point)
	}
	index := t.model.LineAt(pos.Offset)
	rng, _ := t.model.LineRange(index)
	p := t.paragraph(index)
	if p == nil {
		return pos
	}
	if p.paged != nil {
		current, ok := t.viewportCaret(index, p)
		if !ok {
			return pos
		}
		delta := max(t.lineHeight, current.Height)
		if !down {
			delta = -delta
		}
		return t.hitText(geometry.Point{X: current.X, Y: current.Y + delta + current.Height/2})
	}
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
	if t.wrap == WrapNone && rng.End-rng.Start > 64*1024 {
		if end {
			return textedit.Position{Offset: rng.End, Upstream: true}
		}
		return textedit.Position{Offset: rng.Start}
	}
	p := t.paragraph(index)
	if p == nil {
		return textedit.Position{Offset: rng.Start, Upstream: false}
	}
	g := p.geometryAt(t.lineHeight, t.selection.Caret-rng.Start)
	i := g.LineIndex(textedit.Position{Offset: t.selection.Caret - rng.Start, Upstream: t.caretUpstream})
	pos := g.LineEdge(i, end)
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
			t.runEditCommand(textSelectAll)
			return true
		case events.KeyC:
			t.runEditCommand(textCopy)
			return true
		case events.KeyX:
			t.runEditCommand(textCut)
			return true
		case events.KeyV:
			t.runEditCommand(textPaste)
			return true
		case events.KeyZ:
			if shift {
				t.runEditCommand(textRedo)
			} else {
				t.runEditCommand(textUndo)
			}
			return true
		case events.KeyY:
			t.runEditCommand(textRedo)
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
	if err := App.Clipboard().SetText(text); err != nil {
		return
	}
	if cut && !t.readOnly {
		t.replaceSelection("", true)
	}
}

func (t *textEditor) paste() {
	if t.readOnly || !IsEnabled(t.owner) || App == nil || App.Clipboard() == nil {
		return
	}
	model, revision, selection, epoch := t.model, t.model.Revision(), t.selection, t.mountEpoch
	inputRevision := t.inputRevision
	App.Clipboard().RequestText(func(text string, err error) {
		if err != nil || text == "" || t.owner.base().destroyed || t.suspended || !IsEnabled(t.owner) || t.readOnly || t.model != model || model.Revision() != revision || t.selection != selection || t.mountEpoch != epoch || t.inputRevision != inputRevision {
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
		return TextRange{Start: line.Start + p.geometryAt(t.lineHeight, rng.Start-line.Start).Snap(rng.Start-line.Start, false), End: line.Start + p.geometryAt(t.lineHeight, rng.End-line.Start).Snap(rng.End-line.Start, true)}
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
	end := rng.End
	for end < line.End {
		last := min(line.End, end+4096)
		for !t.model.IsRuneBoundary(last) {
			last--
		}
		rest, _ := t.model.Slice(TextRange{Start: end, End: last})
		n := len(rest) - len(strings.TrimLeft(rest, " \t"))
		end += n
		if n < len(rest) {
			break
		}
	}
	if p := t.paragraph(index); p != nil {
		end = line.Start + p.geometryAt(t.lineHeight, end-line.Start).Snap(end-line.Start, true)
	}
	return end
}

// One optional layout per view, separate from document paragraphs and geometry.
// Text/font/wrap/width changes replace it; color changes reuse its geometry.
// It is created only when visible and released on configuration change/unmount.
type textPlaceholderLayout struct {
	layout      typography.TextLayout
	format      typography.TextFormat
	width       float32
	baseline    float32
	hasBaseline bool
}

func (t *textEditor) setPlaceholder(text string) {
	if t.singleLine {
		text = textedit.NormalizeSingleLine(text)
	} else {
		text = textedit.Normalize(text)
	}
	if t.owner.base().destroyed || t.placeholder == text {
		return
	}
	t.placeholder = text
	t.releasePlaceholder()
	t.owner.RequestPaint()
	t.owner.base().requestSemanticUpdate()
}

func (t *textEditor) releasePlaceholder() {
	old := t.placeholderLayout
	t.placeholderLayout = nil
	if old != nil {
		old.layout.Destroy()
	}
}

func (t *textEditor) showsPlaceholder() bool {
	return t.placeholder != "" && t.model.Len() == 0 && t.preedit == nil
}

func (t *textEditor) ensurePlaceholder() *textPlaceholderLayout {
	if !t.showsPlaceholder() || t.owner.base().destroyed || t.suspended || App == nil || App.Typography() == nil {
		return nil
	}
	format := t.format
	if c, ok := t.resolvedStyle("placeholder").ForegroundColor(); ok {
		format.TextColor = c
	}
	width := t.layoutWidth
	if format.WrapMode == WrapNone || width <= 0 {
		width = textInputMeasureExtent
	}
	cached := t.placeholderLayout
	if cached != nil && (cached.format.Font != format.Font || cached.format.WrapMode != format.WrapMode ||
		cached.format.TextAlign != format.TextAlign || cached.format.TabWidth != format.TabWidth || cached.width != width) {
		t.releasePlaceholder()
		cached = nil
	}
	if cached == nil {
		text, epoch := t.placeholder, t.mountEpoch
		valid := func() bool {
			return !t.owner.base().destroyed && !t.suspended && t.mountEpoch == epoch &&
				t.placeholder == text && t.showsPlaceholder() && t.placeholderLayout == nil &&
				t.format.Font == format.Font && t.format.WrapMode == format.WrapMode
		}
		if !valid() {
			return nil
		}
		l, err := App.Typography().NewTextLayout(text, format, width, textInputMeasureExtent)
		if err != nil {
			return nil
		}
		if !valid() {
			l.Destroy()
			return nil
		}
		lines, _ := l.MeasureMetrics()
		if !valid() {
			l.Destroy()
			return nil
		}
		cached = &textPlaceholderLayout{layout: l, format: format, width: width}
		if len(lines) > 0 {
			cached.baseline, cached.hasBaseline = lines[0].Baseline, true
		}
		t.placeholderLayout = cached
	} else if !colors.Equal(cached.format.TextColor, format.TextColor) {
		cached.format.TextColor = format.TextColor
		c := format.TextColor
		if c == nil {
			c = typography.DefaultTextColor()
		}
		cached.layout.SetTextColor(0, len(t.placeholder), c)
		// Layout notifications are synchronous and may unmount/change the view.
		if t.placeholderLayout != cached || t.owner.base().destroyed || t.suspended || !t.showsPlaceholder() {
			return nil
		}
	}
	return cached
}

func (t *textEditor) paintPlaceholder(p Painter) {
	cached := t.ensurePlaceholder()
	if cached == nil {
		return
	}
	origin := geometry.Point{X: t.padding.Left - t.offset.X, Y: t.padding.Top - t.offset.Y}
	if cached.hasBaseline {
		if t.singleLine && t.hasBaseline {
			origin.Y += t.baseline - cached.baseline
		} else if paragraph := t.paragraphs[0]; !t.singleLine && paragraph != nil {
			origin.Y += paragraph.baseline - cached.baseline
		}
	}
	p.DrawTextLayout(origin, cached.layout)
}
