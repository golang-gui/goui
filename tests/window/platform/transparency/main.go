// 透明原生 Surface 的创建、呈现和后端资源契约验证。
// 环境：Linux/X11 + compositor、Windows 或 macOS 桌面；1x/2x 分别运行。
// 启动：go run ./tests/window/platform/transparency
// 初始：依次创建普通 owner、透明 Window 和 Popup，无需人工输入。
// 操作与预期：
//  1. 检查普通窗口不透明；透明请求成功则须报告透明，拒绝则两种角色均显式报错。
//  2. Software/OpenGL（Windows 另有 Direct2D）分别绘制半透明内容和全透明清除，
//     每个角色须完成两帧，单项五秒超时；此项只验呈现和生命周期，不验 compositor 像素。
//  3. Linux 读取 ARGB visual 与 GL framebuffer，在首像素核对预乘 RGBA；
//     R=128±1、G=64±1、B=0、A=128±1，容差只覆盖 8 位量化；全透明清除必须全零。
//     Windows 核对各后端独立设置的扩展窗口样式；macOS 无额外平台断言。
//
// 无透明能力或缺少后端时此程序失败并报告原因，不将其记为通过。
// 副作用：短暂显示透明窗口。复位：重启。退出：自动 PASS/0，失败非零；Ctrl-C 不算通过。
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

	checkTransparentSurfaceCreation(plat)
	checkTransparentSurfacePaint(plat)
	checkNativeAlpha(plat)
	fmt.Println("PASS transparency")
}
