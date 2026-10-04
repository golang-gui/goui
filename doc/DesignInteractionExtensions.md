# 上下文菜单、树行组合与拖放滚动

> 2026-10-04：已确认设计，按本文分阶段实现。本机通过后进行 Windows、macOS
> 原生验证；验证完成后按功能分批提交。本文记录本轮契约，避免上下文丢失。

## 1. 范围与分层

| 切片 | 所属层 | 范围 |
| --- | --- | --- |
| 上下文菜单触发 | gui / ui | 通用 EventController、输入分类与声明绑定 |
| 树行组合 | widgets / widgets/ui | 公开 TreeExpander、整行委托与 RowAt |
| 拖放观察 | gui | 独立观察控制器、共享 rootBase 命中路径 |
| 拖放自动滚动 | gui / ui | ScrollView 行为、Timer、布局后的再次命中 |
| Label 换行 | ui | 绑定既有 gui.Label.SetWrapMode |

业务选择、菜单内容、树节点移动、放置规则和悬停展开仍由控件或应用决定。
不扩展 ListView 的模型、测量或选择职责，不引入新平台 API。

参考：Qt 6.12.0 在线 [QContextMenuEvent](https://doc.qt.io/qt-6/qcontextmenuevent.html)
公开文档区分鼠标和键盘，并采用 Windows 抬起、其他桌面按下触发；
GTK4 4.23.4 在线 [TreeExpander](https://docs.gtk.org/gtk4/class.TreeExpander.html) 公开组合树行；
[DropControllerMotion](https://docs.gtk.org/gtk4/class.DropControllerMotion.html)
观察拖放但不接受放下；[GTK 4.20.0 gtklistbase.c](https://github.com/GNOME/gtk/blob/4.20.0/gtk/gtklistbase.c#L1860)
使用独立拖放观察更新自动滚动。GOUI 采用其职责划分，不复制对象、属性或事件体系。
上述是设计依据；本轮的原生实测结果记录在 §7。

## 2. ContextMenuEventController

```go
func NewContextMenuEventController() *ContextMenuEventController
func (*ContextMenuEventController) ConnectRequest(func(EventContext)) signal.Handle
```

默认冒泡。Position 有效为鼠标请求，坐标为控制器所属 Widget 局部 DIP；
键盘请求没有位置，由控件用当前项、选区或插入点定位。处理者调用
StopPropagation；无人处理则继续向父级传播，不检查监听数量。

Windows 右键抬起请求，Linux/macOS 右键按下请求；macOS Control+左键属于
上下文操作，不触发普通左键点击或 DragSource。菜单键、Shift+F10 沿焦点路径
请求，忽略重复按键。失焦、取消、卸载时清除待处理序列。GUI 内部集中分类，
platform 保持原始事件。菜单键原枚举缺失，本轮仅追加 KeyMenu 并映射
Windows VK_APPS、X11 XK_Menu；不新增窗口能力或原生调用。macOS 无相应
标准物理键，不模拟上报。回调后检查销毁和连接有效期。

TreeView、TabBar 和文本编辑保留现有菜单模型、选择策略、Popover 生命周期和
错误反馈，改用公共控制器。ui.ViewBase.OnContextMenu 是控制器薄绑定，使用
既有协调扩展；不自动替所有 Widget 创建菜单。

## 3. TreeExpander 与整行内容

```go
func NewTreeExpander() *TreeExpander
func (*TreeExpander) SetChild(gui.Widget)
func (*TreeExpander) Child() gui.Widget
func (*TreeExpander) SetDepth(int)
func (*TreeExpander) SetIndentation(float32)
func (*TreeExpander) SetExpandable(bool)
func (*TreeExpander) SetExpanded(bool)
func (*TreeExpander) ConnectToggle(func()) signal.Handle
func (*TreeView) RowAt(geometry.Point) (TreeRow, geometry.Rectangle, bool)
```

TreeExpander 是普通 Bin，负责缩进、展开按钮与内容布局；不持有 TreeModel 或
TreeView。Toggle 表达用户请求，SetExpanded 同步实际状态，不产生 Toggle。
TreeView 内部行容器继续负责选择、当前项、通用行输入和树项 Snapshot。

TreeItemDelegate 的方法签名保留，但 Setup 返回整行可定制内容：调用方可以
用 TreeExpander 组合内容，或在其外部包装自绘反馈。原有内置缩进/箭头迁到
TreeExpander；此契约变更集中迁移所有相关 GUI 委托与测试。
widgets/ui.TreeView 保持原声明形式，内部自动组合、绑定并连接 TreeExpander。
Unbind 清空当前绑定 ID 和声明内容，结构性的 Toggle 连接由复用行持有，始终读
当前绑定 ID；动态业务连接依旧在 Unbind 解除。内部复用行不公开。

RowAt 输入和矩形为 TreeView 局部 DIP，命中包含缩进、箭头和整行空白。
只查询已布局且当前可见的行，不滚动、加载或创建离屏行；空白/未布局返回 false。
返回完整行矩形，可能部分超出视口；滚动或布局后应重新查询。树节点 ID 不用作
Widget ID。Snapshot 仅内部行报告树项层级，箭头报告真实点击行为。

外部 widgets 的 Bin 也需要内部结构子节点。WidgetBase.AddStructuralChild
作为控件实现 helper 显式挂载结构子节点，沿用既有 parent/生命周期机制；普通
AddChild 的 Bin 槽防护保持。它不建立另一套子树或改变 ListView。

## 4. DragMotionEventController

```go
func NewDragMotionEventController() *DragMotionEventController
func (*DragMotionEventController) ConnectEnter(func(geometry.Point)) signal.Handle
func (*DragMotionEventController) ConnectMotion(func(geometry.Point)) signal.Handle
func (*DragMotionEventController) ConnectLeave(func()) signal.Handle
```

观察所属 Widget 或其子树的拖放指针，坐标为局部 DIP。不读取数据，不改变
Action，不接受 Drop；子级接受不能阻止父级观察。兄弟之间移动不重复祖先
Enter/Leave。会话结束、取消、离开、卸载/销毁必须清理。
Window 和 Popover 复用 rootBase 的命中与会话协调，内部/外部拖放同一路径。
注册观察控制器启用原生拖放事件接入，但不增加虚假的可接受数据格式。
观察范围为同一 root 中 DropTarget 已注册格式的 DragOffer，独立于具体目标是否
接受本次放下；仅安装观察控制器不会扩大原生窗口声明支持的格式。

## 5. ScrollView 自动滚动

```go
func (*ScrollView) SetDragAutoScroll(bool)
func (*ScrollView) DragAutoScroll() bool
// ui.ScrollView(content).DragAutoScroll(true)
```

默认开启，可关闭。仅内容视口边缘触发，距离/速度用 DIP 和实际经过时间计算。
静止指针继续滚动；到边界、离开边缘、取消/结束、卸载后停止。复用 Application
Timer；没有 Application 的逻辑测试直接验证计算与单步推进。
嵌套容器每轴优先最内层仍可沿该方向滚动者，避免同轴双重滚动。

滚动改变布局后，依据仍有效的最后 DragOffer 重新命中并协商目标；合并请求，
避免回调重入，不伪造平台鼠标事件。不在 Drop 数据读取阶段继续滚动。
原生拖放循环期间 Timer 投递是否正常必须三平台窗口验证，不能只凭单元测试。
不自动修改业务树或悬停展开节点。

## 6. Label 与验证

ui.Label.WrapMode 直接协调已有 gui.Label.SetWrapMode，默认保持 WrapNone。
各阶段包内测试覆盖事件传播/取消、行绑定复用/语义、拖放观察与清理、速度和
嵌套滚动、静止指针再次命中、声明协调。真实输入均走正常 DispatchEvent。
窗口程序放 tests/window，中文入口说明写明操作和逐项预期，不依赖 DevServer。
本机先验证，再 Windows/macOS 原生验证；最终 go test ./... 并记录所有缺口。

## 7. 进度与验证记录

- [x] 上下文菜单控制器、控件接入、UI 绑定与 Label.WrapMode。
- [x] TreeExpander、整行委托迁移、RowAt 与 widgets/ui 适配。
- [x] 拖放观察、清理与共享命中路径。
- [x] ScrollView 自动滚动、布局后再次协商及嵌套规则。
- [x] 本机阶段验证及完整测试。
- [x] Windows/macOS 原生验证：下列本轮核心场景，不扩大为所有组合。
- [x] 按既往 Add/Modify/Fix 风格分批提交，仅包含本轮文件，划分见 §8。

### 7.1 2026-10-04 验证结果

包内与编译：

- `go test ./...` 通过；`go test -race ./gui ./ui ./widgets ./widgets/ui` 通过。
  既有显式 opt-in 桌面测试的跳过不算验收，既有图像测试仍按原规则产生文件。
- Windows amd64、macOS arm64：验收程序和 GUI/UI/widgets/widgets/ui 测试目标
  交叉编译通过。上传后核对归档 SHA-256；四包相关逻辑测试原生运行通过，
  widgets 完整包内测试通过，其余包只运行本轮相关测试，不声称远端全仓通过。
- 回归包含菜单按下/抬起与取消、冒泡、键盘定位/重复、声明回调替换/移除、
  树行测量/复用/语义、RowAt 的部分可见行及右侧空白、观察回调中销毁、
  静止滚动后目标更换、原生同步 ActionReply 不被缓存、嵌套每轴优先及小数边界。
- 补充远端 `goui-gui-win.test.exe -test.run=TestTextEditor -test.v` 时，旧指针菜单
  测试仅发送 PointerDown，在 Windows 新触发契约下先出现菜单未打开断言失败，
  后续滚动定位测试解引用空菜单。修正这两处测试为完整 Down/Up 点击，保留
  选区、位置、复用、取消等原断言，并增加菜单已打开防护；Windows/macOS 的
  `TestTextEditor`、`TestTextView`、`TestTextInput` 全组复测通过，不靠 skip 或放宽
  预期绕过。修改后再次运行本机全仓和四包 race 测试通过。

桌面操作均使用正常输入路径，窗口为 Integrated，Modern 亮/暗主题：

| 环境 | 后端 / 缩放 | 本轮实际验证 |
| --- | --- | --- |
| Linux X11 / Xfwm4，x64 | Software / 1x | 鼠标及键盘菜单、静止自动滚动、目标更新与放下、Esc 取消、关闭自动滚动 |
| Linux X11 / Xfwm4，x64 | OpenGL / 1x | 同上主要菜单/滚动/放下/取消流程；未重复 OFF 操作 |
| Windows 10 19045，x64 | Direct2D / 1x | 右键按下无菜单/抬起一次菜单、Shift+F10、静止滚动、目标更新/放下、Esc、OFF、树展开/折叠及亮暗 |
| macOS 15.7.7，arm64，裸程序 | OpenGL / 1x | 右键、Control+左键及 Shift+F10、静止滚动、目标更新/放下、Esc、OFF、树展开/折叠及亮暗 |

两端原生拖放循环期间应用 Timer 持续运行，无须新增平台计时器或拖放 API。
Windows 静止采样 scroll 249→554，macOS 113→555；内部断言验证最终放下行等于
即时反馈行，而不是依赖有 100 ms 更新间隔的状态文字。三端 FAIL 均为 0；
Windows/macOS 正常关闭退出码为 0。

初次 OpenGL 窗口验收失败来自新程序漏掉 `runtime.LockOSThread`；按规范修正
验收入口后重跑通过，未用后端 workaround。Windows SSH stdin 大文件传输多次
停在等待状态，改用仅服务本轮归档的临时传输并核对哈希后完成；已清理本轮
传输进程、交互启动任务和临时桌面连接，不改系统安全设置或用户既有程序。

本轮未执行：原生外部文件管理器拖入、Popover 根自动滚动、2x/混合 DPI、
窗口 resize 中拖放、Windows OpenGL/Software、macOS Software 和其他 Linux WM。
这些是后续补充覆盖，不以逻辑测试或历史结果代替。

## 8. 提交划分

沿用往期简短英文、首词 Add/Modify/Fix、句尾句点的风格：

1. `Add gui context menu events.`：通用控制器、原始菜单键映射、主键分类与编辑器接入/回归。
2. `Add ui context menu support.`：通用声明绑定、菜单键及 Label.WrapMode。
3. `Add widgets tree expander.`：公开树行组合、RowAt、既有树/标签菜单迁移、UI 组合和回归。
4. `Add gui drag motion support.`：拖放观察、rootBase 协调、ScrollView 自动滚动和回归。
5. `Add ui drag auto scroll support.`：自动滚动声明绑定及默认值回归。
6. `Add interaction window tests.`：独立跨平台验收程序与本轮设计/验证记录。

仅纳入本轮明确修改/创建的文件；工作区既有未跟踪内容保留。
