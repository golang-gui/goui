package wic

import (
	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/windows/sdk/com"
)

var (
	wic = cgo.NewLazyLibrary("windowscodecs.dll")

	CLSID_WICImagingFactor   = com.DefineGuid(0xcacaf262, 0x9370, 0x4615, 0xa1, 0x3b, 0x9f, 0x55, 0x39, 0xda, 0x4c, 0xa)
	CLSID_WICImagingFactory1 = com.DefineGuid(0xcacaf262, 0x9370, 0x4615, 0xa1, 0x3b, 0x9f, 0x55, 0x39, 0xda, 0x4c, 0xa)
	CLSID_WICImagingFactory2 = com.DefineGuid(0x317d06e8, 0x5f24, 0x433d, 0xbd, 0xf7, 0x79, 0xce, 0x68, 0xd8, 0xab, 0xc2)
)

func CreateImagingFactory[F isImagingFactory](clsid com.CLSID, iid com.IID) (factory *F, err error) {
	if err = wic.Load(); err == nil {
		var hr com.HRESULT
		factory, hr = com.CreateInstance[F](clsid, nil, com.CLSCTX_INPROC_SERVER, iid)
		if hr.Failed() {
			err = hr
		}
	}
	return
}

var IID_IWICImagingFactory = com.DefineGuid(0xec5ec8a9, 0xc395, 0x4314, 0x9c, 0x77, 0x54, 0xd7, 0xa9, 0x35, 0xff, 0x70)

type isImagingFactory interface {
	IsUnknown()
	isImagingFactory()
}

type ImagingFactoryClass struct {
	com.UnknownClass

	CreateDecoderFromFilename                cgo.Symbol //HRESULT(IWICImagingFactory *This,	LPCWSTR wzFilename,	const GUID *pguidVendor,	DWORD dwDesiredAccess,	WICDecodeOptions metadataOptions,	IWICBitmapDecoder **ppIDecoder)
	CreateDecoderFromStream                  cgo.Symbol //HRESULT(IWICImagingFactory *This,	IStream *pIStream,	const GUID *pguidVendor,	WICDecodeOptions metadataOptions,	IWICBitmapDecoder **ppIDecoder)
	CreateDecoderFromFileHandle              cgo.Symbol //HRESULT(IWICImagingFactory *This,	ULONG_PTR hFile,	const GUID *pguidVendor,	WICDecodeOptions metadataOptions,	IWICBitmapDecoder **ppIDecoder)
	CreateComponentInfo                      cgo.Symbol //HRESULT(IWICImagingFactory *This,	REFCLSID clsidComponent,	IWICComponentInfo **ppIInfo)
	CreateDecoder                            cgo.Symbol //HRESULT(IWICImagingFactory *This,	REFGUID guidContainerFormat,	const GUID *pguidVendor,	IWICBitmapDecoder **ppIDecoder)
	CreateEncoder                            cgo.Symbol //HRESULT(IWICImagingFactory *This,	REFGUID guidContainerFormat,	const GUID *pguidVendor,	IWICBitmapEncoder **ppIEncoder)
	CreatePalette                            cgo.Symbol //HRESULT(IWICImagingFactory *This,	IWICPalette **ppIPalette)
	CreateFormatConverter                    cgo.Symbol //HRESULT(IWICImagingFactory *This,	IWICFormatConverter **ppIFormatConverter)
	CreateBitmapScaler                       cgo.Symbol //HRESULT(IWICImagingFactory *This,	IWICBitmapScaler **ppIBitmapScaler)
	CreateBitmapClipper                      cgo.Symbol //HRESULT(IWICImagingFactory *This,	IWICBitmapClipper **ppIBitmapClipper)
	CreateBitmapFlipRotator                  cgo.Symbol //HRESULT(IWICImagingFactory *This,	IWICBitmapFlipRotator **ppIBitmapFlipRotator)
	CreateStream                             cgo.Symbol //HRESULT(IWICImagingFactory *This,	IWICStream **ppIWICStream)
	CreateColorContext                       cgo.Symbol //HRESULT(IWICImagingFactory *This,	IWICColorContext **ppIWICColorContext)
	CreateColorTransformer                   cgo.Symbol //HRESULT(IWICImagingFactory *This,	IWICColorTransform **ppIWICColorTransform)
	CreateBitmap                             cgo.Symbol //HRESULT(IWICImagingFactory *This,	UINT uiWidth,	UINT uiHeight,	REFWICPixelFormatGUID pixelFormat,	WICBitmapCreateCacheOption option,	IWICBitmap **ppIBitmap)
	CreateBitmapFromSource                   cgo.Symbol //HRESULT(IWICImagingFactory *This,	IWICBitmapSource *piBitmapSource,	WICBitmapCreateCacheOption option,	IWICBitmap **ppIBitmap)
	CreateBitmapFromSourceRect               cgo.Symbol //HRESULT(IWICImagingFactory *This,	IWICBitmapSource *piBitmapSource,	UINT x,	UINT y,	UINT width,	UINT height,	IWICBitmap **ppIBitmap)
	CreateBitmapFromMemory                   cgo.Symbol //HRESULT(IWICImagingFactory *This,	UINT uiWidth,	UINT uiHeight,	REFWICPixelFormatGUID pixelFormat,	UINT cbStride,	UINT cbBufferSize,	BYTE *pbBuffer,	IWICBitmap **ppIBitmap)
	CreateBitmapFromHBITMAP                  cgo.Symbol //HRESULT(IWICImagingFactory *This,	HBITMAP hBitmap,	HPALETTE hPalette,	WICBitmapAlphaChannelOption options,	IWICBitmap **ppIBitmap)
	CreateBitmapFromHICON                    cgo.Symbol //HRESULT(IWICImagingFactory *This,	HICON hIcon,	IWICBitmap **ppIBitmap)
	CreateComponentEnumerator                cgo.Symbol //HRESULT(IWICImagingFactory *This,	DWORD componentTypes,	DWORD options,	IEnumUnknown **ppIEnumUnknown)
	CreateFastMetadataEncoderFromDecoder     cgo.Symbol //HRESULT(IWICImagingFactory *This,	IWICBitmapDecoder *pIDecoder,	IWICFastMetadataEncoder **ppIFastEncoder)
	CreateFastMetadataEncoderFromFrameDecode cgo.Symbol //HRESULT(IWICImagingFactory *This,	IWICBitmapFrameDecode *pIFrameDecoder,	IWICFastMetadataEncoder **ppIFastEncoder)
	CreateQueryWriter                        cgo.Symbol //HRESULT(IWICImagingFactory *This,	REFGUID guidMetadataFormat,	const GUID *pguidVendor,	IWICMetadataQueryWriter **ppIQueryWriter)
	CreateQueryWriterFromReader              cgo.Symbol //HRESULT(IWICImagingFactory *This,	IWICMetadataQueryReader *pIQueryReader,	const GUID *pguidVendor,	IWICMetadataQueryWriter **ppIQueryWriter)
}

type ImagingFactory struct {
	com.Unknown
}

func (ImagingFactory) isImagingFactory() {}

func (this *ImagingFactory) CreateBitmap(width, height int, pixelFormat com.GUID, option BitmapCreateCacheOption) (bitmap *Bitmap, hr com.HRESULT) {
	ret, _, _ := this.class().CreateBitmap.CallRaw(uintptr(cgo.Pointer(this)), uintptr(width), uintptr(height), uintptr(cgo.Pointer(&pixelFormat)), uintptr(option), uintptr(cgo.Pointer(&bitmap)))
	hr = com.HRESULT(ret)
	return
}

func (this *ImagingFactory) CreateBitmapFromMemory(width, height int, pixelFormat com.GUID, stride int, data []byte) (bitmap *Bitmap, hr com.HRESULT) {
	ret, _, _ := this.class().CreateBitmapFromMemory.CallRaw(uintptr(cgo.Pointer(this)), uintptr(width), uintptr(height), uintptr(cgo.Pointer(&pixelFormat)), uintptr(stride), uintptr(len(data)), uintptr(cgo.CSlice(data)), uintptr(cgo.Pointer(&bitmap)))
	hr = com.HRESULT(ret)
	return
}

func (this *ImagingFactory) class() *ImagingFactoryClass {
	return (*ImagingFactoryClass)(this.Class)
}

var IID_IWICBitmapSource = com.DefineGuid(0x00000120, 0xa8f2, 0x4877, 0xba, 0x0a, 0xfd, 0x2b, 0x66, 0x45, 0xfb, 0x94)

type BitmapSourceClass struct {
	com.UnknownClass

	GetSize        cgo.Symbol //HRESULT(IWICBitmapSource *This,	UINT *puiWidth,	UINT *puiHeight)
	GetPixelFormat cgo.Symbol //HRESULT(IWICBitmapSource *This,	WICPixelFormatGUID *pPixelFormat)
	GetResolution  cgo.Symbol //HRESULT(IWICBitmapSource *This,	double *pDpiX,	double *pDpiY)
	CopyPalette    cgo.Symbol //HRESULT(IWICBitmapSource *This,	IWICPalette *pIPalette)
	CopyPixels     cgo.Symbol //HRESULT(IWICBitmapSource *This,	const WICRect *prc,	UINT cbStride,	UINT cbBufferSize,	BYTE *pbBuffer)
}

type BitmapSource struct {
	com.Unknown
}

func (this *BitmapSource) CopyPixels(rect *Rect, stride int, data []byte) com.HRESULT {
	ret, _, _ := this.class().CopyPixels.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(rect)), uintptr(stride), uintptr(len(data)), uintptr(cgo.CSlice(data)))
	return com.HRESULT(ret)
}

func (this *BitmapSource) class() *BitmapSourceClass {
	return (*BitmapSourceClass)(this.Class)
}

var IID_IWICBitmap = com.DefineGuid(0x00000121, 0xa8f2, 0x4877, 0xba, 0x0a, 0xfd, 0x2b, 0x66, 0x45, 0xfb, 0x94)

type BitmapClass struct {
	BitmapSourceClass

	Lock          cgo.Symbol //HRESULT(IWICBitmap *This,	const WICRect *prcLock,	DWORD flags,	IWICBitmapLock **ppILock)
	SetPalette    cgo.Symbol //HRESULT(IWICBitmap *This,	IWICPalette *pIPalette)
	SetResolution cgo.Symbol //HRESULT(IWICBitmap *This,	double dpiX,	double dpiY)
}

type Bitmap struct {
	BitmapSource
}

func (this *Bitmap) class() *BitmapClass {
	return (*BitmapClass)(this.Class)
}
