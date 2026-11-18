// Linux GLX 上下文与最大化重绘回归。
// 环境：X11 + 窗口管理器、GLX/OpenGL；1x/2x 分别运行。
// 启动：go run ./tests/window/platform/glx
// 初始：创建测试窗口；程序始终在平台保留的 main 线程使用 GLX。
// 操作与预期：无需输入；16 轮 MakeCurrent/重复绑定/ClearCurrent 中主动让出 goroutine，
// 随后完成 12 轮最大化/恢复，每个状态至少三帧；最大化尺寸必须大于初始尺寸，十二秒超时失败。
// 副作用：显示并最大化窗口。复位：重启。退出：自动 PASS/0，失败非零；Ctrl-C 不算通过。
package main

import (
	"fmt"

	"github.com/golang-gui/goui/platform"
)

func main() {
	plat, err := platform.NewPlatform(platform.DefaultName(), "org.golang-gui.NativeRegression")
	if err != nil {
		panic(err)
	}
	defer plat.Destroy()

	checkGLXMakeCurrentReuse(plat)
	checkGLXWindowMaximizePaint(plat)
	fmt.Println("PASS glx")
}
