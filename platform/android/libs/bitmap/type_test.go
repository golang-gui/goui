package bitmap

import (
	"testing"
	"unsafe"
)

func TestBitmapInfoLayout(t *testing.T) {
	var info Info
	if unsafe.Sizeof(info) != 20 || unsafe.Offsetof(info.Format) != 12 || unsafe.Offsetof(info.Flags) != 16 || RGBA8888 != 1 {
		t.Fatal("AndroidBitmapInfo ABI 错误")
	}
}
