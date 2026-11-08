# 移动平台原生 UI 主线程验收

本例仅依赖 platform，不使用 gui/ui、DevServer、测试 probe 或自行编译的原生宿主。
由 Go 启动 goroutine 准备 Platform/EventLoop，再把资源创建投递到系统 UI 主线程。
窗口、排版、绘制、后台观察与最终清理均独立检查原生线程身份。

## 环境与构建

Android API29+ ARM64，或 iOS ARM64 Simulator/device。使用 cmd/goui 支持的 Go
版本及内置基座。运行前记录系统版本、设备/模拟器、实际后端和 scale。

在仓库根运行，输出目录应为新的仓库外目录：

```sh
go run ./cmd/goui build -config tests/window/platform/mobile_dispatch/goui.toml -target android/arm64 -package apk -o /tmp/mobile-dispatch.apk
go run ./cmd/goui build -config tests/window/platform/mobile_dispatch/goui.toml -target ios/arm64 -variant simulator -package app -unsigned -o /tmp/MobileDispatch.app
```

GLES 版本额外传入 `-ldflags '-X main.painterName=opengl'`，改用新的输出路径。
目标须实际提供 GLES3。模拟器若只开放 ES2，应在其支持的前提下用启动参数
`-feature GLESDynamicVersion` 启用更高版本；不能把创建失败当成验收通过。
iPhone device 使用 `-variant device -package ipa -unsigned` 输出可重签 IPA；
未签名包不能直接安装到物理设备，不把 Simulator 通过当成真机通过。

Android APK 使用工具的开发签名，本例配置启用 debuggable 以便 run-as 读取自己的
沙箱报告；不要将此测试设置照搬到发布配置。安装后启动 `org.golanggui.MobileDispatch`。
iOS Simulator 使用 `simctl install/launch` 安装、启动。设备安装按开发者自己的
证书与 profile 签名；例程不读取签名凭据，不改变系统安全设置。

## 初始状态与操作

1. 启动显示多语言文字、绿色块、蓝色图片。Software/GLES 必须分别符合编译选择，
   后端不允许静默回退。初始化、每次事件和计时投递都检查原生主线程，错误即 panic。
2. 检查 `dispatch.json`：`mainThread=true`，uiThread 不等于 startupThread，
   frames 与 posts 继续增长，首帧后的显式 GC 完成后 gc=true；nonce 在同一次进程内不变。
3. 横竖屏往返至少四次：尺寸、绘制目标代次更新，图形继续显示，排版宽度适应新尺寸。
   不得死锁、崩溃、黑屏；Android 的 native redraw 必须在原生回调返回前完成。
4. 进入后台，等 visible=false 后相隔至少一秒读取两次报告：posts 可以增长，frames
   不应增长。恢复时 visible=true、frames 恢复，nonce 不变。重复两次。
5. Android 另覆盖 Activity 重建：旋转/系统重建后同一进程的主循环投递继续，不能因
   描述符随旧 Activity 销毁而丢失唤醒。进程被系统杀死后的新 nonce 属于冷启动。
6. 在报告目录创建空文件 `stop`：本例在 UI 回调释放图像、Painter、排版和窗口后
   请求 Quit。`completed.json` 必须出现，callbackReturned=true，nonce 与最后报告
   相同；dispatch.json 的 stopped=true。Run 不得在停止回调返回前结束。

Android 报告在应用 cache 目录，可通过以下方式读取或请求停止：

```sh
adb shell run-as org.golanggui.MobileDispatch cat cache/dispatch.json
adb shell run-as org.golanggui.MobileDispatch touch cache/stop
adb shell run-as org.golanggui.MobileDispatch cat cache/completed.json
```

iOS 报告在应用数据容器 `Documents/MobileDispatch/`；先用
`xcrun simctl get_app_container <udid> org.golanggui.MobileDispatch data` 获得实际容器。
仅在本轮专用 Simulator/设备中操作，不修改其他应用容器。

## 差异、副作用、复位与退出

本例使用旧平台内容显示钩子，不代表已实现 gui.Surface/Activate。Quit 停止 Go
应用调度，不能解释为命令系统退出 Activity/Scene；停止后可以仍看到原生空宿主。
触摸、IME、Popup、多 Scene、安全区域及 GPU 真正丢失后的资源重建不在通过范围。

周期请求绘制仅为观察线程和生命周期的测试负载，不作为性能或原生 vsync 指标。
仅写自身沙箱的 dispatch/completed/stop 文件，不读取用户文件、网络或剪贴板。
检查 native 日志不能有 JNI/FFI/线程违规。截图只能辅助，不能替代 JSON 中的断言。

复位：仅清理本例三个文件后重启，或卸载重装本例。退出：Android force-stop、
iOS Simulator terminate，真机按系统方式退出。验证后停止本轮创建的模拟器和私有
ADB server；保持用户已有模拟器、设备与系统配置不变。
