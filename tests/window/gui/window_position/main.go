// 窗口客户区定位验收，不使用 DevServer。
//
// 环境：Windows/macOS 桌面，或支持 EWMH 和 RandR 1.5 的 Linux/X11 桌面。
// 启动：go run ./tests/window/gui/window_position [-auto] [-chrome=none|native|integrated]
// 分别用 GOUI_PLAT_SCALE=1、2 重跑，记录实际后端和窗口管理器。
// 初始：A/B 在 Show 前分别定位到工作区 (40,40)、(120,120) DIP；两窗口无父子关系。
// 初始高度 400 DIP，避免 Windows 字体测量得到的内容最小高度超过客户区，
// 系统随后自动扩高并把越界窗口移回工作区；定位断言不承担内容尺寸约束验收。
//
// 手动操作与逐项预期：
//  1. 点击 B relative to A：B 的客户区原点位于 A 的客户区 (200,100) DIP。
//     Native 装饰在该点之外，不能额外偏移一个标题栏高度。
//  2. 使用 -chrome=native，拖动 A 的原生标题栏，再重复步骤 1：使用新的客户区原点。
//     仅移动 A 不会让 B 跟随。
//  3. 点击 B work-area (120,120)：使用 B 当前 WorkAreaAt(0,0)，重复点击不累积偏移。
//     将 B 移到另一显示区域再重复：使用 B 的工作区，而不是 A 或指针所在区域。
//  4. 点击 B self (+10,+10)：B 客户区移动 (10,10) DIP，不能叠加原生边框偏移。
//  5. 点击 B relative (-20,40)：允许负偏移，不由 GOUI 限制；记录窗口管理器实际约束。
//  6. 点击 Move A only：B 不动，定位不会调整大小或请求激活。
//  7. 点击 Query positions：步骤 1 完成后 Position(A) 为 (200,100)，Position(self)
//     为 (0,0)，Native 同样如此。手动移动 B 后再查，观察值应改变而非返回旧请求。
//     Linux 可用 xwininfo 独立核对客户区原点。
//
// -auto 支持三种 Chrome，断言显示前观察、工作区/相对/自身定位、重复请求、不跟随、
// 非法输入和已销毁对象，并以 WorkAreaAt 的原生观察及独立预期核对 Position。
// 每次请求等待 400ms 后采样；失败非零退出，全部完成输出 PASS 后退出。
// 自动模式不覆盖多屏、混合 DPI、最大化或全屏；不支持必须报错，不能算自动通过。
// 平台差异：系统可能忽略或约束请求，AppKit 可限制窗口保持可达，需单独记录。
// 副作用：仅创建、移动自身两个窗口，不改系统设置。复位：重启。
// 退出：Quit 或关闭任一原生窗口；自动模式自行退出。
package main

import (
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"runtime"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
)

func main() {
	runtime.LockOSThread()
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	auto := flag.Bool("auto", false, "run observed-position assertions and exit")
	chrome := flag.String("chrome", "none", "none, native or integrated")
	flag.Parse()
	modes := map[string]gui.WindowChromeMode{"none": gui.WindowChromeNone, "native": gui.WindowChromeNative, "integrated": gui.WindowChromeIntegrated}
	mode, ok := modes[*chrome]
	if !ok {
		return fmt.Errorf("invalid chrome %q", *chrome)
	}
	app, err := gui.NewApplication("org.golang-gui.WindowPositionProbe")
	if err != nil {
		return err
	}
	a, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 350, Height: 400}, Chrome: mode})
	if err != nil {
		return err
	}
	defer a.Destroy()
	b, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 350, Height: 400}, Chrome: mode})
	if err != nil {
		return err
	}
	defer b.Destroy()
	a.SetTitle("GOUI position A")
	b.SetTitle("GOUI position B")
	a.ConnectDestroy(app.Quit)
	b.ConnectDestroy(app.Quit)
	steps := []struct {
		name string
		run  func() error
	}{
		{"B relative to A", func() error { return b.SetPosition(a, geometry.Point{X: 200, Y: 100}) }},
		{"B work-area (120,120)", func() error { return b.SetPosition(nil, geometry.Point{X: 120, Y: 120}) }},
		{"B self (+10,+10)", func() error { return b.SetPosition(b, geometry.Point{X: 10, Y: 10}) }},
		{"B relative (-20,40)", func() error { return b.SetPosition(a, geometry.Point{X: -20, Y: 40}) }},
		{"Move A only", func() error { return a.SetPosition(nil, geometry.Point{X: 80, Y: 80}) }},
		{"Query positions", func() error {
			for i, ref := range []gui.Window{nil, a, b} {
				p, err := b.Position(ref)
				fmt.Printf("B Position reference=%d (nil/A/self): %v err=%v\n", i, p, err)
				if err != nil {
					return err
				}
			}
			return nil
		}},
	}
	for _, win := range []gui.Window{a, b} {
		box := gui.NewLinearBox(layout.DirectionVertical)
		box.SetPadding(geometry.UniformInsets(12))
		box.SetSpacing(6)
		status := gui.NewLabel(win.Title())
		box.AddChild(status)
		for _, step := range steps {
			button := gui.NewButton()
			button.SetChild(gui.NewLabel(step.name))
			button.ConnectClicked(func() {
				err := step.run()
				status.SetText(fmt.Sprintf("%s: %v", step.name, err))
				fmt.Println(step.name, err)
			})
			box.AddChild(button)
		}
		quit := gui.NewButton()
		quit.SetChild(gui.NewLabel("Quit"))
		quit.ConnectClicked(app.Quit)
		box.AddChild(quit)
		win.SetWidget(box)
		if *auto {
			fmt.Printf("%s content minimum: %v\n", win.Title(), box.Measure(layout.Unbounded()).Size)
		}
	}
	if err := a.SetPosition(nil, geometry.Point{X: 40, Y: 40}); err != nil {
		return err
	}
	if err := b.SetPosition(nil, geometry.Point{X: 120, Y: 120}); err != nil {
		return err
	}
	if *auto {
		for _, tc := range []struct {
			w, ref gui.Window
			want   geometry.Point
		}{
			{a, nil, geometry.Point{X: 40, Y: 40}}, {b, nil, geometry.Point{X: 120, Y: 120}},
			{a, a, geometry.Point{}}, {b, b, geometry.Point{}},
			{b, a, geometry.Point{X: 80, Y: 80}}, {a, b, geometry.Point{X: -80, Y: -80}},
		} {
			got, err := tc.w.Position(tc.ref)
			if err != nil || math.Abs(float64(got.X-tc.want.X)) > 0.5 || math.Abs(float64(got.Y-tc.want.Y)) > 0.5 {
				return fmt.Errorf("pre-Show Position: got %v want %v err=%v", got, tc.want, err)
			}
		}
		fmt.Println("PASS pre-Show client positions (nil/self/other)")
	}
	if err := a.Show(); err != nil {
		return err
	}
	if err := b.Show(); err != nil {
		return err
	}
	fmt.Printf("OS=%s requested painter=%q scale=%q chrome=%s\n", runtime.GOOS, os.Getenv("GOUI_PLAT_PAINTER"), os.Getenv("GOUI_PLAT_SCALE"), *chrome)
	var result error
	if *auto {
		// All samples stay on one work area. Positions are negated work-area
		// origins, independent of the requested/stored coordinates.
		wantA, wantB := geometry.Point{X: 40, Y: 40}, geometry.Point{X: 120, Y: 120}
		index := 0
		timer := app.NewTimer()
		defer timer.Stop()
		timer.ConnectTimeout(func() {
			for i, w := range []gui.Window{a, b} {
				area, err := w.WorkAreaAt(geometry.Point{})
				want := []geometry.Point{wantA, wantB}[i]
				// 0.5 DIP allows at most one physical pixel at the documented
				// 1x/2x test scales, for native rounding and float conversions.
				if err != nil || math.Abs(float64(area.X+want.X)) > 0.5 || math.Abs(float64(area.Y+want.Y)) > 0.5 {
					result = fmt.Errorf("step %d window %d: area=%v want origin=%v err=%v", index, i, area, want, err)
					app.Quit()
					return
				}
			}
			// Keep the WorkAreaAt checks above as an independent observation;
			// don't replace them with a getter/setter round-trip alone.
			for _, tc := range []struct {
				w, ref gui.Window
				want   geometry.Point
			}{
				{a, nil, wantA}, {b, nil, wantB}, {a, a, geometry.Point{}}, {b, b, geometry.Point{}},
				{b, a, geometry.Point{X: wantB.X - wantA.X, Y: wantB.Y - wantA.Y}},
				{a, b, geometry.Point{X: wantA.X - wantB.X, Y: wantA.Y - wantB.Y}},
			} {
				got, err := tc.w.Position(tc.ref)
				if err != nil || math.Abs(float64(got.X-tc.want.X)) > 0.5 || math.Abs(float64(got.Y-tc.want.Y)) > 0.5 {
					result = fmt.Errorf("step %d Position: got %v want %v err=%v", index, got, tc.want, err)
					app.Quit()
					return
				}
			}
			fmt.Printf("PASS step %d A=%v B=%v (WorkAreaAt and Position)\n", index, wantA, wantB)
			switch index {
			case 0:
				result = steps[0].run()
				wantB = geometry.Point{X: 240, Y: 140}
			case 1:
				result = steps[2].run()
				wantB = geometry.Point{X: 250, Y: 150}
			case 2:
				result = steps[4].run()
				wantA = geometry.Point{X: 80, Y: 80}
			case 3, 4:
				result = steps[1].run()
				wantB = geometry.Point{X: 120, Y: 120}
			case 5:
				result = steps[3].run()
				wantB = geometry.Point{X: 60, Y: 120}
			default:
				if err := b.SetPosition(nil, geometry.Point{X: float32(math.NaN())}); err == nil {
					result = errors.New("accepted NaN")
				}
				// Destroying A quits the loop; its native resources are released
				// before the following positioning request.
				a.Destroy()
				if _, err := b.Position(a); err == nil {
					result = errors.New("Position accepted destroyed reference")
				}
				if _, err := a.Position(nil); err == nil {
					result = errors.New("Position accepted destroyed target")
				}
				if err := b.SetPosition(a, geometry.Point{}); err == nil {
					result = errors.New("accepted destroyed reference")
				}
				app.Quit()
				return
			}
			index++
			if result != nil {
				app.Quit()
			}
		})
		if err := timer.Start(400 * time.Millisecond); err != nil {
			return err
		}
	}
	app.Run()
	return result
}
