package pango

import (
	"runtime"
	"testing"
	"unsafe"
)

func TestNativePointerFieldsABI(t *testing.T) {
	// Pango 1.50.6 头文件的 LP64 布局；GlyphString 只映射其公开前缀。
	for _, check := range []struct {
		name      string
		got, want uintptr
	}{
		{"Attribute.size", unsafe.Sizeof(Attribute{}), 16},
		{"Attribute.Class", unsafe.Offsetof(Attribute{}.Class), 0},
		{"Analysis.size", unsafe.Sizeof(Analysis{}), 48},
		{"Analysis.ExtraAttrs", unsafe.Offsetof(Analysis{}.ExtraAttrs), 40},
		{"GlyphString.glyphs", unsafe.Offsetof(GlyphString{}.glyphs), 8},
		{"GlyphString.logClusters", unsafe.Offsetof(GlyphString{}.logClusters), 16},
	} {
		if check.got != check.want {
			t.Errorf("%s: got %d want %d", check.name, check.got, check.want)
		}
	}
}

func TestNativePointerAttributeResult(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	attr := AttrFamilyNew("sans")
	if attr == nil {
		t.Fatal("原生属性分配失败")
	}
	defer attr.Destroy()
	if attr.Type() != AttrFamily {
		t.Fatalf("属性 Class 指针错误：type=%v", attr.Type())
	}
}
