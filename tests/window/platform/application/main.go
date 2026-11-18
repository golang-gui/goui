// 原生应用身份、事件循环与非法外部打开参数验证。
// 环境：Linux/X11、Windows 或 macOS 平台会话；无需绘制，缩放不适用。
// 启动：go run ./tests/window/platform/application
// 初始：创建一个 Platform，不打开外部应用。
// 操作与预期：无需输入；检查同 ID 单例与冲突 ID 拒绝（Linux/Windows）、
// Windows 进程 AppUserModelID；检查跨线程 Post 顺序与 Quit、销毁后不执行任务，
// 并拒绝无效 URL/路径。事件循环五秒看门狗超时视为失败。
// macOS 的重复 Platform 创建不属于原生后端契约，单例场景不适用，其余检查仍执行。
// 副作用：创建原生平台连接及线程消息资源，不修改剪贴板。复位：重启。
// 退出：自动完成；全部断言满足才 PASS/0，失败非零，Ctrl-C 不算通过。
package main

import (
	"fmt"

	"github.com/golang-gui/goui/platform"
)

const testApplicationID = "org.golang-gui.NativeRegression"

func main() {
	plat, err := platform.NewPlatform(platform.DefaultName(), "org.golang-gui.NativeRegression")
	if err != nil {
		panic(err)
	}
	defer plat.Destroy()

	checkIdentity(plat)
	checkEventLoop(plat)
	checkExternalOpenInvalidInput(plat)
	fmt.Println("PASS application")
}
