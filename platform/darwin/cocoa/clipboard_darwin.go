package cocoa

import (
	"errors"
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
	var err error
	AutoReleasePool(func() {
		pb := NSPasteboardClassId.GeneralPasteboard()
		pb.ClearContents()
		if !pb.SetStringForType(ToNSString(text), ToNSString(pasteboardTextType)) {
			err = errors.New("clipboard: pasteboard rejected text")
		}
	})
	return err
}

func (c *clipboard) RequestText(callback func(text string, err error)) {
	var (
		text string
		err  error
	)
	AutoReleasePool(func() {
		pb := NSPasteboardClassId.GeneralPasteboard()
		ns := pb.StringForType(ToNSString(pasteboardTextType))
		if ns.ID != 0 {
			text = ns.UTF8String()
			err = nil
		}
	})
	callback(text, err)
}
