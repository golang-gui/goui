// 原生鼠标修饰键回归，不使用 DevServer 或合成 GOUI 事件。
//
// 环境：Windows 可交互桌面与鼠标，记录系统版本、会话、布局和缩放。
// 启动：go run ./tests/window/platform/pointer_modifiers -mods alt -wheel both
// mods 支持 none、shift、control、alt 及以 + 连接的组合；wheel 为 both/vertical/none。
// 初始：Native 720×420 DIP 空白客户区，标题显示参数，尚未开始。
// 不创建 Painter，绘制后端不适用；其他平台只编译，不作为 Windows 验收。
//
// 操作与逐项预期：
//  1. 聚焦窗口，按住指定修饰键，再按 F6 开始，在空白客户区移动、点击并释放鼠标。
//     预期每个 enter/move/down/up/leave 的修饰位等于 mods，无其他修饰位。
//  2. 保持修饰键，执行竖向及横向滚轮；预期两类 WheelEvent 的修饰位也等于 mods。
//     无横向设备时显式使用 -wheel vertical，不能将横向记为通过。
//  3. 不再移动鼠标，松开修饰键，按 F8。仅在移动、按下/抬起和所选滚轮均已观察
//     且全部断言通过时输出 PASS 并退出0；遗漏、修饰位错误或提前关闭退出1。
//  4. 分别运行 none、alt、shift+control+alt；Alt 分别使用左/右侧。右 Alt 若由布局
//     产生 Ctrl+Alt，按真实组合指定 mods，不将其归一化为普通 Alt。
//  5. 在另一窗口按住 Alt，再切入本窗口，按住 Alt 按 F6，重复移动/点击/滚轮。
//     松开后再以 -mods none 运行，预期不依赖本窗口收到 Alt 原始按下，也不残留 Alt。
//
// F6 重新开始并清零覆盖计数；键盘状态不由本程序修改。系统或远程客户端截获的
// 输入记为未覆盖，不能以派发 GOUI 事件代替。无文件/网络/剪贴板或系统配置改动；
// 日志包含初始化阶段、窗口尺寸、事件类型和修饰位，不记录输入正文。
// 复位：F6 或重启。退出：F8 或关闭窗口。
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
)

func main() {
	mods := flag.String("mods", "alt", "expected native modifiers: none or shift/control/alt joined by +")
	wheel := flag.String("wheel", "both", "required wheel coverage: both, vertical or none")
	flag.Parse()
	if err := run(*mods, *wheel); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(names, wheel string) error {
	var expected events.Modifiers
	for _, name := range strings.Split(names, "+") {
		switch name {
		case "none":
			if names != "none" {
				return errors.New("none cannot be combined")
			}
		case "shift":
			expected |= events.ModifierShift
		case "control":
			expected |= events.ModifierControl
		case "alt":
			expected |= events.ModifierAlt
		default:
			return fmt.Errorf("unknown modifier %q", name)
		}
	}
	required := []string{"move", "down", "up"}
	switch wheel {
	case "both":
		required = append(required, "wheel-x", "wheel-y")
	case "vertical":
		required = append(required, "wheel-y")
	case "none":
	default:
		return fmt.Errorf("unknown wheel coverage %q", wheel)
	}
	fmt.Printf("START modifiers=%s wheel=%s\n", names, wheel)
	p, err := platform.NewPlatform(platform.DefaultName(), "org.golanggui.PointerModifiers")
	if err != nil {
		return err
	}
	defer p.Destroy()
	fmt.Printf("PLATFORM %s\n", p.Name())
	loop, err := p.NewEventLoop()
	if err != nil {
		return err
	}
	defer loop.Destroy()
	fmt.Println("LOOP ready")
	armed, finished := false, false
	seen := make(map[string]int)
	var failure error
	var management platform.WindowManagement
	check := func(kind string, mods events.Modifiers) {
		if !armed || failure != nil {
			return
		}
		fmt.Printf("EVENT kind=%s modifiers=%d expected=%d\n", kind, mods, expected)
		if mods != expected {
			failure = fmt.Errorf("FAIL %s modifiers=%d expected=%d", kind, mods, expected)
			loop.Quit()
			return
		}
		seen[kind]++
	}
	w, err := p.NewWindow(geometry.Size{Width: 720, Height: 420}, func(event events.Event) {
		switch e := event.(type) {
		case events.SizeEvent:
			fmt.Printf("SIZE dip=%gx%g pixels=%gx%g\n", e.Width, e.Height, e.PixelWidth, e.PixelHeight)
		case events.CloseEvent:
			loop.Quit()
		case events.KeyEvent:
			if e.EventType != events.KeyDown || e.Repeat {
				return
			}
			switch e.Key {
			case events.KeyF6:
				e.PreventDefault()
				armed = true
				clear(seen)
				fmt.Printf("ARM expected=%d wheel=%s keyModifiers=%d\n", expected, wheel, e.Modifiers)
				if management != nil {
					_ = management.SetTitle("GOUI pointer modifiers / " + names + " / armed; F8 finish")
				}
			case events.KeyF8:
				e.PreventDefault()
				for _, kind := range required {
					if seen[kind] == 0 {
						failure = fmt.Errorf("FAIL missing %s; observed=%v", kind, seen)
						break
					}
				}
				finished = armed && failure == nil
				loop.Quit()
			}
		case events.PointerEvent:
			kinds := map[events.EventType]string{events.PointerEnter: "enter", events.PointerLeave: "leave", events.PointerMove: "move", events.PointerDown: "down", events.PointerUp: "up"}
			if kind := kinds[e.EventType]; kind != "" {
				check(kind, e.Modifiers)
			}
		case events.WheelEvent:
			if e.DeltaX != 0 {
				check("wheel-x", e.Modifiers)
			}
			if e.DeltaY != 0 {
				check("wheel-y", e.Modifiers)
			}
		}
	}, platform.WindowOptions{Chrome: platform.WindowChromeNative})
	if err != nil {
		return err
	}
	defer w.Destroy()
	fmt.Printf("WINDOW handle=%#x\n", w.NativeHandle())
	management = w.(platform.WindowManagement)
	if err := management.SetTitle("GOUI pointer modifiers / " + names + " / F6 start; F8 finish"); err != nil {
		return err
	}
	if err := management.SetPosition(nil, geometry.Point{X: 100, Y: 100}); err != nil {
		return err
	}
	if err := management.Show(); err != nil {
		return err
	}
	fmt.Println("SHOWN F6 start; F8 finish")
	loop.Run()
	if failure != nil {
		return failure
	}
	if !finished {
		return errors.New("FAIL closed before verification completed")
	}
	fmt.Printf("PASS expected=%d wheel=%s observed=%v\n", expected, wheel, seen)
	return nil
}
