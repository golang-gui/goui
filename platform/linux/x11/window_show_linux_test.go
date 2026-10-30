package x11

import (
	"errors"
	"testing"

	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"
)

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
