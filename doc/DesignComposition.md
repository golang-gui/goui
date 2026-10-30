# 组合声明与对象声明

任意 View 返回值的通用修饰使用 `ui.Modify(view)`，契约见 [显式修饰设计](DesignViewModifiers.md)。

本设计已实现（2026-10-26），替代此前 ui.BindGroup 的隐式 Root 命名组。
平台验收范围和限制见 Plan；MenuButton 的菜单分组行为仍属于独立待实施功能。

## 契约

View 只保留声明身份。ComposeView.Build(BuildContext) View 构建单个内容声明，
WidgetView 维持 Mount／Update／Unmount；两种构建方式互斥。ObjectView 在
ComposeView 基础上增加 Object(BuildContext) any 和 Release(BuildContext)。
Object 是无副作用访问，不能在查询时创建对象。Release 清理声明自己拥有的
资源，不自动 Close／Destroy 外部传入或借用的对象。

ComposeBase 提供 ID、显式 Observe 和封闭接口支持；Widget 的 ViewBase 保留原有样式、
尺寸、焦点等修饰器。组合声明的 ID 不转发到其输出 Widget；布局和绘制属性
不从组合声明隐式传给下层 Widget。WidgetView 无须再提供 return self 的 Build。

组合声明在协调树中保留节点、State、父节点和内容节点；同一挂载目标内用
ID／具体声明类型复用，无 ID 按声明位置。nil 内容保留组合自身的 State 和
对象。nil View 不生成节点；WidgetView.Mount 返回 nil 保持既有跳过行为。
ObjectView 即使没有 Widget 输出仍有独立生命周期。首阶段没有多输出 Fragment。

## Widget 输出与挂载

每个节点有零或一个 Widget 输出。WidgetView 输出自己的 Widget；组合节点
输出其内容的 Widget。实际 Bin／Container 只接收这些输出，不创建占位 Widget。
内容改变时比较旧／新输出，更新实际挂载目标，保留组合自身的身份和对象。
UpdateChildren 的返回切片仍与输入索引对应，没有输出的位置返回 nil；协调
位置包含所有实际声明节点，视觉列表跳过无输出节点。排序不重新 Mount。

Root 负责通用节点协调，不保存组表、组工厂或组成员计数。对象创建由具体
ObjectView 的首次 Build 完成，通常保存在自己的 State。每次 Build 都显式执行，
沿用更新队列，通过 Observe 显式订阅来源，或 BuildContext.RequestUpdate 请求当前节点；
不推断 Get 依赖，不建立资源注入系统。

## 对象关联

`FindAncestor[T](ctx, id)` 只沿声明祖先查找最近的同名节点；匹配节点必须实现
ObjectView 且 Object 的结果兼容 T，否则返回 false，不越过同名节点继续
猜测外层对象。Object 接收其所属节点的上下文，不暴露其他节点的私有 State。
空 ID、失效上下文与释放中的祖先不可查询；外部 Object 回调销毁或移交节点
后须重新核对生命周期，不能提交过期结果。

查询既不创建对象也不建立订阅。首阶段没有兄弟对象、跨窗口全局对象查询。
对象可以是任意 Go 类型，关联范围仍有明确限制。调用方只借用对象；成员在
Unmount 中解除关联，所有后代释放后再执行所属 ObjectView.Release。

```go
wui.CheckButtonGroup(
    ui.HBox(
        wui.CheckButton("Light"),
        wui.CheckButton("Dark"),
        ui.Button("Reset"),
    ),
)
```

CheckButtonGroup 持有稳定的 widgets.CheckButtonGroup；成员默认通过
FindAncestor[T](ctx) 使用最近的兼容对象作用域。显式 Group(shared) 覆盖该默认值，
Group(nil) 保持独立。FindAncestor[T](ctx, id) 仍保留精确 ID 和同名遮蔽语义，
供其他显式对象声明使用；不引入全局组表。详见 DesignButtonGroup.md。

## 释放、回调与移交

先清理后代的声明连接，再释放对象声明；Window 销毁仍保留 GUI 树给窗口
最终销毁，不在 UI 清理阶段擅自拆除。正常移除沿用挂载目标的 detach 约定。
释放不可重入两次，资源回调期间销毁父节点不得复活正在构建的节点。

显式 TransferChild 以真实 Widget 定位直接所属挂载槽位，移交整个声明子树，
包括不产生 Widget 的组合／对象祖先。不能只截取最里面的 Widget 节点。
最终声明归属建立后，通用 ReparentedView 通知相关绑定刷新祖先关联；不重放
全部 Update，以免旧声明覆盖刚编辑的文本或选择。通知先于 committed，既有
AfterTransfer 通知仍在 committed 之后。回滚保留原声明身份；整组移动保留
对象实例；单个成员移动解析目标祖先。转移过程不调用 ObjectView.Release。

## 参考与取舍

2026-10-26 查阅的官方契约：Flutter
[ComponentElement](https://api.flutter.dev/flutter/widgets/ComponentElement-class.html)
区分组合元素与直接创建渲染对象的元素；[State](https://api.flutter.dev/flutter/widgets/State-class.html)
区分稳定状态与多次构建。Qt Quick
[对象父级与视觉父级](https://doc.qt.io/qt-6/qtquick-visualcanvas-visualparent.html)
也明确二者不必一致。采用声明树／Widget 树的区别，不照搬 QObject、Element、
属性系统、自动依赖或平台对象模型。上述为文档证据，不是 GOUI 的跨平台验收。

## 验证范围

包内回归覆盖组合身份／State、空内容、输出替换、排序、位置和 ID、祖先查询
与同名遮蔽、不同对象类型、释放顺序及回调销毁、显式移交和回滚、GUI 树与
快照无包装节点。保留并调整原协调测试，契约改变的断言注明原因。
现有 CheckButton 独立窗口验证改成显式组，复验真实键盘／鼠标、主题、重建
和回调销毁；SourceView 及自定义 WidgetView 使用方同步删除无意义的 Build。
全仓测试、相关 race／vet、Windows／macOS 编译与原生桌面结果分别记录。

## 显式局部更新（2026-10-28）

属性使用 `BindText`／`BindSelection`；计算或结构使用
`ui.Build(fn)`，自定义 ComposeView 同样自动收集同步执行中的 ui.State.Get／ui.Read。
Observe 仍可显式补充依赖，Peek 不添加自动依赖。
`BuildContext.RequestUpdate` 仅请求当前节点；`ui.BindValue` 给自定义 View 注册属性。
State.Get 在构建范围外保持快照语义，Set/Update 不自动全应用更新；窗口声明变更仍调用
App.RequestUpdate。挂载槽回调处理局部输出替换，不增加具体控件特例。
详见 [UI 局部更新与属性绑定](DesignUIUpdates.md)。
