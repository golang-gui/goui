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
