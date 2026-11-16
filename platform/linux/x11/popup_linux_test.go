package x11

import (
	"testing"

	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"
)

// 尺寸查询已观察到本次结果后，队列中上次借用留下的 ConfigureNotify 不能倒灌。
func TestPopupQueriedGeometrySupersedesQueuedConfiguration(t *testing.T) {
	previousPlatform := platform
	platform = nil
	t.Cleanup(func() { platform = previousPlatform })
	const id xlib.Window = 0x7654321
	previous, existed := windowMap[id]
	t.Cleanup(func() {
		if existed {
			windowMap[id] = previous
		} else {
			delete(windowMap, id)
		}
	})
	sizes := 0
	w := &Window{wid: id, width: 220, height: 150, geometrySerial: 42, overrideRedirect: true, paintPending: true}
	w.onEvent = func(event events.Event) {
		if _, ok := event.(events.SizeEvent); ok {
			sizes++
		}
	}
	windowMap[id] = w
	var event xlib.Event
	configure := event.ConfigureEvent()
	configure.Type, configure.Window = xlib.ConfigureNotify, id
	configure.Serial, configure.Width, configure.Height = 41, 100, 30
	handleEvent(event)
	if w.width != 220 || w.height != 150 || sizes != 0 {
		t.Fatal("旧尺寸覆盖了查询所得的实际几何")
	}
	configure.Serial = 42
	configure.Width, configure.Height = 210, 140
	handleEvent(event)
	if w.width != 210 || w.height != 140 || sizes != 1 {
		// 其他 X client 的后续请求可以带相同的“本连接最后已处理 serial”。
		t.Fatal("与查询同 serial 的后续外部配置被误判为旧事件")
	}
	configure.Serial = 43
	configure.Width, configure.Height = 100, 30
	handleEvent(event)
	if w.width != 100 || w.height != 30 || sizes != 2 {
		t.Fatal("真正晚于查询的尺寸通知被丢弃")
	}
}
