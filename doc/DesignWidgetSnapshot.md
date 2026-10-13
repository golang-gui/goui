# WidgetInfo 语义属性扩展

状态：2026-10-05 按用户确认的 Attributes／SetAttribute 方案实现；本轮验证结果见 §9。

## 1. 定位与边界

Snapshot 是用于理解界面和规划输入的时点数据，不是活体 Widget 接口、命中树或
控件属性系统。公共身份、角色、边界、状态、动作和子树继续使用 gui.WidgetInfo
的明确字段；控件包的详细语义通过具名 Attributes 扩展。

保持 Widget.Snapshot() WidgetInfo、Window.Snapshot() WindowInfo 和
Application.Snapshot() ApplicationInfo 不变。不引入注册表、属性绑定、自动类型
恢复或新的 UI 协调机制；动作仍通过 Window.DispatchEvent 的正常输入路径执行。

参考官方公开契约（2026-10-05 核对）：Qt 6.12 的
[QAccessibleInterface](https://doc.qt.io/qt-6/qaccessibleinterface.html) 区分公共信息
与专门能力接口；GTK4 4.23.4 的
[AccessibleRelation](https://docs.gtk.org/gtk4/enum.AccessibleRelation.html) 描述行列、
层级和集合位置等语义关系。采用公共／专门语义分离，不照搬活体对象查询机制。
这不是原生无障碍后端实现，也不承诺 JSON 属性可直接映射全部平台无障碍接口。

## 2. 公共 API

```go
type WidgetInfo struct {
    // 现有公共字段保持。
    Attributes map[string]any `json:"attributes,omitempty"`
}

func (info *WidgetInfo) SetAttribute(name string, value any)
```

Attributes 是快照属性，而非 Widget 的可设置属性；没有 Widget.SetAttribute。
SetAttribute 仅组装尚未发布的快照，不发送信号、不改变控件或布局。

- 名称非空、大小写敏感，不修剪或归一化；空名称 panic，属于编程错误。
- 非 nil 值按需创建 map；同名写入整项替换，不合并嵌套字段。
- value == nil 删除该项；删除不存在项不会创建 map。
- 带类型的 nil 遵循 Go 接口语义，不视为删除；其 JSON 通常为 null。
  删除必须显式传入 nil，不引入反射判空。
- nil／空 map 都不输出 attributes。没有额外属性的普通节点不创建 map。
- 值原样保存，不在 helper 中做编码校验、深拷贝或类型转换。
- 读取直接使用 map 和类型断言；暂不增加 getter、删除方法或通用 Clone API。

## 3. 名称与数据归属

名称表示语义契约，不是具体 Widget 类名，也不与 ID、Name、StyleName 混用。
goui. 为框架提供的语义，外部包使用自己的前缀，例如 acme.chart；不需要注册。
同一名称应保持数据意义和字段约定稳定，未知属性由消费者忽略。

widgets/info.go 保留已有 Tab／Split 角色，并接管：

```go
const (
    TableInfoKey     = "goui.table"
    HierarchyInfoKey = "goui.hierarchy"
)

// 类型字段及 JSON 标签与原 gui 定义保持。
type TableInfo struct { /* 行列数量、模型身份、位置、Current、Sort */ }
type HierarchyInfo struct { /* 节点／父级身份、层级、兄弟位置、展开、Current */ }
```

内置生产者统一写入结构体值，消费者断言 widgets.TableInfo／HierarchyInfo，而非
指针；不要求外部属性一律采用此形式，但推荐普通数据结构体描述成组语义。

TableInfo 的行／列位置从 1 开始，0 为不适用；RowID／ColumnID 不是 Widget ID。
Sort 为 ascending、descending 或空。HierarchyInfo 的 Level／PositionInSet 从
1 开始，SetSize 为当前已知同父节点数量，根 ParentID 为空。两者 Current 均不等于
Focused；选择仍使用公共 Selected。迁移不改变这些字段或原 JSON 标签的含义。

## 4. 组装与组合

```go
info := v.WidgetBase.Snapshot()
info.Role = RoleTable
info.SetAttribute(TableInfoKey, TableInfo{
    RowCount: len(v.rowIDs), ColumnCount: len(v.columns),
})
```

- 基类、组合控件和具体控件追加自己的属性，不直接替换整个 map 丢失已有数据。
- 不同名称可共存；同名覆盖由组装者显式决定，GUI 不猜测如何合并。
- 父节点不自动向子节点传播属性，也不根据 Role 推断属性。
- 属性位于独立的 attributes JSON 对象，不能覆盖外层 ID／Bounds 等公共字段。
- 表头在本地构造完整 TableInfo（包括排序方向）后写入，不修改已发布载荷。

## 5. 生命周期与隔离

Snapshot 在 GUI 线程采集，结果之后按只读数据使用；编码阶段不能重新查询活体控件。
每次快照独立组装 map，不在 Widget 中缓存并反复返回同一可变集合。

值不得包含 Widget、Model、原生资源、回调或延迟查询。生产者复制内部可变 slice、
map 或指针数据，结构体值的浅拷贝不足以隔离这些字段。SetAttribute 不替生产者
进行隐式深拷贝；copy := info 同样是普通 Go 浅拷贝，不保证可独立修改内部 map。

正确构造后，协议层可在 GUI 线程外读取独立的快照；任何自定义 MarshalJSON 也只能
编码已采集数据，不得访问 GUI。并发写同一快照不是受支持操作。

不为快照实例化离屏行或导出整个大模型；保留语义子树过滤、窗口局部 DIP 边界、
自身 Visible 和既有 Actions 可达性限制。

## 6. JSON 契约与失败响应

相关 JSON 片段：

```json
{
  "role": "table",
  "attributes": {
    "goui.table": { "rowCount": 10000, "columnCount": 4 }
  }
}
```

不为 WidgetInfo 实现自定义 MarshalJSON，也不要求各类型实现编码接口。
普通结构体使用 [encoding/json](https://pkg.go.dev/encoding/json#Marshal)；函数、
循环引用、NaN／Inf 或自定义编码错误不被静默丢弃。

直接取得的 Go 快照可以断言具体结构体值；JSON 解码不会自动恢复包的类型。
远端消费者可使用 map[string]json.RawMessage 接收，再按名称解码所需的数据结构；
默认解码到 any 的对象／数字遵循
[标准库规则](https://pkg.go.dev/encoding/json#Unmarshal)，不提供全局类型注册表。

gui/dev 的 writeJSON 在发送 HTTP 状态前编码完整响应；失败改为 HTTP 500 和
ok:false 的固定 bool/string 错误 DTO，不写成功快照或部分响应。固定错误 DTO
自身不包含动态载荷。网络写入失败不等于编码失败；响应开始写出后不重发另一响应。

DevServer 这里只做自身协议的包内验证，不成为其它控件的自动化验收依赖。

## 7. 本次迁移边界

- 移除 gui.WidgetInfo.Table／Hierarchy 和 gui.TableInfo／HierarchyInfo。
- widgets TableView／表头／行／单元格、TreeView 行改用 Attributes。
- 更新原有包内和窗口程序的读取方式，保留有效字段、状态、几何和虚拟化断言。
- Range、DragDrop、TextEditing、滚动和列表范围字段本轮保持。
- GUI Widget 接口、UI 协调、平台窗口、布局与 Painter 都不改。
- 尚未正式 Release，直接迁移：不保留旧字段、类型别名或重复 JSON 输出。

未来公共语义确需演进时仍可调整 WidgetInfo，但不能仅为一个新控件添加固定字段。

## 8. 实施与验证

1. gui/info.go 的 map 与 helper；widgets/info.go 接管数据，迁移生产者／读取者。
2. gui/dev 修正完整编码及错误响应，补包内协议回归。
3. 更新控件设计与进度，执行全仓、相关 race、目标编译和已有窗口 Snapshot 检查。

gui/info_test.go 使用外部包验证无需注册的自定义控件；覆盖替换／删除／空名称、
大小写、false、typed nil、空 map 省略、未知属性和由生产者复制的可变数据。
不把 helper 的原样保存误测成自动深拷贝。

Table／Tree 测试仍集中在原 table_view_test.go／tree_view_test.go，检查实际类型、
旧快照稳定、新状态生效、其它能力共存及独立预期 JSON，确认旧 table／hierarchy
顶层字段消失；不削弱虚拟化、状态或布局覆盖。

server_test.go 覆盖未知属性透传、默认解码语义，以及函数、循环引用、NaN／Inf、
自定义编码失败的完整 HTTP 500；新回归在旧 writeJSON 上明确得到 200 空响应。

命令：go test ./...；go test -race ./gui ./gui/dev ./widgets/...。
受影响包和 table／tree_view 窗口入口补 Windows amd64／macOS arm64 编译；本机
通过正常鼠标点击既有 Snapshot 按钮检查集成，不以其替代全面平台交互验收。

## 9. 本轮验证记录

环境：Linux amd64、Go 1.25.8、X11 :3.0、XFCE／Xfwm4 4.16.1；本机窗口使用
Software、Native、1x。本轮没有性能结论，也没有更改平台绑定或原生能力。

通过：

- go test ./...。
- go test -race ./gui ./gui/dev ./widgets/...。
- 移除 DISPLAY／WAYLAND_DISPLAY 后，go test ./gui ./gui/dev ./widgets/...
  -run 'WidgetInfo|Snapshot|WriteJSONEncoding' -count=1；widgets/style 和 widgets/ui
  在该筛选命令中没有匹配测试，不计作其快照专项运行；它们由全仓／race 覆盖。
- CGO_ENABLED=0，Windows amd64／macOS arm64 的 gui、gui/dev、widgets、widgets/ui
  测试目标，以及 table／tree_view 窗口程序编译通过，不等同于原生运行。
- 本机 TableView UI：初始／选择后的 Snapshot、亮暗切换、Reveal last 和 Rebuild 后
  Snapshot 均 ASSERT PASS；逻辑 10000 行、实际 8 行，滚动和稳定当前／选择信息保留。
  既有 RenderWidget 自绘内容边缘断言同样通过。
- 本机 TreeView GUI：初始／选择后的 Snapshot、展开万行／Reveal last、Refresh／
  亮暗切换后 Snapshot 均 ASSERT PASS；展开后逻辑 10006 行、实际 18 行。

回归证据：新增 TestWriteJSONEncodingFailure 在修复前的函数／循环引用／NaN／Inf／
自定义编码失败五项均以 HTTP 200 空体失败；修复后完整 HTTP 500 错误响应通过。
开发中发现并修正新增测试未启用列排序的设置问题；没有删除断言或放宽预期。

Windows／macOS 原生桌面本轮未执行，Linux OpenGL／2x 也未重跑；这次仅调整
快照数据和协议编码，不将有限窗口检查扩展为完整 Table／Tree 输入与外观验收。
全仓原有条件跳过和 PNG 输出规则不变，不能把跳过记作桌面通过。本轮创建的两个
窗口均已正常关闭；构建与截图只在 /tmp/goui-snapshot-build.OTJ6sT，未操作业务文件、
真实剪贴板或系统设置。未使用 DevServer 验收控件，未提交或推送。
