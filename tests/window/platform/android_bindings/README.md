# Android 原生绑定验收

本例覆盖 NDK 布局、JNI 引用和参数、SDK 文字/图形类、Typography 生命周期、
EGL 配置与 context 的原生调用。需要 Android
API 29+ ARM64 真机或模拟器、Go 和 `cmd/goui`，安装、取证使用 `adb`。模拟器须支持
OpenGL ES 3；仅构建 APK 不算运行通过。所有临时工程和结果放在仓库外。

## 生成与构建

在仓库根目录运行，输出目录必须尚不存在：

```sh
go run ./tests/window/platform/android_bindings -o /tmp/goui-android-bindings
cd /tmp/goui-android-bindings
go mod tidy
go run github.com/golang-gui/goui/cmd/goui build -target android/arm64 -package apk -o bindings.apk
```

生成工程位于 `github.com/golang-gui/goui/platform/validation` 模块命名空间，
以便借用既有 `platform/internal/androidruntime.Environment` 获取当前线程的
JNIEnv。它不是公开应用示例，也不为测试导出生产私有状态、反射访问 ART 私有符号，
或引入测试专用 C/Java 宿主。生成的 `replace` 指向当前源码，可按正常本地模块检查。

不要用裸 `GOOS=android CGO_ENABLED=0 go build` 替代此步骤；移动产物通过
`goui build` 接入预置 Go/native 基座。消费者不需要本机安装 NDK 或原生编译器。

## 操作与预期

以下命令默认只有一台设备；存在多台设备时为每条命令指定同一个 `adb -s SERIAL`。
CheckJNI 只在自己的测试设备开启，它影响随后启动的进程；不修改正在使用的设备。

```sh
adb shell setprop debug.checkjni 1
adb install -r /tmp/goui-android-bindings/bindings.apk
adb shell am force-stop org.golanggui.AndroidBindings
adb shell pm clear org.golanggui.AndroidBindings
adb shell am start -W -n org.golanggui.AndroidBindings/android.app.NativeActivity
adb shell run-as org.golanggui.AndroidBindings cat cache/bindings.json
```

启动后初始界面为空，本例没有交互按钮；通常数秒内完成。逐项预期：

1. 检查发生在真实 Android UI 主线程，而不是 Go 启动 goroutine 所绑定的其他线程。
2. JNI 正确往返中文、NUL、非 BMP 字符及空串；整型和对象数组、负 `jint`、
   `jfloat` 和 `void` 方法返回值正确。global 引用跨 local frame 和 GC 有效。
3. SDK ClassLoader 加载不存在的类、JNI 查询不存在的方法产生异常，格式化并消费
   异常后仍能继续正常调用；SDK File 正确往返包含中文的绝对路径。
   循环创建 local frame 并触发 GC，引用和参数存储保持有效。
4. EGL 可接收 NULL/空属性、只查询配置数量、写入调用方配置数组、传出版本，
   创建 ES3 与共享 context、pbuffer，绑定、解绑并销毁；不得在进入 EGL 前越界。
5. `bindings.json` 中 `passed` 为 `true`，`check` 与所选检查相符，包含实际线程和 Go 版本。
   缺少结果、进程退出、panic、CheckJNI 错误或任意断言失败均为验收失败。

可通过 `cache/bindings-stage.txt` 定位最后到达的阶段，通过 `adb logcat` 查看
CheckJNI/崩溃信息。阶段文件只是诊断证据，不代替最终断言。若设备驱动不支持某项
合法 EGL 行为，保留阶段和原生日志，明确标注未通过，不删断言或由绑定静默改参数。

默认 `check=all` 执行原有 JNI/EGL 断言。可通过构建参数 `-ldflags '-X main.check=jni'`、
`-X main.check=egl`、`-X main.check=egl-null-query` 分开运行；每次按上述步骤停止、
清空应用后重新安装、启动并检查结果，保存各轮日志。不能用部分检查代替 all。

排版整理的专项检查另行选择，不要求 EGL 数量查询成功才能验证文字绑定：

- `text-sdk`：直接使用 SDK 的 TextPaint、SpannableString、StaticLayout.Builder，
  检查行宽/基线、LTR/RTL boolean，以及 Font、FontFamily、Typeface 原生调用。
  Bitmap 的 ARGB=0x80ff0000 经 NDK 读出必须为预乘 RGBA=(128,0,0,128)，精确比较；
  Canvas 覆盖浮点变换、clipRect 的 true/false、解绑后 recycle。无字体金图。
- `text`：执行上述 SDK 检查、下述耗时观测及生命周期回归。Context.Destroy 后修改
  已有 Layout 的字体/颜色，再按 1x/2x/1x 栅格化；预乘蓝通道必须等于 alpha，
  0<alpha<=128，其余颜色为零。另一份 Layout 的图像必须逐字节不变；销毁一份后
  另一份仍可设置装饰并绘制，重复 Destroy 安全。使用系统 Roboto-Regular.ttf。
- `text-bench`：仅观测性能。固定重复八段 Latin/中文/阿拉伯/emoji，240 DIP 宽，
  16pt；40 次创建+首次度量+销毁，1000 次命中缓存的度量，以及各 20 次预热后
  1x/2x 栅格化。Context 创建不在计时范围内，栅格化复用输出缓冲。结果在 `text`
  的 coldNs/cachedNs/rasterNs；不能将其解释为 FPS 或真机性能结论。
- `text-component`：故意使用旧 Dex 的负向测试。Go 后端应明确返回含 component ABI
  的错误，JNI 异常已经消费；不是成功创建排版 Context。仅用于维护者的混版验收，
  需通过 `-components` 指定旧组件，正常组件运行该检查会按预期失败。

专项均经 `platform.NewTypography` 和真实 UI 主线程调用；保存 check、passed 和日志。
text-sdk 不经过 GOUI 的 Java LayoutEngine，text 的位图缓存按 Layout 隔离。
完整文字几何/像素独立参考另运行 [android_text](../android_text/README.md)。

2026-11-09 的 Android API30 ARM64 / AOSP ATD / SwiftShader 验收中，`jni` 和
`egl` 可通过，但属性和输出数组同时为 NULL 的配置数量查询会在原生
`eglChooseConfig` → `rcChooseConfig_enc` → `qemu_pipe_read` 挂起。该组合由 EGL
规范允许，现保留 `egl-null-query` 和默认 all 的断言并标记该环境未通过；不把参数
改写为非空列表来掩盖结果，也不将分别支持 NULL 的结果视为这个组合已经通过。
绑定的 Go 空参数转换及其余原生参数另有独立断言；真机和其他系统版本待验收。

本例不覆盖可见绘制、生命周期恢复、触摸、IME 或真正 GPU context loss。
Software/GLES 的文字、图片、旋转、后台停绘/恢复和最终 UI 清理另运行
[mobile_dispatch](../mobile_dispatch/README.md)。NDK/Bitmap/JNI 固定类型布局及
EGL 参数存储、eventfd 并发累积与耗尽由包内测试覆盖。

## 复位与退出

完成后应用可能保留空白 NativeActivity；显式停止并卸载自己的测试应用，关闭只为
本次验收启动的模拟器，不停止共享设备或其他应用：

```sh
adb shell am force-stop org.golanggui.AndroidBindings
adb uninstall org.golanggui.AndroidBindings
adb shell setprop debug.checkjni 0
```

清理生成工程即可删除 APK、临时依赖记录和配置；不删除仓库资源或系统共享缓存。
