package diagnostics

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/internal/diagnostic"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
)

const (
	hudInterval     = 250 * time.Millisecond
	hudIdleInterval = time.Second
)

// HUD draws recent CPU-frame statistics over a window without taking input or
// participating in layout. Its owner is the Capture; window destruction and
// capture stop release it automatically. All methods run on the GUI thread.
// After one second without drawing, one repaint changes FPS to zero. The other
// readings retain the last completed sample; no continuous frame loop is added.
type HUD struct {
	capture   *Capture
	window    gui.Window
	position  geometry.Point
	paint     signal.Handle
	destroy   signal.Handle
	layout    typography.TextLayout
	size      geometry.Size
	updated   time.Time
	timer     *gui.Timer
	timeout   signal.Handle
	lastPaint time.Time
	idle      bool
	idlePaint bool
}

// ShowHUD displays one overlay for window in this active capture. Repeated
// calls return the existing HUD. The window must belong to the captured app.
// The initial position is (12, 12) DIP; use SetPosition to avoid title controls.
// Showing requests one repaint. Subsequent updates only use ordinary window
// frames, at most four text updates per second. A one-shot idle timer requests
// one repaint to show zero FPS after a second without drawing.
// Draw FPS measures CPU-frame completion, not GPU or display presentation.
func (c *Capture) ShowHUD(window gui.Window) (*HUD, error) {
	if c == nil || c.session == nil || !c.session.Active() {
		return nil, errors.New("diagnostics HUD requires an active capture")
	}
	app, ok := c.session.Owner().(gui.Application)
	if !ok || window == nil {
		return nil, errors.New("diagnostics HUD requires a window from the captured application")
	}
	owned := false
	for _, candidate := range app.Windows() {
		if candidate == window {
			owned = true
			break
		}
	}
	if !owned {
		return nil, errors.New("diagnostics HUD window is unavailable or belongs to another application")
	}
	if h := c.huds[window]; h != nil {
		return h, nil
	}
	h := &HUD{capture: c, window: window, position: geometry.Point{X: 12, Y: 12}}
	if err := h.setText(hudText(diagnostic.RootSample{}, false)); err != nil {
		return nil, err
	}
	h.timer = app.NewTimer()
	h.timeout = h.timer.ConnectTimeout(h.onIdle)
	if c.huds == nil {
		c.huds = make(map[gui.Window]*HUD)
	}
	c.huds[window] = h
	if c.stopped == nil {
		c.stopped = c.session.ConnectStop(c.destroyHUDs)
	}
	h.paint = window.ConnectAfterPaint(h.draw)
	h.destroy = window.ConnectDestroy(func() { h.release(false) })
	if err := window.RequestPaint(); err != nil {
		h.release(false)
		return nil, err
	}
	return h, nil
}

// SetPosition moves the overlay in window-local DIP and requests one repaint.
// Non-finite coordinates panic; content outside the window is clipped.
// Calls after Destroy have no effect.
func (h *HUD) SetPosition(position geometry.Point) {
	if h == nil || h.window == nil {
		return
	}
	if math.IsNaN(float64(position.X)) || math.IsInf(float64(position.X), 0) ||
		math.IsNaN(float64(position.Y)) || math.IsInf(float64(position.Y), 0) {
		panic("diagnostics HUD position must be finite")
	}
	if h.position != position {
		h.position = position
		_ = h.window.RequestPaint()
	}
}

// Destroy removes the overlay, releases its resources and requests one repaint
// to erase it. Collection continues. Destroy is idempotent.
func (h *HUD) Destroy() { h.release(true) }

func (h *HUD) release(repaint bool) {
	if h == nil || h.window == nil {
		return
	}
	window, capture := h.window, h.capture
	h.window, h.capture = nil, nil
	h.paint.Disconnect()
	h.destroy.Disconnect()
	h.timer.Stop()
	h.timeout.Disconnect()
	h.paint, h.destroy = nil, nil
	h.timer, h.timeout = nil, nil
	h.layout.Destroy()
	h.layout = nil
	delete(capture.huds, window)
	if len(capture.huds) == 0 {
		capture.stopped.Disconnect()
		capture.stopped = nil
	}
	if repaint {
		_ = window.RequestPaint()
	}
}

func (c *Capture) destroyHUDs() {
	for _, h := range c.huds {
		h.Destroy()
	}
}

func (h *HUD) setText(text string) error {
	if h.layout != nil && h.layout.Text() == text {
		return nil
	}
	layout, err := h.capture.session.Owner().(gui.Application).Typography().NewTextLayout(text,
		typography.TextFormat{
			Font: typography.FontInfo{Size: 9}, TextColor: graphics.RGB(245, 247, 250),
		}, 1024, 512)
	if err != nil {
		return fmt.Errorf("diagnostics HUD text: %w", err)
	}
	if h.layout != nil {
		h.layout.Destroy()
	}
	h.layout = layout
	w, height := layout.MeasureSize()
	h.size = geometry.Size{Width: w + 20, Height: height + 16}
	return nil
}

func (h *HUD) draw(p gui.Painter, bounds geometry.Rectangle) {
	if h.window == nil {
		return
	}
	now := time.Now()
	if h.idlePaint {
		// This repaint only clears FPS. It must not restart its own idle timer.
		h.idlePaint = false
	} else {
		h.lastPaint = now
		if h.idle {
			h.idle = false
			h.updated = time.Time{}
		}
		if !h.timer.Active() {
			_ = h.timer.StartOnce(hudIdleInterval)
		}
	}
	if h.updated.IsZero() || now.Sub(h.updated) >= hudInterval {
		h.updated = now
		sample, ok := h.capture.session.ReadRoot(h.window)
		if h.idle {
			sample.Root.Idle = true
		}
		// A transient native text failure retains the previous readable sample.
		_ = h.setText(hudText(sample, ok))
	}
	p.Save()
	defer p.Restore()
	p.SetClipRect(bounds)
	p.FillRoundRect(geometry.Rectangle{Pos: h.position, Size: h.size}, 6, graphics.RGBA(20, 24, 30, 230))
	p.DrawTextLayout(h.position.Add(geometry.Point{X: 10, Y: 8}), h.layout)
}

func (h *HUD) onIdle() {
	if h.window == nil || h.idle || h.lastPaint.IsZero() {
		return
	}
	last := h.lastPaint
	if sample, ok := h.capture.session.ReadRoot(h.window); ok && sample.Frame.Completed {
		// End may block after AfterPaint. Count the silence from completion,
		// rather than clearing FPS while the preceding frame is still finishing.
		if completed := sample.Frame.Started.Add(sample.Frame.Total); completed.After(last) {
			last = completed
		}
	}
	if delay := hudIdleInterval - time.Since(last); delay > 0 {
		// Ordinary frames only update lastPaint; reschedule here, rather than
		// removing and reinserting a timer on every animation frame.
		_ = h.timer.StartOnce(delay)
		return
	}
	h.idle, h.idlePaint = true, true
	h.updated = time.Time{}
	if err := h.window.RequestPaint(); err != nil {
		h.idlePaint = false
	}
}

func hudText(sample diagnostic.RootSample, ok bool) string {
	fps, frame, layout, paint, end, backend := "--", "--", "--", "--", "--", "--"
	if ok {
		if sample.Root.Idle && sample.Frame.Completed {
			fps = "0"
		} else if sample.Root.FPS > 0 && !sample.Root.Idle {
			fps = fmt.Sprintf("%.1f", sample.Root.FPS)
		}
		if f := sample.Frame; f.Completed {
			ms := func(d time.Duration) string { return fmt.Sprintf("%.2f ms", float64(d)/float64(time.Millisecond)) }
			frame, layout, paint, end = ms(f.Total), ms(f.Layout), ms(f.Paint), ms(f.End)
		}
		if sample.Root.Backend != "" {
			backend = sample.Root.Backend
		}
	}
	return fmt.Sprintf("Draw FPS   %s\nFrame   %s\nLayout   %s\nPaint   %s\nEnd   %s\n%s · recent CPU frames", fps, frame, layout, paint, end, backend)
}
