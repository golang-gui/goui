# ScrollView 滚动方向与布局一致性

## 范围与依据

2026-10-07 从 demo 的表单、WrapBox 和嵌套 Stretch 场景提取本切片。
先修正布局契约，再提供普通内容按视口宽度/高度测量的便利用法。
不修改 platform、Painter、Root、BuildContext 或 Scrollable/ListView 接口；
高频 UI 更新与样式动画另行设计，不作未经对照测量的性能承诺。

- [Flutter RenderFlex 布局算法](https://api.flutter.dev/flutter/rendering/RenderFlex-class.html)
  的自然主轴测量与最终分配分离；采用约束思路，不把 LinearBox 改为 FlexBox，
  不增加 shrink、fit 或新的 intrinsic-size API。
- [Avalonia 11.3.0 ScrollContentPresenter 源码](https://github.com/AvaloniaUI/Avalonia/blob/11.3.0/src/Avalonia.Controls/Presenters/ScrollContentPresenter.cs#L354)
  在可滚动轴使用无界约束，在另一轴保留可用尺寸。本切片的单轴普通内容
  使用紧交叉约束明确填充；不复制属性系统、逻辑滚动或锚点机制。

以上是源码/文档参考，不代表对这些框架进行过运行对照。

## 1. LinearLayout 的自然尺寸基线

所有孩子先以主轴无界约束测量自然尺寸，交叉轴仍受 CrossAlign 与可用尺寸
约束。固定孩子保持自然主轴尺寸；正 MainWeight 的孩子再分配剩余空间，并
在最终紧主轴约束下重新测量，保留换行和 baseline 一致性。

旧实现只给加权孩子无界主轴约束。外层 VBox 的固定孩子得到整个父级高度，
其内部 HBox.CrossStretch 再次消费这段高度：独立无窗口回归测量总高为
140 DIP，Arrange 后末尾却到 170 DIP。新回归覆盖行列转置、Padding、
MinHeight、内部权重及最终矩形；不能通过增大 Scrollable.ContentSize 修复。

兼容影响：固定孩子的自然测量不再收到父级的有限主轴上限。需要在主轴获得
有限分配的孩子使用既有 MainWeight 或自身 Min/Max；不增加控件级对齐。
旧测试保留几何断言，仅按新契约修正 fixed child 的测量约束预期。

Button/MenuButton 不是线性容器。它们曾用 LinearLayout 居中单个内容，现改
为 GUI 私有 buttonContentLayout：两轴保留有限上限，内容自然居中，测量
baseline 包含居中偏移。用户仍可设置自定义 LayoutManager；不增加公共
CenterLayout/AlignBox，不把 Bin 特例放入 LinearLayout。按钮的最大宽度换行、
padding、硬约束和 baseline 原有测试必须继续通过。

## 2. 公共 API

```go
type ScrollDirection uint8

const (
    ScrollBoth ScrollDirection = iota
    ScrollVertical
    ScrollHorizontal
)

func (*ScrollView) Direction() ScrollDirection
func (*ScrollView) SetDirection(ScrollDirection)
```

GUI 默认 ScrollBoth；UI 公开类型别名和同名常量，提供链式 Direction。
声明移除恢复默认并复用 Widget；不引入 Root/BuildContext 特例。
非法值归一为 ScrollBoth，重复设置不请求布局。

| 配置 | 普通内容测量 | 宿主交互 |
| --- | --- | --- |
| ScrollBoth | 双轴自然尺寸，保留既有默认 | 两轴可滚动 |
| ScrollVertical | 有限视口宽度为紧约束，高度无界 | 只允许 Y |
| ScrollHorizontal | 有限视口高度为紧约束，宽度无界 | 只允许 X |

交叉轴无界时取得自然尺寸，不假设 640 DIP 等默认视口尺寸；Arrange 得到
最终视口后重新测量。短内容仅填充交叉轴，不强制沿滚动轴铺满。
ScrollView 自身 Min/Max 先参与测量约束，硬父约束优先；普通内容的交叉轴
Min/Max 不能突破宿主紧约束。ScrollBoth 沿用既有纵向 viewport 的固有尺寸
规则，不借本切片重设计默认值。Direction 不改变 MainWeight，不替代父级分配。

Scrollable 仍收到最终 viewport/offset，负责虚拟化和换行。Direction 仅限制
宿主的滚动轴、偏移和滚动条，不篡改 ContentSize，不要求已有虚拟列表或编辑器
因关闭横向滚动自动换行。

```go
ui.ScrollView(
    ui.VBox(/* 普通表单与 HWrap */).CrossAlign(layout.CrossStretch),
).Direction(ui.ScrollVertical)
```

## 3. 滚动条与交互

两条滚动条集中解析：从不显示开始，按内容范围与扣除滚动条后的区域单调
增加所需滚动条，两轴最多三次判断。每次分配重新开始，内容缩短或窗口变大
不会因旧滚动条保留多余占位。区域宽高至少为零，极小视口的结构矩形不得
出现负尺寸或伸出宿主。

明确回归：100×100 视口包含 96×200 内容，竖条挤出横向溢出；最终两条
均可见，内容区 92×92，MaxScrollX=4、MaxScrollY=108。水平诱发垂直溢出
同样覆盖。旧测试中内容恰好占满另一轴的滚动范围因此增加 8 DIP，这是占位
错误修正，不是容差调整或删除覆盖。

普通单轴内容在完整交叉尺寸测量，再按滚动条缩减的交叉尺寸重测，保证常规
换行内容使用最终宽度/高度。虚拟内容在 LayoutVisible 后刷新范围，视口或
偏移变化时进行有界重新布局；不展开全部项目，不增加接口，不无限循环。

滚动条、viewport clip、偏移钳制、Snapshot、可见请求与拖放自动滚动共享
区域/范围。关闭轴的偏移立即归零，setter、滚轮、ScrollIntoView 不得重新
设置；嵌套拖放按轴寻找可用宿主，不让禁用轴阻挡外层滚动。
Snapshot 保留 RoleScrollView、偏移、范围和内容子树；禁用轴最大范围为零，
不向 WidgetInfo 添加控件专属字段。

### 嵌套滚轮消费（2026-10-10）

滚轮沿既有命中路径由内向外冒泡。ScrollView 先按原规则处理 Shift 横向转换，
再用共享 scrollRange 判断请求轴：任一非零增量对应的轴有范围，便调用
EventContext.StopPropagation 并滚动；否则保持传播，允许可滚动的祖先响应。
一次双轴事件作为整体消费，不拆分或重新派发剩余增量。范围包括方向设置和
滚动条占位，不用本次偏移是否改变判断是否消费；到达边界、小数增量经整数
DIP 舍入后未移动，仍不带动外层。零增量、空内容或请求轴无范围则不消费。

参考 [GTK 4.18.6 GtkScrolledWindow 滚轮处理源码](https://github.com/GNOME/gtk/blob/4.18.6/gtk/gtkscrolledwindow.c#L1322)：
两轴均不可滚动时传播，否则消费，不以单次移动量决定。GOUI 采用该职责划分，
但进一步按当前事件的请求轴判断，保留竖滚轮经过仅横向滚动容器时交给外层
的行为；不复制其平滑滚动、overshoot 或 adjustment 系统。
WheelEventController 仍是被动信号控制器，由使用方决定消费；EventDispatcher
继续仅负责通用传播。TabBar 在实际标签/箭头有溢出范围时采用同样的消费原则，
栏和 viewport 空白区不增加控制器。

包内正常窗口/派发器路径回归先在旧实现复现内外层同时滚动，再验证首尾、
横向/Shift、双轴、单轴方向限制、短/空内容及小数/零增量。
窗口例程在既有 Nested Stretch 面板中加入独立内层 ScrollView；F10 定位、
F4 切换长短，F6 输出外层与内层偏移/范围及窗口局部 bounds，便于真实输入核对。

本轮验证：

- 正常窗口/派发器路径的 16 组 ScrollView、9 组 TabBar 回归：旧实现失败，修正后通过。
  无 DISPLAY 的 GUI/widgets 包测试、相关 GUI/widgets/UI race 通过。
- `DISPLAY=:89 XMODIFIERS=@im=none go test -json -count=1 ./...`：2245 个测试/子测试通过，
  14 个既有条件跳过。保留原有窗口和源码目录 PNG 输出，不把跳过算作桌面验收。
- Windows amd64/macOS arm64 的 GUI/widgets 测试目标与窗口例程编译通过；不是原生运行结果。
- 本机 Linux amd64，任务专用 Xvnc 1.12.0、Xfwm4 4.16.1 合成器，实际 Software/OpenGL
  （llvmpipe）各应用 1x/2x。真实 XTest 滚轮经平台事件进入窗口：内层移动、滚到末尾、
  末尾继续、滚回开头、开头继续，外层始终不动；短内层将滚轮交给外层，鼠标移到外层
  普通内容后仍正常滚动。四组各 9 次公开偏移检查通过，不使用 DevServer。
  窗口例程 F6 另检查双条占位、横向范围和面板高度；TabBar 消费本轮由包内测试覆盖。
- 普通 `go vet ./gui ./widgets ./tests/window/gui/scroll_layout` 因未修改的文本编辑/
  Popover 等文件中既有 unkeyed struct literal 警告失败；`-composites=false` 其余检查通过。
  不删除断言、不更改既有测试或跳过条件来掩盖结果。

未执行：Windows/macOS 原生桌面、Direct2D、真实设备 HiDPI/触控板惯性输入及其他 WM。
两项拖出反馈按用户确认保留原生 DnD 与系统位置约束，本轮不修改或重复验收平台拖放。
日志和截图存于仓库外 `/tmp/goui-nested-wheel.NrMT1M`；仅使用任务专用桌面。

## 4. 例程与验证

demo 表单直接用 Direction(ScrollVertical)，删除 flowWidget/flowView 和
640 DIP 回退，保留背景 Bin，不再自行适配滚动。

包内测试覆盖嵌套 Stretch 行列回归、按钮换行/baseline、双条相互占位、
缩放/内容缩短、有限/无界交叉约束、自身最大尺寸、零/极小分配、虚拟内容
首次范围刷新、方向切换、正常输入路径的滚轮、可见请求、嵌套拖放及 UI 默认
恢复。旧覆盖不迁移、不删除。

窗口验收为 `tests/window/gui/scroll_layout`。中文入口说明列出操作及预期，
F6 仅检查公开几何与 Snapshot，输入仍走正常事件路径，不使用 DevServer。
demo 同时验证 UI 组合的表单、WrapBox 与页内滚动。

### 执行记录（2026-10-07）

环境：Linux amd64、Go 1.25.8；独立 Xvnc 1.12.0 / Xfwm4 4.16.1 桌面，
不操作用户会话。所有产物放 `/tmp/goui-scroll-validation.JF8laC`。

- 通过：`env -u DISPLAY go test ./layout ./gui ./ui ./widgets ./widgets/ui ./example/demo -count=1`。
- 通过：`DISPLAY=:84 XMODIFIERS=@im=none go test -count=1 -json ./...`，
  2087 项测试/子测试通过、14 个既有跳过、0 失败；不是缓存结果。
- 通过：六个相关包的无 DISPLAY `go test -race`。
- 完整相关包 `go vet` 失败：既有 text editor/selection 与 Popover 等未命名
  字段初始化报警，涉及文件未由本轮修改。不顺带改写用户测试；
  `go vet -composites=false` 同范围通过，不能将其表述为完整 vet 通过。
- 通过：Windows amd64、macOS arm64 `CGO_ENABLED=0 go build ./...`；
  demo 与窗口程序亦单独交叉编译。未执行远端原生运行。
- Linux arm64 全仓编译失败：既有 `platform/linux/libs/libc/syscall_linux.go`
  引用 `syscall.SYS_PIPE`、`syscall.SYS_POLL`，该架构未定义；这两处已存在于
  HEAD，文件无本轮改动。不属于滚动实现故障，但该目标不能记作编译通过。
- 本机专项窗口通过：实际 `*software.imageResource` / `*opengl.imageResource`，
  每种应用缩放覆盖 1x、2x。四组各 12 次 F6 几何/输入检查，共 48 次。
  鼠标拖竖条到底与回顶、双条拖到 X=4/Y=108、普通竖滚轮不移动水平样例、
  Shift+滚轮横移、长短切换、620×560 DIP 窄窗、最大化/恢复、亮暗切换与
  正常退出均通过。Screenshot 确认 END 可见、换行与胶囊滑块，无 DevServer。
- demo 复验：Software 配置 1x、OpenGL 配置 2x 的初始页、布局页、820×600
  DIP 窄窗、HWrap 折行与滚动；Software 1x 系统页末尾完整可达。只验收这些
  场景，不把页面切换当作全部控件功能验收，不主动打开外部应用或文件对话框。

1x/2x 是应用覆盖值，非混合 DPI 或真实 Retina。未执行 Windows/macOS 桌面、
硬件 OpenGL、其余后端矩阵与性能对照；这些不作为本机通过项。
