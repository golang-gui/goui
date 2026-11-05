# 官方标准库移动 App 验收

本例是维护者生成器，用固定 Go 1.25.8 的官方标准库测试源生成一个独立 Go 工程。
生成的 App 只依赖公开 platform/graphics/typography 和 Go 标准库，不导入 gui/ui，
不调用测试 probe，不需要触摸或 IME。

## 覆盖范围

- 选取 12 个标准库包的完整外部测试文件：unicode/utf8、unicode、math、cmp、io、
  path、strings、bytes、sync、sync/atomic、errors、reflect，共 134 项顶层测试。
- 原文件逐字复制，只将文件名从 *_test.go 改为普通构建文件；不复制标准库实现，
  不删除或修改断言、原有 skip。生成清单记录来源文件、测试名和 SHA-256，附 Go LICENSE。
- 另有 4 项明确标为 goui-platform 的运行检查：私有文件临时目录/读写/重命名、
  本地 HTTPS 与系统信任拒绝测试证书、localhost DNS、随机数/哈希/GC 活对象。
- 使用 Go 1.25.8 自带 testing.MainStart / M.Run。适配器与 Go 版本一起固定，
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

在 goui2 的 feature/mobile-base 分支根执行，输出必须是仓库外不存在的目录：

```sh
go run ./tests/window/platform/mobile_std -o /absolute/standard-app
# 可选 -painter opengl，默认 software；两者是不同构建，不在 App 内切换后端。
```

输出包含普通 main.go、goui.toml、图标、suite runner、official 原始测试文件和
official-sources.json。生成阶段不调用原生编译器或 SDK。开发分支尚未发布完整
移动依赖，维护者必须显式用独立 go.work 连接该工程、goui2 与移动版 cgo/purego/
nanovgo；不得偷偷修改主会话仓库或把原型 replace 描述成正式发布依赖。

## 构建

先由维护者提供与宿主/目标/variant/Go 版本匹配的本地组件集。基础归档之外，
本例的 Android net 需要相应标准库 cgo 归档；iOS 需要 Security、resolv 等链接输入。
组件和用户构建的分离见 [移动构建设计](../../../../doc/DesignMobileBuild.md)。

```sh
go build -o /absolute/tools/goui ./cmd/goui

# 调用方只需匹配的 Go、goui 和预编译组件；以下步骤不调用 C/Java 编译器。
goui doctor -config /absolute/standard-app/goui.toml \
  -target android/arm64 -components /absolute/components
goui build -config /absolute/standard-app/goui.toml \
  -target android/arm64 -components /absolute/components \
  -package apk -o /absolute/artifacts/standard-tests.apk

goui build -config /absolute/standard-app/goui.toml \
  -target ios/arm64 -variant simulator -components /absolute/components \
  -package app -unsigned -o /absolute/artifacts/StandardTests-Simulator.app

# 真机 IPA 供测试平台重签；此文件不能直接安装到未授权设备。
goui build -config /absolute/standard-app/goui.toml \
  -target ios/arm64 -components /absolute/components \
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
2. 等待结束。初始选择集为 134 项官方测试 + 4 项平台检查；正常短模式结果为
   PASS 137、FAIL 0、SKIP 1、code 0。若平台出现额外跳过或失败，按报告记录，
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
