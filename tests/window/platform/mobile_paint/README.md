# 移动 Typography / Graphics 平台验收

## 环境与构建

Android API 29+ ARM64，或 iOS ARM64 Simulator/device。固定 Go 1.25.8 与匹配的
预编译基座、FFI 分支及链接输入；消费端不安装原生编译器。本例不导入 gui/ui，
直接复用 `mobile.Window`、现有 Typography 和 Painter。模拟器通过不代表真机通过。
当前 GLES 依赖 nanovgo 的独立 `feature/mobile-base` 分支；未发布到正式依赖 tag。

维护者制作组件时：

- Android：NDK 编译 `platform/mobile/native/android.c`，增加
  `-DANativeActivity_onCreate=goui_android_create`；另编译本例 `native/android.c`。
  使用 Software 时 `-DGOUI_GLES=0`，GLES 时 `-DGOUI_GLES=1`。
  生产 `androidtext/java` helper 用 Android SDK 编译并经 D8 生成 Dex，放进 APK；
  测试不另写一套文字引擎。链接输入需包含公开 NDK `android` 库，GLES 另需
  `EGL`、`GLESv3`。Manifest 使用 `android.app.NativeActivity`，lib_name 为 goui，
  `hasCode=true`、`debuggable=true`，测试包 ID 为 `org.golanggui.mobilepaint`。
- iOS：按 Simulator/device 身份分别编译 `platform/mobile/native/ios.m` 与本例
  `native/ios.m`，使用 ARC，Software/GLES 选择与 Android 相同。
  链接 UIKit、Foundation、CoreGraphics、QuartzCore、objc；GLES 另链 OpenGLES。
  Info.plist 的 Scene delegate 为 GoUIScene，启用横竖屏，测试包 ID 同上。
  不调用 AppKit，不混用 Simulator 与 device 组件；真机使用自己的签名与 profile。

消费步骤使用已验证的 `goui-base build -entry`：`-dir` 指向本目录，`-base`、
`-inputs`、`-tools` 指向匹配组件清单，两个 `-object` 分别传宿主和测试 probe，
按上面的目标传 `-library` / `-framework`。保留 `-x` 日志，确认固定入口归档被
复用且没有运行 cgo 或原生编译器。Android 将输出 libgoui.so 与 Dex 组装、签名
为 APK；iOS 将输出 Probe 与 Scene plist 组成 .app 后签名。组件制作、组包签名
与消费 Go 构建分别记录，不能把维护者有 SDK 描述成用户必须安装 SDK。
这些是维护者验证步骤，不代表正式 `cmd/goui` 的移动组件分发已经完成。

测试按宿主是否导出 GLES 入口选择 Painter。两种宿主分别构建，不能将 GLES
宿主的 ANativeWindow/CAEAGLLayer 与 Software 呈现混用。iOS 首版 GPU 测试在
前台目标就绪后创建 Painter；首次 Scene 之前的 GPU 构造不属于本轮通过范围。

## 初始状态与自动断言

启动后检查注册字体、行/Cluster 范围、样式隔离、占位、1x/2x/3x 区域栅格、
输出缓冲复用和预乘约束；失败 panic，通过项写应用沙箱 `checks.json`。
iOS 另断言 CoreText 初始化不加载 AppKit/桌面 OpenGL；两端检查 GOUI 线程与
系统 UI 主线程分离、窗口回调固定 OS 线程、逻辑窗口创建不等待目标。

在同一 Painter 上以 1x/2x RenderImage，独立核对 Image.Update 后绘制、半透明
合成、Transform、clip 内外、ClearPath 擦除和文字上传；GLES 查询实际版本。
图形内部每通道允许误差 1（8-bit 混合量化），透明擦除为零，避开抗锯齿边界。
文字只验证有墨迹，区域栅格比较同后端同 scale 的完整位图裁切，不使用跨平台金图。

屏幕显示多语言文字、绿色圆角矩形、蓝色缓存图片和受 clip 限制的橙色块。
每 40 ms 通过 GOUI 队列请求一次绘制，仅为放大生命周期竞态的测试负载，
不代表生产帧调度或性能指标。`paint.json` 原子更新 nonce、visible、frames、
实际 DIP/scale、绘制目标 generation 及后端名称。

## 操作与逐项预期

1. 安装并启动：检查 checks.json 全部 Passed，画面有完整文字和三块图形。
   GLES 不能静默回退到 Software，错误不得仅被诊断日志消费。
2. 横竖屏往返至少八次：尺寸和 generation 变化，色块维持 DIP 尺寸；同一 nonce、
   Painter、字体布局和缓存图片继续使用，不能出现黑屏、死锁或永久停止提交。
3. Home，等待 visible=false，再间隔至少一秒读取 paint.json：frames 不变。
   返回后 visible=true、frames 增长、nonce 不变。重复三次。
   iOS 可用独立 XCTest target 编译 `native/PaintUITests.m`，外部同时读取 JSON
   检查上述事实；XCTest 成功本身不能替代后台停绘和图片持续有效的断言。
4. 检查竖屏、横屏截图三块色彩：sRGB 不透明 RGB(32,144,96)、RGB(48,128,224)、
   RGB(224,96,48)。纯色中心为窗口 DIP (76,222)、(184,222)、(260,222)，由实际
   scale 转像素，再扣除截图中已确认的系统栏偏移；不得靠移动采样点掩盖位置错误。
   每通道允许误差 1；截图若有色彩空间变换，先明确转换再比较。
   截图前先等新尺寸至少两个完成帧，不能只等待 SizeEvent；截图前后核对尺寸
   一致。simctl 截图可保持物理面板方向，LandscapeLeft 用已知系统旋转方向
   归一后再按窗口坐标采样，不能用颜色猜测旋转方向。
5. 保存原生日志；Android 可开启 CheckJNI，不能出现 JNI DETECTED ERROR 或
   Fatal signal。系统杀进程后的新 nonce 属于冷启动，不视为同进程恢复。

Android 状态：`adb shell run-as org.golanggui.mobilepaint cat files/paint.json`；
iOS 用 `simctl get_app_container ... data` 查 `Documents/paint.json`。

## 差异、副作用与清理

iOS GLES 已弃用，首版固定 GLES 3；Metal、ES2 和 context 真正丢失后的资源重建
不属于通过范围。模拟器会报告其自己的 GPU/字体环境，不代表实体 GPU、IME、
触摸输入、多 Scene 或全部系统版本。iOS 的 swap interval 不支持，明确返回错误。

本例把 stderr 重定向到自身沙箱 `go.stderr`，便于保留原生/Go 崩溃；还会写测试
字体、checks.json 与 paint.json。不开网络，不读剪贴板或用户文件。

复位：清除测试 App 数据。退出：测试工具 terminate/force-stop；后台不自行退出。
验证后停止本轮专用模拟器、私有 ADB，恢复本轮修改的模拟器旋转设置；不接管
用户设备，不修改桌面、安全设置或签名账户。SDK、证书、编译与截图产物置于仓库外。
