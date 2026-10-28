package xerrors

import (
	"testing"

	"github.com/golang-gui/goui/platform/linux/libs/xlib"
)

func TestExpectedErrorScope(t *testing.T) {
	// 不需要 X server：验证原生错误必须同时匹配连接、序列、资源和错误码。
	cases := []struct {
		name    string
		display xlib.Display
		event   xlib.ErrorEvent
		want    bool
	}{
		{"match", 1, xlib.ErrorEvent{Display: 1, Serial: 10, ResourceID: 42, ErrorCode: xlib.BadWindow}, true},
		{"other connection", 2, xlib.ErrorEvent{Display: 1, Serial: 10, ResourceID: 42, ErrorCode: xlib.BadWindow}, false},
		{"event connection", 1, xlib.ErrorEvent{Display: 2, Serial: 10, ResourceID: 42, ErrorCode: xlib.BadWindow}, false},
		{"before", 1, xlib.ErrorEvent{Display: 1, Serial: 9, ResourceID: 42, ErrorCode: xlib.BadWindow}, false},
		{"after", 1, xlib.ErrorEvent{Display: 1, Serial: 12, ResourceID: 42, ErrorCode: xlib.BadWindow}, false},
		{"other resource", 1, xlib.ErrorEvent{Display: 1, Serial: 10, ResourceID: 43, ErrorCode: xlib.BadWindow}, false},
		{"other error", 1, xlib.ErrorEvent{Display: 1, Serial: 10, ResourceID: 42, ErrorCode: 2}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			failed := false
			pending = []*Trap{{display: 1, first: 10, end: 12, resources: []xlib.ID{42}, codes: []uint8{xlib.BadWindow}, failed: &failed}}
			t.Cleanup(func() { pending = nil })
			if got := Consume(c.display, &c.event); got != c.want || failed != c.want {
				t.Fatalf("consumed=%v failed=%v want=%v", got, failed, c.want)
			}
		})
	}
}

func TestExpectedErrorRetirement(t *testing.T) {
	failed := false
	active := &Trap{display: 1, first: 20, failed: &failed}
	empty := &Trap{display: 1, first: 18, end: 18, failed: &failed}
	late := &Trap{display: 1, first: 10, end: 12, resources: []xlib.ID{42}, codes: []uint8{xlib.BadDrawable}, failed: &failed}
	other := &Trap{display: 2, first: 1, end: 2, failed: &failed}
	pending = []*Trap{late, empty, active, other}
	t.Cleanup(func() { pending = nil })
	retire(1, 10)
	if len(pending) != 3 || pending[0] != late {
		t.Fatal("尚未处理的异步请求被提前移除")
	}
	if !Consume(1, &xlib.ErrorEvent{Display: 1, Serial: 11, ResourceID: 42, ErrorCode: xlib.BadDrawable}) || !failed {
		t.Fatal("作用域结束后的延迟错误没有匹配")
	}
	retire(1, 11)
	if len(pending) != 2 || pending[0] != active || pending[1] != other {
		t.Fatal("回收改变了进行中的请求或其他连接")
	}
}
