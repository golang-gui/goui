package core_graphics

import (
	"errors"
	"github.com/ebitengine/purego"
	"github.com/golang-gui/goui/platform/cocoa/frameworks/common"
	"github.com/golang-gui/goui/platform/cocoa/frameworks/core_foundation"
)

var handle uintptr

func Init(load common.LoadFunc) (err error) {
	handle, err = load("CoreGraphics")
	if err != nil {
		return
	}

	return initGraphics()
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

func CGMakeRect(x, y, w, h CGFloat) CGRect {
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

func initGraphics() (err error) {
	defer func() {
		if msg := recover(); msg != nil {
			err = errors.New(msg.(string))
		}
	}()

	purego.RegisterLibFunc(&fnCGDataProviderCreateWithCFData, handle, "CGDataProviderCreateWithCFData")
	purego.RegisterLibFunc(&fnCGDataProviderRelease, handle, "CGDataProviderRelease")
	purego.RegisterLibFunc(&fnCGColorSpaceCreateDeviceRGB, handle, "CGColorSpaceCreateDeviceRGB")
	purego.RegisterLibFunc(&fnCGColorSpaceRelease, handle, "CGColorSpaceRelease")
	purego.RegisterLibFunc(&fnCGImageCreate, handle, "CGImageCreate")
	purego.RegisterLibFunc(&fnCGImageRelease, handle, "CGImageRelease")
	purego.RegisterLibFunc(&fnCGContextDrawImage, handle, "CGContextDrawImage")

	return nil
}

var (
	fnCGDataProviderCreateWithCFData func(data core_foundation.CFDataRef) CGDataProviderRef
	fnCGDataProviderRelease          func(p CGDataProviderRef)
	fnCGColorSpaceCreateDeviceRGB    func() CGColorSpaceRef
	fnCGColorSpaceRelease            func(p CGColorSpaceRef)
	fnCGImageCreate                  func(width, height, bitsPerComponent, bitsPerPixel, bytesPerRow uint, space CGColorSpaceRef, bitmapInfo CGBitmapInfo, provider CGDataProviderRef, decode *CGFloat, shouldInterpolate bool, intent CGColorRenderingIntent) CGImageRef
	fnCGImageRelease                 func(img CGImageRef)
	fnCGContextDrawImage             func(ctx CGContextRef, rect CGRect, img CGImageRef)
)

type CGDataProviderRef uintptr

func CGDataProviderCreateWithCFData(data core_foundation.CFDataRef) CGDataProviderRef {
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

type CGImageRef uintptr

func CGImageCreate(width, height, bitsPerComponent, bitsPerPixel, bytesPerRow uint, space CGColorSpaceRef, bitmapInfo CGBitmapInfo, provider CGDataProviderRef, decode *CGFloat, shouldInterpolate bool, intent CGColorRenderingIntent) CGImageRef {
	return fnCGImageCreate(width, height, bitsPerComponent, bitsPerPixel, bytesPerRow, space, bitmapInfo, provider, decode, shouldInterpolate, intent)
}

func CGImageRelease(img CGImageRef) {
	fnCGImageRelease(img)
}

type CGContextRef uintptr

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
