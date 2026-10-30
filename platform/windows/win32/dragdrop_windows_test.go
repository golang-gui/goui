package win32

import (
	"image"
	"image/color"
	"runtime"
	"slices"
	"syscall"
	"testing"
	"unsafe"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/dragdrop"
	"github.com/golang-gui/goui/platform/windows/sdk/com"
	"github.com/golang-gui/goui/platform/windows/sdk/shell"
	"github.com/golang-gui/goui/platform/windows/sdk/winapi"
)

// 验证源对象的 COM 身份、ABI 和引用计数；不创建窗口、不进入 OLE 拖放循环。
func TestDragSourceIdentity(t *testing.T) {
	var table oleSourceVTable
	word := unsafe.Sizeof(uintptr(0))
	if unsafe.Sizeof(table) != 5*word || unsafe.Offsetof(table.queryContinueDrag) != 3*word || unsafe.Offsetof(table.giveFeedback) != 4*word {
		t.Fatal("IDropSource ABI layout")
	}
	s := newOLESourceObject(nil)
	defer s.releaseRef()
	base := (*com.Unknown)(unsafe.Pointer(s))
	var source *com.Unknown
	if hr := base.QueryInterface(com.IID_IDropSource, &source); hr.Failed() {
		t.Fatal(hr)
	}
	defer source.Release()
	if source != base || s.refs.Load() != 2 {
		t.Fatal("incorrect IDropSource identity/reference count")
	}
	var controlling *com.Unknown
	if hr := source.QueryInterface(com.IID_IUnknown, &controlling); hr.Failed() {
		t.Fatal(hr)
	}
	if controlling != base {
		t.Fatal("different controlling IUnknown")
	}
	if got := controlling.Release(); got != 2 {
		t.Fatalf("Release=%d", got)
	}
	if got := source.AddRef(); got != 3 {
		t.Fatalf("AddRef=%d", got)
	}
	if got := source.Release(); got != 2 {
		t.Fatalf("Release=%d", got)
	}
	// 临时诊断接口不再暴露；失败的查询必须清空输出且不增加引用。
	notifyIID := com.DefineGuid(0x0000012b, 0, 0, 0xc0, 0, 0, 0, 0, 0, 0, 0x46)
	unknown := base
	if hr := base.QueryInterface(notifyIID, &unknown); hr != com.E_NOINTERFACE || unknown != nil || s.refs.Load() != 2 {
		t.Fatalf("unsupported QI: hr=%v out=%p refs=%d", hr, unknown, s.refs.Load())
	}
}

// 通过真实 vtable 回调入口验证 x64 ABI；不创建窗口、不注册 OLE 拖放。
// 调用侧故意使用独立的 SyscallN 和已知寄存器值，不与被测端共享结构体桥接。
func TestDropTargetPointABI(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	d := &oleDragService{}
	target := newOLEDropTarget(d)
	defer target.releaseRef()
	object := new(com.DataObject)
	effect := new(uint32)
	var pin runtime.Pinner
	pin.Pin(object)
	pin.Pin(effect)
	defer pin.Unpin()
	this, data, out := uintptr(unsafe.Pointer(target)), uintptr(unsafe.Pointer(object)), uintptr(unsafe.Pointer(effect))
	const point = uintptr(0xffffffe300000011) // POINT{X: 17, Y: -29}
	for _, tc := range []struct {
		name string
		fn   uintptr
		args []uintptr
	}{
		{"DragEnter", target.vtable.dragEnter, []uintptr{this, data, 0x89abcdef, point, out}},
		{"DragOver", target.vtable.dragOver, []uintptr{this, 0x89abcdef, point, out}},
		{"Drop", target.vtable.drop, []uintptr{this, data, 0x89abcdef, point, out}},
	} {
		*effect = 0xfeedbeef
		ret, _, _ := syscall.SyscallN(tc.fn, tc.args...)
		if com.HRESULT(ret) != com.S_OK || *effect != 0 {
			t.Fatalf("%s: HRESULT=%#x effect=%#x", tc.name, ret, *effect)
		}
		// 无效输出参数应由原回调返回 E_INVALIDARG，验证 32-bit HRESULT。
		tc.args[len(tc.args)-1] = 0
		ret, _, _ = syscall.SyscallN(tc.fn, tc.args...)
		if com.HRESULT(ret) != com.E_INVALIDARG {
			t.Fatalf("%s nil effect: HRESULT=%#x", tc.name, ret)
		}
	}

	// 无有效 offer 的 DragOver 会把坐标传给预览 helper。用仅记录参数的
	// COM 替身观察真实回调解码出的坐标，不调用桌面 API 或坐标转换。
	var got winapi.POINT
	var calls int
	class := &shell.DropTargetHelperClass{DragOver: cgo.Symbol(syscall.NewCallback(func(this, point, effect uintptr) uintptr {
		got = *(*winapi.POINT)(unsafe.Pointer(point))
		calls++
		return 0
	}))}
	helper := &shell.DropTargetHelper{Unknown: com.Unknown{Class: unsafe.Pointer(class)}}
	pin.Pin(class)
	pin.Pin(helper)
	d.imageHelper, d.imageActive = helper, true
	for _, tc := range []struct {
		word uintptr
		want winapi.POINT
	}{
		{point, winapi.POINT{X: 17, Y: -29}},
		{0x0000001dffffffef, winapi.POINT{X: -17, Y: 29}},
		{0x7fffffff80000000, winapi.POINT{X: -2147483648, Y: 2147483647}},
	} {
		ret, _, _ := syscall.SyscallN(target.vtable.dragOver, this, 0, tc.word, out)
		if com.HRESULT(ret) != com.S_OK || got != tc.want {
			t.Fatalf("DragOver point: got=%v want=%v HRESULT=%#x", got, tc.want, ret)
		}
	}
	if calls != 3 {
		t.Fatalf("helper calls=%d, want 3", calls)
	}
}

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

func TestOLEFeedbackPreservesAcceptedAndCanceledNativeCursors(t *testing.T) {
	session := new(oleSourceSession)
	source := newOLESourceObject(session)
	defer source.releaseRef()
	// 动画控制独立于 OLE 光标反馈，不能覆盖原生光标。
	session.feedback.DisableReturnAnimation = true
	for _, tc := range []struct {
		fallback, canceled bool
		effect             uint32
	}{
		{false, false, 0},
		{true, false, uint32(dragdrop.Move)},
		{true, false, uint32(dragdrop.Copy)},
		{true, false, uint32(dragdrop.Move) | 0x80000000},
		{true, false, uint32(dragdrop.Copy) | 0x80000000},
		{true, true, 0},
		{true, true, 0x80000000},
	} {
		session.feedback.NeutralOutsideTargets, session.cancelRequested = tc.fallback, tc.canceled
		result := oleSourceFeedback(uintptr(unsafe.Pointer(source)), uintptr(tc.effect))
		if result != oleResult(com.DRAGDROP_S_USEDEFAULTCURSORS) {
			t.Fatalf("native feedback overridden for %+v: %x", tc, result)
		}
	}
}

func TestOLEFeedbackOperationBits(t *testing.T) {
	// Explorer 拒绝放置时可能只附加 DROPEFFECT_SCROLL；该标志不能压制
	// NeutralOutsideTargets，也不能被解释为已接受拖放。
	for _, effect := range []uint32{0, 0x80000000} {
		if oleActions(effect) != 0 {
			t.Fatalf("advisory bits became an operation: %x", effect)
		}
	}
	for _, action := range []dragdrop.Action{dragdrop.Copy, dragdrop.Move, dragdrop.Link} {
		if oleActions(uint32(action)|0x80000000) != action {
			t.Fatal("scroll altered the accepted operation")
		}
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
