//go:build darwin && arm64

package appkit

import (
	"testing"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
)

// These tests verify the risky ABI assumptions behind the NSTextInputClient
// struct-return trampolines (nstextinput_darwin_arm64.s) BEFORE the rest of the
// IME backend is written:
//
//  1. the *IMP() getters return the raw ABI0 entry of each trampoline, and
//  2. the trampolines return a struct by value in the correct registers
//     (NSRange in X0:X1, NSRect as a 4-double HFA in V0..V3),
//
// which is exactly the ABI objc_msgSend uses when the input system calls these
// methods. The direct-call tests call the trampoline as a plain C function via
// purego.RegisterFunc; the msgSend test additionally proves it works once
// registered as a real Obj-C method and dispatched through objc_msgSend.
//
// Run on an Apple-Silicon Mac:  go test ./platform/darwin/frameworks/appkit/

func TestSelectedRangeTrampolineReturnsNSRange(t *testing.T) {
	var call func(self, cmd uintptr) NSRange
	purego.RegisterFunc(&call, selectedRangeIMP())

	currentSelectedRange = NSRange{Location: 3, Length: 5}
	got := call(0, 0)
	if got.Location != 3 || got.Length != 5 {
		t.Fatalf("selectedRange trampoline = %+v, want {3 5}", got)
	}
}

func TestMarkedRangeTrampolineReturnsNSRange(t *testing.T) {
	var call func(self, cmd uintptr) NSRange
	purego.RegisterFunc(&call, markedRangeIMP())

	currentMarkedRange = NSRange{Location: 7, Length: 2}
	got := call(0, 0)
	if got.Location != 7 || got.Length != 2 {
		t.Fatalf("markedRange trampoline = %+v, want {7 2}", got)
	}
}

func TestFirstRectTrampolineReturnsNSRect(t *testing.T) {
	var call func(self, cmd, rangeLoc, rangeLen, actual uintptr) NSRect
	purego.RegisterFunc(&call, firstRectIMP())

	currentCaretRect = NSMakeRect(10, 20, 2, 16)
	got := call(0, 0, 0, 0, 0)
	if got.Origin.X != 10 || got.Origin.Y != 20 || got.Size.Width != 2 || got.Size.Height != 16 {
		t.Fatalf("firstRect trampoline = %+v, want {{10 20} {2 16}}", got)
	}
}

func TestFirstRectTrampolineWritesActualRange(t *testing.T) {
	var call func(self, cmd, rangeLoc, rangeLen uintptr, actual *NSRange) NSRect
	purego.RegisterFunc(&call, firstRectIMP())

	currentCaretRect = NSMakeRect(0, 0, 1, 1)
	currentMarkedRange = NSRange{Location: 1, Length: 4}
	var actual NSRange
	call(0, 0, 0, 0, &actual)
	if actual.Location != 1 || actual.Length != 4 {
		t.Fatalf("firstRect actualRange out-param = %+v, want {1 4}", actual)
	}
}

// TestTrampolinesDispatchThroughObjcMsgSend proves the trampolines work as real
// Obj-C methods: registered on a throwaway class and invoked via objc_msgSend,
// exactly as the macOS input system will call them.
func TestTrampolinesDispatchThroughObjcMsgSend(t *testing.T) {
	libobjc, err := purego.Dlopen("/usr/lib/libobjc.A.dylib", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
	if err != nil {
		t.Skipf("cannot open libobjc: %v", err)
	}
	msgSend, err := purego.Dlsym(libobjc, "objc_msgSend")
	if err != nil {
		t.Skipf("cannot find objc_msgSend: %v", err)
	}

	cls, err := objc.RegisterClass("GouiIMEABITest", objc.GetClass("NSObject"), nil, nil, nil)
	if err != nil {
		t.Fatalf("RegisterClass: %v", err)
	}
	selRange := objc.RegisterName("selectedRange")
	selRect := objc.RegisterName("firstRectForCharacterRange:actualRange:")
	cls.AddMethod(selRange, objc.IMP(selectedRangeIMP()), "{_NSRange=QQ}@:")
	cls.AddMethod(selRect, objc.IMP(firstRectIMP()), "{CGRect={CGPoint=dd}{CGSize=dd}}@:{_NSRange=QQ}^{_NSRange=QQ}")

	obj := objc.Send[objc.ID](objc.ID(cls), objc.RegisterName("new"))

	var sendRange func(objc.ID, objc.SEL) NSRange
	purego.RegisterFunc(&sendRange, msgSend)
	currentSelectedRange = NSRange{Location: 8, Length: 1}
	if got := sendRange(obj, selRange); got.Location != 8 || got.Length != 1 {
		t.Fatalf("objc_msgSend selectedRange = %+v, want {8 1}", got)
	}

	var sendRect func(objc.ID, objc.SEL, NSRange, uintptr) NSRect
	purego.RegisterFunc(&sendRect, msgSend)
	currentCaretRect = NSMakeRect(11, 22, 3, 17)
	got := sendRect(obj, selRect, NSRange{0, 0}, 0)
	if got.Origin.X != 11 || got.Origin.Y != 22 || got.Size.Width != 3 || got.Size.Height != 17 {
		t.Fatalf("objc_msgSend firstRect = %+v, want {{11 22} {3 17}}", got)
	}
}
