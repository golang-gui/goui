package core_text

import (
	"errors"
	"fmt"
	"unsafe"

	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_foundation"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_graphics"
	"github.com/golang-gui/goui/platform/darwin/frameworks/utils"

	"github.com/goexlib/cgo"
)

var framework utils.Framework

func InitCoreText() (err error) {
	framework, err = utils.LoadSystemFramework("CoreText")
	if err != nil {
		return
	}

	err = framework.LoadFunctions(functions)
	if err != nil {
		return
	}

	return framework.LoadConstants(constants)
}

// CTFont

type CTFontRef = CFTypeRef

func CTFontCreateWithName(name string, size float64, matrix *CGAffineTransform) CTFontRef {
	cfName := CFStringCreateWithString(name)
	defer CFRelease(cfName)
	return fnCTFontCreateWithName(cfName, size, matrix)
}

func CTFontCreateWithFontDescriptor(descriptor CTFontDescriptorRef, size float64, matrix *CGAffineTransform) CTFontRef {
	return fnCTFontCreateWithFontDescriptor(descriptor, size, matrix)
}

func CTFontGetAscent(font CTFontRef) float64 {
	return fnCTFontGetAscent(font)
}

func CTFontGetDescent(font CTFontRef) float64 {
	return fnCTFontGetDescent(font)
}

func CTFontGetLeading(font CTFontRef) float64 {
	return fnCTFontGetLeading(font)
}

func CTFontGetUnderlinePosition(font CTFontRef) float64 {
	return fnCTFontGetUnderlinePosition(font)
}

func CTFontGetUnderlineThickness(font CTFontRef) float64 {
	return fnCTFontGetUnderlineThickness(font)
}

func CTFontGetSize(font CTFontRef) float64 {
	return fnCTFontGetSize(font)
}

func CTFontCopyFamilyName(font CTFontRef) string {
	cfStr := fnCTFontCopyFamilyName(font)
	if cfStr == 0 {
		return ""
	}
	defer CFRelease(cfStr)
	return CFStringToString(cfStr)
}

// CTFontDescriptor

type CTFontDescriptorRef = CFTypeRef

func CTFontDescriptorCreateWithAttributes(attributes CFDictionaryRef) CTFontDescriptorRef {
	return fnCTFontDescriptorCreateWithAttributes(attributes)
}

func CTFontDescriptorCreateWithNameAndSize(name string, size float64) CTFontDescriptorRef {
	cfName := CFStringCreateWithString(name)
	defer CFRelease(cfName)
	return fnCTFontDescriptorCreateWithNameAndSize(cfName, size)
}

// CTFramesetter

type CTFramesetterRef = CFTypeRef

func CTFramesetterCreateWithAttributedString(attrString CFAttributedStringRef) CTFramesetterRef {
	return fnCTFramesetterCreateWithAttributedString(attrString)
}

func CTFramesetterCreateFrame(framesetter CTFramesetterRef, stringRange CFRange, path CGPathRef, frameAttributes CFDictionaryRef) CTFrameRef {
	return fnCTFramesetterCreateFrame(framesetter, stringRange, path, frameAttributes)
}

func CTFramesetterSuggestFrameSizeWithConstraints(framesetter CTFramesetterRef, stringRange CFRange, frameAttributes CFDictionaryRef, constraints CGSize) (CGSize, CFRange) {
	var fitRange CFRange
	size := fnCTFramesetterSuggestFrameSizeWithConstraints(framesetter, stringRange, frameAttributes, constraints, &fitRange)
	return size, fitRange
}

// CTFrame

type CTFrameRef = CFTypeRef

type CTLineRef = CFTypeRef

type CTLineTruncationType int32

const (
	KCTLineTruncationStart  CTLineTruncationType = 0
	KCTLineTruncationEnd    CTLineTruncationType = 1
	KCTLineTruncationMiddle CTLineTruncationType = 2
)

type CTLineBoundsOptions uint

const (
	KCTLineBoundsExcludeTypographicLeading CTLineBoundsOptions = 1 << 0
	KCTLineBoundsExcludeTypographicShifts  CTLineBoundsOptions = 1 << 1
	KCTLineBoundsUseHangingPunctuation     CTLineBoundsOptions = 1 << 2
	KCTLineBoundsUseGlyphPathBounds        CTLineBoundsOptions = 1 << 3
	KCTLineBoundsUseOpticalBounds          CTLineBoundsOptions = 1 << 4
	KCTLineBoundsIncludeLanguageExtents    CTLineBoundsOptions = 1 << 5
)

func CTFrameGetLines(frame CTFrameRef) CFArrayRef {
	return fnCTFrameGetLines(frame)
}

func CTFrameGetLineOrigins(frame CTFrameRef, r CFRange, origins []CGPoint) {
	if len(origins) == 0 {
		return
	}
	fnCTFrameGetLineOrigins(frame, r, cgo.CSlice(origins))
}

func CTFrameGetVisibleStringRange(frame CTFrameRef) CFRange {
	return fnCTFrameGetVisibleStringRange(frame)
}

func CTFrameDraw(frame CTFrameRef, ctx CGContextRef) {
	fnCTFrameDraw(frame, ctx)
}

func CTLineGetStringRange(line CTLineRef) CFRange {
	return fnCTLineGetStringRange(line)
}

func CTLineGetTypographicBounds(line CTLineRef) (width, ascent, descent, leading float64) {
	width = fnCTLineGetTypographicBounds(line, &ascent, &descent, &leading)
	return
}

func CTLineGetGlyphRuns(line CTLineRef) CFArrayRef {
	return fnCTLineGetGlyphRuns(line)
}

func CTLineDraw(line CTLineRef, ctx CGContextRef) {
	fnCTLineDraw(line, ctx)
}

func CTLineGetOffsetForStringIndex(line CTLineRef, charIndex int) (primaryOffset, secondaryOffset float64) {
	primaryOffset = fnCTLineGetOffsetForStringIndex(line, CFIndex(charIndex), &secondaryOffset)
	return
}

func CTLineGetStringIndexForPosition(line CTLineRef, position CGPoint) int {
	return int(fnCTLineGetStringIndexForPosition(line, position))
}

func CTLineGetBoundsWithOptions(line CTLineRef, options CTLineBoundsOptions) CGRect {
	return fnCTLineGetBoundsWithOptions(line, options)
}

// CTRun

type CTRunRef = CFTypeRef

type CTRunStatus uint32

const (
	KCTRunStatusNoStatus       CTRunStatus = 0
	KCTRunStatusRightToLeft    CTRunStatus = 1 << 0
	KCTRunStatusNonMonotonic   CTRunStatus = 1 << 1
	KCTRunStatusHasNonIdentity CTRunStatus = 1 << 2
)

func CTRunGetGlyphCount(run CTRunRef) int {
	return int(fnCTRunGetGlyphCount(run))
}

func CTRunGetStringRange(run CTRunRef) CFRange {
	return fnCTRunGetStringRange(run)
}

func CTRunGetTypographicBounds(run CTRunRef, r CFRange) (width, ascent, descent, leading float64) {
	width = fnCTRunGetTypographicBounds(run, r, &ascent, &descent, &leading)
	return
}

func CTRunGetPositions(run CTRunRef, r CFRange, positions []CGPoint) {
	if len(positions) == 0 {
		return
	}
	fnCTRunGetPositions(run, r, unsafe.Pointer(&positions[0]))
}

func CTRunGetStringIndices(run CTRunRef, r CFRange, indices []CFIndex) {
	if len(indices) == 0 {
		return
	}
	fnCTRunGetStringIndices(run, r, unsafe.Pointer(&indices[0]))
}

func CTRunGetStatus(run CTRunRef) CTRunStatus {
	return fnCTRunGetStatus(run)
}

func CTRunGetAttributes(run CTRunRef) CFDictionaryRef {
	return fnCTRunGetAttributes(run)
}

// CTTypesetter

type CTTypesetterRef = CFTypeRef

func CTTypesetterCreateWithAttributedString(attrString CFAttributedStringRef) CTTypesetterRef {
	return fnCTTypesetterCreateWithAttributedString(attrString)
}

func CTTypesetterSuggestLineBreak(typesetter CTTypesetterRef, startIndex int, width float64) int {
	return int(fnCTTypesetterSuggestLineBreak(typesetter, CFIndex(startIndex), width))
}

func CTTypesetterSuggestClusterBreak(typesetter CTTypesetterRef, startIndex int, width float64) int {
	return int(fnCTTypesetterSuggestClusterBreak(typesetter, CFIndex(startIndex), width))
}

func CTTypesetterCreateLine(typesetter CTTypesetterRef, stringRange CFRange) CTLineRef {
	return fnCTTypesetterCreateLine(typesetter, stringRange)
}

// CTFontCollection

type CTFontManagerScope uint32

const (
	KCTFontManagerScopeNone       CTFontManagerScope = 0
	KCTFontManagerScopeProcess    CTFontManagerScope = 1
	KCTFontManagerScopeUser       CTFontManagerScope = 2
	KCTFontManagerScopePersistent CTFontManagerScope = 2
	KCTFontManagerScopeSession    CTFontManagerScope = 3
)

func CTFontManagerRegisterFontsForURL(fontURL CFURLRef, scope CTFontManagerScope) error {
	var err CFErrorRef
	ok := fnCTFontManagerRegisterFontsForURL(fontURL, scope, &err)
	if err != 0 {
		defer CFRelease(err)
		return fmt.Errorf("register err: %d", CFErrorGetCode(err))
	}
	if !ok {
		return errors.New("register failed")
	}
	return nil
}

type CTParagraphStyleRef = CFTypeRef

type CTTextAlignment uint8

const (
	KCTTextAlignmentLeft      CTTextAlignment = 0
	KCTTextAlignmentRight     CTTextAlignment = 1
	KCTTextAlignmentCenter    CTTextAlignment = 2
	KCTTextAlignmentJustified CTTextAlignment = 3
	KCTTextAlignmentNatural   CTTextAlignment = 4
)

type CTLineBreakMode uint8

const (
	KCTLineBreakByWordWrapping     CTLineBreakMode = 0
	KCTLineBreakByCharWrapping     CTLineBreakMode = 1
	KCTLineBreakByClipping         CTLineBreakMode = 2
	KCTLineBreakByTruncatingHead   CTLineBreakMode = 3
	KCTLineBreakByTruncatingTail   CTLineBreakMode = 4
	KCTLineBreakByTruncatingMiddle CTLineBreakMode = 5
)

type CTParagraphStyleSpecifier uint32

const (
	KCTParagraphStyleSpecifierAlignment              CTParagraphStyleSpecifier = 0
	KCTParagraphStyleSpecifierFirstLineHeadIndent    CTParagraphStyleSpecifier = 1
	KCTParagraphStyleSpecifierHeadIndent             CTParagraphStyleSpecifier = 2
	KCTParagraphStyleSpecifierTailIndent             CTParagraphStyleSpecifier = 3
	KCTParagraphStyleSpecifierTabStops               CTParagraphStyleSpecifier = 4
	KCTParagraphStyleSpecifierDefaultTabInterval     CTParagraphStyleSpecifier = 5
	KCTParagraphStyleSpecifierLineBreakMode          CTParagraphStyleSpecifier = 6
	KCTParagraphStyleSpecifierLineHeightMultiple     CTParagraphStyleSpecifier = 7
	KCTParagraphStyleSpecifierMaximumLineHeight      CTParagraphStyleSpecifier = 8
	KCTParagraphStyleSpecifierMinimumLineHeight      CTParagraphStyleSpecifier = 9
	KCTParagraphStyleSpecifierLineSpacing            CTParagraphStyleSpecifier = 10 // deprecated
	KCTParagraphStyleSpecifierParagraphSpacing       CTParagraphStyleSpecifier = 11
	KCTParagraphStyleSpecifierParagraphSpacingBefore CTParagraphStyleSpecifier = 12
	KCTParagraphStyleSpecifierBaseWritingDirection   CTParagraphStyleSpecifier = 13
	KCTParagraphStyleSpecifierMaximumLineSpacing     CTParagraphStyleSpecifier = 14
	KCTParagraphStyleSpecifierMinimumLineSpacing     CTParagraphStyleSpecifier = 15
	KCTParagraphStyleSpecifierLineSpacingAdjustment  CTParagraphStyleSpecifier = 16
	KCTParagraphStyleSpecifierLineBoundsOptions      CTParagraphStyleSpecifier = 17
)

type CTWritingDirection int8

const (
	KCTWritingDirectionNatural     CTWritingDirection = -1
	KCTWritingDirectionLeftToRight CTWritingDirection = 0
	KCTWritingDirectionRightToLeft CTWritingDirection = 1
)

type CTParagraphStyleSetting struct {
	Spec      CTParagraphStyleSpecifier
	ValueSize uintptr
	Value     unsafe.Pointer
}

func CTParagraphStyleCreate(settings []CTParagraphStyleSetting) CTParagraphStyleRef {
	if len(settings) == 0 {
		return fnCTParagraphStyleCreate(nil, 0)
	}
	return fnCTParagraphStyleCreate(unsafe.Pointer(&settings[0]), uint(len(settings)))
}

func CreateParagraphStyle(alignment CTTextAlignment, lineBreakMode CTLineBreakMode) CTParagraphStyleRef {
	settings := []CTParagraphStyleSetting{
		{
			Spec:      KCTParagraphStyleSpecifierAlignment,
			ValueSize: unsafe.Sizeof(alignment),
			Value:     unsafe.Pointer(&alignment),
		},
		{
			Spec:      KCTParagraphStyleSpecifierLineBreakMode,
			ValueSize: unsafe.Sizeof(lineBreakMode),
			Value:     unsafe.Pointer(&lineBreakMode),
		},
	}
	return CTParagraphStyleCreate(settings)
}

func CreateParagraphStyleFull(alignment CTTextAlignment, lineBreakMode CTLineBreakMode, lineSpacing CGFloat) CTParagraphStyleRef {
	settings := []CTParagraphStyleSetting{
		{
			Spec:      KCTParagraphStyleSpecifierAlignment,
			ValueSize: unsafe.Sizeof(alignment),
			Value:     unsafe.Pointer(&alignment),
		},
		{
			Spec:      KCTParagraphStyleSpecifierLineBreakMode,
			ValueSize: unsafe.Sizeof(lineBreakMode),
			Value:     unsafe.Pointer(&lineBreakMode),
		},
		{
			Spec:      KCTParagraphStyleSpecifierLineSpacingAdjustment,
			ValueSize: unsafe.Sizeof(lineSpacing),
			Value:     unsafe.Pointer(&lineSpacing),
		},
	}
	return CTParagraphStyleCreate(settings)
}

// Constants

// CTUnderlineStyle values
type CTUnderlineStyle int32

const (
	KCTUnderlineStyleNone   CTUnderlineStyle = 0
	KCTUnderlineStyleSingle CTUnderlineStyle = 1
	KCTUnderlineStyleThick  CTUnderlineStyle = 2
	KCTUnderlineStyleDouble CTUnderlineStyle = 9
)

var (
	KCTFontAttributeName                       CFStringRef
	KCTForegroundColorAttributeName            CFStringRef
	KCTForegroundColorFromContextAttributeName CFStringRef
	KCTParagraphStyleAttributeName             CFStringRef
	KCTUnderlineStyleAttributeName             CFStringRef
	KCTStrikethroughStyleAttributeName         CFStringRef
	KCTFontFamilyNameAttribute                 CFStringRef
	KCTFontSizeAttribute                       CFStringRef
	KCTFontTraitsAttribute                     CFStringRef
	KCTFontWeightTrait                         CFStringRef
	KCTFontWidthTrait                          CFStringRef
)

var constants = []utils.Constant{
	utils.Const[CFStringRef]{Name: "kCTFontAttributeName", PVar: &KCTFontAttributeName},
	utils.Const[CFStringRef]{Name: "kCTForegroundColorAttributeName", PVar: &KCTForegroundColorAttributeName},
	utils.Const[CFStringRef]{Name: "kCTForegroundColorFromContextAttributeName", PVar: &KCTForegroundColorFromContextAttributeName},
	utils.Const[CFStringRef]{Name: "kCTParagraphStyleAttributeName", PVar: &KCTParagraphStyleAttributeName},
	utils.Const[CFStringRef]{Name: "kCTUnderlineStyleAttributeName", PVar: &KCTUnderlineStyleAttributeName},
	utils.Const[CFStringRef]{Name: "kCTStrikethroughStyleAttributeName", PVar: &KCTStrikethroughStyleAttributeName},
	utils.Const[CFStringRef]{Name: "kCTFontFamilyNameAttribute", PVar: &KCTFontFamilyNameAttribute},
	utils.Const[CFStringRef]{Name: "kCTFontSizeAttribute", PVar: &KCTFontSizeAttribute},
	utils.Const[CFStringRef]{Name: "kCTFontTraitsAttribute", PVar: &KCTFontTraitsAttribute},
	utils.Const[CFStringRef]{Name: "kCTFontWeightTrait", PVar: &KCTFontWeightTrait},
	utils.Const[CFStringRef]{Name: "kCTFontWidthTrait", PVar: &KCTFontWidthTrait},
}

var functions = []utils.Function{
	{Name: "CTFontCreateWithName", PFunc: &fnCTFontCreateWithName},
	{Name: "CTFontCreateWithFontDescriptor", PFunc: &fnCTFontCreateWithFontDescriptor},
	{Name: "CTFontGetAscent", PFunc: &fnCTFontGetAscent},
	{Name: "CTFontGetDescent", PFunc: &fnCTFontGetDescent},
	{Name: "CTFontGetLeading", PFunc: &fnCTFontGetLeading},
	{Name: "CTFontGetUnderlinePosition", PFunc: &fnCTFontGetUnderlinePosition},
	{Name: "CTFontGetUnderlineThickness", PFunc: &fnCTFontGetUnderlineThickness},
	{Name: "CTFontGetSize", PFunc: &fnCTFontGetSize},
	{Name: "CTFontCopyFamilyName", PFunc: &fnCTFontCopyFamilyName},

	{Name: "CTFontDescriptorCreateWithAttributes", PFunc: &fnCTFontDescriptorCreateWithAttributes},
	{Name: "CTFontDescriptorCreateWithNameAndSize", PFunc: &fnCTFontDescriptorCreateWithNameAndSize},

	{Name: "CTFramesetterCreateWithAttributedString", PFunc: &fnCTFramesetterCreateWithAttributedString},
	{Name: "CTFramesetterCreateFrame", PFunc: &fnCTFramesetterCreateFrame},
	{Name: "CTFramesetterSuggestFrameSizeWithConstraints", PFunc: &fnCTFramesetterSuggestFrameSizeWithConstraints},

	{Name: "CTFrameGetLines", PFunc: &fnCTFrameGetLines},
	{Name: "CTFrameGetLineOrigins", PFunc: &fnCTFrameGetLineOrigins},
	{Name: "CTFrameGetVisibleStringRange", PFunc: &fnCTFrameGetVisibleStringRange},
	{Name: "CTFrameDraw", PFunc: &fnCTFrameDraw},

	{Name: "CTLineGetStringRange", PFunc: &fnCTLineGetStringRange},
	{Name: "CTLineGetTypographicBounds", PFunc: &fnCTLineGetTypographicBounds},
	{Name: "CTLineGetGlyphRuns", PFunc: &fnCTLineGetGlyphRuns},
	{Name: "CTLineDraw", PFunc: &fnCTLineDraw},
	{Name: "CTLineGetOffsetForStringIndex", PFunc: &fnCTLineGetOffsetForStringIndex},
	{Name: "CTLineGetStringIndexForPosition", PFunc: &fnCTLineGetStringIndexForPosition},
	{Name: "CTLineCreateTruncatedLine", PFunc: &fnCTLineCreateTruncatedLine},
	{Name: "CTLineGetBoundsWithOptions", PFunc: &fnCTLineGetBoundsWithOptions},

	{Name: "CTRunGetGlyphCount", PFunc: &fnCTRunGetGlyphCount},
	{Name: "CTRunGetStringRange", PFunc: &fnCTRunGetStringRange},
	{Name: "CTRunGetTypographicBounds", PFunc: &fnCTRunGetTypographicBounds},
	{Name: "CTRunGetPositions", PFunc: &fnCTRunGetPositions},
	{Name: "CTRunGetStringIndices", PFunc: &fnCTRunGetStringIndices},
	{Name: "CTRunGetStatus", PFunc: &fnCTRunGetStatus},
	{Name: "CTRunGetAttributes", PFunc: &fnCTRunGetAttributes},

	{Name: "CTTypesetterCreateWithAttributedString", PFunc: &fnCTTypesetterCreateWithAttributedString},
	{Name: "CTTypesetterSuggestLineBreak", PFunc: &fnCTTypesetterSuggestLineBreak},
	{Name: "CTTypesetterSuggestClusterBreak", PFunc: &fnCTTypesetterSuggestClusterBreak},
	{Name: "CTTypesetterCreateLine", PFunc: &fnCTTypesetterCreateLine},

	{Name: "CTFontManagerRegisterFontsForURL", PFunc: &fnCTFontManagerRegisterFontsForURL},

	{Name: "CTParagraphStyleCreate", PFunc: &fnCTParagraphStyleCreate},
}

var (
	fnCTFontCreateWithName           func(name CFStringRef, size CGFloat, matrix *CGAffineTransform) CTFontRef
	fnCTFontCreateWithFontDescriptor func(descriptor CTFontDescriptorRef, size CGFloat, matrix *CGAffineTransform) CTFontRef
	fnCTFontGetAscent                func(font CTFontRef) CGFloat
	fnCTFontGetDescent               func(font CTFontRef) CGFloat
	fnCTFontGetLeading               func(font CTFontRef) CGFloat
	fnCTFontGetUnderlinePosition     func(font CTFontRef) CGFloat
	fnCTFontGetUnderlineThickness    func(font CTFontRef) CGFloat
	fnCTFontGetSize                  func(font CTFontRef) CGFloat
	fnCTFontCopyFamilyName           func(font CTFontRef) CFStringRef

	fnCTFontDescriptorCreateWithAttributes  func(attributes CFDictionaryRef) CTFontDescriptorRef
	fnCTFontDescriptorCreateWithNameAndSize func(name CFStringRef, size CGFloat) CTFontDescriptorRef

	fnCTFramesetterCreateWithAttributedString      func(attrString CFAttributedStringRef) CTFramesetterRef
	fnCTFramesetterCreateFrame                     func(framesetter CTFramesetterRef, stringRange CFRange, path CGPathRef, frameAttributes CFDictionaryRef) CTFrameRef
	fnCTFramesetterSuggestFrameSizeWithConstraints func(framesetter CTFramesetterRef, stringRange CFRange, frameAttributes CFDictionaryRef, constraints CGSize, fitRange *CFRange) CGSize

	fnCTFrameGetLines              func(frame CTFrameRef) CFArrayRef
	fnCTFrameGetLineOrigins        func(frame CTFrameRef, r CFRange, origins unsafe.Pointer)
	fnCTFrameGetVisibleStringRange func(frame CTFrameRef) CFRange
	fnCTFrameDraw                  func(line CTFrameRef, ctx CGContextRef)

	fnCTLineGetStringRange            func(line CTLineRef) CFRange
	fnCTLineGetTypographicBounds      func(line CTLineRef, ascent *CGFloat, descent *CGFloat, leading *CGFloat) CGFloat
	fnCTLineGetGlyphRuns              func(line CTLineRef) CFArrayRef
	fnCTLineDraw                      func(line CTLineRef, ctx CGContextRef)
	fnCTLineGetOffsetForStringIndex   func(line CTLineRef, charIndex CFIndex, secondaryOffset *CGFloat) CGFloat
	fnCTLineGetStringIndexForPosition func(line CTLineRef, position CGPoint) CFIndex
	fnCTLineCreateTruncatedLine       func(line CTLineRef, width CGFloat, truncationType CTLineTruncationType, truncationToken CTLineRef) CTLineRef
	fnCTLineGetBoundsWithOptions      func(line CTLineRef, options CTLineBoundsOptions) CGRect

	fnCTRunGetGlyphCount        func(run CTRunRef) CFIndex
	fnCTRunGetStringRange       func(run CTRunRef) CFRange
	fnCTRunGetTypographicBounds func(run CTRunRef, r CFRange, ascent *CGFloat, descent *CGFloat, leading *CGFloat) CGFloat
	fnCTRunGetPositions         func(run CTRunRef, r CFRange, positions unsafe.Pointer)
	fnCTRunGetStringIndices     func(run CTRunRef, r CFRange, indices unsafe.Pointer)
	fnCTRunGetStatus            func(run CTRunRef) CTRunStatus
	fnCTRunGetAttributes        func(run CTRunRef) CFDictionaryRef

	fnCTTypesetterCreateWithAttributedString func(attrString CFAttributedStringRef) CTTypesetterRef
	fnCTTypesetterSuggestLineBreak           func(typesetter CTTypesetterRef, startIndex CFIndex, width CGFloat) CFIndex
	fnCTTypesetterSuggestClusterBreak        func(typesetter CTTypesetterRef, startIndex CFIndex, width CGFloat) CFIndex
	fnCTTypesetterCreateLine                 func(typesetter CTTypesetterRef, stringRange CFRange) CTLineRef

	fnCTFontManagerRegisterFontsForURL func(fontURL CFURLRef, scope CTFontManagerScope, err *CFTypeRef) bool

	fnCTParagraphStyleCreate func(settings unsafe.Pointer, settingCount uint) CTParagraphStyleRef
)
