package cairo

import (
	"runtime"
	"testing"
)

// 调用 Cairo 的纯计算接口，不创建 surface 或桌面窗口。
// 原生返回 C int：无效格式或过大宽度必须为 -1，不能零扩展成 4294967295。
func TestFormatStrideForWidth(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	for _, tc := range []struct {
		format Format
		width  int
		want   int
	}{
		{FormatARGB32, 10, 40},
		{FormatARGB32, 0, 0},
		{FormatA8, 1, 4},
		{FormatInvalid, 10, -1},
		{FormatARGB32, 2147483647, -1},
	} {
		if got := FormatStrideForWidth(tc.format, tc.width); got != tc.want {
			t.Fatalf("format=%d width=%d: got %d, want %d", tc.format, tc.width, got, tc.want)
		}
	}
}
