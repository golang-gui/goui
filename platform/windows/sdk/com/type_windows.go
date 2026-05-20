package com

type COINIT uint32

const (
	COINIT_MULTITHREADED     COINIT = 0
	COINIT_APARTMENTTHREADED COINIT = 0x2
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
