package win32

import (
	"errors"
	"unicode/utf16"
	"unsafe"

	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/windows/sdk/winapi"
)

type clipboard struct{}

func newClipboard() (common.Clipboard, error) {
	return &clipboard{}, nil
}

func (c *clipboard) SetText(text string) error {
	// UTF-16 with a trailing NUL, as required by CF_UNICODETEXT.
	u16 := append(utf16.Encode([]rune(text)), 0)
	size := uintptr(len(u16) * 2)

	mem := winapi.GlobalAlloc(winapi.GMEM_MOVEABLE, size)
	if mem == 0 {
		return errors.New("clipboard: GlobalAlloc failed")
	}
	dst := winapi.GlobalLock(mem)
	if dst == nil {
		return errors.New("clipboard: GlobalLock failed")
	}
	copy(unsafe.Slice((*uint16)(dst), len(u16)), u16)
	winapi.GlobalUnlock(mem)

	if !winapi.OpenClipboard(0) {
		return errors.New("clipboard: OpenClipboard failed")
	}
	defer winapi.CloseClipboard()
	winapi.EmptyClipboard()
	if winapi.SetClipboardData(winapi.CF_UNICODETEXT, mem) == 0 {
		// Ownership was not transferred; the block leaks on this rare path.
		return errors.New("clipboard: SetClipboardData failed")
	}
	// On success the system owns mem; do not free it.
	return nil
}

func (c *clipboard) RequestText(callback func(text string, ok bool)) {
	text, ok := readClipboardText()
	callback(text, ok)
}

func readClipboardText() (string, bool) {
	if !winapi.OpenClipboard(0) {
		return "", false
	}
	defer winapi.CloseClipboard()

	mem := winapi.GetClipboardData(winapi.CF_UNICODETEXT)
	if mem == 0 {
		return "", false
	}
	ptr := winapi.GlobalLock(mem)
	if ptr == nil {
		return "", false
	}
	defer winapi.GlobalUnlock(mem)

	// Scan the NUL-terminated UTF-16 buffer.
	p := (*uint16)(ptr)
	n := 0
	for *(*uint16)(unsafe.Add(unsafe.Pointer(p), uintptr(n)*2)) != 0 {
		n++
	}
	return string(utf16.Decode(unsafe.Slice(p, n))), true
}
