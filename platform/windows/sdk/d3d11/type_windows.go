package d3d11

type DriverType uint32

const (
	D3D_DRIVER_TYPE_UNKNOWN  DriverType = 0
	D3D_DRIVER_TYPE_HARDWARE DriverType = 1
	D3D_DRIVER_TYPE_WARP     DriverType = 5
)

type CreateDeviceFlag uint32

const (
	D3D11_CREATE_DEVICE_BGRA_SUPPORT CreateDeviceFlag = 0x20
	D3D11_SDK_VERSION                                 = 7
)

type FeatureLevel uint32

const (
	D3D_FEATURE_LEVEL_11_1 FeatureLevel = 0xb100
	D3D_FEATURE_LEVEL_11_0 FeatureLevel = 0xb000
	D3D_FEATURE_LEVEL_10_1 FeatureLevel = 0xa100
	D3D_FEATURE_LEVEL_10_0 FeatureLevel = 0xa000
)
