// UI Timer 原生事件循环验收，不使用 DevServer。
//
// 环境：Windows/macOS，或具备 DISPLAY 的 Linux/X11 独立桌面会话。
// 启动：go run ./tests/window/ui/timer -scenario=runtime
// 再以新进程运行：go run ./tests/window/ui/timer -scenario=initial-failure
// 可加 GOMAXPROCS=1 检查单 P 下原生等待和 GUI 投递；不要求关闭异步抢占。
// 初始：不创建可见窗口、不使用 Painter；仍需真实平台事件循环，缩放不适用。
//
// 自动步骤与预期：
//  1. runtime：单次通知在启动线程执行，回调时 inactive；通知本身不重建 UI。
//  2. 重复 Stop、再次 StartOnce、回调中重用均有效；非法时长 panic 且保留原任务。
//  3. 周期通知在回调时 active，第三次自行 Stop；切回单次模式保留回调且不重复通知。
//  4. 修改未被 View 消费的 State：值增加一次，全应用构建次数不变。
//  5. 另一次通知显式 RequestUpdate：等待构建完成，确认已完成的 Timer 没有重启。
//  6. 活动任务存在时退出：旧句柄不可重启，新工厂返回 inactive 句柄，无退出后通知。
//  7. initial-failure：初次声明因空窗口 ID 失败；UI Timer 和共享调度器上的 GUI
//     Timer 均变为 inactive，后者不能重新启动，防止 UI 包装层掩盖底层任务泄漏。
//
// 两个场景分别输出 PASS；断言失败退出1，15秒超时退出2。无需手动输入。
// 不覆盖可见窗口、原生拖动/模态循环或帧动画；纯参数和状态依赖逻辑仍在包内测试。
// 副作用：仅本进程的平台连接和定时任务，不改设置、剪贴板或文件。复位：新进程重跑。
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/ui"
	"github.com/xuges/gothread"
)

func main() {
	scenario := flag.String("scenario", "runtime", "runtime or initial-failure")
	flag.Parse()
	watchdog := time.AfterFunc(15*time.Second, func() {
		fmt.Fprintln(os.Stderr, "FAIL：UI Timer 验收超过15秒")
		os.Exit(2)
	})
	defer watchdog.Stop()
	var err error
	switch *scenario {
	case "runtime":
		err = runUITimerNative()
	case "initial-failure":
		err = runUITimerInitialFailure()
	default:
		err = fmt.Errorf("未知场景 %q", *scenario)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	fmt.Printf("PASS UI Timer %s OS=%s GOMAXPROCS=%d\n", *scenario, runtime.GOOS, runtime.GOMAXPROCS(0))
}

func runUITimerInitialFailure() error {
	var timer *ui.Timer
	var guiTimer *gui.Timer
	err := ui.Run("", func(a ui.App) ui.RootView {
		timer = a.TickFunc(time.Hour, func() { panic("意外的 UI Timer 通知") })
		if !timer.Active() {
			panic("UI Timer 在 Run 前未激活")
		}
		// 公共 GUI 句柄直接观察同一调度器，不依赖 UI Timer 的停止保护。
		guiTimer = gui.App.NewTimer()
		if err := guiTimer.Start(time.Hour); err != nil || !guiTimer.Active() {
			panic(fmt.Sprintf("GUI Timer 在 Run 前未激活：%v", err))
		}
		return ui.Window("")
	})
	if !errors.Is(err, ui.ErrWindowIDEmpty) {
		return fmt.Errorf("初次声明错误：%v，期望 ErrWindowIDEmpty", err)
	}
	if timer == nil || guiTimer == nil || timer.Active() || guiTimer.Active() {
		return errors.New("初次声明失败后仍保留活动定时任务")
	}
	if err := guiTimer.StartOnce(time.Hour); err == nil {
		return errors.New("初次声明失败后底层调度器仍接受任务")
	}
	return nil
}

func runUITimerNative() error {
	thread := gothread.GetId()
	builds := 0
	value := ui.MakeState(0)
	rebuilt := make(chan int, 8)
	result := make(chan error, 1)
	err := ui.Run("", func(a ui.App) ui.RootView {
		builds++
		if builds == 1 {
			go func() {
				defer a.Quit()
				result <- exerciseUITimers(a, thread, &value, func() int { return builds }, rebuilt)
			}()
		} else {
			rebuilt <- builds
		}
		return ui.Root()
	})
	if err != nil {
		return err
	}
	return <-result
}

type timerNotification struct {
	thread int
	active bool
	count  int
}

func exerciseUITimers(a ui.App, thread int, state *ui.State[int], builds func() int, rebuilt <-chan int) error {
	notifications := make(chan timerNotification, 16)
	check := func(count int, active bool) error {
		select {
		case got := <-notifications:
			if got != (timerNotification{thread, active, count}) {
				return fmt.Errorf("通知：%+v，期望 thread=%d active=%v count=%d", got, thread, active, count)
			}
		case <-time.After(2 * time.Second):
			return fmt.Errorf("未收到第%d次通知", count)
		}
		return nil
	}
	var once *ui.Timer
	onceCalls := 0 // 仅 GUI 线程访问。
	// 先使用长时限，让工厂返回后再启动会读取句柄的回调。
	once = a.TimeoutFunc(time.Hour, func() {
		onceCalls++
		notifications <- timerNotification{gothread.GetId(), once.Active(), onceCalls}
		if onceCalls == 2 {
			once.StartOnce(time.Millisecond)
		}
	})
	defer once.Stop()
	if once == nil || !once.Active() {
		return errors.New("TimeoutFunc 未返回活动定时器")
	}
	if panicValue(func() { once.StartOnce(0) }) != "ui: timer duration must be positive" || !once.Active() {
		return errors.New("非法重启未保留原任务")
	}
	before := 0
	a.Sync(func() { before = builds() })
	once.StartOnce(time.Millisecond)
	if err := check(1, false); err != nil {
		return err
	}
	after := 0
	a.Sync(func() { after = builds() })
	if after != before || once.Active() {
		return errors.New("普通通知重建了 UI，或单次任务仍为活动状态")
	}
	once.Stop()
	once.Stop()
	once.StartOnce(time.Millisecond)
	for _, count := range []int{2, 3} {
		if err := check(count, false); err != nil {
			return err
		}
	}

	var periodic *ui.Timer
	periodicCalls := 0
	periodic = a.TickFunc(time.Hour, func() {
		periodicCalls++
		active := periodic.Active()
		if periodicCalls == 3 {
			periodic.Stop()
		}
		notifications <- timerNotification{gothread.GetId(), active, periodicCalls}
	})
	defer periodic.Stop()
	periodic.Start(time.Millisecond)
	for count := 1; count <= 3; count++ {
		if err := check(count, true); err != nil {
			return err
		}
	}
	if periodic.Active() {
		return errors.New("回调内 Stop 未取消周期任务")
	}
	periodic.StartOnce(time.Millisecond)
	if err := check(4, false); err != nil {
		return err
	}

	updater := a.TimeoutFunc(time.Hour, func() {
		state.Update(func(value int) int { return value + 1 })
		notifications <- timerNotification{gothread.GetId(), false, 5}
	})
	defer updater.Stop()
	updater.StartOnce(time.Millisecond)
	if err := check(5, false); err != nil {
		return err
	}
	a.Sync(func() { after = builds() })
	if state.Peek() != 1 || after != before || once.Active() || periodic.Active() {
		return errors.New("未消费 State 更新应只改变值，不应重建 UI 或重启已完成的任务")
	}
	// 显式更新由另一轮通知请求；等待构建通知，而不是假设 Sync 会排空更新队列。
	explicit := a.TimeoutFunc(time.Hour, func() {
		a.RequestUpdate()
		notifications <- timerNotification{gothread.GetId(), false, 6}
	})
	defer explicit.Stop()
	explicit.StartOnce(time.Millisecond)
	if err := check(6, false); err != nil {
		return err
	}
	select {
	case count := <-rebuilt:
		if count != before+1 {
			return fmt.Errorf("显式重建次数：%d，期望%d", count, before+1)
		}
	case <-time.After(2 * time.Second):
		return errors.New("RequestUpdate 未触发全应用重建")
	}
	a.Sync(func() { after = builds() })
	if after != before+1 || once.Active() || periodic.Active() || updater.Active() || explicit.Active() {
		return errors.New("显式重建未完成，或重启了已完成的任务")
	}

	once.Start(time.Hour)
	periodic.StartOnce(time.Hour)
	a.Quit()
	once.StartOnce(time.Millisecond)
	periodic.Start(time.Millisecond)
	inert := a.TimeoutFunc(time.Millisecond, func() { panic("退出后收到通知") })
	if inert == nil || inert.Active() || once.Active() || periodic.Active() {
		return errors.New("退出后仍允许活动 UI Timer")
	}
	return nil
}

func panicValue(fn func()) (value any) {
	defer func() { value = recover() }()
	fn()
	return
}
