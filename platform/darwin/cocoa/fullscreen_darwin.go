package cocoa

import (
	"github.com/ebitengine/purego/objc"
	"github.com/golang-gui/goui/platform/common"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/appkit"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_foundation"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
)

// Requests remain separate from State: native notifications and queries are
// authoritative, including intermediate states and rejected transitions.
func (w *Window) applyStateRequest() (err error) {
	if w.closed || !w.window.Valid() {
		return common.ErrUnavailable
	}
	if !w.statePending || w.stateRequest || w.stateAwaiting || w.controlsFullscreenTransition {
		return nil
	}
	w.stateRequest = true
	defer func() {
		w.stateRequest = false
		w.queueStateRequest()
	}()
	target, revision := w.stateTarget, w.stateRevision
	complete := func() {
		if w.stateRevision == revision {
			w.statePending = false
		}
	}
	native := w.window
	native.Retain()
	defer native.Release()
	AutoReleasePool(func() {
		if w.State() == target {
			complete()
			return
		}
		if target == common.WindowStateHidden {
			complete()
			err = w.Hide()
			return
		}
		if native.IsMiniaturized() {
			w.stateAwaiting, w.stateAwaitingTarget = true, target
			native.Deminiaturize(0)
			return
		}
		if native.StyleMask()&NSWindowStyleMaskFullScreen != 0 && target != common.WindowStateFullscreen {
			if target == common.WindowStateMinimized {
				w.minimizedFullscreen = true
			}
			w.stateAwaiting, w.stateAwaitingTarget = true, target
			native.ToggleFullScreen(0)
			return
		}
		if target == common.WindowStateMinimized {
			w.stateAwaiting, w.stateAwaitingTarget = true, target
			native.Miniaturize(0)
			return
		}
		if !native.IsVisible() {
			native.MakeKeyAndOrderFront(0)
		}
		if w.closed || !w.window.Valid() {
			err = common.ErrUnavailable
			return
		}
		if w.stateRevision != revision {
			return // a native callback installed a newer target
		}
		if target == common.WindowStateFullscreen {
			if native.StyleMask()&NSWindowStyleMaskFullScreen != 0 {
				complete()
				return
			}
			if mask := native.StyleMask(); mask&NSWindowStyleMaskResizable == 0 {
				w.fullscreenStyle = &mask
				native.SetStyleMask(mask | NSWindowStyleMaskResizable)
			}
			if w.closed || !w.window.Valid() {
				err = common.ErrUnavailable
				return
			}
			w.stateAwaiting, w.stateAwaitingTarget = true, target
			native.ToggleFullScreen(0)
			return
		}
		complete()
		wantZoom := target == common.WindowStateMaximized
		if native.StyleMask()&NSWindowStyleMaskResizable != 0 && native.IsZoomed() != wantZoom {
			native.Zoom(0)
		}
		if w.closed || !w.window.Valid() {
			err = common.ErrUnavailable
			return
		}
		w.notifyState()
	})
	if w.closed || !w.window.Valid() {
		return common.ErrUnavailable
	}
	return err
}

func (w *Window) queueStateRequest() {
	if !w.statePending || w.stateRequest || w.stateAwaiting || w.controlsFullscreenTransition || w.stateQueued || w.closed || !w.window.Valid() {
		return
	}
	w.stateQueued = true
	loop := CFRunLoopGetMain()
	block := objc.NewBlock(func(_ objc.Block) {
		w.stateQueued = false
		if !w.closed && w.window.Valid() {
			_ = w.applyStateRequest()
		}
	})
	CFRunLoopPerformBlock(loop, KCFRunLoopCommonModes, block)
	block.Release()
	CFRunLoopWakeUp(loop)
}

func (w *Window) restoreFullscreenStyle() {
	if w.fullscreenStyle != nil && !w.closed && w.window.Valid() && w.window.StyleMask()&NSWindowStyleMaskFullScreen == 0 {
		mask := *w.fullscreenStyle
		w.fullscreenStyle = nil
		w.window.SetStyleMask(mask)
	}
}

func windowDidChangePresentation(self NSWindowDelegate, notification NSNotification) {
	self.Retain()
	defer self.Release()
	if w := windowMap[Cast[NSWindow](notification.Object())]; w != nil {
		w.finishStateRequest(false, false)
	}
}

func (w *Window) finishStateRequest(fullscreen, failed bool) {
	requesting := w.stateRequest
	w.stateRequest = true
	defer func() {
		w.stateRequest = requesting
		w.queueStateRequest()
	}()
	w.stateAwaiting = false
	if fullscreen {
		w.controlsFullscreenTransition = false
		if failed && w.stateTarget == w.stateAwaitingTarget {
			w.statePending = false
		}
		w.restoreFullscreenStyle()
	}
	if w.closed || !w.window.Valid() {
		return
	}
	w.updateControlsPosition()
	if w.closed || !w.window.Valid() {
		return
	}
	w.notifyState()
}
