// 原生窗口契约验证：仅显式启动，所有原生调用均在 main 的平台线程执行。
//
// 环境：Linux/X11 + 窗口管理器、Windows 或 macOS 桌面；默认绘制后端，1x/2x 分别运行。
// 启动：go run ./tests/window/platform/window -scenario=basic
// 初始：按场景依次创建、显示并销毁测试窗口，无需人工输入。
// 操作与预期：
//  1. basic：检查标题、首帧、主动重绘、两次关闭请求（首次否决）、装饰创建、
//     缺少输入时的移动/缩放拒绝、工作区与销毁后的错误契约。
//  2. states：检查普通/最小化/隐藏、最大化后最小化再普通、全屏/最大化/普通；
//     每步须观察到实际事件或绘制状态，单组八秒超时，最后核对原生按钮几何。
//  3. integrated：Windows/macOS 检查普通/最大化/恢复的客户区、原生按钮与两次关闭。
//     Linux 不支持该装饰模式，basic 单独断言 ErrUnsupported，不将此场景视为通过。
//  4. native：Windows 检查 Win32 客户区、最小尺寸和 NCCALCSIZE；
//     macOS 检查原生按钮平移/隔离/恢复、缩放状态及回调销毁。
//
// 任一断言失败以非零退出；全部通过才输出 PASS，不能将启动成功当作视觉验收。
// 副作用：创建、移动、隐藏、最大化及全屏窗口；请在专用测试桌面运行。
// 复位：重启。退出：自动完成，关闭窗口或 Ctrl-C 中断不算通过。
package main

import (
	"flag"
	"fmt"

	"github.com/golang-gui/goui/platform"
)

func main() {
	scenario := flag.String("scenario", "basic", "basic, states, integrated or native")
	flag.Parse()
	plat, err := platform.NewPlatform(platform.DefaultName(), "org.golang-gui.NativeRegression")
	if err != nil {
		panic(err)
	}
	defer plat.Destroy()

	switch *scenario {
	case "basic":
		checkWindow(plat)
		checkWindowRequestPaint(plat)
		checkWindowChromeCreation(plat)
		checkWindowInteractionCapabilities(plat)
		checkWindowWorkArea(plat)
	case "states":
		checkWindowStateMinimizeTransitions(plat)
		checkWindowStateMaximizeTransitions(plat)
		checkWindowStateFullscreenTransitions(plat)
	case "integrated":
		checkWindowChromeIntegrated(plat)
	case "native":
		checkNativeGeometry(plat)
	default:
		panic("unknown scenario: " + *scenario)
	}
	fmt.Println("PASS window", *scenario)
}
