package appkit

import (
	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_graphics"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
	"github.com/golang-gui/goui/platform/darwin/frameworks/utils"

	"github.com/ebitengine/purego/objc"
	"github.com/goexlib/cgo"
)

var framework utils.Framework

func InitAppKit() (err error) {
	framework, err = utils.LoadSystemFramework("AppKit")
	if err != nil {
		return
	}

	initNSEvent()
	initNSApplication()
	initNSWindow()
	initNSWindowDelegate()
	initNSResponder()
	initNSView()
	initNSTrackingArea()
	initNSGraphicsContext()
	return
}

// NSApplication

func initNSApplication() {
	NSApplicationClassId.Class = objc.GetClass("NSApplication")
	NSApplicationSel.SharedApplication = objc.RegisterName("sharedApplication")
	NSApplicationSel.SendEvent = objc.RegisterName("sendEvent:")
	NSApplicationSel.PostEvent = objc.RegisterName("postEvent:atStart:")
	NSApplicationSel.NextEvent = objc.RegisterName("nextEventMatchingMask:untilDate:inMode:dequeue:")
	NSApplicationSel.Run = objc.RegisterName("run")
	NSApplicationSel.Stop = objc.RegisterName("stop:")
}

var (
	NSApplicationClassId NSApplicationClass
	NSApplicationSel     struct {
		SharedApplication objc.SEL
		SendEvent         objc.SEL
		PostEvent         objc.SEL
		NextEvent         objc.SEL
		Run               objc.SEL
		Stop              objc.SEL
	}
)

type (
	NSApplication      struct{ NSObject }
	NSApplicationClass struct{ NSObjectClass }
)

var NSApp NSApplication

func (c NSApplicationClass) SharedApplication() (res NSApplication) {
	NSApp.ID = c.Send(NSApplicationSel.SharedApplication)
	return NSApp
}

func (a NSApplication) SendEvent(event NSEvent) {
	a.Send(NSApplicationSel.SendEvent, event)
}

func (a NSApplication) PostEvent(event NSEvent, atStart bool) {
	a.Send(NSApplicationSel.PostEvent, event, atStart)
}

func (a NSApplication) NextEvent(mask NSEventMask, untilDate NSDate, inMode NSRunLoopMode, dequeue bool) (res NSEvent) {
	res.ID = a.Send(NSApplicationSel.NextEvent, mask, untilDate, inMode, dequeue)
	return
}

func (a NSApplication) Run() {
	a.Send(NSApplicationSel.Run)
}

func (a NSApplication) Stop() {
	a.Send(NSApplicationSel.Stop, 0)
}

// NSGraphicsContext

func initNSGraphicsContext() {
	NSGraphicsContextClassId.Class = objc.GetClass("NSGraphicsContext")
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
	NSGraphicsContext      struct{ NSObject }
	NSGraphicsContextClass struct{ NSObjectClass }
)

func (c NSGraphicsContextClass) CurrentContext() (res NSGraphicsContext) {
	res.ID = c.Send(NSGraphicsContextDef.CurrentContext)
	return
}

func (c NSGraphicsContext) CGContext() CGContextRef {
	return CGContextRef(c.Send(NSGraphicsContextDef.CGContext))
}

// NSEvent

func initNSEvent() {
	NSEventClassId.Class = objc.GetClass("NSEvent")
	NSEventSel.OtherEventWithType = objc.RegisterName("otherEventWithType:location:modifierFlags:timestamp:windowNumber:context:subtype:data1:data2:")
	NSEventSel.LocationInWindow = objc.RegisterName("locationInWindow")
	NSEventSel.ModifierFlags = objc.RegisterName("modifierFlags")
	NSEventSel.ButtonNumber = objc.RegisterName("buttonNumber")
}

var (
	NSEventClassId NSEventClass
	NSEventSel     struct {
		OtherEventWithType objc.SEL
		LocationInWindow   objc.SEL
		ModifierFlags      objc.SEL
		ButtonNumber       objc.SEL
	}
)

type (
	NSEvent      struct{ NSObject }
	NSEventClass struct{ NSObjectClass }
)

type NSEventType NSUInteger

// TODO: event types
const NSEventTypeApplicationDefined NSEventType = 15

type NSEventModifierFlags int

func (c NSEventClass) OtherEventWithType(eventType NSEventType, location NSPoint, modifierFlags NSEventModifierFlags, timestamp NSTimeInterval, windowNumber NSInteger, context NSGraphicsContext, subtype cgo.Short, data1, data2 NSInteger) (res NSEvent) {
	res.ID = c.Send(NSEventSel.OtherEventWithType, eventType, location, modifierFlags, timestamp, windowNumber, context, subtype, data1, data2)
	return
}

func (e NSEvent) LocationInWindow() NSPoint {
	return objc.Send[NSPoint](e.ID, NSEventSel.LocationInWindow)
}

func (e NSEvent) ModifierFlags() NSEventModifierFlags {
	return objc.Send[NSEventModifierFlags](e.ID, NSEventSel.ModifierFlags)
}

func (e NSEvent) ButtonNumber() NSInteger {
	return objc.Send[NSInteger](e.ID, NSEventSel.ButtonNumber)
}

const (
	NSEventModifierFlagCapsLock   NSEventModifierFlags = 1 << 16
	NSEventModifierFlagShift      NSEventModifierFlags = 1 << 17
	NSEventModifierFlagControl    NSEventModifierFlags = 1 << 18
	NSEventModifierFlagOption     NSEventModifierFlags = 1 << 19
	NSEventModifierFlagCommand    NSEventModifierFlags = 1 << 20
	NSEventModifierFlagNumericPad NSEventModifierFlags = 1 << 21
)

type NSEventMask uint

// TODO: other event mask
const NSEventMaskAny NSEventMask = (9223372036854775807*2 + 1)

// NSResponder

func initNSResponder() {
	NSResponderClassId.Class = objc.GetClass("NSResponder")
}

type (
	NSResponder      struct{ NSObject }
	NSResponderClass struct{ NSObjectClass }
)

var NSResponderClassId NSObjectClass

// NSView

func initNSView() {
	NSViewClassId.Class = objc.GetClass("NSView")
	NSViewSel.Window = objc.RegisterName("window")
	NSViewSel.Frame = objc.RegisterName("frame")
	NSViewSel.Bounds = objc.RegisterName("bounds")
	NSViewSel.ConvertRectToBacking = objc.RegisterName("convertRectToBacking:")
	NSViewSel.ConvertPointFromView = objc.RegisterName("convertPoint:fromView:")
	NSViewSel.AddTrackingArea = objc.RegisterName("addTrackingArea:")
	NSViewSel.RemoveTrackingArea = objc.RegisterName("removeTrackingArea:")
	NSViewSel.CanBecomeKeyView = objc.RegisterName("canBecomeKeyView")
	NSViewSel.AcceptsFirstResponder = objc.RegisterName("acceptsFirstResponder")
	NSViewSel.WantsUpdateLayer = objc.RegisterName("wantsUpdateLayer")
	NSViewSel.UpdateLayer = objc.RegisterName("updateLayer")
	NSViewSel.DrawRect = objc.RegisterName("drawRect:")
	NSViewSel.ViewDidChangeBackingProperties = objc.RegisterName("viewDidChangeBackingProperties")
	NSViewSel.UpdateTrackingAreas = objc.RegisterName("updateTrackingAreas")
	NSViewSel.MouseEntered = objc.RegisterName("mouseEntered:")
	NSViewSel.MouseExited = objc.RegisterName("mouseExited:")
	NSViewSel.MouseMoved = objc.RegisterName("mouseMoved:")
	NSViewSel.MouseDragged = objc.RegisterName("mouseDragged:")
	NSViewSel.MouseDown = objc.RegisterName("mouseDown:")
	NSViewSel.MouseUp = objc.RegisterName("mouseUp:")
	NSViewSel.RightMouseDown = objc.RegisterName("rightMouseDown:")
	NSViewSel.RightMouseUp = objc.RegisterName("rightMouseUp:")
	NSViewSel.RightMouseDragged = objc.RegisterName("rightMouseDragged:")
	NSViewSel.OtherMouseDown = objc.RegisterName("otherMouseDown:")
	NSViewSel.OtherMouseUp = objc.RegisterName("otherMouseUp:")
	NSViewSel.OtherMouseDragged = objc.RegisterName("otherMouseDragged:")
}

var (
	NSViewClassId NSViewClass
	NSViewSel     struct {
		Window                         objc.SEL
		Frame                          objc.SEL
		Bounds                         objc.SEL
		ConvertRectToBacking           objc.SEL
		ConvertPointFromView           objc.SEL
		AddTrackingArea                objc.SEL
		RemoveTrackingArea             objc.SEL
		CanBecomeKeyView               objc.SEL
		AcceptsFirstResponder          objc.SEL
		WantsUpdateLayer               objc.SEL
		UpdateLayer                    objc.SEL
		DrawRect                       objc.SEL
		ViewDidChangeBackingProperties objc.SEL
		UpdateTrackingAreas            objc.SEL
		MouseEntered                   objc.SEL
		MouseExited                    objc.SEL
		MouseMoved                     objc.SEL
		MouseDragged                   objc.SEL
		MouseDown                      objc.SEL
		MouseUp                        objc.SEL
		RightMouseDown                 objc.SEL
		RightMouseUp                   objc.SEL
		RightMouseDragged              objc.SEL
		OtherMouseDown                 objc.SEL
		OtherMouseUp                   objc.SEL
		OtherMouseDragged              objc.SEL
	}
)

type (
	NSView         struct{ NSResponder }
	NSViewClass    struct{ NSResponderClass }
	NSViewOverride struct {
		CanBecomeKeyView               func(self NSView) bool
		AcceptsFirstResponder          func(self NSView) bool
		WantsUpdateLayer               func(self NSView) bool
		UpdateLayer                    func(self NSView)
		DrawRect                       func(self NSView, rect NSRect)
		ViewDidChangeBackingProperties func(self NSView)
		UpdateTrackingAreas            func(self NSView)
		MouseEntered                   func(self NSView, event NSEvent)
		MouseExited                    func(self NSView, event NSEvent)
		MouseMoved                     func(self NSView, event NSEvent)
		MouseDragged                   func(self NSView, event NSEvent)
		MouseDown                      func(self NSView, event NSEvent)
		MouseUp                        func(self NSView, event NSEvent)
		RightMouseDown                 func(self NSView, event NSEvent)
		RightMouseUp                   func(self NSView, event NSEvent)
		RightMouseDragged              func(self NSView, event NSEvent)
		OtherMouseDown                 func(self NSView, event NSEvent)
		OtherMouseUp                   func(self NSView, event NSEvent)
		OtherMouseDragged              func(self NSView, event NSEvent)
	}
)

func ImplementNSView(className string, override NSViewOverride) (class NSViewClass, err error) {
	methods := make([]objc.MethodDef, 0, 20)
	if override.CanBecomeKeyView != nil {
		methods = append(methods, objc.MethodDef{
			Cmd: NSViewSel.CanBecomeKeyView,
			Fn: func(self objc.ID, cmd objc.SEL) bool {
				return override.CanBecomeKeyView(Cast[NSView](self))
			},
		})
	}
	if override.AcceptsFirstResponder != nil {
		methods = append(methods, objc.MethodDef{
			Cmd: NSViewSel.AcceptsFirstResponder,
			Fn: func(self objc.ID, cmd objc.SEL) bool {
				return override.AcceptsFirstResponder(Cast[NSView](self))
			},
		})
	}
	if override.WantsUpdateLayer != nil {
		methods = append(methods, objc.MethodDef{
			Cmd: NSViewSel.WantsUpdateLayer,
			Fn: func(self objc.ID, cmd objc.SEL) bool {
				return override.WantsUpdateLayer(Cast[NSView](self))
			},
		})
	}
	if override.UpdateLayer != nil {
		methods = append(methods, objc.MethodDef{
			Cmd: NSViewSel.UpdateLayer,
			Fn: func(self objc.ID, cmd objc.SEL) {
				override.UpdateLayer(Cast[NSView](self))
			},
		})
	}
	if override.DrawRect != nil {
		methods = append(methods, objc.MethodDef{
			Cmd: NSViewSel.DrawRect,
			Fn:  makeNSViewDrawRect(override.DrawRect),
		})
	}
	if override.ViewDidChangeBackingProperties != nil {
		methods = append(methods, objc.MethodDef{
			Cmd: NSViewSel.ViewDidChangeBackingProperties,
			Fn: func(self objc.ID, cmd objc.SEL) {
				override.ViewDidChangeBackingProperties(Cast[NSView](self))
			},
		})
	}
	if override.UpdateTrackingAreas != nil {
		methods = append(methods, objc.MethodDef{
			Cmd: NSViewSel.UpdateTrackingAreas,
			Fn: func(self objc.ID, cmd objc.SEL) {
				override.UpdateTrackingAreas(Cast[NSView](self))
			},
		})
	}
	addEventMethod := func(sel objc.SEL, fn func(NSView, NSEvent)) {
		if fn != nil {
			methods = append(methods, objc.MethodDef{
				Cmd: sel,
				Fn: func(self objc.ID, cmd objc.SEL, event objc.ID) {
					fn(Cast[NSView](self), Cast[NSEvent](event))
				},
			})
		}
	}
	addEventMethod(NSViewSel.MouseEntered, override.MouseEntered)
	addEventMethod(NSViewSel.MouseExited, override.MouseExited)
	addEventMethod(NSViewSel.MouseMoved, override.MouseMoved)
	addEventMethod(NSViewSel.MouseDragged, override.MouseDragged)
	addEventMethod(NSViewSel.MouseDown, override.MouseDown)
	addEventMethod(NSViewSel.MouseUp, override.MouseUp)
	addEventMethod(NSViewSel.RightMouseDown, override.RightMouseDown)
	addEventMethod(NSViewSel.RightMouseUp, override.RightMouseUp)
	addEventMethod(NSViewSel.RightMouseDragged, override.RightMouseDragged)
	addEventMethod(NSViewSel.OtherMouseDown, override.OtherMouseDown)
	addEventMethod(NSViewSel.OtherMouseUp, override.OtherMouseUp)
	addEventMethod(NSViewSel.OtherMouseDragged, override.OtherMouseDragged)
	class.Class, err = objc.RegisterClass(className, NSViewClassId.Class, nil, nil, methods)
	return
}

func (c NSViewClass) Alloc() (res NSView) {
	res.NSObject = c.NSResponderClass.Alloc()
	return
}

func (v NSView) Init() (res NSView) {
	res.ID = v.Send(NSObjectSel.Init)
	return
}

func (v NSView) Window() (res NSWindow) {
	res.ID = v.Send(NSViewSel.Window)
	return
}

func (v NSView) Frame() NSRect {
	return objc.Send[NSRect](v.ID, NSViewSel.Frame)
}

func (v NSView) Bounds() NSRect {
	return objc.Send[NSRect](v.ID, NSViewSel.Bounds)
}

func (v NSView) ConvertRectToBacking(rect NSRect) NSRect {
	return objc.Send[NSRect](v.ID, NSViewSel.ConvertRectToBacking, rect)
}

func (v NSView) ConvertPointFromView(point NSPoint, view NSView) NSPoint {
	return objc.Send[NSPoint](v.ID, NSViewSel.ConvertPointFromView, point, view)
}

func (v NSView) AddTrackingArea(area NSTrackingArea) {
	v.Send(NSViewSel.AddTrackingArea, area)
}

func (v NSView) RemoveTrackingArea(area NSTrackingArea) {
	v.Send(NSViewSel.RemoveTrackingArea, area)
}

// NSTrackingArea

func initNSTrackingArea() {
	NSTrackingAreaClassId.Class = objc.GetClass("NSTrackingArea")
	NSTrackingAreaSel.InitWith = objc.RegisterName("initWithRect:options:owner:userInfo:")
}

var (
	NSTrackingAreaClassId NSTrackingAreaClass
	NSTrackingAreaSel     struct {
		InitWith objc.SEL
	}
)

type (
	NSTrackingArea        struct{ NSObject }
	NSTrackingAreaClass   struct{ NSObjectClass }
	NSTrackingAreaOptions NSUInteger
)

const (
	NSTrackingMouseEnteredAndExited NSTrackingAreaOptions = 1 << 0
	NSTrackingMouseMoved            NSTrackingAreaOptions = 1 << 1
	NSTrackingActiveAlways          NSTrackingAreaOptions = 1 << 7
	NSTrackingInVisibleRect         NSTrackingAreaOptions = 1 << 9
)

func (c NSTrackingAreaClass) Alloc() (res NSTrackingArea) {
	res.NSObject = c.NSObjectClass.Alloc()
	return
}

func (a NSTrackingArea) InitWith(rect NSRect, options NSTrackingAreaOptions, owner NSView, userInfo NSObject) NSTrackingArea {
	return Cast[NSTrackingArea](a.Send(NSTrackingAreaSel.InitWith, rect, options, owner, userInfo))
}

// NSWindow

func initNSWindow() {
	NSWindowClassId.Class = objc.GetClass("NSWindow")
	NSWindowSel.InitWith = objc.RegisterName("initWithContentRect:styleMask:backing:defer:")
	NSWindowSel.Center = objc.RegisterName("center")
	NSWindowSel.Title = objc.RegisterName("title")
	NSWindowSel.SetTitle = objc.RegisterName("setTitle:")
	NSWindowSel.SetDelegate = objc.RegisterName("setDelegate:")
	NSWindowSel.ContentView = objc.RegisterName("contentView")
	NSWindowSel.SetContentView = objc.RegisterName("setContentView:")
	NSWindowSel.SetCollectionBehavior = objc.RegisterName("setCollectionBehavior:")
	NSWindowSel.SetAcceptsMouseMovedEvents = objc.RegisterName("setAcceptsMouseMovedEvents:")
	NSWindowSel.SetRestorable = objc.RegisterName("setRestorable:")
	NSWindowSel.BackingScaleFactor = objc.RegisterName("backingScaleFactor")
	NSWindowSel.MakeFirstResponder = objc.RegisterName("makeFirstResponder:")
	NSWindowSel.OrderFront = objc.RegisterName("orderFront:")
	NSWindowSel.OrderOut = objc.RegisterName("orderOut:")
	NSWindowSel.AddChildWindow = objc.RegisterName("addChildWindow:ordered:")
	NSWindowSel.RemoveChildWindow = objc.RegisterName("removeChildWindow:")
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
		SetRestorable              objc.SEL
		BackingScaleFactor         objc.SEL
		MakeFirstResponder         objc.SEL
		OrderFront                 objc.SEL
		OrderOut                   objc.SEL
		AddChildWindow             objc.SEL
		RemoveChildWindow          objc.SEL
		PerformClose               objc.SEL
		Close                      objc.SEL
		CanBecomeKeyWindow         objc.SEL
		CanBecomeMainWindow        objc.SEL
	}
)

type (
	NSWindow         struct{ NSObject }
	NSWindowClass    struct{ NSObjectClass }
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
				return override.CanBecomeKeyWindow(Cast[NSWindow](self))
			},
		})
	}
	if override.CanBecomeMainWindow != nil {
		methods = append(methods, objc.MethodDef{
			Cmd: NSWindowSel.CanBecomeMainWindow,
			Fn: func(self objc.ID, cmd objc.SEL) bool {
				return override.CanBecomeMainWindow(Cast[NSWindow](self))
			},
		})
	}
	class.Class, err = objc.RegisterClass(className, NSWindowClassId.Class, nil, nil, methods)
	return
}

func (c NSWindowClass) Alloc() (res NSWindow) {
	res.NSObject = c.NSObjectClass.Alloc()
	return
}

func (w NSWindow) InitWith(contentRect NSRect, styleMask NSWindowStyleMask, backing NSBackingStoreType, defer_ bool) (res NSWindow) {
	res.ID = w.Send(NSWindowSel.InitWith, contentRect, styleMask, backing, defer_)
	return
}

func (w NSWindow) Center() {
	w.Send(NSWindowSel.Center)
}

func (w NSWindow) Title() string {
	var title NSString
	title.ID = w.Send(NSWindowSel.Title)
	return title.UTF8String()
}

func (w NSWindow) SetTitle(title string) {
	w.Send(NSWindowSel.SetTitle, ToNSString(title))
}

func (w NSWindow) ContentView() (res NSView) {
	res.ID = w.Send(NSWindowSel.ContentView)
	return
}

func (w NSWindow) SetContentView(view NSView) {
	w.Send(NSWindowSel.SetContentView, view)
}

func (w NSWindow) SetDelegate(delegate NSWindowDelegate) {
	w.Send(NSWindowSel.SetDelegate, delegate)
}

func (w NSWindow) SetCollectionBehavior(behavior NSWindowCollectionBehavior) {
	w.Send(NSWindowSel.SetCollectionBehavior, behavior)
}

func (w NSWindow) SetAcceptsMouseMovedEvents(v bool) {
	w.Send(NSWindowSel.SetAcceptsMouseMovedEvents, v)
}

func (w NSWindow) SetRestorable(v bool) {
	w.Send(NSWindowSel.SetRestorable, v)
}

func (w NSWindow) BackingScaleFactor() CGFloat {
	return objc.Send[CGFloat](w.ID, NSWindowSel.BackingScaleFactor)
}

func (w NSWindow) MakeFirstResponder(responder NSResponder) bool {
	return objc.Send[bool](w.ID, NSWindowSel.MakeFirstResponder, responder)
}

func (w NSWindow) OrderFront(sender objc.ID) {
	w.Send(NSWindowSel.OrderFront, sender)
}

func (w NSWindow) OrderOut(sender objc.ID) {
	w.Send(NSWindowSel.OrderOut, sender)
}

func (w NSWindow) AddChildWindow(childWin NSWindow, ordered NSWindowOrderingMode) {
	w.Send(NSWindowSel.AddChildWindow, childWin, ordered)
}

func (w NSWindow) RemoveChildWindow(childWin NSWindow) {
	w.Send(NSWindowSel.RemoveChildWindow, childWin)
}

func (w NSWindow) PerformClose(sender objc.ID) {
	w.Send(NSWindowSel.PerformClose, sender)
}

func (w NSWindow) Close() {
	w.Send(NSWindowSel.Close)
}

// NSWindowDelegate

func initNSWindowDelegate() {
	NSWindowDelegateClassId.Class = objc.GetClass("NSWindowDelegate")
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
	NSWindowDelegate         struct{ NSObject }
	NSWindowDelegateClass    struct{ NSObjectClass }
	NSWindowDelegateOverride struct {
		WindowShouldClose func(self NSWindowDelegate, sender NSWindow) bool
		WindowDidResize   func(self NSWindowDelegate, notification NSNotification)
	}
)

func ImplementNSWindowDelegate(className string, override NSWindowDelegateOverride) (class NSWindowDelegateClass, err error) {
	methods := make([]objc.MethodDef, 0, 2)
	if override.WindowShouldClose != nil {
		methods = append(methods, objc.MethodDef{
			Cmd: NSWindowDelegateSel.WindowShouldClose,
			Fn: func(self objc.ID, cmd objc.SEL, arg objc.ID) bool {
				if override.WindowShouldClose != nil {
					return override.WindowShouldClose(Cast[NSWindowDelegate](self), Cast[NSWindow](arg))
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
					override.WindowDidResize(Cast[NSWindowDelegate](self), Cast[NSNotification](arg))
				}
			},
		})
	}
	class.Class, err = objc.RegisterClass(className, NSObjectClassId.Class, nil, nil, methods)
	return
}

func (c NSWindowDelegateClass) Alloc() (res NSWindowDelegate) {
	res.NSObject = c.NSObjectClass.Alloc()
	return
}

type NSWindowStyleMask NSUInteger

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

type NSBackingStoreType NSUInteger

const (
	NSBackingStoreRetained    NSBackingStoreType = 0
	NSBackingStoreNonretained NSBackingStoreType = 1
	NSBackingStoreBuffered    NSBackingStoreType = 2
)

type NSWindowCollectionBehavior NSUInteger

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

type NSWindowOrderingMode NSInteger

const (
	NSWindowAbove NSWindowOrderingMode = 1
	NSWindowBelow NSWindowOrderingMode = -1
	NSWindowOut   NSWindowOrderingMode = 0
)
