// X11原生XSETTINGS与GUI/UI自动更新验收。整个应用不运行或推进GTK/GLib事件循环。
//
// 环境：仅Linux/X11，必须使用没有XSETTINGS manager的独立Xvnc+WM会话；若已有
// owner立即退出，不替换用户桌面的设置服务。记录GTK版本、WM、后端和1x/2x。
// 启动：go run ./tests/window/platform/xsettings（仓库根目录）。
// GOUI_PLAT_PAINTER=software/opengl，GOUI_PLAT_SCALE=1/2。
// 可选-dpi 192发布Xft/DPI，-expect-scale 2校验实际Painter缩放。依次验证
// GOUI_PLAT_SCALE > GDK_SCALE > XSETTINGS Xft/DPI > X Resources Xft.dpi；
// X Resources测试只在专用会话使用xrdb设置，退出后由启动者恢复。
// 初始：本例在独立X连接上发布Adwaita、Sans 10和24 px光标；先查询平台设置，
// 再创建GUI窗口。GUI仍沿用2秒轮询。两个固定CSS主题解压到进程私有临时目录。
//
// 操作与逐项预期：
//  1. 点击输入框、Ctrl+A，F6；记住输入身份、焦点与选区。后续按钮不抢焦点。
//  2. 每次点击后等待至少一次设置轮询（以BUILD日志为准），再F6：Dark、Light
//     双向更新整个Modern界面；Font变为Serif 15；Red、Green、Red、Green验证
//     固定主题的独立颜色预期，Red含半透明通道；按钮只修改X属性，不主动更新UI。
//  3. Missing删除主题项；Bad type将主题名改为整数；Corrupt发布截断数据。
//     F6要求平台按错误返回、GUI使用明暗／颜色fallback，不保留旧主题；恢复使用
//     Restart。Missing／Bad type仍保留有效字体，Corrupt使整个协议快照失效。
//  4. Gone销毁owner，F6确认无manager不沿用旧值；Restart发现新owner。Replace
//     换owner而保留旧窗口存活，F6验证新值；再次Dark/Light。Cursor切换到40，
//     悬停右侧Cross区域检查原生光标不丢失；Bool仅改动画开关，等待后F6要求UI
//     构建次数不增加（该GTK设置尚未映射到GUI公开字段）。
//  5. F8要求上述操作完成、输入焦点／选区保留、窗口与控件身份稳定。PASS退出0；
//     提前关闭或断言失败退出1。原生截图核对整体明暗、文字尺寸、强调色与光标。
//
// 副作用：专用显示内的XSETTINGS owner/属性、临时主题文件及进程环境；不修改
// 用户配置、系统剪贴板或应用业务数据。退出销毁本例manager并删除临时文件。
// 复位：重启。无manager或字段无效时GUI逐字段兜底；Pango原生字体fallback保留。
package main

import (
	"embed"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/linux/libs/gtk"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"
	"github.com/golang-gui/goui/platform/linux/x11"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
)

//go:embed testdata/themes
var themes embed.FS

const appID = "org.golanggui.XSettings"

type manager struct {
	display                      xlib.Display
	root, owner                  xlib.Window
	selection, property, manager xlib.Atom
	windows                      []xlib.Window
	serial                       uint32
	mode, theme, font            string
	size                         int32
	animations                   int32
	dpi                          int32
}

func newManager() (*manager, error) {
	d := xlib.OpenDisplay("")
	if d == 0 {
		return nil, errors.New("no X display")
	}
	m := &manager{display: d, root: d.DefaultRootWindow(), theme: "Adwaita", font: "Sans 10", size: 24, animations: 1}
	m.selection = d.InternAtom(fmt.Sprintf("_XSETTINGS_S%d", d.DefaultScreen()), false)
	m.property = d.InternAtom("_XSETTINGS_SETTINGS", false)
	m.manager = d.InternAtom("MANAGER", false)
	if d.GetSelectionOwner(m.selection) != 0 {
		d.Close()
		return nil, errors.New("use a dedicated X11 session with no XSETTINGS manager")
	}
	m.replace()
	return m, nil
}
func (m *manager) replace() {
	s := m.display.DefaultScreenOfDisplay()
	m.owner = m.display.CreateWindow(m.root, 0, 0, 1, 1, 0, int(s.RootDepth), xlib.WindowClassInputOutput, s.RootVisual, 0, nil)
	m.windows = append(m.windows, m.owner)
	m.mode = ""
	m.publish()
	m.display.SetSelectionOwner(m.selection, m.owner, 0)
	var e xlib.Event
	*e.ClientMessageEvent() = xlib.ClientMessageEvent{Type: xlib.ClientMessage, Window: m.root, MessageType: m.manager, Format: 32, L: [5]int64{0, int64(m.selection), int64(m.owner)}}
	m.display.SendEvent(m.root, false, xlib.EventMaskStructureNotify, &e)
	m.display.Flush()
}
func (m *manager) gone() {
	if m.owner != 0 {
		m.display.DestroyWindow(m.owner)
		for i, w := range m.windows {
			if w == m.owner {
				m.windows = append(m.windows[:i], m.windows[i+1:]...)
				break
			}
		}
		m.owner = 0
		m.display.Flush()
	}
}
func (m *manager) close() {
	for _, w := range m.windows {
		m.display.DestroyWindow(w)
	}
	m.display.Close()
}
func (m *manager) publish() {
	if m.owner == 0 {
		return
	}
	m.serial++
	order := binary.LittleEndian
	type entry struct {
		kind       byte
		name, text string
		number     int32
	}
	entries := []entry{{1, "Gtk/FontName", m.font, 0}, {1, "Gtk/CursorThemeName", "Adwaita", 0}, {0, "Gtk/CursorThemeSize", "", m.size}, {0, "Gtk/EnableAnimations", "", m.animations}}
	if m.dpi > 0 {
		entries = append(entries, entry{0, "Xft/DPI", "", m.dpi})
	}
	if m.mode != "missing" {
		e := entry{1, "Net/ThemeName", m.theme, 0}
		if m.mode == "badtype" {
			e = entry{0, "Net/ThemeName", "", 7}
		}
		entries = append(entries, e)
	}
	data := []byte{0, 0, 0, 0}
	data = order.AppendUint32(data, m.serial)
	data = order.AppendUint32(data, uint32(len(entries)))
	for _, e := range entries {
		data = append(data, e.kind, 0)
		data = order.AppendUint16(data, uint16(len(e.name)))
		data = append(data, e.name...)
		for len(data)%4 != 0 {
			data = append(data, 0)
		}
		data = order.AppendUint32(data, m.serial)
		if e.kind == 0 {
			data = order.AppendUint32(data, uint32(e.number))
		} else {
			data = order.AppendUint32(data, uint32(len(e.text)))
			data = append(data, e.text...)
			for len(data)%4 != 0 {
				data = append(data, 0)
			}
		}
	}
	if m.mode == "corrupt" {
		data = data[:len(data)-1]
	}
	m.display.ChangeProperty(m.owner, m.property, m.property, 8, xlib.PropModeReplace, cgo.CSlice(data), len(data))
	m.display.Flush()
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL", err)
		os.Exit(1)
	}
}
func run() error {
	dpi := flag.Float64("dpi", 0, "Initial XSETTINGS DPI; zero omits the field")
	expectScale := flag.Float64("expect-scale", 0, "Expected painter scale; zero disables the assertion")
	flag.Parse()
	dir, err := os.MkdirTemp("", "goui-xsettings-themes-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	err = fs.WalkDir(themes, "testdata", func(name string, item fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if item.IsDir() {
			return nil
		}
		data, err := themes.ReadFile(name)
		if err != nil {
			return err
		}
		out := filepath.Join(dir, strings.TrimPrefix(name, "testdata/"))
		if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
			return err
		}
		return os.WriteFile(out, data, 0644)
	})
	if err != nil {
		return err
	}
	if err := os.Setenv("XDG_DATA_HOME", dir); err != nil {
		return err
	}
	m, err := newManager()
	if err != nil {
		return err
	}
	defer m.close()
	m.dpi = int32(*dpi * 1024)
	m.publish()
	p, err := x11.NewPlatform(appID)
	if err != nil {
		return err
	}
	native, err := p.NewSettings()
	if err != nil {
		return err
	}
	dark, err := native.ColorScheme()
	if err != nil || dark != common.ColorSchemeLight {
		return fmt.Errorf("before-window color=%v err=%v", dark, err)
	}
	family, err := native.FontFamily()
	if err != nil || family != "Sans" {
		return fmt.Errorf("before-window font=%q err=%v", family, err)
	}
	fmt.Println("CHECK native settings before any GOUI window/cursor")
	lastPaint := 0
	stage := "initial"
	builds, baselineBuild := 0, 0
	ready, finished := false, false
	seen := map[string]bool{}
	var failure error
	var originalInput gui.Widget
	var originalWindow gui.Window
	var originalText string
	var originalSheet color.RGBA64
	var originalNativeTheme string
	err = ui.Run(appID, func(app ui.App) ui.RootView {
		builds++
		settings := app.Settings()
		fmt.Printf("BUILD n=%d dark=%t family=%q size=%g accent=%v\n", builds, settings.ColorScheme() == ui.ColorSchemeDark, settings.FontFamily(), settings.FontSize(), color.RGBA64Model.Convert(settings.AccentColor()))
		fail := func(err error) { failure = err; app.Quit() }
		bounds := func() map[string]geometry.Rectangle {
			out := map[string]geometry.Rectangle{}
			for _, id := range []string{"dark", "light", "font", "red", "green", "missing", "badtype", "corrupt", "gone", "restart", "replace", "cursor", "bool", "input", "cross"} {
				if w := app.FindWidget("main", id); w != nil {
					out[id] = w.Snapshot().Bounds
				}
			}
			return out
		}
		set := func(next string, fn func()) func() {
			return func() { stage = next; fn(); m.publish(); baselineBuild = builds }
		}
		button := func(id, text string, fn func()) ui.View {
			return ui.Button(text).ID(id).Focusable(false).FocusOnPointer(false).OnClick(fn)
		}
		check := func() error {
			input := app.FindWidget("main", "input").(*gui.TextInput)
			if !ready || input != originalInput || app.FindWindow("main") != originalWindow || input.Text() != originalText {
				return errors.New("rebuild replaced window/input or reset text")
			}
			if !input.Focused() || input.Selection().Anchor != 0 || input.Selection().Caret != len(input.Text()) {
				return fmt.Errorf("selection/focus lost: %+v focus=%t", input.Selection(), input.Focused())
			}
			wantError := m.owner == 0 || m.mode != ""
			scheme, schemeErr := native.ColorScheme()
			if (schemeErr != nil) != wantError {
				return fmt.Errorf("native source error=%v mode=%q owner=%d", schemeErr, m.mode, m.owner)
			}
			wantDark := !wantError && strings.Contains(strings.ToLower(m.theme), "dark")
			if !wantError && (scheme == common.ColorSchemeDark) != wantDark {
				return errors.New("native scheme disagrees with published theme")
			}
			if (settings.ColorScheme() == ui.ColorSchemeDark) != wantDark {
				return fmt.Errorf("GUI still has old color scheme at %s", stage)
			}
			if m.owner != 0 && m.mode != "corrupt" {
				wantFamily, wantSize := "Sans", float32(10)
				if m.font == "Serif 15" {
					wantFamily, wantSize = "Serif", 15
				}
				if settings.FontFamily() != wantFamily || settings.FontSize() != wantSize {
					return fmt.Errorf("stale font %q/%g want %q/%g", settings.FontFamily(), settings.FontSize(), wantFamily, wantSize)
				}
			}
			got := color.RGBA64Model.Convert(settings.AccentColor()).(color.RGBA64)
			want := originalSheet
			if wantError {
				want = color.RGBA64{R: 70 * 257, G: 130 * 257, B: 220 * 257, A: 65535}
			} else if m.theme == "GouiXSettingsRed" {
				want = color.RGBA64{R: 26214, G: 4369, A: 32768}
			} else if m.theme == "GouiXSettingsDark" {
				want = color.RGBA64{G: 43690, B: 17476, A: 65535}
			}
			// 固定CSS字面量的独立预乘sRGB预期；16位归一化舍入容许1级。
			for i, v := range []uint16{got.R, got.G, got.B, got.A} {
				w := []uint16{want.R, want.G, want.B, want.A}[i]
				if int(v) < int(w)-1 || int(v) > int(w)+1 {
					return fmt.Errorf("accent=%v want=%v", got, want)
				}
			}
			gtkSettings, _ := gtk.SettingsGetDefault()
			cached, _ := gtkSettings.StringProperty("gtk-theme-name")
			if cached != originalNativeTheme {
				return errors.New("test unexpectedly advanced or changed GtkSettings")
			}
			if stage == "bool" && builds != baselineBuild {
				return fmt.Errorf("unmapped GTK bool caused rebuild %d -> %d", baselineBuild, builds)
			}
			seen[stage] = true
			data, _ := json.Marshal(bounds())
			fmt.Printf("CHECK stage=%s builds=%d cachedGTK=%q bounds=%s\n", stage, builds, cached, data)
			return nil
		}
		input := ui.TextInput().ID("input").MinSize(420, 0)
		if !ready {
			input.Text("keep selection across theme changes")
		}
		content := ui.VBox(
			ui.Label(fmt.Sprintf("dark=%t font=%s %g pt", settings.ColorScheme() == ui.ColorSchemeDark, settings.FontFamily(), settings.FontSize())),
			ui.HBox(button("dark", "Dark", set("dark", func() { m.mode = ""; m.theme = "Adwaita-dark" })), button("light", "Light", set("light", func() { m.mode = ""; m.theme = "Adwaita" })), button("font", "Font", set("font", func() { m.font = "Serif 15" })), button("red", "Red", set("red", func() { m.mode = ""; m.theme = "GouiXSettingsRed" })), button("green", "Green", set("green", func() { m.mode = ""; m.theme = "GouiXSettingsDark" }))).Spacing(8),
			ui.HBox(button("missing", "Missing", set("missing", func() { m.mode = "missing" })), button("badtype", "Bad type", set("badtype", func() { m.mode = "badtype" })), button("corrupt", "Corrupt", set("corrupt", func() { m.mode = "corrupt" })), button("gone", "Gone", set("gone", m.gone)), button("restart", "Restart", set("restart", func() { m.theme = "Adwaita-dark"; m.replace() })), button("replace", "Replace", set("replace", func() { m.theme = "Adwaita"; m.replace() }))).Spacing(8),
			ui.HBox(button("cursor", "Cursor", set("cursor", func() { m.size = 40 })), button("bool", "Bool", set("bool", func() { m.animations = 1 - m.animations })), ui.Label("Cross cursor area").ID("cross").Cursor(ui.CursorCrosshair)).Spacing(10),
			input, ui.Button("Accent sample").Style(modern.Primary).Focusable(false).FocusOnPointer(false),
			ui.Label("Select input with Ctrl+A. Change setting, wait for polling, F6 check; F8 finish."),
		).Spacing(12).Padding(16).CrossAlign(layout.CrossStart)
		return ui.Root().StyleSheet(modern.FromSettings(settings)).Windows(ui.Window("main").Title("GOUI XSETTINGS").Size(1100, 430).Content(content).Shortcuts(
			ui.Shortcut(ui.KeyF6).OnActivate(func() {
				if err := check(); err != nil {
					fail(err)
				}
			}),
			ui.Shortcut(ui.KeyF8).OnActivate(func() {
				for _, s := range []string{"initial", "dark", "light", "font", "red", "green", "missing", "badtype", "corrupt", "gone", "restart", "replace", "cursor", "bool"} {
					if !seen[s] {
						fail(fmt.Errorf("missing stage %s", s))
						return
					}
				}
				finished = true
				fmt.Println("PASS XSETTINGS")
				app.Quit()
			}),
		).OnAfterPaint(func(painter gui.Painter, _ geometry.Rectangle) {
			if ready {
				if builds != lastPaint {
					lastPaint = builds
					data, _ := json.Marshal(bounds())
					fmt.Printf("PAINT build=%d bounds=%s\n", builds, data)
				}
				return
			}
			w := app.FindWidget("main", "input")
			if w == nil || w.Snapshot().Bounds.Width <= 0 || w.Snapshot().Bounds.X <= 0 {
				return
			}
			if *expectScale > 0 && math.Abs(float64(painter.PixelScale())-*expectScale) > 1e-6 {
				failure = fmt.Errorf("painter scale=%g want=%g", painter.PixelScale(), *expectScale)
				app.Post(app.Quit)
				return
			}
			originalInput = w
			originalWindow = app.FindWindow("main")
			originalText = w.(*gui.TextInput).Text()
			originalSheet = color.RGBA64Model.Convert(settings.AccentColor()).(color.RGBA64)
			gs, _ := gtk.SettingsGetDefault()
			originalNativeTheme, _ = gs.StringProperty("gtk-theme-name")
			resource, e := painter.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
			if e != nil {
				failure = e
				app.Post(app.Quit)
				return
			}
			defer resource.Destroy()
			ready = true
			data, _ := json.Marshal(bounds())
			fmt.Printf("READY backend=%T scale=%g GTK=%q bounds=%s\n", resource, painter.PixelScale(), originalNativeTheme, data)
		}))
	})
	if err != nil {
		return err
	}
	if failure != nil {
		return failure
	}
	if !finished {
		return errors.New("closed before all checks")
	}
	return nil
}
