# 任意 View 的显式修饰

> 2026-10-30：已确认设计，分阶段实现与验证；进度见 Plan。

## API 与范围

`ui.Modify(content View) *ui.ModifiedView` 接收具体、接口类型或组合声明，复用现有
ViewBase 的通用链式修饰。它只增加声明节点，不增加 GUI Widget、布局、绘制、命中或
焦点边界。具体控件仍直接使用 Label(...).MainWeight(1)，View 接口保持最小且封闭。
不公开内部 Base，不添加属性注册表或另一套控件体系。

```go
func details() ui.View { return ui.VBox(ui.Label("Details"), ui.TextInput()) }

ui.HBox(sidebar(), ui.Modify(details()).MainWeight(1).MinWidth(240))
ui.Modify(ui.Build(func() ui.View {
    if loading.Get() { return ui.Label("Loading…") }
    return details()
})).MainWeight(1)
```

只有显式 ModifiedView 可以携带输出修饰；普通 ComposeView 的 ID 不转发，仍拒绝
Widget 修饰。Modify(nil) 保留空声明，不创建 Widget。嵌套 Modify 不折叠身份。
尺寸、布局、显隐、启用、样式、语义、光标、焦点回调、上下文菜单、快捷键及拖放均复用
通用方法；Padding、Text 等具体控件属性不加入。内部复用单内容组合协调机制。

## 输出与覆盖

次序：Widget 构造默认值 → WidgetView 自身通用声明 → 内层 Modify → 外层 Modify。
只覆盖显式字段，最外层优先；宽高分别合并，零、空串、false、nil 可显式覆盖。
同层后调用覆盖先调用。省略某项外层修饰时恢复当前内层声明，无内层声明才恢复初次
Mount 默认值，不能把覆盖后的值当作默认。

Style 只配置最终输出 Widget，不向子 Widget 隐式继承。Enabled 的子树约束由 GUI
处理；Visible(false) 不卸载或暂停状态订阅。收集只沿单输出组合链，遇到实际 Widget
或独立挂载槽停止；组合内 UpdateChild/UpdateChildren 的独立内容不继承外部修饰。

ModifiedView.ID 同时标识自身声明与最终 Widget，覆盖内层 GUI ID，不修改内容的协调 ID。
普通 ComposeView.ID 仍只标识组合。列表身份放最外层，不猜测内部 ID、不另加 Key。
Snapshot 只报告实际 Widget 和最终 ID/状态，不增加占位节点。

OnFocus/OnContextMenu、DragSource/DropTarget 由外层显式配置替换；Shortcuts 替换整组。
不自动串联回调/合并列表。显式 nil/空列表清除内层配置；补齐 presence bit，不改变
普通 View 的单层行为。多个处理者由使用方显式组合。覆盖只针对 UI 通用声明，
不移除控件内部控制器或改变事件传播顺序；例如 TextInput 自带编辑菜单仍可优先消费请求。

## 协调、绑定与归属

实际 WidgetView 节点持有唯一的默认快照、信号与控制器上下文。先合并再统一应用，
不逐层写 Widget，不让包装持有挂载资源。组合边在内容协调前明确，不能依赖尚未安装的
parent.body；独立挂载槽不属于输出链。

内部局部 Build 即使复用同一 Widget 也应用有效配置；类型替换、nil 与恢复沿用外层。
属性绑定不重放 View.Update，执行后只应用外层显式属性；逐轴覆盖保留其他轴当前值，
不恢复无关默认。自定义 BindValue 只保证所属节点输出，任意修改无关 Widget 不在契约内。

连接由 Widget 节点维护，不为每层增加控制器。可能回调的步骤后检查所有者、版本、Root
与 Widget，避免回调销毁/移交后继续访问。旧输出清理连接，新输出建立自己的上下文；
不同挂载不共享可变状态。修饰子树随 TransferChild 移交，不重复 Mount 或连接。

包装只持有内容声明，不改写它；同一内容可由不同包装分别配置。链式赋值是普通 Go 操作，
不承诺线程安全。增删包装改变声明结构，不保证穿透保留状态；动态属性保持包装稳定。
Modify 不增加状态绑定：Get 只有在 ComposeView.Build 同步范围内才追踪，否则为快照。
修改声明不直接更新 Widget，复用 State 通知或 RequestUpdate。

## 开发与验收

阶段一：Modify、通用标记、合并及基础回归。阶段二：绑定、连接、生命周期与移交。
阶段三：tests/window/ui/modifiers、独立桌面验收及记录。只改 ui，不改 gui/platform。
测试集中在已有 view/composition/binding/transfer 测试文件，窗口程序使用中文操作说明。
覆盖接口/自定义 View、嵌套、逐轴/零值、省略恢复、局部更新、类型/nil、共享、ID、
Snapshot、独立挂载边界、连接去重、销毁重入及跨 Root。全仓、race/vet、目标编译与
独立 X11 Software/OpenGL 1x/2x；不以编译冒充 Windows/macOS 原生验收。结果完成后补记。

## 参考与取舍

2026-10-30 查阅 Flutter 官方 [ParentDataWidget](https://api.flutter.dev/flutter/widgets/ParentDataWidget-class.html)
与 [Expanded](https://api.flutter.dev/flutter/widgets/Expanded-class.html)：声明可以为内容提供布局配置。
采用显式组合与输出归属，不照搬 Element/ParentData，不引入布局注册体系；不是原生平台
行为的等价承诺。
