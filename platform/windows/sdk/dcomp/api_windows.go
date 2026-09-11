package dcomp

import (
	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/windows/sdk/com"
	"github.com/golang-gui/goui/platform/windows/sdk/dxgi"
)

var (
	lib                     = cgo.NewLazyLibrary("dcomp.dll")
	createDevice            = lib.NewSymbol("DCompositionCreateDevice")
	IID_IDCompositionDevice = com.DefineGuid(0xc37ea93a, 0xe7aa, 0x450d, 0xb1, 0x6f, 0x97, 0x46, 0xcb, 0x04, 0x07, 0xf3)
)

type Device struct{ com.Unknown }
type Target struct{ com.Unknown }
type Visual struct{ com.Unknown }

// Vtables mirror dcomp.h, including overloaded slots in native COM order.
type DeviceClass struct {
	com.UnknownClass
	Commit, WaitForCommitCompletion, GetFrameStatistics                                             cgo.Symbol
	CreateTargetForHwnd, CreateVisual                                                               cgo.Symbol
	CreateSurface, CreateVirtualSurface, CreateSurfaceFromHandle, CreateSurfaceFromHwnd             cgo.Symbol
	CreateTranslateTransform, CreateScaleTransform, CreateRotateTransform, CreateSkewTransform      cgo.Symbol
	CreateMatrixTransform, CreateTransformGroup, CreateTranslateTransform3D, CreateScaleTransform3D cgo.Symbol
	CreateRotateTransform3D, CreateMatrixTransform3D, CreateTransform3DGroup                        cgo.Symbol
	CreateEffectGroup, CreateRectangleClip, CreateAnimation, CheckDeviceState                       cgo.Symbol
}

type TargetClass struct {
	com.UnknownClass
	SetRoot cgo.Symbol
}

type VisualClass struct {
	com.UnknownClass
	SetOffsetXAnimation, SetOffsetX, SetOffsetYAnimation, SetOffsetY        cgo.Symbol
	SetTransformObject, SetTransformMatrix, SetTransformParent, SetEffect   cgo.Symbol
	SetBitmapInterpolationMode, SetBorderMode, SetClipObject, SetClipRect   cgo.Symbol
	SetContent, AddVisual, RemoveVisual, RemoveAllVisuals, SetCompositeMode cgo.Symbol
}

func Available() error { return createDevice.Find() }

func CreateDevice(dxgiDevice *dxgi.Device) (device *Device, hr com.HRESULT) {
	if err := createDevice.Find(); err != nil {
		return nil, com.HRESULT(-2147467263)
	}
	ret, _, _ := createDevice.CallRaw(uintptr(cgo.Pointer(dxgiDevice)),
		uintptr(cgo.Pointer(&IID_IDCompositionDevice)), uintptr(cgo.Pointer(&device)))
	return device, com.HRESULT(ret)
}

func (d *Device) Commit() com.HRESULT {
	ret, _, _ := (*DeviceClass)(d.Class).Commit.CallRaw(uintptr(cgo.Pointer(d)))
	return com.HRESULT(ret)
}

func (d *Device) CreateTargetForHwnd(hwnd uintptr, topmost bool) (target *Target, hr com.HRESULT) {
	ret, _, _ := (*DeviceClass)(d.Class).CreateTargetForHwnd.CallRaw(
		uintptr(cgo.Pointer(d)), hwnd, uintptr(cgo.CBool(topmost)), uintptr(cgo.Pointer(&target)))
	return target, com.HRESULT(ret)
}

func (d *Device) CreateVisual() (visual *Visual, hr com.HRESULT) {
	ret, _, _ := (*DeviceClass)(d.Class).CreateVisual.CallRaw(uintptr(cgo.Pointer(d)), uintptr(cgo.Pointer(&visual)))
	return visual, com.HRESULT(ret)
}

func (t *Target) SetRoot(visual *Visual) com.HRESULT {
	ret, _, _ := (*TargetClass)(t.Class).SetRoot.CallRaw(uintptr(cgo.Pointer(t)), uintptr(cgo.Pointer(visual)))
	return com.HRESULT(ret)
}

func (v *Visual) SetContent(content *com.Unknown) com.HRESULT {
	ret, _, _ := (*VisualClass)(v.Class).SetContent.CallRaw(uintptr(cgo.Pointer(v)), uintptr(cgo.Pointer(content)))
	return com.HRESULT(ret)
}
