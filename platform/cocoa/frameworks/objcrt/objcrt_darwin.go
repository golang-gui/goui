package objcrt

import "github.com/ebitengine/purego/objc"

type (
	ID  = objc.ID
	SEL = objc.SEL
)

const Nil = 0

func Init() {
	initNSObject()
}

func initNSObject() {
	NSObjectClassId = NSObjectClass(objc.GetClass("NSObject"))
	NSObjectSel.Alloc = objc.RegisterName("alloc")
	NSObjectSel.Init = objc.RegisterName("init")
	NSObjectSel.New = objc.RegisterName("new")
	NSObjectSel.retain = objc.RegisterName("retain")
	NSObjectSel.release = objc.RegisterName("release")
	NSObjectSel.autorelease = objc.RegisterName("autorelease")
	NSObjectSel.retainCount = objc.RegisterName("retainCount")
	NSObjectSel.respondsToSelector = objc.RegisterName("respondsToSelector:")
}

var (
	NSObjectClassId NSObjectClass
	NSObjectSel     struct {
		Alloc objc.SEL
		Init  objc.SEL
		New   objc.SEL

		retain             objc.SEL
		release            objc.SEL
		autorelease        objc.SEL
		retainCount        objc.SEL
		respondsToSelector objc.SEL
	}
)

type (
	NSObject      objc.ID
	NSObjectClass objc.Class
)

func (o NSObject) Retain() {
	objc.ID(o).Send(NSObjectSel.retain)
}

func (o NSObject) Release() {
	objc.ID(o).Send(NSObjectSel.release)
}

func (o NSObject) AutoRelease() {
	objc.ID(o).Send(NSObjectSel.autorelease)
}

func (o NSObject) RetainCount() uint {
	ret := objc.ID(o).Send(NSObjectSel.retainCount)
	return uint(ret)
}

func (o NSObject) RespondsToSelector(sel string) bool {
	return objc.Send[bool](objc.ID(o), NSObjectSel.respondsToSelector, objc.RegisterName(sel))
}
