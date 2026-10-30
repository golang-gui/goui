package x11

import (
	"errors"
	"math"
	"os"
	"runtime"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"
)

func TestPositionMessage(t *testing.T) {
	m := positionMessage(42, 71, -200, 350)
	e := m.ClientMessageEvent()
	if e.Type != xlib.ClientMessage || e.Window != 42 || e.MessageType != 71 || e.Format != 32 {
		t.Fatalf("invalid message header: %+v", e)
	}
	// EWMH：Static gravity 为 10，x/y 位为 8/9，来源位为 12，不设置大小。
	if e.L != [5]int64{0x130a, -200, 350, 0, 0} {
		t.Fatalf("invalid positioning payload: %v", e.L)
	}
}

func TestPositionHintsSurviveMinimumChanges(t *testing.T) {
	w := &Window{initialPosition: &[2]int32{-80, 120}, minW: 200, minH: 100}
	h := w.normalHints(2)
	if h.Flags != xlib.PMinSize|xlib.PPosition|xlib.PWinGravity || h.MinWidth != 400 || h.MinHeight != 200 {
		t.Fatalf("missing hints: %+v", h)
	}
	w.minW, w.minH = 0, 0
	h = w.normalHints(2)
	if h.Flags != xlib.PPosition|xlib.PWinGravity || h.X != -80 || h.Y != 120 || h.WinGravity != 10 {
		t.Fatalf("clearing minimum lost placement: %+v", h)
	}
	w.initialPosition = nil
	if w.normalHints(2).Flags != 0 {
		t.Fatal("empty hints not cleared")
	}
}

func TestInitialClientPositionUsesStaticGravityAndSurvivesMinimumChanges(t *testing.T) {
	w := &Window{initialPosition: &[2]int32{120, 250}, minW: 160}
	for _, minimum := range []float32{160, 0, 240} {
		w.minW = minimum
		h := w.normalHints(2)
		if h.WinGravity != 10 || h.X != 120 || h.Y != 250 || h.Flags&xlib.PPosition == 0 {
			t.Fatalf("client placement lost across minimum update: %+v", h)
		}
	}
}

func TestPositionUnavailableAndInvalid(t *testing.T) {
	w := &Window{}
	if _, err := w.Position(nil); !errors.Is(err, common.ErrUnavailable) {
		t.Fatal(err)
	}
	if err := w.SetPosition(nil, geometry.Point{}); !errors.Is(err, common.ErrUnavailable) {
		t.Fatal(err)
	}
	for _, v := range []float32{float32(math.NaN()), float32(math.Inf(1))} {
		if err := w.SetPosition(nil, geometry.Point{X: v}); err == nil || errors.Is(err, common.ErrUnavailable) {
			t.Fatalf("invalid point was not rejected: %v", err)
		}
	}
}

func TestActivationMessage(t *testing.T) {
	// EWMH 固定为应用来源 1、32 位服务器时间；不冒充 pager，也不捏造活动窗口。
	for _, timestamp := range []xlib.Time{0, 12345, 0xffffffff} {
		message := activationMessage(42, 71, timestamp)
		event := message.ClientMessageEvent()
		if event.Type != xlib.ClientMessage || event.Window != 42 || event.MessageType != 71 || event.Format != 32 {
			t.Fatalf("激活消息头错误：%+v", event)
		}
		if want := [5]int64{1, int64(timestamp), 0, 0, 0}; event.L != want {
			t.Fatalf("激活消息内容为 %v，期望 %v", event.L, want)
		}
	}
}

func TestActivationUserTime(t *testing.T) {
	old := windowMap
	windowMap = map[xlib.Window]*Window{42: {}}
	t.Cleanup(func() { windowMap = old })
	p := &Platform{}
	for _, tc := range []struct {
		kind            xlib.EventType
		window          xlib.Window
		synthetic       xlib.Bool
		timestamp, want xlib.Time
	}{
		{xlib.KeyPress, 42, 0, 123, 123},
		{xlib.ButtonPress, 42, 0, 456, 456},
		{xlib.KeyRelease, 42, 0, 789, 456},
		{xlib.ButtonRelease, 42, 0, 789, 456},
		{xlib.KeyPress, 42, 1, 789, 456},
		{xlib.KeyPress, 99, 0, 789, 456},
		{xlib.KeyPress, 42, 0, 0xffffffff, 0xffffffff},
		{xlib.KeyPress, 42, 0, 1, 1}, // X11 时间回绕仍按服务器事件顺序记录。
	} {
		var event xlib.Event
		// 核心按键与按钮事件的这些头字段具有相同原生布局。
		*event.KeyEvent() = xlib.KeyEvent{Type: tc.kind, Window: tc.window, SendEvent: tc.synthetic, Time: tc.timestamp}
		p.recordUserTime(&event)
		if p.lastUserTime != tc.want {
			t.Fatalf("事件 %+v：时间为 %d，期望 %d", tc, p.lastUserTime, tc.want)
		}
	}
}

func TestShowDestroyedWindow(t *testing.T) {
	if err := (&Window{}).Show(); !errors.Is(err, common.ErrUnavailable) {
		t.Fatalf("已销毁窗口 Show 应返回不可用：%v", err)
	}
	if err := (&Popup{win: &Window{}}).Show(); !errors.Is(err, common.ErrUnavailable) {
		t.Fatalf("已销毁 Popup Show 应返回不可用：%v", err)
	}
}

func TestPopupPhysicalExtent(t *testing.T) {
	for _, tc := range []struct {
		name           string
		logical, scale float32
		want           int
	}{
		{"menu width at 150 percent", 227, 1.5, 341},
		{"menu height at 150 percent", 544, 1.5, 816},
		{"fractional logical width at 125 percent", 151.25, 1.25, 190},
		{"fractional logical height at 175 percent", 160.25, 1.75, 281},
		{"fractional height at 1x", 40.25, 1, 41},
		{"integral pixels at 1x", 227, 1, 227},
		{"integral pixels at 2x", 227, 2, 454},
		{"small positive extent", .25, 1.5, 1},
		{"zero extent", 0, 1.5, 1},
		{"negative extent", -10, 1.5, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := popupPhysicalExtent(tc.logical, tc.scale)
			if got != tc.want {
				t.Fatalf("logical=%g scale=%g: got %d pixels, want %d", tc.logical, tc.scale, got, tc.want)
			}
			// 请求范围有效时，结果必须容纳全文，且不能多占一个完整像素。
			requested := float64(tc.logical) * float64(tc.scale)
			if float64(got) < requested || got > 1 && float64(got-1) >= requested {
				t.Fatal("not the minimum containing physical extent")
			}
		})
	}
}

// 历史桌面测试：依赖 DISPLAY，保留原有入口与跳过条件，后续迁至独立窗口程序。
func TestWindowApplicationClass(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("DISPLAY is not set")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	p, err := NewPlatform("org.golang-gui.IdentityTest")
	if err != nil {
		t.Fatal(err)
	}
	win, err := p.NewWindow(geometry.Size{Width: 120, Height: 80}, func(events.Event) {}, common.WindowOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer win.Destroy()
	d := p.display
	property := d.InternAtom("WM_CLASS", false)
	check := func(class string) {
		t.Helper()
		// 使用创建窗口的连接查询，Xlib 会按顺序提交缓冲请求，无需增加生产同步调用。
		got, actualType := d.GetWindowPropertyBytes(xlib.Window(win.NativeHandle()), property, xlib.AtomString)
		want := p.instanceName + "\x00" + class + "\x00"
		if string(got) != want || actualType != xlib.AtomString {
			t.Fatalf("WM_CLASS = %q (%v), want %q", got, actualType, want)
		}
		var attrs xlib.WindowAttributes
		d.GetWindowAttributes(xlib.Window(win.NativeHandle()), &attrs)
		if attrs.MapState != xlib.IsUnmapped {
			t.Fatal("test window unexpectedly mapped")
		}
	}
	check(p.appId)
	win.SetTitle("A different title")
	runtime.GC()
	check(p.appId)
	// 直接验证未指定应用 ID 的回退，不另行创建平台单例。
	savedID := p.appId
	defer func() { p.appId = savedID }()
	p.appId = ""
	win.(*Window).setApplicationClass()
	check(p.instanceName)
}
