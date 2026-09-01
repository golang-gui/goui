package d3d11

import (
	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/windows/sdk/com"
)

var (
	d3d11             = cgo.NewLazyLibrary("d3d11.dll")
	d3d11CreateDevice = d3d11.NewSymbol("D3D11CreateDevice")
)

// Device and DeviceContext only need IUnknown for this backend. Direct2D uses
// the device through IDXGIDevice and the immediate context is released after
// creation.
type Device struct{ com.Unknown }
type DeviceContext struct{ com.Unknown }

func CreateDevice(driverType DriverType, flags CreateDeviceFlag, levels []FeatureLevel) (device *Device, selected FeatureLevel, immediate *DeviceContext, hr com.HRESULT) {
	if err := d3d11CreateDevice.Find(); err != nil {
		return nil, 0, nil, com.HRESULT(-2147467259)
	}
	ret, _, _ := d3d11CreateDevice.CallRaw(
		0,
		uintptr(driverType),
		0,
		uintptr(flags),
		uintptr(cgo.CSlice(levels)),
		uintptr(len(levels)),
		D3D11_SDK_VERSION,
		uintptr(cgo.Pointer(&device)),
		uintptr(cgo.Pointer(&selected)),
		uintptr(cgo.Pointer(&immediate)),
	)
	hr = com.HRESULT(ret)
	return
}
