// Package xerrors scopes expected X errors to requests on the owning thread.
// The platform's existing process-wide Xlib handler delegates to Consume;
// this package does not install a second handler or perform server round trips.
package xerrors

import "github.com/golang-gui/goui/platform/linux/libs/xlib"

type Trap struct {
	display   xlib.Display
	first     uintptr
	end       uintptr // zero while the native call is in progress
	resources []xlib.ID
	codes     []uint8
	failed    *bool // only records state; never runs GUI from inside Xlib
}

var pending []*Trap // platform-thread-affine, including late asynchronous errors

func Begin(display xlib.Display, resources []xlib.ID, failed *bool, codes ...uint8) *Trap {
	retire(display, display.LastKnownRequestProcessed())
	t := &Trap{display: display, first: display.NextRequest(), resources: resources, codes: codes, failed: failed}
	pending = append(pending, t)
	return t
}

func (t *Trap) End() {
	t.end = t.display.NextRequest()
	retire(t.display, t.display.LastKnownRequestProcessed())
}

func retire(display xlib.Display, processed uintptr) {
	kept := pending[:0]
	for _, t := range pending {
		if t.display == display && t.end != 0 && (t.end == t.first || t.end-1 <= processed) {
			continue
		}
		kept = append(kept, t)
	}
	clear(pending[len(kept):])
	pending = kept
}

// Consume returns false for every error outside an explicitly expected scope.
func Consume(display xlib.Display, event *xlib.ErrorEvent) bool {
	for i := len(pending) - 1; i >= 0; i-- {
		t := pending[i]
		if display != t.display || event.Display != t.display || event.Serial < t.first ||
			(t.end != 0 && event.Serial >= t.end) {
			continue
		}
		resource := false
		for _, id := range t.resources {
			resource = resource || event.ResourceID == id
		}
		if !resource {
			continue
		}
		for _, code := range t.codes {
			if event.ErrorCode == code {
				*t.failed = true
				return true
			}
		}
	}
	return false
}
