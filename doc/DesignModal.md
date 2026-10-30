# 通用模态与窗口内 Dialog、Toast

> 状态（2026-10-30）：通用 ModalScope 与 OverlayItem.Modal 已实现；Dialog、Toast 及其声明绑定仍为后续切片。
> 采用普通 Overlay 组合，不增加 HostOverlay。验证范围见 [Plan](Plan.md)。

## 1. 分层和边界

`gui.ModalScope` 是附着于 Widget 子树、作用于所属 Root 的输入边界，不绘制、不布局、
不创建窗口、不处理关闭。Root 包括 Window 与 Popover；其它窗口不受影响。
后续 `widgets.Dialog` 计划提供遮罩、内容卡片、关闭请求和可选 Caption 区域；
`widgets.Toast` 计划提供内容、计时和自隐藏。声明层通过 `wui.Toast(&handle)` 建立
展示位置，`ToastHandle.Show(view, duration)` 提交请求。这些组件尚未实现；位置
与顺序继续由 `ui.OverlayItem` 控制。

首版是 Root 级模态，不根据父容器大小推断局部模态。背景 Enabled/Visible 和业务状态
保持不变；程序仍能修改后台数据。窗口拥有的装饰操作与内容输入分开。

## 2. ModalScope 契约

```go
func NewModalScope(widget Widget) *ModalScope
func (s *ModalScope) Active() bool
func (s *ModalScope) Destroy()
```

所有操作在 GUI 线程执行。构造允许尚未挂载；nil、已销毁 Widget 或重复关联属于
编程错误。挂载且自身和祖先可见时参与选择，禁用不撤销模态。多个边界按真实树的
绘制顺序选择最前方，不能按构造或连接顺序抢占。隐藏、卸载自动撤销；重新挂载
重新参与；Widget 销毁自动清理。Destroy 可重复调用，只移除关联，不销毁 Widget。

Root 内部保存候选集合、当前边界、进入前后台焦点、各边界最近焦点和修订号。
树/可见性/顺序变更更新选择，避免每次指针移动遍历整棵树。初始焦点在完整挂载后、
下一次输入前修复，已有有效内部焦点不被覆盖。Active 查询不触发外部焦点回调。

自定义全区域确认层可以直接组合；layer 的测量、背景、卡片和关闭信号由自身实现：

```go
gui.NewModalScope(layer)
overlay.AddOverlay(layer)
overlay.SetOverlayFill(layer, true)
layer.SetVisible(true)
```

隐藏 layer 撤销边界，再次显示重新参与；无需每次展示重新创建 scope。
UI 自定义 View 仍通过 Build／Update 管理同一个 GUI Widget。

### UI 覆盖项声明

```go
func (v *OverlayItemView) Modal(modal bool) *OverlayItemView

ui.Overlay(
    content,
    ui.OverlayItem(confirmationLayer.Visible(showDialog)).Fill(true).Modal(true),
)
```

Modal 默认 false，不放入 ViewBase；Root 级输入约束在覆盖项关系上显式声明，避免
普通 Label／Button 的公共修饰符带来意外的窗口级影响。Visible 属于 Child，Fill
决定布局，Modal 决定输入策略，彼此独立。非铺满覆盖项也可模态；不自动绘制遮罩、
关闭内容或赋予 Caption。false 或下一轮省略 Modal 只移除 UI 创建的边界，保留内容。

每个已输出 Widget 的覆盖项状态持有一个 Scope，普通重建复用；隐藏自动撤销参与，
重新显示保留最近焦点。子输出替换、Build 返回 nil、覆盖项移除或声明释放时清理，
新输出按原槽位声明重新关联。重排沿用 GUI 绘制顺序，窗口销毁只先释放 UI 关联，
不代替 Window 拆除或销毁控件树。沿用 UpdateChildren 的 apply 回调处理局部更新，
不增加 Root／BuildContext 特例、包装 Widget 或 UI 输入状态机。

关联由 UI 覆盖项唯一拥有，不同时对同一个 Child 手动 NewModalScope；重复关联
沿用 GUI 的编程错误契约。未声明 Modal 的覆盖项不干预控件自有的 Scope。

## 3. 输入与焦点

- 先正常命中，再验证目标是否属于活动边界；外部目标阻挡，不重新寻找下方目标。
- 捕获、目标、冒泡路径裁切到边界；所有阶段的窗口内容快捷键及空目标退避均受约束。
- Tab/Shift+Tab 只遍历内部并循环；无候选时消费 Tab，其它按键可交给边界本身。
- Window/Popover.SetFocusedWidget 验证同一 Root、存活、可见、启用、Focusable 和边界。
  nil 可以清空焦点，但后续按键不回退到后台；普通包含焦点状态仍按真实父链维护。
- 首次进入保存后台焦点，切换边界保存其内部焦点；退出顶层恢复下层，全部退出恢复
  后台。目标失效时选当前允许范围内候选或保持 nil。移除中间层不能丢失后台记录。
- 激活时取消后台手势、捕获、失效拖放、Tooltip，并按既有流程解绑 IME。输入法提交
  和 CommitText 不能落到后台编辑器。失去系统焦点/最小化不自动关闭逻辑模态。
- 回调可能隐藏、重排、销毁或替换边界，事件上下文校验修订号，旧路径立即失效；
  当前事件不重新派发给后台，失效抬起不能组成后台点击。不清空全部信号来兜底。

## 4. Popover 与 Chrome

Popover 依据 Anchor 判断所属上下文，并沿锚点链处理跨 Root。后台 Popup 收起，
后台 Anchor 的新请求不接管输入。保留 ModalTarget，增加可选 AnchoredModalTarget
（嵌入 ModalTarget 并提供 Anchor() Widget）；无来源的自定义目标仅在无边界时沿用
原行为。边界内菜单正常接管输入，关闭后返回原边界。

ModalScope 不安装为 Window.SetModalTarget，避免把 Widget 边界误作独立原生表面。
WindowChrome 继续负责真实窗口控制和缩放。被覆盖的后台 HeaderBar 不得命中；
活动遮罩可显式报告 Caption，沿用拖动及双击最大化/恢复语义。Caption 区域不同时
触发遮罩点击关闭。Native Chrome 的支持范围遵循现有能力，不模拟窗口移动。
自绘卡片的内边距和控件间隙也属于卡片，应明确报告 Client；不能仅用 Pick 命中
遮罩 Widget 就报告 Caption，因为同一 Widget.Paint 可能同时绘制遮罩与卡片。

## 5. 普通 Dialog（待实现）

Dialog 为可放入普通 Overlay 的单子 Widget，使用 Fill(true)，自身绘制遮罩和卡片，
内容由正常遍历绘制。模态关联整个 Dialog：命令式按需 NewModalScope(dialog)，
声明式由 OverlayItem.Modal 管理；Dialog 不重复创建边界。通过 Visible 开关展示。
提供 Child、Padding（默认 24 DIP）、BackdropCaption（默认 false）、
CloseOnOutside（默认 false）和 CloseRequest 信号。不预置标题、操作按钮或业务结果。
Escape 在内容/IME 未消费时请求关闭，遮罩完整左键点击可选请求关闭，不自动更改
应用状态。卡片内容测量决定尺寸，父硬约束优先；宽高偏好放 Child 上，大内容可组合
ScrollView。卡片默认居中，沿用结构矩形裁剪，不新增 Painter 能力。

## 6. 普通 Toast 与声明协调（待实现）

GUI Toast 为单子 Widget，Show(content,duration) 每次是新展示，Close 结束展示。
duration<=0 为持续显示，普通内容更新不重启计时。悬停/内部聚焦暂停，隐藏/卸载停止
等待资源，生命周期校验后恢复或清理。新提示替换旧提示，不提供队列。阴影预留在
Widget 外框内，外围位置和顺序由普通 Overlay 决定，不自动处理另一个 Dialog 的状态。

ToastHandle 零值可用，使用后不能复制，一个句柄同时关联一个持久 Toast 声明。
Show(view,duration) error 与 Close 可以跨 goroutine 提交，GUI 操作在主线程完成。
未绑定/已释放返回 ErrToastUnavailable，不保存到未来挂载。nil 内容为参数错误。
每次请求使用内部序号；不比较 View，不要求业务换 ID。旧超时不得清理新请求。
无请求时声明组合不输出 Widget，Show 后协调实际 Toast；超时后请求清空并正常释放
声明子树，避免通用 Visible 属性重放导致复现。普通重建不重复展示或重置计时。
内部复用持久组合、现有局部更新及 UpdateChild，不为 Toast 增加 Root/BuildContext 特例。

## 7. 外观与语义

后续组件由 Basic 提供兜底，Modern 使用中性色卡片、圆角、边框和适量阴影，父控件不改写内容
StyleName。Dialog 的 backdrop/card 和 Toast 表面使用独立样式入口，Padding 是布局
属性。ModalScope 在关联节点 Attributes 提供通用模态信息；Dialog/Toast 各自报告
语义角色与内容，不伪造后台 Disabled，不为装饰添加动作。自动化仍走正常输入路径。

## 8. 验证与交付

包内覆盖完整事件路径、捕获快捷键、焦点验证、空候选、嵌套/中间层移除、重排、
可见性和跨 Root 生命周期、回调销毁、Popover Anchor、Chrome、拖放和语义。
后续 Toast 回归须覆盖重复请求、普通重建、旧超时、解绑和并发提交。窗口程序按 Testing
约定在 tests/window 中记录中文步骤，通过真实输入验证 Tab/快捷键/中文输入/菜单/
Caption 和外观。先在独立 X11 会话验证，再验证远端 Windows/macOS，运行全仓
go test ./... 及风险相关 race/跨目标编译。未验收的平台单独记录，不能视作通过。

## 9. 参考与取舍

- GTK4 4.18 起 [limit-events](https://docs.gtk.org/gtk4/property.Widget.limit-events.html)
  明确限制全局控制器进入模态内容，是传播边界的参考，不等于完整焦点隔离。
- Flutter [FocusScope](https://api.flutter.dev/flutter/widgets/FocusScope-class.html)
  提供焦点范围和最近焦点记录；不采用 Navigator/Route 体系。
- Compose Material3 [Snackbar](https://developer.android.com/develop/ui/compose/components/snackbar)
  采用声明宿主并向持久对象提交请求；GOUI 使用 ToastHandle，不引入协程或隐式追踪。

以上为公开契约参考与 GOUI 的设计选择，具体原生行为以窗口验证记录为准。
