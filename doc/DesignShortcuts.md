# 快捷键：按键事实与 GUI 策略

> 当前已接入平台修饰键、基础布局字母键、GUI Key/KeyModifiers 转换和 GUI 快捷键控制器。
> UI 声明及原生文字提交消费衔接已实现。本机 Linux 窗口输入已验证；
> Windows/macOS 验证范围为无头逻辑测试，真实桌面输入仍待补验。逐次结果见下文。

## 已确认的范围

首版面向控件、子树和当前窗口的单次组合键，不同时引入统一命令系统、多段序列、
系统全局热键。默认焦点控件先处理，再向外冒泡；确需覆盖时显式使用捕获阶段。
实际操作仍经过 Window.DispatchEvent 和既有事件控制器，不另开语义执行捷径。

## 按键与修饰键契约

| 名称 | 平台事实 |
| --- | --- |
| Control | 始终为 Ctrl/Control |
| Shift | Shift |
| Alt | Windows/Linux Alt |
| Option | 独立的 macOS Option，不作为 Alt 的枚举别名 |
| Command | 独立的 macOS Command，不等于 Control 或 Super |
| Super | Linux Super |
| Win | 独立的 Windows 徽标键，不等于 Super 或 Command |
| AltGraph | X11 ISO_Level3_Shift，不把 Windows Ctrl+Alt 合成该键 |

Command、Win、Option 使用独立修饰位和按键值；新值追加到枚举尾部，不重排既有值。
macOS 左右 Command（原生 keycode 55/54）均报告 KeyCommand，并保留左右位置；
Option 使用独立值，Control 不变。Pointer/Wheel/Key 中的修饰位遵循相同命名契约。
依据 Apple [NSEvent.ModifierFlags](https://developer.apple.com/documentation/appkit/nsevent/modifierflags-swift.struct)
的独立 control、option（Option or Alt）、command 定义。

迁移：原先在 macOS 使用 ModifierSuper/KeySuper 表示 Command 的调用方需改用
Command；Windows 原先用 Super 表示徽标键的调用方改用 Win；macOS 原先用 Alt
表示 Option 的调用方改用 Option。GUI 文本编辑快捷键已跟随调整，Option 按词导航不变。
Alt/Option 都不能被当作普通编辑命令组合中的无关修饰位。gui/dev 的
`command`/`cmd` 解析为 Command，`super`/`meta` 保留 Super，`win` 为 Win，
`option` 为 Option，`alt` 为 Alt；模拟输入与真实输入使用相同的按键事实，
不接受 `primary` 原生修饰键。

Windows 的 VK_LWIN/VK_RWIN 分别报告 KeyWin 的左右位置，见
[Windows 虚拟键定义](https://learn.microsoft.com/en-us/windows/win32/inputdev/virtual-key-codes)。
Qt 将 Windows 徽标键映射为 Meta（[Qt 6 文档](https://doc.qt.io/qt-6/qt.html)），
但 GOUI 不采用这种平台层归一化：日志显示事实键名，跨平台意图由 GUI 策略表达。
共享语义的 Control/Shift 仍共用，不因平台不同而拆分。尚未接入的原生键不预加枚举。

## GUI 修饰键转换

`gui.KeyModifiers` 是独立类型，不是 events.Modifiers 的 alias。
`ModShift`、`ModControl` 始终对应 Shift、Control；`ModAlt` 在 Windows/Linux
对应 Alt、macOS 对应 Option；`ModPrimary` 在 Windows/Linux 对应 Control、
macOS 对应 Command。`ModWin` 仅适用于 Windows，`ModSuper` 仅适用于 Linux，
`ModCommand`、`ModOption` 仅适用于 macOS，不做替换；`ModAltGraph` 仅适用于 Linux/X11。

`KeyModifiers.Resolve() (events.Modifiers, bool)` 将声明转换为当前平台的实际掩码，
不修改原始事件，也不在原始事件中添加 Primary。未知位、不适用的平台专属位、
未知平台统一返回 `(0, false)`；调用方必须先检查 bool，不能降级为无修饰键匹配。
支持平台上的零值返回 `(0, true)`。组合按位合并，Windows/Linux 的
`ModPrimary|ModControl` 为 Control，macOS 则为 Command|Control；
macOS 的 `ModAlt|ModOption` 为 Option。

文本编辑器的主命令修饰键与 macOS Alt 语义复用此转换。按词导航在 Windows/Linux
使用 Control、macOS 使用 Option 的行为策略仍属于编辑器，不由转换器改变。
转换层不改变既有编辑器匹配优先级；快捷键控制器在下面单独规定。

包内测试在同一主机验证三平台全部 512 种声明组合、未知位和未知平台，
并验证当前平台的公开 Resolve 入口；这些测试不需要桌面。

本切片验证：Linux amd64 全仓 `go test ./...`、GUI 修饰键转换与编辑命令的
定向 race 测试、`git diff --check` 通过；Windows amd64/macOS arm64 的 GUI
测试目标交叉编译通过。未执行 Windows/macOS 原生运行或桌面操作验收。
全仓既有图片测试仍有 PNG 输出副作用，交叉编译产物放在仓库外临时目录。

## 后续输入能力

Key 将来可以容纳 Android Back、音量等实际按键，本轮不预加无后端支持的枚举。
应用只能处理系统实际投递的事件，不保证截获系统保留键。系统返回手势不是普通
KeyDown/KeyUp，未来单独接入返回/导航能力，不伪造键盘事件；参考 Android
[OnBackInvokedCallback](https://developer.android.com/reference/android/window/OnBackInvokedCallback)。

逻辑键与物理键位、键盘布局、Repeat、IME/AltGr 的原生投递和消费边界仍需在完整
快捷键系统前明确验证；本轮修饰键调整不宣称这些差异已经解决。

## 基础布局适配与 GUI Key

- 平台 KeyA–KeyZ 表示逻辑字母，忽略字母大小写但保留 Shift 修饰位。文字和组合字符
  仍走 IME，不从 KeyEvent 猜输入字符串。不支持的非 ASCII 字母不映射成固定美式字母。
- Windows 保留 WM_KEYDOWN 的虚拟键：它已经过当前键盘布局翻译，不根据 scan code
  重建字母。见 [Windows Keyboard Input Overview](https://learn.microsoft.com/en-us/windows/win32/inputdev/about-keyboard-input)。
- macOS 字母来源改为 NSEvent.charactersIgnoringModifiers，固定 keyCode 表只用于
  基础键分类、非字母键和左右位置。字母可来自原美式标点位置；Shift 大写仍对应同一字母。
  [Apple 文档](https://developer.apple.com/documentation/appkit/nsevent/charactersignoringmodifiers)
  明确该属性忽略 Shift 以外修饰键，可用于基于字符的键盘等价操作。
- X11 使用 core state 中的有效 XKB group、level 0 查询逻辑键，KeyDown、KeyUp 和
  IME 未消费的按键共用 XkbLookupKeySym 路径，保留每个键自己的 group 回绕规则；
  MappingNotify 刷新键盘映射、NumLock 和 AltGraph 掩码。文字继续使用 XIM。
  参见 [XKB library specification](https://www.x.org/archive/X11R7.7/doc/libX11/XKB/xkblib.html)。
- X11 通过 XkbKeysymToModifiers 查询 ISO_Level3_Shift 的实际修饰掩码，不假定 Mod5。
  单键报告 KeyAltGraph、位置 Standard（keysym 不能证明它位于键盘右侧）。事件包含
  ModifierAltGraph，避免 Control+Level3 被误匹配为普通 Control；未扩展为任意 XKB 层级系统。
- 适配范围是 AZERTY/QWERTZ 等拉丁字母布局及大小写；不宣称非拉丁布局的美式快捷键
  回退、全部标点组合、任意 Unicode 快捷键或布局无关物理按键已支持。KeyCode 仍保留。
- `gui.Key` 是独立类型，通过 `Resolve() (events.Key, bool)` 转换。普通键直接对应；
  KeyAlt 在 macOS 对应 Option，KeyPrimary 对应 Command（其余桌面对应 Control），
  Win/Super/Command/Option/AltGraph 仅在对应平台匹配。Unknown、未知枚举和不支持的平台返回 false。

## GUI 快捷键机制

采用 GTK4 的 EventController/传播阶段思路，但不引入其额外作用域管理器：
[GTK4 输入传播](https://docs.gtk.org/gtk4/input-handling.html)。GOUI 默认焦点控件优先，
不是 GTK 捕获阶段全局快捷键的默认顺序。

```go
shortcut := gui.NewShortcut(gui.KeyGesture{Key: gui.KeyS, Modifiers: gui.ModPrimary})
shortcut.ConnectActivate(save)
window.Shortcuts().AddShortcut(shortcut)
```

- KeyGesture 精确匹配 Key 和修饰位，额外修饰键不匹配。按键本身是修饰键时，忽略
  它自身按下所携带的那一位；KeyPrimary 的单键声明因此能正常匹配。
- Shortcut 提供 Gesture/SetGesture、Enabled/SetEnabled、AutoRepeat/SetAutoRepeat、
  ConnectActivate；默认启用、自动重复关闭。可以注册在多个控制器，同一控制器内重复
  Add 幂等。Remove/Clear 只移除注册，不销毁共享 Shortcut 或信号连接。
- ShortcutController 是 EventController，默认 Bubble，覆盖所在控件的焦点子树；
  Target 仅目标控件，Capture 显式优先。按注册顺序第一个启用匹配项消费事件；即使
  该项关闭重复激活，也消费匹配的重复 KeyDown，避免落入另一条绑定。KeyUp 不激活。
- Window.Shortcuts 为窗口持有的稳定控制器，默认在 Widget 冒泡之后，Capture 在树
  捕获之前，Target 在目标控件之后。空内容窗口也能处理。模态路由先于所有窗口快捷键。
- 回调前消费 GUI 传播；回调销毁窗口/目标控件、移除注册后，后续快捷键信号回调不能
  访问失效对象。窗口销毁清空自身注册并使控制器不可再注册，不清空外部 Shortcut 信号。
- X11 用相邻同时间戳/同窗口/同 keycode 的 release/press 识别 core auto-repeat，
  只记录通过 XIM 过滤、实际交给应用的按键状态，焦点丢失清空。XIM 消费的原始
  press/release 不更新状态，避免与随后回送的事件重复计数；Windows/macOS 使用原生重复事实。

### 左右修饰键：身份与组合分开

`KeyEvent.Key + Location` 保留发生变化的键及其侧别；`Modifiers` 是不分左右的
原生修饰位。普通 `KeyGesture` 仍只声明 Key/Modifiers，不新增 LeftControl 等
修饰位，也不从 S 的 Location 推断所按 Ctrl 的侧别。左右同时按住后放开一侧，
不能清除另一侧仍然贡献的修饰状态。macOS Option/Command 的身份保持独立。

参考：GTK 4.20.3 的 [KeyvalTrigger](https://github.com/GNOME/gtk/blob/4.20.3/gtk/gtkshortcuttrigger.c#L466)
与 Avalonia 11.3 的 [KeyGesture](https://github.com/AvaloniaUI/Avalonia/blob/11.3.0/src/Avalonia.Base/Input/KeyGesture.cs#L146)
都将触发键与聚合修饰位分开；Flutter 3.35 的
[LogicalKeySet](https://github.com/flutter/flutter/blob/3.35.0/packages/flutter/lib/src/widgets/shortcuts.dart#L293)
匹配也合并左右。采用这种分离，不照搬 Flutter 的全局键盘服务或合成按键事件。

平台接入：

- Windows：用已有 `GetKeyState` 绑定读取消息队列对应的左右 Shift/Ctrl/Alt/Win
  状态并合并，删除 Window 的累计修饰位。不能用 GetAsyncKeyState 的当前硬件
  状态替代事件时刻的状态；在窗口外按下、焦点切换后抬起不依赖本窗口历史。
  依据：[Microsoft GetKeyState 契约](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getkeystate)。
- macOS：flagsChanged 使用 keycode 和事件内的 NX_DEVICE 左右标志决定 Down/Up，
  Modifiers 继续读取 AppKit 聚合标志；删除 Window 的旧聚合缓存。使用
  [Apple IOLLEvent.h](https://github.com/apple-oss-distributions/IOHIDFamily/blob/main/IOHIDSystem/IOKit/hidsystem/IOLLEvent.h)
  的公开常量，绑定放 frameworks/iokit，适配留 Cocoa。不因另一侧仍按住而把 Up
  误报 Down，也不靠当前键盘查询推断历史事件。这里只处理已有瞬时修饰键，不扩展
  CapsLock/Fn 等事件行为；真实 AppKit 设备事件尚需桌面验证。
- X11：KeyRelease.state 是事件前的状态，释放一键后检查是否还有其他按住的
  keycode 贡献同一修饰位。使用原生 modifier map，不硬编码左右键码。FocusOut
  清除窗口状态，订阅 KeymapStateMask，以 FocusIn/EnterNotify 后排队的
  KeymapNotify 恢复原生按键快照，避免 XQueryKeymap 读到更晚的状态。
  [Xlib 的 KeymapNotify](https://www.x.org/releases/current/doc/libX11/libX11/libX11.html)
  为键盘全局快照，Window 字段不用；libX11 实际填 None，所以不按这个字段查窗口。
  快照只更新修饰键判定所需的键位图，不初始化 XIM 重复状态；有效按键仍在 XIM
  过滤后更新，不能再次把首次回送当作 Repeat。
- X11 AltGraph 的掩码按事件的 XKB group 计算并缓存，MappingNotify 后重建。
  XkbKeysymToModifiers 合并所有组，在 `us,cn(altgr-pinyin)` 上得到 Mod1|Mod5，
  会错误地给 US Alt 添加 AltGraph。改为按原生 modifier map 中的 keycode，使用
  XkbLookupKeySym 查询各组的 Level3 身份；保留 XKB 自己的 group wrap/redirect。
  不把所有右 Alt 当作 AltGraph，也不改变系统布局。

回归至少覆盖双侧按下、两种释放顺序、最后释放、窗口外按住再切入、外部抬起后
回来、XIM 过滤与回送、快照不污染 Repeat。无桌面逻辑在各平台 input 测试内，
真实事件逐项断言入口为 `tests/window/platform/modifiers`。

### 当前验证与剩余工作

UI 提供 `Shortcut(key).Modifiers(...).Enabled(...).AutoRepeat(...).OnActivate(...)`
链式声明，通过通用 ViewBase.Shortcuts 或 WindowView.Shortcuts 安装。
`Shortcut` 返回 `*ShortcutView`，与 MenuItem、OverlayItem 的描述对象风格一致，
不是 Widget；构造时只传必需的 Key，其余属性统一由链式方法配置。
Key/KeyModifiers 复用 GUI 类型，UI 不导入 platform，也不公开 KeyGesture 别名；
GUI 的 KeyGesture 保留，由 UI 内部组装。
Modifiers 多次调用替换原值，不累加；默认无修饰键、启用、不自动重复；
未提供 OnActivate 回调时不拦截按键。Shortcuts 替换整个声明列表，忽略 nil 条目。
列表按非 nil 条目的位置协调，而不是以描述对象指针为身份，更新已有 Shortcut 和回调；
缺失声明及卸载只移除 UI 自己的注册与连接，保留同控制器的命令式条目。
默认 Widget Bubble / Window Bubble；声明式层本轮不增加传播阶段配置。
回调不因重建执行，也不隐式 RequestUpdate。窗口入口为 tests/window/ui/shortcuts。

```go
ui.Window("main").Shortcuts(
    ui.Shortcut(ui.KeyS).Modifiers(ui.ModPrimary).Enabled(canSave).OnActivate(save),
    ui.Shortcut(ui.KeyF5).OnActivate(refresh),
)
```

逻辑回归覆盖三平台字母映射输入、GUI Key 转换、修饰组合、快捷键冲突顺序、
Capture/Target/Bubble、编辑器优先、重复抑制、模态路由和回调销毁。
基础映射测试不等于真实布局切换验收；X11 group/重复状态测试不等于原生服务器测试。

### 原生默认动作与文字输入

KeyEvent 的按键事实保持只读；可选 `Handled *bool` 是仅在本次同步派发内有效的
默认动作响应通道。`PreventDefault()` 只将它设为 true；不得异步保留指针或写回 false。
模拟输入可以不提供指针。GUI StopPropagation 只控制 GUI 传播，不自动禁止原生文字输入。

- Shortcut 匹配后在回调前同时停止传播、取消原生默认动作；被抑制的重复也消费。
- 文本编辑命令同样取消默认动作。普通文字、Option 字符、X11 AltGraph 字符及可能的
  Windows Ctrl+Alt 文本在编辑目标停止冒泡，但不取消原生提交。Ctrl+Alt 在这里是保守
  的编辑策略，不是平台声称它一定是 AltGr。编辑主命令精确匹配 Primary，可额外带 Shift。
- Windows 主消息循环在 TranslateMessage 前派发普通 GOUI 按键；被消费时不生成
  WM_CHAR/WM_DEADCHAR。VK_PROCESSKEY 保持 IME 路径；原生嵌套循环仍可通过 WndProc
  派发，但无法撤销该循环已执行的 TranslateMessage。本实现不通过 ToUnicodeEx 猜测字符。
- macOS 优先交给 input context，insertText/setMarkedText/doCommandBySelector 记录该键已派发；
  context 拒绝且尚未产生文本或命令时，再发一次 KeyEvent，避免 Cmd+S 等非编辑命令丢失。
  X11 继续让 XIM 优先消费；GUI 捕获阶段不能截获平台输入法已经消费的键。
- 模态目标打开时重置并暂停 owner 的输入法，关闭后按当前文本焦点恢复；模态期间不把
  迟到的 owner commit 插入背景编辑器。原生按键被路由给模态目标后取消 owner 默认动作。

快捷键不是 Widget，不新增 Snapshot 节点，也不把任意回调伪装成语义 Action；
模拟操作仍须派发输入，不能直接调用快捷键信号作为自动化捷径。

剩余步骤（保持完整目标，不以当前切片代替完成）：

1. 按 2026-09-22 确认的验证范围：真实窗口在本机验证，远端 Windows/macOS 只运行
   不依赖桌面的测试。不再把远端桌面会话作为本轮开发验收前置条件。
2. 本机 XIM、AltGr/死键/中文输入、快捷键优先级和重建已验证；Windows/macOS
   按键映射及 GUI/UI 快捷键无头测试已通过。Windows 此前 EXE 启动失败的记录保留，
   2026-09-22 左右修饰键修复轮次已重新执行成功，不再作为当前阻塞。
3. Windows WM_CHAR 队列用例需要原生窗口（即使窗口不可见），不属于无头逻辑测试；
   保留 tests/window/platform/keydefault 供后续本机 Windows 验证，不在远端偷偷运行。
   Windows/macOS 真实窗口输入仍是验证覆盖缺口，不以本机 Linux 的结果替代。

### 2026-09-22 本机窗口与远端无头复验

此节为首次验收记录：其中 F5 单次输入采用瞬时按下/抬起，未覆盖真实按住时间下
XIM 异步回送的时序；此前对 F5 验收范围的表述过宽。后续发现的问题和修复复验见下节。

- 环境：本机 Linux amd64、X11/Xfwm4（XFCE）、Software、1x，Fcitx/XIM。
  实际运行 tests/window/ui/shortcuts；通过 XTest 注入原生键鼠事件，不使用 DevServer，
  不直接调用 Widget 动作或快捷键信号。屏幕计数与运行日志相互核对，文本用原生
  Ctrl+A/C 和 xclip 精确断言 UTF-8 内容，不仅判断“窗口能启动”。
- 通过：Ctrl+S 单次和长按各只触发一次；长按 F6 可重复；额外 Shift 不误匹配。
  编辑框 Ctrl+A 不触发窗口绑定；编辑框 F5 命中局部绑定，按钮焦点 F5 命中窗口绑定。
  多次 Rebuild 后触发一次，Disable/Enable 正确；Ctrl+S 前后文本仍精确为 `abc`。
- 通过：运行中切换 French AZERTY / German QWERTZ，发送固定物理 X keycode，
  核对实际 XKB 映射后验证 A/Q、Y/Z 跟随逻辑字母；CapsLock 不改变匹配，额外 Shift
  不匹配。Fcitx 会在焦点变化时重设布局，初次被重设为 US 的输入不计为目标布局验收；
  重试先固定焦点、再设置布局并检查映射，避免把错误测试输入当作实现缺陷。
- 通过：French AltGr + dead-circumflex/e 提交内容精确为 `@ê`；Fcitx Pinyin 的
  `nihao` 出现候选、空格提交内容精确为 `你好`。其后 Ctrl+S 不污染文本，新组合
  `ni` 经 Escape 取消后已提交内容仍为 `你好`，没有误触发 Q/Y 等窗口快捷键。
- 通过：关闭用例退出码 0，无遗留用例窗口；恢复原 `us,cn` / `,altgr-pinyin`
  键盘映射、输入法激活状态，清空测试剪贴板内容。截图与辅助脚本位于仓库外临时目录。
- 远端 macOS 15.7.7 arm64：仅执行 Cocoa 字母/修饰键映射、GUI 转换/快捷键/编辑修饰位、
  UI 绑定协调测试，全部通过。GUI/UI 测试二进制与当前代码重新编译后的 SHA-256 相同；
  未启动远端 GUI 窗口，不涉及绘制后端或缩放。
- 远端 Windows 10 amd64：仅尝试 TestLogicalWindowsLetters；进程启动仍返回
  NativeCommandFailed / Access denied，未进入测试，不能记为通过，也不能断言由
  Session 0 导致。没有修改执行权限或安全策略，没有启动 keydefault 窗口用例。

本阶段验证记录：

- Linux amd64 `go test ./...` 通过；GUI/UI 快捷键和 X11 逻辑映射/重复状态的
  定向 `go test -race` 通过；`git diff --check` 通过。既有窗口测试的环境开关不变，
  既有图片测试仍可能生成 PNG；没有将跳过的桌面测试计作操作验收。
- Windows amd64 GUI/win32/UI 和 macOS arm64 GUI/Cocoa/UI 测试目标、两平台
  tests/window/ui/shortcuts 可执行程序交叉编译通过，产物均在仓库外临时目录。
- remote-shell：macOS 15.7.7 arm64 实际运行 TestLogicalMacLetters 和
  TestNativeModifierMapping 通过。这是纯映射测试，不是 NSEvent 真实键入验证。
- macOS 15.7.7 arm64 原生运行 GUI Key/KeyModifiers、快捷键顺序/重复/销毁、
  编辑目标优先及模态输入隔离逻辑测试通过；不涉及 Painter、缩放或真实桌面输入。
- Windows 10 SSH 进程位于 Session 0，macOS 控制台为 root；本阶段未验收桌面操作。
  Windows 测试 EXE 传输完整且 SHA-256 一致，但启动返回 Access denied，原因未确认；
  不将交叉编译替代实际运行，也不为执行测试修改安全策略。
- 新增 tests/window/platform/keydefault：只向自有隐藏 HWND 投递原生消息，断言消费 S
  不产生 WM_CHAR、未消费 A 提交一次；已交叉编译，Windows 原生执行仍待验证。
- 收尾复验：Linux amd64 全仓测试、GUI/UI/事件/X11 定向 race 和 diff 检查通过；
  AltGraph 增补后 Windows amd64、macOS arm64 上述编译目标重新通过，macOS GUI
  转换/快捷键/编辑修饰位原生逻辑测试重新通过。已有依赖开关的手动窗口、GLX、
  透明绘制及阴影像素测试存在 skip，不计为验收；既有图片测试仍会输出 PNG。

### 2026-09-22 XIM 回送与 F5 重复判定修复

- 根因实测：Fcitx/XIM 过滤原始 KeyPress，随后回送同一按键。原实现过滤前更新
  pressedKeys，导致第一次有效 KeyDown 已是 Repeat=true；默认不允许重复的局部
  快捷键消费该事件但不激活，窗口快捷键也不会再收到它。输入法英文模式同样经过 XIM。
  原版本用正常按住时长复现，原生跟踪观察到过滤与回送两次更新；瞬时抬起会改变时序，
  是此前未检出的原因，不是键盘或 GUI 作用域错误。
- 修复范围：仅 X11 在 XFilterEvent 返回并完成预编辑通知后，对未过滤的有效事件
  更新重复状态。保留 FocusOut 清理；没有放宽 Shortcut 的 AutoRepeat 契约。
- 无桌面逻辑回归：覆盖回送早于/晚于物理抬起、消费后不回送、连续重复、原始抬起
  不提前清除回送流状态、窗口隔离、无效 keycode 和焦点丢失后的首次按下。
- 本机窗口复验：Linux amd64、X11/Xfwm4、Software、1x、Fcitx/XIM；通过 XTest
  走原生事件路径。英文模式三次各保持 150ms 的 F5 使 local-F5 从 0 到 3；保持
  1.2 秒仅再加一，抬起重按再加一。F6 保持 1.2 秒产生 15 次激活。按钮焦点 F5
  只增加 window-F5，回到编辑框只增加 local-F5。中文模式三次 150ms F5 及一次
  1.2 秒 F5 共增加四次。日志按各独立计数断言，不以窗口启动代替验收。
- 中文输入回归：nihao 空格提交、新预编辑 ni 经 Escape 取消、Ctrl+S 激活后，
  原生 Ctrl+A/C 获取的文本精确为 `你好`，没有附加字符。初次文本检查被同时运行的
  既有窗口测试抢焦点而中止，不记为通过；全仓测试结束后重新聚焦完成以上检查。
- 验证命令：`go test -json ./...` 通过；39 个包通过，33 个包无测试文件；
  既有手动窗口、GLX、透明和阴影测试的 skip 保留，不记为验收。
  `go test -race ./platform/linux/x11 ./gui ./ui -run 'Test(X11Repeat|Shortcut)' -count=1`
  通过，Windows amd64/macOS arm64 的 shortcuts 窗口用例交叉编译通过。
  本轮未运行远端测试，也未新增 Windows/macOS 桌面验收结论。
- 窗口操作仅针对本次启动的测试窗口；结束后关闭它，保留用户原有窗口，恢复测试前
  剪贴板和 Pinyin 激活状态。全仓既有图片测试有生成 PNG 的副作用；测试日志和
  编译产物保存在仓库外临时目录。

### 2026-09-22 左右修饰键修复验证

- 无桌面回归新增：X11 双侧/反向释放、原生快照恢复与焦点清理、快照不污染 XIM
  Repeat、按布局组区分 AltGraph、同一原生槽位贡献多个公开修饰位时的最后释放；
  XKeymapEvent LP64 ABI；Windows 消息状态左右合并（含低位 toggle 不等于 down）；
  macOS 四组左右 modifier 的 Down/Up；GUI 通过 Window.DispatchEvent 验证普通
  快捷键仍不区分左右。
- 本机 Linux amd64/X11/Xfwm4：`tests/window/platform/modifiers` 用 XTest 原生输入
  逐事件断言，Ctrl/Shift/Alt 两种释放顺序通过；Ctrl 在其他自有窗口按住再切入、
  失焦后外部释放再切回通过。不创建 Painter，后端和缩放不适用。系统 Win/Super
  类组合本轮不作桌面验收，逻辑测试覆盖其状态。
- 本机首次 Alt 验证失败（US Alt 被额外标为 AltGraph，mods=132）；定位到
  XkbKeysymToModifiers 跨组掩码合并。按组修复后保持 `us,cn` / `,altgr-pinyin`
  原始布局不变，Alt 正反向及最终代码复验通过。测试最初的中途 Ctrl+F8 被 WM
  截获，未计为通过，窗口用例改用普通 S 探测中途修饰状态。
- 本机 F5 回归：Software 1x、Fcitx/XIM，三个 150ms F5、一个 1.2s F5 后
  local-F5=4，长按 F6 repeat=15，按钮焦点 F5 只使 window-F5=1。仅操作自有
  测试窗口，结束后关闭；未修改布局、输入法模式或剪贴板。
- remote-shell 无头运行：Windows 10 19045 amd64 的
  `TestWindowsSidedModifiers`、`TestWindowsLogoKey`、`TestLogicalWindowsLetters`、
  `TestConsumedQueuedKey` 及 GUI Key/Modifiers/Shortcut、UI 快捷键协调通过；
  macOS 15.7.7 arm64 的 `TestMacSidedModifierEvents`、原生映射、逻辑字母及 GUI
  Key/Modifiers/Shortcut 通过。上传测试二进制 SHA-256 与本机一致。本轮没有
  在远端创建或操作窗口，不等于 Windows/AppKit 真机键入验收。
- `go test -json ./...` 通过：39 个包通过、34 个包无测试文件；31 条既有子测试
  skip，主要是显式开关的窗口/GLX/阴影/透明测试，不记为通过。
  `go test -race ./platform/linux/x11 ./platform/linux/libs/xlib ./gui ./ui -run 'Test(X11|KeymapEventABI|Shortcut)' -count=1`
  通过；Windows amd64/macOS arm64 的原生测试目标及 modifiers 窗口程序交叉编译通过。
  `git diff --check` 通过。既有全仓测试仍有短暂窗口和 PNG 产物副作用；新增用例
  不输出仓库内产物，日志/二进制在本机及远端临时目录。

## 验证记录（Command 初始切片）

- Linux amd64：`go test ./...` 通过；
  `go test -race ./gui ./gui/dev ./ui ./platform/events -run 'Test(Text|ExplicitModifier|ModifierNames)' -count=1`
  通过。枚举别名/独立位、协议解析与正常事件分发下的文本全选修饰键均有断言。
- 交叉编译：Windows amd64 的 GUI/UI 测试目标、macOS arm64 的 Cocoa/GUI/UI
  测试目标均通过。新增 Cocoa 纯映射测试覆盖独立修饰位及左右 Command/Option/
  Control，但本轮未在 macOS 运行这些测试，也未进行真实窗口验收。
- Overlay 例程的 Ctrl/Cmd+F 及事件日志格式同步更新；未修改 Windows/Linux 的
  Super 原生映射。新增 KeyCommand 位于枚举末尾，既有整数值保持不变。
- `git diff --check` 通过。全仓既有图片测试仍有 PNG 输出副作用；交叉编译产物
  位于仓库外 `/tmp/goui-command-keys.LJKE9p/`，未新增源码目录产物。
- 待真机验证：macOS 左右 Command 的按下/抬起，Ctrl 与 Command 区分，
  Option 词导航、Cmd+A/C/V/Z 及 IME 组合；Windows/Linux Super 不触发 Ctrl 编辑命令。

## Win / Option 独立化验收

新增枚举独立性、协议名称解析、日志名称、Windows 左右徽标键映射和按下/抬起状态测试。
现有 Cocoa 测试验证 Option 的独立值；GUI 编辑器测试验证 Win/Option 不误触发全选，
并保留 Alt/Option 对编辑命令的抑制。Windows/Linux 原生 Alt 与 Linux Super 映射不变。
真实桌面仍需验证 Windows Win、macOS Option 的日志、按下/抬起和按词导航；
系统可能截获徽标键组合，框架不承诺收到系统保留的按键。

验证结果：Linux amd64 `go test ./...` 通过；GUI 编辑命令、gui/dev 名称解析、
events 枚举独立性与 platform/test 日志名称的定向 race 测试通过。
Windows amd64 的 win32/GUI、macOS arm64 的 Cocoa/GUI 测试目标交叉编译通过，
产物位于仓库外临时目录；不代表 Windows/macOS 原生运行通过。
`git diff --check` 通过。全仓既有图片测试仍有 PNG 输出副作用；本轮未执行真实窗口验收。
