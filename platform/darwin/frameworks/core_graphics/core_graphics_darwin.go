package core_graphics

import (
	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_foundation"
	"github.com/golang-gui/goui/platform/darwin/frameworks/utils"

	"github.com/goexlib/cgo"
)

var framework utils.Framework

func InitCoreGraphics() (err error) {
	framework, err = utils.LoadSystemFramework("CoreGraphics")
	if err != nil {
		return
	}

	return framework.LoadFunctions(functions)
}

// Types

type (
	CGFloat = float64

	CGPoint struct {
		X, Y CGFloat
	}

	CGSize struct {
		Width, Height CGFloat
	}

	CGRect struct {
		Origin CGPoint
		Size   CGSize
	}
)

func CGRectMake(x, y, w, h CGFloat) CGRect {
	return CGRect{
		Origin: CGPoint{
			X: x,
			Y: y,
		},
		Size: CGSize{
			Width:  w,
			Height: h,
		},
	}
}

// Core Graphics

type CGDataProviderRef uintptr

func CGDataProviderCreateWithCFData(data CFDataRef) CGDataProviderRef {
	return fnCGDataProviderCreateWithCFData(data)
}

func CGDataProviderRelease(dataProvider CGDataProviderRef) {
	fnCGDataProviderRelease(dataProvider)
}

type CGColorSpaceRef uintptr

func CGColorSpaceCreateDeviceRGB() CGColorSpaceRef {
	return fnCGColorSpaceCreateDeviceRGB()
}

func CGColorSpaceRelease(colorSpace CGColorSpaceRef) {
	fnCGColorSpaceRelease(colorSpace)
}

// CGColor

type CGColorRef uintptr

func CGColorCreate(space CGColorSpaceRef, components []CGFloat) CGColorRef {
	if len(components) == 0 {
		return 0
	}
	return fnCGColorCreate(space, cgo.Pointer(&components[0]))
}

func CGColorRelease(color CGColorRef) {
	fnCGColorRelease(color)
}

type CGImageRef uintptr

func CGImageCreate(width, height, bitsPerComponent, bitsPerPixel, bytesPerRow uint, space CGColorSpaceRef, bitmapInfo CGBitmapInfo, provider CGDataProviderRef, decode *CGFloat, shouldInterpolate bool, intent CGColorRenderingIntent) CGImageRef {
	return fnCGImageCreate(width, height, bitsPerComponent, bitsPerPixel, bytesPerRow, space, bitmapInfo, provider, decode, shouldInterpolate, intent)
}

func CGImageRelease(img CGImageRef) {
	fnCGImageRelease(img)
}

// CGPath

type CGPathRef = CFTypeRef
type CGMutablePathRef = CGPathRef

func CGPathCreateWithRect(rect CGRect, transform *CGAffineTransform) CGPathRef {
	return fnCGPathCreateWithRect(rect, transform)
}

func CGPathCreateMutable() CGMutablePathRef {
	return fnCGPathCreateMutable()
}

func CGPathAddRect(path CGMutablePathRef, transform *CGAffineTransform, rect CGRect) {
	fnCGPathAddRect(path, transform, rect)
}

func CGPathRelease(path CGPathRef) {
	fnCGPathRelease(path)
}

type CGContextRef uintptr

type CGAffineTransform struct {
	A, B, C, D, Tx, Ty CGFloat
}

var CGAffineTransformIdentity = CGAffineTransform{A: 1, D: 1}

type CGTextDrawingMode int32

const (
	CGTextFill CGTextDrawingMode = iota
	CGTextStroke
	CGTextFillStroke
	CGTextInvisible
	CGTextFillClip
	CGTextStrokeClip
	CGTextFillStrokeClip
	CGTextClip
)

func CGBitmapContextCreate(data []byte, width, height, bitsPerComponent, bytesPerRow int, space CGColorSpaceRef, bitmapInfo CGBitmapInfo) CGContextRef {
	return fnCGBitmapContextCreate(cgo.CSlice(data), width, height, bitsPerComponent, bytesPerRow, space, bitmapInfo)
}

func CGBitmapContextGetData(ctx CGContextRef) cgo.Pointer {
	return fnCGBitmapContextGetData(ctx)
}

func CGContextRelease(ctx CGContextRef) {
	fnCGContextRelease(ctx)
}

func CGContextTranslateCTM(ctx CGContextRef, tx, ty CGFloat) {
	fnCGContextTranslateCTM(ctx, tx, ty)
}

func CGContextScaleCTM(ctx CGContextRef, sx, sy CGFloat) {
	fnCGContextScaleCTM(ctx, sx, sy)
}

func CGContextConcatCTM(ctx CGContextRef, transform CGAffineTransform) {
	fnCGContextConcatCTM(ctx, transform)
}

func CGContextSetTextMatrix(ctx CGContextRef, t CGAffineTransform) {
	fnCGContextSetTextMatrix(ctx, t)
}

func CGContextGetTextMatrix(ctx CGContextRef) CGAffineTransform {
	return fnCGContextGetTextMatrix(ctx)
}

func CGContextSetTextPosition(ctx CGContextRef, x, y CGFloat) {
	fnCGContextSetTextPosition(ctx, x, y)
}

func CGContextSetShouldAntialias(ctx CGContextRef, shouldAntialias bool) {
	fnCGContextSetShouldAntialias(ctx, shouldAntialias)
}

func CGContextSetAllowsAntialiasing(ctx CGContextRef, allowsAntialiasing bool) {
	fnCGContextSetAllowsAntialiasing(ctx, allowsAntialiasing)
}

func CGContextSetShouldSmoothFonts(ctx CGContextRef, shouldSmoothFonts bool) {
	fnCGContextSetShouldSmoothFonts(ctx, shouldSmoothFonts)
}

func CGContextSetAllowsFontSmoothing(ctx CGContextRef, allowsFontSmoothing bool) {
	fnCGContextSetAllowsFontSmoothing(ctx, allowsFontSmoothing)
}

func CGContextSetShouldSubpixelPositionFonts(ctx CGContextRef, v bool) {
	fnCGContextSetShouldSubpixelPositionFonts(ctx, v)
}

func CGContextSetShouldSubpixelQuantizeFonts(ctx CGContextRef, v bool) {
	fnCGContextSetShouldSubpixelQuantizeFonts(ctx, v)
}

func CGContextSetGrayFillColor(ctx CGContextRef, gray, alpha CGFloat) {
	fnCGContextSetGrayFillColor(ctx, gray, alpha)
}

func CGContextSetRGBFillColor(ctx CGContextRef, red, green, blue, alpha CGFloat) {
	fnCGContextSetRGBFillColor(ctx, red, green, blue, alpha)
}

func CGContextFillRect(ctx CGContextRef, rect CGRect) {
	fnCGContextFillRect(ctx, rect)
}

func CGContextClearRect(ctx CGContextRef, rect CGRect) {
	fnCGContextClearRect(ctx, rect)
}

func CGContextSetTextDrawingMode(ctx CGContextRef, mode CGTextDrawingMode) {
	fnCGContextSetTextDrawingMode(ctx, mode)
}

func CGContextDrawImage(ctx CGContextRef, rect CGRect, img CGImageRef) {
	fnCGContextDrawImage(ctx, rect, img)
}

type CGInt32Enum int32

type CGColorRenderingIntent = CGInt32Enum

const (
	CGRenderingIntentDefault CGColorRenderingIntent = iota
	CGRenderingIntentAbsoluteColorimetric
	CGRenderingIntentRelativeColorimetric
	CGRenderingIntentPerceptual
	CGRenderingIntentSaturatio
)

type CGUint32Enum uint32

type CGImageAlphaInfo = CGUint32Enum

const (
	CGImageAlphaNone CGImageAlphaInfo = iota
	CGImageAlphaPremultipliedLast
	CGImageAlphaPremultipliedFirst
	CGImageAlphaLast
	CGImageAlphaFirst
	CGImageAlphaNoneSkipLast
	CGImageAlphaNoneSkipFirst
	CGImageAlphaOnly
)

type CGImageByteOrderInfo = CGUint32Enum

const (
	CGImageByteOrderMask     CGImageByteOrderInfo = 0x7000
	CGImageByteOrderDefault  CGImageByteOrderInfo = (0 << 12)
	CGImageByteOrder16Little CGImageByteOrderInfo = (1 << 12)
	CGImageByteOrder32Little CGImageByteOrderInfo = (2 << 12)
	CGImageByteOrder16Big    CGImageByteOrderInfo = (3 << 12)
	CGImageByteOrder32Big    CGImageByteOrderInfo = (4 << 12)
)

type CGBitmapInfo = CGUint32Enum

const (
	CGBitmapAlphaInfoMask     CGBitmapInfo = 0x1F
	CGBitmapFloatInfoMask     CGBitmapInfo = 0xF00
	CGBitmapFloatComponents   CGBitmapInfo = 1 << 8
	CGBitmapByteOrderMask     CGBitmapInfo = CGImageByteOrderMask
	CGBitmapByteOrderDefault  CGBitmapInfo = CGImageByteOrderDefault
	CGBitmapByteOrder16Little CGBitmapInfo = CGImageByteOrder16Little
	CGBitmapByteOrder32Little CGBitmapInfo = CGImageByteOrder32Little
	CGBitmapByteOrder16Big    CGBitmapInfo = CGImageByteOrder16Big
	CGBitmapByteOrder32Big    CGBitmapInfo = CGImageByteOrder32Big
	CGBitmapByteOrder16Host   CGBitmapInfo = CGBitmapByteOrder16Little
	CGBitmapByteOrder32Host   CGBitmapInfo = CGBitmapByteOrder32Little
)

// functions

var functions = []utils.Function{
	{Name: "CGDataProviderCreateWithCFData", PFunc: &fnCGDataProviderCreateWithCFData},
	{Name: "CGDataProviderRelease", PFunc: &fnCGDataProviderRelease},

	{Name: "CGColorSpaceCreateDeviceRGB", PFunc: &fnCGColorSpaceCreateDeviceRGB},
	{Name: "CGColorSpaceRelease", PFunc: &fnCGColorSpaceRelease},

	{Name: "CGColorCreate", PFunc: &fnCGColorCreate},
	{Name: "CGColorRelease", PFunc: &fnCGColorRelease},

	{Name: "CGImageCreate", PFunc: &fnCGImageCreate},
	{Name: "CGImageRelease", PFunc: &fnCGImageRelease},

	{Name: "CGPathCreateWithRect", PFunc: &fnCGPathCreateWithRect},
	{Name: "CGPathCreateMutable", PFunc: &fnCGPathCreateMutable},
	{Name: "CGPathAddRect", PFunc: &fnCGPathAddRect},
	{Name: "CGPathRelease", PFunc: &fnCGPathRelease},

	{Name: "CGBitmapContextCreate", PFunc: &fnCGBitmapContextCreate},
	{Name: "CGBitmapContextGetData", PFunc: &fnCGBitmapContextGetData},

	{Name: "CGContextRelease", PFunc: &fnCGContextRelease},
	{Name: "CGContextTranslateCTM", PFunc: &fnCGContextTranslateCTM},
	{Name: "CGContextScaleCTM", PFunc: &fnCGContextScaleCTM},
	{Name: "CGContextConcatCTM", PFunc: &fnCGContextConcatCTM},
	{Name: "CGContextSetTextMatrix", PFunc: &fnCGContextSetTextMatrix},
	{Name: "CGContextGetTextMatrix", PFunc: &fnCGContextGetTextMatrix},
	{Name: "CGContextSetTextPosition", PFunc: &fnCGContextSetTextPosition},
	{Name: "CGContextSetShouldAntialias", PFunc: &fnCGContextSetShouldAntialias},
	{Name: "CGContextSetAllowsAntialiasing", PFunc: &fnCGContextSetAllowsAntialiasing},
	{Name: "CGContextSetShouldSmoothFonts", PFunc: &fnCGContextSetShouldSmoothFonts},
	{Name: "CGContextSetAllowsFontSmoothing", PFunc: &fnCGContextSetAllowsFontSmoothing},
	{Name: "CGContextSetShouldSubpixelPositionFonts", PFunc: &fnCGContextSetShouldSubpixelPositionFonts},
	{Name: "CGContextSetShouldSubpixelQuantizeFonts", PFunc: &fnCGContextSetShouldSubpixelQuantizeFonts},
	{Name: "CGContextSetGrayFillColor", PFunc: &fnCGContextSetGrayFillColor},
	{Name: "CGContextSetRGBFillColor", PFunc: &fnCGContextSetRGBFillColor},
	{Name: "CGContextFillRect", PFunc: &fnCGContextFillRect},
	{Name: "CGContextClearRect", PFunc: &fnCGContextClearRect},
	{Name: "CGContextSetTextDrawingMode", PFunc: &fnCGContextSetTextDrawingMode},
	{Name: "CGContextDrawImage", PFunc: &fnCGContextDrawImage},
}

var (
	fnCGDataProviderCreateWithCFData func(data CFDataRef) CGDataProviderRef
	fnCGDataProviderRelease          func(p CGDataProviderRef)

	fnCGColorSpaceCreateDeviceRGB func() CGColorSpaceRef
	fnCGColorSpaceRelease         func(p CGColorSpaceRef)

	fnCGColorCreate  func(space CGColorSpaceRef, components cgo.Pointer) CGColorRef
	fnCGColorRelease func(color CGColorRef)

	fnCGImageCreate  func(width, height, bitsPerComponent, bitsPerPixel, bytesPerRow uint, space CGColorSpaceRef, bitmapInfo CGBitmapInfo, provider CGDataProviderRef, decode *CGFloat, shouldInterpolate bool, intent CGColorRenderingIntent) CGImageRef
	fnCGImageRelease func(img CGImageRef)

	fnCGPathCreateWithRect func(rect CGRect, transform *CGAffineTransform) CGPathRef
	fnCGPathCreateMutable  func() CGMutablePathRef
	fnCGPathAddRect        func(path CGMutablePathRef, transform *CGAffineTransform, rect CGRect)
	fnCGPathRelease        func(path CGPathRef)

	fnCGBitmapContextCreate  func(data cgo.Pointer, width, height, bitsPerComponent, bytesPerRow int, space CGColorSpaceRef, bitmapInfo CGBitmapInfo) CGContextRef
	fnCGBitmapContextGetData func(ctx CGContextRef) cgo.Pointer

	fnCGContextRelease                        func(ctx CGContextRef)
	fnCGContextTranslateCTM                   func(ctx CGContextRef, tx, ty CGFloat)
	fnCGContextScaleCTM                       func(ctx CGContextRef, sx, sy CGFloat)
	fnCGContextConcatCTM                      func(ctx CGContextRef, transform CGAffineTransform)
	fnCGContextSetTextMatrix                  func(ctx CGContextRef, t CGAffineTransform)
	fnCGContextGetTextMatrix                  func(ctx CGContextRef) CGAffineTransform
	fnCGContextSetTextPosition                func(ctx CGContextRef, x, y CGFloat)
	fnCGContextSetShouldAntialias             func(ctx CGContextRef, shouldAntialias bool)
	fnCGContextSetAllowsAntialiasing          func(ctx CGContextRef, allowsAntialiasing bool)
	fnCGContextSetShouldSmoothFonts           func(ctx CGContextRef, shouldSmoothFonts bool)
	fnCGContextSetAllowsFontSmoothing         func(ctx CGContextRef, allowsFontSmoothing bool)
	fnCGContextSetShouldSubpixelPositionFonts func(ctx CGContextRef, shouldSubpixelPositionFonts bool)
	fnCGContextSetShouldSubpixelQuantizeFonts func(ctx CGContextRef, shouldSubpixelQuantizeFonts bool)
	fnCGContextSetGrayFillColor               func(ctx CGContextRef, gray, alpha CGFloat)
	fnCGContextSetRGBFillColor                func(ctx CGContextRef, red, green, blue, alpha CGFloat)
	fnCGContextFillRect                       func(ctx CGContextRef, rect CGRect)
	fnCGContextClearRect                      func(ctx CGContextRef, rect CGRect)
	fnCGContextSetTextDrawingMode             func(ctx CGContextRef, mode CGTextDrawingMode)
	fnCGContextDrawImage                      func(ctx CGContextRef, rect CGRect, img CGImageRef)
)
