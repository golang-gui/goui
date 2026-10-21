package dwrite

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/windows/sdk/com"
)

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
