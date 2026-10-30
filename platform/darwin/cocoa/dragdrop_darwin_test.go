package cocoa

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"runtime"
	"slices"
	"testing"

	"github.com/ebitengine/purego/objc"
	"github.com/golang-gui/goui/platform/darwin/frameworks"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/appkit"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
	"github.com/golang-gui/goui/platform/dragdrop"
)

func TestMacDragFormatMapping(t *testing.T) {
	cases := []struct {
		format dragdrop.Format
		native string
	}{
		{dragdrop.FormatText, dragTextType},
		{dragdrop.FormatFiles, dragFileType},
		{dragdrop.FormatURLs, dragURLType},
		{dragdrop.FormatLocalMarker, dragLocalType},
		{dragdrop.MIMEFormat("application/x-goui-test"), dragMIMEPrefix + "6170706c69636174696f6e2f782d676f75692d74657374"},
	}
	for _, test := range cases {
		got, err := macType(test.format)
		if err != nil || got != test.native {
			t.Errorf("macType(%q) = %q, %v; want %q", test.format, got, err, test.native)
		}
	}
	if _, err := macType("mime:text/plain"); err == nil {
		t.Fatal("reserved MIME accepted")
	}
}

func TestMacDragActions(t *testing.T) {
	for _, actions := range []dragdrop.Action{0, dragdrop.Copy, dragdrop.Move, dragdrop.Link, dragdrop.AllActions} {
		if got := macActions(macOperation(actions)); got != actions {
			t.Fatalf("action round trip: %v -> %v", actions, got)
		}
	}
	if NSDragOperationMove != 16 || NSDragOperationLink != 2 || NSDragOperationCopy != 1 {
		t.Fatal("AppKit drag operation constants changed")
	}
}

func TestMacDragResult(t *testing.T) {
	for _, tt := range []struct {
		name      string
		operation uint
		cancel    bool
		kind      NSEventType
		key       uint16
		stamp     NSTimeInterval
		action    dragdrop.Action
		canceled  bool
	}{
		{"accepted", NSDragOperationCopy, false, NSEventTypeLeftMouseUp, 0, 11, dragdrop.Copy, false},
		{"accepted after cancel", NSDragOperationCopy, true, NSEventTypeLeftMouseUp, 0, 11, dragdrop.Copy, false},
		{"accepted beats Escape", NSDragOperationMove, true, NSEventTypeKeyDown, 53, 11, dragdrop.Move, false},
		{"unaccepted", 0, false, NSEventTypeLeftMouseUp, 0, 11, 0, false},
		{"program cancel", 0, true, NSEventTypeLeftMouseUp, 0, 11, 0, true},
		{"Escape", 0, false, NSEventTypeKeyDown, 53, 11, 0, true},
		{"stale Escape", 0, false, NSEventTypeKeyDown, 53, 9, 0, false},
		{"other key", 0, false, NSEventTypeKeyDown, 12, 11, 0, false},
		{"key up", 0, false, NSEventTypeKeyUp, 53, 11, 0, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := macDragResult(tt.operation, dragdrop.AllActions, tt.cancel, 10, tt.kind, tt.key, tt.stamp)
			if got.Action != tt.action || got.Canceled != tt.canceled || got.Validate() != nil {
				t.Fatalf("result=%+v; want action=%d canceled=%t", got, tt.action, tt.canceled)
			}
		})
	}
}

// 仅加载 framework、查询协议并注册测试类，不创建 NSWindow 或启动拖放会话。
func TestMacDragNativeProtocols(t *testing.T) {
	if err := frameworks.Init(); err != nil {
		t.Fatal(err)
	}
	if objc.GetProtocol("NSDraggingSource") == nil {
		t.Fatal("NSDraggingSource unavailable")
	}
	if objc.GetProtocol("NSDraggingDestination") == nil {
		t.Fatal("NSDraggingDestination unavailable")
	}
	view, err := ImplementNSView("GouiDragHeadlessTestView", NSViewOverride{})
	if err != nil {
		t.Fatal(err)
	}
	err = ImplementNSDragging(view, DraggingOverride{
		Entered:    func(NSView, DraggingInfo) uint { return 0 },
		Updated:    func(NSView, DraggingInfo) uint { return 0 },
		Exited:     func(NSView, DraggingInfo) {},
		Perform:    func(NSView, DraggingInfo) bool { return false },
		Conclude:   func(NSView, DraggingInfo) {},
		SourceMask: func(NSView, DraggingSession, uint) uint { return 0 },
		Ended:      func(NSView, DraggingSession, NSPoint, uint) {},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestMacDragPasteboardItemHeadless(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := frameworks.Init(); err != nil {
		t.Fatal(err)
	}
	AutoReleasePool(func() {
		item := NewPasteboardItem()
		defer item.Release()
		typ := ToNSString("org.golang-gui.goui.test-bytes")
		defer typ.Release()
		if !item.SetData(NewNativeData([]byte{0, 1, 2, 255}), typ) {
			t.Fatal("SetData failed")
		}
		if got := item.Data(typ).Bytes(); !slices.Equal(got, []byte{0, 1, 2, 255}) {
			t.Fatalf("bytes=%v", got)
		}
		path := ToNSString("/tmp/测试 file.txt")
		defer path.Release()
		url := NSURLClassId.FileURLWithPath(path)
		if got := url.AbsoluteString(); got != "file:///tmp/%E6%B5%8B%E8%AF%95%20file.txt" {
			t.Fatalf("URL=%q", got)
		}
		pixels := image.NewRGBA(image.Rect(0, 0, 2, 2))
		pixels.Set(0, 0, color.RGBA{R: 255, A: 255})
		var encoded bytes.Buffer
		if err := png.Encode(&encoded, pixels); err != nil {
			t.Fatal(err)
		}
		preview := NewNativeImage(NewNativeData(encoded.Bytes()))
		if !preview.Valid() {
			t.Fatal("NSImage from PNG failed")
		}
		defer preview.Release()
		dragItem := NewDraggingItem(item)
		if !dragItem.Valid() {
			t.Fatal("NSDraggingItem from pasteboard writer failed")
		}
		defer dragItem.Release()
		dragItem.SetFrame(NSMakeRect(0, 0, 2, 2), preview)
	})
}

func TestMacDragMultiFilePasteboardHeadless(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := frameworks.Init(); err != nil {
		t.Fatal(err)
	}
	AutoReleasePool(func() {
		pb := NSPasteboardClassId.UniquePasteboard()
		if !pb.Valid() {
			t.Fatal("private pasteboard unavailable")
		}
		defer pb.ReleaseGlobally()
		paths := []string{"/tmp/测试 A.txt", "/tmp/测试 B.txt"}
		ids := make([]objc.ID, 0, len(paths))
		for i, path := range paths {
			item := NewPasteboardItem()
			defer item.Release()
			uri, err := dragdrop.FileURLFromPath(path)
			if err != nil {
				t.Fatal(err)
			}
			if !macSetString(item, dragFileType, uri) || !macSetString(item, dragURLType, uri) {
				t.Fatal("URL representation refused")
			}
			if i == 0 && !macSetData(item, dragMIMEPrefix+"6170706c69636174696f6e2f782d676f75692d74657374", []byte{0, 1, 255}) {
				t.Fatal("MIME representation refused")
			}
			ids = append(ids, item.ID)
		}
		pb.ClearContents()
		if !pb.WriteObjects(Cast[NSArray](NSArray_arrayWithObjects(ids))) {
			t.Fatal("WriteObjects failed")
		}
		format := dragdrop.MIMEFormat("application/x-goui-test")
		available := macOfferFormats(pb, []dragdrop.Format{format, dragdrop.FormatURLs, dragdrop.FormatFiles, dragdrop.FormatText})
		if !slices.Equal(available, []dragdrop.Format{format, dragdrop.FormatURLs, dragdrop.FormatFiles}) {
			t.Fatalf("formats=%v", available)
		}
		files, err := readMacPasteboard(pb, dragdrop.FormatFiles)
		if err != nil {
			t.Fatal(err)
		}
		got, _ := files.Files()
		if !slices.Equal(got, paths) {
			t.Fatalf("files=%q", got)
		}
		urls, err := readMacPasteboard(pb, dragdrop.FormatURLs)
		if err != nil {
			t.Fatal(err)
		}
		list, _ := urls.URLs()
		if len(list) != 2 {
			t.Fatalf("URLs=%q", list)
		}
		bytes, err := readMacPasteboard(pb, format)
		if err != nil {
			t.Fatal(err)
		}
		value, _ := bytes.Bytes("application/x-goui-test")
		if !slices.Equal(value, []byte{0, 1, 255}) {
			t.Fatalf("MIME bytes=%v", value)
		}
	})
}
