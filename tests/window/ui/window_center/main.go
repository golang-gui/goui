// 窗口初始居中与声明式创建选项验收，不使用 DevServer。
//
// 环境：Windows/macOS 可交互桌面，或支持工作区查询的 Linux/X11 窗口管理器。
// 启动：go run ./tests/window/ui/window_center [-auto] [-chrome=native|integrated|none]
// 可加 -center=false、-override、-skip-taskbar=true；用 GOUI_PLAT_SCALE=1/2 重跑，
// 记录系统版本、窗口管理器和实际后端。应用缩放覆盖不等于真实系统 HiDPI 验收。
// 初始：360×300 DIP 客户区，默认请求工作区数学居中；不是 AppKit 默认的垂直偏上。
// AfterUpdate 在首次 Show 前断言隐藏状态及居中坐标；-override 随后显式定位到
// 工作区 (120,80)，首次出现时应以这个位置为准。-center=false 不断言系统默认坐标。
//
// 操作与逐项预期：
//  1. 三种 Chrome 分别启动：客户区中心与工作区中心重合，无先出现再搬动的视觉跳变。
//     Native 原生标题栏不参与居中尺寸，Integrated/None 的内容区仍按同一契约定位。
//  2. 按 F5 或点击 Move：移到工作区 (100,60)；再按 F6/Rebuild，窗口对象及位置不变。
//  3. 按 F7/Hide-Show：隐藏后重现，保留当前位置，不重新居中。
//     Native/Integrated 可先拖动标题栏，再重复 F6/F7，保留手动移动的位置。
//  4. 加 -override 重启：窗口直接出现在 (120,80)，Center 不覆盖后续显式定位。
//  5. 加 -center=false 重启：由系统选择默认位置；F5/F6/F7 的行为不变。
//  6. 加 -skip-taskbar=true：居中、激活与按钮输入仍可用；任务栏差异见 taskbar 用例。
//
// -auto：每阶段等待 400ms 处理真实窗口事件，断言显示前位置、显示后实际尺寸/工作区
// 对称性、显式移动、UI 重建身份与位置、Hide/Show 保持，输出 PASS 后退出。
// 坐标容差 0.5 DIP：在指定 1x/2x 下最多允许一个物理像素的原生取整误差。
// 系统可以约束或忽略定位，必须记录为未满足本例断言，不伪装成功；此例不覆盖多屏、
// 混合 DPI、超大窗口、最大化或内容最小尺寸大于初始 Size 的情况。
// 已知差异：Xfwm4 对 -center=false、没有初始位置提示的窗口，可能在 Hide/Show 后
// 重新选择位置。本例仍报告位置保持断言失败；此行为已在修改前代码复现，不是 Center
// 在重现时执行。Center=true 和 -override 的初始定位提示不受本次实测差异影响。
// 副作用：仅创建、激活、移动和关闭自身窗口，不更改系统设置或剪贴板。
// 复位：重新启动。退出：Esc、Quit 或关闭窗口；失败非零退出，自动模式自行退出。
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"runtime"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/ui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run() error {
	auto := flag.Bool("auto", false, "check placement, rebuild and redisplay, then exit")
	center := flag.Bool("center", true, "request initial work-area centering")
	override := flag.Bool("override", false, "explicitly position the hidden window after initial centering")
	skip := flag.Bool("skip-taskbar", false, "omit a separate taskbar entry")
	chrome := flag.String("chrome", "native", "native, integrated or none")
	flag.Parse()
	mode, ok := map[string]ui.WindowChromeMode{
		"native": ui.WindowChromeNative, "integrated": ui.WindowChromeIntegrated, "none": ui.WindowChromeNone,
	}[*chrome]
	if !ok {
		return fmt.Errorf("unknown chrome %q", *chrome)
	}
	size := geometry.Size{Width: 360, Height: 300}
	var result error
	var original gui.Window
	initialized, rebuilds, phase := false, 0, 0
	err := ui.Run("org.golang-gui.InitialCenter", func(app ui.App) ui.RootView {
		rebuilds++
		check := func(name string, err error) bool {
			if err != nil {
				result = fmt.Errorf("%s: %w", name, err)
				app.Quit()
				return false
			}
			fmt.Println("PASS", name)
			return true
		}
		preserve := func(redisplay bool) {
			win := app.FindWindow("main")
			before, err := win.Position(nil)
			if !check("observe before operation", err) {
				return
			}
			if redisplay {
				if !check("hide", win.PlatformWindow().Hide()) || !check("show", win.Show()) {
					return
				}
			} else {
				app.RequestUpdate()
			}
			app.TimeoutFunc(400*time.Millisecond, func() {
				current := app.FindWindow("main")
				if current != win {
					check("preserved window identity", fmt.Errorf("window replaced"))
					return
				}
				check("preserved position", checkPosition(current, before))
			})
		}
		move := func() {
			check("move to (100,60)", app.FindWindow("main").SetPosition(nil, geometry.Point{X: 100, Y: 60}))
		}
		body := []ui.View{
			ui.HeaderBar(ui.Label("Initial window centering")),
			ui.Label(fmt.Sprintf("Center=%t / rebuild=%d", *center, rebuilds)),
			ui.Button("F5 Move to (100,60)").OnClick(move),
			ui.Button("F6 Rebuild").OnClick(func() { preserve(false) }),
			ui.Button("F7 Hide / Show").OnClick(func() { preserve(true) }),
			ui.Button("Esc Quit").OnClick(app.Quit),
		}
		probe := &inspectionView{children: body}
		probe.Self = probe
		probe.inspect = func() {
			if initialized {
				return
			}
			initialized = true
			original = app.FindWindow("main")
			if original.State() != gui.WindowStateHidden {
				check("hidden creation", fmt.Errorf("state=%v", original.State()))
				return
			}
			if *center && !check("center before first Show", checkCenter(original, size)) {
				return
			}
			if *override && !check("explicit pre-Show override", original.SetPosition(nil, geometry.Point{X: 120, Y: 80})) {
				return
			}
			fmt.Printf("OS=%s painter=%q scale=%q chrome=%s center=%t override=%t skip-taskbar=%t\n",
				runtime.GOOS, os.Getenv("GOUI_PLAT_PAINTER"), os.Getenv("GOUI_PLAT_SCALE"), *chrome, *center, *override, *skip)
			if !*auto {
				return
			}
			var timer *ui.Timer
			timer = app.TickFunc(400*time.Millisecond, func() {
				win := app.FindWindow("main")
				if win != original {
					check("window identity", fmt.Errorf("missing or replaced window"))
					return
				}
				switch phase {
				case 0:
					bounds := win.Snapshot().Bounds
					if math.Abs(float64(bounds.Width-size.Width)) > .5 || math.Abs(float64(bounds.Height-size.Height)) > .5 {
						check("actual client size", fmt.Errorf("bounds=%v want size=%v", bounds, size))
						return
					}
					if *override {
						if !check("override after Show", checkPosition(win, geometry.Point{X: 120, Y: 80})) {
							return
						}
					} else if *center && !check("actual client center after Show", checkCenter(win, bounds.Size)) {
						return
					}
					if !check("explicit move", win.SetPosition(nil, geometry.Point{X: 100, Y: 60})) {
						return
					}
				case 1:
					if !check("observed explicit move", checkPosition(win, geometry.Point{X: 100, Y: 60})) {
						return
					}
					app.RequestUpdate()
				case 2:
					if !check("rebuild preserves position", checkPosition(win, geometry.Point{X: 100, Y: 60})) {
						return
					}
					if !check("hide", win.PlatformWindow().Hide()) || !check("show again", win.Show()) {
						return
					}
				case 3:
					if !check("Hide/Show preserves position", checkPosition(win, geometry.Point{X: 100, Y: 60})) {
						return
					}
					timer.Stop()
					app.Quit()
				}
				phase++
			})
		}
		return ui.Window("main").Title("GOUI initial center").Size(size.Width, size.Height).
			Chrome(mode).Center(*center).SkipTaskbar(*skip).Content(probe).
			Shortcuts(ui.Shortcut(ui.KeyF5).OnActivate(move),
				ui.Shortcut(ui.KeyF6).OnActivate(func() { preserve(false) }),
				ui.Shortcut(ui.KeyF7).OnActivate(func() { preserve(true) }),
				ui.Shortcut(ui.KeyEscape).OnActivate(app.Quit)).OnDestroy(app.Quit)
	})
	if err != nil {
		return err
	}
	return result
}

type inspectionView struct {
	ui.ViewBase[inspectionView]
	children []ui.View
	inspect  func()
}

func (v *inspectionView) Build() ui.View { return v }
func (v *inspectionView) Mount(ui.BuildContext) gui.Widget {
	box := gui.NewLinearBox(layout.DirectionVertical)
	box.SetSpacing(6)
	return box
}
func (v *inspectionView) Update(ctx ui.BuildContext, widget gui.Widget) {
	ctx.UpdateChildren(widget.(ui.Container), v.children)
	ctx.AfterUpdate(v.inspect)
}
func (*inspectionView) Unmount(ui.BuildContext, gui.Widget) {}

func checkCenter(win gui.Window, size geometry.Size) error {
	area, err := win.WorkAreaAt(geometry.Point{})
	if err != nil {
		return err
	}
	// WorkAreaAt is an observation independent of Position or a stored request:
	// in client-local coordinates, opposite margins must have equal lengths.
	left, top := -area.X, -area.Y
	right, bottom := area.X+area.Width-size.Width, area.Y+area.Height-size.Height
	if math.Abs(float64(left-right)) > 1 || math.Abs(float64(top-bottom)) > 1 {
		return fmt.Errorf("unequal work-area margins: left/top=(%g,%g) right/bottom=(%g,%g)", left, top, right, bottom)
	}
	return checkPosition(win, geometry.Point{X: (area.Width - size.Width) / 2, Y: (area.Height - size.Height) / 2})
}

func checkPosition(win gui.Window, want geometry.Point) error {
	got, err := win.Position(nil)
	if err != nil {
		return err
	}
	if math.Abs(float64(got.X-want.X)) > .5 || math.Abs(float64(got.Y-want.Y)) > .5 {
		return fmt.Errorf("position=%v want=%v (tolerance 0.5 DIP)", got, want)
	}
	return nil
}
