package cocoa

import (
	"github.com/golang-gui/goui/platform/common"

	. "github.com/golang-gui/goui/platform/darwin/frameworks/appkit"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
)

// NSPasteboardTypeString's documented UTI value; used directly to avoid binding
// the global framework symbol.
const pasteboardTextType = "public.utf8-plain-text"

type clipboard struct{}

func newClipboard() (common.Clipboard, error) {
	return &clipboard{}, nil
}

func (c *clipboard) SetText(text string) error {
	AutoReleasePool(func() {
		pb := NSPasteboardClassId.GeneralPasteboard()
		pb.ClearContents()
		pb.SetStringForType(ToNSString(text), ToNSString(pasteboardTextType))
	})
	return nil
}

func (c *clipboard) RequestText(callback func(text string, ok bool)) {
	var (
		text string
		ok   bool
	)
	AutoReleasePool(func() {
		pb := NSPasteboardClassId.GeneralPasteboard()
		ns := pb.StringForType(ToNSString(pasteboardTextType))
		if ns.ID != 0 {
			text = ns.UTF8String()
			ok = true
		}
	})
	callback(text, ok)
}
