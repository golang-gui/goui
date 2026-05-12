package cairo

// Opaque types
type (
	Surface     uintptr
	Context     uintptr
	FontOptions uintptr
)

// Format is used to identify the memory format of image data
type Format int32

const (
	FormatInvalid   Format = -1
	FormatARGB32    Format = 0
	FormatRGB24     Format = 1
	FormatA8        Format = 2
	FormatA1        Format = 3
	FormatRGB16_565 Format = 4
	FormatRGB30     Format = 5
)

// Antialias specifies the type of antialiasing to do when rendering text or shapes
type Antialias int32

const (
	AntialiasDefault  Antialias = 0
	AntialiasNone     Antialias = 1
	AntialiasGray     Antialias = 2
	AntialiasSubpixel Antialias = 3
	AntialiasFast     Antialias = 4
	AntialiasGood     Antialias = 5
	AntialiasBest     Antialias = 6
)

// SubpixelOrder specifies the order of color elements within each pixel
type SubpixelOrder int32

const (
	SubpixelOrderDefault SubpixelOrder = 0
	SubpixelOrderRGB     SubpixelOrder = 1
	SubpixelOrderBGR     SubpixelOrder = 2
	SubpixelOrderVRGB    SubpixelOrder = 3
	SubpixelOrderVBGR    SubpixelOrder = 4
)

// HintStyle specifies the type of hinting to do on font outlines
type HintStyle int32

const (
	HintStyleDefault HintStyle = 0
	HintStyleNone    HintStyle = 1
	HintStyleSlight  HintStyle = 2
	HintStyleMedium  HintStyle = 3
	HintStyleFull    HintStyle = 4
)

// HintMetrics specifies whether to hint font metrics
type HintMetrics int32

const (
	HintMetricsDefault HintMetrics = 0
	HintMetricsOff     HintMetrics = 1
	HintMetricsOn      HintMetrics = 2
)

// FontType identifies the type of font backend
type FontType int32

const (
	FontTypeToy    FontType = 0
	FontTypeFT     FontType = 1
	FontTypeWin32  FontType = 2
	FontTypeQuartz FontType = 3
	FontTypeUser   FontType = 4
	FontTypeDWrite FontType = 5
)

type Status int32

const (
	StatusSuccess Status = iota
	StatusNoMemory
	StatusInvalidRestore
	StatusInvalidPopGroup
	StatusNoCurrentPoint
	StatusInvalidMatrix
	StatusInvalidStatus
	StatusNullPointer
	StatusInvalidString
	StatusInvalidPathData
	StatusReadError
	StatusWriteError
	StatusSurfaceFinished
	StatusSurfaceTypeMismatch
	StatusPatternTypeMismatch
	StatusInvalidContent
	StatusInvalidFormat
	StatusInvalidVisual
	StatusFileNotFound
	StatusInvalidDash
	StatusInvalidDscComment
	StatusInvalidIndex
	StatusClipNotRepresentable
	StatusTempFileError
	StatusInvalidStride
	StatusFontTypeMismatch
	StatusUserFontImmutable
	StatusUserFontError
	StatusNegativeCount
	StatusInvalidClusters
	StatusInvalidSlant
	StatusInvalidWeight
	StatusInvalidSize
	StatusUserFontNotImplemented
	StatusDeviceTypeMismatch
	StatusDeviceError
	StatusInvalidMeshConstruction
	StatusDeviceFinished
	StatusJBIG2GlobalMissing
	StatusPNGError
	StatusFreetypeError
	StatusWin32GDIError
	StatusTagError
)
