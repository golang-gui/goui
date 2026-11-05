//go:build android || ios

// 移动端入口与事件线程验收，必须由 goui-base build -entry 注入 main 注册。
// 环境：Android ARM64 NativeActivity 或 iOS ARM64 UIKit Scene；原生宿主只为
// 该测试制作，不是 GOUI 窗口后端。宿主代码和操作说明见 native/ 与 README.md。
// 初始：原生页面保持响应；Go main 在独立固定线程等待验证消息。点击页面、切到
// 后台再返回，预期 Go 计数递增、mainCalls 始终为 1、threadFailures 始终为 0。
// 原生生命周期重新调用 goui_run，不能重复启动。退出/复位由模拟器停止和清除
// 测试应用完成。测试状态写在应用私有目录，不能据此认定 GOUI 控件已适配。
package main

import (
	"runtime"

	ffi "github.com/goexlib/cgo"
)

func symbol(name string) uintptr {
	p, err := ffi.Dlsym(ffi.RTLD_DEFAULT, name)
	if err != nil {
		panic(err)
	}
	return p
}

func main() {
	// 本例仅验证最小启动 ABI，不依赖正式窗口宿主或内部事件队列。
	// 容量覆盖规定操作；溢出即失败，不能阻塞原生 UI 回调。
	codes := make(chan int64, 64)
	publish, record := symbol("probe_publish"), symbol("probe_record")
	threadName := "gettid"
	if runtime.GOOS == "ios" {
		threadName = "pthread_self"
	}
	thread, isUI := symbol(threadName), symbol("probe_is_ui")
	threadID := func() uint64 { return ffi.CallRet[uint64](thread) }
	if ffi.CallRet[int32](isUI) != 0 {
		panic("Go main is on the native UI thread")
	}
	callback := ffi.NewCallback(func(code int64) int64 {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		if ffi.CallRet[int32](isUI) == 0 {
			panic("native test callback is not on its UI thread")
		}
		select {
		case codes <- code:
		default:
			panic("启动验收消息队列溢出")
		}
		return 0
	})
	ffi.Call(publish, callback, threadID())
	ffi.Call(record, int64(0), threadID())
	for code := range codes {
		runtime.GC()
		ffi.Call(record, code, threadID())
		if code == 99 {
			break
		}
	}
	ffi.Call(record, int64(100), threadID())
}
