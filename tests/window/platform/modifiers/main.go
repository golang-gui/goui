// 左右修饰键原生事件回归。
//
// 环境：Windows/macOS/Linux 桌面与键盘；Linux 记录 WM、布局及 XIM。
// 不创建 Painter，不依赖字体、渲染后端或缩放。使用窗口标题和终端显示状态。
// 启动：go run ./tests/window/platform/modifiers -modifier control
// modifier 可选 shift/control/alt/system；macOS alt=Option、system=Command，
// Windows system=Win，Linux system=Super。加 -reverse 交换以下左右顺序。
// 初始：未开始，按 F8 开始普通场景；按 Escape 复位。不要启用粘滞键。
//
// 操作与预期：
//  1. 所有修饰键松开，聚焦本窗口，按 F8；按左侧、按右侧、放左侧、按 S、
//     放右侧、按 F8。程序逐项断言 Down/Down/Up、侧别和修饰位；中途 S
//     必须仍带对应修饰位，最后抬起和 F8 必须没有它。最终打印 PASS。
//  2. 焦点切到另一个窗口，按住左右两侧，通过鼠标切回本窗口，按 S 开始；
//     放左侧、按 S、放右侧、按 F8。预期同上；不能依赖本窗口收到原始按下。
//  3. 中途切走焦点并在外部松开按键，回来后按 F8 重做步骤 1，不应残留修饰位。
//  4. 分别运行 shift/control/alt/system 和 -reverse。Linux 若右 Alt 是 AltGr，
//     它不是普通 Alt，此场景不适用；不能修改布局后假称原布局通过。
//
// 系统保留组合（如 Win/Super+S 等）可能被 WM 截获，不到达应用时记为
// 未覆盖，不当作通过。macOS 可需 Fn+F8 发送真正功能键。
// 副作用：只创建本窗口；不修改系统设置/剪贴板。程序不请求 IME 文本输入。
// 退出：关闭窗口；任何断言失败记录 FAIL，退出码 1，否则退出码 0。
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
)

func main() {
	runtime.LockOSThread()
	name := flag.String("modifier", "control", "shift/control/alt/system")
	reverse := flag.Bool("reverse", false, "start with the right key")
	flag.Parse()
	if err := run(*name, *reverse); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(name string, reverse bool) error {
	key, mods := events.KeyControl, events.ModifierControl
	switch name {
	case "control":
	case "shift":
		key, mods = events.KeyShift, events.ModifierShift
	case "alt":
		key, mods = events.KeyAlt, events.ModifierAlt
		if runtime.GOOS == "darwin" {
			key, mods = events.KeyOption, events.ModifierOption
		}
	case "system":
		switch runtime.GOOS {
		case "windows":
			key, mods = events.KeyWin, events.ModifierWin
		case "darwin":
			key, mods = events.KeyCommand, events.ModifierCommand
		default:
			key, mods = events.KeySuper, events.ModifierSuper
		}
	default:
		return fmt.Errorf("unknown modifier %q", name)
	}
	p, err := platform.NewPlatform(platform.DefaultName(), "org.golang-gui.ModifierProbe")
	if err != nil {
		return err
	}
	defer p.Destroy()
	loop, err := p.NewEventLoop()
	if err != nil {
		return err
	}
	defer loop.Destroy()
	first, other := events.KeyLocationLeft, events.KeyLocationRight
	if reverse {
		first, other = other, first
	}
	type expected struct {
		typ      events.EventType
		key      events.Key
		location events.KeyLocation
		mods     events.Modifiers
	}
	sequence := []expected{
		{events.KeyDown, key, first, mods},
		{events.KeyDown, key, other, mods},
		{events.KeyUp, key, first, mods},
		{events.KeyDown, events.KeyS, events.KeyLocationStandard, mods},
		{events.KeyUp, key, other, 0},
		{events.KeyDown, events.KeyF8, events.KeyLocationStandard, 0},
	}
	step := -1
	var failure error
	var w platform.Window
	report := func(message string) {
		fmt.Println(message)
		if w != nil {
			_ = w.SetTitle("GOUI modifiers / " + name + " / " + message)
		}
	}
	fail := func(message string) {
		failure = fmt.Errorf("FAIL: %s", message)
		step = -1
		report(failure.Error())
	}
	w, err = p.NewWindow(geometry.Size{Width: 650, Height: 180}, func(event events.Event) {
		switch e := event.(type) {
		case events.CloseEvent:
			loop.Quit()
		case events.FocusEvent:
			if !e.Focused {
				step = -1
				report("focus lost; F8 fresh / S held")
			}
		case events.KeyEvent:
			fmt.Printf("event type=%v key=%v location=%v mods=%d repeat=%v\n", e.EventType, e.Key, e.Location, e.Modifiers, e.Repeat)
			if e.Key == events.KeyEscape && e.EventType == events.KeyDown {
				step = -1
				report("reset; F8 fresh / S held")
				return
			}
			if step < 0 {
				if e.EventType != events.KeyDown || e.Repeat {
					return
				}
				switch e.Key {
				case events.KeyF8:
					if e.Modifiers != 0 {
						fail(fmt.Sprintf("start has modifiers=%d", e.Modifiers))
						return
					}
					step = 0
				case events.KeyS:
					if e.Modifiers != mods {
						fail(fmt.Sprintf("held start modifiers=%d want=%d", e.Modifiers, mods))
						return
					}
					step = 2
				default:
					return
				}
				report(fmt.Sprintf("armed step=%d reverse=%v", step, reverse))
				return
			}
			if e.Key != key && !((e.Key == events.KeyF8 || e.Key == events.KeyS) && e.EventType == events.KeyDown) {
				return
			}
			want := sequence[step]
			if e.EventType != want.typ || e.Key != want.key || e.Location != want.location || e.Modifiers != want.mods || e.Repeat {
				fail(fmt.Sprintf("step %d got %v/%v/%v/%d repeat=%v; want %+v", step, e.EventType, e.Key, e.Location, e.Modifiers, e.Repeat, want))
				return
			}
			step++
			if step == len(sequence) {
				step = -1
				report("PASS; F8 fresh / S held")
			} else {
				report(fmt.Sprintf("step %d OK", step))
			}
		}
	}, platform.WindowOptions{})
	if err != nil {
		return err
	}
	defer w.Destroy()
	report("F8 fresh / S held")
	if err = w.Show(); err != nil {
		return err
	}
	loop.Run()
	return failure
}
