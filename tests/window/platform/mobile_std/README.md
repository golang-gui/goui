# 官方标准库移动 App 验收

本例是维护者生成器，用 Go 1.24.13、1.25.8、1.26.10 或1.27.3 的官方标准库测试源生成独立 Go 工程。
生成的 App 只依赖公开 platform/graphics/typography 和 Go 标准库，不导入 gui/ui，
不调用测试 probe，不需要触摸或 IME。

## 覆盖范围

- 选取 12 个标准库包的完整外部测试文件：unicode/utf8、unicode、math、cmp、io、
  path、strings、bytes、sync、sync/atomic、errors、reflect。Go1.24.13为133项、
  Go1.25.8为134项、Go1.26.10／1.27.3为130项顶层测试。
- 原文件逐字复制，只将文件名从 *_test.go 改为普通构建文件；不复制标准库实现，
  不删除或修改断言、原有 skip。生成清单记录来源文件、测试名和 SHA-256，附 Go LICENSE。
- 另有 4 项明确标为 goui-platform 的运行检查：私有文件临时目录/读写/重命名、
  本地 HTTPS 与系统信任拒绝测试证书、localhost DNS、随机数/哈希/GC 活对象。
- 使用对应 Go 版本自带 testing.MainStart / M.Run。适配器与 Go 版本一起固定，
  不以 linkname 绕过内部包，不自行模拟 testing.T 或把用例改成日志检查。
- 使用 test.short=true；例如 path/TestCleanMallocs 保留官方短模式 skip。
  用例内部的子测试照常执行，汇总 JSON 的 Results 按顶层测试计数。

这是可移植的官方测试选择集，**不是 go test std 全集**。需要 Go 内部测试包、
export_test 导出、私有 linkname、子进程或额外测试资源的文件尚未接入；包括 math
all_test、math/bits、io io_test、sync oncefunc_test、atomic atomic_test 等，不能
为了编译通过而删除其中部分断言。当前不运行 benchmark、fuzz、example 或覆盖率。
临时错误测试验证包装保留失败 exit code，不允许空跑或吞掉失败。

DNS 使用系统 localhost 解析，HTTPS 使用本地 httptest 服务和专用测试 CA，另检查
系统根不能信任该证书；没有 InsecureSkipVerify，不要求外部网站可达。这不等于
已经验收公网 DNS、联网策略、代理或所有系统证书链。

## 生成工程

在包含本例的 GOUI 源码根目录执行，输出必须是仓库外不存在的目录：

```sh
go run ./tests/window/platform/mobile_std -o /absolute/standard-app
# 可选 -painter opengl，默认 software；两者是不同构建，不在 App 内切换后端。
```

输出包含普通 main.go、goui.toml、图标、suite runner、official 原始测试文件和
official-sources.json。生成阶段不调用原生编译器或 SDK。FFI/渲染依赖使用正式
版本：goexlib/cgo v0.1.2、官方 purego v0.10.2、nanovgo v0.0.13，不需要这些
依赖的本地源码分支或 replace。若生成工程使用尚未发布的 GOUI 功能，维护者可
用仓库外独立 go.work 或仅替换 GOUI 模块来连接当前源码。命令默认使用内置组件；
Go 依赖已发布不代表 SDK 输入再分发审核或真机验收已经完成。

## 构建

使用匹配的受支持 Go 版本和内置组件构建，不需要 SDK、NDK、JDK 或原生编译器。
本例所需的 net cgo 生成源码/原生对象、Security/resolv 链接输入已包含在对应目标中。
维护者仍可用 -components 显式覆盖；组件边界和支持版本见
[移动构建设计](../../../../doc/DesignMobileBuild.md)。

```sh
go build -o /absolute/tools/goui ./cmd/goui

# 调用方只需匹配的 Go 与 goui；以下步骤不调用 C/Java 编译器。
goui doctor -config /absolute/standard-app/goui.toml \
  -target android/arm64
goui build -config /absolute/standard-app/goui.toml \
  -target android/arm64 \
  -package apk -o /absolute/artifacts/standard-tests.apk

goui build -config /absolute/standard-app/goui.toml \
  -target ios/arm64 -variant simulator \
  -package app -unsigned -o /absolute/artifacts/StandardTests-Simulator.app

# 真机 IPA 供测试平台重签；此文件不能直接安装到未授权设备。
goui build -config /absolute/standard-app/goui.toml \
  -target ios/arm64 \
  -package ipa -unsigned -o /absolute/artifacts/standard-tests-resign.ipa
```

APK 使用用户配置目录中的持久化开发 key/cert；本例 debuggable=true，便于 adb
run-as 取报告，不用于正式分发。用户允许的设备上可按正常系统流程信任安装。
iOS device 默认拒绝没有签名材料的组包；显式 -unsigned 是本轮用户确认的云测试
重签路径，不伪造证书/profile、不宣称可直接装机。Simulator `.app` 需独立签名/安装，
不得把 simulator Mach-O 放入真机 IPA。

## 初始状态、操作与预期

1. 安装并启动。界面上方显示 Go/平台/后端和运行状态，用例自动开始；无需点击。
   例程保守保留 80 DIP 顶部留白以便读报告，不表示 SafeArea 已实现。
2. 等待结束。Go1.25.8 选择集为134项官方测试+4项平台检查；正常短模式结果为
   PASS137、FAIL0、SKIP1、code0。Go1.26.10／1.27.3为130+4项，
   PASS133、FAIL0、SKIP1。Go1.24.13为133+4项，PASS136、FAIL0、SKIP1。若平台出现额外跳过或失败，按报告记录，
   不修改预期掩盖。发生失败时显示失败名称；panic 或测试超时同样属于失败。
3. 等待数秒，frames 应增长；界面不随测试结束退出。旋转设备后报告仍可绘制，
   原 nonce 保持。横屏空间有限可能裁剪文本，完整结果以文件为准。
4. 按系统 Home，后台等待两秒，再回到 App。后台 frames 应停止，恢复后增加，
   nonce 与完成结果保持。系统终止进程后的下一次启动会重新运行测试，不承诺
   跨进程保留测试状态。
5. 查看完整 report.json、tests.log 和 window.json。Android 位于实际 Context
   缓存目录的 goui-standard-tests；iOS 位于应用 Documents/GOUIStandardTests。
   绝对路径显示在界面。Android 可使用下面命令；云平台按其 App 文件导出能力读取。
6. 复位使用清除 App 数据/卸载后重装，或终止后重新启动；同开发签名支持覆盖安装。
   退出用系统后台/终止。验证结束卸载本例，并删除本轮专用模拟器；不停止他人的设备。

```sh
adb shell run-as org.golanggui.StandardTests cat cache/goui-standard-tests/report.json
adb shell run-as org.golanggui.StandardTests cat cache/goui-standard-tests/tests.log
```

本例会在自身沙箱写日志/结果、创建本地回环 HTTPS 服务和临时文件、运行 goroutine/
GC 检查；不打开外部浏览器、不申请特殊权限，不用 DevServer。测试运行期间不会
接入触摸/键盘/IME。原生 JNI/Objective-C 错误、画面缺失或后台 GPU 错误另记，不以
testing 的 code 0 代替窗口/生命周期验收。

## 2026-11-05 首轮记录

本机 Linux amd64 / Go 1.25.8 在仓库外空缓存、拒绝编译器 PATH 下通过正式 CLI
生成 APK、iOS Simulator app 和 device IPA；初版 134+3 在远端 Mac 上 Android
API30 ARM64/Software/1x、iOS26.3 ARM64 Simulator/Software/3x 运行通过。
最终 DNS/系统信任补充、源码冻结、完整计数、全仓结果与真机缺口见
[PlanMobileConvergence](../../../../doc/PlanMobileConvergence.md)。

## 2026-11-06 内置组件验收

Linux amd64、Windows10 amd64、macOS15.7.7 ARM64 均以 Go1.25.8、仅 Go PATH、
无 -components 参数生成 APK 和待重签 IPA。Android API30 ARM64 / SwiftShader /
OpenGL ES / 1x、iOS26.3 ARM64 Simulator / OpenGL ES / 3x 的最终构建均为
137通过、1项官方 short 跳过、0失败；GLES 1x/2x FBO 像素检查通过。

APK 资源与签名另用官方 aapt2/apksigner 检查。Go 签名器的合成身份测试 App 在
macOS 上通过 codesign 严格验签；模拟器安装另使用系统 ad-hoc bundle 签名。
这两项都不等于有效 Apple 开发身份在物理 iPhone 上安装通过。完整范围、预置
体积与真机/16 KiB/再分发缺口见
[移动构建设计第11节](../../../../doc/DesignMobileBuild.md#11-内置最小组件与纯-go-组包2026-11-06)。

## 新 Go 版本与原生 CGO

Go1.26.10／1.27.3 的 sync/waitgroup_test.go 整文件新增 internal/testenv 依赖，
不能在外部模块逐字执行。生成器只对这两个版本排除整个文件，清单的 Excluded
保留原始摘要和原因；不改写其中的断言，不影响 Go1.25.8 既有134项覆盖。
这不是通过扩大运行时 skip 隐藏失败。testing 适配仍有失败注入的回归测试。

维护者可添加真实用户 C 代码，验证显式 native 构建与用户宏参数：

```sh
go run ./tests/window/platform/mobile_std -painter opengl -cgo -o /absolute/native-app
CGO_ENABLED=1 CC=/path/to/android-target-clang \
CGO_CFLAGS='-O2 -g -DGOUI_NATIVE_SENTINEL=73' \
goui build -config /absolute/native-app/goui.toml -target android/arm64 -cgo native \
  -package apk -o /absolute/native.apk
```

原生用例增加一项 TestUserNativeCGO；宏73必须得到76，错误宏触发测试失败，
缺少宏则编译失败。iOS 原生构建需有效目标 Apple SDK；macOS 未指定 SDKROOT
时自动选择目标 SDK，显式 SDKROOT 保留。某些 Apple 工具启动器会注入 macOS
SDKROOT；不能把这个路径当成 iOS SDK。预置模式完全不需要这些原生工具。

Go1.24.13 单独生成133项官方测试，完整保留其 waitgroup_test.go，未复用新版的
排除规则。它的正常报告为137项（136pass/1原skip）；添加-cgo时为138项
（137pass/1原skip）。Go1.24 native模式与其它版本相同，必须提供真实目标工具链。
