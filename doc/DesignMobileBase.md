# 移动基座的原生对象级构建

本文记录已实现的移动构建基座设计和验收范围。它补充
[DesignMobileBuild](DesignMobileBuild.md)，不改变 platform/gui/ui 的职责。

## 目标与边界

应用使用者只安装 Go 和 goui 即可构建。维护者仍可使用原生工具链生产支持数据。
预置内容收敛为原生 ELF/Mach-O 对象、cgo 生成的 Go 源码及构建描述；普通 Go、
Go 汇编、导出数据和最终包归档均由使用者的当前 Go 工具链生成。

CGO_ENABLED=1 是当前移动链接模型需要的 Go 构建选择，不表示必须在消费阶段
运行 C 编译器。真正关闭 cgo 与免安装原生工具链是两个不同契约。

当前支持 Go1.25.8（保留兼容）、Go1.26.10、Go1.27.3，目标为 Android ARM64、
iOS ARM64 device 和 simulator。runtime/cgo、net、FFI 与固定入口使用对象级
描述；旧 format1 组件只保留 Go1.25.8 兼容，不自动回退或跨格式拼接。

## 构建描述

BaseManifest 的新格式按包记录选中源码、模块身份、目标、variant、最低系统、
Go/cgo 生成器及编译参数。每个包保存生成 Go 文件、原生对象、原包成员名称、
必要辅助头和链接要求；所有文件有 SHA-256。普通 Go 依赖的导出信息交给当前
编译器检查，不再保存已编译 Go 对象及依赖导出数据摘要。

原生对象按内容摘要存储，不同 Go 补丁版本可在验证后引用相同对象。最终仍按
原包归档；不把不同包的原生对象合并为全局静态库。生成文件与原生对象必须来自
同一份生产记录，不接受任意拼接。输入必须通过源码、摘要和目标校验。

保留精确 Go 版本的支持清单，不因降低粒度就承诺跨版本 ABI 稳定。源码或 cgo
生成协议变化时生成新描述；原生字节不变时复用对象。SDK/CRT/Dex/iOS辅助对象
继续独立于 Go 版本，不能将 SDK 派生输入许可审核与构建验证混为一谈。

## 维护者生产

扩展 goui-base export，使用匹配的官方 Go 和原生工具链完成正常 cgo 处理，
在受校验的 Go 构建接入点记录生成源码和最终原生对象，不解析 -x 文本为协议。
普通 Go 源码不复制到组件；保留真正 cgo 生成的源码、动态导入和原生链接要求。
Go 汇编对象和临时探测程序不属于原生预编译数据。

使用 cgo 的 -trimpath 归一化路径，禁止生产机器的 GOROOT/工作目录进入公共
组件身份。生成器遵守仓库仅提交 Go 源码的约定，原生产物在临时目录，最终压缩
到生成 Go 文件。生产和消费均不修改用户 GOROOT。

## 预置消费

1. 使用最终目标/tags 检查包依赖；仅为实际含 cgo 的包匹配构建描述。
2. 在缓存命中或 cgo/原生编译 action 运行前校验输入并加入描述身份。
3. 准备生成的 Go 文件及原生对象，官方 Go 编译/汇编/归档流程继续执行。
4. 保留 cgo 导入/导出、初始化、TLS、系统库和 framework 等链接语义。
5. 使用现有 Android/iOS 原生链接及组包流程，失败不发布部分最终产物。

netgo 等 tags 使某包不再使用 cgo 时，完全按普通 Go 包构建，不强制套用描述。
未知 cgo 包、源码变化或参数冲突明确失败，不自动调用原生编译器。

Go 1.25 是包内 cgo 流程；Go 1.26/1.27 将 cgo/原生编译拆为独立 action，须在
正确的 action 接入，不能只在最后包编译阶段替换。按源码结构复用小型版本适配，
但每个支持补丁仍验证官方源码摘要。所有 Go 内容现场编译后，旧归档所需的模块
路径归一化及依赖导出 ABI 匹配可以随旧格式退出。

## 用户 CGO 兼容

移动 build/doctor 提供 -cgo=prebuilt|native，默认 prebuilt；generate/package
不重新选择编译策略，只使用目标资源或原二进制构建记录。桌面构建行为保持。

| 行为 | prebuilt | native |
| --- | --- | --- |
| 内置原生支持 | 描述/生成源码/原生对象 | 标准 Go 从源码构建 |
| 用户 import C | 需显式匹配的支持描述，否则报错 | 标准 Go/cgo |
| 编译/链接工具 | 不运行 C 编译器，使用受控链接器 | 用户目标 CC/CXX/SDK 和完整链接器 |
| 参数 | 与预置描述匹配 | 保留 Go 的 CGO_*、CC/CXX、PKG_CONFIG 语义 |

不根据 gcc 是否存在自动切换模式，不自动降级。CGO_ENABLED=1 不选择 native；
未显式设置时为移动子进程启用。用户明确设置 CGO_ENABLED=0 时提前报告当前
移动链接模型不支持，不静默覆盖。构建 goui 自身仍可 CGO_ENABLED=0。
prebuilt 中显式且会被覆盖的 CC/CXX、非匹配 CGO 参数应报出冲突，native 保留。

native 完全绕过基座替换驱动、Go 归档和受控 golink 限制。Android 使用目标
工具链生成共享库；iOS 使用标准 c-archive 并以目标 SDK 链接。工具链由用户
提供，不承诺安装 gcc 就具备移动交叉编译能力。包外动态依赖的自动搜集/分发
不属于本切片；未满足现有组包能力的依赖必须明确反馈。

## 缓存与职责

对象与生成源码使用不可变摘要缓存；Go 编译结果继续由 Go 缓存管理。缓存身份
包含工具链、源码、目标、参数、构建策略、生成源码和原生对象摘要。预置和原生
action 不得错误共享结果。损坏缓存报错，不从损坏数据反推可信摘要。

platform/mobile/base 负责生产/消费协议、构建接入及两条构建路径；cmd/goui-base
负责生产命令；cmd/goui 负责策略、组件选择及打包。golink 不解析 Go 包，gui/ui
不参与。目标资源选择独立于 Go 基座选择，组包不要求当前宿主具有匹配基座。

## 验证与阶段

1. 分离 prebuilt/native，验证参数、环境隔离及真实用户 C 代码构建。
2. Go1.25.8 runtime/cgo 对象级消费，再覆盖 net/FFI/入口与 iOS。
3. 适配 Go1.26/1.27 的 action 及组件，逐版本构建/运行验收。
4. 内置数据切换并测量；旧格式明确兼容，不自动混用或隐藏回退。

逻辑测试覆盖输入/对象篡改、cache identity、tags、未知包、显式CGO关闭及两种
策略。预置冷构建使用拒绝编译器验证；native 用真实 C 宏和返回值证明用户编译
参数生效。各目标检查加载/初始化/回调/GC/网络/GLES/生命周期，标准库选择集
跟随对应 Go 版本生成，不固定旧版本测试数量。全仓测试、race/vet、三宿主构建
与模拟器执行分开记录；物理设备、16KiB及再分发审核仍单独跟踪。

依据：Go 官方 [cgo 生成说明](https://go.googlesource.com/go/+/master/src/cmd/cgo/doc.go)、
[cgo 环境契约](https://pkg.go.dev/cmd/cgo)、
[Go1.26构建action](https://github.com/golang/go/blob/go1.26.10/src/cmd/go/internal/work/action.go)。
以上是源码/文档依据，不代替 GOUI 各阶段运行证据。

## 使用与格式

默认使用预置路径，用户不需要 CC、SDK 或原生链接器：

```sh
goui build -config goui.toml -target android/arm64 -package apk -o app.apk
goui build -config goui.toml -target ios/arm64 -package ipa -unsigned -o resign.ipa
```

用户主动使用自己的 CGO 时显式选择原生路径：

```sh
CGO_ENABLED=1 CC=/path/to/android-target-clang \
CGO_CFLAGS='-O2 -g -DAPPLICATION_OPTION=73' \
goui build -config goui.toml -target android/arm64 -cgo native -package apk -o app.apk
```

iOS native 使用调用方 Apple 工具链；macOS 未设置 SDKROOT 时用 xcrun 查询
目标 SDK，设置时保留该路径。CC/CXX 中追加目标 triple 和 sysroot，保留调用方
其它参数；最终 native c-archive 由完整目标链接器处理。外置动态库不自动打包，
Android 系统依赖须有目标组件接口，iOS 限系统库/framework；自有依赖优先静态链接。
需要携带其它动态库时应另行设计组包支持，不能把仅成功链接当作运行成功。
这个限制在 package 验证，单独 build 仍可生成交给自定义组包流程使用的原生二进制。

维护者使用 goui-base export（默认-format=2），Go API ExportOptions.Format=0
保留旧格式调用语义。format2 的关键结构为：

```json
{
  "format": 2,
  "go_version": "go1.27.3",
  "goos": "android",
  "goarch": "arm64",
  "build_mode": "c-shared",
  "variant": "device",
  "minimum_version": "29",
  "architecture": "v8.0",
  "experiment": "",
  "cgo_flags": {
    "CGO_CFLAGS": "-O2 -g", "CGO_CPPFLAGS": "",
    "CGO_CXXFLAGS": "-O2 -g", "CGO_FFLAGS": "-O2 -g",
    "CGO_LDFLAGS": "-O2 -g"
  },
  "producer_tools": {"CC": "<sha256>", "CXX": "<sha256>", "cgo": "<sha256>"},
  "packages": {
    "runtime/cgo": {
      "source_sha256": "<sha256>",
      "generated_go": [{"name": "_cgo_gotypes.go", "path": "objects/<sha256>", "sha256": "<sha256>"}],
      "objects": [{"name": "_x001.o", "path": "objects/<sha256>", "sha256": "<sha256>"}],
      "auxiliary": [{"name": "_cgo_export.h", "path": "objects/<sha256>", "sha256": "<sha256>"}]
    }
  }
}
```

上例省略其它实际成员。ProducerTools 是生产端工具可执行文件或编译器包装器的
摘要来源记录，不要求消费端安装这些工具，也不宣称它覆盖包装器所有间接依赖。
SDK/CRT 来源仍由目标 inputs.json 的 provenance 表达。生成源码与原生对象共用
内容摘要池，不因文件名或 Go 补丁版本重复存储；版本清单保留各自原包成员名称。
生产者模块版本作为来源记录，format2 兼容性使用模块路径/语言、完整选中源码、
目标及参数，普通 Go 依赖的 ABI 由现场编译保证。

内置数据由 GOUI_MOBILE_PROFILES 指定的 JSON 目录列表一次生成，三目标为一组，
可包含多组精确 Go 版本。目标 SDK 输入须一致，独立压缩于 Go 版本之外；每目标
对象池只保存一份相同内容，各版本仅保存 base.json。generate/package 只选择
目标数据，不要求安装生成原二进制的 Go；format2 构建记录固定实际 Go 版本、
编译策略、目标数据身份与二进制摘要。旧 format1 构建记录仍须匹配原组件或重建。

## 本轮验证记录（2026-11-06）

- Linux amd64 / Go1.25.8、1.26.10、1.27.3：限定 PATH 只有对应 Go，三版本
  各构建 Android APK、待重签 device IPA、simulator App，共九份产物通过。
- Windows10 build19045 amd64、macOS15.7.7 ARM64 / Go1.25.8：仅 Go PATH 的
  默认 APK/待重签 IPA 构建及 cmd/goui 相关逻辑测试通过。新 Go 的宿主驱动与
  九目标产物验证在 Linux 完成；没有把它算成 Windows 新 Go 的原生运行验证。
- Android API30 ARM64（隔离只读 AVD、SwiftShader、1x）和 iOS26.3 ARM64
  Simulator（独立 iPhone16、3x）：三版本均执行对应官方标准库选择集、私有文件、
  HTTPS/系统信任拒绝、localhost DNS、GC及GLES；FBO在1x/2x验证颜色/clip/
  Transform。Go1.25.8均137通过/1原skip；Go1.26.10和1.27.3均133通过/1原skip，
  0失败。新版本整文件选择差异见 mobile_std/README，未删改官方断言。
- native：真实目标编译器编译用户 C 和宏参数，Android APK、iOS simulator App
  均运行138通过/1原skip；额外原生用例验证宏73得到76。宿主回归还验证错误宏
  返回失败、缺少宏编译失败。iOS device native 真机尚未运行。
- Linux Go1.25.8 新旧基座冷构建：拒绝编译器证明对象消费不执行 C 编译，原生
  宿主实际运行512次外来线程回调及GC/栈增长。普通 Go 依赖修改可重新编译，
  旧归档路径仍拒绝依赖导出ABI变化；对象篡改、tags、未知包、参数冲突均有断言。

内嵌 gzip 总计1,923,063字节（约1.83MiB），其中共享目标数据1,740,410字节，
三个Go版本的描述+去重对象池182,653字节。此前仅Go1.25.8的完整归档方案总计
3,076,433字节（约2.93MiB）。这是相同三个目标的构建工具输入体积，不是应用
大小或运行内存；没有将未压缩大小与压缩大小混比。

物理 iPhone/Android、Android16KiB设备、正式用户证书签名/安装、SDK派生输入
再分发审核和商店合规均未由本轮替代。macOS 签名复用、GUI触摸/IME/SafeArea
及任意第三方动态库组包不在此切片。

最终冻结源码在本机独立 Xvnc/Xfwm4 会话执行 go test -p 4 -json -count=1 ./...：
4505通过、11跳过、0失败。包括10项既有平台/手工验收skip，以及Linux不适用的
TestNativeIOSSDKSelection；后者已在远端macOS实际执行通过。冻结范围为已跟踪
源码加本轮新增文件，未纳入工作区历史未跟踪试验程序。既有离屏测试仍可能在
该临时副本写出图片，窗口测试只作用于独立会话。cmd/goui、platform/mobile/base
的 -race 和相关工具/生成器 go vet 均通过。
