package gui

import (
	"log"
	"time"

	"github.com/golang-gui/goui/core/signal"
)

const frameInterval = time.Second / 60

// ConnectFrame observes updates before layout and paint on the GUI thread.
// Connecting does not request a frame. RenderWidget does not emit this signal.
func (b *rootBase) ConnectFrame(fn func(time.Time)) signal.Handle {
	if b.frameDestroyed {
		return signal.Handles(nil)
	}
	return b.frames.Connect(func(now time.Time) {
		if !b.frameDestroyed && !b.frameSuspended && b.surfaceEpoch == b.frameEpoch {
			fn(now)
		}
	})
}

func (b *rootBase) requestPaint(native paintRequester) error {
	b.paintDirty = true
	b.frameRequester = native
	if native == nil || b.frameDestroyed || b.frameSuspended || b.framePainting || b.framePending {
		return nil
	}
	if b.frameTimer != nil && b.frameTimer.Active() {
		return nil
	}
	delay := b.lastFrame.Add(frameInterval).Sub(b.frameNow())
	if delay > 0 && b.app != nil && b.app.timers != nil {
		if b.frameTimer == nil {
			b.frameTimer = b.app.NewTimer()
			b.frameTimer.ConnectTimeout(func() {
				if err := b.submitFrame(); err != nil {
					log.Printf("goui: request frame: %v", err)
				}
			})
		}
		return b.frameTimer.StartOnce(delay)
	}
	return b.submitFrame()
}

func (b *rootBase) frameNow() time.Time {
	if b.app != nil && b.app.timers != nil {
		return b.app.timers.now()
	}
	return time.Now()
}

func (b *rootBase) submitFrame() error {
	if !b.paintDirty || b.frameDestroyed || b.frameSuspended || b.frameRequester == nil {
		return nil
	}
	b.framePending = true
	if err := b.frameRequester.RequestPaint(); err != nil {
		b.framePending = false
		return err
	}
	return nil
}

func (b *rootBase) beginFrame() bool {
	if b.frameDestroyed || b.frameSuspended {
		return false
	}
	if b.framePainting {
		b.paintDirty = true
		return false
	}
	b.stopFrameTimer()
	b.framePending, b.paintDirty = false, false
	b.framePainting = true
	b.lastFrame, b.frameEpoch = b.frameNow(), b.surfaceEpoch
	b.frames.Emit(b.lastFrame)
	if b.frameDestroyed || b.frameSuspended || b.surfaceEpoch != b.frameEpoch {
		b.endFrame()
		return false
	}
	return true
}

func (b *rootBase) endFrame() {
	b.framePainting = false
	if b.paintDirty && !b.frameDestroyed && !b.frameSuspended {
		if err := b.requestPaint(b.frameRequester); err != nil {
			log.Printf("goui: request frame: %v", err)
		}
	}
}

func (b *rootBase) stopFrameTimer() {
	if b.frameTimer != nil {
		b.frameTimer.Stop()
	}
}

func (b *rootBase) suspendFrames() {
	b.frameSuspended = true
	b.framePending = false
	b.stopFrameTimer()
}

func (b *rootBase) resumeFrames(native paintRequester) {
	b.frameSuspended = false
	_ = b.requestPaint(native)
}

func (b *rootBase) destroyFrames() {
	b.frameDestroyed = true
	b.suspendFrames()
	b.frameRequester = nil
}
