# UI 的 Target、Group 与 Scope

本设计区分三类关系：Target 表达有方向的目标关联，Group 表达成员共同参与的
行为或外观，Scope 表达定义的查找、遮蔽与生效范围。实现进度及验收结果见 [Plan](Plan.md)。

| 类型 | 入口 | 归属 |
| --- | --- | --- |
| Target | 消费者声明 `.Target(id)` | UI 协调层维护持久引用，具体 View 应用业务连接 |
| Group | 显式声明，范围内默认加入，按需 `.Group(object)` | 具体组维护行为，UI 适配成员来源 |
| Scope | 宿主默认范围及显式局部声明 | 具体能力维护定义、查找及生命周期 |

不建立通用 GroupID、全局名字中心或所有 Widget 都具备的组属性。

## Target

```go
ui.HBox(
    wui.TableView(model, columns...).ID("orders"),
    ui.ScrollBar(layout.DirectionVertical).Target("orders"),
)
wui.TabBar().Target("pages")
```

具名目标在当前 UI Root 管理的有效 Widget 输出中解析，不跨 Root 查找。
普通容器及组合不建立额外命名空间。使用最终 Widget.ID，不将仅用于协调的
ComposeView.ID 当作目标名；组合投影不重复登记同一 Widget。返回 nil、尚未构建或
被回收的虚拟行没有可用目标。被引用 ID 在当前范围内必须唯一；未被引用的重复
输出 ID 不改变既有局部协调规则。复用组件通过参数或稳定实例前缀区分目标名。

第三方绑定使用通用入口：

```go
func BindTarget[T any](ctx BuildContext, property, id string, apply func(T))
```

property 是 View 内固定关联名，T 为 Widget 的具体指针或能力接口。Build/Update
只登记声明，实际回调在树挂载完成后运行。引用归持久节点所有，不占用 ctx.State。
引用方未更新时登记继续有效；引用方更新但未再次登记时移除关联。目标缺失或
声明清除时传入零值，适配器解除自己的业务连接；释放后不保留待执行任务。

引用声明更新时使用最新 apply 协调配置；仅因其他节点更新而检查引用时，解析
结果相同不重复回调。适配器以实际连接键决定是否重连，例如 ScrollBar 使用
宿主及轴，保留未变化的信号连接。回调不得同步开始 UI 协调或改写 UI 管理的树，
可以配置 GUI 对象及请求后续更新。所有外部回调后重新检查节点与宿主生命周期。

空 ID 或目标暂时不存在是合法未绑定状态。最终输出重名或类型不兼容属于声明
配置错误，报出引用方、关联名、目标 ID 和类型，不按遍历顺序或预期类型挑选目标。
每个 Root 验证本批解析结果后再应用连接，不把更新途中暂时存在的同名输出当作最终冲突。

协调提交顺序：

```text
本批所有受影响树协调、修饰器及实际输出挂载完成
    -> Target 提交
    -> 普通 AfterUpdate
    -> 布局与绘制
```

全树、局部组合、属性更新、延迟 UpdateChild/UpdateChildren 和显式移交使用同一
完成边界。延迟构建即使没有普通 AfterUpdate，也安排内部提交。AfterUpdate 仍为
一次性钩子，不承诺布局完成。初版按受影响 Root 收集当前被引用 ID 的输出，
不建立持久全量 ID 索引；Root 不识别 ScrollBar、TabBar 或第三方控件类型。

旧目标真正释放时使关联失效并解除旧连接。移交的临时脱离不视为销毁，最终归属
确定后刷新双方 Root，再发送完成通知。引用方随移交进入新范围；目标与引用方
一起迁移且连接身份未变时保留业务连接。回调中的销毁、解除及更新使旧任务失效。
宿主最终销毁也须清理关联，不能只依赖 Widget.Destroyed 检查延迟销毁。

自动跟随覆盖 UI 管理的输出变化，不承诺观察绕过协调器的任意 GUI 树修改。
命令式 API 继续使用直接对象和显式绑定。布局尺寸不是引用解析完成的条件，
滚动范围等数据仍由对应 GUI 控件在布局后同步。

## Group

```go
wui.CheckButtonGroup(ui.VBox(
    wui.CheckButton("Name"),
    wui.CheckButton("Time"),
))
// sortGroup is retained by the component, outside repeated builds.
wui.CheckButton("Name").Group(sortGroup)
```

省略 Group 使用最近的同类型组声明；显式对象覆盖默认来源；显式 nil 表示独立。
内层同类型声明建立独立默认组，不同类型组可组合。普通容器和窗口不隐式分组。
范围声明持有稳定组对象；显式对象由调用方持有，组不拥有成员 Widget。

CheckButtonGroup 保留非可视对象及两种成员入口。MenuButtonGroup 保留现有布局及
菜单切换协调。ButtonGroup/ListRowGroup 依据真实相邻子项组织外观，不通过对象
引用跨布局拼接边框；后者继续维护 UniformRows。各组无需机械提供全部组织形式，
MenuButton 的分散成员入口待具体需求再增加。

成员来源在正常更新和显式移交后刷新，真正卸载时解除成员关系。保留既有选择、
通知及重入语义，不因本次分类更改移除选中成员、程序设置或菜单导航的行为。

## Scope

CommandGroup 改名为 CommandScope，仍为无布局输出的 ObjectView。Window/Surface
的 Commands 及快捷键使用宿主默认 CommandScope，应用无须额外包装。显式局部
CommandScope 建立命名空间：当前定义遮蔽外层同名定义，缺失时按既有契约向外
查找，同一范围重复定义报错，缺失引用不可执行。命令 ID 与 Widget ID 独立。

弹出菜单保留明确的来源 Scope，不按原生 Popup 所在位置推断命令范围。命令引用
继续走 Scope 的定义和通知机制，不改为 Widget Target 查找。默认范围由各能力的
自然宿主提供，不是全局单例，也不要求所有 Scope 使用相同继承规则。

## 迁移与验证

| 旧接口 | 新接口 |
| --- | --- |
| ScrollGroup / ScrollGroupBinding | 移除，ScrollBar.Target(id)；宿主不再登记 |
| TabBar(viewID) | TabBar().Target(viewID) |
| 每控件 AfterUpdate + FindWidget | BindTarget |
| CommandGroup / CommandGroupView | CommandScope / CommandScopeView |

ScrollHost 保留为能力接口。TableView 薄转发 BindScrollBar，不暴露内部 ScrollView。
外部绑定不改变内置滚动条 Options、布局、样式或所有权。目标或轴改变结束旧拖动；
未绑定时清除旧范围。公共 API 变化随相应版本发布迁移说明。

逻辑回归覆盖仅目标局部更新且消费者 Update 计数不变、前向引用、缺失后重现、
输出投影、重名与类型错误、Root 隔离、引用移除、延迟构建、移交及回调销毁。
ScrollBar 覆盖多消费者、双轴、换轴、旧源断开及拖动中替换；TabBar 检查内容和
语义树随目标切换。Group/Scope 保留有效覆盖。非可视对象不伪造交互语义节点。
迁移现有滚动窗口用例，保留独立布局、真实输入和状态断言。运行全仓测试、相关
race 检查、交叉编译和本机独立窗口验收；本次不改变平台或绘制后端契约。

## 参考与取舍

- Qt Quick Controls 6.12 [ButtonGroup](https://doc.qt.io/qt-6/qml-qtquick-controls-buttongroup.html)
  文档提供非可视组、布局子项收集及显式对象成员，支持两种入口并存的取舍。
- GTK4 4.24.1 [CheckButton.set_group](https://docs.gtk.org/gtk4/method.CheckButton.set_group.html)
  文档以另一个 CheckButton 建立关系，不是任意宿主 Widget。
- Flutter [ScrollController](https://api.flutter.dev/flutter/widgets/ScrollController-class.html)
  文档说明跨 Build 持有 Controller，并区分 ScrollPosition 的 attach/detach。
  GOUI 借鉴长期关联与短期挂载对象的区分，沿用现有 GUI 绑定，不增加完整 Controller 体系。

以上为公共契约参考；GOUI 的 Root、引用及提交行为由本设计定义，不能据此推断
参考框架对动态同名替换有相同承诺。
