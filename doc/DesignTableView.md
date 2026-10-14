# TableView：通用多列记录视图

> 状态：2026-10-05 首版代码、包内回归及本机窗口主要场景已实现／验证。
> Windows／macOS 仅完成交叉编译，原生窗口验收仍待完成；详见第 13 节。
> 边框／背景／斑马色及自定义内容安全区域已补齐，方案见 §10.2，验证见 §14。
> 本文包含后续确认的 ListView 稳定性修正：不增加 InvalidateMeasurements 等公开接口。
> 先验证既有 RequestLayout／Refresh 与虚拟行缓存的契约，再组合实现 TableView。

## 1. 定位、范围与参考

TableView 面向文件列表和记录管理，不是电子表格。复杂控件留在 `widgets`，
声明绑定放 `widgets/ui`，复用既有列表、滚动、输入、信号和协调机制。

首版包括：

- 固定表头、双向滚动、行虚拟化和可变行高。
- 显式列、确定列宽、宽度上下限与拖动调整。
- 自定义单元格、整行单／多选、Current、键盘导航和行激活。
- 单列排序请求、排序标记、表体上下文菜单。
- GUI 命令式接口、UI 链式声明、兜底／modern 样式和语义快照。

暂不实现单元格范围选择、区域复制、内置编辑事务、自动生成列、反射字段绑定、
列拖拽重排、冻结列、多级表头、横向列虚拟化、权重列宽和全表内容宽度扫描。
单元格可组合按钮或编辑器，但业务数据和编辑状态必须由应用保存；这不构成内置
表格编辑事务。数据存储、排序、筛选、加载、取消及业务错误反馈由模型／应用负责。

### 1.1 参考与取舍

- [GTK4 ColumnViewColumn](https://docs.gtk.org/gtk4/class.ColumnViewColumn.html)
  的公开接口把内容工厂、标题、宽度及排序相关配置放在列上。采用按列构建内容的
  分工，不引入 GObject 属性或工厂体系。
- [GTK 4.20.0 ColumnView 源码](https://github.com/GNOME/gtk/blob/4.20.0/gtk/gtkcolumnview.c)
  使用内部 ListView、统一列分配和共享水平偏移协调表头与正文。采用组合和统一
  几何数据的思路，不照搬 GTK 的 Scrollable border／adjustment 协议。
- [Qt 6 QTableView 文档](https://doc.qt.io/qt-6/qtableview.html)
  将内容适配列宽和末列拉伸作为显式能力。GOUI 首版采用稳定列宽与横向滚动，
  不为完整二维模型、编辑代理和自动内容扫描扩大首版范围。

GTK 官方在线 API 文档会滚动更新，源码行为以所链接的 4.20.0 为参考版本；Qt 页面
查阅时为 6.12.0。上述为控件层参考，不涉及原生后端承诺，也不是 GOUI 实测结果。
以下是 GOUI 的设计选择，实际行为必须通过后续代码和测试确认。

## 2. 数据模型与身份

```go
type TableModel interface {
    gui.ListModel
    RowID(index int) string
}

type TableData[T any] interface {
    TableModel
    ItemAt(index int) T
}
```

复用 ListModel 的 `ItemsCount`、`ConnectItems`。TableView 不泛型化，类型化数据访问
由 Delegate 和 UI 构建器使用。

模型契约：

1. RowID 非空，在模型内唯一；修改字段、排序或插入其他记录不改变原记录的 ID。
2. 行 ID 不是 Widget ID，不自动调用 SetID，也不要求跨模型唯一。
3. 查询是同步只读内存访问，不执行阻塞 I/O，不在查询期间发送通知。
4. 完整提交一次修改后发出 Items；与控件连接期间在 GUI 线程访问，后台结果经
   Application 投递回 GUI 线程。
5. Model 不保存列、列宽、Widget、Selection、Current 或排序箭头。
6. 空／重复 ID 属于模型违反契约，应明确报告编程错误，不能退化成索引身份。

首版不新增 TableStore，可组合已有 SliceListModel：

```go
type FileModel struct {
    *gui.SliceListModel[File]
}

func (m *FileModel) RowID(index int) string {
    return m.ItemAt(index).ID
}
```

排序可使用既有 Modify，完成后一次通知。业务数据与查询适配器必须保持一致，不能
只重排显示 ID 而继续用旧索引读取 ItemAt。

TableView 保存 `rowIDs []string` 和 `indices map[string]int`；安装和模型通知时
O(N) 重建身份索引，只读取 ID，不为全部记录创建 Widget 或进行文字排版。
内部 ListView 使用私有模型适配器，由 TableView 在完成身份／状态同步后通知，
不同时独立订阅业务模型，避免依赖订阅顺序。

## 3. 列对象与单元格生命周期

### 3.1 TableColumn

```go
func NewTableColumn(id, title string) *TableColumn

func (c *TableColumn) ID() string
func (c *TableColumn) Title() string
func (c *TableColumn) SetTitle(string)
func (c *TableColumn) Width() float32
func (c *TableColumn) SetWidth(float32)
func (c *TableColumn) MinWidth() float32
func (c *TableColumn) SetMinWidth(float32)
func (c *TableColumn) MaxWidth() float32
func (c *TableColumn) SetMaxWidth(float32)
func (c *TableColumn) Resizable() bool
func (c *TableColumn) SetResizable(bool)
func (c *TableColumn) Sortable() bool
func (c *TableColumn) SetSortable(bool)
func (c *TableColumn) Delegate() TableCellDelegate
func (c *TableColumn) SetDelegate(TableCellDelegate)
func (c *TableColumn) ConnectResize(func(width float32)) signal.Handle
```

- Column 是普通配置对象，不是 Widget，不拥有原生资源，不增加 Destroy。
- ID 非空、创建后不变，同一表格内唯一，与 Widget ID 查找无关。
- 一个可变 Column 同时只归属一个 TableView；移除后可安装到另一个表格。
- 表头、单元格和内容 Widget 由 TableView 管理；列通过私有所有者联系通知相关更新。
- Resize 表示用户交互，包括取消时的宽度恢复；SetWidth 不发送交互信号。
- SetDelegate 显式重新安装，即使传入相同对象，也必须用旧 Delegate 解绑旧内容，
  新 Delegate 不接收旧 Delegate 的行壳。不猜测接口值是否相同。

```go
func (v *TableView) Columns() []*TableColumn
func (v *TableView) SetColumns(...*TableColumn)
```

Columns 返回副本，参数顺序就是显示顺序。先验证完整集合，再修改现有集合；拒绝
nil、重复 ID、重复对象或属于其他表格的列。相同指针可保留内容，相同 ID 的新对象
表示替换，不隐式继承旧对象的可变状态。零列合法，保留模型但不实现记录行。
首版不添加另一组插入／重排接口。

### 3.2 Delegate

```go
type TableRow struct {
    ID       string
    Index    int
    Selected bool
    Current  bool
}

type TableCellDelegate interface {
    Setup() gui.Widget
    Bind(TableRow, gui.Widget)
    Unbind(TableRow, gui.Widget)
}
```

每列使用独立 Delegate，Bind 无需额外列索引。TableRow 是调用时快照，Index 不能
持久标识记录。Setup 返回未挂载、未销毁的内容；nil Delegate 表示空单元格。

- 同一绑定可以重复 Bind，内容及信号连接必须更新为最新状态，不能累积旧连接。
- 切换记录前先 Unbind；它收到旧快照，不能用旧 Index 查询已经改变的模型。
- 回收、移除列、替换 Delegate、卸载时解除绑定；回收与 Unmount 不得重复解绑。
- 列表管理行壳复用，行壳管理各列单元格；复用池有界，不保留全部历史记录的 Widget。
- 内容 Widget 可以拥有布局和交互，业务编辑状态不能只存于将被回收的 Widget。

## 4. TableView 接口与状态

```go
func NewTableView() *TableView
func (v *TableView) Model() TableModel
func (v *TableView) SetModel(TableModel)
func (v *TableView) SelectionMode() SelectionMode
func (v *TableView) SetSelectionMode(SelectionMode)
func (v *TableView) Selection() []string
func (v *TableView) SetSelection([]string)
func (v *TableView) Current() string
func (v *TableView) SetCurrent(string)
func (v *TableView) Reveal(rowID string)
func (v *TableView) Refresh()
func (v *TableView) ConnectSelection(func([]string)) signal.Handle
func (v *TableView) ConnectCurrent(func(string)) signal.Handle
func (v *TableView) ConnectActivate(func(string)) signal.Handle
```

复用 widgets.SelectionMode。默认单选、无选择／Current、可聚焦、MainWeight(1)，
自带 ScrollView。Selection／Current／Reveal 独立，不把设置当前项当成选择或滚动。
Selection 返回当前模型顺序的副本，过滤未知／重复 ID；单选模式最多一个 ID。
Current 允许空字符串清除，未知 ID 不变成悬空状态。

SetModel 始终重新安装，包括同一对象：断开旧通知、取消待执行 Reveal／菜单／行
交互，清空选择、Current、锚点和排序标记，滚动归零，保留列。nil 清除模型。
数据变化通过 Items，表现变化通过 Refresh，声明层避免重复安装。

收到 Items 后先完成身份索引和状态同步，再通知内部列表及外部监听者：

- 保留仍存在的选中 ID 和 Current；被删除的选中项移除。
- Current 被删除时，按旧位置寻找附近有效记录作为新 Current；是否补选遵循
  TreeView 已有删除当前项规则，单选／多选分别覆盖回归。
- 无效锚点回退 Current；关闭依赖旧命中的菜单，按 ID 重新解析待 Reveal 目标。
- 行顺序未变时刷新内容，顺序改变时重新绑定索引；同 Delegate 的行壳可复用。
- 选择成员不变时，不因排序改变索引或返回顺序而发送虚假的 Selection 变化。
- 保留并钳制原滚动偏移；首版不承诺排序后追随 Current 或保持首个可见记录，
  应用可显式 Reveal。SetModel 的复位与普通 Items 通知的保留不是同一种操作。

## 5. 布局与滚动组合

```text
TableView
├── HeaderViewport
│   └── HeaderRow
│       ├── ColumnHeader
│       └── ResizeHandle
└── ScrollView
    └── TableBody
        └── ListView
            └── TableRow
                └── TableCell
                    └── Delegate 内容
```

裁剪使用真实结构 Widget。Paint 只绘制自身，不通过仅绘制平移伪装几何位置。
TableBody 实现现有 Scrollable，并转发内部列表的 ScrollIntoView 请求；实际滚动仍
归 ScrollView。TableView 不实现第二套滚动算法，不需要外包 ScrollView。

### 5.1 列分配与测量

首版默认列宽 160 DIP、最小宽度 40 DIP、最大宽度无限制，Resizable=true，
Sortable=false。所有数值规范化沿用既有尺寸 API 风格，不让负数或 NaN 进入计算；
MaxWidth=0 表示无限制，设置时先归一化再计算有效上限。有效上限不能小于最小宽度。

```text
width[i] = clamp(preferredWidth[i], minWidth[i], effectiveMaxWidth[i])
x[i] = sum(width[0:i])
contentWidth = sum(width)
```

列宽包含内部垂直分隔线，分隔线在列内占用独立空间，不增加总宽。
表头和所有行共享同一份列
分配。小于视口时右侧留白，大于视口时横向滚动；不根据滚入的内容自动改宽。

正文内容宽度为列宽减垂直分隔线，单元格不提供 Padding。Measure 在这一宽度下
按内容测量期望高度，行高取各单元格内容的最大值，并满足最小内容行高 32 DIP，
再加行间水平分隔线；Arrange 使用 Tight 约束让内容填满最终分配区域。
填满不意味着 Measure 时占满无限高度，也不让内容撑破确定列宽。换行、文字
内边距和内容对齐由内容 Widget／普通容器负责，不新增 CellFill／CellPadding／
TableAlignment。UI 的内部承载容器也使用填满布局，不能重新把内容收缩。

表头布局独立保留最小内容高度 32 DIP、水平内边距 8 DIP、垂直内边距 4 DIP
及文字垂直居中。2026-10-05 的正文填满调整不改变表头的视觉密度。

通过 layout.Child 等现有缓存测量入口访问内容，不能绕过样式通知直接调用具体
控件的 Measure。TableView 的 Measure 计算表头和视口需求，主体沿用 ScrollView
尺寸策略；表头列总宽是可滚动内容宽度，不作为窗口最小视口宽度。
不测量全部记录，不以全部记录总高度撑开窗口。ListView 对行采用自然
测量时，TableRow 自身仍以共享列宽约束单元格。

### 5.2 固定表头

TableBody.LayoutVisible(viewport, offset) 接收 ScrollView 给出的实际表体视口：

1. 表头裁剪宽度使用实际 viewport.Width，不重复猜测滚动条宽度。
2. 表头每列位置为 x[i]-offset.X，Y 不跟随 offset.Y。
3. 内部 ListView 使用同一个 offset，行内单元格使用共享列分配。
4. 表体 ContentSize.Width 由列总宽决定，Height 来自列表测量与估计。
5. 空模型仍保留表头及由列总宽决定的横向滚动范围。

同步属于控件内部组合，不给 ScrollView 添加表头特例或公开偏移通知。滚动与
滚动条显隐变化均须更新表头视口，布局期间不递归重新安排整个 TableView。
Reveal 使用正文坐标，由内部 ScrollView 执行，不能往请求矩形里添加表头补偿。

### 无界约束的占位扣减

2026-10-13：完整 Basic 默认表头暴露 tableLayout.Measure 对 layout.Inf 的直接
扣减。Inf 是有限数值哨兵；边框／表头只从有限约束轴扣除，无界轴保持无界，
否则 ScrollView 会误把它当成巨大有限视口。未约束的表体不发明视口尺寸、
不实现全部记录；完整默认外框仍计入返回的期望尺寸。高表头四种有限／无界
组合的回归在旧实现复现、修正后通过，详见 DesignThemeFallback 执行记录。

## 6. ListView 稳定契约：不新增测量失效接口

### 6.1 现状与问题定位

本节是针对当前源码的设计审视，不是已经验证的缺陷修复记录：

- WidgetBase.RequestLayout 已清除自身和祖先的 measureValid，再请求 Root 布局。
- ListView.LayoutVisible 已检查实际行的 measureValid；内容 RequestLayout 后应重新
  测量该行，无需调用方补充另一种失效通知。
- ListView 另有按索引缓存的 heights／widths，以及高度估计和定位前缀。这些信息
  覆盖离屏、甚至已经没有对应 Widget 的记录，不能靠原 Widget 的标记持续维护。
- 现有 Refresh 重新绑定可见行并保留离屏尺寸；整体表现变化后，历史尺寸的可信度、
  重新测量和滚动定位之间还需要独立回归确认，不能直接断言已经满足所有情况。

欠缺的是虚拟行尺寸与既有布局失效机制的完整契约，不是缺少更多公开方法。
撤销早期提议的 `InvalidateMeasurements`，也不增加同义的 InvalidateMeasurement
或向调用方暴露尺寸缓存版本。

### 6.2 既有入口的职责

| 变化 | 调用方入口 | ListView 的内部责任 |
| --- | --- | --- |
| 已实现行的内容或布局状态改变 | 行／内容 RequestLayout | 重新测量受影响行，修正尺寸统计与定位信息 |
| 实际测量约束改变 | 正常布局流程 | 判断缓存依赖是否仍有效，按需重测 |
| 数据改变 | Model.Items | 更新绑定，处理索引身份与尺寸有效性 |
| Delegate 整体表现改变 | Refresh | 更新可见绑定，处理离屏历史尺寸的可信度 |
| 样式改变 | StyleChanged 流程 | 使相关尺寸失效并重新布局 |

RequestLayout 不等于重新 Bind，也不递归清空所有后代缓存；单行变化不能退化为
全表测量失效。仅在父容器 RequestLayout 不能自动发现闭包读取的任意外部状态。
TableView 负责把共享列宽变化落实到行布局状态，并通过已有 Refresh 表达整体
表现变化；实际行调用正常 RequestLayout。

Refresh 不重新安装 Model／Delegate，不通过伪造 Items 通知重建，不卸载全部可见
行。离屏历史尺寸不再可信时只能作为估计，不能一直冒充精确测量；保留估计是为了
避免滚动范围暂时归零、离屏最宽行暂时丢失导致横向回跳。行重新实现后必须用新
表现测量并修正估计；定位前缀也必须反映已经发生的尺寸变化。

内部可根据回归需要区分精确值与历史估计、合并失效请求，但具体缓存表示不是公共
协议。普通滚动、重复布局和局部状态变化仍应复用有效结果，不为每帧清空全部缓存。
不要求虚拟列表在未测量全部记录时给出精确总高度，也不通过实例化全部行来满足
滚动范围断言。基于未知行的 Reveal 仍允许先估计、实现后校正。

### 6.3 独立前置切片

先用普通 ListView 测试验证上述契约。若存在缺口，进行独立的缓存正确性修正，
不导入 widgets、不识别 TableView 类型、不读取列配置，也不增加公开失效入口。
既有 TreeView／菜单等调用方作为回归对象，而不是新增接口的迁移对象。

至少覆盖局部 RequestLayout、整体 Refresh、重新绑定、样式、约束变化、离屏最宽
行和滚动定位。测试应确认重新测量后的确定尺寸，不能仅比较两个都可能使用旧缓存
的结果；同时用计数验证没有全表实例化或在普通滚动时反复失效全部记录。

## 7. 输入、选择与列宽拖动

### 7.1 行交互

选择与 TreeView 对齐：普通点击更新单行选择、Current 和锚点；Primary 点击切换
多选，Shift 选择锚点范围，Primary+Shift 加入范围；双击激活记录。

- 上下、Home／End 移动 Current；Primary 修饰仅移动 Current，Shift 扩展选择。
- PageUp／PageDown 按当前可见行数量移动并 Reveal。
- Primary+A 在多选模式全选，Space 使用现有选择规则，Enter 激活 Current。
- Shift+F10 为 Current 请求菜单，先 Reveal，完成后在可见位置弹出。
- 不增加左右键单元格导航或另一套 Tab 焦点遍历。

正文空白点击可清空选择；表头、滚动条、手柄不能误当正文空白。行控制器使用正常
传播／手势竞争路径，子控件已处理的点击和按键不再选择或激活行，不按控件类型猜测
交互能力。回收持有真实焦点的内容前通过现有焦点机制安全移交到表格，不能让同一
Widget 在下一记录上继承旧焦点。

### 7.2 列宽拖动

每列右侧手柄命中宽度 8 DIP，视觉分隔线独立由样式绘制，不依赖 Resizable；
命中范围受表头裁剪，使用现有
水平调整光标及 DragEventController。手柄不触发排序，首版不做边缘自动滚动。

```text
newWidth = clamp(startWidth + pointerDeltaX, minWidth, maxWidth)
```

宽度实际变化后更新列分配、按 §6 请求表现／布局更新，再发 Resize。不在 PointerMove
中重新 SetModel／SetDelegate，不同步递归执行整个窗口布局。
Esc／手势取消恢复起始宽度并通知 Resize，使 UI 受控值同步回退。列被删除／替换、
窗口卸载时取消交互；外部回调后检查对象存活和列归属。松手后清理按下状态，hover
由真实命中决定，不能因焦点停留保持拖动高亮。

## 8. 排序请求与菜单

```go
type SortOrder uint8

const (
    SortNone SortOrder = iota
    SortAscending
    SortDescending
)

func (v *TableView) Sort() (columnID string, order SortOrder)
func (v *TableView) SetSort(columnID string, order SortOrder)
func (v *TableView) ConnectSortRequest(
    func(columnID string, order SortOrder),
) signal.Handle
```

点击另一个可排序列请求升序，当前升序请求降序，当前降序请求升序。SortNone 由应用
显式设置，不假设存在可恢复的原始顺序。表头可经正常焦点／键盘输入触发同一请求。
不检查信号是否有监听者，不在发请求前修改已应用的排序标记。

```text
表头输入 → SortRequest → 应用排序模型 → Model.Items → 应用 SetSort
```

SetSort 只更新标记，不改变模型、不再发送请求。SortNone 规范化为空列 ID；其他
状态必须引用当前可排序列。删除该列或关闭其排序能力后清除标记。异步排序的版本、
取消、过期结果和错误由应用管理，失败不能伪装成排序成功。

```go
func (v *TableView) ConnectContextMenu(
    func(rowID, columnID string, result *gui.MenuModel),
) signal.Handle
func (v *TableView) ConnectContextMenuError(func(error)) signal.Handle
```

菜单信号首版只服务表体：单元格提供行／列 ID；行右侧空白列 ID 为空；正文底部
空白行 ID 为空，横向仍在列范围时可提供列 ID；键盘菜单提供 Current，列 ID 为空。
不把表头菜单混入这个协议。右键已选中行保留多选，右键未选中行先选择它。
子控件优先处理事件；查询按连接顺序执行，后连接可覆盖结果，nil 不弹出。
复用 PopoverMenu 和错误信号，回调销毁或修改目标后不能继续用旧命中打开菜单。

## 9. UI 声明绑定

```go
func TableView[T any](
    model widgets.TableData[T],
    columns ...*TableColumnView[T],
) *TableViewView[T]

func TableColumn[T any](
    id, title string,
    cell func(widgets.TableRow, T) ui.View,
) *TableColumnView[T]
```

列描述器不是独立 View，不对应可通过 Widget ID 查找的节点。通过链式调用设置
Width、MinWidth、MaxWidth、Resizable、Sortable 和 OnResize。TableView 链式提供
SelectionMode、Selection、Current、Sort、对应 OnSelection／OnCurrent／OnActivate／
OnSortRequest，以及 ContextMenu(builder)／OnContextMenuError。

示例（wui 为 widgets/ui，ui 为基础声明层）：

```go
wui.TableView(model,
    wui.TableColumn("name", "名称", func(row widgets.TableRow, file File) ui.View {
        return ui.Label(file.Name).StyleName("table-cell-text")
    }).Width(state.NameWidth).MinWidth(120).Sortable(true).
        OnResize(func(width float32) {
            state.NameWidth = width
            app.RequestUpdate()
        }),
    wui.TableColumn("size", "大小", func(row widgets.TableRow, file File) ui.View {
        return ui.Label(formatSize(file.Size)).StyleName("table-cell-text")
    }).Width(100),
).SelectionMode(widgets.SelectionMultiple).
    OnActivate(openFile).
    OnSortRequest(sortFiles)
```

### 9.1 状态契约

- 不声明 Width 时使用默认初始宽度，并保留用户调整；显式 Width 为受控值，OnResize
  应回写应用状态，下一次更新重新应用该值。与 SplitView 一致，不增加 InitialWidth。
- 不声明 Selection／Current／Sort 时保留状态；声明时协调至对应 setter。
- 移除状态声明保留当时值，不自动复位。可选字段使用既有 bits.Bitmap 风格。
- 同 ID 的列描述更新保留 GUI Column；删除后重新添加不承诺恢复旧交互状态。

### 9.2 协调过程

state 保存最新模型／回调、按列 ID 的协调记录、稳定 Column／Delegate、signal
handles 和借用的 BuildContext。更新时先替换最新回调，再协调列和内容构建器，
必要时安装模型，然后应用显式状态并刷新内容。协调 setter 不回送声明式业务回调。

相同非 nil 模型指针不重复 SetModel；值模型不猜测相等性，每次按安装处理，typed
nil 不支持。相同列 ID 的构建函数变化更新稳定 Delegate 的最新函数，不重新安装
Delegate；GUI SetDelegate 自身仍是明确重新安装语义。

单元格使用普通 Bin／Container 和既有 BuildContext.UpdateChild／UpdateChildren。
Unbind 清理旧声明内容／连接，列删除先解绑再清理对应 state。延迟 Bind 的上下文
遵守现有借用有效期和 Root 转移契约，不缓存旧 Window，不增加 Coordinator 构建
接口，不向 ui.Root／BuildContext 加 TableView 特例。

## 10. 生命周期、样式与语义

### 10.1 生命周期与重入

TableView 拥有内部 Widget 树、菜单和模型连接，但不拥有业务模型。Mount 重新同步
离线期间的数据；Unmount 断开外部连接、取消菜单／Reveal／拖动，释放绑定，再次
挂载可以恢复工作。UI Root 只清理协调和声明连接，不夺取窗口的最终销毁职责。

Setup／Bind／Unbind／业务信号返回后检查对象存活和绑定版本。模型通知重入时标记
需要再次同步，在当前同步完成后处理，不递归重建，不用 goroutine 掩盖同步语义。
不能靠清空所有信号保证安全，也不能继续执行已经过期的选中／菜单／激活通知。

### 10.2 样式

独立名称：table-view、table-grid、table-header、table-header-text、table-row、table-cell-text、
table-cell-icon。表头分隔线／排序图标、行 selected／current 使用明确 Part。
父控件不向内容子树隐式提供样式，自定义内容选择自己的 StyleName。

完整兜底放 theme/basic，由 GUI 自动使用；modern 放 theme/modern，使用中性 hover／selected，
不默认铺主题色，表头底部细分隔线，默认薄外框和低对比水平线，不默认画垂直网格
或斑马色。Modern 的 selected 与 TreeView 共用浅中性色阶，表头只略区别于
正文；外框／表头底线比行分隔线稍清晰。Current 与实际焦点相区分，Modern
默认只在 FocusVisible 状态绘制 1 DIP current 边框；Widget 转交真实焦点状态，
不禁止应用显式设置鼠标 current 轮廓。列宽手柄不显示宽大高亮。
StyleChanged 使用现有失效路径，本轮不为行高和 padding 增加新属性系统。

2026-10-09 外观调整的依据、回归及本机验收见
[DemoFrameworkNotes §13](DemoFrameworkNotes.md#13-tableview-modern-外观收敛)。

#### 边框、背景与斑马色（2026-10-05 补充）

参考 GTK4 在线文档（查阅时 4.23.4）的
[行分隔线](https://docs.gtk.org/gtk4/method.ColumnView.set_show_row_separators.html)、
[列分隔线](https://docs.gtk.org/gtk4/method.ColumnView.set_show_column_separators.html)，
以及 Qt 6.12 的
[alternatingRowColors](https://doc.qt.io/qt-6/qabstractitemview.html#alternatingRowColors-prop)。
这些是公开 API 的视觉分类参考，不是其原生绘制源码或实测结论。GOUI 将开关、
颜色和线宽统一交给既有样式值，不新增 ShowGrid／AlternatingRows 控件属性。

| Name / Part | 使用的字段与语义 |
| --- | --- |
| table-view / 默认 | BackgroundColor 为全视口背景；BorderColor / BorderWidth 为外框默认值 |
| table-view / outer-horizontal | 覆盖上、下外框颜色／宽度 |
| table-view / outer-vertical | 覆盖左、右外框颜色／宽度 |
| table-grid / horizontal | BorderColor / BorderWidth 为行间线 |
| table-grid / vertical | BorderColor / BorderWidth 为列间线，表头、正文共享 |
| table-row / 默认 | 普通行 BackgroundColor |
| table-row / alternate | 奇数零基模型索引的 BackgroundColor（第二、四……行） |
| table-header / separator | ForegroundColor / BorderWidth 为表头底部分隔线 |

`table-grid` 的默认规则可同时设置两方向，Part 可分别覆盖；不从 table-view
隐式继承外框配置。外框 Part 则按既有同名样式的 Part fallback 继承统一外框。
线宽为零关闭；负数、NaN、Inf 视为零。透明颜色不释放线宽所占空间，因此仅变色
不改变布局。首版外框为矩形边线，不承诺圆角子树裁剪。

```go
style.Name("table-view").BackgroundColor(background).BorderColor(border).BorderWidth(1)
style.Name("table-view").Part("outer-vertical").BorderWidth(0) // 仅保留上下外框
style.Name("table-grid").Part("horizontal").BorderColor(grid).BorderWidth(1)
style.Name("table-grid").Part("vertical").BorderColor(grid).BorderWidth(1)
style.Name("table-row").BackgroundColor(background)
style.Name("table-row").Part("alternate").BackgroundColor(alternate).Radius(0)
```

- 外框向视口内部预留空间，覆盖区域包括滚动条；表头／ScrollView 位于框内。
  表体同步表头实际视口宽度时保留外框的 X/Y，不通过绘制偏移补偿。
- 列间线归前一列，在该列右边独占线宽；最后一列无内部右边线。行间线归前一行，
  在行底部独占线高；最后一条记录无内部下边线。空白视口不虚构记录／网格。
- 自定义内容先扣线区域，填满剩余矩形；正文 Cell 不再额外扣固定内边距。内容
  若需要留白自行组合容器。其测量／布局矩形不与线条相交；行高
  加入线高，而不是压缩已测得内容。极窄约束钳制到零，内容仍遵循既有结构裁剪。
  不要求自定义 Widget 自行判断表格线宽，不新增 Painter / 绘制后置钩子。
- 相邻边界只画一次。垂直线止于水平线之前，外框的横边拥有角点，防止半透明
  交点重复合成。装饰不新增 Widget / Snapshot 子节点或事件目标。
- 斑马色按当前模型索引计算，排序后重排条纹，回收壳不能携带旧奇偶状态。
  选中 Part（含其 Hovered / Pressed）优先；未选中时 Pressed > Hovered >
  alternate > 普通背景。alternate 用于 Normal，hover / pressed 仍使用公共行状态。
  Current 轮廓独立。正常背景与条纹分别通过自己的明确样式解析，不覆盖子树样式。
- 样式变动使用已有 StyleChanged 与测量失效；没有额外样式监听、缓存版本或 UI
  专用协调入口。默认水平线与外框为 1 DIP、竖线为零、条纹继承普通背景。

### 10.3 Snapshot

table／columnheader／row／cell 角色常量放 widgets。表格详细语义由 widgets.TableInfo
提供，以结构体值写入 WidgetInfo.Attributes[widgets.TableInfoKey]（goui.table），
表达逻辑行列数量、行／列 ID、位置、Current 和排序方向；GUI 不依赖 widgets 类型，
也不继续为每种控件增加专用字段。扩展契约见 [Widget Snapshot](DesignWidgetSnapshot.md)。

语义位置为一基索引，0 表示不适用：表头无数据行位置，整行无列位置。内部裁剪壳、
ScrollView／ListView 不形成重复语义层次；表格转述实际滚动状态和可见范围。只报告
真实实现的行及单元格，不为全部记录创建 Widget，不把 RowID／ColumnID 写成 Widget.ID。
排序表头、可点击行和内容控件如实报告动作；检查尺寸的手柄可复用 RangeInfo。
数据类型与 JSON 标签归 widgets/info.go；公共状态和边界仍遵守 gui/info.go 约定，
不能新增动作执行捷径。原顶层 table 字段不再保留或重复输出。

## 11. 开发切片与文件边界

1. **ListView 契约回归**：独立验证 §6，必要时修正内部缓存；不增加公开失效接口。
2. **基础表格**：模型、列、绑定、共享几何、固定表头、虚拟化和初始语义。
3. **交互**：选择／Current／导航、列宽拖动、排序请求、激活、菜单与生命周期。
4. **UI 和外观**：稳定列协调、受控状态、兜底／modern、完整快照和窗口例程。

| 文件 | 职责 |
| --- | --- |
| widgets/table_model.go | 模型、TableRow、Delegate 契约 |
| widgets/table_column.go | 列配置与所有权 |
| widgets/table_view.go | 模型同步、状态、导航、排序、菜单 |
| widgets/table_input.go | 选择／键盘输入、上下文菜单查询与弹出 |
| widgets/table_row.go | 行、单元格、绑定和复用 |
| widgets/table_header.go | 表头及宽度交互 |
| widgets/table_layout.go | 共享列分配与表头／表体布局 |
| widgets/table_paint.go | 线宽规范化、独立边界和网格绘制几何 |
| widgets/ui/table_view.go | 声明与协调 |
| theme/basic/widgets.go、theme/modern/modern.go | 完整兜底与 Modern 样式 |

GUI 核心预期只有 ListView 既有契约的内部修正（须以回归证明必要性）。表格语义
使用通用 Attributes，类型归 widgets；不得顺带修改 platform、Painter、Window、
EventDispatcher 或 UI Root。
文件拆分以职责为准，不把每个 helper 拆成文件，不先抽象通用 DataGrid 框架。

## 12. 测试与验收

包内按职责集中测试：widgets/table_view_test.go 使用子测试覆盖列、布局、模型、
交互、生命周期和快照；widgets/ui/table_view_test.go 覆盖协调；gui/list_view_test.go
补缓存回归；modern 样式断言进入既有主题测试。首版无独立存储实现，不为接口声明
本身另建零散 table_model_test.go。

必须有明确断言：

- 表头和单元格 X／宽度一致，横向滚动同步、纵向表头固定，滚动条显隐不造成错位。
- 小内容与高内容同处一行时，都填满最终单元格的内容区；UI 承载壳不得截断填满
  语义。编辑器保留自己的角色、焦点、选区与编辑动作，不伪装为表格编辑事务。
- 两列一增一减、总宽／视口不变时，换行高度仍正确更新；离屏行重现使用新尺寸。
- 普通 ListView 的 RequestLayout／Refresh／StyleChanged 失效与估计规则正确，
  单行高度改变后后续位置更新，最宽离屏行不因局部刷新造成水平回跳。
- 空模型、零列、极窄视口、宽内容和超高行不崩溃；Reveal 遵循正文坐标。
- 万行数据只创建与视口／回收池相称的 Widget；普通滚动不无条件重置全部测量。
- 排序保持选中记录；删除 Current、多选范围、过滤未知 ID、重复通知均有确定结果。
- 子控件不被抢事件，回收不串焦点，Bind／Unbind 不积累或重复释放连接。
- 调整宽度不误排序，Esc 恢复，拖动结束 hover 正确；回调删除列／记录或销毁窗口安全。
- UI 稳定模型和列不重复安装，受控／非受控状态正确，构建器替换生效，卸载清理连接。
- Snapshot 行列位置、身份、状态和动作真实，不伪造离屏子树或扩大输入能力。

窗口程序放 tests/window/widgets/table/，覆盖 GUI／UI、万行、长文本、交互单元格、
多选、排序、菜单、拖宽、亮暗切换。按 Testing.md 用中文写环境、启动、初始状态、
操作、逐项预期、平台差异、副作用、复位和退出方式。不得依赖 DevServer 验收。

实施后运行 go test ./...，按风险补相关 race、受影响目标编译及本机真实窗口操作；
Windows／macOS 的原生窗口验证单独记录，不以编译代替操作验收。每次记录环境、后端、
缩放和通过／失败／未执行，不将设计检查标记成代码或窗口测试通过。

## 13. 首版实现与验证记录（2026-10-05）

### 13.1 实现边界

- 命令式控件、列／委托生命周期、稳定 ID、固定表头、可变行高、双向滚动、
  选择／Current／Reveal、激活、列宽拖动与 Esc 回退、排序请求及表体菜单已实现。
- `widgets/ui` 使用稳定列 ID 和模型指针协调；委托保留最新构建器，受控状态在
  更新时阻断回传，未声明的宽度／选择／Current／Sort 保留现有状态。
- `widgets/style` 和 Modern 提供独立文本样式及中性选中背景。未修改 GUI 兜底样式，
  未让父控件向内容继承字体／颜色。
- 首版 `gui.TableInfo` 描述行列数、模型 ID、一基序号、Current 与排序方向；
  2026-10-05 按最新设计迁为 `widgets.TableInfo`，通过 Attributes 提供，见 §10.3。
  TableView 快照保留表头、真实可见行／单元格、滚动状态，不实现离屏 Widget。
- ListView 独立回归先复现宽度失效后仍使用旧累计定位高度的问题，修正内部定位
  缓存；实际行高变化也修正定位估计，Reveal 完成要求目标已实现且测量有效。
  RequestLayout／Refresh 公共接口不变，刷新仍保留离屏尺寸作为估计。
- 列宽取消回调执行完毕后才移除列集合；旧委托使用旧绑定快照 Unbind。
  焦点回收查询宿主实际焦点链，不依赖尚未送达的 ContainsFocus 跨越通知。
  键盘菜单等待 Reveal 完成后弹出。

未修改 platform、Painter、Window、EventDispatcher 或 UI Root／BuildContext。
没有新增存储、排序算法、二维编辑协议或公开测量失效入口。

### 13.2 包内与编译

本机为 Linux amd64、Go 1.25.8。

- 通过：`go test ./...`。
- 通过：`go test -race ./gui ./widgets/... ./theme/modern`。
- 通过：移除 DISPLAY／WAYLAND_DISPLAY 后执行
  `go test ./widgets/... -run 'Table|RulesCompose' -count=1`；新增表格逻辑不需要桌面。
- 通过：Windows amd64／macOS arm64，`CGO_ENABLED=0`，分别对 gui、widgets、
  widgets/ui、theme/modern 使用 `go test -c`，并编译窗口例程。
  这是编译检查，不是目标系统运行验收。
- 失败（既有阻塞）：额外的 Linux arm64 编译在
  `platform/linux/libs/libc/syscall_linux.go:29/:61` 报
  `undefined: syscall.SYS_PIPE`／`undefined: syscall.SYS_POLL`。
  对照 HEAD 确认这些未修改的绑定原已存在；本切片不扩展平台修改范围。

回归集中在 table_view_test.go；覆盖列所有权／原子校验、零列／空模型、同总宽度
调整换行、离屏重新测量、极窄视口、行池上界、模型重排与删除、委托重装／卸载、
焦点回收、子控件事件优先、排序／菜单／拖宽与取消，以及回调修改模型／列集合／
关闭宿主后的安全防护。UI 和主题测试验证身份、受控／非受控值、最新回调与调色板。

### 13.3 本机窗口操作

入口：`go run ./tests/window/widgets/table`。注释包含完整中文步骤；默认 UI，
`-gui` 为 GUI，`-fallback-style` 为显式兜底样式组合。使用正常桌面鼠标／键盘输入，
未使用 DevServer 或生产代码私有测试入口。

环境：Linux/X11、XFCE/Xfwm4 4.16.1，有桌面合成；原生 Native 窗口、10000 行。

| 模式／后端／缩放 | 已操作且通过 | 未执行／限制 |
| --- | --- | --- |
| UI / Software / 1x / Modern | 单选、键盘移动与激活、行内按钮、名称排序、右键菜单、拖宽及 Esc 回退、重建保留宽度、亮暗切换、末行 Reveal、水平滚动、Snapshot | 全套组合键／菜单删除尚未逐项桌面验收 |
| GUI / OpenGL / 1x / Modern | 单选、键盘移动与激活、大小排序、右键菜单、拖宽及 Esc 回退、亮暗切换、Reveal、窗口 resize、水平滚动、Snapshot | 同上；最终显式最小窗口尺寸调整后另由 Software 兜底例程复测窄窗 |
| UI / Software / 2x / Modern | 初始绘制、选择、键盘移动与激活、末行 Reveal、Snapshot | 未逐项复验菜单／拖宽／亮暗；不等同于完整 2x 验收 |
| GUI / Software / 1x / fallback | 绘制、680×510 DIP 窄窗、末行 Reveal、水平滚动、Snapshot | fallback 不跟随本例 Dark 开关；其余输入由同一控件实现及包内回归覆盖 |

Snapshot 明确断言真实逻辑数量、模型身份、Current／Selection，以及表头与每个
可见单元格的 X／宽度一致；观察到仅约 4～9 条可见行，不创建万行 Widget。
上述是功能验证，不构成帧率／性能基准结论。
本轮窗口例程的构建和截图产物位于仓库外临时目录，测试窗口均已关闭；没有
剪贴板、磁盘业务数据或系统设置修改。全仓既有测试仍可能更新原有的源码目录
PNG 产物，本轮未迁移这些测试。条件跳过不算作相应桌面场景通过。

待验收：Windows Direct2D／OpenGL／Software、macOS OpenGL 的原生操作，
以及 2x 的完整菜单／拖宽交互。进度见 Plan.md／Roadmap.md。

## 14. 边框与背景补充验证（2026-10-05）

实现范围为 widgets 的样式解析、绘制和布局，以及 widgets/style、theme/modern；
未修改 platform、GUI Painter、公共 Style 字段、UI 协调或表格交互接口。
上一切片的 GUI 缓存／TableInfo 改动保留，本次不追加 GUI 核心改动。

包内回归仍集中在 table_view_test.go：独立方向的框内布局、水平滚动后的表头偏移、
列线与行线安全空间、禁用 Resizable、最后行／列、极窄约束、非法线宽、换行重测、
透明颜色保留几何、排序及万行回收后的条纹、选择／Hover／Pressed 优先级。
Software 像素测试不依赖桌面或字体：1x/2x 整数 DIP 线带，按预乘 RGBA 采样，
允许 1 个 8-bit 色阶的合成舍入误差；验证半透明内外交点只合成一次、自绘内容
四边完整、内部线和内容不重叠、最后列不重复画右边线。

通过：`go test ./...`、`go test -race ./widgets/... ./theme/modern`、移除
DISPLAY/WAYLAND_DISPLAY 后 `go test ./widgets/... -run 'Table|RulesCompose' -count=1`。
Windows amd64／macOS arm64（CGO_ENABLED=0）的 widgets、widgets/ui、modern
测试目标及窗口程序交叉编译通过；仍不等同于原生桌面验收。

窗口入口增加 `-appearance=plain/zebra/rows/grid/thick` 和 Appearance 按钮，不增加
新的统一测试框架。部分 Action 为填满自身矩形的青色 Widget；Snapshot 除原来的
身份／几何断言外，调用 RenderWidget(table, 1) 走正常绘制遍历与当前后端，按语义
边界检查可见自绘内容四角。不输出图片，不以重绘取样代替原生窗口观察。

本机环境仍为 Linux amd64、X11、XFCE/Xfwm4 4.16.1、有桌面合成：

| 模式 / 后端 / 缩放 | 本轮通过的操作 | 范围限制 |
| --- | --- | --- |
| UI / Software / 1x / Modern | 五种外观切换，粗线及自绘色块，斑马色，滚动，列宽拖动与 Esc 恢复，重建，亮暗切换，Snapshot 几何 | 最后追加的 RenderWidget 边缘断言由下列 2x / OpenGL / fallback 模式执行 |
| GUI / OpenGL / 1x / Modern | grid/thick/plain 切换，滚动后表头与正文对齐，色块与粗线避让，亮暗切换，Refresh，Snapshot；最终程序验证实际子树重绘边缘与横向滚动 | 未逐项重验旧的菜单、组合键等全部交互 |
| UI / Software / 2x / Modern | thick 绘制与滚动，自绘色块完整，Snapshot 几何及实际子树重绘边缘 | WM 因桌面物理尺寸限制将窗口最大化；未逐项重验菜单或拖宽 |
| GUI / Software / 1x / fallback | grid/thick 切换、滚动、色块避让、Snapshot 几何及实际子树重绘边缘 | fallback 不跟随本例 Dark 开关 |

已关闭本轮测试窗口；构建和截图只写入 `/tmp` 临时目录。此前已运行的用户窗口
保留。全仓既有测试的 PNG 写入和条件跳过规则未改变；没有系统设置、业务文件或
剪贴板修改。没有性能基准结论。Windows/macOS 原生外观与交互继续列为待验收，
既有 Linux arm64 绑定编译阻塞未在本轮处理。

## 15. 正文单元格填满与编辑器例程（2026-10-05）

### 15.1 确认的契约

- 取消正文 Cell 的固定左右 8／上下 4 DIP Padding 和内容收缩／垂直居中策略。
  单元格默认且始终填满，不引入表格级、列级或逐 Cell 的 Padding／Fill 配置。
- Measure 按列内可绘制宽度测量内容；Arrange 再按该行最大期望高度，把所有
  内容分配为完整矩形。右侧列间线与底部行间线继续独占空间，不通过后画边框
  遮盖自定义 Widget。空内容和极窄列不会制造负尺寸，最后行／列不重复留线。
- 表头的原有留白不变，常量明确改为 tableHeaderPaddingX／Y，避免误用于正文。
- widgets/ui 使用普通私有 Container 和既有 FillLayout／UpdateChildren，只承载
  一棵声明子树。仍保留稳定列委托与正常 Bind／Unbind，不修改 ui.Root、BuildContext、
  gui.Widget、ListView 或 Painter。用户返回的 View 自己决定内部的留白和布局。
- Padding 继续是控件／容器自己的配置，不进入 Style，也不让父 Cell 向内容
  隐式提供字体、颜色或状态。正文 min-height 仍为 32 DIP。

参考证据：GTK4 4.18.0 的
[gtk_column_view_cell_widget_size_allocate](https://github.com/GNOME/gtk/blob/4.18.0/gtk/gtkcolumnviewcellwidget.c#L255)
把可用区域分配给子 Widget；默认主题的
[complex columnview](https://github.com/GNOME/gtk/blob/4.18.0/gtk/theme/Default/_common.scss#L3410)
把留白移到内容，使内容事件控制器覆盖整个 Cell。Avalonia.Controls.DataGrid 的
[Fluent 主题](https://github.com/AvaloniaUI/Avalonia.Controls.DataGrid/blob/main/src/Avalonia.Controls.DataGrid/Themes/Fluent.xaml#L72)
（2026-10-05 查阅 main）将 Cell 内容 Stretch，并分别配置普通文本和编辑器主题。
以上是具体源码行为，不是 GOUI 必须复制 CSS／模板系统的理由。Qt 6.8.3 的
[QStyledItemDelegate::updateEditorGeometry](https://github.com/qt/qtbase/blob/v6.8.3/src/widgets/itemviews/qstyleditemdelegate.cpp#L473)
仍通过样式的文字子区域确定编辑器矩形，不将各框架都描述成一律填满。
GOUI 采用填满分配和内容自主管理的边界，不采用它们的样式 Padding／属性系统。

### 15.2 窗口例程

tests/window/widgets/table 保留原四列验收，增加 Notes 编辑列，GUI／UI 都使用
普通 TextInput。普通文本通过自己的 HBox／LinearBox 提供 4 DIP 留白；Cell
没有为 Label／Button／TextInput 分支判断布局。

例程使用独立 table-cell-input 样式：从当前编辑器样式规则显式组合字体、前景、
caret／selection／焦点颜色，默认背景透明、无额外边框、Radius=0；Focused
为向内 2 DIP 主题色直角边框。普通 TextInput 和全局 Modern 样式不变。
TextInput 内部 4 DIP 文字 Padding 不缩小其外部绘制／命中矩形。

应用把编辑值存储在 RowID -> string 中，空字符串也是有效值。GUI Bind 防止
SetText 回送业务写入，Unbind 清除旧 ID；UI OnText 写入相同业务存储，不在每次
键入时重排整个表格。排序、回收和 Rebuild／Refresh 按 RowID 恢复文字；删除记录
清理对应编辑值。这是例程业务，不新增 TableView 的编辑状态机或持久化接口。

Snapshot 按钮检查五列语义和虚拟化，递归查找 Notes 内真实 TextInput 并验证
边界与业务文字。F6 走正常快捷键输入路径，不夺取当前输入焦点；焦点存在时还用
RenderWidget 重绘并采样四角颜色，验证 2 DIP 直角边框。此为当前子树离屏验证，
不能代替桌面合成、1x／2x 裁剪与实际编辑体验。完整中文操作步骤位于入口注释。

### 15.3 验证记录

- 已复现旧实现：新增 TestTableViewCellContentFillsAllocation 得到行高 77 而非
  69；TestTableViewTextInputFillsCell 得到壳 (8,4,160,26.666664)，预期为
  (0,0,200,32)。修改后两条回归通过。
- 既有网格像素、粗线避让和换行／虚拟化断言按新几何契约调整，保留有效覆盖。
  新增例程逻辑测试验证独立样式、普通 TextInput 不受影响和编辑值绑定／回收。
- 通过：`go test ./...`、`git diff --check`。
- 通过：`go test -race ./widgets/... ./tests/window/widgets/table`。
- 通过：移除 DISPLAY／WAYLAND_DISPLAY 后执行
  `go test ./widgets/... ./tests/window/widgets/table -run 'Table|CellEditor|NoteCell' -count=1`。
- 通过：Windows amd64／macOS arm64（CGO_ENABLED=0）分别编译 widgets、
  widgets/ui、窗口例程的测试目标及窗口程序；这是编译检查，不是原生运行。

本机窗口环境为 Linux amd64、X11、XFCE/Xfwm4 4.16.1、有桌面合成，Modern／Native
窗口、10000 行。通过真实鼠标／键盘输入，不使用 DevServer；用 XMODIFIERS=@im=none
限定本轮 ASCII 输入，不将结果扩展为 IME 验收。

| 模式 / 后端 / 缩放 | 本轮通过的操作 | 范围限制 |
| --- | --- | --- |
| UI / Software / 1x | Notes 输入、整格焦点直角边框和 F6、亮暗、重建、grid/thick、滚动回收、名称升降序后文字按 ID 保留、Notes 拖宽后填满 | 未重验菜单、剪贴板及全部组合键 |
| GUI / Software / 1x | Notes 输入、焦点边框和 F6、亮暗、Refresh、grid/thick、滚动回收后文字保留 | 未逐项重验排序及菜单 |
| UI / OpenGL / 1x | Notes 输入、焦点边框和 F6、亮暗、grid/thick、滚动回收后文字保留 | 未逐项重验拖宽、重建及菜单 |
| UI / Software / 2x | Notes 输入、焦点边框和 F6、亮暗、grid/thick，观察 2 DIP 为 4 物理像素且不覆盖网格 | WM 受桌面物理尺寸限制将窗口最大化；未逐项重验拖宽、排序、滚动及菜单 |

F6 均通过真实输入触发，断言编辑器与内容区同大小、业务文字一致、四角颜色正确，
编辑后 Current 为空且 Selection=[]。1x 滚动实测 scrollY=475 后回到零，原记录
文字仍在；UI 拖宽实测 Notes 从 200 到 280 DIP，thick 内容宽随之从 188 到 268。
已有包内 1x／2x Software 像素测试继续验证预乘 RGBA 网格与内容边界，允许 1 个
8-bit 色阶合成舍入；F6 的不透明焦点边框四角要求精确匹配，没有扩大容差。

本轮测试窗口已关闭，构建／截图产物只存于仓库外临时目录，没有修改系统设置、
剪贴板或业务文件。全仓既有测试仍可能生成源码目录中的 PNG，未迁移或删除；
条件跳过不算桌面场景通过。不作性能结论。

未执行：Windows／macOS 原生桌面、Direct2D、OpenGL 2x、IME／编辑菜单／全部
组合键；Linux arm64 的既有绑定编译阻塞仍保留为此前记录，本轮没有重复尝试。
上述缺口不以交叉编译或其它后端替代。
