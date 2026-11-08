# GUI 激活与退出生命周期验收

验证 `NewApplication → ConnectActivate → Run` 的启动顺序、UI 线程、原生服务
初始化、GUI 计时器及 `ConnectShutdown` 清理，不使用 DevServer。

## 桌面

环境：Linux/X11、Windows 或 macOS 桌面；记录 OS/WM、实际 Painter 和缩放。

```sh
go run ./tests/window/gui/activation
go run ./tests/window/gui/activation -ui
```

初始：GUI 模式显示 ready；UI 模式额外显示 Update 按钮。GUI 模式由 Activate
创建窗口和 Label；UI 模式由 ui.Run 在首次激活构建，不在启动准备时创建 Widget。

1. GUI 模式关闭窗口：只触发一次 Shutdown，线程与 Activate 相同，退出输出
   `ACTIVATION PASS`。不得出现空白窗口、首次尺寸为零或原生线程异常。
2. UI 模式点击 Update：文字变为 updated，窗口保持；关闭窗口后正常清理并退出。
3. 分别增加 `-auto`：GUI 自动检查初始布局及 Snapshot、更新文本后销毁窗口；
   UI 自动检查绑定更新后 Quit。两种模式均必须输出 PASS 且退出码为 0。
4. Software/OpenGL 和适用的 Windows Direct2D 分别运行；1x/2x 记录为不同配置，
   不把交叉编译当作窗口验收。

## Android/iOS

移动部分通过 Application.NewSurface 建立实际 GUI 控件树，包含 Label、TextInput、
Button 和 LinearBox。没有 Show/窗口管理调用。原生触摸、IME 与 ui.Surface 声明
仍待后续接入；本例的输入由正常 DispatchEvent/CommitText 路径注入，不绕过控件行为。

```sh
go run ./cmd/goui build -config tests/window/gui/activation/goui.toml -target android/arm64 -package apk -o /tmp/gui-activation.apk
go run ./cmd/goui build -config tests/window/gui/activation/goui.toml -target ios/arm64 -variant simulator -package app -unsigned -o /tmp/GUIActivation.app
```

环境：Android API29+ ARM64；iOS ARM64 Simulator/device。默认 Software；
GLES 增加 `-ldflags '-X main.painterName=opengl'` 并换用新的输出路径，目标须支持
GLES3；AOSP_ATD 如仅开放 ES2，在其支持下使用 `-feature GLESDynamicVersion`。
设备 IPA 改用 `-variant device -package ipa -unsigned`；必须自行签名/重签才能安装。
本例 Android debuggable 和开发签名只用于读取自身沙箱报告，不用于发布。

初始：多语言标题、编辑框、Increment 按钮和 Count 标签。启动线程不得初始化
Typography；Activate 后领取 Surface、创建控件树和启动 GUI Timer。测试显式设置
顶部留白，不代表已接入 SafeArea。首次有效绘制后自动提交文本并派发一次鼠标点击。

1. `dispatch.json` 应有 activations=1、shutdowns=0、deliveries=1、mainThread=true、
   uiThread≠startupThread。frames/posts 持续增长；gc=true、inputVerified=true、count=1。
   mounts=1、unmounts=0；编辑框为 initial committed。backend 必须是 Software/OpenGL，
   该值由公开 diagnostics 帧记录核对，而不是只回显绘制偏好。
2. Android 横竖屏往返四次：尺寸与目标代次改变，activations/deliveries 始终为 1，
   控件继续显示，文本、count 和 Widget 身份不变，mounts 仍为 1。
   不得因 Activity/目标重建重复初始化应用或重新挂载子树。
3. 两次后台/恢复：visible=false 后 frames 停止增长，posts 可增长；恢复后 frames
   继续，nonce 和 UI 线程不变。前台恢复不额外触发 Activate。
4. 在报告目录创建空 stop 文件：Timer 请求 Application.Quit，Shutdown 在原生 UI
   线程停止 Timer 并释放 Surface/控件及其绘制资源。最终报告 stopped=true、shutdowns=1、
   surface.Ready=false、unmounts=1、destroys=1；随后 completed.json 出现
   callbackReturned=true，nonce 一致。
5. 检查 native 日志：不得有 JNI/线程异常、重复清理、退出死锁或静默绘图后端降级。

Android 包 ID `org.golanggui.MobileActivation`，报告在 cache 目录：

```sh
adb shell run-as org.golanggui.MobileActivation cat cache/dispatch.json
adb shell run-as org.golanggui.MobileActivation touch cache/stop
adb shell run-as org.golanggui.MobileActivation cat cache/completed.json
```

iOS 通过 `simctl get_app_container <udid> org.golanggui.MobileActivation data`
获取本次容器，报告在 `Documents/MobileActivation/`。每次安装后重新查询容器。

## 差异、副作用、复位与退出

桌面暂保留 Run 前创建资源的迁移兼容路径。移动原生主线程与启动线程分离；
Quit 停止 Go 调度，不命令操作系统结束 Activity/Scene，退出后可能留下空宿主。
不修改主题、安全设置、用户文件、剪贴板或外部应用；只写自己的沙箱报告。
周期请求仅为验证负载，不作为帧率/vsync 或性能结论。截图辅助检查不能代替断言。

复位：清理本例报告后重启，或卸载重装本例。退出：桌面关闭窗口；Android
force-stop、iOS Simulator terminate。只清理本轮自建模拟器、私有 ADB 和应用容器，
保持用户已有设备、模拟器及系统配置不变。物理设备、iOS 旋转、多 Scene、
SafeArea、移动 Popup、原生触摸/IME 和 ui.Surface 声明接入不属于本例已验证范围。
