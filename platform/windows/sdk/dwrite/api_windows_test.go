package dwrite

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
	"unicode/utf16"
	"unsafe"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/windows/sdk/com"
)

// 不需要桌面。两个 BOOL* 都是输出，原生 BOOL 为 32 位；检查真实 DirectWrite
// 对内部/外部及字符前后半区的返回，防止传入 bool 值或单字节 *bool。
func TestTextLayoutHitTestPoint(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	factory, err := CreateFactory[Factory](DWRITE_FACTORY_TYPE_SHARED, IID_IDWriteFactory)
	if err != nil {
		t.Fatal(err)
	}
	defer factory.Release()
	format, hr := factory.CreateTextFormat("Arial", nil, DWRITE_FONT_WEIGHT_NORMAL, DWRITE_FONT_STYLE_NORMAL, DWRITE_FONT_STRETCH_NORMAL, 20, "en-us")
	if hr.Failed() {
		t.Fatal(hr)
	}
	defer format.Release()
	layout, hr := factory.CreateTextLayout("M", format, 200, 100)
	if hr.Failed() {
		t.Fatal(hr)
	}
	defer layout.Release()
	_, _, m, hr := layout.HitTestTextPosition(0, false)
	if hr.Failed() || m.Width <= 0 || m.Height <= 0 {
		t.Fatalf("text metrics: %+v %v", m, hr)
	}
	for _, tc := range []struct {
		x                float32
		trailing, inside bool
	}{
		{m.Left - 5, false, false},
		{m.Left + m.Width*.25, false, true},
		{m.Left + m.Width*.75, true, true},
	} {
		trailing, inside, hit, hr := layout.HitTestPoint(tc.x, m.Top+m.Height/2)
		if hr != com.HRESULT(0) || trailing != tc.trailing || inside != tc.inside || hit.TextPosition != 0 {
			t.Errorf("x=%v: trailing=%v inside=%v hit=%+v hr=%v", tc.x, trailing, inside, hit, hr)
		}
	}
}

func genericHitPosition(layout *TextLayout, pos int, trailing bool) (x, y float32, hit HitTestMetrics, hr com.HRESULT) {
	hr = cgo.CallRet[com.HRESULT](layout.class().HitTestTextPosition, layout, pos, trailing, &x, &y, &hit)
	return
}

func hitPositionLayout(t testing.TB, text string, width float32) (*TextLayout, func()) {
	factory, err := CreateFactory[Factory](DWRITE_FACTORY_TYPE_SHARED, IID_IDWriteFactory)
	if err != nil {
		t.Fatal(err)
	}
	format, hr := factory.CreateTextFormat("Arial", nil, DWRITE_FONT_WEIGHT_NORMAL, DWRITE_FONT_STYLE_NORMAL, DWRITE_FONT_STRETCH_NORMAL, 20, "en-us")
	if hr.Failed() {
		factory.Release()
		t.Fatal(hr)
	}
	layout, hr := factory.CreateTextLayout(text, format, width, 1<<24)
	if hr.Failed() {
		format.Release()
		factory.Release()
		t.Fatal(hr)
	}
	return layout, func() { layout.Release(); format.Release(); factory.Release() }
}

func TestHitPositionPreservesNativeOutputs(t *testing.T) {
	for _, text := range []string{"", "Aé中", "a\n\nb\n", "a\tbc", "office e\u0301 👩‍💻 عربى אבג xyz", "abc \u202e123 XYZ\u202c end", strings.Repeat("abc \u2067עברית العربية\u2069 123 ", 40)} {
		for _, width := range []float32{31, 317, 1 << 20} {
			t.Run(fmt.Sprintf("%q/%g", text[:min(20, len(text))], width), func(t *testing.T) {
				runtime.LockOSThread()
				defer runtime.UnlockOSThread()
				layout, release := hitPositionLayout(t, text, width)
				defer release()
				for pos := 0; pos <= len(utf16.Encode([]rune(text))); pos++ {
					for _, trailing := range []bool{false, true} {
						x, y, hit, hr := layout.HitTestTextPosition(pos, trailing)
						wx, wy, want, whr := genericHitPosition(layout, pos, trailing)
						if hr != whr || x != wx || y != wy || hit != want {
							t.Fatalf("position %d trailing=%v: %g,%g %+v %v; want %g,%g %+v %v", pos, trailing, x, y, hit, hr, wx, wy, want, whr)
						}
					}
				}
			})
		}
	}
}

func BenchmarkHitPosition(b *testing.B) {
	for _, direct := range []bool{false, true} {
		b.Run(fmt.Sprint(direct), func(b *testing.B) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			text := strings.Repeat("alpha beta gamma ", 8192)
			layout, release := hitPositionLayout(b, text, 1000)
			defer release()
			layout.GetMetrics()
			b.ReportAllocs()
			b.ResetTimer()
			for i := range b.N {
				if direct {
					layout.HitTestTextPosition(i%len(text), false)
				} else {
					genericHitPosition(layout, i%len(text), false)
				}
			}
		})
	}
}

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
