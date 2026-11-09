//go:build android

// Android 原生宿主回调重入验收。环境、构建、预期、复位和退出见 README.md。
// 实际 SizeEvent 中先释放并重领逻辑 Window，再在第二个 Window 的事件中销毁平台。
// 不依赖 GUI/UI、桌面、私有状态导出或测试 C/Java 代码。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func main() {
	p, err := platform.NewPlatform(platform.DefaultName(), "org.golanggui.AndroidLifecycle")
	must(err)
	loop, err := p.NewEventLoop()
	must(err)
	source := p.(platform.WindowSource)
	deliveries, destroys, sizes := 0, 0, 0
	var callbackReturned atomic.Bool
	var factory func(platform.Window) platform.EventHandler
	factory = func(w platform.Window) platform.EventHandler {
		if syscall.Gettid() != syscall.Getpid() {
			panic("宿主交付没有运行在原生主线程")
		}
		deliveries++
		index := deliveries
		handled := false
		if index > 2 {
			panic("同一逻辑宿主重复交付")
		}
		return func(event platform.Event) {
			if syscall.Gettid() != syscall.Getpid() {
				panic("原生观察没有运行在主线程")
			}
			switch event.(type) {
			case events.DestroyEvent:
				destroys++
			case events.SizeEvent:
				if handled {
					return
				}
				handled = true
				sizes++
				if index == 1 {
					w.Destroy()
					loop.Post(func() { must(source.SetWindowHandler(factory)) })
					return
				}
				p.Destroy()
				if deliveries != 2 || destroys != 2 || sizes != 2 {
					panic(fmt.Sprintf("回调计数错误：%d/%d/%d", deliveries, destroys, sizes))
				}
				defer callbackReturned.Store(true)
				loop.Quit()
			}
		}
	}
	must(source.SetWindowHandler(factory))
	timer := time.AfterFunc(15*time.Second, func() { loop.Post(func() { panic("原生宿主重领超时") }) })
	defer timer.Stop()
	loop.Run()
	loop.Destroy()
	if !callbackReturned.Load() {
		panic("Run 提前返回")
	}
	data, err := json.Marshal(map[string]any{"passed": true, "deliveries": deliveries, "destroys": destroys, "sizes": sizes, "callbackReturned": callbackReturned.Load()})
	must(err)
	must(os.WriteFile(filepath.Join(os.TempDir(), "lifecycle.json"), data, 0600))
}
