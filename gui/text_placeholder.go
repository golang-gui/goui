package gui

import (
	"github.com/golang-gui/goui/core/colors"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui/textedit"
	"github.com/golang-gui/goui/platform/typography"
)

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
	origin := geometry.Point{X: t.padding - t.offset.X, Y: t.padding - t.offset.Y}
	if cached.hasBaseline {
		if t.singleLine && t.hasBaseline {
			origin.Y += t.baseline - cached.baseline
		} else if paragraph := t.paragraphs[0]; !t.singleLine && paragraph != nil {
			origin.Y += paragraph.editGeometry(t.lineHeight).LineMetrics(0).Baseline - cached.baseline
		}
	}
	p.DrawTextLayout(origin, cached.layout)
}
