package win32

import (
	"image"
	"image/color"
	"runtime"
	"slices"
	"testing"
	"unsafe"

	"github.com/golang-gui/goui/platform/dragdrop"
	"github.com/golang-gui/goui/platform/windows/sdk/com"
	"github.com/golang-gui/goui/platform/windows/sdk/shell"
	"github.com/golang-gui/goui/platform/windows/sdk/winapi"
)

func TestOLEDragDropABI(t *testing.T) {
	if got := unsafe.Sizeof(com.FormatEtc{}); got != 32 {
		t.Fatalf("FORMATETC size = %d, want 32 on Windows amd64", got)
	}
	if got := unsafe.Sizeof(com.StgMedium{}); got != 24 {
		t.Fatalf("STGMEDIUM size = %d, want 24 on Windows amd64", got)
	}
	if got := unsafe.Sizeof(shell.DragImage{}); got != 32 {
		t.Fatalf("SHDRAGIMAGE size = %d, want 32 on Windows amd64", got)
	}
}

func TestOLEDragImageHeadless(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if hr := com.InitializeOLE(); hr.Failed() {
		t.Fatalf("OleInitialize: %v", hr)
	}
	defer com.UninitializeOLE()
	obj := newOLEDataObject(map[uint16][]byte{})
	defer obj.releaseRef()
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, G: 10, B: 20, A: 128})
	preview := dragdrop.Preview{Image: img, Scale: 1}
	if err := initializeOLEPreview(obj, preview, 1); err != nil {
		t.Fatal(err)
	}
	helper, hr := shell.NewDropTargetHelper()
	if hr.Failed() || helper == nil {
		t.Fatalf("IDropTargetHelper: %v", hr)
	}
	helper.Release()
}

func TestOLEDragDataObjectAndFileMediumHeadless(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if hr := com.InitializeOLE(); hr.Failed() {
		t.Fatalf("OleInitialize: %v", hr)
	}
	defer com.UninitializeOLE()
	path := `C:\Temp\GOUI 文件.txt`
	data := new(dragdrop.Data)
	data.SetFiles([]string{path})
	data.SetText("中文 text")
	data.SetBytes("application/x-goui-test", []byte{1, 2, 3})
	wire, err := oleWireData(data)
	if err != nil {
		t.Fatal(err)
	}
	obj := newOLEDataObject(wire)
	defer obj.releaseRef()
	iface := (*com.DataObject)(unsafe.Pointer(obj))
	custom := winapi.RegisterClipboardFormat("application/x-goui-test")
	query := com.FormatEtc{Format: custom, Aspect: com.DVAspectContent, Index: -1, Tymed: com.TymedHGlobal}
	var medium com.StgMedium
	if hr := iface.GetData(&query, &medium); hr.Failed() {
		t.Fatalf("GetData(custom): %v", hr)
	}
	got, err := readOLEBytes(medium.Handle)
	com.ReleaseStgMedium(&medium)
	if err != nil || !slices.Equal(got, []byte{1, 2, 3}) {
		t.Fatalf("custom bytes %v, err=%v", got, err)
	}
	for _, format := range []uint16{winapi.CF_HDROP, winapi.CF_UNICODETEXT} {
		query := com.FormatEtc{Format: format, Aspect: com.DVAspectContent, Index: -1, Tymed: com.TymedHGlobal}
		if hr := iface.QueryGetData(&query); hr.Failed() {
			t.Fatalf("QueryGetData(%d): %v", format, hr)
		}
		var medium com.StgMedium
		if hr := iface.GetData(&query, &medium); hr.Failed() {
			t.Fatalf("GetData(%d): %v", format, hr)
		}
		if medium.Tymed != com.TymedHGlobal || medium.Handle == 0 {
			t.Fatalf("medium = %+v", medium)
		}
		var decoded *dragdrop.Data
		if format == winapi.CF_HDROP {
			decoded, err = decodeOLEMedium(dragdrop.FormatFiles, format, &medium)
		} else {
			decoded, err = decodeOLEMedium(dragdrop.FormatText, format, &medium)
		}
		com.ReleaseStgMedium(&medium)
		if err != nil {
			t.Fatal(err)
		}
		if format == winapi.CF_HDROP {
			files, ok := decoded.Files()
			if !ok || !slices.Equal(files, []string{path}) {
				t.Fatalf("files = %q present=%t", files, ok)
			}
		} else if text, ok := decoded.Text(); !ok || text != "中文 text" {
			t.Fatalf("text = %q present=%t", text, ok)
		}
	}
	var enumerator uintptr
	if result := oleDataEnumFormatEtc(uintptr(unsafe.Pointer(obj)), uintptr(com.DataDirGet), uintptr(unsafe.Pointer(&enumerator))); result != 0 || enumerator == 0 {
		t.Fatalf("EnumFormatEtc result=%x ptr=%x", result, enumerator)
	}
	(*com.Unknown)(unsafe.Pointer(enumerator)).Release()
}

func TestOLEURIListRoundTripHeadless(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if hr := com.InitializeOLE(); hr.Failed() {
		t.Fatalf("OleInitialize: %v", hr)
	}
	defer com.UninitializeOLE()
	urls := []string{"https://example.org/a?x=1", "file:///C:/Temp/example.txt"}
	data := new(dragdrop.Data)
	data.SetURLs(urls)
	wire, err := oleWireData(data)
	if err != nil {
		t.Fatal(err)
	}
	obj := newOLEDataObject(wire)
	defer obj.releaseRef()
	iface := (*com.DataObject)(unsafe.Pointer(obj))
	id := winapi.RegisterClipboardFormat("text/uri-list")
	query := com.FormatEtc{Format: id, Aspect: com.DVAspectContent, Index: -1, Tymed: com.TymedHGlobal}
	var medium com.StgMedium
	if hr := iface.GetData(&query, &medium); hr.Failed() {
		t.Fatalf("GetData(uri-list): %v", hr)
	}
	decoded, err := decodeOLEMedium(dragdrop.FormatURLs, id, &medium)
	com.ReleaseStgMedium(&medium)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := decoded.URLs()
	if !ok || !slices.Equal(got, urls) {
		t.Fatalf("URLs=%q present=%t", got, ok)
	}
}
