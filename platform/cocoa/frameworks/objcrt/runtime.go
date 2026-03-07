package objcrt

import "github.com/ebitengine/purego/objc"

func initNSObject() {
	_NSObject.Class = objc.GetClass("NSObject")
	_NSObject.retain = objc.RegisterName("retain")
	_NSObject.release = objc.RegisterName("release")
	_NSObject.autorelease = objc.RegisterName("autorelease")
	_NSObject.retainCount = objc.RegisterName("retainCount")
}

var _NSObject struct {
	objc.Class
	retain      objc.SEL
	release     objc.SEL
	autorelease objc.SEL
	retainCount objc.SEL
}

type NSObject objc.ID

func (o NSObject) Retain() {
	objc.ID(o).Send(_NSObject.retain)
}

func (o NSObject) Release() {
	objc.ID(o).Send(_NSObject.release)
}

func (o NSObject) AutoRelease() {
	objc.ID(o).Send(_NSObject.autorelease)
}

func (o NSObject) RetainCount() uint {
	ret := objc.ID(o).Send(_NSObject.retainCount)
	return uint(ret)
}
