package appkit

import (
	"github.com/ebitengine/purego/objc"
	"github.com/golang-gui/goui/core/cgo"
	"github.com/golang-gui/goui/platform/cocoa/frameworks/common"
	"github.com/golang-gui/goui/platform/cocoa/frameworks/core_graphics"
	"github.com/golang-gui/goui/platform/cocoa/frameworks/foundation"
	"github.com/golang-gui/goui/platform/cocoa/frameworks/objcrt"
)

var handle uintptr

func Init(load common.LoadFunc) (err error) {
	handle, err = load("AppKit")
	if err != nil {
		return
	}

	initNSEvent()
	initNSApplication()
	initNSWindow()
	initNSWindowDelegate()
	initNSResponder()
	initNSView()
	initNSGraphicsContext()
	return
}

// NSApplication

func initNSApplication() {
	NSApplicationClassId = NSApplicationClass(objc.GetClass("NSApplication"))
	NSApplicationSel.SharedApplication = objc.RegisterName("sharedApplication")
	NSApplicationSel.SendEvent = objc.RegisterName("sendEvent:")
	NSApplicationSel.PostEvent = objc.RegisterName("postEvent:atStart:")
	NSApplicationSel.NextEvent = objc.RegisterName("nextEventMatchingMask:untilDate:inMode:dequeue:")
}

var (
	NSApplicationClassId NSApplicationClass
	NSApplicationSel     struct {
		SharedApplication objc.SEL
		SendEvent         objc.SEL
		PostEvent         objc.SEL
		NextEvent         objc.SEL
	}
)

type (
	NSApplication      objc.ID
	NSApplicationClass objc.Class
)

var NSApp NSApplication

func (c NSApplicationClass) SharedApplication() NSApplication {
	NSApp = NSApplication(objc.ID(NSApplicationClassId).Send(NSApplicationSel.SharedApplication))
	return NSApp
}

func (a NSApplication) SendEvent(event NSEvent) {
	objc.ID(a).Send(NSApplicationSel.SendEvent, event)
}

func (a NSApplication) PostEvent(event NSEvent, atStart bool) {
	objc.ID(a).Send(NSApplicationSel.PostEvent, event, atStart)
}

func (a NSApplication) NextEvent(mask NSEventMask, untilDate foundation.NSDate, inMode foundation.NSRunLoopMode, dequeue bool) NSEvent {
	event := objc.ID(a).Send(NSApplicationSel.NextEvent, mask, untilDate, inMode, dequeue)
	return NSEvent(event)
}

// NSGraphicsContext

func initNSGraphicsContext() {
	NSGraphicsContextClassId = NSGraphicsContextClass(objc.GetClass("NSGraphicsContext"))
	NSGraphicsContextDef.CurrentContext = objc.RegisterName("currentContext")
	NSGraphicsContextDef.CGContext = objc.RegisterName("CGContext")
}

var (
	NSGraphicsContextClassId NSGraphicsContextClass
	NSGraphicsContextDef     struct {
		CurrentContext objc.SEL
		CGContext      objc.SEL
	}
)

type (
	NSGraphicsContext      objcrt.NSObject
	NSGraphicsContextClass objc.Class
)

func (c NSGraphicsContextClass) CurrentContext() NSGraphicsContext {
	return NSGraphicsContext(objc.ID(c).Send(NSGraphicsContextDef.CurrentContext))
}

func (c NSGraphicsContext) CGContext() core_graphics.CGContextRef {
	return core_graphics.CGContextRef(objc.ID(c).Send(NSGraphicsContextDef.CGContext))
}

// NSEvent

func initNSEvent() {
	NSEventClassId = NSEventClass(objc.GetClass("NSEvent"))
	NSEventSel.OtherEventWithType = objc.RegisterName("otherEventWithType:location:modifierFlags:timestamp:windowNumber:context:subtype:data1:data2:")
}

var (
	NSEventClassId NSEventClass
	NSEventSel     struct {
		OtherEventWithType objc.SEL
	}
)

type (
	NSEvent      objcrt.NSObject
	NSEventClass objc.Class
)

type NSEventType foundation.NSUInteger

// TODO: event types
const NSEventTypeApplicationDefined NSEventType = 15

type NSEventModifierFlags int

func (c NSEventClass) OtherEventWithType(eventType NSEventType, location foundation.NSPoint, modifierFlags NSEventModifierFlags, timestamp foundation.NSTimeInterval, windowNumber foundation.NSInteger, context NSGraphicsContext, subtype cgo.Short, data1, data2 foundation.NSInteger) NSEvent {
	event := objc.ID(c).Send(NSEventSel.OtherEventWithType, eventType, location, modifierFlags, timestamp, windowNumber, context, subtype, data1, data2)
	return NSEvent(event)
}

type NSEventMask uint

// TODO: other event mask
const NSEventMaskAny NSEventMask = (9223372036854775807*2 + 1)

// NSResponder

func initNSResponder() {
	NSResponderClassId = objc.GetClass("NSResponder")
}

type NSResponder objcrt.NSObject

var NSResponderClassId objc.Class

// NSView

func initNSView() {
	NSViewClassId = NSViewClass(objc.GetClass("NSView"))
	NSViewSel.Window = objc.RegisterName("window")
	NSViewSel.Frame = objc.RegisterName("frame")
	NSViewSel.Bounds = objc.RegisterName("bounds")
	NSViewSel.ConvertRectToBacking = objc.RegisterName("convertRectToBacking:")
	NSViewSel.CanBecomeKeyView = objc.RegisterName("canBecomeKeyView")
	NSViewSel.AcceptsFirstResponder = objc.RegisterName("acceptsFirstResponder")
	NSViewSel.WantsUpdateLayer = objc.RegisterName("wantsUpdateLayer")
	NSViewSel.UpdateLayer = objc.RegisterName("updateLayer")
	NSViewSel.DrawRect = objc.RegisterName("drawRect:")
	NSViewSel.ViewDidChangeBackingProperties = objc.RegisterName("viewDidChangeBackingProperties")
}

var (
	NSViewClassId NSViewClass
	NSViewSel     struct {
		Window                         objc.SEL
		Frame                          objc.SEL
		Bounds                         objc.SEL
		ConvertRectToBacking           objc.SEL
		CanBecomeKeyView               objc.SEL
		AcceptsFirstResponder          objc.SEL
		WantsUpdateLayer               objc.SEL
		UpdateLayer                    objc.SEL
		DrawRect                       objc.SEL
		ViewDidChangeBackingProperties objc.SEL
	}
)

type (
	NSView         NSResponder
	NSViewClass    objc.Class
	NSViewOverride struct {
		CanBecomeKeyView               func(self NSView) bool
		AcceptsFirstResponder          func(self NSView) bool
		WantsUpdateLayer               func(self NSView) bool
		UpdateLayer                    func(self NSView)
		DrawRect                       func(self NSView, rect foundation.NSRect)
		ViewDidChangeBackingProperties func(self NSView)
	}
)

func ImplementNSView(className string, override NSViewOverride) (class NSViewClass, err error) {
	methods := make([]objc.MethodDef, 0, 6)
	if override.CanBecomeKeyView != nil {
		methods = append(methods, objc.MethodDef{
			Cmd: NSViewSel.CanBecomeKeyView,
			Fn: func(self objc.ID, cmd objc.SEL) bool {
				return override.CanBecomeKeyView(NSView(self))
			},
		})
	}
	if override.AcceptsFirstResponder != nil {
		methods = append(methods, objc.MethodDef{
			Cmd: NSViewSel.AcceptsFirstResponder,
			Fn: func(self objc.ID, cmd objc.SEL) bool {
				return override.AcceptsFirstResponder(NSView(self))
			},
		})
	}
	if override.WantsUpdateLayer != nil {
		methods = append(methods, objc.MethodDef{
			Cmd: NSViewSel.WantsUpdateLayer,
			Fn: func(self objc.ID, cmd objc.SEL) bool {
				return override.WantsUpdateLayer(NSView(self))
			},
		})
	}
	if override.UpdateLayer != nil {
		methods = append(methods, objc.MethodDef{
			Cmd: NSViewSel.UpdateLayer,
			Fn: func(self objc.ID, cmd objc.SEL) {
				override.UpdateLayer(NSView(self))
			},
		})
	}
	if override.DrawRect != nil {
		methods = append(methods, objc.MethodDef{
			Cmd: NSViewSel.DrawRect,
			Fn:  MakeIMP_NSView_DrawRect(override.DrawRect),
		})
	}
	if override.ViewDidChangeBackingProperties != nil {
		methods = append(methods, objc.MethodDef{
			Cmd: NSViewSel.ViewDidChangeBackingProperties,
			Fn: func(self objc.ID, cmd objc.SEL) {
				override.ViewDidChangeBackingProperties(NSView(self))
			},
		})
	}
	cls, err := objc.RegisterClass(className, objc.Class(NSViewClassId), nil, nil, methods)
	if err != nil {
		return
	}
	return NSViewClass(cls), nil
}

func (c NSViewClass) Alloc() NSView {
	return objc.Send[NSView](objc.ID(c), objcrt.NSObjectSel.Alloc)
}

func (v NSView) Init() NSView {
	objc.ID(v).Send(objcrt.NSObjectSel.Init)
	return v
}

func (v NSView) Window() NSWindow {
	win := objc.ID(v).Send(NSViewSel.Window)
	return NSWindow(win)
}

func (v NSView) Frame() foundation.NSRect {
	return objc.Send[foundation.NSRect](objc.ID(v), NSViewSel.Frame)
}

func (v NSView) Bounds() foundation.NSRect {
	return objc.Send[foundation.NSRect](objc.ID(v), NSViewSel.Bounds)
}

func (v NSView) ConvertRectToBacking(rect foundation.NSRect) foundation.NSRect {
	return objc.Send[foundation.NSRect](objc.ID(v), NSViewSel.ConvertRectToBacking, rect)
}

// NSWindow

func initNSWindow() {
	NSWindowClassId = NSWindowClass(objc.GetClass("NSWindow"))
	NSWindowSel.InitWith = objc.RegisterName("initWithContentRect:styleMask:backing:defer:")
	NSWindowSel.Center = objc.RegisterName("center")
	NSWindowSel.Title = objc.RegisterName("title")
	NSWindowSel.SetTitle = objc.RegisterName("setTitle:")
	NSWindowSel.SetDelegate = objc.RegisterName("setDelegate:")
	NSWindowSel.ContentView = objc.RegisterName("contentView")
	NSWindowSel.SetContentView = objc.RegisterName("setContentView:")
	NSWindowSel.SetCollectionBehavior = objc.RegisterName("setCollectionBehavior:")
	NSWindowSel.SetAcceptsMouseMovedEvents = objc.RegisterName("setAcceptsMouseMovedEvents:")
	NSWindowSel.BackingScaleFactor = objc.RegisterName("backingScaleFactor")
	NSWindowSel.MakeFirstResponder = objc.RegisterName("makeFirstResponder:")
	NSWindowSel.OrderFront = objc.RegisterName("orderFront:")
	NSWindowSel.OrderOut = objc.RegisterName("orderOut:")
	NSWindowSel.PerformClose = objc.RegisterName("performClose:")
	NSWindowSel.Close = objc.RegisterName("close")
	NSWindowSel.CanBecomeKeyWindow = objc.RegisterName("canBecomeKeyWindow")
	NSWindowSel.CanBecomeMainWindow = objc.RegisterName("canBecomeMainWindow")
}

var (
	NSWindowClassId NSWindowClass
	NSWindowSel     struct {
		InitWith                   objc.SEL
		Center                     objc.SEL
		Title                      objc.SEL
		SetTitle                   objc.SEL
		SetDelegate                objc.SEL
		ContentView                objc.SEL
		SetContentView             objc.SEL
		SetCollectionBehavior      objc.SEL
		SetAcceptsMouseMovedEvents objc.SEL
		BackingScaleFactor         objc.SEL
		MakeFirstResponder         objc.SEL
		OrderFront                 objc.SEL
		OrderOut                   objc.SEL
		PerformClose               objc.SEL
		Close                      objc.SEL
		CanBecomeKeyWindow         objc.SEL
		CanBecomeMainWindow        objc.SEL
	}
)

type (
	NSWindow         objc.ID
	NSWindowClass    objc.Class
	NSWindowOverride struct {
		CanBecomeKeyWindow  func(self NSWindow) bool
		CanBecomeMainWindow func(self NSWindow) bool
	}
)

func ImplementNSWindow(className string, override NSWindowOverride) (class NSWindowClass, err error) {
	methods := make([]objc.MethodDef, 0, 2)
	if override.CanBecomeKeyWindow != nil {
		methods = append(methods, objc.MethodDef{
			Cmd: NSWindowSel.CanBecomeKeyWindow,
			Fn: func(self objc.ID, cmd objc.SEL) bool {
				return override.CanBecomeKeyWindow(NSWindow(self))
			},
		})
	}
	if override.CanBecomeMainWindow != nil {
		methods = append(methods, objc.MethodDef{
			Cmd: NSWindowSel.CanBecomeMainWindow,
			Fn: func(self objc.ID, cmd objc.SEL) bool {
				return override.CanBecomeMainWindow(NSWindow(self))
			},
		})
	}
	cls, err := objc.RegisterClass(className, objc.Class(NSWindowClassId), nil, nil, methods)
	if err != nil {
		return
	}
	return NSWindowClass(cls), nil
}

func (c NSWindowClass) Alloc() NSWindow {
	win := objc.ID(c).Send(objcrt.NSObjectSel.Alloc)
	return NSWindow(win)
}

func (w NSWindow) InitWith(contentRect foundation.NSRect, styleMask NSWindowStyleMask, backing NSBackingStoreType, defer_ bool) NSWindow {
	objc.ID(w).Send(NSWindowSel.InitWith, contentRect, styleMask, backing, defer_)
	return w
}

func (w NSWindow) Center() {
	objc.ID(w).Send(NSWindowSel.Center)
}

func (w NSWindow) Title() string {
	title := foundation.NSString(objc.ID(w).Send(NSWindowSel.Title))
	return title.UTF8String()
}

func (w NSWindow) SetTitle(title string) {
	objc.ID(w).Send(NSWindowSel.SetTitle, foundation.ToNSString(title))
}

func (w NSWindow) ContentView() NSView {
	return objc.Send[NSView](objc.ID(w), NSWindowSel.ContentView)
}

func (w NSWindow) SetContentView(view NSView) {
	objc.ID(w).Send(NSWindowSel.SetContentView, view)
}

func (w NSWindow) SetDelegate(delegate NSWindowDelegate) {
	objc.ID(w).Send(NSWindowSel.SetDelegate, delegate)
}

func (w NSWindow) SetCollectionBehavior(behavior NSWindowCollectionBehavior) {
	objc.ID(w).Send(NSWindowSel.SetCollectionBehavior, behavior)
}

func (w NSWindow) SetAcceptsMouseMovedEvents(v bool) {
	objc.ID(w).Send(NSWindowSel.SetAcceptsMouseMovedEvents, v)
}

func (w NSWindow) BackingScaleFactor() core_graphics.CGFloat {
	return objc.Send[core_graphics.CGFloat](objc.ID(w), NSWindowSel.BackingScaleFactor)
}

func (w NSWindow) MakeFirstResponder(responder NSResponder) bool {
	return objc.Send[bool](objc.ID(w), NSWindowSel.MakeFirstResponder, responder)
}

func (w NSWindow) OrderFront(sender objc.ID) {
	objc.ID(w).Send(NSWindowSel.OrderFront, sender)
}

func (w NSWindow) OrderOut(sender objc.ID) {
	objc.ID(w).Send(NSWindowSel.OrderOut, sender)
}

func (w NSWindow) PerformClose(sender objc.ID) {
	objc.ID(w).Send(NSWindowSel.PerformClose, sender)
}

func (w NSWindow) Close() {
	objc.ID(w).Send(NSWindowSel.Close)
}

// NSWindowDelegate

func initNSWindowDelegate() {
	NSWindowDelegateClassId = NSWindowDelegateClass(objc.GetClass("NSWindowDelegate"))
	NSWindowDelegateSel.WindowShouldClose = objc.RegisterName("windowShouldClose:")
	NSWindowDelegateSel.WindowDidResize = objc.RegisterName("windowDidResize:")
}

var (
	NSWindowDelegateClassId NSWindowDelegateClass
	NSWindowDelegateSel     struct {
		WindowShouldClose objc.SEL
		WindowDidResize   objc.SEL
	}
)

type (
	NSWindowDelegate         objcrt.NSObject
	NSWindowDelegateClass    objc.Class
	NSWindowDelegateOverride struct {
		WindowShouldClose func(self NSWindowDelegate, sender NSWindow) bool
		WindowDidResize   func(self NSWindowDelegate, notification foundation.NSNotification)
	}
)

func ImplementNSWindowDelegate(className string, override NSWindowDelegateOverride) (class NSWindowDelegateClass, err error) {
	methods := make([]objc.MethodDef, 0, 2)
	if override.WindowShouldClose != nil {
		methods = append(methods, objc.MethodDef{
			Cmd: NSWindowDelegateSel.WindowShouldClose,
			Fn: func(self objc.ID, cmd objc.SEL, arg objc.ID) bool {
				if override.WindowShouldClose != nil {
					return override.WindowShouldClose(NSWindowDelegate(self), NSWindow(arg))
				}
				return true
			},
		})
	}
	if override.WindowDidResize != nil {
		methods = append(methods, objc.MethodDef{
			Cmd: NSWindowDelegateSel.WindowDidResize,
			Fn: func(self objc.ID, cmd objc.SEL, arg objc.ID) {
				if override.WindowDidResize != nil {
					override.WindowDidResize(NSWindowDelegate(self), foundation.NSNotification(arg))
				}
			},
		})
	}
	cls, err := objc.RegisterClass(className, objc.Class(objcrt.NSObjectClassId), nil, nil, methods)
	if err != nil {
		return
	}
	return NSWindowDelegateClass(cls), nil
}

func (c NSWindowDelegateClass) Alloc() NSWindowDelegate {
	return objc.Send[NSWindowDelegate](objc.ID(c), objcrt.NSObjectSel.Alloc)
}

type NSWindowStyleMask foundation.NSUInteger

const (
	NSWindowStyleMaskBorderless             NSWindowStyleMask = 0
	NSWindowStyleMaskTitled                 NSWindowStyleMask = 1 << 0
	NSWindowStyleMaskClosable               NSWindowStyleMask = 1 << 1
	NSWindowStyleMaskMiniaturizable         NSWindowStyleMask = 1 << 2
	NSWindowStyleMaskResizable              NSWindowStyleMask = 1 << 3
	NSWindowStyleMaskTexturedBackground     NSWindowStyleMask = 1 << 8
	NSWindowStyleMaskUnifiedTitleAndToolbar NSWindowStyleMask = 1 << 12
	NSWindowStyleMaskFullScreen             NSWindowStyleMask = 1 << 14
	NSWindowStyleMaskFullSizeContentView    NSWindowStyleMask = 1 << 15
	NSWindowStyleMaskUtilityWindow          NSWindowStyleMask = 1 << 4
	NSWindowStyleMaskDocModalWindow         NSWindowStyleMask = 1 << 6
	NSWindowStyleMaskNonactivatingPanel     NSWindowStyleMask = 1 << 7
	NSWindowStyleMaskHUDWindow              NSWindowStyleMask = 1 << 13
)

type NSBackingStoreType foundation.NSUInteger

const (
	NSBackingStoreRetained    NSBackingStoreType = 0
	NSBackingStoreNonretained NSBackingStoreType = 1
	NSBackingStoreBuffered    NSBackingStoreType = 2
)

type NSWindowCollectionBehavior foundation.NSUInteger

const (
	NSWindowCollectionBehaviorDefault                   NSWindowCollectionBehavior = 0
	NSWindowCollectionBehaviorCanJoinAllSpaces          NSWindowCollectionBehavior = 1 << 0
	NSWindowCollectionBehaviorMoveToActiveSpace         NSWindowCollectionBehavior = 1 << 1
	NSWindowCollectionBehaviorManaged                   NSWindowCollectionBehavior = 1 << 2
	NSWindowCollectionBehaviorTransient                 NSWindowCollectionBehavior = 1 << 3
	NSWindowCollectionBehaviorStationary                NSWindowCollectionBehavior = 1 << 4
	NSWindowCollectionBehaviorParticipatesInCycle       NSWindowCollectionBehavior = 1 << 5
	NSWindowCollectionBehaviorIgnoresCycle              NSWindowCollectionBehavior = 1 << 6
	NSWindowCollectionBehaviorFullScreenPrimary         NSWindowCollectionBehavior = 1 << 7
	NSWindowCollectionBehaviorFullScreenAuxiliary       NSWindowCollectionBehavior = 1 << 8
	NSWindowCollectionBehaviorFullScreenNone            NSWindowCollectionBehavior = 1 << 9
	NSWindowCollectionBehaviorFullScreenAllowsTiling    NSWindowCollectionBehavior = 1 << 11
	NSWindowCollectionBehaviorFullScreenDisallowsTiling NSWindowCollectionBehavior = 1 << 12
	//NSWindowCollectionBehaviorPrimary  __attribute__((availability(macos,introduced=13.0))) = 1 << 16,
	//NSWindowCollectionBehaviorAuxiliary  __attribute__((availability(macos,introduced=13.0))) = 1 << 17,
	//NSWindowCollectionBehaviorCanJoinAllApplications  __attribute__((availability(macos,introduced=13.0))) = 1 << 18,
)
