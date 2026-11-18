// 原生按键、指针、滚轮和焦点输入验证。
//
// 环境：Linux/X11、Windows 或 macOS 可交互桌面；不使用 Painter，缩放由窗口事件记录。
// 启动：go run ./tests/window/platform/input -kind=pointer -count=6 -timeout=15s
// 初始：显示一个空白窗口，控制台显示目标事件类型、数量和超时。
// 操作与预期：
//  1. -kind=pointer：移入客户区，按下/移动/释放，再移出；日志逐事件包含坐标、
//     当前按钮、按住按钮集合和 modifiers，累计至少六个事件后自动退出。
//  2. -kind=key -count=4：窗口激活后按下并释放两枚按键；日志包含 KeyDown/Up、
//     key、原生 code、左右位置、重复状态和 modifiers。完成四个事件后自动退出。
//  3. -kind=wheel -count=1：在客户区滚动；日志包含坐标、双轴增量、单位、按钮和 modifiers。
//  4. -kind=focus -count=3：点击本窗口/其他窗口/本窗口；获取/失去/获取各计一次，
//     原生重复通知仍记录，但相同焦点状态不重复计数。
//
// 以上是独立运行场景；达到指定数量才 PASS/0，超时或提前关闭非零退出。
// 数量断言不能替代对日志字段与实际输入的逐项核对，不预设平台的滚轮精度或重复策略。
// 副作用：创建并激活窗口、记录按键坐标；勿输入敏感内容。复位：重启。
// 退出：达到数量自动关闭，手工关闭/超时算失败；Ctrl-C 中断不算通过。
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
)

func main() {
	kind := flag.String("kind", "pointer", "pointer, key, wheel or focus")
	count := flag.Int("count", 6, "所需事件数量")
	timeout := flag.Duration("timeout", 15*time.Second, "等待期限")
	flag.Parse()
	if err := run(*kind, *count, *timeout); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	fmt.Println("PASS input", *kind)
}

func run(kind string, count int, timeout time.Duration) error {
	switch kind {
	case "pointer", "key", "wheel", "focus":
	default:
		return fmt.Errorf("unknown kind %q", kind)
	}
	if count <= 0 || timeout <= 0 {
		return fmt.Errorf("count and timeout must be positive")
	}
	plat, err := platform.NewPlatform(platform.DefaultName(), "org.golang-gui.InputRegression")
	if err != nil {
		return err
	}
	defer plat.Destroy()
	loop, err := plat.NewEventLoop()
	if err != nil {
		return err
	}
	defer loop.Destroy()
	seen := 0
	haveFocus, lastFocus := false, false
	window, err := plat.NewWindow(geometry.Size{Width: 800, Height: 600}, func(event platform.Event) {
		matched := false
		switch e := event.(type) {
		case events.PointerEvent:
			if kind == "pointer" {
				matched = true
				fmt.Printf("pointer type=%s pos=%v button=%s buttons=%s modifiers=%s\n", pointerEventTypeName(e.Type()), e.Position, pointerButtonName(e.Button), pointerButtonsName(e.Buttons), modifiersName(e.Modifiers))
			}
		case events.KeyEvent:
			if kind == "key" {
				matched = true
				fmt.Printf("key type=%s key=%s code=%v location=%s repeat=%t modifiers=%s\n", keyEventTypeName(e.Type()), keyName(e.Key), e.Code, keyLocationName(e.Location), e.Repeat, modifiersName(e.Modifiers))
			}
		case events.WheelEvent:
			if kind == "wheel" {
				matched = true
				fmt.Printf("wheel pos=%v delta=%g,%g mode=%s buttons=%s modifiers=%s\n", e.Position, e.DeltaX, e.DeltaY, wheelDeltaModeName(e.Mode), pointerButtonsName(e.Buttons), modifiersName(e.Modifiers))
			}
		case events.FocusEvent:
			if kind == "focus" {
				matched = !haveFocus || lastFocus != e.Focused
				haveFocus, lastFocus = true, e.Focused
				fmt.Printf("focus=%t\n", e.Focused)
			}
		case events.SizeEvent:
			fmt.Printf("size=%+v\n", e)
		case events.CloseEvent:
			loop.Quit()
		}
		if matched {
			seen++
			if seen >= count {
				loop.Quit()
			}
		}
	}, platform.WindowOptions{})
	if err != nil {
		return err
	}
	defer window.Destroy()
	desktop := window.(platform.WindowManagement)
	if err := desktop.SetTitle("GOUI input: " + kind); err != nil {
		return err
	}
	fmt.Printf("READY kind=%s count=%d timeout=%s\n", kind, count, timeout)
	watchdog := time.AfterFunc(timeout, loop.Quit)
	defer watchdog.Stop()
	// Show 可能同步派发 FocusEvent；loop 首任务再次检查数量，不丢失已完成的场景。
	if err := desktop.Show(); err != nil {
		return err
	}
	loop.Post(func() {
		if seen >= count {
			loop.Quit()
		}
	})
	loop.Run()
	if seen < count {
		return fmt.Errorf("window closed or timed out: received %d/%d %s events", seen, count, kind)
	}
	return nil
}
