# GOUI 发布流程

本篇供维护者和 AI 执行框架版本发布。只规定操作与验收，不增加运行时接口、
发布命令或 CI。应用构建工具的职责仍见 [构建与打包](DesignBuildPackaging.md)，
测试分类和副作用见 [测试约定](Testing.md)。读取本文不代表获得提交、推送、
创建标签、上传附件或修改 Release 的授权；执行范围以用户明确指令为准。

核心顺序：**准备版本并提交 → 冻结候选提交 → 验证与构建 → 签名公证 →
推送标签 → Draft 附件核对 → 正式发布 → 下载复核与文档收尾**。

## 1. 版本与发布范围

每次先确定框架版本、比较基线、Demo 策略、目标平台及可接受的已知限制。

### 1.1 版本号与变更分级

框架标签使用 `vX.Y.Z`。版本号表达变更规模，维护状态表达支持范围，发布说明列出
实际兼容性影响；三者不能互相替代。同一次发布包含多类变更时，按影响最大的变更
确定级别，不按提交数、代码行数或距离上次发布的时间定级。

| 层级 | 含义与增长条件 | 示例 |
| --- | --- | --- |
| 主版本 `X` | 一代框架。重大能力扩展、核心概念重构，或 API／行为变化涉及广泛迁移时增长 | `v1.2.3 → v2.0.0` |
| 次版本 `Y` | 一轮功能演进。新增独立功能、扩展能力边界，或调整既有 API／交互契约时增长 | `v1.2.3 → v1.3.0` |
| 迭代版本 `Z` | 既有能力范围内补全 API、修复缺陷、纠正实现偏差，或不改变功能契约的内部优化 | `v1.2.3 → v1.2.4` |

增长较高位时，后面的位数归零。仅文档或发布元数据变化若需要重新分发模块／程序，
也使用新版本；不为已有标签下的源码或附件静默替换内容。

“增加 API”不必然等于增加功能：为既有属性补齐缺失的读取／设置入口，可以属于
迭代；增加控件、交互方式、布局策略或平台能力，通常属于次版本。API 评估同时检查
调用方和第三方实现方；给公开 Go 接口增加方法也可能破坏既有实现，不能按增加了
导出符号就认定兼容。

修正“不符合设计或预期”的行为须有依据：已有公开契约、已确认设计、原生平台契约，
或有证据支持的一致行为要求。原行为符合既有约定、只是现在选择另一种设计时，按
功能或契约调整处理；没有明确约定时先确认缺陷依据。修复目的不自动决定发布级别。
修复所必需的局部 API 调整在 `v0.x` 阶段可以随迭代交付，但须说明必要性、影响范围
和迁移方式；实际涉及能力扩展、核心契约变化或广泛迁移时仍按较高层级发布。

### 1.2 开发阶段与 Go 模块兼容性

当前 `v0.x` 采用上述变更分级，不承诺 API 或行为稳定，也不承诺各迭代之间均可
无修改升级。这不免除正式发布的验证、已知问题披露和迁移说明要求。

进入 `v1` 前须确定公开兼容契约。`v1` 及以后遵循 Go 模块与 SemVer 的兼容规则：
不兼容的公开接口或已约定行为变化提升主版本，兼容的公开 API／能力增加提升
次版本，仅兼容修复和不改变公开契约的优化使用迭代版本。此兼容性约束优先于按
改动规模作出的初步分级；重大能力演进也可以选择提升主版本。

`v2` 及以上的 Go module 路径和 import 路径需带相应主版本后缀。不以 README 中的
自定义声明代替 Go 的模块兼容约定。应用明确要求一个版本也不构成依赖上限：
`go.mod` 的 `require` 是最低要求，依赖图中其他模块可能要求更高版本。
应用升级应审阅迁移项、验证实际选中的模块版本及应用行为。

依据：[SemVer 2.0.0](https://semver.org/spec/v2.0.0.html)、
[Go 模块版本](https://go.dev/doc/modules/version-numbers)和
[最小版本选择](https://go.dev/ref/mod#minimal-version-selection)。`v0.x` 的内部
变更分级不宣称是 `v1` 稳定模块的 SemVer 兼容承诺。

### 1.3 维护范围与发布质量

| 对象 | 维护规则 |
| --- | --- |
| 主版本系列，如 `1.x` | 维护周期较长，保持该代框架的发展方向并明确支持状态；不等于同时维护全部次版本 |
| 次版本系列，如 `1.2.x` | 维护周期较短，默认重点维护当前主版本的最新次版本；旧系列的关键缺陷／安全修复范围单独声明 |
| 单个迭代，如 `1.2.3` | 不可变的发布快照，不单独承诺长期维护；后续修正通过新的迭代发布 |

发布新次版本时，说明旧系列是否继续维护、接收哪些修复及结束支持的条件；发布新
主版本时，同样说明旧主版本的支持范围。只有明确宣布持续支持的系列才建立维护
分支，不默认维护所有历史分支。不在资源没有保障时承诺固定年限或永久支持。
每次 Release 明确所属系列的维护状态；实际维护中的系列和待结束项记录在 Plan。

维护期、兼容性与质量分别判断。“迭代不保证稳定”在当前 `v0.x` 指不保证无修改
升级和逐个迭代长期维护，不表示可以绕过验证；`v1` 之后不再以此放弃兼容承诺。
全部正式版本均需满足后文发布门槛。需要提前试用的版本使用 `-alpha.N`、`-beta.N`
或 `-rc.N`，说明试用范围并标记 Prerelease；不能用正式版本号掩盖尚未完成的验收。

### 1.4 Demo 与分发范围

| 内容 | 规则 |
| --- | --- |
| 框架版本 | 使用 Go module 的 `vX.Y.Z` 标签；0.x 的 API 变更也必须列出迁移说明 |
| 同步更新 Demo（默认） | TOML 的 `version` 为 `X.Y.Z`，三平台附件从同一候选提交重建 |
| 仅发布库修正 | 明确不重发 Demo，链接既有 Demo 的原始版本和下载地址；不改名冒充新版 |
| Demo 构建号 | 新 Demo 版本从 `build = 1` 开始；发布前需要区分重新打包时可递增并提交 |
| 已发布内容需修正 | 按上述规则确定新版本，通常为迭代；不移动标签，不以增加 build 为由悄悄替换公开附件 |

Demo 功能是否变化不能只看 example 目录：框架修正也可能改变运行效果。
选择“仅发布库”时，要明确用户下载到的仍是旧版 Demo，不能宣称已包含本轮修复。

当前 TOML 只接受三个数字段，各段 0..65535，build 为 1..9999。
Windows 文件／产品／清单版本为 `X.Y.Z.B`；macOS
`CFBundleShortVersionString` 为 `X.Y.Z`，`CFBundleVersion` 为 `B`。
Linux 单文件未默认嵌入 TOML 版本，不能承诺存在 `--version` 接口。
预发布若使用 `vX.Y.Z-rc.N`，TOML 仍只能写 `X.Y.Z`，用 build 和发布说明标识
候选轮次；GitHub 标记 Prerelease，不设置为 Latest。

当前正式附件基线为 Windows amd64、Linux amd64/X11、macOS arm64。
Linux arm64 的 pipe／poll 编译修正已合入，但尚未完成专项原生桌面验收，见
[适配计划](PlanLinuxArm64.md)；工具接受目标名不等于已经支持发布。
新增架构必须先补齐对应运行证据，再扩大发布列表。

## 2. 准备与冻结源码

1. 审阅上个版本以来的提交，区分新增、修复、API 迁移、已知限制和未实现规划。
2. 更新 `example/demo/goui.toml`、必要文档及验证记录，提交并合入 GitHub main。
   下载链接等只有发布后才能成立的内容留到收尾，不提前链接不存在的附件。
3. 核对 go.mod/go.sum；不携带本地路径 replace，不依赖 go.work 或未提交源码。
   如需 tidy，先审阅其变更、提交并重新验证，不在冻结后静默修改依赖。
4. 确认新版本的标签和 Release 尚不存在；网络或权限错误不能当作“不存在”。
   若任务是续跑，先核对已有标签、Draft、附件与日志，不重复创建。
5. 记录完整候选 SHA。准备提交先于冻结；构建工具也必须来自这份源码，不使用
   PATH 中来源不明的 goui 或 `@latest`。冻结后不在临时源码中修改版本或功能。

以下发布汇总命令使用 Linux/Bash；各代码块按阶段执行，不是可直接整篇运行的脚本。
先替换版本占位符，确认 origin 指向 `github.com/golang-gui/goui`：

```bash
set -euo pipefail
release_version='<X.Y.Z>'
[[ "$release_version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]
release_tag="v$release_version"
release_repo='golang-gui/goui'
git remote -v
git fetch origin main --tags
git diff --quiet
git diff --cached --quiet
candidate=$(git rev-parse HEAD)
test "$candidate" = "$(git rev-parse origin/main)"
git status --short
git ls-remote origin "refs/tags/$release_tag" "refs/tags/$release_tag^{}"
```

未跟踪文件属于使用者，不自动 add、删除或纳入产物。用候选提交的干净快照验证
可发现遗漏提交的依赖；不能因为原工作区能运行就认为发布源码完整。

```bash
release_dir=$(mktemp -d /tmp/goui-release.XXXXXX)
mkdir -p "$release_dir/source" "$release_dir/logs" "$release_dir/assets"
git archive "$candidate" -o "$release_dir/source.tar"
tar -xf "$release_dir/source.tar" -C "$release_dir/source"
cd "$release_dir/source"
export GOWORK=off CGO_ENABLED=0 GOFLAGS=-trimpath
go version
go mod verify
go build -o "$release_dir/goui" ./cmd/goui
```

记录候选 SHA、Go 精确版本、目标及构建参数。独立 shell／远端不继承这些变量，
须显式设置并核对路径；不要依赖工具连接保存 shell 状态。
源码、依赖或影响资源的配置发生变化后，产生新的候选提交并重做受影响的验证、
构建和签名；不得沿用旧产物却记录新 SHA。后续 main 前进也不自动改变候选 SHA。

## 3. 发布验证门槛

### 3.1 代码与编译

在候选源码的主验证平台执行全仓非缓存测试，保留完整日志和实际退出码：

```bash
go test -json ./... -count=1 > "$release_dir/logs/full-test.jsonl" \
  2> "$release_dir/logs/full-test.stderr"
```

当前仍有既有包内测试依赖桌面、创建窗口、修改剪贴板或写 PNG；这不是纯无头命令。
Linux 使用独立 X11 测试桌面及快照；远端也使用独立目录，不覆盖用户工作区。
没有桌面时如实记录限制，不能扩大 skip 或使用用户桌面却不报告副作用。

```bash
GOOS=linux GOARCH=amd64 go build ./...
GOOS=windows GOARCH=amd64 go build ./...
GOOS=darwin GOARCH=arm64 go build ./...
```

按改动风险增加相关包 race、vet、原生 ABI／离屏回归和专项窗口验收。
race 需宿主支持与 C 工具链，执行时单独使用 `CGO_ENABLED=1`；不改变正式产物
`CGO_ENABLED=0` 的要求，也不把开发验证依赖说成用户构建依赖。

新增失败应在发布前解决。既有失败必须在明确基线复现、记录命令和影响，并由
维护者确认是否接受；影响本次核心功能或承诺的平台可用性时仍应阻止发布。
跳过、未执行、不适用不计入通过。测试数量每次从本次日志统计，不抄历史数字。

### 3.2 三平台窗口

发布同步更新的 Demo 时，三平台至少验证最终分发程序的启动／退出、基础按钮和
文本输入、滚动、菜单／下拉、窗口调整，并复验本轮新增或受影响场景。
按改动风险补充其他后端、HiDPI、IME、DnD、多窗口等专题，不要求每轮重复全部组合。

记录 OS／架构、Linux WM／合成器、实际 Painter、应用缩放与系统缩放。
默认后端启动不等于指定后端通过；应用 2x 不等于真实 Retina／混合 DPI。
执行路径和窗口步骤遵守 Testing，不以 DevServer 快捷动作替代本轮验收。
远端无头测试、交叉编译、公证和窗口打开都不能代替桌面操作验收。
无法完成某个平台时，不声称通过；在发布前报告缺口，由维护者决定继续、延期或
明确缩小该版本的分发范围，不能自行悄悄漏掉平台。

## 4. 构建三平台 Demo

继续在候选源码中执行；所有 `-o` 使用绝对路径，避免与配置相对路径混淆：

```bash
for target in linux/amd64 windows/amd64 darwin/arm64; do
  "$release_dir/goui" doctor -config "$release_dir/source/example/demo/goui.toml" \
    -target "$target"
done

"$release_dir/goui" build -config "$release_dir/source/example/demo/goui.toml" \
  -target linux/amd64 -ldflags '-s -w' \
  -o "$release_dir/assets/goui-demo-$release_tag-linux-amd64"
"$release_dir/goui" build -config "$release_dir/source/example/demo/goui.toml" \
  -target windows/amd64 -ldflags '-s -w' \
  -o "$release_dir/assets/goui-demo-$release_tag-windows-amd64.exe"
"$release_dir/goui" build -config "$release_dir/source/example/demo/goui.toml" \
  -target darwin/arm64 -ldflags '-s -w' \
  -o "$release_dir/goui-demo-darwin-arm64"
"$release_dir/goui" package -config "$release_dir/source/example/demo/goui.toml" \
  -target darwin/arm64 -format app -binary "$release_dir/goui-demo-darwin-arm64" \
  -o "$release_dir/GOUI Demo.app"
```

各次构建和打包分别保留日志，不用未检查 pipefail 的管道掩盖失败。
Windows GUI 子系统和 syso 由 cmd/goui 生成，不给资源生成另建临时实现。
macOS package 消费已完成二进制，不重编译；输出目录必须尚不存在。
当前 Demo 无 signing_identity／sandbox，可先在 Linux 打包，再在 Mac 正式签名。
不为发行擅自启用沙箱、额外权限、调试或放宽执行限制的 entitlements。

构建后检查最终文件，不只检查配置或临时资源：

- Linux：ELF amd64、可执行权限、`go version -m` 的目标与 CGO_ENABLED；在要求的
  X11 环境启动。单文件仍动态依赖系统运行库，no-cgo 不等于完全静态链接。
- Windows：PE32+ amd64、GUI 子系统、链接后的图标／清单／版本资源。
  在 Windows 用系统 VersionInfo 或独立 PE 解析核对四段文件及产品版本，
  不仅查文件名或生成的 syso。未执行 Authenticode 签名时不得宣称 Windows 已签名。
- macOS：Mach-O arm64、Bundle ID、图标、可执行权限和 Info.plist 两个版本字段；
  最终分发 ZIP 的检查在下一节完成。

相同源码重建不承诺逐字节一致；签名时间戳等也会改变哈希。
Linux 若仅原生版本元数据变化，重建文件可能相同，应如实说明，而不是强行改字节。

## 5. macOS 正式签名、公证与归档

签名与公证有两条显式路径：跨平台 PKCS#12／Notary REST，或下述 macOS
钥匙串／Apple 工具。两条路径均须在 macOS 验收最终 ZIP 解压产物；传递中间产物
与最终 ZIP 时校验 SHA256，使用独立临时目录。

跨平台路径的配置与支持范围见[分发设计](DesignMacOSDistribution.md)：

```bash
goui package -config example/demo/goui.toml -target darwin/arm64 -format app
goui notarize -config example/demo/goui.toml -app "<已签名的 .app>" -o "<本轮收据.json>" -wait
# 超时后使用已保存 ID 续查，先检查 history，不能盲目重新提交。
goui notarize -config example/demo/goui.toml -submission "<本轮 ID>" -o "<本轮收据.json>" -wait
goui staple -config example/demo/goui.toml -app "<同一个 .app>"
goui archive -config example/demo/goui.toml -app "<已装订的 .app>" -o "<最终 ZIP>"
```

仅在配置了正式 Developer ID PKCS#12 和 Notary API 团队密钥时使用该路径；
不得把合成证书测试或 Apple 工具的成功当作 REST 正式认证验收。Linux 与 macOS
宿主已使用独立正式 PKCS#12／团队 API 密钥验证签名、REST 上传、公证、装订与
ZIP 路径；Windows 的原生全流程仍待验收。每次正式发布仍须冻结本轮源码并检查
对应最终附件，不能用既往验收代替。

以下变量属于 macOS shell：

```bash
set -euo pipefail
mac_release_dir='<Mac 上的本次独立目录>'
release_tag='<vX.Y.Z>'
demo_version='<X.Y.Z>'
demo_build='<B>'
signing_identity='<Developer ID Application 身份或证书指纹>'
notary_profile='<已有的 notarytool 凭据配置名>'
app="$mac_release_dir/GOUI Demo.app"
```

### 5.1 签名

正式直分发使用 Developer ID Application，不使用 Apple Development 或 ad-hoc。
`cmd/goui` 的 PKCS#12 路径已强制开启 runtime 与安全时间戳，不再用系统工具
重签。原 `macos.signing_identity` 路径保留开发用途的 `--timestamp=none` 语义，
使用该路径正式分发时，须按以下步骤明确重新签名。

```bash
/usr/bin/security find-identity -v -p codesigning
/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$app/Contents/Info.plist"
/usr/libexec/PlistBuddy -c 'Print :CFBundleVersion' "$app/Contents/Info.plist"
/usr/bin/codesign --force --options runtime --timestamp --sign "$signing_identity" "$app"
/usr/bin/codesign --verify --deep --strict --verbose=2 "$app"
/usr/bin/codesign --display --verbose=4 "$app"
```

核对实际版本、Team ID、Developer ID 信任链、runtime 标志和安全时间戳。
`--deep` 用于验证，不用于递归代签；若未来出现嵌套可执行内容，明确逐项签名顺序。

### 5.2 提交和续查

```bash
/usr/bin/ditto -c -k --sequesterRsrc --keepParent "$app" "$mac_release_dir/submission.zip"
/usr/bin/xcrun notarytool submit "$mac_release_dir/submission.zip" \
  --keychain-profile "$notary_profile" --output-format json \
  > "$mac_release_dir/notary-submit.json" 2> "$mac_release_dir/notary-submit.stderr"
```

提交前确认本轮尚无已有任务。上传成功立即保存返回的 submission ID，后续按 ID 查询：

```bash
submission_id='<本轮已返回的 submission ID>'
/usr/bin/xcrun notarytool info "$submission_id" \
  --keychain-profile "$notary_profile" --output-format json
/usr/bin/xcrun notarytool wait "$submission_id" --keychain-profile "$notary_profile" \
  --timeout 10m --output-format json > "$mac_release_dir/notary-result.json" \
  2> "$mac_release_dir/notary-result.stderr"
```

超时、工具输出延迟或连接中断不是提交失败；先检查已有结果、任务 ID 和 history，
不要盲目重复上传。状态为 Accepted 才继续；Invalid 时用 notarytool log 获取该任务
诊断，不用旧任务的 Accepted 代替本轮结果。公证不等于 App Store 审核。

钥匙串访问受安全会话约束；使用已有 profile 所在的凭据存储，不盲目增加
`--keychain` 指向另一个存储。不自动导出私钥、修改访问控制、重建凭据或降低
系统安全策略；密码不进入命令行、脚本、截图或日志。需要用户认证时，由用户
在本机安全交互会话完成。

### 5.3 装订并检查最终 ZIP

```bash
/usr/bin/grep -Eq '"status"[[:space:]]*:[[:space:]]*"Accepted"' \
  "$mac_release_dir/notary-result.json"
/usr/bin/xcrun stapler staple "$app"
/usr/bin/xcrun stapler validate "$app"
/usr/sbin/spctl --assess --type execute --verbose=4 "$app"
/usr/bin/ditto -c -k --sequesterRsrc --keepParent "$app" \
  "$mac_release_dir/goui-demo-$release_tag-darwin-arm64.zip"
mkdir "$mac_release_dir/verification"
/usr/bin/ditto -x -k "$mac_release_dir/goui-demo-$release_tag-darwin-arm64.zip" \
  "$mac_release_dir/verification"
verified_app="$mac_release_dir/verification/GOUI Demo.app"
/usr/bin/plutil -lint "$verified_app/Contents/Info.plist"
/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$verified_app/Contents/Info.plist"
/usr/libexec/PlistBuddy -c 'Print :CFBundleVersion' "$verified_app/Contents/Info.plist"
test "$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$verified_app/Contents/Info.plist")" = "$demo_version"
test "$(/usr/libexec/PlistBuddy -c 'Print :CFBundleVersion' "$verified_app/Contents/Info.plist")" = "$demo_build"
/usr/bin/codesign --verify --deep --strict --verbose=2 "$verified_app"
/usr/bin/xcrun stapler validate "$verified_app"
/usr/sbin/spctl --assess --type execute --verbose=4 "$verified_app"
/usr/bin/shasum -a 256 "$mac_release_dir/goui-demo-$release_tag-darwin-arm64.zip"
```

核对版本为本轮值，Gatekeeper 为 accepted / Notarized Developer ID，并以解压后的
app 完成窗口操作验收。最终 ZIP 使用本轮已验证的归档字节，不另行解压重打包。
签名后不得再修改 plist、二进制或受签名保护资源；发生修改就重新签名、公证和验证。

## 6. 附件与说明

同步发布 Demo 时，正式附件只有本轮明确验证过的目标，例如：

```text
goui-demo-vX.Y.Z-windows-amd64.exe
goui-demo-vX.Y.Z-linux-amd64
goui-demo-vX.Y.Z-darwin-arm64.zip
SHA256SUMS
BUILD-INFO.md
```

BUILD-INFO 至少包含：候选完整 SHA／标签、构建日期与 Go 版本、构建工具来源、
目标和 flags、TOML 版本／build、三份最终文件的大小与 SHA256、公证 submission ID
和结果、签名／票据／Gatekeeper 结果，以及验证环境、失败／跳过／未执行的边界。
不能只写“来自 main”“最新源码”或“全平台通过”，不附证书私钥、密码或凭据文件。

Release Notes 使用中英文摘要，至少包括：本次变化、API 迁移、Demo 是否更新、
下载和启动方法、验证范围、已知限制。不能把 SourceView 等独立项目或 Roadmap
中的规划描述成当前模块已包含功能。自动生成提交列表只能作为草稿，必须审阅。

先写完 BUILD-INFO，再在 Linux 汇总主机为最终附件生成校验文件，避免循环摘要：

```bash
cd "$release_dir/assets"
sha256sum "goui-demo-$release_tag-windows-amd64.exe" \
  "goui-demo-$release_tag-linux-amd64" "goui-demo-$release_tag-darwin-arm64.zip" \
  BUILD-INFO.md > SHA256SUMS
sha256sum -c SHA256SUMS
```

SHA256SUMS 包含其他四份附件，不包含自己；BUILD-INFO 中的二进制摘要与它一致。
Windows EXE 无须另做安装包；Linux 首次执行可能需要 chmod +x，macOS 解压打开 app。
不把 submission.zip、未签名 app、原始 Mach-O 或临时日志误上传为正式附件。

## 7. 标签与 Draft Release

全部验证和正式附件就绪后，回到原 Git 仓库，再次核对候选提交在 main 历史中，
且新标签确实不存在。示例从未发布的新版本创建 annotated tag：

```bash
cd '<GOUI Git 仓库绝对路径>'
git fetch origin main
git merge-base --is-ancestor "$candidate" origin/main
git tag -a "$release_tag" "$candidate" -m "Release $release_tag."
test "$(git rev-parse "$release_tag^{commit}")" = "$candidate"
git push origin "refs/tags/$release_tag"
git ls-remote origin "refs/tags/$release_tag" "refs/tags/$release_tag^{}"
```

核对远端 peeled SHA 等于候选 SHA；annotated tag 对象 SHA 与提交 SHA 不同是正常的。
只推本次标签，不使用 `--tags`、force push 或自动推送其他 remote。
**标签一旦推送，即可能被 Go proxy 获取；GitHub Draft 不会把 Go module 变回私有。**
续跑已有标签时只能核对并复用，不能删除重建或挪到新的提交。

```bash
gh release create "$release_tag" --repo "$release_repo" --draft --verify-tag \
  --title "GOUI $release_tag" --notes-file "$release_dir/release-notes.md" \
  "$release_dir/assets/goui-demo-$release_tag-windows-amd64.exe" \
  "$release_dir/assets/goui-demo-$release_tag-linux-amd64" \
  "$release_dir/assets/goui-demo-$release_tag-darwin-arm64.zip" \
  "$release_dir/assets/SHA256SUMS" "$release_dir/assets/BUILD-INFO.md"
gh release view "$release_tag" --repo "$release_repo" \
  --json databaseId,tagName,isDraft,isPrerelease,assets,body
```

核对 Draft、标签、附件数、名称、大小、上传状态与 GitHub 返回的 digest。
再从 Draft 下载到新目录，执行 SHA256SUMS，并将 BUILD-INFO／SHA256SUMS 与本地原件
逐字节比较；防止同一份错误校验文件和附件同时被上传后仍自检通过。
私有 Draft 的按 tag API 查询若返回 404，先用 gh release view／release 列表或明确的
release ID 核查，不当成不存在而重建。失败重试只处理经检查缺失或损坏的 Draft 附件。

仅发布库时省略 Demo 构建、签名和附件命令，Draft 说明明确引用旧 Demo；其余
源码冻结、代码验证、标签与发布后模块核对不省略，不创建空的校验文件冒充附件验证。

## 8. 发布与下载复核

Draft 核对通过并在用户授权范围内后，才发布正式版本并设为 Latest：

```bash
gh release edit "$release_tag" --repo "$release_repo" --draft=false \
  --prerelease=false --latest --verify-tag
gh release view "$release_tag" --repo "$release_repo" \
  --json url,tagName,isDraft,isPrerelease,isImmutable,assets
gh release view --repo "$release_repo" --json tagName,url
```

确认 Release 已公开、正式版本状态和 Latest 正确。预发布须采用相应标志，
不直接复用以上正式发布命令。同步 Demo 发布还要重新下载公开附件：

```bash
gh release download "$release_tag" --repo "$release_repo" --dir "$release_dir/downloaded"
cd "$release_dir/downloaded"
sha256sum -c SHA256SUMS
cmp BUILD-INFO.md "$release_dir/assets/BUILD-INFO.md"
cmp SHA256SUMS "$release_dir/assets/SHA256SUMS"
GOWORK=off GOPROXY=https://proxy.golang.org \
  go list -m -json "github.com/golang-gui/goui@$release_tag"
```

公开附件的名称、大小、digest 和本地已验证原件一致；下载结果不能只凭 HTTP 成功判断。
Go 查询使用仓库外目录，确认版本和来源正确；如需运行消费方测试，使用临时独立模块、
关闭 workspace 且无 replace。代理暂时不可用时保留错误、稍后重试，不为传播延迟改标签。

## 9. 收尾、失败恢复与禁止事项

- 更新 `README.md` 与 `doc/README_CN.md` 的 Demo 下载链接、版本相关启动命令；
  按实际发布的 Demo 版本填写，不机械套用最新框架版本。只在公开下载验证后修改。
- 在 [Plan](Plan.md) 记录发布结果及验证缺口；能力变化才同步 Roadmap，不把临时日志
  和本轮凭据处理堆进 AGENTS。归档日志、构建说明、公证 ID 和最终文件摘要。
- 收尾提交是否执行遵守用户指令；不 amend 已发布提交，不为包含下载链接而移动标签。
- 清理自己创建的测试桌面／临时 GUI 连接，保留已有用户会话。临时产物按明确路径
  管理，不递归删除仓库、home、工作区或未核实的变量目标。
- 失败停在当前阶段：源码变更回到冻结阶段，构建失败不上传，公证未接受不发布，
  Draft 不完整不公开。网络中断先查实际效果，避免重复推送、创建或公证。
- 已公开标签永不改写。发布后发现二进制或资源有错，按第1节定级发布新版本；先说明影响，
  再经维护者明确决策处理旧下载入口。安全问题按其授权处理，不暗中删除历史证据。
- Release 正文和下载指引可以修正，但事实和版本来源必须保留。确需替换公开附件时，
  属于明确批准的例外：先核对 immutable 状态、备份旧附件及元数据，说明新旧差异，
  重新构建／签名／校验并更新记录。不可变 Release 不能靠删除重建或改标签绕过。

v0.2.0 曾在标签发布后修正 Demo TOML 版本并替换附件，属于已记录的历史例外，
不是本流程的常规做法；来源与公证结果见该 Release 的 BUILD-INFO 和 Plan。

## 10. 每次发布记录模板

复制到本轮记录，未执行的项目不能勾选；“仅发布库”将 Demo 项标为不适用：

```text
版本／比较基线／候选 SHA：
Demo：同步更新 X.Y.Z / build B，或明确沿用哪个旧版
Go 版本／构建参数／目标：

[ ] 版本与依赖修改已提交并在 main，可从干净快照独立构建
[ ] 主验证平台全仓结果：通过 __ / 失败 __ / 跳过 __；日志及退出码 __
[ ] 相关 race／原生回归：环境、命令、结果及不适用项 __
[ ] Linux／Windows／macOS Demo 原生操作：后端、缩放、步骤、结果 __
[ ] 三端产物架构、图标、实际版本字段与候选来源一致
[ ] macOS Developer ID / runtime / 时间戳；公证 ID __、状态 Accepted
[ ] 最终 ZIP 解压后版本、签名、票据、Gatekeeper 与启动通过
[ ] BUILD-INFO、SHA256SUMS、双语 Release Notes 完整并交叉核对
[ ] 本次 annotated tag 的远端 peeled SHA 与候选一致
[ ] Draft 附件完整、下载哈希／原件比较通过
[ ] 正式 Release 状态与 Latest 正确；公开附件再次下载复核通过
[ ] Go proxy 可解析发布版本；消费方独立验证结果 __
[ ] 双语 README 下载入口更新，提交状态 __；Plan 的剩余缺口明确

已接受的既有失败／未执行／平台限制及维护者确认：
Release URL／日志与产物归档位置：
本轮副作用与清理结果：
```

## 11. 依据与维护

截至 2026-10-19，本流程结合仓库 cmd/goui 实现及实际三平台发布经验制定。
Go、GitHub、Apple 的外部规则以执行时官方文档和安装工具帮助为准：

- [Go module 发布](https://go.dev/doc/modules/publishing)：标签发布与版本不可变要求。
- [GitHub Release 管理](https://docs.github.com/en/repositories/releasing-projects-on-github/managing-releases-in-a-repository)：Draft、附件及不可变发布边界。
- [gh release create](https://cli.github.com/manual/gh_release_create)、[gh release edit](https://cli.github.com/manual/gh_release_edit)：核对本机支持的参数，不依赖 CLI 隐式创建标签。
- [Apple 公证要求](https://developer.apple.com/documentation/security/notarizing-macos-software-before-distribution)、[自定义公证工作流](https://developer.apple.com/documentation/security/customizing-the-notarization-workflow)：Developer ID、Hardened Runtime、时间戳、公证与票据。

官方规则、项目流程、源码行为和本轮实测结果是不同层次。本篇不声称所有第三方
应用、远端安全会话或系统版本行为相同；具体证据写进每次发布记录。
