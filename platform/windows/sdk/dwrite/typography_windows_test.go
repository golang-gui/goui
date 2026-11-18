package dwrite

import (
	"testing"
	"unsafe"
)

// Windows x64 SDK 的 vtable 顺序及原生 FLOAT/BOOL 结构体布局。
func TestTypographyABI(t *testing.T) {
	for name, tc := range map[string][2]uintptr{
		"Factory3.CreateFontFaceReference":         {unsafe.Offsetof(Factory3Class{}.CreateFontFaceReference_), 33 * 8},
		"Factory3.GetSystemFontSet":                {unsafe.Offsetof(Factory3Class{}.GetSystemFontSet), 35 * 8},
		"Factory3.CreateFontCollectionFromFontSet": {unsafe.Offsetof(Factory3Class{}.CreateFontCollectionFromFontSet), 37 * 8},
		"FontSetBuilder.AddFontFaceReference":      {unsafe.Offsetof(FontSetBuilderClass{}.AddFontFaceReference), 4 * 8},
		"InlineObject.GetMetrics":                  {unsafe.Offsetof(InlineObjectClass{}.GetMetrics), 4 * 8},
		"InlineObject.GetBreakConditions":          {unsafe.Offsetof(InlineObjectClass{}.GetBreakConditions), 6 * 8},
		"InlineObjectMetrics.size":                 {unsafe.Sizeof(InlineObjectMetrics{}), 16},
		"InlineObjectMetrics.BOOL":                 {unsafe.Offsetof(InlineObjectMetrics{}.SupportsSideways), 12},
		"OverhangMetrics.size":                     {unsafe.Sizeof(OverhangMetrics{}), 16},
		"HitTestMetrics.BidiLevel":                 {unsafe.Offsetof(HitTestMetrics{}.BidiLevel), 24},
		"Font.GetFaceNames":                        {unsafe.Offsetof(FontClass{}.GetFaceNames), 8 * 8},
		"Font.GetSimulations":                      {unsafe.Offsetof(FontClass{}.GetSimulations), 10 * 8},
	} {
		if tc[0] != tc[1] {
			t.Errorf("%s: got %d want %d", name, tc[0], tc[1])
		}
	}
}
