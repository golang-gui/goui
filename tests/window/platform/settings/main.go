// 原生设置读取验证。
// 环境：Linux/X11、Windows 或 macOS 平台会话；不创建可见窗口，绘制/缩放不适用。
// 启动：go run ./tests/window/platform/settings
// 初始：创建平台设置对象，不更改系统设置。
// 操作与预期：无需输入；检查明暗枚举、非透明主题色、非空字体、正且有限字号。
// 单项明确返回 ErrSettingUnsupported 时输出不适用，其他错误失败；不允许所有项均不支持。
// 副作用：只读取设置。复位：重启。退出：自动 PASS/0 或断言失败非零。
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

	checkSettings(plat)
	fmt.Println("PASS settings")
}
