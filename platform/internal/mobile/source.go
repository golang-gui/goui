package mobile

import (
	"image"
	"sync"

	"github.com/golang-gui/goui/platform/events"
)

// Source transfers platform observations to the Go window thread. Target
// revocation is synchronous and independent of that thread's responsiveness.
// Platform adapters own native objects, callbacks, geometry and presentation.
type Source struct {
	window     *Window
	mu         sync.Mutex
	current    *surfaceTarget
	generation uint64
	closed     bool
}

// Source returns the platform's native-observation sink.
func (w *Window) Source() *Source { return w.source }

// Attach publishes a fresh drawing target. Present must not wait for native UI.
func (s *Source) Attach(handle uintptr, generation uint64, width, height int32, scale float32, present func(image.Image) error) {
	s.mu.Lock()
	if s.closed || handle == 0 || generation <= s.generation {
		s.mu.Unlock()
		return
	}
	if s.current != nil {
		s.current.revoke()
	}
	target := &surfaceTarget{handle: handle, generation: generation, alive: true, present: present}
	s.current = target
	s.generation = generation
	s.mu.Unlock()
	s.window.loop.Post(func() { s.window.attach(target, width, height, scale) })
}

// Revoke rejects new borrowers and drains existing work before native release.
func (s *Source) Revoke() {
	s.mu.Lock()
	target := s.current
	if target != nil {
		target.revoke()
		s.current = nil
	}
	s.mu.Unlock()
	if target != nil {
		s.window.loop.Post(func() { s.window.detach(target) })
	}
}

func (s *Source) Visible(value bool) { s.window.loop.Post(func() { s.window.setVisible(value) }) }
func (s *Source) Focused(value bool) {
	s.window.loop.Post(func() { s.window.emit(events.FocusEvent{Focused: value}) })
}

// Redraw waits for a required native redraw boundary, without native UI calls
// from the paint handler. Android uses it for onNativeWindowRedrawNeeded.
func (s *Source) Redraw() {
	done := make(chan struct{})
	s.window.loop.Post(func() {
		defer close(done)
		w := s.window
		if !w.destroyed && w.visible && w.SurfaceState().Ready {
			w.paintDirty = false
			w.emit(events.PaintEvent{})
		}
	})
	select {
	case <-done:
	case <-s.window.loop.quit:
	}
}

// Close permanently stops native observations before disconnecting the adapter.
func (s *Source) Close() {
	s.mu.Lock()
	s.closed = true
	if s.current != nil {
		s.current.revoke()
		s.current = nil
	}
	s.mu.Unlock()
}
