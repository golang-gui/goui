package x11

import (
	"encoding/binary"
	"os"
	"runtime"
	"testing"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/linux/libs/xcursor"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"
)

func TestResizeCursorMapping(t *testing.T) {
	for shape, want := range map[common.CursorShape]uint{
		common.CursorResizeHorizontal:  xlib.XC_sb_h_double_arrow,
		common.CursorResizeVertical:    xlib.XC_sb_v_double_arrow,
		common.CursorResizeNWSE:        xlib.XC_top_left_corner,
		common.CursorResizeNESW:        xlib.XC_top_right_corner,
		common.CursorResizeLeft:        xlib.XC_left_side,
		common.CursorResizeRight:       xlib.XC_right_side,
		common.CursorResizeTop:         xlib.XC_top_side,
		common.CursorResizeBottom:      xlib.XC_bottom_side,
		common.CursorResizeTopLeft:     xlib.XC_top_left_corner,
		common.CursorResizeTopRight:    xlib.XC_top_right_corner,
		common.CursorResizeBottomLeft:  xlib.XC_bottom_left_corner,
		common.CursorResizeBottomRight: xlib.XC_bottom_right_corner,
	} {
		if got := cursorFontIndex(shape); got != want {
			t.Fatalf("shape %v: %v != %v", shape, got, want)
		}
	}
}

func TestCursorThemeNamesAndFallback(t *testing.T) {
	for _, tc := range []struct {
		shape        common.CursorShape
		name, legacy string
	}{
		{common.CursorResizeLeft, "w-resize", "left_side"},
		{common.CursorResizeRight, "e-resize", "right_side"},
		{common.CursorResizeTop, "n-resize", "top_side"},
		{common.CursorResizeBottom, "s-resize", "bottom_side"},
		{common.CursorResizeTopLeft, "nw-resize", "top_left_corner"},
		{common.CursorResizeTopRight, "ne-resize", "top_right_corner"},
		{common.CursorResizeBottomLeft, "sw-resize", "bottom_left_corner"},
		{common.CursorResizeBottomRight, "se-resize", "bottom_right_corner"},
	} {
		names := cursorNames(tc.shape)
		if len(names) != 2 || names[0] != tc.name || names[1] != tc.legacy {
			t.Fatalf("shape %v: %v", tc.shape, names)
		}
		for hit := 0; hit < 3; hit++ {
			var calls []string
			coreCalled := false
			got := loadNamedCursor(tc.shape, func(name string) xlib.Cursor {
				calls = append(calls, name)
				if len(calls)-1 == hit {
					return 123
				}
				return 0
			}, func(index uint) xlib.Cursor {
				coreCalled = true
				if index != cursorFontIndex(tc.shape) {
					t.Fatal("wrong core fallback")
				}
				return 123
			})
			if got != 123 || coreCalled != (hit == 2) || len(calls) != min(hit+1, 2) {
				t.Fatalf("fallback %d: cursor=%v calls=%v core=%v", hit, got, calls, coreCalled)
			}
		}
	}
}

func cursorSettingsBytes(order binary.ByteOrder, name string, size int32) []byte {
	appendOrder := order.(binary.AppendByteOrder)
	data := make([]byte, 12)
	if order == binary.BigEndian {
		data[0] = 1
	}
	order.PutUint32(data[8:], 3)
	pad := func() {
		for len(data)%4 != 0 {
			data = append(data, 0)
		}
	}
	entry := func(kind byte, key string) {
		data = append(data, kind, 0)
		data = appendOrder.AppendUint16(data, uint16(len(key)))
		data = append(data, key...)
		pad()
		data = appendOrder.AppendUint32(data, 1)
	}
	entry(1, "Gtk/CursorThemeName")
	data = appendOrder.AppendUint32(data, uint32(len(name)))
	data = append(data, name...)
	pad()
	entry(0, "Gtk/CursorThemeSize")
	data = appendOrder.AppendUint32(data, uint32(size))
	entry(2, "Unrelated/Color")
	data = append(data, make([]byte, 8)...)
	return data
}

func TestParseCursorSettings(t *testing.T) {
	for _, order := range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian} {
		data := cursorSettingsBytes(order, "Adwaita", 32)
		got, ok := parseCursorSettings(data)
		if !ok || got != (cursorConfig{"Adwaita", 32}) {
			t.Fatalf("parsed %v, %v", got, ok)
		}
		for i := 0; i < len(data); i++ {
			if _, ok := parseCursorSettings(data[:i]); ok {
				t.Fatalf("accepted truncated settings: %d/%d", i, len(data))
			}
		}
		data[12] = 3
		if _, ok := parseCursorSettings(data); ok {
			t.Fatal("accepted unknown type")
		}
		data[0] = 2
		if _, ok := parseCursorSettings(data); ok {
			t.Fatal("accepted invalid byte order")
		}
	}
	// 超长字符串必须直接拒绝，不能按声明长度分配内存。
	data := cursorSettingsBytes(binary.LittleEndian, "x", 24)
	binary.LittleEndian.PutUint32(data[40:44], ^uint32(0))
	if _, ok := parseCursorSettings(data); ok {
		t.Fatal("accepted oversized string")
	}
}

func FuzzParseCursorSettings(f *testing.F) {
	f.Add(cursorSettingsBytes(binary.LittleEndian, "default", 24))
	f.Add(cursorSettingsBytes(binary.BigEndian, "", 0))
	f.Fuzz(func(t *testing.T, data []byte) { parseCursorSettings(data) })
}

// 历史桌面测试：保留原有 DISPLAY／libXcursor 条件，后续迁至独立窗口程序。
// 使用私有窗口和 selection，不修改用户设置或真实 XSETTINGS selection owner。
func TestNativeCursorThemeLifecycle(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("DISPLAY is not set")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if !xcursor.Available() {
		t.Skip("libXcursor unavailable")
	}
	t.Setenv("XCURSOR_THEME", "")
	t.Setenv("XCURSOR_SIZE", "")
	d := xlib.OpenDisplay("")
	if d == 0 {
		t.Fatal("cannot open display")
	}
	defer d.Close()
	screen := d.DefaultScreenOfDisplay()
	makeWindow := func() xlib.Window {
		return d.CreateWindow(screen.Root, 0, 0, 20, 20, 0, int(screen.RootDepth),
			xlib.WindowClassInputOutput, screen.RootVisual, 0, nil)
	}
	root, owner, window := makeWindow(), makeWindow(), makeWindow()
	defer d.DestroyWindow(root)
	defer d.DestroyWindow(window)
	theme := &cursorTheme{
		display: d, root: root, available: true, defaultSize: 24,
		selection: d.InternAtom("GOUI_TEST_CURSOR_SELECTION", false),
		settings:  d.InternAtom("_XSETTINGS_SETTINGS", false),
		manager:   d.InternAtom("MANAGER", false),
		resources: d.InternAtom("RESOURCE_MANAGER", false),
		cursors:   make(map[*cursor]struct{}),
	}
	theme.rootMask = theme.subscribe(root)
	d.SetSelectionOwner(theme.selection, owner, 0)
	write := func(w xlib.Window, property, kind xlib.Atom, data []byte) {
		d.ChangeProperty(w, property, kind, 8, xlib.PropModeReplace, cgo.CSlice(data), len(data))
	}
	write(root, theme.resources, xlib.AtomString, []byte("Xcursor.theme: default\nXcursor.size: 20\n"))
	write(owner, theme.settings, theme.settings, cursorSettingsBytes(binary.LittleEndian, "Adwaita", 32))
	theme.refresh()
	if theme.config != (cursorConfig{"Adwaita", 32}) {
		t.Fatalf("initial config: %v", theme.config)
	}
	c := &cursor{window: &Window{wid: window}, display: d, theme: theme, cache: make(map[common.CursorShape]xlib.Cursor)}
	theme.cursors[c] = struct{}{}
	defer c.Destroy()
	c.SetShape(common.CursorResizeLeft)
	first := c.current
	if first == 0 {
		t.Fatal("no theme cursor")
	}
	c.SetShape(common.CursorResizeLeft)
	if c.current != first {
		t.Fatal("cursor cache miss")
	}
	// 通过原生 PropertyNotify 更新主题，不以 GUI SetShape 代替原生通知。
	write(owner, theme.settings, theme.settings, cursorSettingsBytes(binary.BigEndian, "default", 40))
	drain := func() {
		d.GetSelectionOwner(theme.selection) // 查询往返，确保此前的通知已经入队。
		for d.Pending() > 0 {
			event := d.NextEvent()
			theme.handleEvent(&event)
		}
	}
	drain()
	if theme.config != (cursorConfig{"default", 40}) || xcursor.GetDefaultSize(d) != 40 ||
		xcursor.GetTheme(d) != "default" || c.current == 0 || c.current == first {
		t.Fatalf("live theme update failed: %v cursor=%v", theme.config, c.current)
	}
	// 设置缺失时回退至 X Resources，并保留当前光标形状。
	d.DeleteProperty(owner, theme.settings)
	drain()
	if theme.config != (cursorConfig{"default", 20}) || c.shape != common.CursorResizeLeft {
		t.Fatalf("resource fallback failed: %v", theme.config)
	}
	c.SetShape(common.CursorNone)
	hidden := c.current
	write(root, theme.resources, xlib.AtomString, []byte("Xcursor.size: 28\n"))
	drain()
	if theme.config.size != 28 || c.current != hidden {
		t.Fatal("hidden cursor changed on reload")
	}
	d.DestroyWindow(owner)
	drain()
	if theme.owner != 0 {
		t.Fatal("retained destroyed settings owner")
	}
	// 设置管理器重启后，通过根窗口 MANAGER 通知发现新的 owner。
	owner = makeWindow()
	defer d.DestroyWindow(owner)
	d.SetSelectionOwner(theme.selection, owner, 0)
	write(owner, theme.settings, theme.settings, cursorSettingsBytes(binary.LittleEndian, "default", 36))
	var event xlib.Event
	*event.ClientMessageEvent() = xlib.ClientMessageEvent{
		Type: xlib.ClientMessage, Window: root, MessageType: theme.manager, Format: 32,
		L: [5]int64{0, int64(theme.selection), int64(owner)},
	}
	d.SendEvent(root, false, xlib.EventMaskStructureNotify, &event)
	drain()
	if theme.owner != owner || theme.config.size != 36 {
		t.Fatal("manager restart not observed")
	}
	c.Destroy()
	if len(theme.cursors) != 0 || c.display != 0 {
		t.Fatal("cursor not detached")
	}
	var attributes xlib.WindowAttributes
	d.GetWindowAttributes(root, &attributes)
	if attributes.YourEventMask != theme.rootMask {
		t.Fatal("root subscription leaked")
	}
}
