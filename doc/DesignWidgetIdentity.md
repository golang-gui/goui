# Widget 的 ID、Name 与查询

状态：2026-10-01 增加 UI 挂载目标内的 ID 协调；不引入 Key、全局唯一 ID 或隐式跨 Root 认领。
2026-09-24 已实现 ID/Name 分离、Root 查询、Snapshot 与 HeaderBar 迁移；
窗口用例的预览查询路径已补充，真实桌面操作待验收。UI 拖放采用 Data 描述符。
UI 拖放接入见当前 [DragSource 与 DropTarget 声明接口](../ui/dragdrop.go)。

## 1. 四种独立语义

| 属性 | 含义 | 唯一性 | 用途 |
| --- | --- | --- | --- |
| ID | 程序标识 | 所属范围内非空值唯一 | 精确查询、自动化定位、UI 局部协调 |
| Name | 自然名称 | 可重复 | 描述、调试、按名称匹配 |
| StyleName | 样式名称 | 可重复 | 解析控件自身样式 |
| Text | 控件内容 | 可重复 | 显示文字、编辑内容 |

各字段独立，不相互填充：Name 不自动取自 Text，不回退为 ID，也不参与样式匹配。
名称不替代控件 Role、Text 或其他语义信息；本次不另建无障碍命名推导机制。
API 统一使用 Go 风格的 `ID`，不使用 `Id`。

## 2. GUI 与 UI 接口

GUI 保留 `ID()/SetID(string)`，在 Widget/WidgetBase 增加 `Name()/SetName(string)`。
UI 新增 `.ID(string)`，`.Name(string)` 改为设置真正的 Name：

```go
ui.Button("保存").
    ID("document-save").
    Name("保存文档").
    Style("toolbar-button")
```

共享修饰器分别维护 ID、Name 的字段、显式设置位和初始值；沿用 `bits.Bitmap`、
初始值快照及统一协调路径。显式空字符串清空该字段，移除修饰器恢复该字段初始值。
GUI `SetID` 是对同一对象改名，只触发语义失效；UI `.ID` 同时声明节点身份。
因此改变 UI 声明 ID 会释放旧节点并创建新节点，而不是对原对象改名。
Name、StyleName 和 Text 不参与身份匹配，改变它们仍更新原对象。

### UI 局部身份与状态

- 无 ID：在同一挂载目标内按实际节点位置和具体 WidgetView 类型匹配。
- 有 ID：在同一挂载目标内按 ID 和具体 WidgetView 类型匹配，插入、删除和重排
  保留匹配节点的 Widget、State 与信号连接，使用相对兄弟重排而非卸载再挂载。
- ID 相同但类型改变也重建；重复的非空兄弟 ID 在修改列表前报声明错误。
- 换父级或 Root 不自动保留状态。普通声明会移除旧节点、建立新节点；只有明确
  的节点移交才能保留原子树。单子槽位及根节点也遵守 ID／类型变更重建的规则。
- 位置忽略 nil View／Build，以及未产生 Widget 的 Mount。GUI `SetID` 不改变协调
  记录的声明身份；下一次 UI 更新仍应用当前声明，不读取可变 GUI ID 来认领节点。
- Root 内非空 ID 唯一仍是调用方契约；并未建立应用全局 ID 注册表。

参考 Flutter 官方 [Key 契约](https://api.flutter.dev/flutter/foundation/Key-class.html) 与
[Widget.key](https://api.flutter.dev/flutter/widgets/Widget/key.html)：其父级局部的类型／身份
匹配适合 GOUI；不采用 [GlobalKey](https://api.flutter.dev/flutter/widgets/GlobalKey-class.html)
跨树自动重挂。这里复用已有 ID，而非另引入 Key。SwiftUI 的
[Demystify SwiftUI](https://developer.apple.com/videos/play/wwdc2021/10022/)
说明身份、生命周期与状态的关系；不把其结构身份机制等同于全局 Widget 查询。
上述公开契约于 2026-10-01 核对，属于设计参考，不表示 GOUI 与其实现完全相同。

TabPage 也是真正的 WidgetView：`TabPage("document-32", content)` 的参数就是页面 ID，
不是另一个局部 key。同 Root 多个 TabView 的页面 ID 不能重复；不同 Root 可以重复，
但移交时若目标 Root 有同 ID，明确拒绝。绑定内部不公开任意 View State 查询。

## 3. 范围、查询与生命周期

- Window ID 在应用内唯一；非空 Widget ID 在所属 Root 的 Widget 树内唯一。
- Window 与 Popover 各自形成范围，不通过所属 Window 将独立 Popover 合并查询。
  Overlay 是普通容器，不创建新范围。不同 Root 可使用相同 Widget ID。
- 空 ID 表示未指定，不参加唯一性检查，不作为查询目标；不自动分配 UUID。
- 调用方应保证非空 ID 唯一；查询不校验唯一性，重复时返回树遍历中的首个匹配项。
  重复 Name 合法，按 Name 查询允许多个结果。唯一性针对当前树，ID 可在移除后复用。

通过明确范围和 ID 获取当前挂载的 Widget，而不是从临时 View 值反向取得 Widget。
`ui.App.FindWidget(windowID, widgetID) gui.Widget` 先定位当前 Window，
再委托 `gui.FindWidget(root, widgetID) Widget` 查询其 Widget 树。
不存在或空 ID 返回 `nil`；重复 ID 返回首个匹配项，不返回错误。
`gui.FindWidgetsByName(root, name)` 返回当前树中所有同名对象。
GUI 查询函数同样可用于独立 Popover Root；没有全局 Widget 注册表或 UI 第二棵控件树。
公开的 `Window` 与 `Popover` 接口均嵌入 `Root`，调用方直接传入宿主即可，
不需要断言内部具体类型；包内回归也通过公开接口验证此契约。

GUI 查询及 Widget 使用都在 GUI 线程进行；`ui.App.FindWidget` 通过 `Sync` 调度查询，
应用停止后返回 `nil`。这不使返回的 Widget 可跨线程操作，使用仍须进入 GUI 线程。
不存在、尚未挂载或已经移除时不返回对象。
查询的是当前已协调的树，不是尚未应用的新声明；Build 期间不能依赖新节点已经存在。

返回的 Widget 是借用对象，所有权仍属于 GUI/Window。优先在需要时重新查询，
不要将结果当作跨重建、跨销毁永远有效的引用。外部回调可能移除或销毁对象，
后续操作仍要遵守生命周期检查。不得绕过协调器销毁、重挂其管理的 Widget；
直接设置声明管理的属性也可能被下一次更新覆盖。

取得对象不等于已完成布局。`gui.RenderWidget` 仍要求 GUI 线程、Paint 之外、
挂载宿主已完成布局；传 scale=0 使用宿主当前缩放，失败交调用方处理。
详见 [RenderWidget](DesignRenderWidget.md)。

## 4. Snapshot、HeaderBar 与迁移

- `WidgetInfo` 增加 Name（JSON 字段 `name`），保留 ID、Text，三者独立报告。
  GUI/dev 透传快照；定位到对象也不提供绕过输入分发的动作捷径。
- `HeaderBar.Captions(ids ...string)` 将实际命中 Widget 的非空 ID 视为 Caption 区域，
  不搜索祖先或其他 HeaderBar。自动判断仍生效；交互控件被显式命中时也可作为 Caption。
- 旧 `.Name(...)` 用于精确定位、ID 断言时迁移为 `.ID(...)`；用于自然描述、
  调试时保留。按用途迁移，不全仓机械替换。
- 内置控件的固定 ID 逐项检查；例如窗口控制按钮仅用于描述的 `window-close`
  等标记改为 Name，避免占用应用的唯一 ID。同步检查依赖这些标记的测试和快照。
- 不保留 `.Name()` 同时写 ID 的兼容分支；GUI、UI、例程、文档及相关测试一起迁移。

## 5. 后续验证要求

包内测试覆盖：ID/Name/StyleName/Text 独立；显式清空与移除修饰器恢复初始值；
改名不重建或请求布局；重复 Name；重复 ID 返回首个匹配及改名后的重新查询；空 ID；跨 Root 同 ID；
Overlay 共用范围；卸载/替换后查询；Snapshot 三字段；HeaderBar.Captions 按 ID 精确命中。
保留现有快照和输入路径覆盖，不用修改断言隐藏身份混用问题。

UI 拖放窗口用例已增加 `-widget-preview`：按 ID 查询源控件并调用 RenderWidget，
供 1x/2x 的缩放、热点、重建和目标移除操作验收。包内回归已覆盖查询与协调行为；
真实窗口验收需单独记录，不将编译或包内通过计作桌面验收。
