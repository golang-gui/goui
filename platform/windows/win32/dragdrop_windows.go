package win32

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"math"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/dragdrop"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/windows/sdk/com"
	"github.com/golang-gui/goui/platform/windows/sdk/shell"
	"github.com/golang-gui/goui/platform/windows/sdk/winapi"
	xdraw "golang.org/x/image/draw"
)

type oleDragService struct {
	window           *Window
	target           *oleDropTarget
	offer            *oleOffer
	source           *oleSourceSession
	formats          []dragdrop.Format
	clipboardFormats map[dragdrop.Format][]uint16
	registered       bool
	destroyed        bool
	imageHelper      *shell.DropTargetHelper
	imageActive      bool
}

type oleOffer struct {
	service       *oleDragService
	id            uint64
	sourceID      uint64
	data          *com.DataObject
	formats       []dragdrop.Format
	nativeFormats map[dragdrop.Format]uint16
	actions       dragdrop.Action
	selected      dragdrop.Action
	finished      bool
	dropped       bool
	reading       bool
}

type oleDropTargetVTable struct {
	queryInterface uintptr
	addRef         uintptr
	release        uintptr
	dragEnter      uintptr
	dragOver       uintptr
	dragLeave      uintptr
	drop           uintptr
}

type oleDropTarget struct {
	vtable  *oleDropTargetVTable
	refs    atomic.Int32
	pin     runtime.Pinner
	service *oleDragService
}

var oleDropTargetTable = oleDropTargetVTable{
	queryInterface: syscall.NewCallback(oleTargetQueryInterface),
	addRef:         syscall.NewCallback(oleTargetAddRef),
	release:        syscall.NewCallback(oleTargetRelease),
	dragEnter:      syscall.NewCallback(oleTargetEnter),
	dragOver:       syscall.NewCallback(oleTargetOver),
	dragLeave:      syscall.NewCallback(oleTargetLeave),
	drop:           syscall.NewCallback(oleTargetDrop),
}

var oleDropTargetRoots sync.Map // uintptr -> *oleDropTarget, until final COM Release
var nextOLEOfferID uint64

func oleResult(hr com.HRESULT) uintptr { return uintptr(uint32(hr)) }

func newDragService(surface common.Surface) (common.DragDrop, error) {
	var w *Window
	switch typed := surface.(type) {
	case *Window:
		w = typed
	case *Popup:
		w = typed.win
	default:
		return nil, common.ErrUnsupported
	}
	if w == nil || w.hwnd == 0 || w.dnd != nil {
		return nil, common.ErrUnavailable
	}
	d := &oleDragService{window: w}
	w.dnd = d
	return d, nil
}

func (d *oleDragService) SetFormats(formats []dragdrop.Format) error {
	if d.destroyed || d.window == nil || d.window.hwnd == 0 {
		return common.ErrUnavailable
	}
	unique := make([]dragdrop.Format, 0, len(formats))
	registered := make(map[dragdrop.Format][]uint16)
	for _, format := range formats {
		if slices.Contains(unique, format) {
			continue
		}
		var ids []uint16
		switch format {
		case dragdrop.FormatFiles:
			ids = []uint16{winapi.CF_HDROP}
		case dragdrop.FormatText:
			ids = []uint16{winapi.CF_UNICODETEXT}
		case dragdrop.FormatURLs:
			ids = []uint16{winapi.RegisterClipboardFormat("UniformResourceLocatorW"), winapi.RegisterClipboardFormat("text/uri-list")}
		case dragdrop.FormatLocalMarker:
			ids = []uint16{winapi.RegisterClipboardFormat("GouiLocalDragMarker")}
		default:
			if !strings.HasPrefix(string(format), "mime:") || dragdrop.MIMEFormat(strings.TrimPrefix(string(format), "mime:")) != format {
				return fmt.Errorf("win32 dragdrop: invalid format %q", format)
			}
			ids = []uint16{winapi.RegisterClipboardFormat(strings.TrimPrefix(string(format), "mime:"))}
		}
		if slices.Contains(ids, 0) {
			return fmt.Errorf("win32 dragdrop: register format %q failed", format)
		}
		unique = append(unique, format)
		registered[format] = ids
	}
	if len(unique) == 0 && d.registered {
		if hr := com.RevokeDragDrop(uintptr(d.window.hwnd)); hr.Failed() {
			return fmt.Errorf("RevokeDragDrop: %w", hr)
		}
		d.registered = false
	}
	if len(unique) != 0 && !d.registered {
		if d.target == nil {
			d.target = newOLEDropTarget(d)
		}
		if hr := com.RegisterDragDrop(uintptr(d.window.hwnd), unsafe.Pointer(d.target)); hr.Failed() {
			return fmt.Errorf("RegisterDragDrop: %w", hr)
		}
		d.registered = true
		if d.imageHelper == nil {
			if helper, hr := shell.NewDropTargetHelper(); hr.Succeeded() {
				d.imageHelper = helper
			}
		}
	}
	d.formats, d.clipboardFormats = unique, registered
	return nil
}

func (d *oleDragService) Destroy() {
	if d.destroyed {
		return
	}
	d.destroyed = true
	d.Cancel()
	if d.imageActive && d.imageHelper != nil {
		d.imageHelper.Leave()
		d.imageActive = false
	}
	if d.offer != nil {
		d.offer.release()
	}
	if d.registered && d.window != nil && d.window.hwnd != 0 {
		_ = com.RevokeDragDrop(uintptr(d.window.hwnd))
		d.registered = false
	}
	if d.target != nil {
		d.target.service = nil
		d.target.releaseRef()
		d.target = nil
	}
	if d.imageHelper != nil {
		d.imageHelper.Release()
		d.imageHelper = nil
	}
	if d.window != nil {
		d.window.dnd = nil
	}
	d.window = nil
}

func newOLEDropTarget(service *oleDragService) *oleDropTarget {
	target := &oleDropTarget{vtable: &oleDropTargetTable, service: service}
	target.refs.Store(1)
	target.pin.Pin(target)
	target.pin.Pin(&oleDropTargetTable)
	oleDropTargetRoots.Store(uintptr(unsafe.Pointer(target)), target)
	return target
}

func (t *oleDropTarget) releaseRef() uint32 {
	count := t.refs.Add(-1)
	if count == 0 {
		oleDropTargetRoots.Delete(uintptr(unsafe.Pointer(t)))
		t.pin.Unpin()
	}
	return uint32(count)
}

func oleTargetQueryInterface(this, iid, out uintptr) uintptr {
	if out == 0 || iid == 0 {
		return oleResult(com.E_INVALIDARG)
	}
	*(*uintptr)(unsafe.Pointer(out)) = 0
	requested := *(*com.IID)(unsafe.Pointer(iid))
	if requested != com.IID_IUnknown && requested != com.IID_IDropTarget {
		return oleResult(com.E_NOINTERFACE)
	}
	*(*uintptr)(unsafe.Pointer(out)) = this
	t := (*oleDropTarget)(unsafe.Pointer(this))
	t.refs.Add(1)
	return 0
}

func oleTargetAddRef(this uintptr) uintptr {
	return uintptr((*oleDropTarget)(unsafe.Pointer(this)).refs.Add(1))
}

func oleTargetRelease(this uintptr) uintptr {
	return uintptr((*oleDropTarget)(unsafe.Pointer(this)).releaseRef())
}

func oleTargetEnter(this, object, keyState, packedPoint, effectPtr uintptr) uintptr {
	t := (*oleDropTarget)(unsafe.Pointer(this))
	if t.service == nil || effectPtr == 0 || object == 0 {
		return oleResult(com.E_INVALIDARG)
	}
	t.service.enter((*com.DataObject)(unsafe.Pointer(object)), uint32(keyState), packedPoint, (*uint32)(unsafe.Pointer(effectPtr)))
	return 0
}

func oleTargetOver(this, keyState, packedPoint, effectPtr uintptr) uintptr {
	t := (*oleDropTarget)(unsafe.Pointer(this))
	if t.service == nil || effectPtr == 0 {
		return oleResult(com.E_INVALIDARG)
	}
	t.service.over(uint32(keyState), packedPoint, (*uint32)(unsafe.Pointer(effectPtr)))
	return 0
}

func oleTargetLeave(this uintptr) uintptr {
	if service := (*oleDropTarget)(unsafe.Pointer(this)).service; service != nil {
		service.leave()
	}
	return 0
}

func oleTargetDrop(this, object, keyState, packedPoint, effectPtr uintptr) uintptr {
	t := (*oleDropTarget)(unsafe.Pointer(this))
	if t.service == nil || effectPtr == 0 {
		return oleResult(com.E_INVALIDARG)
	}
	t.service.drop(uint32(keyState), packedPoint, (*uint32)(unsafe.Pointer(effectPtr)))
	return 0
}

func olePoint(packed uintptr, hwnd winapi.HWND) geometry.Point {
	point := oleScreenPoint(packed)
	winapi.ScreenToClient(hwnd, &point)
	scale := hwndScale(hwnd)
	return geometry.Point{X: float32(point.X) / scale, Y: float32(point.Y) / scale}
}

func oleScreenPoint(packed uintptr) winapi.POINT {
	return winapi.POINT{X: winapi.LONG(int32(uint32(packed))), Y: winapi.LONG(int32(uint32(packed >> 32)))}
}

func oleSuggested(keyState uint32, actions dragdrop.Action) (dragdrop.Action, bool) {
	if keyState&winapi.MK_CONTROL != 0 && keyState&winapi.MK_SHIFT != 0 {
		return dragdrop.Link, true
	}
	if keyState&winapi.MK_CONTROL != 0 {
		return dragdrop.Copy, true
	}
	if keyState&winapi.MK_SHIFT != 0 {
		return dragdrop.Move, true
	}
	for _, action := range []dragdrop.Action{dragdrop.Copy, dragdrop.Move, dragdrop.Link} {
		if actions&action != 0 {
			return action, false
		}
	}
	return 0, false
}

func oleActions(mask uint32) dragdrop.Action {
	return dragdrop.Action(mask & uint32(dragdrop.AllActions))
}

func (d *oleDragService) enter(object *com.DataObject, key uint32, packed uintptr, effect *uint32) {
	d.leave()
	if d.destroyed || d.window == nil || len(d.formats) == 0 {
		*effect = 0
		return
	}
	formats := make([]dragdrop.Format, 0, len(d.formats))
	nativeFormats := make(map[dragdrop.Format]uint16)
	for _, format := range d.formats {
		if format == dragdrop.FormatLocalMarker {
			if activeOLESource != nil && activeOLESource.data != nil &&
				uintptr(unsafe.Pointer(activeOLESource.data)) == uintptr(unsafe.Pointer(object)) {
				formats = append(formats, format)
			}
			continue
		}
		for _, id := range d.clipboardFormats[format] {
			query := com.FormatEtc{Format: id, Aspect: com.DVAspectContent, Index: -1, Tymed: com.TymedHGlobal}
			if object.QueryGetData(&query).Succeeded() {
				formats = append(formats, format)
				nativeFormats[format] = id
				break
			}
		}
	}
	if len(formats) == 0 {
		*effect = 0
		d.enterDragImage(object, packed, *effect)
		return
	}
	object.AddRef()
	nextOLEOfferID++
	o := &oleOffer{service: d, id: nextOLEOfferID, data: object, formats: formats, nativeFormats: nativeFormats, actions: oleActions(*effect)}
	if activeOLESource != nil && uintptr(unsafe.Pointer(activeOLESource.data)) == uintptr(unsafe.Pointer(object)) {
		o.sourceID = activeOLESource.id
	}
	d.offer = o
	d.notify(o, events.DragEnter, key, packed, effect)
	if !d.destroyed {
		d.enterDragImage(object, packed, *effect)
	}
}

func (d *oleDragService) enterDragImage(object *com.DataObject, packed uintptr, effect uint32) {
	if d.imageHelper == nil || d.window == nil {
		return
	}
	point := oleScreenPoint(packed)
	if d.imageHelper.Enter(d.window.hwnd, unsafe.Pointer(object), &point, effect).Succeeded() {
		d.imageActive = true
	}
}

func (d *oleDragService) over(key uint32, packed uintptr, effect *uint32) {
	if d.offer == nil || d.destroyed {
		*effect = 0
		if d.imageActive && d.imageHelper != nil {
			point := oleScreenPoint(packed)
			d.imageHelper.Over(&point, 0)
		}
		return
	}
	d.notify(d.offer, events.DragMotion, key, packed, effect)
	if d.imageActive && d.imageHelper != nil {
		point := oleScreenPoint(packed)
		d.imageHelper.Over(&point, *effect)
	}
}

func (d *oleDragService) notify(o *oleOffer, typ events.EventType, key uint32, packed uintptr, effect *uint32) {
	allowed := o.actions & oleActions(*effect)
	suggested, forced := oleSuggested(key, allowed)
	answer := dragdrop.Action(0)
	d.window.onEvent(events.DragOfferEvent{EventType: typ, Position: olePoint(packed, d.window.hwnd),
		Offer: o, Actions: allowed, Suggested: suggested, Forced: forced, ActionReply: &answer})
	if d.destroyed || d.offer != o || !answer.ValidResult() || answer&allowed == 0 {
		*effect = 0
		o.selected = 0
	} else {
		*effect = uint32(answer)
		o.selected = answer
	}
}

func (d *oleDragService) leave() {
	if d.imageActive && d.imageHelper != nil {
		d.imageHelper.Leave()
		d.imageActive = false
	}
	o := d.offer
	if o == nil {
		return
	}
	d.offer = nil
	if !o.finished && d.window != nil && !d.destroyed {
		d.window.onEvent(events.DragOfferEvent{EventType: events.DragLeave, Offer: o})
	}
	o.release()
}

func (d *oleDragService) drop(key uint32, packed uintptr, effect *uint32) {
	o := d.offer
	if o == nil || d.destroyed || d.window == nil {
		*effect = 0
		if d.imageActive && d.imageHelper != nil {
			d.imageHelper.Leave()
			d.imageActive = false
		}
		return
	}
	o.dropped = true
	suggested, forced := oleSuggested(key, o.actions)
	d.window.onEvent(events.DragOfferEvent{EventType: events.DragDrop, Position: olePoint(packed, d.window.hwnd),
		Offer: o, Actions: o.actions, Suggested: suggested, Forced: forced})
	if !o.finished {
		_ = o.Finish(0)
	}
	*effect = uint32(o.selected)
	if d.imageActive && d.imageHelper != nil {
		point := oleScreenPoint(packed)
		d.imageHelper.Finish(unsafe.Pointer(o.data), &point, *effect)
		d.imageActive = false
	}
	o.release()
}

func (o *oleOffer) ID() uint64                 { return o.id }
func (o *oleOffer) SourceID() uint64           { return o.sourceID }
func (o *oleOffer) Formats() []dragdrop.Format { return slices.Clone(o.formats) }

func (o *oleOffer) Read(format dragdrop.Format) error {
	if o.finished || !o.dropped || o.reading || o.service.offer != o || !slices.Contains(o.formats, format) {
		return fmt.Errorf("win32 dragdrop: offer not readable")
	}
	o.reading = true
	query := com.FormatEtc{Format: o.nativeFormats[format], Aspect: com.DVAspectContent, Index: -1, Tymed: com.TymedHGlobal}
	var medium com.StgMedium
	hr := o.data.GetData(&query, &medium)
	if hr.Failed() {
		return fmt.Errorf("IDataObject.GetData: %w", hr)
	}
	defer com.ReleaseStgMedium(&medium)
	data, err := decodeOLEMedium(format, query.Format, &medium)
	if o.service.window != nil && !o.service.destroyed {
		o.service.window.onEvent(events.DragDataEvent{OfferID: o.id, Format: format, Data: data, Err: err})
	}
	return nil
}

func (o *oleOffer) Finish(action dragdrop.Action) error {
	if o.finished || !o.dropped || !action.ValidResult() || action&o.actions != action {
		return fmt.Errorf("win32 dragdrop: invalid finish action %d", action)
	}
	o.finished, o.selected = true, action
	return nil
}

func (o *oleOffer) release() {
	if o.data == nil {
		return
	}
	o.data.Release()
	o.data = nil
	if o.service.offer == o {
		o.service.offer = nil
	}
}

func decodeOLEMedium(format dragdrop.Format, nativeFormat uint16, medium *com.StgMedium) (*dragdrop.Data, error) {
	if medium.Tymed != com.TymedHGlobal || medium.Handle == 0 {
		return nil, fmt.Errorf("win32 dragdrop: unsupported storage medium")
	}
	data := new(dragdrop.Data)
	switch format {
	case dragdrop.FormatFiles:
		count := shell.DragQueryFileW(medium.Handle, ^uint32(0), nil, 0)
		if count == 0 || count > dragdrop.MaxItems {
			return nil, fmt.Errorf("win32 dragdrop: invalid file count %d", count)
		}
		paths := make([]string, 0, count)
		for i := uint32(0); i < count; i++ {
			length := shell.DragQueryFileW(medium.Handle, i, nil, 0)
			if length == 0 || length > 32767 {
				return nil, fmt.Errorf("win32 dragdrop: invalid file name length %d", length)
			}
			buffer := make([]uint16, length+1)
			if got := shell.DragQueryFileW(medium.Handle, i, &buffer[0], uint32(len(buffer))); got != length {
				return nil, fmt.Errorf("win32 dragdrop: file path changed during read")
			}
			paths = append(paths, string(utf16.Decode(buffer[:length])))
		}
		data.SetFiles(paths)
	case dragdrop.FormatText:
		text, err := readOLEUTF16(medium.Handle)
		if err != nil {
			return nil, err
		}
		data.SetText(text)
	case dragdrop.FormatURLs:
		if nativeFormat == winapi.RegisterClipboardFormat("text/uri-list") {
			bytes, err := readOLEBytes(medium.Handle)
			if err != nil {
				return nil, err
			}
			urls, err := dragdrop.DecodeURIList(bytes)
			if err != nil {
				return nil, err
			}
			data.SetURLs(urls)
		} else {
			text, err := readOLEUTF16(medium.Handle)
			if err != nil {
				return nil, err
			}
			data.SetURLs([]string{text})
		}
	default:
		if !strings.HasPrefix(string(format), "mime:") {
			return nil, common.ErrUnsupported
		}
		buffer, err := readOLEBytes(medium.Handle)
		if err != nil {
			return nil, err
		}
		data.SetBytes(strings.TrimPrefix(string(format), "mime:"), buffer)
	}
	return data, data.Validate()
}

func readOLEBytes(handle uintptr) ([]byte, error) {
	size := winapi.GlobalSize(winapi.HGLOBAL(handle))
	if size > dragdrop.MaxDataBytes {
		return nil, fmt.Errorf("win32 dragdrop: data exceeds %d bytes", dragdrop.MaxDataBytes)
	}
	ptr := winapi.GlobalLock(winapi.HGLOBAL(handle))
	if ptr == nil {
		return nil, fmt.Errorf("win32 dragdrop: GlobalLock failed")
	}
	defer winapi.GlobalUnlock(winapi.HGLOBAL(handle))
	return slices.Clone(unsafe.Slice((*byte)(ptr), int(size))), nil
}

func readOLEUTF16(handle uintptr) (string, error) {
	bytes, err := readOLEBytes(handle)
	if err != nil {
		return "", err
	}
	if len(bytes)%2 != 0 {
		return "", fmt.Errorf("win32 dragdrop: odd UTF-16 byte count")
	}
	units := make([]uint16, 0, len(bytes)/2)
	terminated := false
	for i := 0; i+1 < len(bytes); i += 2 {
		value := uint16(bytes[i]) | uint16(bytes[i+1])<<8
		if value == 0 {
			terminated = true
			break
		}
		units = append(units, value)
	}
	if !terminated {
		return "", fmt.Errorf("win32 dragdrop: unterminated UTF-16 string")
	}
	runes := utf16.Decode(units)
	if !slices.Equal(utf16.Encode(runes), units) {
		return "", fmt.Errorf("win32 dragdrop: malformed UTF-16 string")
	}
	return string(runes), nil
}

// InitializeFromBitmap multiplies RGB by alpha itself. The DIB fed to it must
// therefore contain straight (not graphics' usual premultiplied) BGRA bytes.
func initializeOLEPreview(data *oleDataObject, preview dragdrop.Preview, physicalScale float32) error {
	img := preview.Image
	if img == nil {
		return nil
	}
	scale := preview.Scale
	if scale == 0 {
		scale = physicalScale
	}
	bounds := img.Bounds()
	if int64(bounds.Dx())*int64(bounds.Dy()) > 16<<20 {
		return fmt.Errorf("win32 dragdrop: source preview exceeds pixel limit")
	}
	if math.Abs(float64(preview.Hotspot.X*physicalScale)) > 1<<20 || math.Abs(float64(preview.Hotspot.Y*physicalScale)) > 1<<20 {
		return fmt.Errorf("win32 dragdrop: preview hotspot exceeds native coordinate limit")
	}
	width := int(math.Round(float64(bounds.Dx()) * float64(physicalScale/scale)))
	height := int(math.Round(float64(bounds.Dy()) * float64(physicalScale/scale)))
	if width <= 0 || height <= 0 || width > 16384 || height > 16384 || int64(width)*int64(height) > 16<<20 {
		return fmt.Errorf("win32 dragdrop: preview exceeds physical image limit")
	}
	raster := image.NewNRGBA(image.Rect(0, 0, width, height))
	xdraw.CatmullRom.Scale(raster, raster.Bounds(), img, bounds, xdraw.Over, nil)
	bmi := winapi.BITMAPINFO{Header: winapi.BITMAPINFOHEADER{
		Size: winapi.Sizeof_BITMAPINFOHEADER, Width: winapi.LONG(width), Height: -winapi.LONG(height), Planes: 1, BitCount: 32,
	}}
	var bits unsafe.Pointer
	bitmap, err := winapi.CreateDIBSection(&bmi, &bits)
	if err != nil || bitmap == 0 || bits == nil {
		return fmt.Errorf("win32 dragdrop: CreateDIBSection: %w", err)
	}
	output := unsafe.Slice((*byte)(bits), width*height*4)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := color.NRGBAModel.Convert(raster.At(x, y)).(color.NRGBA)
			i := (y*width + x) * 4
			output[i+0], output[i+1], output[i+2], output[i+3] = c.B, c.G, c.R, c.A
		}
	}
	helper, hr := shell.NewDragSourceHelper()
	if hr.Failed() || helper == nil {
		winapi.DeleteObject(winapi.HGDIOBJ(bitmap))
		return fmt.Errorf("win32 dragdrop: create drag image helper: %w", hr)
	}
	defer helper.Release()
	var native shell.DragImage
	native.Size.Width, native.Size.Height = int32(width), int32(height)
	native.Hotspot = winapi.POINT{X: winapi.LONG(math.Round(float64(preview.Hotspot.X * physicalScale))), Y: winapi.LONG(math.Round(float64(preview.Hotspot.Y * physicalScale)))}
	native.Bitmap = bitmap
	native.ColorKey = ^uint32(0)
	hr = helper.InitializeFromBitmap(&native, unsafe.Pointer(data))
	if hr.Failed() {
		winapi.DeleteObject(winapi.HGDIOBJ(bitmap))
		return fmt.Errorf("win32 dragdrop: InitializeFromBitmap: %w", hr)
	}
	// On success, the Shell helper takes ownership of the HBITMAP.
	return nil
}

type oleSourceSession struct {
	service         *oleDragService
	window          *Window
	id              uint64
	data            *oleDataObject
	source          *oleSourceObject
	cancelRequested bool
}

type oleDataObjectVTable struct {
	queryInterface        uintptr
	addRef                uintptr
	release               uintptr
	getData               uintptr
	getDataHere           uintptr
	queryGetData          uintptr
	getCanonicalFormatEtc uintptr
	setData               uintptr
	enumFormatEtc         uintptr
	dAdvise               uintptr
	dUnadvise             uintptr
	enumDAdvise           uintptr
}

type oleDataObject struct {
	vtable  *oleDataObjectVTable
	refs    atomic.Int32
	pin     runtime.Pinner
	wire    map[uint16][]byte
	streams map[uint16]com.StgMedium // owned STGMEDIUMs supplied by Shell helpers
}

type oleSourceVTable struct {
	queryInterface    uintptr
	addRef            uintptr
	release           uintptr
	queryContinueDrag uintptr
	giveFeedback      uintptr
}

type oleSourceObject struct {
	vtable  *oleSourceVTable
	refs    atomic.Int32
	pin     runtime.Pinner
	session *oleSourceSession
}

var oleDataTable = oleDataObjectVTable{
	queryInterface:        syscall.NewCallback(oleDataQueryInterface),
	addRef:                syscall.NewCallback(oleDataAddRef),
	release:               syscall.NewCallback(oleDataRelease),
	getData:               syscall.NewCallback(oleDataGetData),
	getDataHere:           syscall.NewCallback(oleDataGetDataHere),
	queryGetData:          syscall.NewCallback(oleDataQueryGetData),
	getCanonicalFormatEtc: syscall.NewCallback(oleDataGetCanonicalFormatEtc),
	setData:               syscall.NewCallback(oleDataSetData),
	enumFormatEtc:         syscall.NewCallback(oleDataEnumFormatEtc),
	dAdvise:               syscall.NewCallback(oleDataDAdvise),
	dUnadvise:             syscall.NewCallback(oleDataDUnadvise),
	enumDAdvise:           syscall.NewCallback(oleDataEnumDAdvise),
}

var oleSourceTable = oleSourceVTable{
	queryInterface:    syscall.NewCallback(oleSourceQueryInterface),
	addRef:            syscall.NewCallback(oleSourceAddRef),
	release:           syscall.NewCallback(oleSourceRelease),
	queryContinueDrag: syscall.NewCallback(oleSourceQueryContinue),
	giveFeedback:      syscall.NewCallback(oleSourceFeedback),
}

var oleSourceRoots sync.Map
var activeOLESource *oleSourceSession // one OLE modal source on the GUI STA

func (d *oleDragService) Begin(id uint64, data *dragdrop.Data, actions dragdrop.Action, preview dragdrop.Preview) error {
	if d.destroyed || d.window == nil || d.window.hwnd == 0 || d.source != nil || activeOLESource != nil {
		return common.ErrUnavailable
	}
	if id == 0 || actions == 0 || !actions.ValidSet() {
		return fmt.Errorf("win32 dragdrop: invalid source id or actions")
	}
	if err := data.Validate(); err != nil {
		return err
	}
	if err := preview.Validate(); err != nil {
		return err
	}
	if !d.window.dragPress || !d.window.dragMotion || d.window.lastButtons&events.PointerButtonLeftDown == 0 {
		return fmt.Errorf("win32 dragdrop: Begin requires current native left-button motion")
	}
	wire, err := oleWireData(data)
	if err != nil {
		return err
	}
	session := &oleSourceSession{service: d, window: d.window, id: id}
	session.data = newOLEDataObject(wire)
	if preview.Image != nil {
		if err := initializeOLEPreview(session.data, preview, hwndScale(d.window.hwnd)); err != nil {
			session.data.releaseRef()
			return err
		}
	}
	session.source = newOLESourceObject(session)
	d.source, activeOLESource = session, session
	d.window.dragPress = false
	winapi.ReleaseCapture()
	session.window.onEvent(events.DragSourceEvent{EventType: events.DragSourceBegin, ID: id})
	var result dragdrop.Result
	if !d.destroyed && d.window != nil && d.window.hwnd != 0 && !session.cancelRequested {
		var effect uint32
		hr := com.DoDragDrop(unsafe.Pointer(session.data), unsafe.Pointer(session.source), uint32(actions), &effect)
		switch {
		case hr == com.DRAGDROP_S_CANCEL:
			result.Canceled = true
		case hr.Failed():
			result.Err = fmt.Errorf("DoDragDrop: %w", hr)
		default:
			result.Action = oleActions(effect)
			if !result.Action.ValidResult() || result.Action&actions == 0 {
				result.Action = 0
			}
		}
	} else {
		result.Canceled = true
	}
	if d.source == session {
		d.source = nil
	}
	if activeOLESource == session {
		activeOLESource = nil
	}
	session.source.session = nil
	session.window.onEvent(events.DragSourceEvent{EventType: events.DragSourceEnd, ID: id, Result: result})
	session.source.releaseRef()
	session.data.releaseRef()
	return nil
}

func (d *oleDragService) Cancel() {
	if d.source != nil {
		d.source.cancelRequested = true
	}
}

func oleWireData(data *dragdrop.Data) (map[uint16][]byte, error) {
	wire := make(map[uint16][]byte)
	for _, format := range data.Formats() {
		var id uint16
		var bytes []byte
		switch format {
		case dragdrop.FormatFiles:
			id = winapi.CF_HDROP
			paths, _ := data.Files()
			bytes = encodeOLEFiles(paths)
		case dragdrop.FormatText:
			id = winapi.CF_UNICODETEXT
			value, _ := data.Text()
			bytes = encodeOLEUTF16(value)
		case dragdrop.FormatURLs:
			urls, _ := data.URLs()
			if len(urls) == 1 {
				urlID := winapi.RegisterClipboardFormat("UniformResourceLocatorW")
				if urlID == 0 {
					return nil, fmt.Errorf("win32 dragdrop: register URL format failed")
				}
				wire[urlID] = encodeOLEUTF16(urls[0])
			}
			id = winapi.RegisterClipboardFormat("text/uri-list")
			var err error
			bytes, err = dragdrop.EncodeURIList(urls)
			if err != nil {
				return nil, err
			}
		default:
			if !strings.HasPrefix(string(format), "mime:") {
				continue
			}
			name := strings.TrimPrefix(string(format), "mime:")
			id = winapi.RegisterClipboardFormat(name)
			bytes, _ = data.Bytes(name)
		}
		if id == 0 {
			return nil, fmt.Errorf("win32 dragdrop: register format %q failed", format)
		}
		wire[id] = bytes
	}
	local := winapi.RegisterClipboardFormat("GouiLocalDragMarker")
	if local == 0 {
		return nil, fmt.Errorf("win32 dragdrop: register local marker failed")
	}
	wire[local] = []byte{1}
	return wire, nil
}

func encodeOLEUTF16(value string) []byte {
	units := append(utf16.Encode([]rune(value)), 0)
	bytes := make([]byte, len(units)*2)
	for i, unit := range units {
		binary.LittleEndian.PutUint16(bytes[i*2:], unit)
	}
	return bytes
}

func encodeOLEFiles(paths []string) []byte {
	units := make([]uint16, 0)
	for _, path := range paths {
		units = append(units, utf16.Encode([]rune(path))...)
		units = append(units, 0)
	}
	units = append(units, 0)
	bytes := make([]byte, 20+len(units)*2)
	binary.LittleEndian.PutUint32(bytes[0:], 20) // DROPFILES.pFiles
	binary.LittleEndian.PutUint32(bytes[16:], 1) // DROPFILES.fWide
	for i, unit := range units {
		binary.LittleEndian.PutUint16(bytes[20+i*2:], unit)
	}
	return bytes
}

func newOLEDataObject(wire map[uint16][]byte) *oleDataObject {
	d := &oleDataObject{vtable: &oleDataTable, wire: wire}
	d.refs.Store(1)
	d.pin.Pin(d)
	d.pin.Pin(&oleDataTable)
	oleSourceRoots.Store(uintptr(unsafe.Pointer(d)), d)
	return d
}

func newOLESourceObject(session *oleSourceSession) *oleSourceObject {
	s := &oleSourceObject{vtable: &oleSourceTable, session: session}
	s.refs.Store(1)
	s.pin.Pin(s)
	s.pin.Pin(&oleSourceTable)
	oleSourceRoots.Store(uintptr(unsafe.Pointer(s)), s)
	return s
}

func (d *oleDataObject) releaseRef() uint32 {
	count := d.refs.Add(-1)
	if count == 0 {
		oleSourceRoots.Delete(uintptr(unsafe.Pointer(d)))
		for _, medium := range d.streams {
			m := medium
			com.ReleaseStgMedium(&m)
		}
		d.streams = nil
		d.pin.Unpin()
		d.wire = nil
	}
	return uint32(count)
}

func (s *oleSourceObject) releaseRef() uint32 {
	count := s.refs.Add(-1)
	if count == 0 {
		oleSourceRoots.Delete(uintptr(unsafe.Pointer(s)))
		s.pin.Unpin()
	}
	return uint32(count)
}

func oleDataQueryInterface(this, iid, out uintptr) uintptr {
	if iid == 0 || out == 0 {
		return oleResult(com.E_INVALIDARG)
	}
	*(*uintptr)(unsafe.Pointer(out)) = 0
	want := *(*com.IID)(unsafe.Pointer(iid))
	if want != com.IID_IUnknown && want != com.IID_IDataObject {
		return oleResult(com.E_NOINTERFACE)
	}
	*(*uintptr)(unsafe.Pointer(out)) = this
	(*oleDataObject)(unsafe.Pointer(this)).refs.Add(1)
	return 0
}

func oleDataAddRef(this uintptr) uintptr {
	return uintptr((*oleDataObject)(unsafe.Pointer(this)).refs.Add(1))
}
func oleDataRelease(this uintptr) uintptr {
	return uintptr((*oleDataObject)(unsafe.Pointer(this)).releaseRef())
}

func oleDataQueryGetData(this, formatPtr uintptr) uintptr {
	if formatPtr == 0 {
		return oleResult(com.E_INVALIDARG)
	}
	format := (*com.FormatEtc)(unsafe.Pointer(formatPtr))
	if format.Aspect != com.DVAspectContent || format.Index != -1 {
		return oleResult(com.DV_E_TYMED)
	}
	d := (*oleDataObject)(unsafe.Pointer(this))
	if medium, exists := d.streams[format.Format]; exists {
		if format.Tymed&medium.Tymed != 0 {
			return 0
		}
		return oleResult(com.DV_E_TYMED)
	}
	if format.Tymed&com.TymedHGlobal == 0 {
		return oleResult(com.DV_E_TYMED)
	}
	if _, exists := d.wire[format.Format]; !exists {
		return oleResult(com.DV_E_FORMATETC)
	}
	return 0
}

func oleDataGetData(this, formatPtr, mediumPtr uintptr) uintptr {
	if mediumPtr == 0 {
		return oleResult(com.E_INVALIDARG)
	}
	if result := oleDataQueryGetData(this, formatPtr); result != 0 {
		return result
	}
	format := (*com.FormatEtc)(unsafe.Pointer(formatPtr))
	d := (*oleDataObject)(unsafe.Pointer(this))
	if medium, exists := d.streams[format.Format]; exists {
		if medium.ReleaseUnknown != nil {
			(*com.Unknown)(medium.ReleaseUnknown).AddRef()
		} else {
			(*com.Unknown)(unsafe.Pointer(medium.Handle)).AddRef()
		}
		*(*com.StgMedium)(unsafe.Pointer(mediumPtr)) = medium
		return 0
	}
	data := d.wire[format.Format]
	size := len(data)
	if size == 0 {
		size = 1
	}
	mem := winapi.GlobalAlloc(winapi.GMEM_MOVEABLE, uintptr(size))
	if mem == 0 {
		return oleResult(com.HRESULT(-2147024882))
	} // E_OUTOFMEMORY
	ptr := winapi.GlobalLock(mem)
	if ptr == nil {
		winapi.GlobalFree(mem)
		return oleResult(com.HRESULT(-2147024882))
	}
	copy(unsafe.Slice((*byte)(ptr), size), data)
	winapi.GlobalUnlock(mem)
	*(*com.StgMedium)(unsafe.Pointer(mediumPtr)) = com.StgMedium{Tymed: com.TymedHGlobal, Handle: uintptr(mem)}
	return 0
}

func oleDataGetDataHere(_, _, _ uintptr) uintptr           { return oleResult(com.E_NOTIMPL) }
func oleDataGetCanonicalFormatEtc(_, _, _ uintptr) uintptr { return oleResult(com.E_NOTIMPL) }
func oleDataSetData(this, formatPtr, mediumPtr, release uintptr) uintptr {
	if this == 0 || formatPtr == 0 || mediumPtr == 0 {
		return oleResult(com.E_INVALIDARG)
	}
	format := (*com.FormatEtc)(unsafe.Pointer(formatPtr))
	medium := (*com.StgMedium)(unsafe.Pointer(mediumPtr))
	if format.Aspect != com.DVAspectContent || format.Index != -1 || format.Tymed&medium.Tymed == 0 || medium.Handle == 0 {
		return oleResult(com.DV_E_TYMED)
	}
	d := (*oleDataObject)(unsafe.Pointer(this))
	switch medium.Tymed {
	case com.TymedHGlobal:
		value, err := readOLEBytes(medium.Handle)
		if err != nil {
			return oleResult(com.E_INVALIDARG)
		}
		if old, exists := d.streams[format.Format]; exists {
			com.ReleaseStgMedium(&old)
			delete(d.streams, format.Format)
		}
		d.wire[format.Format] = value
		if release != 0 {
			com.ReleaseStgMedium(medium)
		}
	case com.TymedIStream:
		if d.streams == nil {
			d.streams = make(map[uint16]com.StgMedium)
		}
		if old, exists := d.streams[format.Format]; exists {
			com.ReleaseStgMedium(&old)
		}
		if release == 0 {
			if medium.ReleaseUnknown != nil {
				(*com.Unknown)(medium.ReleaseUnknown).AddRef()
			} else {
				(*com.Unknown)(unsafe.Pointer(medium.Handle)).AddRef()
			}
		}
		d.streams[format.Format] = *medium
		delete(d.wire, format.Format)
	default:
		return oleResult(com.DV_E_TYMED)
	}
	return 0
}

func oleDataEnumFormatEtc(this, direction, out uintptr) uintptr {
	if out == 0 {
		return oleResult(com.E_INVALIDARG)
	}
	*(*uintptr)(unsafe.Pointer(out)) = 0
	if direction != uintptr(com.DataDirGet) {
		return oleResult(com.E_NOTIMPL)
	}
	formats := make([]com.FormatEtc, 0)
	for id := range (*oleDataObject)(unsafe.Pointer(this)).wire {
		formats = append(formats, com.FormatEtc{Format: id, Aspect: com.DVAspectContent, Index: -1, Tymed: com.TymedHGlobal})
	}
	for id := range (*oleDataObject)(unsafe.Pointer(this)).streams {
		formats = append(formats, com.FormatEtc{Format: id, Aspect: com.DVAspectContent, Index: -1, Tymed: com.TymedIStream})
	}
	slices.SortFunc(formats, func(a, b com.FormatEtc) int { return int(a.Format) - int(b.Format) })
	var enumerator unsafe.Pointer
	hr := shell.CreateStdEnumFmtEtc(formats, &enumerator)
	if hr.Failed() {
		return oleResult(hr)
	}
	*(*uintptr)(unsafe.Pointer(out)) = uintptr(enumerator)
	return 0
}

func oleDataDAdvise(_, _, _, _, _ uintptr) uintptr { return oleResult(com.E_NOTIMPL) }
func oleDataDUnadvise(_, _ uintptr) uintptr        { return oleResult(com.E_NOTIMPL) }
func oleDataEnumDAdvise(_, _ uintptr) uintptr      { return oleResult(com.E_NOTIMPL) }

func oleSourceQueryInterface(this, iid, out uintptr) uintptr {
	if iid == 0 || out == 0 {
		return oleResult(com.E_INVALIDARG)
	}
	*(*uintptr)(unsafe.Pointer(out)) = 0
	want := *(*com.IID)(unsafe.Pointer(iid))
	if want != com.IID_IUnknown && want != com.IID_IDropSource {
		return oleResult(com.E_NOINTERFACE)
	}
	*(*uintptr)(unsafe.Pointer(out)) = this
	(*oleSourceObject)(unsafe.Pointer(this)).refs.Add(1)
	return 0
}

func oleSourceAddRef(this uintptr) uintptr {
	return uintptr((*oleSourceObject)(unsafe.Pointer(this)).refs.Add(1))
}
func oleSourceRelease(this uintptr) uintptr {
	return uintptr((*oleSourceObject)(unsafe.Pointer(this)).releaseRef())
}

func oleSourceQueryContinue(this, escape, keyState uintptr) uintptr {
	session := (*oleSourceObject)(unsafe.Pointer(this)).session
	if session == nil || session.cancelRequested || escape != 0 {
		return oleResult(com.DRAGDROP_S_CANCEL)
	}
	if keyState&winapi.MK_LBUTTON == 0 {
		return oleResult(com.DRAGDROP_S_DROP)
	}
	return 0
}

func oleSourceFeedback(_, _ uintptr) uintptr { return oleResult(com.DRAGDROP_S_USEDEFAULTCURSORS) }
