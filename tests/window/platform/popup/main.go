// 原生 Popup 的基础宿主与 Painter 验证。
// 环境：Linux/X11、Windows 或 macOS 桌面；按默认/Software/OpenGL（Windows 另有 Direct2D）重复。
// 启动：go run ./tests/window/platform/popup
// 初始：创建 owner，先用只暴露 Window 基础契约的包装对象创建 Popup。
// 操作与预期：无需输入；检查非零句柄、定位/调整尺寸、直接 Draw 的首帧，
// 再检查 NewPainter(Popup) 能绘制一帧；每项最多五秒，资源按 Painter/Popup/owner 顺序释放。
// 副作用：短暂显示两个窗口。复位：重启。退出：自动 PASS/0，断言失败非零；Ctrl-C 不算通过。
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

	checkPopup(plat)
	checkPopupPainter(plat)
	fmt.Println("PASS popup")
}
