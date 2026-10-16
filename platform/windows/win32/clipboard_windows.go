package win32

import (
	"errors"
	"unicode/utf16"
	"unsafe"

	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/windows/sdk/winapi"
)

type clipboard struct{ platform *Platform }

func newClipboard(p *Platform) (common.Clipboard, error) {
	if p.destroyed || p.helperWindow == 0 {
		return nil, common.ErrUnavailable
	}
	return &clipboard{platform: p}, nil
}

func (c *clipboard) SetText(text string) error {
	if c.platform.destroyed || c.platform.helperWindow == 0 {
		return common.ErrUnavailable
	}
	// UTF-16 with a trailing NUL, as required by CF_UNICODETEXT.
	u16 := append(utf16.Encode([]rune(text)), 0)
	size := uintptr(len(u16) * 2)

	mem := winapi.GlobalAlloc(winapi.GMEM_MOVEABLE, size)
	if mem == 0 {
		return errors.New("clipboard: GlobalAlloc failed")
	}
	transferred := false
	defer func() {
		if !transferred {
			winapi.GlobalFree(mem)
		}
	}()
	dst := winapi.GlobalLock(mem)
	if dst == nil {
		return errors.New("clipboard: GlobalLock failed")
	}
	copy(unsafe.Slice((*uint16)(dst), len(u16)), u16)
	winapi.GlobalUnlock(mem)

	// EmptyClipboard makes this HWND the owner. A NULL owner would cause
	// SetClipboardData to fail even when OpenClipboard itself succeeds.
	if !winapi.OpenClipboard(c.platform.helperWindow) {
		return errors.New("clipboard: OpenClipboard failed")
	}
	defer winapi.CloseClipboard()
	if !winapi.EmptyClipboard() {
		return errors.New("clipboard: EmptyClipboard failed")
	}
	if winapi.SetClipboardData(winapi.CF_UNICODETEXT, mem) == 0 {
		return errors.New("clipboard: SetClipboardData failed")
	}
	// On success the system owns mem; do not free it.
	transferred = true
	return nil
}

func (c *clipboard) RequestText(callback func(text string, err error)) {
	if c.platform.destroyed {
		callback("", common.ErrUnavailable)
		return
	}
	text, err := readClipboardText()
	callback(text, err)
}

func readClipboardText() (string, error) {
	if !winapi.IsClipboardFormatAvailable(winapi.CF_UNICODETEXT) {
		return "", nil
	}
	if !winapi.OpenClipboard(0) {
		return "", errors.New("clipboard: OpenClipboard failed")
	}
	defer winapi.CloseClipboard()

	mem := winapi.GetClipboardData(winapi.CF_UNICODETEXT)
	if mem == 0 {
		return "", errors.New("clipboard: GetClipboardData failed")
	}
	ptr := winapi.GlobalLock(mem)
	if ptr == nil {
		return "", errors.New("clipboard: GlobalLock failed")
	}
	defer winapi.GlobalUnlock(mem)

	// Bound the terminator search by the allocated block. Clipboard data is
	// supplied by another process and need not be well formed.
	p := (*uint16)(ptr)
	limit := int(winapi.GlobalSize(mem) / 2)
	n := 0
	for n < limit && *(*uint16)(unsafe.Add(unsafe.Pointer(p), uintptr(n)*2)) != 0 {
		n++
	}
	if n == limit {
		return "", errors.New("clipboard: unterminated UTF-16 data")
	}
	return string(utf16.Decode(unsafe.Slice(p, n))), nil
}
