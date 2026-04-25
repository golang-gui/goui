package com

import (
	"github.com/goexlib/cgo"
	"syscall"
)

var (
	ole32            = cgo.NewLazyLibrary("Ole32.dll")
	coInitializeEx   = ole32.NewSymbol("CoInitializeEx")
	coUninitialize   = ole32.NewSymbol("CoUninitialize")
	coCreateInstance = ole32.NewSymbol("CoCreateInstance")
)

func Initialize(coInit COINIT) HRESULT {
	ret, _, _ := coInitializeEx.CallRaw(0, uintptr(coInit))
	return HRESULT(ret)
}

func Uninitialize() {
	coUninitialize.CallRaw()
}

func CreateInstance[T isUnknown](clsid CLSID, outer **T, clsCtx CLSCTX, iid IID) (inst *T, hr HRESULT) {
	ret, _, _ := coCreateInstance.CallRaw(uintptr(cgo.Pointer(&clsid)), uintptr(cgo.Pointer(outer)), uintptr(clsCtx), uintptr(cgo.Pointer(&iid)), uintptr(cgo.Pointer(&inst)))
	hr = HRESULT(ret)
	return
}

type isUnknown interface {
	IsUnknown()
}

type UnknownClass struct {
	QueryInterface cgo.Symbol //HRESULT(IUnknown *This,REFIID riid,void **ppvObject);
	AddRef         cgo.Symbol //ULONG(IUnknown *This);
	Release        cgo.Symbol //ULONG(IUnknown *This);
}

type Unknown struct {
	Class cgo.Pointer
}

func (Unknown) IsUnknown() {}

func (this *Unknown) QueryInterface(iid IID, ppvObject **Unknown) HRESULT {
	ret, _, _ := this.class().QueryInterface.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(&iid)), uintptr(cgo.Pointer(ppvObject)))
	return HRESULT(ret)
}

func (this *Unknown) AddRef() ULONG {
	ret, _, _ := this.class().AddRef.CallRaw(uintptr(cgo.Pointer(this)))
	return ULONG(ret)
}

func (this *Unknown) Release() ULONG {
	ret, _, _ := this.class().Release.CallRaw(uintptr(cgo.Pointer(this)))
	return ULONG(ret)
}

func (this *Unknown) class() *UnknownClass {
	return (*UnknownClass)(this.Class)
}

type HRESULT int32

func (hr HRESULT) Succeeded() bool {
	return hr >= 0
}

func (hr HRESULT) Failed() bool {
	return hr < 0
}

func (hr HRESULT) Code() int {
	return int(hr & 0xFFFF)
}

func (hr HRESULT) Facility() int {
	return int((hr >> 16) & 0x1FFF)
}

func (hr HRESULT) Severity() bool {
	return (hr>>31)&0x1 != 0
}

func (hr HRESULT) Error() string {
	return syscall.Errno(hr).Error()
}
