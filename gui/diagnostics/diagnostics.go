// Package diagnostics provides optional GUI-thread CPU-frame and resource
// captures. Importing it does not enable recording or schedule application work.
package diagnostics

import (
	"errors"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/internal/diagnostic"
)

// Options selects optional widget counters; frame and resource data are always
// recorded after Start. Widgets adds call counts, not individual widget timings.
type Options = diagnostic.Options

// Resources counts process-wide observed operations, not live resources or GPU
// memory. PixelBytes is supplied RGBA payload, including Software operations.
type Resources = diagnostic.Resources

// Frame describes CPU work. Completed requires normal drawing and End return;
// neither Completed nor EndReturned confirms GPU execution or presentation.
type Frame = diagnostic.Frame

// Root summarizes a window or popover in this capture. FPS estimates recent CPU
// completion intervals; it is zero during Idle or without sufficient samples.
type Root = diagnostic.Root

// Widget counts framework cache queries, paint traversal and StyleChanged calls.
// Direct user calls may bypass these entries. Bounds are root-local DIP.
// Arrange calls and rectangle writes are intentionally not counted.
type Widget = diagnostic.Widget

// Snapshot is a bounded detached result. Refs are capture-local, independent of
// application IDs and dev refs. Durations are nanoseconds in JSON.
type Snapshot = diagnostic.Snapshot

// Capture owns optional diagnostics until Stop or Application.Run returns.
// All methods must run on the GUI thread. It never requests layout or painting.
type Capture struct{ session *diagnostic.Session }

// Start starts a fresh capture for the GOUI application (including before Run).
// Only one capture can be active. Native resource counters are process-wide.
func Start(app gui.Application, options Options) (*Capture, error) {
	if app == nil || gui.App == nil || app != gui.App {
		return nil, errors.New("diagnostics requires the GOUI application")
	}
	s, err := diagnostic.StartCapture(app, options)
	if err != nil {
		return nil, err
	}
	return &Capture{session: s}, nil
}

// Snapshot copies current data. It does not start collection, query native
// window APIs, dispatch input or request a frame. Prefer occasional queries.
func (c *Capture) Snapshot() Snapshot {
	if c == nil {
		return Snapshot{}
	}
	return c.session.Snapshot()
}

// Stop is idempotent and freezes an independent result. Restart with Start;
// stopping an old capture cannot stop a newer one. Call on the GUI thread.
func (c *Capture) Stop() {
	if c != nil {
		c.session.Stop()
	}
}
