# 声明式局部更新与状态绑定

> 2026-10-28：本切片已实现，迁移与验收范围见 [Plan](Plan.md)。本篇是实现契约，不代表已发布版本。

## 使用入口

```go
ui.Label("static text")
ui.Label().BindText(&text)
ui.TextInput().BindText(&text)
ui.Build(func() ui.View {
    if !visible.Get() { return nil }
    return ui.Label(title.Get())
}).Observe(&visible, &title)
```

Label 接收零或一个文本参数，多于一个 panic，和现有 Button 一致。Text 与 BindText 后调用者决定来源。
Build 返回普通 ComposeView 声明，不立即调用函数，不创建包装 Widget。闭包不接收 BuildContext；函数按普通 Go 捕获规则读取数据，不自动追踪 Get。Observe 在 ComposeBase 上，替换显式来源列表；空列表清除，重复来源去重。构建函数为 nil 是配置错误。nil 输出保留组合身份和订阅。

State 只存值并发通知，不查找当前 App。Get 不记录依赖；每次 Set/Update 仍通知，包括同值。未订阅的 State 不触发 UI。状态在构建函数外持有，或由自定义组合的私有持久状态持有，使用后不可复制。

## 来源与绑定

```go
type Observable interface { Connect(func()) signal.Handle }
type ReadState[T any] interface { Observable; Get() T }
type BindState[T any] interface { ReadState[T]; Set(T) }
```

来源为稳定、非 nil 指针，以指针身份比较，不深比较内容/闭包。Observable 供 Observe，ReadState 供 Label，BindState 供编辑器。core/state.State 可直接满足接口。自定义 BindState 必须补 Connect，不保留全局重建 fallback。

Label.BindText 单向读取；TextInput/TextView.BindText、BindSelection 读取并回写用户编辑。来源在应用绑定时读取，不在声明构造时保存 Get 快照。现有 BindText(nil)/BindSelection(nil) 保持 no-op；typed nil 非有效来源。

- BindText(s).OnText(f)：保持读绑定，用户编辑交给 f。
- OnText(f).BindText(s)：读绑定，写回 s。
- BindText(s).Text(x)：改成字面值，取消绑定生成的写回。
- BindText(s).OnText(f).Text(x)：改成字面值，保留显式 f。

Selection 采用相同规则。程序应用值阻断 UI 编辑回调，避免反馈；业务 State 自己的通知不被阻断。TextView Model 与 Text/BindText 互斥，大文档保持 TextModel 路径。

属性失效只重读本节点的绑定属性，不重放静态声明、完整 WidgetView.Update 或共享修饰器。编辑器绑定按文本、选区顺序应用，未绑定的字面属性不重放。完整声明更新仍应用全部声明属性。相等值交由既有 GUI setter 的契约处理，不给任意 T 添加比较。

View 实现者用泛型 BindValue(ctx, property, source, apply) 登记属性；property 是 View 内固定名字，不是用户 ID。登记立即订阅、取值并应用，本轮未再登记的绑定清除。节点保存连接和最新 apply，不占用 ctx.State。普通应用只使用 BindText 等接口。

## 节点失效与批次

App.RequestUpdate 保持全应用语义。BuildContext.RequestUpdate 请求所属节点 Build/Update，异步执行，过期无效。不提供 ID 依赖关联。来源回调仅线程安全入队，不读取 Widget 或节点当前 Root；UI 线程执行时读取最新声明/归属。

App 共用一个任务队列，区分构建失效和属性失效；同节点合并，祖先优先。只跳过本批实际完成更新的节点：父 Update 可能未访问延迟/虚拟行，不能按祖先关系清空请求。全应用更新后仍处理未访问的局部请求。本批运行中新增的通知留给下一批。

初次连接早于 Get，防止丢失初始并发通知。来源替换或节点释放先作废连接，再 Disconnect；已开始的通知仍须通过执行端生命期检查。State 可后台 Set，绑定 getter/setter 与构建都在 UI 线程。无订阅的 State 在 App 退出后仍能正常读写。

订阅归挂载 node，不归可重复创建/共享的 View 值；共享同一个 View 或 State 不共享可变挂载状态。移交保留节点和订阅，调度读取新 Root；临时脱离/移交期间不嵌套执行更新。释放后不复活。

ctx.State 仍为 View 的私有实现状态。普通 Build 不暴露上下文；自定义 ComposeView 可使用 RequestUpdate 和 Observe。统一绑定不向 Root 添加控件类型分支。

## 挂载与完成边界

节点记录挂载目标和声明索引。局部组合输出变动沿组合祖先更新输出，再经真实 Bin/Container/Window 替换，不添加占位 Widget。移除、换类型及 nil/非 nil 转换保留既有资源归属和相对排序。

UpdateChild/UpdateChildren 增加零或一个可选 apply 回调，分别接收 Widget 或声明索引与 Widget。在普通协调完成及局部输出替换完成后应用槽位配置；nil 表示无输出。Overlay 用于重设 Fill/定位回调。Root 不识别 Overlay。槽位回调属于当前声明，不能携带源容器配置跨移交；新目标下一轮声明建立自己的配置。

本批全部受影响节点挂接完成后运行有效 AfterUpdate。全应用更新仍先完成所有窗口树，再回调，再显示新窗口。属性更新不伪造完整 Update/AfterUpdate。协调和移交期间禁止嵌套更新，结束后安排积累的请求。AfterUpdate 不承诺布局已完成。

## 迁移与范围

Get 返回字面快照；持续跟随改 BindText，结构/计算表达式放在 Build 或自定义 ComposeView 并 Observe。窗口集合、标题、应用样式表仍显式 RequestUpdate。Settings/mlang 的现有全局请求保留。移除 State 专用当前 App 引用，保留 Run 的一次运行约束。

首切片覆盖节点调度、Build/Observe、通用 BindValue、Label、TextInput、TextView，以及 demo/正式例程迁移。其他 BindXxx 按需求扩展，不建立属性路径、反射映射、隐式 Get 依赖或派生状态系统。

包内测试集中在既有 state/root/composition/text_editing/app/after_update 等文件；绑定基础设施可有专门 binding_test.go。覆盖更新隔离、通知合并/重入、虚拟行、来源替换/卸载、并发、输出替换/槽位配置、编辑反馈/历史/选区和跨窗口移交。独立窗口用例 tests/window/ui/binding 使用真实输入及计数断言，不用 DevServer。验证完成运行 go test ./...、相关 race/vet 和目标编译。

## 参考

2026-10-28 核对官方 API 与所示源码：Flutter [Element.markNeedsBuild](https://api.flutter.dev/flutter/widgets/Element/markNeedsBuild.html) 的节点标脏/去重，[Builder](https://api.flutter.dev/flutter/widgets/Builder-class.html) 与 [ListenableBuilder](https://api.flutter.dev/flutter/widgets/ListenableBuilder-class.html) 的回调构建与显式来源；GObject [bind_property](https://docs.gtk.org/gobject/method.Object.bind_property.html) 的来源/目标和 [unbind](https://docs.gtk.org/gobject/method.Binding.unbind.html) 的连接生命周期。仅借鉴机制，GOUI 保持普通 Go 状态、现有 UI 调度和 GUI setter；不照搬 GObject 属性系统或 Flutter 帧调度。
