package ui

import "github.com/golang-gui/goui/gui"

// Cursor re-exports the gui cursor value type so ui consumers use ui types only,
// without importing the gui or platform packages (same pattern as ColorScheme).
// It is a sealed interface; only gui-provided cursors (standard shapes today,
// custom image cursors later) satisfy it.
type Cursor = gui.Cursor

// CursorShape re-exports the standard-shape cursor, a Cursor backed by a named
// system shape.
type CursorShape = gui.CursorShape

const (
	CursorDefault   = gui.CursorDefault
	CursorText      = gui.CursorText
	CursorPointing  = gui.CursorPointing
	CursorCrosshair = gui.CursorCrosshair
	CursorForbidden = gui.CursorForbidden
	CursorNone      = gui.CursorNone
)
