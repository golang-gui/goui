package quartzcore

import (
	"github.com/ebitengine/purego/objc"
	"github.com/goexlib/cgo"
	cg "github.com/golang-gui/goui/platform/darwin/frameworks/core_graphics"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
	"github.com/golang-gui/goui/platform/darwin/frameworks/utils"
)

var message uintptr
var CAEAGLLayerClassId CAEAGLLayerClass
var CATransactionClassId CATransactionClass
var CALayerSel struct{ SetFrame, SetContents, SetContentsScale, SetOpaque, SetHidden, AddSublayer, RemoveFromSuperlayer objc.SEL }
var CATransactionSel struct{ Begin, Commit, SetDisableActions objc.SEL }

func InitQuartzCore() (err error) {
	_, err = utils.LoadSystemFramework("QuartzCore")
	if err != nil {
		return
	}
	message, err = cgo.Dlsym(cgo.RTLD_DEFAULT, "objc_msgSend")
	if err != nil {
		return
	}
	CAEAGLLayerClassId.Class = objc.GetClass("CAEAGLLayer")
	CATransactionClassId.Class = objc.GetClass("CATransaction")
	CALayerSel.SetFrame = objc.RegisterName("setFrame:")
	CALayerSel.SetContents = objc.RegisterName("setContents:")
	CALayerSel.SetContentsScale = objc.RegisterName("setContentsScale:")
	CALayerSel.SetOpaque = objc.RegisterName("setOpaque:")
	CALayerSel.SetHidden = objc.RegisterName("setHidden:")
	CALayerSel.AddSublayer = objc.RegisterName("addSublayer:")
	CALayerSel.RemoveFromSuperlayer = objc.RegisterName("removeFromSuperlayer")
	CATransactionSel.Begin = objc.RegisterName("begin")
	CATransactionSel.Commit = objc.RegisterName("commit")
	CATransactionSel.SetDisableActions = objc.RegisterName("setDisableActions:")
	return
}

type CALayer struct{ NSObject }
type CAEAGLLayerClass struct{ NSObjectClass }
type CATransactionClass struct{ NSObjectClass }

func (c CAEAGLLayerClass) Alloc() CALayer  { return Cast[CALayer](c.Send(NSObjectSel.Alloc)) }
func (l CALayer) SetFrame(frame cg.CGRect) { cgo.Call(message, l.ID, CALayerSel.SetFrame, frame) }
func (l CALayer) SetContents(image ID)     { l.Send(CALayerSel.SetContents, image) }
func (l CALayer) SetContentsScale(scale float64) {
	cgo.Call(message, l.ID, CALayerSel.SetContentsScale, scale)
}
func (l CALayer) SetOpaque(value bool)      { l.Send(CALayerSel.SetOpaque, value) }
func (l CALayer) SetHidden(value bool)      { l.Send(CALayerSel.SetHidden, value) }
func (l CALayer) AddSublayer(layer CALayer) { l.Send(CALayerSel.AddSublayer, layer.ID) }
func (l CALayer) RemoveFromSuperlayer()     { l.Send(CALayerSel.RemoveFromSuperlayer) }
func (c CATransactionClass) Begin()         { c.Send(CATransactionSel.Begin) }
func (c CATransactionClass) Commit()        { c.Send(CATransactionSel.Commit) }
func (c CATransactionClass) SetDisableActions(value bool) {
	c.Send(CATransactionSel.SetDisableActions, value)
}
