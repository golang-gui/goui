package pango

import (
	"math"
	"time"

	native "github.com/golang-gui/goui/platform/linux/libs/pango"
	"github.com/golang-gui/goui/platform/typography"
)

var _ typography.ReflowLayout = (*TextLayout)(nil)

func (t *TextLayout) Reflow(width, height float32) bool {
	if t.life.IsDestroyed() || t.format.WrapMode == typography.WrapNone || t.format.TextAlign != typography.TextAlignBegin || len(t.placeholders) != 0 || t.hasUnderline || len(t.strikes) != 0 || width <= 0 || math.IsNaN(float64(width)) || math.IsInf(float64(width), 0) {
		return false
	}
	if t.pending != nil && t.pendingWidth == width && t.pendingHeight == height {
		return true
	}
	t.CancelReflow()
	if t.width == width && t.height == height {
		return true
	}
	t.pendingWidth, t.pendingHeight = width, height
	t.pending = &retainedReflow{paragraph: t.retained, width: int64(roundToPixel(width) * native.Scale), wrap: t.format.WrapMode}
	return true
}

func (t *TextLayout) CancelReflow() {
	if t.pending != nil {
		t.pending.destroy()
		t.pending = nil
	}
}

func (t *TextLayout) AdvanceReflow(deadline time.Time) (bool, error) {
	if t.pending == nil {
		return true, nil
	}
	if !time.Now().Before(deadline) {
		return false, nil
	}
	if t.retained == nil {
		// Cold analysis remains a native, non-preemptible operation. Only the
		// first deferred resize pays it; later widths reuse this exact snapshot.
		m, err := retainParagraph(t)
		if err != nil {
			t.CancelReflow()
			return true, err
		}
		t.retained, t.pending.paragraph = m, m
	}
	for time.Now().Before(deadline) {
		if t.pending.advance(1) {
			if t.prepared != nil {
				t.prepared.destroy()
			}
			t.prepared, t.pending = t.pending, nil
			t.width, t.height = t.pendingWidth, t.pendingHeight
			t.layout.SetWidth(roundToPixel(t.width) * native.Scale)
			t.life.Changed()
			return true, nil
		}
	}
	return false, nil
}

func (t *TextLayout) clearReflow() {
	t.CancelReflow()
	t.reflowColorsDirty = false
	if t.prepared != nil {
		t.prepared.destroy()
		t.prepared = nil
	}
	if t.retained != nil {
		t.retained.destroy()
		t.retained = nil
	}
}

func (t *TextLayout) recolorReflow() {
	if t.retained == nil {
		return
	}
	t.reflowColorsDirty = true
	if t.prepared != nil {
		t.prepared.clearPages()
	}
	if t.pending != nil {
		t.pending.clearPages()
	}
}

func (t *TextLayout) syncReflowColors() {
	if !t.reflowColorsDirty || t.retained == nil {
		return
	}
	t.reflowColorsDirty = false
	t.retained.paintAttrs.Unref()
	t.retained.paintAttrs = t.attrs.Copy()
	// Internal diagnostic queries may have prepared pages since the setter.
	if t.prepared != nil {
		t.prepared.clearPages()
	}
}
