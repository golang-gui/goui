package win32

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/windows/sdk/winapi"
)

func TestWindowStyle(t *testing.T) {
	for _, tc := range []struct {
		build uint32
		style winapi.DWORD
		top   winapi.LONG
	}{
		{19045, winapi.WS_POPUP | winapi.WS_THICKFRAME | winapi.WS_MAXIMIZEBOX, 1},
		{21999, winapi.WS_POPUP | winapi.WS_THICKFRAME | winapi.WS_MAXIMIZEBOX, 1},
		{22000, winapi.WS_OVERLAPPEDWINDOW, 0},
		{26100, winapi.WS_OVERLAPPEDWINDOW, 0},
	} {
		if style := windowStyle(common.WindowOptions{}, tc.build); style != winapi.WS_OVERLAPPEDWINDOW {
			t.Fatalf("build %d native style: %#x", tc.build, style)
		}
		if style := windowStyle(common.WindowOptions{Chrome: common.WindowChromeIntegrated}, tc.build); style != tc.style {
			t.Fatalf("build %d integrated style: %#x, want %#x", tc.build, style, tc.style)
		}
		if top := integratedTopBorder(tc.build); top != tc.top {
			t.Fatalf("build %d top border: %d, want %d", tc.build, top, tc.top)
		}
		if style := windowStyle(common.WindowOptions{Chrome: common.WindowChromeNone}, tc.build); style != winapi.WS_POPUP {
			t.Fatalf("build %d undecorated top-level style: %#x", tc.build, style)
		}
	}
}

func TestCustomWindowSize(t *testing.T) {
	for _, chrome := range []common.WindowChrome{common.WindowChromeNone, common.WindowChromeIntegrated} {
		for _, scale := range []float32{1, 1.25, 2} {
			for _, dpi := range []winapi.UINT{96, 144, 192} {
				for _, frame := range []winapi.RECT{
					{Left: 7, Top: 1, Right: 7, Bottom: 7},
					{Left: 8, Right: 8, Bottom: 8},
					{Left: 15, Top: 1, Right: 15, Bottom: 15},
					{Left: 16, Right: 16, Bottom: 16},
				} {
					// Insets are an independent native observation, not logical
					// padding. In particular the Win10 top stays one pixel at 2x.
					got, err := windowSizeRect(geometry.Size{Width: 320, Height: 200}, scale, dpi, chrome, 0, frame)
					if err != nil {
						t.Fatal(err)
					}
					want := winapi.RECT{Right: winapi.LONG(320 * scale), Bottom: winapi.LONG(200 * scale)}
					if chrome == common.WindowChromeIntegrated {
						want = winapi.RECT{Left: -frame.Left, Top: -frame.Top,
							Right: winapi.LONG(320*scale) + frame.Right, Bottom: winapi.LONG(200*scale) + frame.Bottom}
					}
					if got != want {
						t.Fatalf("chrome %v scale %g dpi %d frame %+v: got %+v, want %+v", chrome, scale, dpi, frame, got, want)
					}
				}
			}
		}
	}
}

func TestSkipTaskbarWindowExStyle(t *testing.T) {
	for _, chrome := range []common.WindowChrome{common.WindowChromeNative, common.WindowChromeNone, common.WindowChromeIntegrated} {
		for _, transparent := range []bool{false, true} {
			options := common.WindowOptions{Chrome: chrome, Transparent: transparent}
			ordinary := windowExStyle(options)
			options.SkipTaskbar = true
			got := windowExStyle(options)
			if got != ordinary|winapi.WS_EX_TOOLWINDOW || got&(winapi.WS_EX_NOACTIVATE|winapi.WS_EX_TOPMOST|winapi.WS_EX_APPWINDOW) != 0 {
				t.Fatalf("incorrect tool-window treatment: %#x, ordinary %#x", got, ordinary)
			}
			if windowStyle(options, 19045) != windowStyle(common.WindowOptions{Chrome: chrome, Transparent: transparent}, 19045) {
				t.Fatal("SkipTaskbar changed Chrome")
			}
		}
	}
}

func TestSkipTaskbarNativeClientSize(t *testing.T) {
	for _, dpi := range []winapi.UINT{96, 192} {
		for _, skip := range []bool{false, true} {
			exStyle := windowExStyle(common.WindowOptions{SkipTaskbar: skip})
			size := geometry.Size{Width: 320, Height: 200}
			scale := float32(dpi) / 96
			got, err := windowSizeRect(size, scale, dpi, common.WindowChromeNative, exStyle, winapi.RECT{})
			if err != nil {
				t.Fatal(err)
			}
			want := winapi.RECT{Right: winapi.LONG(320 * scale), Bottom: winapi.LONG(200 * scale)}
			if err := winapi.AdjustWindowRectExForDpi(&want, winapi.WS_OVERLAPPEDWINDOW, 0, exStyle, dpi); err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("DPI %d skip %v: got %+v want native %+v", dpi, skip, got, want)
			}
		}
	}
}

func TestIntegratedClientRect(t *testing.T) {
	for _, tc := range []struct {
		name    string
		outer   winapi.RECT
		native  winapi.RECT
		wantTop winapi.LONG
		top     winapi.LONG
	}{
		{"Win10 normal 1x", winapi.RECT{Left: 100, Top: 100, Right: 900, Bottom: 700}, winapi.RECT{Left: 108, Top: 108, Right: 892, Bottom: 692}, 101, 1},
		{"Win10 normal 2x negative origin", winapi.RECT{Left: -1700, Top: -500, Right: -100, Bottom: 700}, winapi.RECT{Left: -1684, Top: -484, Right: -116, Bottom: 684}, -499, 1},
		{"Win11 caption removed 1x", winapi.RECT{Left: 100, Top: 100, Right: 900, Bottom: 700}, winapi.RECT{Left: 108, Top: 131, Right: 892, Bottom: 692}, 100, 0},
		{"Win11 caption removed 2x negative origin", winapi.RECT{Left: -1700, Top: -500, Right: -100, Bottom: 700}, winapi.RECT{Left: -1684, Top: -438, Right: -116, Bottom: 684}, -500, 0},
		{"empty height Win10", winapi.RECT{Left: 10, Top: 20, Right: 10, Bottom: 20}, winapi.RECT{Left: 10, Top: 20, Right: 10, Bottom: 20}, 20, 1},
		{"empty height Win11", winapi.RECT{Left: 10, Top: 20, Right: 10, Bottom: 20}, winapi.RECT{Left: 10, Top: 20, Right: 10, Bottom: 20}, 20, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := integratedClientRect(tc.outer, tc.native, tc.top)
			want := tc.native
			want.Top = tc.wantTop
			if got != want {
				t.Fatalf("client = %+v, want %+v", got, want)
			}
		})
	}
}

func TestWindowsBuild(t *testing.T) {
	p := new(Platform)
	build, err := p.windowsBuild()
	if err != nil || build == 0 {
		t.Fatalf("build=%d error=%v", build, err)
	}
	if p.buildNumber != build {
		t.Fatal("build observation was not cached on Platform")
	}
	// A cached build needs no native query and can be used without windows.
	p.buildNumber = 22000
	if got, err := p.windowsBuild(); got != 22000 || err != nil {
		t.Fatalf("cached build=%d error=%v", got, err)
	}
}

func TestMaximizedBounds(t *testing.T) {
	for _, tc := range []struct {
		name    string
		monitor winapi.RECT
		work    winapi.RECT
		pos     winapi.POINT
		size    winapi.POINT
	}{
		{"bottom taskbar", winapi.RECT{Right: 1920, Bottom: 1080}, winapi.RECT{Right: 1920, Bottom: 1040}, winapi.POINT{}, winapi.POINT{X: 1920, Y: 1040}},
		{"top taskbar", winapi.RECT{Right: 1920, Bottom: 1080}, winapi.RECT{Top: 40, Right: 1920, Bottom: 1080}, winapi.POINT{Y: 40}, winapi.POINT{X: 1920, Y: 1040}},
		{"negative monitor left taskbar", winapi.RECT{Left: -1920, Top: -400, Bottom: 680}, winapi.RECT{Left: -1880, Top: -400, Bottom: 680}, winapi.POINT{X: 40}, winapi.POINT{X: 1880, Y: 1080}},
		{"2x monitor above", winapi.RECT{Top: -2160, Right: 3840}, winapi.RECT{Top: -2080, Right: 3840}, winapi.POINT{Y: 80}, winapi.POINT{X: 3840, Y: 2080}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mmi := winapi.MINMAXINFO{MinTrackSize: winapi.POINT{X: 320, Y: 200}, MaxTrackSize: winapi.POINT{X: 8000, Y: 8000}}
			setMaximizedBounds(&mmi, winapi.MONITORINFO{Monitor: tc.monitor, Work: tc.work})
			if mmi.MaxPosition != tc.pos || mmi.MaxSize != tc.size {
				t.Fatalf("maximized bounds: %+v", mmi)
			}
			if mmi.MinTrackSize != (winapi.POINT{X: 320, Y: 200}) || mmi.MaxTrackSize != (winapi.POINT{X: 8000, Y: 8000}) {
				t.Fatal("maximization changed tracking constraints")
			}
		})
	}
}

func TestIntegratedTopHit(t *testing.T) {
	client := winapi.RECT{Right: 800, Bottom: 600}
	for _, tc := range []struct {
		name   string
		point  winapi.POINT
		native winapi.LRESULT
		want   winapi.LRESULT
	}{
		{"non-client top pixel", winapi.POINT{X: 400, Y: -1}, winapi.HTTOP, winapi.HTTOP},
		{"client top band", winapi.POINT{X: 400, Y: 16}, winapi.HTCLIENT, winapi.HTTOP},
		{"client left corner", winapi.POINT{X: 16, Y: 8}, winapi.HTCLIENT, winapi.HTTOPLEFT},
		{"client right corner", winapi.POINT{X: 784, Y: 8}, winapi.HTCLIENT, winapi.HTTOPRIGHT},
		{"native left corner", winapi.POINT{X: -8, Y: 8}, winapi.HTTOPLEFT, winapi.HTTOPLEFT},
		{"native right corner", winapi.POINT{X: 808, Y: 8}, winapi.HTTOPRIGHT, winapi.HTTOPRIGHT},
		{"overlong left corner", winapi.POINT{X: -8, Y: 17}, winapi.HTTOPLEFT, winapi.HTLEFT},
		{"overlong right corner", winapi.POINT{X: 808, Y: 17}, winapi.HTTOPRIGHT, winapi.HTRIGHT},
		{"below top band", winapi.POINT{X: 400, Y: 17}, winapi.HTCLIENT, winapi.HTCLIENT},
		{"side", winapi.POINT{X: -8, Y: 100}, winapi.HTLEFT, winapi.HTLEFT},
		{"bottom", winapi.POINT{X: 400, Y: 608}, winapi.HTBOTTOM, winapi.HTBOTTOM},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if hit := integratedTopHit(tc.point, client, 16, 16, tc.native); hit != tc.want {
				t.Fatalf("hit %v, want %v", hit, tc.want)
			}
		})
	}
}
