package com

import "unsafe"

type COINIT uint32

const (
	COINIT_MULTITHREADED     COINIT = 0
	COINIT_APARTMENTTHREADED COINIT = 0x2
	COINIT_DISABLE_OLE1DDE   COINIT = 0x4
)

type (
	IID   = GUID
	CLSID = GUID
	ULONG = uint32
)

type GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

func DefineGuid(l uint32, w1, w2 uint16, b1, b2, b3, b4, b5, b6, b7, b8 byte) GUID {
	return GUID{
		Data1: l,
		Data2: w1,
		Data3: w2,
		Data4: [8]byte{b1, b2, b3, b4, b5, b6, b7, b8},
	}
}

type CLSCTX uint32

const (
	CLSCTX_INPROC_SERVER  CLSCTX = 0x1
	CLSCTX_INPROC_HANDLER CLSCTX = 0x2
	CLSCTX_LOCAL_SERVER   CLSCTX = 0x4
	CLSCTX_REMOTE_SERVER  CLSCTX = 0x10
)

var (
	IID_IUnknown    = DefineGuid(0x00000000, 0x0000, 0x0000, 0xc0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46)
	IID_IDataObject = DefineGuid(0x0000010e, 0x0000, 0x0000, 0xc0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46)
	IID_IDropTarget = DefineGuid(0x00000122, 0x0000, 0x0000, 0xc0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46)
	IID_IDropSource = DefineGuid(0x00000121, 0x0000, 0x0000, 0xc0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46)
)

const (
	S_OK                         HRESULT = 0
	E_NOINTERFACE                HRESULT = -2147467262
	E_NOTIMPL                    HRESULT = -2147467263
	E_INVALIDARG                 HRESULT = -2147024809
	DV_E_FORMATETC               HRESULT = -2147221404
	DV_E_TYMED                   HRESULT = -2147221399
	DRAGDROP_S_DROP              HRESULT = 0x00040100
	DRAGDROP_S_CANCEL            HRESULT = 0x00040101
	DRAGDROP_S_USEDEFAULTCURSORS HRESULT = 0x00040102
)

const (
	TymedHGlobal    uint32 = 1
	TymedIStream    uint32 = 4
	DVAspectContent uint32 = 1
	DataDirGet      uint32 = 1
	DropEffectCopy  uint32 = 1
	DropEffectMove  uint32 = 2
	DropEffectLink  uint32 = 4
)

// FormatEtc and StgMedium match the Win64 OLE ABI. The union in STGMEDIUM is
// represented by Handle, valid as HGLOBAL when Tymed==TymedHGlobal.
type FormatEtc struct {
	Format       uint16
	TargetDevice unsafe.Pointer
	Aspect       uint32
	Index        int32
	Tymed        uint32
}

type StgMedium struct {
	Tymed          uint32
	Handle         uintptr
	ReleaseUnknown unsafe.Pointer
}
