# 移动 Typography / Graphics 平台验收

## 环境与构建

Android API 29+ ARM64，或 iOS ARM64 Simulator/device。固定 Go 1.25.8 与匹配的
预编译基座、FFI 分支及链接输入；消费端不安装原生编译器。本例不导入 gui/ui，
通过 `platform.NewPlatform` 创建窗口、Typography 和 Painter，不直接选择共享移动实现。
模拟器通过不代表真机通过。
当前 GLES 依赖 nanovgo 的独立 `feature/mobile-base` 分支；未发布到正式依赖 tag。

维护者 Go 工具将三个阶段分开，配置文件与产物都放在仓库外：

```sh
go run ./tests/window/platform/mobile_build native /absolute/path/config.json
go run ./tests/window/platform/mobile_build build /absolute/path/config.json
go run ./tests/window/platform/mobile_build package /absolute/path/config.json
```

`native` 仅由维护者使用 NDK/Xcode 制作 测试 `probe.o`、iOS 最小 `main.o`，Android 另制作 Dex。
`build` 只需要 Go、Go driver、链接器、匹配基座和链接输入，不读取 `cc` 或调用 SDK；
可以将上一步的对象/Dex 目录复制到其他主机，再执行该阶段。
`package` 中 Android 使用指定资源工具和测试证书签名并验证 APK；iOS 只组装未签名
`.app`，在 Mac 使用自己的签名身份签名后才能安装。device 构建和 Simulator 分开；
脚本不生成正式 IPA，也不申请开发者凭据。

配置全部使用绝对路径，共同字段示例（按目标选择，不把两种 variant 混用）：

```json
{
  "target": "ios",
  "variant": "simulator",
  "painter": "software",
  "output": "/absolute/validation/ios-sim-software",
  "cc": "/absolute/maintainer/ios-sim-clang",
  "go": "/absolute/go1.25.8/bin/go",
  "gouiBase": "/absolute/tools/goui-base",
  "driver": "/absolute/tools/go-mobile",
  "linker": "/absolute/tools/goui-link",
  "base": "/absolute/components/base-ios-sim/base.json",
  "inputs": "/absolute/components/inputs-ios-sim/inputs.json",
  "tools": "/absolute/components/host-tools/tools.json",
  "consumerPath": "/absolute/consumer-bin-without-native-compilers",
  "modules": ["/absolute/cgo", "/absolute/purego", "/absolute/nanovgo"]
}
```

`cc` 可为可执行文件路径或 argv 数组；只在 native 阶段使用。`modules` 为可选的
本地模块替换目录，当前实验依赖需要明确列出；不修改仓库 go.mod。
Android 使用 `target=android`、`variant=device`，另提供 `java`、`androidJar`、`d8`、
`aapt2`、`signer`、`key`、`cert`、`apksigner`：前四者分别为 Java、平台 jar、D8 jar
和资源链接工具，`signer` 为接受 `-key/-cert/-in/-out` 的测试 APK 签名工具，
`apksigner` 为 Android SDK 的验证 jar。证书/密钥不放入仓库。
`painter` 仅控制测试 probe 的后端选择，生产窗口适配与上下文由 Go 实现；不存在两套原生宿主。

每阶段单独写入 `*-receipt.json`、命令日志与产物 SHA-256；build 检查固定入口归档
确实被使用且没有执行 cgo。保留消费 PATH 的拒绝编译器记录以独立确认没有调用
原生编译器。组件发布/下载、许可审核和正式签名仍是后续工作。

维护者制作、固定入口归档、Go 生成器、链接与签名的完整步骤见
[mobile_build](../mobile_build/README.md)。Android 测试通过 NativeActivity 的
func_name=probe_onCreate 包装真实入口，仅采集独立参考；iOS 最小 main 调用
固定 goui_run，UIKit 类和 Scene 回调由 Go 注册。原生参考源内嵌 generate.go，
通过 go generate 输出到仓库外，不再维护独立 C/Objective-C 文件或 Python 脚本。

测试使用 probe_use_gles 返回编译配置，再通过正常 Painter 选项选择后端。
iOS 首版 GPU 测试在前台目标就绪后创建 Painter；首次 Scene 前返回暂不可用。
正式 cmd/goui 移动组件分发、真机签名和商店验收仍在后续路线。

## 初始状态与自动断言

启动后检查注册字体、行/Cluster 范围、样式隔离、占位、1x/2x/3x 区域栅格、
输出缓冲复用和预乘约束；失败 panic，通过项写应用沙箱 `checks.json`。
iOS 另断言 CoreText 初始化不加载 AppKit/桌面 OpenGL；两端检查启动 goroutine 与
系统 UI 主线程分离、窗口回调运行于原生 UI 主线程、逻辑窗口创建不等待目标。
另检查平台单例、默认平台名、拒绝第二个窗口/队列、拒绝未知 Painter；Software
在 Run 和原生表面就绪前创建，首帧再执行离屏像素断言。GLES 显式请求在首帧创建。

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
