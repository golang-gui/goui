package win32

import (
	"testing"

	"github.com/golang-gui/goui/platform/windows/sdk/winapi"
)

func TestFullscreenStylePreservesIndependentFlags(t *testing.T) {
	for _, style := range []winapi.DWORD{
		winapi.WS_OVERLAPPEDWINDOW,
		winapi.WS_POPUP | winapi.WS_THICKFRAME | winapi.WS_MAXIMIZEBOX,
		winapi.WS_POPUP,
	} {
		flags := winapi.DWORD(winapi.WS_VISIBLE | winapi.WS_DISABLED | winapi.WS_CLIPCHILDREN | winapi.WS_CLIPSIBLINGS)
		got := fullscreenStyle(style | flags | winapi.WS_MINIMIZE | winapi.WS_MAXIMIZE)
		want := flags | winapi.WS_POPUP
		if got != want {
			t.Fatalf("普通样式 %#x 的全屏样式 got=%#x want=%#x", style, got, want)
		}
	}
}

func TestFullscreenLimitsUseCompleteDisplay(t *testing.T) {
	for _, screen := range []winapi.RECT{
		{Right: 1920, Bottom: 1080},
		{Left: -1920, Top: -400, Bottom: 680},
		{Top: -2160, Right: 3840},
	} {
		m := winapi.MINMAXINFO{MinTrackSize: winapi.POINT{X: 9000, Y: 8000}}
		setFullscreenBounds(&m, screen)
		want := winapi.POINT{X: screen.Right - screen.Left, Y: screen.Bottom - screen.Top}
		if m.MaxSize != want || m.MaxTrackSize != want || m.MaxPosition != (winapi.POINT{}) || m.MinTrackSize != (winapi.POINT{}) {
			t.Fatalf("全屏范围 %+v 应完整覆盖显示器 %+v", m, screen)
		}
	}
}
