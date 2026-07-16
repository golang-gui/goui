package appkit

import (
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
)

// State read by the struct-returning NSTextInputClient IMPs. Those trampolines
// cannot call back into Go (purego forbids struct/float callback returns), so
// the Go side stashes the current values here and the asm reads them. Only one
// view is first responder at a time and all access is on the main thread, so
// package-level storage is safe. Keep the field order/offsets in sync with the
// .s file: NSRange {+0 location, +8 length}; NSRect {+0 x, +8 y, +16 w, +24 h}.
var (
	currentSelectedRange NSRange
	currentMarkedRange   NSRange
	currentCaretRect     NSRect // screen coordinates
)

// SetInputSelectedRange/MarkedRange/CaretRect update the state the struct
// -returning IMPs report. Call from the focused view before/while composing.
func SetInputSelectedRange(r NSRange) { currentSelectedRange = r }
func SetInputMarkedRange(r NSRange)   { currentMarkedRange = r }
func SetInputCaretRect(r NSRect)      { currentCaretRect = r }

// Trampolines and their address getters live in nstextinput_darwin_arm64.s.
func selectedRangeTrampoline()
func markedRangeTrampoline()
func firstRectTrampoline()

func selectedRangeIMP() uintptr
func markedRangeIMP() uintptr
func firstRectIMP() uintptr
