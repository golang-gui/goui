package dwrite

import (
	"runtime"
	"testing"

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
