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

移动部分暂以平台 WindowSource 承载验证图形，因为 gui.Surface 尚待下一切片。
这仅验证 GUI 应用启动和退出，不代表移动 Widget 树、触摸或 IME 已可用。

```sh
go run ./cmd/goui build -config tests/window/gui/activation/goui.toml -target android/arm64 -package apk -o /tmp/gui-activation.apk
go run ./cmd/goui build -config tests/window/gui/activation/goui.toml -target ios/arm64 -variant simulator -package app -unsigned -o /tmp/GUIActivation.app
```

环境：Android API29+ ARM64；iOS ARM64 Simulator/device。默认 Software；
GLES 增加 `-ldflags '-X main.painterName=opengl'` 并换用新的输出路径，目标须支持
GLES3；AOSP_ATD 如仅开放 ES2，在其支持下使用 `-feature GLESDynamicVersion`。
设备 IPA 改用 `-variant device -package ipa -unsigned`；必须自行签名/重签才能安装。
本例 Android debuggable 和开发签名只用于读取自身沙箱报告，不用于发布。

初始：多语言文字、绿色矩形和蓝色图片。启动线程不得初始化 Typography；
Activate 初始化完成后才创建排版、订阅系统宿主和启动 GUI Timer。

1. `dispatch.json` 应有 activations=1、shutdowns=0、deliveries=1、mainThread=true、
   uiThread≠startupThread。frames/posts 持续增长；首帧后的显式 GC 完成后 gc=true。
2. Android 横竖屏往返四次：尺寸与目标代次改变，activations/deliveries 始终为 1，
   文字和图形继续显示。不得因 Activity/目标重建重复初始化应用。
3. 两次后台/恢复：visible=false 后 frames 停止增长，posts 可增长；恢复后 frames
   继续，nonce 和 UI 线程不变。前台恢复不额外触发 Activate。
4. 在报告目录创建空 stop 文件：Timer 请求 Application.Quit，Shutdown 在原生 UI
   线程停止 Timer、释放排版/图像/Painter/窗口。最终报告 stopped=true、shutdowns=1、
   surface.Ready=false；随后 completed.json 出现 callbackReturned=true，nonce 一致。
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
SafeArea、移动 Popup、触摸、IME 和完整 GUI/UI Surface 不属于本例已验证范围。
