# 构建、打包与调试

以下命令在使用方的应用模块目录执行，不是在已安装 Skill 目录中执行。
先确认实际依赖版本、目标和可用桌面；平台支持范围以该版本 README 为准。

## 普通 Go 构建

GOUI 不要求 cgo 或 GCC，但运行时仍加载原生系统库。“单二进制”不是“无系统依赖”。

Bash：

```sh
CGO_ENABLED=0 go build -trimpath -o ./dist/app .
CGO_ENABLED=0 go test ./...
```

Windows PowerShell：

```powershell
$env:CGO_ENABLED = '0'
go build -trimpath -o ./dist/app.exe .
go test ./...
```

调试时保留控制台和符号便于观察错误；发布时可加 `-ldflags="-s -w"`，Windows
GUI 子系统另加 `-H=windowsgui`，否则启动时会额外弹出控制台窗口，截图前也要用
这个选项重建。资源配置使用 cmd/goui 时不要再生成第二套 syso。
在不同 shell 中使用其自己的环境变量语法；PowerShell 变量会保留在当前会话，
检查完成后恢复用户原值，而不是一律删除。

通过 go.work 使用相邻的本地框架仓库联调时，不把机器专属路径写进 go.mod 的
replace。发布前用 `GOWORK=off go build ./...` 再验证一次；尚未推送的伪版本在
远端无法解析，本地工作区通过不等于发布依赖已经通过。

使用扩展控件时同时检查 GOUI 与扩展实际选中的 Version／Dir／Replace，以及扩展的
最低 GOUI 要求；不能只看应用 go.mod 的一行 require。发布构建在仓库外干净副本中
执行 GOWORK=off 的构建与测试，确认整个模块图可获取且不靠缓存或本地替换掩盖缺失
依赖。需要 tidy、补校验和或升级时先审阅差异，不自动改用户工作区配置。

框架升级逐项核对旧补偿的触发条件、目标公开接口与实际交互。宿主滚动顺序修复
不等于扩展编辑器提供了无副作用的行几何查询；新增行组也不等于虚拟列表行布局。
上游已修复但应用尚未迁移的项目应标为迁移待办，不继续宣称框架缺失，也不凭相似
名称直接删除有效补偿。

当前 README 的发布目标为 Windows amd64、macOS arm64、Linux amd64／X11。
不要仅凭 Go 可交叉编译，就承诺其他架构、原生 Wayland 或移动端可用。
Linux 需要显示连接及 X11／GLX、Pango／Cairo、Fontconfig、GLib／GIO、GTK 等运行库；
Wayland 会话需要 XWayland，Integrated 装饰需要合成器或按 GUI 契约回退。
依赖库名、系统版本和可用后端先查所选版本 README，再查目标机器。

交叉编译只作编译检查，例如：

```sh
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o ./dist/app.exe .
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o ./dist/app-macos .
```

## 应用身份、图标与打包

普通 go build 足够验证应用；需要图标、应用身份和平台分发结构时使用
GOUI 的 [cmd/goui](https://github.com/golang-gui/goui/tree/main/cmd/goui)。
优先查同版本 cmd/goui/README.md 和命令帮助，不将未来包格式当作已实现。

安装 CLI 时选用与应用一致的、已经存在的版本：

```sh
go install github.com/golang-gui/goui/cmd/goui@<version>
```

将 `<version>` 替换为确认存在的版本，不直接把占位符当作可执行命令。
在已有项目中不以 `@latest` 静默替换工具或应用依赖。

应用自己的 goui.toml 示例：

```toml
id = "com.example.Hello"
name = "Hello GOUI"
executable = "hello-goui"
main = "."
version = "0.1.0"
build = 1
icon = "assets/app.svg"

[windows]
console = false

[linux]
categories = ["Utility"]
```

文件路径相对于配置文件；icon 指向应用实际提供的资源。id 与应用入口保持一致。
示例 version 是应用版本，不是 GOUI 依赖版本。无图标时先核对可选配置，不创建
空白图标掩盖缺失。使用 cli 时可先执行：

```sh
goui doctor
goui build -config goui.toml -target windows/amd64
goui build -config goui.toml -target darwin/arm64
goui build -config goui.toml -target linux/amd64
```

按用户实际需要选择目标，不要求每个应用都同时交付三平台。

- build 生成裸二进制；package 消费已有二进制，不重新编译。
- Windows EXE 可直接分发；图标和版本资源由生成的 syso 参与构建。
- macOS `.app` 属于 package，不是 build。使用 macos.signing_identity 调用本机
  钥匙串／codesign 时，package 必须在 macOS 执行；使用 macos.signing 的 PKCS#12
  入口时，可在 Linux、Windows 或 macOS 组包和签名，包括显式配置的沙箱。
  正式分发需要 Developer ID Application 身份，iOS 签名证书不能代替它。
  notarize 使用 macos.notary 的 API 密钥跨平台公证；staple／archive 也可跨平台运行。
  两种签名配置不能混用，具体参数及支持范围见同版本 cmd/goui/README.md。
  构建、签名、公证 Accepted 与实际运行是不同证据；正式分发仍需在 macOS 对最终
  ZIP 解压产物核对签名、票据、Gatekeeper 并验收窗口操作。选择签名身份、公证、
  安装或发布都需要符合用户授权；不能擅自导出私钥或使用未获授权的开发者凭据。
- Linux dir 是 desktop／hicolor 安装目录树，二进制本身可独立运行，但桌面图标
  不是随便放在二进制旁就自动出现。当前工具不生成 DEB／RPM／AppImage。
- 生成资源有 .goui 摘要保护；冲突时先检查，不删掉用户已有 syso 或修改生成文件
  强行通过。细节以该版本 CLI 说明为准。

文件选择器返回 FileAccess，调用者应在访问完成后释放；取消与错误分开处理。
macOS 沙箱需要用户选择授予的资源及其访问凭据，不能把普通路径字符串视为永久
授权。读写时保持访问对象有效，恢复凭据失败时交给业务处理，不让整个 GUI 提权。

## 桌面验证

先运行使用方的包内测试和构建；窗口程序另行显式启动。检查本次实际涉及的场景：

- 基础界面：小窗口布局、长文案、Tab／Shift+Tab、Space／Enter、禁用和关闭。
- 文本：Unicode／CJK、选区、撤销重做；IME 预编辑和候选位置需要真实输入法。
- 样式／绘制：亮暗切换、主题色、1x／2x、透明／clip、图标原色与前景色。
- 数据／交互：虚拟化、列宽／排序保留、滚动消费、拖放取消与外部目标、窗口间移交。

不要求为无关功能穷举矩阵。记录 OS、后端、缩放，Linux 必要时记录 WM／合成器；
应用缩放不代替系统 HiDPI。go test 的 main 包编译、窗口启动、操作场景通过是
不同结果；无桌面时记录未执行，不扩大 skip 或删除失败断言来伪装通过。

复杂窗口场景尽量复用产品实际的 View 和组件，以固定／临时数据直接进入加载、空、
成功、失败等相关状态，提供切换、复位与可观察的检查结果。测试应隔离用户文件和
设置；窗口入口显式运行，不把网络服务或真实用户项目作为默认前提。
手工调用 View.Mount／Update 的最小替身不等于完整协调器，可能没有应用共享尺寸、
样式和行为修饰；这类测试只证明覆盖到的结构／逻辑，最终布局、焦点和输入需通过
真实协调与事件分发验收，不为测试公开框架私有状态。

自动检查等待实际布局、可见性或操作结果，设置时间／次数上限。固定延时、启动
截图或只有成功日志都不构成断言。跨窗口移交后还应继续输入、滚动及保存，验证真实
状态仍可用；接口存在或一次编译通过不能代替完整工作流。

可在进程启动前选择后端和缩放做对照，如 Bash：

```sh
GOUI_PLAT_PAINTER=software GOUI_PLAT_SCALE=2 ./dist/app
```

后端包括 software、opengl，Windows 另有 direct2d。设置不用于运行中切换；
初始化可能按平台策略降级，测量前核对实际后端，不能仅凭环境变量声称使用了 GPU。
检查后恢复配置。窗口边界和工作区的位置调整可能是系统事实，先核对原生契约，
不要加经验偏移、固定延迟或强制重绘去掩盖底层问题。

- Linux 的 Integrated 装饰依赖合成器。没有合成器的会话（如关闭合成的窗口管理器）
  会按契约回退，不显示应用绘制的标题按钮；看到外观差异先核对合成器，不当作应用缺陷。
- 自动化发送按键前，确认输入法环境变量（XMODIFIERS、GTK_IM_MODULE、QT_IM_MODULE）
  指向的输入法在该显示中实际运行；否则文本可能进不了输入框。真实 IME 验收仍需真实输入法。
- 性能对照记录实际后端、线程设置和是否有 GPU。软件 GL（如 llvmpipe）下的 CPU 占用
  不能代表有 GPU 的机器；先确认静止时不再请求帧，再比较播放或动画时的开销。

## 可选本地诊断

gui/dev 可以提供语义快照、输入分发和文本提交，不要求 AI 有视觉能力。
仅在用户需要时启用：应用须导入该包，并设置 GOUI_DEV_ADDR；优先仅调试构建
包含该导入，监听回环地址。具体协议查看所选版本的 gui/dev 源码或说明，
不虚构路由／截图接口。操作使用 dev 的 ref，不把它误当作 Widget ID。

Snapshot 用于理解控件、边界和动作，操作仍走正常输入分发。服务未启用时不要
修改生产启动设置；当前不以 DevServer 取代其他功能的真实桌面验收。收集日志／
快照供 Issue 使用前，移除私人内容、凭据和不必要的个人路径。

疑似框架问题先在所选版本做最小复现，将应用状态错误、使用限制和原生差异分开。
如需临时应用侧绕行，明确范围和代价，不静默修改框架。按 SKILL.md 的社区规则
提醒用户提交 Issue 或带回归测试的 PR；用户授权前不自动公开或提交。
