package common

import "github.com/golang-gui/goui/platform/dragdrop"

// DragDrop is a surface-scoped native drag and drop capability. It does not
// perform widget hit testing or decide whether the application accepts data.
// All methods are thread-affine to the owning platform event loop.
type DragDrop interface {
	// SetFormats replaces the portable formats accepted by this surface.
	SetFormats([]dragdrop.Format) error
	// Begin starts a session from the current native left-button press. A nil
	// Data is valid for a source offering only application-local values.
	Begin(id uint64, data *dragdrop.Data, actions dragdrop.Action, preview dragdrop.Preview) error
	Cancel()
	Destroy()
}
