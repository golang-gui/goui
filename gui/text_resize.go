package gui

import "time"

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
	first := t.heights.At(max(0, t.offset.Y-t.padding))
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
