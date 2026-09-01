package dxgi

import (
	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/windows/sdk/com"
)

var (
	IID_IDXGIDevice   = com.DefineGuid(0x54ec77fa, 0x1377, 0x44e6, 0x8c, 0x32, 0x88, 0xfd, 0x5f, 0x44, 0xc8, 0x4c)
	IID_IDXGIFactory2 = com.DefineGuid(0x50c83a1c, 0xe072, 0x4c48, 0x87, 0xb0, 0x36, 0x30, 0xfa, 0x36, 0xa6, 0xd0)
	IID_IDXGISurface  = com.DefineGuid(0xcafcb56c, 0x6ac3, 0x4889, 0xbf, 0x47, 0x9e, 0x23, 0xbb, 0xd2, 0x60, 0xec)
)

type ObjectClass struct {
	com.UnknownClass
	SetPrivateData          cgo.Symbol
	SetPrivateDataInterface cgo.Symbol
	GetPrivateData          cgo.Symbol
	GetParent               cgo.Symbol
}

type Object struct{ com.Unknown }

func (o *Object) GetParent(iid com.IID, result **com.Unknown) com.HRESULT {
	ret, _, _ := (*ObjectClass)(o.Class).GetParent.CallRaw(uintptr(cgo.Pointer(o)), uintptr(cgo.Pointer(&iid)), uintptr(cgo.Pointer(result)))
	return com.HRESULT(ret)
}

type DeviceClass struct {
	ObjectClass
	GetAdapter             cgo.Symbol
	CreateSurface          cgo.Symbol
	QueryResourceResidency cgo.Symbol
	SetGPUThreadPriority   cgo.Symbol
	GetGPUThreadPriority   cgo.Symbol
}

type Device struct{ Object }

func (d *Device) GetAdapter() (adapter *Adapter, hr com.HRESULT) {
	ret, _, _ := (*DeviceClass)(d.Class).GetAdapter.CallRaw(uintptr(cgo.Pointer(d)), uintptr(cgo.Pointer(&adapter)))
	return adapter, com.HRESULT(ret)
}

type AdapterClass struct {
	ObjectClass
	EnumOutputs           cgo.Symbol
	GetDesc               cgo.Symbol
	CheckInterfaceSupport cgo.Symbol
}

type Adapter struct{ Object }

type FactoryClass struct {
	ObjectClass
	EnumAdapters          cgo.Symbol
	MakeWindowAssociation cgo.Symbol
	GetWindowAssociation  cgo.Symbol
	CreateSwapChain       cgo.Symbol
	CreateSoftwareAdapter cgo.Symbol
}

type Factory1Class struct {
	FactoryClass
	EnumAdapters1 cgo.Symbol
	IsCurrent     cgo.Symbol
}

type Factory2Class struct {
	Factory1Class
	IsWindowedStereoEnabled       cgo.Symbol
	CreateSwapChainForHwnd        cgo.Symbol
	CreateSwapChainForCoreWindow  cgo.Symbol
	GetSharedResourceAdapterLuid  cgo.Symbol
	RegisterStereoStatusWindow    cgo.Symbol
	RegisterStereoStatusEvent     cgo.Symbol
	UnregisterStereoStatus        cgo.Symbol
	RegisterOcclusionStatusWindow cgo.Symbol
	RegisterOcclusionStatusEvent  cgo.Symbol
	UnregisterOcclusionStatus     cgo.Symbol
	CreateSwapChainForComposition cgo.Symbol
}

type Factory2 struct{ Object }

func (f *Factory2) CreateSwapChainForHwnd(device *com.Unknown, hwnd uintptr, desc *SwapChainDesc1) (swapChain *SwapChain1, hr com.HRESULT) {
	ret, _, _ := (*Factory2Class)(f.Class).CreateSwapChainForHwnd.CallRaw(
		uintptr(cgo.Pointer(f)), uintptr(cgo.Pointer(device)), hwnd,
		uintptr(cgo.Pointer(desc)), 0, 0, uintptr(cgo.Pointer(&swapChain)),
	)
	return swapChain, com.HRESULT(ret)
}

type DeviceSubObjectClass struct {
	ObjectClass
	GetDevice cgo.Symbol
}

type DeviceSubObject struct{ Object }

type SurfaceClass struct {
	DeviceSubObjectClass
	GetDesc cgo.Symbol
	Map     cgo.Symbol
	Unmap   cgo.Symbol
}

type Surface struct{ DeviceSubObject }

type SwapChainClass struct {
	DeviceSubObjectClass
	Present             cgo.Symbol
	GetBuffer           cgo.Symbol
	SetFullscreenState  cgo.Symbol
	GetFullscreenState  cgo.Symbol
	GetDesc             cgo.Symbol
	ResizeBuffers       cgo.Symbol
	ResizeTarget        cgo.Symbol
	GetContainingOutput cgo.Symbol
	GetFrameStatistics  cgo.Symbol
	GetLastPresentCount cgo.Symbol
}

type SwapChain1Class struct {
	SwapChainClass
	GetDesc1                 cgo.Symbol
	GetFullscreenDesc        cgo.Symbol
	GetHwnd                  cgo.Symbol
	GetCoreWindow            cgo.Symbol
	Present1                 cgo.Symbol
	IsTemporaryMonoSupported cgo.Symbol
	GetRestrictToOutput      cgo.Symbol
	SetBackgroundColor       cgo.Symbol
	GetBackgroundColor       cgo.Symbol
	SetRotation              cgo.Symbol
	GetRotation              cgo.Symbol
}

type SwapChain1 struct{ DeviceSubObject }

func (s *SwapChain1) Present(syncInterval, flags uint32) com.HRESULT {
	ret, _, _ := (*SwapChain1Class)(s.Class).Present.CallRaw(uintptr(cgo.Pointer(s)), uintptr(syncInterval), uintptr(flags))
	return com.HRESULT(ret)
}

func (s *SwapChain1) GetBuffer(index uint32, iid com.IID) (surface *Surface, hr com.HRESULT) {
	ret, _, _ := (*SwapChain1Class)(s.Class).GetBuffer.CallRaw(uintptr(cgo.Pointer(s)), uintptr(index), uintptr(cgo.Pointer(&iid)), uintptr(cgo.Pointer(&surface)))
	return surface, com.HRESULT(ret)
}

func (s *SwapChain1) ResizeBuffers(count, width, height uint32, format Format, flags uint32) com.HRESULT {
	ret, _, _ := (*SwapChain1Class)(s.Class).ResizeBuffers.CallRaw(uintptr(cgo.Pointer(s)), uintptr(count), uintptr(width), uintptr(height), uintptr(format), uintptr(flags))
	return com.HRESULT(ret)
}
