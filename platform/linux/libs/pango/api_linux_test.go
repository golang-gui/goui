package pango

import (
	"runtime"
	"testing"
	"unsafe"
)

func TestTypographyABI(t *testing.T) {
	for name, tc := range map[string][2]uintptr{
		"Attribute.size":      {unsafe.Sizeof(Attribute{}), 16},
		"AttrClass.Copy":      {unsafe.Offsetof(AttrClass{}.Copy), 8},
		"AttrInt.Value":       {unsafe.Offsetof(AttrInt{}.Value), 16},
		"AttrColor.Color":     {unsafe.Offsetof(AttrColor{}.Color), 16},
		"Analysis.Font":       {unsafe.Offsetof(Analysis{}.Font), 16},
		"Analysis.ExtraAttrs": {unsafe.Offsetof(Analysis{}.ExtraAttrs), 40},
		"GlyphItem.YOffset":   {unsafe.Offsetof(GlyphItem{}.YOffset), 16},
	} {
		if tc[0] != tc[1] {
			t.Errorf("%s: got %d want %d", name, tc[0], tc[1])
		}
	}
}

func TestShapeAttributeCopiesRectangles(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ink, logical := Rectangle{}, Rectangle{Y: -21, Width: 37, Height: 42}
	attr := AttrShapeNew(ink, logical)
	if attr == nil {
		t.Fatal("shape allocation failed")
	}
	defer attr.Destroy()
	// PangoAttrShape 的基属性之后内嵌两个 PangoRectangle。
	shape := (*struct {
		Attribute
		Ink, Logical Rectangle
	})(unsafe.Pointer(attr))
	if attr.Type() != AttrShape || shape.Ink != ink || shape.Logical != logical {
		t.Fatalf("shape ABI/content: %+v", shape)
	}
}

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
