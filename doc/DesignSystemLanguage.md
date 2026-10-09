# 系统界面语言与 demo 语言选择

## 契约

新增 `platform.Settings.Language() (string, error)`，返回用户首选界面语言的
BCP 47 标签；不是地区、日期格式或键盘布局。读取失败返回错误，不由平台假定
英语。平台提供中立 locale（Linux C/POSIX）时返回 `und`，不冒充配置了英文。

`gui.Settings.Language() string` 纳入既有初始/轮询快照：规范化标签，缺失、非法、
中立语言或查询失败使用 GUI 自有 `en` 默认值。getter 不调用原生 API；变化
复用 ConnectChanged。`ui.Settings.Language()` 使用既有 Sync 封装，无新通知。

Settings 与 mlang 是两个独立状态：查询系统语言不会改写应用语言。应用主动
调用 SetLanguage；不在 ui.Run 内自动选语言，也不因为系统轮询覆盖用户选择。

## 原生来源与绑定

- Windows：GetUserPreferredUILanguages(MUI_LANGUAGE_NAME) 首项，不使用区域
  格式的 GetUserDefaultLocaleName。SDK 按原契约查询 MULTI_SZ 大小/缓冲区、
  处理失败，win32 解释首项。沿用 sdk/winapi 的 DLL/类型与 ABI 风格。
- macOS：NSLocale.preferredLanguages 首项，扩展 frameworks/foundation 的
  NSLocale class/selector，复用既有 NSArray/NSString 方法；不在 cocoa 另建 objc 入口。
- Linux/X11：GLib g_get_language_names 首项。它表达进程语言环境
  LANGUAGE/LC_ALL/LC_MESSAGES/LANG，不承诺运行中修改桌面偏好必然修改进程
  环境。借用 GLib 数组并复制为 Go 字符串，不能释放它。x11 适配 POSIX locale
  的编码/分隔符/脚本修饰为 BCP 47，不改变环境或调用 setlocale。

参考原生契约：

- [Microsoft GetUserPreferredUILanguages](https://learn.microsoft.com/en-us/windows/win32/api/winnls/nf-winnls-getuserpreferreduilanguages)：有序、双零结尾显示语言列表；Vista 起支持。
- [Apple NSLocale.preferredLanguages](https://developer.apple.com/documentation/foundation/nslocale/preferredlanguages)：用户有序语言偏好，不等同 currentLocale 区域格式。
- [GLib g_get_language_names](https://docs.gtk.org/glib/func.get_language_names.html)：GLib 2.6 起支持，返回借用列表，含 C；查文档与当前本机 GLib 头文件。

## demo

- 工具栏显示八语言名称，增加“使用系统语言”操作；该操作是一次性应用当前
  Settings 值，不引入隐式持续跟随。显示系统标签与当前应用标签，便于核对。
- 展示文本放 `example/demo/resource`，使用具名 Getter/普通格式函数，不引入
  自动翻译任意字符串的全局字典。标题、分类、按钮、提示随 Build 重新取值。
- 页面 ID 使用固定英文标识，不由翻译后的标题拼接。业务 ID、路径、协议、
  用户输入/表格备注/文档及历史操作结果不重写；新操作使用当时语言。
- 命令式独立窗口/Popover 的固定展示 Label 通过 translateUI + ConnectChanged
  更新，按销毁/卸载释放句柄，不更换 Widget。已打开的声明式菜单由既有协调
  更新，不在 Build 中重建持久模型；DropDown 示例选项更新原模型项目。
- 不增加 RTL 整窗镜像、系统设置写入、国家枚举、区域格式器或新 UI 生命周期。

## 验证与提交切片

1. 原生绑定、locale 逻辑、GUI 快照/变化/失败默认值、UI 线程包装测试。
2. demo 八语言、固定 ID、原业务模型/文本/选区/列宽、命令式翻译生命周期回归。
3. 全仓、相关 race/vet、目标编译、本机真实鼠标和语言切换验收；原生系统语言
   查询优先通过 Windows/macOS 无头执行验证，不用改远端系统语言。
4. Settings 与 demo 分别提交，遵循既有英文 Add/Modify/Fix 句式；文档记录实际
   结果。编译、原生调用和桌面操作分别记录，不把缺失环境或跳过算作通过。

## 2026-10-09 执行结果

已完成三平台 Language 查询、GUI 初始/轮询快照与默认值、UI Sync 包装；未在
UI Build 或系统轮询中隐式更改应用语言。demo 使用固定页面 ID、同一选项模型，
234 条具名资源提供八语言，格式句子的占位符顺序保持一致；英文/德文等较长
导航标签允许换行，不通过改变业务标识实现翻译。命令式独立窗口在销毁时断开
连接；示例 Popover 关闭/失败断开，重新打开创建新实例。历史反馈保留原文。

验证环境：Go 1.25.8，Ubuntu 22.04.5 / Linux amd64；独立 Xvnc 1.12 / X11 /
Xfwm4 4.16.1 桌面 `:89`，未在用户桌面进行输入操作。

- `DISPLAY=:89 go test ./... -count=1 -json`：2135 个测试/子测试通过，14 个
  既有手动/显式启用项跳过，无失败。旧测试仍有原生窗口和图片输出副作用，
  没有迁移或弱化这些测试；本轮构建/日志/截图位于仓库外临时目录。
- `go test -race ./mlang ./internal/resource ./gui ./ui ./example/demo/... -count=1`
  通过。Settings 测试覆盖 getter 无原生查询、语言单独变化、失败默认值、
  快照完整通知，以及查询系统语言不改应用语言；UI 复用既有自动更新和退出清理。
- `go vet ./mlang ./internal/resource ./example/demo/...` 通过；GUI/UI 使用
  `go vet -composites=false ./gui ./ui` 通过。未重新修整既有 composites 警告。
- Windows amd64、macOS arm64 全仓 `go build ./...` 及 GUI/UI/原生 Settings
  测试目标编译通过。Linux arm64 的 mlang/GLib 绑定编译通过；全仓编译失败：
  既有 `platform/linux/libs/libc/syscall_linux.go:29,61` 使用该目标不存在的
  `syscall.SYS_PIPE/SYS_POLL`。HEAD 同样存在且本轮未修改，未扩展为 syscall 修复。
- 远端 Windows 10 19045 amd64、macOS 15.7.7 arm64 在 SSH 无头运行实际
  `TestSettingsLanguageNative`，均通过；传输后二进制 SHA-256 与本机一致。
  Windows 返回 `zh-CN`，与 Get-UICulture 一致；macOS 返回 `zh-Hans-AL`，
  与 `defaults read -g AppleLanguages` 首项一致。未改系统语言或操作远端桌面，
  这些结果不冒充 Direct2D/原生窗口、混合 DPI 验收。
- 本机临时验收驱动复用 demo 原声明，在 Software/OpenGL 配置的 1x/2x
  四组窗口逐语言检查：page-text、编辑器、输入框、菜单 Widget 身份不变，
  setter 准备的编辑内容和选区保留，标题与选项更新；实际独立窗口/Popover
  切换后文案更新、关闭后再切换不崩溃。临时驱动在仓库外，不为验证导出接口，
  不将 setter 准备或直接切换语言宣称为键盘/点击验收。
- 本机 OpenGL 配置 1x 另用真实 X11 鼠标操作 demo 菜单，中/英/阿拉伯切换
  与“使用系统语言”生效，截图检查导航换行、工具栏和文字显示；输入路径未
  绕过事件分发。OpenGL 配置允许按平台契约回退 Software，未强制探测每帧后端；
  Xvnc 图形环境为 Mesa llvmpipe，不代表硬件加速或原生混合 DPI。

工具问题记录：Windows 连接元数据称 PowerShell，但直接命令实际上由 cmd
解释，因此显式使用 powershell.exe -EncodedCommand。remote-shell 向 Windows
的标准输入二进制传输在 Console.OpenStandardInput().CopyTo 时阻塞且暂存文件
为零字节，已取消该命令，改用 SFTP 并校验哈希；POSIX/macOS 标准输入传输正常。
未重启健康连接、关闭安全服务或修改桌面配置。

待后续：母语人工校对、Windows/macOS demo 真实桌面、原生 IME 与混合 DPI。
不承诺 RTL 镜像、复数/区域格式化或自动持久化语言偏好。
