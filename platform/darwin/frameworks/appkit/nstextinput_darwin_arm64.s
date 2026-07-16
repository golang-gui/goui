#include "textflag.h"

// Obj-C IMPs for the struct-returning NSTextInputClient methods. purego cannot
// register a Go callback that returns a struct/float (see doc/DesignIME.md §9),
// so these are hand-written C-ABI trampolines that read package-level state and
// place the result directly in the return registers. They take no frame, do not
// touch g, and never call back into Go, so they are safe to invoke from the
// Obj-C runtime on the main thread.

// selectedRangeTrampoline: -selectedRange -> NSRange in R0:R1.
TEXT ·selectedRangeTrampoline(SB), NOSPLIT|NOFRAME, $0-0
	MOVD	·currentSelectedRange+0(SB), R0
	MOVD	·currentSelectedRange+8(SB), R1
	RET

// markedRangeTrampoline: -markedRange -> NSRange in R0:R1.
TEXT ·markedRangeTrampoline(SB), NOSPLIT|NOFRAME, $0-0
	MOVD	·currentMarkedRange+0(SB), R0
	MOVD	·currentMarkedRange+8(SB), R1
	RET

// firstRectTrampoline: -firstRectForCharacterRange:actualRange: -> NSRect.
// C ABI args: self=R0 cmd=R1 range={R2,R3} actualRange*=R4.
// Returns NSRect {x,y,w,h} (screen coords) in F0..F3 from currentCaretRect and
// writes currentMarkedRange to *actualRange when it is non-nil.
TEXT ·firstRectTrampoline(SB), NOSPLIT|NOFRAME, $0-0
	CBZ	R4, noactual
	MOVD	·currentMarkedRange+0(SB), R5
	MOVD	R5, 0(R4)
	MOVD	·currentMarkedRange+8(SB), R5
	MOVD	R5, 8(R4)
noactual:
	FMOVD	·currentCaretRect+0(SB), F0
	FMOVD	·currentCaretRect+8(SB), F1
	FMOVD	·currentCaretRect+16(SB), F2
	FMOVD	·currentCaretRect+24(SB), F3
	RET

// Address getters: return the raw ABI0 entry of each trampoline for use as an
// Obj-C IMP (objc.IMP(...)).
TEXT ·selectedRangeIMP(SB), NOSPLIT, $0-8
	MOVD	$·selectedRangeTrampoline(SB), R0
	MOVD	R0, ret+0(FP)
	RET

TEXT ·markedRangeIMP(SB), NOSPLIT, $0-8
	MOVD	$·markedRangeTrampoline(SB), R0
	MOVD	R0, ret+0(FP)
	RET

TEXT ·firstRectIMP(SB), NOSPLIT, $0-8
	MOVD	$·firstRectTrampoline(SB), R0
	MOVD	R0, ret+0(FP)
	RET
