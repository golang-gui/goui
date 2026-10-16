// 原生剪贴板结果回归，必须显式运行，不由 go test 启动。
//
// 环境：Linux/X11、Windows、macOS。main 在平台要求的启动线程执行。
// 本程序没有可见窗口，使用真实平台与事件循环；排版和渲染缩放不适用。
// 启动：go run ./tests/window/platform/clipboard -write
// 初始：-write 明确允许用可丢弃文本覆盖剪贴板。仅在隔离 X server、Windows
// Session 0 或专用测试桌面执行；macOS 使用外部快照/恢复全部格式的包装程序。
// 操作：无需输入。-write 自动核对空文本、Unicode、多行连续覆盖和恰好一次回调。
// 跨进程读取：-expect '预先复制的文本'；-expect ” 核对正常无文本；
// -error timeout/unavailable/failure
// 分别核对五秒期限、退出取消、其他转换失败。-quit-after 100ms 测退出取消。
// 预期：仅全部断言满足才打印 PASS 并以 0 退出；8 秒外部看门狗不能算读取期限通过。
// Linux 的 owner 子程序用于构造外部 selection，包括无应答、格式错误和 Latin-1。
// 副作用：-write 改写真实剪贴板；X11 退出失去所有权，不能靠退出前写回原文本恢复。
// Windows/macOS 系统保留最后一条文本。请使用上述隔离或完整恢复措施。
// 复位：重启程序/隔离会话。正常自动退出；Ctrl-C 中断不算通过。
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/common"
)

func main() {
	write := flag.Bool("write", false, "overwrite clipboard with disposable text and verify round trips")
	expect := flag.String("expect", "", "exact expected external text")
	expectError := flag.String("error", "", "timeout, unavailable or failure")
	quitAfter := flag.Duration("quit-after", 0, "quit while a read is pending")
	flag.Parse()
	if err := run(*write, *expect, *expectError, *quitAfter); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	fmt.Println("PASS clipboard")
}

func run(write bool, expect, expectError string, quitAfter time.Duration) error {
	p, err := platform.NewPlatform(platform.DefaultName(), "org.golang-gui.ClipboardRegression")
	if err != nil {
		return err
	}
	defer p.Destroy()
	loop, err := p.NewEventLoop()
	if err != nil {
		return err
	}
	defer loop.Destroy()
	c, err := p.NewClipboard()
	if err != nil {
		return err
	}
	fmt.Printf("platform=%s scale=N/A renderer=N/A\n", p.Name())
	if write {
		for _, text := range []string{"", "你好 🧧 é", "one\ntwo\tend"} {
			if err := c.SetText(text); err != nil {
				return fmt.Errorf("write: %w", err)
			}
			calls := 0
			var readError error
			c.RequestText(func(got string, err error) {
				calls++
				if err != nil || got != text {
					readError = fmt.Errorf("round trip %q: got %q, %v", text, got, err)
				}
			})
			if calls != 1 {
				return fmt.Errorf("round trip callbacks: %d", calls)
			}
			if readError != nil {
				return readError
			}
		}
		return nil
	}
	calls := 0
	var got string
	var result error
	start := time.Now()
	loop.Post(func() {
		c.RequestText(func(text string, err error) { calls++; got, result = text, err; loop.Quit() })
	})
	watchdog := time.AfterFunc(8*time.Second, loop.Quit)
	defer watchdog.Stop()
	if quitAfter > 0 {
		timer := time.AfterFunc(quitAfter, loop.Quit)
		defer timer.Stop()
	}
	loop.Run()
	if calls != 1 {
		return fmt.Errorf("read callbacks: %d", calls)
	}
	switch expectError {
	case "":
		if result != nil || got != expect {
			return fmt.Errorf("read got %q, %v; want %q", got, result, expect)
		}
	case "timeout":
		if !errors.Is(result, common.ErrClipboardTimeout) || time.Since(start) < 5*time.Second || time.Since(start) >= 7*time.Second {
			return fmt.Errorf("want five-second read timeout: %v, elapsed %v", result, time.Since(start))
		}
	case "unavailable":
		if !errors.Is(result, common.ErrUnavailable) {
			return fmt.Errorf("want loop-exit cancellation: %v", result)
		}
	case "failure":
		if result == nil || errors.Is(result, common.ErrClipboardTimeout) || errors.Is(result, common.ErrUnavailable) {
			return fmt.Errorf("want conversion failure: %v", result)
		}
	default:
		return fmt.Errorf("unknown error expectation: %s", expectError)
	}
	return nil
}
