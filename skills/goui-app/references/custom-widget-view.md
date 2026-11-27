# 自定义 Widget 与 View：AI 使用要点

先判断用户需要新增行为、独立类型还是只是复用界面。GUI 的 Widget 是持久对象，
UI 的 View 是本次声明；不要为每段 UI 新建一套状态、事件或协调框架。

## 六种方式

| 用户需要 | 建议 |
| --- | --- |
| 组合 Widget | 普通 Go 工厂组合 LinearBox／Button／Label 等，返回根；需要操作内部控件时由组件对象持有引用 |
| 扩展 Widget | 嵌入已正确初始化的具体控件，添加 setter 或重写少量方法，保留其排版／行为／资源清理 |
| 实现 Widget | 嵌入 gui.WidgetBase，按需实现 Bin／Container、布局策略、Paint、交互与 Snapshot |
| 组合 View | 简单片段用函数；有独立身份／State 时嵌 ComposeBase 实现 ComposeView.Build(ctx) |
| 扩展 View | 需要新增参数或协调行为时，使用自己的 ui.ViewBase[T]，以命名字段复用原 View 并显式委托生命周期；仅共享修饰用 ui.Modify |
| 实现 View | 实现 ui.WidgetView，将公开 setter／signal 协调到 Widget，使用 UpdateChild／UpdateChildren 管理内容 |

## Widget 必须核对的边界

- WidgetBase 通过未导出方法提供接口基础；外部自定义类型可以嵌入它或已有 Widget，
  但不能在应用包凭同名私有方法手写另一套 base。用编译期接口断言检查。
- 初始化已有控件使用其构造函数，不能把零值 Label／Button 当作完整实例。
  Go 嵌入没有虚方法：原方法内部调用自身方法、连接闭包或挂接孩子仍可能使用
  原对象，重写外层方法不会自动改变这些行为。
- 自定义孩子通过自己的 Bin.SetChild／Container.AddChild 接入；实现内调用
  WidgetBase.AddChild(实际外层对象, child)，不要把孩子 Parent 设成错误的内层对象。
  Bin 先更新 Child 槽再挂接，公开 API 与真实 Children 保持一致。
- 优先给 WidgetBase 设置现有 LayoutManager，复用显隐、min／max 和父约束。
  自绘叶子没有自然尺寸，需要明确 MinSize 或布局策略。自己实现 Measure 时
  返回 layout.Measurement，处理自身偏好与父约束；不能调用框架私有 helper。
- Paint 借用同步 Painter，只画自身局部 DIP，子树由 GUI 遍历。变换／clip 不自动
  成为孩子布局属性；背景圆角不等于圆角裁剪。颜色用 graphics.ColorOf 转换，
  不二次预乘；不要在缓存里保留 Painter。
- 自身背景／圆角／边框在 Paint 内显式调用 `gui.PaintWidgetStyle(p, w, "widget")`；
  LinearBox／WrapBox 已支持这些字段，无需再包装自绘容器。复杂 Part 用 WidgetStyle
  配合 PaintStyledBackground／PaintStyledBorder；解析遵循最近 StyleScope，不继承父样式。
- 用 `w.State().Has(style.Pressed | style.Checked)` 查询组合状态，不从 Snapshot 反查。
  WidgetBase.SetState 只供实现方同步 Pressed／Selected／Checked／Mixed／DropActive；
  Hovered／Focused／ContainsFocus／FocusVisible／Disabled 归 GUI 管理。取消或复用时清理
  自己的临时状态；CheckState 等业务值仍通过专门 setter 修改。
- 改尺寸／文字等请求布局；只改外观请求绘制。StyleChanged 清理本控件派生缓存。
  Snapshot 保留基类的子树与状态，补正确 Role／Text／Attributes；不伪造动作，
  不把 Widget／native resource／回调放进 Attributes。
- mount／unmount 管理与宿主相关的资源；拆挂不是最终销毁。Painter.NewImage
  的资源不可跨 Painter 复用，释放后清空引用，重挂按需创建；不忽略创建失败。

## 自绘与动画

- ui.DrawingArea 在离屏 Software Painter 上栅格化并缓存，适合不常变化的内容。
  新的声明式绘制闭包、UI 重建和 RequestPaint 不自动使缓存失效。业务内容变化后，
  通过 OnShouldDraw 判断是否重新生成，例如 `*redraw = *redraw || dirty`，保留首次
  绘制、尺寸或缩放变化给出的初始值；同时请求绘制以触发查询。请求帧与重新生成
  是两件事，OnShouldDraw 本身不调度帧。
  每帧都变化的图表、频谱或转动的图形，实现一个小 Widget，在 Paint 里直接用
  窗口的 Painter 绘制，避免每帧栅格化和上传。
- 持续动画：在 ConnectMount 中连接 Root.ConnectFrame（推进时间并 RequestPaint）
  和 ConnectAfterPaint（仍活跃且 gui.IsDrawable 时再请求下一帧），ConnectUnmount
  中断开。只在活跃期间请求帧；静止时不请求，空闲 CPU 应接近零。
- 数值或曲线按固定周期采样时，用与周期等长的 animation.Linear 过渡；ease-out
  这类提前结束的曲线会让画面每个周期先动后停，看起来一顿一顿。切换主题色、
  启停等一次性变化适合 EaseOutCubic。Animation 需要窗口作为 FrameSource，没有
  窗口时直接 SetValue；所有者停止时调用 Stop。
- DrawBoxShadow 只在控件边界内可见，模糊延伸约 1.5 × BlurRadius 加偏移；
  为阴影预留留白，否则会被裁出直边。大半径模糊的成本随后端不同，先在实际后端
  测量 CPU 再决定是否用渐变代替。
- 圆角背景不裁剪子控件。紧贴父控件圆角的子控件若绘制背景，矩形角会盖住父
  控件的圆角边框；给子控件对应角设置 CornerRadius，或加足够内缩，或改用
  BorderBox（按圆角自动内缩）。
- 自定义输入控件（如进度拖动条）组合 Motion／Click／Drag EventController，
  通过 AddEventController 安装。按业务选择实时更新、结束提交或预览与提交分离：
  音量等可持续更新，进度定位可在结束时提交；取消时清理临时交互状态，并明确是否
  恢复业务值，不为所有控件统一规定提交时机。Snapshot 补 Role 和 Text，让语义
  快照能描述它。

## 自定义文本输入与焦点

- 复用 IMClient／IMContext 接收 Commit 和 Preedit，用局部 DIP 报告光标矩形。
  预编辑不是已提交正文，不把它直接写入新文档、查找框或撤销历史；普通按键与文本
  提交也不能合并成一条路径。只读或可编辑状态在回调中继续校验。
- 已聚焦控件改变输入策略或 IMContext 后，在 GUI 线程调用公开的
  gui.RefreshInputMethod(widget)。重复 SetFocusedWidget 不是刷新绑定的替代，
  不采用反复失焦／聚焦的补偿。上下文可按控件生命周期稳定持有，是否返回它仍由
  输入策略决定；不为绕过绑定刷新而改变只读语义。
- 当前 IMContext.Reset 是取消组合，不是提交当前候选；没有可请求并等待的组合
  完成入口。不要把预编辑拼音当最终文本，或合成空格键猜测候选。业务需要切换目标
  或执行命令时，先确定可实现的组合处理策略并说明差异。
- 当前默认指针聚焦早于 Widget 捕获阶段。StopPropagation 不撤销已经发生的焦点
  变化，OnFocus 也不是可取消的聚焦前通知。需要避免工具栏夺走编辑焦点时使用公开
  FocusOnPointer 策略，并分别验证鼠标、键盘及菜单操作。
- ClickInfo.Count 在按下时可用，属于各控制器自己的序列。接入自定义文本选择手势
  时核对竞争与取消，不能假定独立控制器能共享原生多击计数；优先复用内置文本行为。
- CommitText 只提交已确认文本，不验证真实键盘、原生输入法绑定、预编辑和候选窗。
  自定义文本输入须在实际桌面检查只读切换、焦点切换及 CJK 组合；键位差异先核对
  原生提交与快捷键路径，不通过丢弃合法文本来补偿。

## View 必须核对的边界

- 具体类型嵌入 ui.ViewBase[Concrete]，构造函数设置 `v.Self = v`，通过指针使用。
  WidgetView 只由 Mount、Update、Unmount 提供生命周期，无须 Build 占位方法。
- 组合声明嵌 ui.ComposeBase[T]，实现 Build(ctx)；节点拥有独立 State，返回 nil
  仅清空内容。声明 ID 控制组件身份，不自动转发给输出 Widget；样式／布局
  属性可直接声明在内容上。若只需给组合组件或 ui.View 接口值应用共享修饰，
  使用 ui.Modify(view)，不必为此扩展 View 或增加布局容器。包装声明需保持稳定，
  外层显式修饰覆盖内容对应属性，ID 同时标识包装声明与输出 Widget。
  WidgetView 和 ComposeView 不能同时实现。
- 对象声明实现 ObjectView：Build 初始化／更新 State，Object(ctx) 只读返回
  现有对象，Release(ctx) 在后代清理后释放自己拥有的资源，也须处理构建失败。
  `ui.FindAncestor[T](ctx, id)` 只查询最近的同名对象祖先，不隐式创建或跨窗口搜索。
- CheckButton 默认使用最近的 wui.CheckButtonGroup(child)；显式 .Group(shared)
  使用稳定持有的互斥对象，.Group(nil) 保持独立。不要在每次 Build 中重建组对象。
- `ui.MenuButtonGroup(children...)` 是普通布局容器；后代 MenuButton 自动加入最近的组，嵌套组独立。不要使用 ObjectView 或组名绑定。
  菜单为非模态自动关闭面；通过 OnOpenError 处理输入失败，Open 的错误直接处理。
  自定义非模态 Popover 可显式 DispatchEvent 转发需要的键，不抢占统一键盘目标。
- 直接嵌入 *ui.LabelView 等，链式方法通常仍返回内层类型。若传出这个返回值，
  外层扩展的 Update 等逻辑就丢失。独立链式扩展用自己的 ViewBase 加命名代理，
  不同时嵌两个 base，不强行更换内层 Self 的泛型类型。
- Mount 一次创建 Widget／连接。Update 每次写专属参数与最新回调，框架随后应用
  共享修饰器，不手工重复 SetID／SetStyleName。永久闭包捕获 ctx.State 中的持久
  状态，而非第一次创建的瞬时 View；Unmount 断开连接并释放协调资源。
- 复用原 View 时，一个节点只有一个 State 槽。不能覆盖原 Mount 保存的状态后
  再调用原 Update／Unmount。需要独立状态时，将原 View 当真实受管孩子协调。
- 同挂载目标中的 ID 与声明自身具体类型控制复用；未设 ID 按位置。
  回调、模型和资源是否持久不能靠 View 指针是否相等判断。受管子树不能任意重挂。
- 子树通过 BuildContext.UpdateChild／UpdateChildren，目标是稳定非 nil 指针。
  nil 单子／空列表用于明确清空；没有调用不代表清空。不重复 Mount 每个孩子，
  不自行维护另一套协调树。多子挂载目标实现 MoveChildBefore。
- Unmount 不代窗口最终销毁 Widget；AfterUpdate 在布局前，不是帧结束或已显示。
  外部异步回调通过 App.Post，检查失效的所有者，避免同步嵌套协调。
  单次挂载后的操作可在当次 AfterUpdate 查找最新 Widget；长期关联用 Target，
  绑定归零时释放自己建立的连接。细节见 [有状态工作区](stateful-apps.md#持久关联与挂载后的操作)。

## 获取可编译示例

完整六种方式与生命周期示例在 GOUI 的
[CustomWidgetView.md](https://github.com/golang-gui/goui/blob/main/doc/CustomWidgetView.md)。
已有项目优先使用 `go list -m -json github.com/golang-gui/goui` 选中模块 Dir／Replace
中的同版本 `doc/CustomWidgetView.md`；源码没有这份文档时，可读官方说明，但
所有使用接口仍须对照依赖版本。

按需查看该版本 ui/view.go、ui/root.go、ui/button.go、ui/box.go、gui/widget.go，
示例实际路径为 example/ui_test/main.go 和 example/demo/surface.go。不要把旧例程
中忽略错误或过时资源假设当作生产契约；不要为读文档安装整个框架到 Skill 目录。
本参考不使用跳出 Skill 目录的相对链接，独立安装后仍可阅读。

实现后验证外部包可实现接口、共享链式方法返回正确类型、父链／测量／Snapshot，
以及回调更新与释放。测试上下文的手动调用不等于完整 UI 协调或真实桌面通过；
不要为测试导出私有状态。编译、逻辑和窗口验证分开报告。
