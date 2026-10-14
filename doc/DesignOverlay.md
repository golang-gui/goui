# 窗口内 Overlay

## 1. 定位与边界

Overlay 是 `gui` 层的普通容器，不是 Window 拥有的特殊图层。一个主 Child
决定期望尺寸，多个覆盖节点浮动在它上方。Overlay 可以放在窗口根部、编辑区、
SplitView 的一个分区里，也可以嵌套。

GUI 容器及其 UI 薄适配已实现，不新增 Dialog 类型、通知管理器、对齐枚举、
锚点系统或通用鼠标穿透属性。菜单和 ComboBox 下拉框继续使用原生 Popover。

参考 [GtkOverlay](https://docs.gtk.org/gtk4/class.Overlay.html) 的一个主节点、多覆盖
节点及定位查询思路，但保留 GOUI 现有的约束布局、结构裁剪和命中机制。

## 2. 接口

```go
func NewOverlay() *Overlay

func (o *Overlay) Child() Widget
func (o *Overlay) SetChild(child Widget)

func (o *Overlay) AddOverlay(child Widget)
func (o *Overlay) RemoveOverlay(child Widget)

func (o *Overlay) OverlayFill(child Widget) bool
func (o *Overlay) SetOverlayFill(child Widget, fill bool)

func (o *Overlay) ConnectOverlayPosition(
    child Widget,
    fn func(available, size geometry.Size, position *geometry.Point),
) signal.Handle
```

定位信号针对指定覆盖节点，不要求组件自行过滤其它节点。先 `AddOverlay`，再设置
Fill 或连接信号；未挂载节点、主 Child 和 nil 回调不会建立有效连接。

信号在 Arrange 内、覆盖节点测量之后同步发出：

- `available` 是 Overlay 最终分配的大小，`size` 是覆盖节点的测量结果。
- `position` 初始为 `(0, 0)`，单位为 Overlay 局部 DIP，不含 Overlay 在父容器中的偏移。
- 回调按连接顺序修改坐标，后连接者可以覆盖前面的结果。
- 允许负坐标及超出边界；不自动退避，超出部分沿用普通控件树裁剪。
- NaN 和无穷坐标归零；尺寸不能通过该信号修改。
- 不保留结果指针，不在查询中修改控件树、派发事件或递归布局。
- 连接信号会请求布局；外部定位状态改变，以及阻塞、解除阻塞、断开 handle 后，
  调用 `RequestLayout()` 使其生效。

## 3. 测量与布局

Overlay 的 Measure 只测量可见主 Child，保留其 baseline，并遵循自身
MinSize／MaxSize 及父级约束。覆盖节点不撑大 Overlay；没有可见主 Child 时，
仅由自身尺寸偏好及父级约束确定尺寸。首版不提供让覆盖节点参与测量的开关。

Arrange 使用最终分配区域，而不是之前 Measure 收到的最大可用区域：

1. 主 Child 在 `Tight(available)` 下测量并铺满局部区域。
2. 覆盖节点默认在 `Loose(available)` 下测量，沿用其自身内容测量和尺寸偏好。
3. `Fill=true` 时改用 `Tight(available)`，同时铺满两个方向。
4. 查询坐标，再以测量尺寸和查询位置安排覆盖节点。

Fill 默认 false，仅控制尺寸。Fill=true 时也可以查询坐标，默认仍为原点。
父级硬约束优先：Tight 可以覆盖覆盖节点的 MaxSize，这与现有布局一致。
默认 Loose 是“不强制铺满”，不是强制缩小；具体控件可以自行测量为可用大小。

`AddOverlay(linearBox)` 不改变 LinearBox 的测量和内部布局语义。MainAlign／
CrossAlign 排列它自己的子控件，不负责把整个 LinearBox 定位到 Overlay 中。
MainWeight 由线性父布局解释，Overlay 不使用覆盖节点的 MainWeight。

## 4. 绘制、输入与归属

所有节点都在 WidgetBase 的同一棵真实控件树中。主 Child 始终排在最底层，
覆盖节点按添加顺序绘制；普通 Pick 按相反顺序命中。不增加 ZIndex 或特殊渲染通道。

覆盖节点矩形之外继续命中下层；矩形以内包括 Padding、布局间隙和透明背景，
仍然可以命中该节点。没有控制器或没有处理事件不表示穿透。

因此，按内容大小布局的浮动按钮／查找栏之外自然可交互；铺满的遮罩层自然阻挡
下层鼠标输入。铺满 LinearBox 的空白区域也会阻挡下层，这是现有命中的正常行为。
需要几个彼此分离的浮动按钮时，分别 AddOverlay，而不是用铺满容器制造空白区域。

这些都不等于键盘模态。Overlay 不改变焦点、输入法、键盘传播或已有指针捕获。
Dialog 的焦点恢复、背景快捷键屏蔽等需要单独设计，不能以铺满遮罩代替。
现有面向原生 Popover 的 `Window.SetModalTarget` 也不直接用于窗口内覆盖节点。

节点通过现有挂载／卸载流程管理资源，Window 对仍在树中的节点执行最终销毁。
RemoveOverlay 仅解除挂载、不销毁节点，并丢弃它在该 Overlay 中的 Fill 设置和
定位查询；重新添加从默认设置开始。调用方仍应释放自己持有的 signal handle。
通过其它容器重新挂载节点时，旧 Overlay 在下一次布局清除失效元数据。

重复 AddOverlay 不调整顺序；把主 Child 作为覆盖节点添加无效。
SetChild 可以将已有覆盖节点改为主节点，其覆盖设置随之移除。
通过 RemoveChild 可以移除任一种节点；RemoveOverlay 不移除主节点。
非法父子关系或已销毁节点不被挂载，也不替换原有主节点。

反复显示的查找栏／确认层可以保留挂载，使用 SetVisible 控制显示；隐藏不会丢失
位置查询和 Fill 设置。隐藏节点不测量、不查询位置、不绘制也不参与命中。

## 5. 使用场景

以下片段中的 editor、input、按钮、mask、panel 都是应用构建的普通 Widget，
不是新增的 Overlay 专用类型。

### 5.1 编辑区右上角查找栏

```go
overlay := gui.NewOverlay()
overlay.SetChild(editor)

findBar := gui.NewLinearBox(layout.DirectionHorizontal)
findBar.SetSpacing(4)
findBar.AddChild(input)
findBar.AddChild(previousButton)
findBar.AddChild(nextButton)
findBar.AddChild(closeButton)

overlay.AddOverlay(findBar)
positionHandle := overlay.ConnectOverlayPosition(findBar,
    func(available, size geometry.Size, position *geometry.Point) {
        position.X = max(0, available.Width-size.Width-8)
        position.Y = 8
    },
)
// 由组件生命周期管理 positionHandle。
```

查找栏不挤压 editor，仅自己的矩形参与覆盖命中。显示后由宿主将焦点移到 input；
关闭按钮和 Esc 由查找组件处理。隐藏前若焦点仍在查找栏内部，再恢复到有效目标，
不要无条件抢回已经转移到其它内容的焦点。

### 5.2 关闭确认层

```text
窗口 Overlay
├── Child：编辑区域
├── Overlay：查找栏（内容尺寸）
└── Overlay：确认层（铺满）
    ├── Child：遮罩（铺满确认层）
    └── Overlay：确认面板（内容尺寸、居中）
```

```go
confirmLayer := gui.NewOverlay()
confirmLayer.SetChild(mask)
confirmLayer.AddOverlay(panel)
positionHandle := confirmLayer.ConnectOverlayPosition(panel,
    func(available, size geometry.Size, position *geometry.Point) {
        position.X = (available.Width - size.Width) / 2
        position.Y = (available.Height - size.Height) / 2
    },
)

overlay.AddOverlay(confirmLayer)
overlay.SetOverlayFill(confirmLayer, true)
```

Fill 作用于确认层，不是确认面板。mask 自己绘制遮罩，panel 自己绘制内容和样式。
二者是兄弟节点：面板内命中 panel 的子树，面板外命中 mask，面板点击不沿父链
传播到 mask。关闭确认推荐点击遮罩不关闭，取消按钮／Esc 才取消。

业务关闭流程：

1. CloseRequest 遇到未保存修改时设置 `*allow=false`，显示确认层。
2. 重复关闭请求不重复创建确认层；显示后保存原焦点并选择适当的初始焦点。
3. 取消时隐藏确认层并恢复有效焦点，不关闭窗口。
4. 确认后完成必要操作，用一次性放行标记重新 RequestClose；其它关闭监听者仍可拒绝。
   放行标记在下一次关闭查询时消费，请求失败时也要清除，不直接 Destroy 绕过关闭流程。

这是组合与业务流程说明，不表示本切片已经提供完整模态 Dialog。

## 6. 语义快照

Snapshot 用于理解界面内容、状态及规划输入，不是具体控件类型或布局参数的转储。
Overlay 使用现有 `RoleBox` 表达普通容器，不新增 `RoleOverlay` 或 `WidgetInfo`
字段。查找栏、通知、对话框等用途由具体内容表达，不从 Fill、尺寸或位置推断。

- 复用基础快照的 `Children`：主 Child 若存在则在最前，随后是按添加顺序排列的
  覆盖节点；保留从底到顶的顺序。没有主 Child 时不增加空占位节点。
- 内容保留自己的角色、文字、动作及子树；不再复制一份 Overlays，也不暴露布局元数据。
- Bounds 为窗口局部 DIP 下的布局矩形，不扣除裁剪或遮挡区域。
- 隐藏节点保留在快照中，Visible 报告节点自身的标志，不代表经过祖先和遮挡计算的
  最终可见性。移除节点则从子树中消失。
- 遮罩阻挡指针输入，不意味着下层控件被禁用；不改写其 Enabled、Visible 或 Actions。
  Actions 表达支持的动作，不保证向 Bounds 中某点派发输入就能触发该动作。

语义树允许控件过滤内部结构，不能当作完整的命中测试树。自动化执行仍经过
`Window.DispatchEvent`，由真实输入分发判断命中。后续具体 Dialog 组件才应提供
对话框语义，实际具备键盘模态行为时才报告模态状态；不能将铺满遮罩视为模态。
裁剪、遮挡和可交互区域的进一步表达属于 Snapshot 的通用规划，不在 Overlay 中特判。

## 7. UI 绑定

公开声明与 MenuItem 一样使用链式描述，不混合公开配置结构体：

```go
func Overlay(child View, items ...*OverlayItemView) *OverlayView
func (v *OverlayView) Child(child View) *OverlayView
func (v *OverlayView) Overlays(items ...*OverlayItemView) *OverlayView

func OverlayItem(child View) *OverlayItemView
func (v *OverlayItemView) Fill(fill bool) *OverlayItemView
func (v *OverlayItemView) OnPosition(
    fn func(available, size geometry.Size, position *geometry.Point),
) *OverlayItemView
```

OverlayItemView 仅描述子 View 与 Overlay 的布局关系，不实现 View，不嵌入 ViewBase，
不创建额外 Widget。Visible、Style、MinSize／MaxSize 等修饰符属于内部 Child。
Fill 默认 false；OnPosition 替换回调，nil 或下一轮未设置时恢复原点定位。
Overlays 替换整个列表，不累积添加。nil 项、nil Child 和 Build 返回 nil 的内容不生成节点。

```go
ui.Overlay(
    editor,
    ui.OverlayItem(
        ui.HBox(searchInput, previousButton, nextButton, closeButton).
            Visible(showFind),
    ).OnPosition(func(available, size geometry.Size, p *geometry.Point) {
        p.X = available.Width - size.Width - 8
        p.Y = 8
    }),
    ui.OverlayItem(confirmationLayer.Visible(showConfirmation)).Fill(true),
)
```

confirmationLayer 可以继续使用 `ui.Overlay(mask, ui.OverlayItem(panel).OnPosition(...))`
组合遮罩和居中面板。UI 不添加点击外部关闭、键盘模态或焦点恢复策略。

协调约定：

- 主 Child 与覆盖列表分别协调，主节点增删／换类型不影响覆盖列表的身份。
- 覆盖列表沿用通用挂载目标内协调：非空 ID 与具体 View 类型匹配，支持同目标重排；
  无 ID 时使用位置／类型回退。Name 和描述对象指针不是协调身份，见 [WidgetIdentity](DesignWidgetIdentity.md)。
- nil 项不占槽位；无 ID 的动态插入／删除可能改变后续身份。需要持续复用的内容使用
  稳定 ID，或保留声明并切换 Child 的 Visible；不要依赖临时描述对象的地址。
- 主节点通过 `ctx.UpdateChild(overlay, child)` 协调，覆盖列表通过
  `ctx.UpdateChildren(floating, children)` 协调。floating 是 Mount 时创建的私有
  `ui.Container` 适配器，不是 Widget；返回结果按声明索引应用 Fill 和定位回调。
- 使用 [UI 挂载目标接口](DesignUI.md#21-子节点挂载目标已实现)，由同一 Root
  统一管理各目标的节点生命周期。不增加独立 Root、包装 Widget 或控件专属协调分支。
- 每个覆盖节点只连接一次 GUI 定位信号，重建时刷新回调和 Fill，移除／卸载时断开。
  更新后请求布局；窗口尺寸变化会在 GUI Arrange 中重新查询定位，无须 UI 重建。
- 定位查询仍受 GUI 的同步约束：不保留结果指针，不在回调中改变控件树或同步重建布局。
- 主槽位与覆盖列表有独立、稳定的目标身份，Root 无需识别 Overlay。普通卸载调用
  挂载目标清理 UI 管理的节点；窗口销毁路径仅释放 UI 连接及状态，不先拆走 GUI 控件树。
- Snapshot、测量、绘制、裁剪和命中全部复用 GUI；UI 不引入 platform 依赖。

## 8. 验证

- 主节点决定尺寸与 baseline、空／隐藏主节点、覆盖节点不撑大容器。
- 内容尺寸、MinSize／MaxSize、Fill 的父级硬约束优先，以及窗口缩放后的重新测量定位。
- 每节点信号隔离、连接顺序、断开／阻塞、隐藏保留、移除失效。
- 主节点替换、前后顺序、重新挂载、语义快照、挂载卸载和窗口销毁。
- 快照使用 RoleBox，覆盖无主节点、隐藏／移除及嵌套场景，保留内容角色和绝对边界。
- 遮挡不改变底层按钮的语义状态，但真实点击被遮罩阻挡；隐藏遮罩后同坐标可点击。
- 真实事件分发路径验证浮动控件外命中底层、控件内点击、遮罩阻挡、面板／遮罩分离。
- 普通绘制遍历验证叠放顺序和超出父边界时的裁剪。
- UI 主节点独立协调、覆盖列表局部 ID／类型复用及重排、无 ID 回退、nil 项和组合 View 展开。
- UI Fill／回调更新与重置、隐藏后保留实例、无重建缩放定位、连接复用及单次释放。
- UI 普通卸载与窗口销毁路径，以及嵌套确认层的快照和真实输入分发。
- 运行 `go test ./...`；不修改平台或事件命中机制。
