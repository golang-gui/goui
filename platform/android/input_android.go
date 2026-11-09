package android

import native "github.com/golang-gui/goui/platform/android/libs/android"

func (h *nativeApplication) onInputCreated(address, queue uintptr) {
	withNativeThread(func() {
		a := h.activityFor(address)
		if a == nil {
			return
		}
		a.detachInput()
		a.input = queue
		native.InputQueueAttachLooper(queue, native.LooperForThread(), native.LooperPollCallback, h.inputCallback, queue)
	})
}
func (h *nativeApplication) onInputDestroyed(address, queue uintptr) {
	withNativeThread(func() {
		if a := h.activityFor(address); a != nil && a.input == queue {
			a.detachInput()
		}
	})
}
func (a *activityState) detachInput() {
	if queue := a.input; queue != 0 {
		a.input = 0
		native.InputQueueDetachLooper(queue)
	}
}
func (h *nativeApplication) onInput(_ int32, _ int32, queue uintptr) int32 {
	withNativeThread(func() {
		a := h.activity
		if a == nil || a.input != queue {
			return
		}
		for h.activity == a && a.input == queue {
			event, status := native.InputQueueGetEvent(queue)
			if status < 0 {
				return
			}
			if native.InputQueuePreDispatchEvent(queue, event) != 0 {
				continue
			}
			// Native pointer decoding is a separate slice. Unhandled input must
			// still be finished exactly once unless Android pre-dispatched it.
			native.InputQueueFinishEvent(queue, event, 0)
		}
	})
	return 1
}
