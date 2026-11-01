package main

import (
	"fmt"
	"os"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/linux/libs/glx"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"
)

func prepareNativeTrace() error {
	if painter := os.Getenv("GOUI_PLAT_PAINTER"); painter != "" && painter != "opengl" {
		return fmt.Errorf("Linux 原生观察器需要 OpenGL/GLX，请移除 GOUI_PLAT_PAINTER=%q", painter)
	}
	return os.Setenv("GOUI_PLAT_PAINTER", "opengl")
}

func startNativeTrace(w gui.Window, out *traceLog, ready func(error)) (func() error, error) {
	var display xlib.Display
	var previous [128]xlib.WireToEvent
	attempted := false
	decoder := xlib.WireToEvent(cgo.NewCallback(func(d xlib.Display, event *xlib.Event, wire *[32]byte) int32 {
		raw := *wire
		kind := raw[0] & 0x7f
		converted := previous[kind].Call(d, event, wire)
		detail := ""
		if converted != 0 {
			e := event.AnyEvent()
			detail = fmt.Sprintf(" serial=%d sendEvent=%d window=%#x", e.Serial, e.SendEvent, e.Window)
			switch event.Type {
			case xlib.KeyPress, xlib.KeyRelease:
				k := event.KeyEvent()
				detail += fmt.Sprintf(" time=%d keycode=%d state=%#x pos=%d,%d root=%d,%d", k.Time, k.KeyCode, k.State, k.X, k.Y, k.XRoot, k.YRoot)
			case xlib.ButtonPress, xlib.ButtonRelease:
				b := event.ButtonEvent()
				detail += fmt.Sprintf(" time=%d button=%d state=%#x pos=%d,%d root=%d,%d", b.Time, b.Button, b.State, b.X, b.Y, b.XRoot, b.YRoot)
			case xlib.MotionNotify:
				m := event.MotionEvent()
				detail += fmt.Sprintf(" time=%d state=%#x pos=%d,%d root=%d,%d", m.Time, m.State, m.X, m.Y, m.XRoot, m.YRoot)
			}
		}
		out.record("NATIVE", "x11 type=%d(%s) converted=%d raw=%x%s", kind, xEventName(int(kind)), converted, raw, detail)
		return int32(converted)
	}))
	frame := w.ConnectAfterPaint(func(_ gui.Painter, bounds geometry.Rectangle) {
		if attempted || bounds.Width <= 0 || bounds.Height <= 0 {
			return
		}
		attempted = true
		display = glx.GetCurrentDisplay()
		if display == 0 {
			ready(fmt.Errorf("当前没有 GLX 上下文；不能安装 X11 原生观察器"))
			return
		}
		count := 0
		for kind := 2; kind < len(previous); kind++ {
			previous[kind] = display.SetWireToEvent(int32(kind), decoder)
			if previous[kind] == 0 {
				// 没有原转换器时立即恢复，不能替 Xlib 决定是否入队。
				display.SetWireToEvent(int32(kind), 0)
			} else {
				count++
			}
		}
		out.record("ENV", "native=X11 display=%#x decoderCount=%d install=first-GLX-frame; raw is fixed wire header, not extension payload", display, count)
		ready(nil)
	})
	return func() error {
		frame.Disconnect()
		if display != 0 {
			for kind, old := range previous {
				if old != 0 {
					display.SetWireToEvent(int32(kind), old)
				}
			}
			display = 0
		}
		return nil
	}, nil
}

func xEventName(kind int) string {
	names := [...]string{"Error", "Reply", "KeyPress", "KeyRelease", "ButtonPress", "ButtonRelease", "MotionNotify", "EnterNotify", "LeaveNotify", "FocusIn", "FocusOut", "KeymapNotify", "Expose", "GraphicsExpose", "NoExpose", "VisibilityNotify", "CreateNotify", "DestroyNotify", "UnmapNotify", "MapNotify", "MapRequest", "ReparentNotify", "ConfigureNotify", "ConfigureRequest", "GravityNotify", "ResizeRequest", "CirculateNotify", "CirculateRequest", "PropertyNotify", "SelectionClear", "SelectionRequest", "SelectionNotify", "ColormapNotify", "ClientMessage", "MappingNotify", "GenericEvent"}
	if kind >= 0 && kind < len(names) {
		return names[kind]
	}
	return "extension"
}
