# TreeView：通用数据树

> 状态：2026-10-03 首版代码、声明绑定、样式、语义和测试已实现。
> 本机 Linux/X11/Xfwm4 已完成 Software 1x UI、OpenGL 2x GUI 核心交互验证；
> Windows amd64／macOS arm64 编译通过，原生桌面验收尚未执行。详细记录见 §9.4。

## 1. 范围与架构

首版面向项目目录、资源管理、分组记录等通用数据树，提供：

- 多根节点、展开／折叠、可变行高及虚拟化。
- 单选、多选、当前项和树内键盘导航。
- 按需加载、自定义行内容、激活和上下文菜单。
- GUI 命令式控件、UI 声明绑定、兜底／Modern 样式和语义快照。

暂不实现树内拖放重排、拖放悬停展开、行内重命名、多列树或内置排序／过滤 UI。
行内容可以组合现有 DragSource／DropTarget，但 TreeView 不因此自动修改业务树。
排序、过滤和异步加载的数据策略由模型负责。

| 位置 | 职责 |
| --- | --- |
| `widgets` | TreeModel、TreeStore、TreeView、行容器、交互及树角色 |
| `widgets/ui` | Model＋行构建器的声明绑定 |
| `theme/basic` | GUI 自动使用的完整兜底样式；widgets/style 只作兼容转发 |
| `theme/modern` | Modern 外观 |
| `gui` | 列表项定位与既有可见性请求、语义层级信息；实际滚动仍由 ScrollView 负责 |
| `ui` | 延迟构建时有效的 BuildContext 及其生命周期；不承载树状态机 |

内部组合：

```text
ScrollView                         滚动位置、裁剪、滚动条
└── TreeView                       模型、展开、选择、当前项
    └── ListView                   平铺行的测量和虚拟化
        ├── treeRow
        │   ├── treeExpander       展开按钮或等宽占位
        │   └── delegate content  普通 Widget 子树
        └── …
```

TreeView 实现 `gui.Scrollable`，作为 ScrollView 的内容使用，不另建滚动条，
也不查找祖先后直接修改 ScrollView 的私有偏移。数据树、展开后的逻辑行和实际挂载的
Widget 是三个不同层次；节点不等同于 Widget。

### 1.1 参考与取舍

- GTK4 的 [TreeListModel](https://docs.gtk.org/gtk4/class.TreeListModel.html)
  将树展开为列表，再配合列表控件和展开器。参考
  [GTK 4.20.0 实现](https://github.com/GNOME/gtk/blob/4.20.0/gtk/gtktreelistmodel.c)，
  采用“模型＋展开平铺＋可复用行”的分工，不引入 GObject 属性系统。
- [Qt 6.8.0 QTreeView](https://github.com/qt/qtbase/blob/v6.8.0/src/widgets/itemviews/qtreeview.cpp)
  的展开项表示和导航实现用于对照可见行、当前项与滚动的关系。
  不复制 QModelIndex／持久索引体系，GOUI 以模型内稳定字符串 ID 标识节点。
- [Avalonia 11.3.0 TreeView](https://github.com/AvaloniaUI/Avalonia/blob/11.3.0/src/Avalonia.Controls/TreeView.cs)
  用于对照选择、展开与输入处理的职责。不引入其模板／属性体系。

上述是控件层设计和源码参考，不是原生平台行为的承诺，也不表示 GOUI 的全部交互
与某一框架完全一致。以下规范是 GOUI 的设计选择，实际结果须由后续实现和测试确认。

## 2. 数据模型

### 2.1 TreeModel 与 TreeData

```go
type TreeModel interface {
    Children(parentID string) []string
    Parent(id string) (parentID string, exists bool)
    Expandable(id string) bool
    ConnectItems(func()) signal.Handle
}

type TreeData[T any] interface {
    TreeModel
    Item(id string) T
}
```

TreeView 本身不泛型化；数据访问、内置 Store 和 UI 行构建器使用泛型，沿用现有
ListModel／ListData 的职责划分。

模型契约：

1. `Children("")` 返回有序根节点。空字符串是不可见的虚拟根，不是数据节点。
2. 节点 ID 非空且在一个模型内唯一。改名、数据更新、排序和换父级不改变节点 ID。
3. 根节点的 `Parent` 返回 `("", true)`，不存在的节点返回 `("", false)`。
4. 父子关系一致、无环，一个节点只能有一个父级。
5. `Expandable` 表示节点是否可展开，不等同于当前子项数量。未加载或暂时为空的分支
   仍然可以显示展开器。
6. 查询方法是同步内存查询，不执行阻塞 I/O，也不在查询过程中修改模型或发送通知。
7. 调用方只读 `Children` 返回的切片。TreeStore 返回副本，不将内部子项数组交给调用方。
8. 一次模型变更完整提交后同步发出 Items 通知。模型被挂载到界面期间在 GUI 线程访问；
   后台结果通过 Application 投递回 GUI 线程。

节点 ID 与 `Widget.ID` 完全独立，不自动调用 `SetID`。同一模型可以在同一窗口的
多个 TreeView 中显示，展开和选择互不污染，也不会产生 Widget ID 冲突。

只遍历展开路径，不为校验或统计总数提前查询全部折叠后代。自定义模型违反契约时，
在实际访问到错误关系的位置明确报告契约错误，不悄悄截断成一棵看似正常的树。
模型是程序内部接口，此类错误不转成“加载失败”或普通用户取消。

### 2.2 内置 TreeStore

```go
func NewTreeStore[T any]() *TreeStore[T]

func (m *TreeStore[T]) Insert(parentID string, index int, id string, value T)
func (m *TreeStore[T]) Append(parentID string, id string, value T)
func (m *TreeStore[T]) Set(id string, value T)
func (m *TreeStore[T]) Remove(id string)
func (m *TreeStore[T]) SetExpandable(id string, expandable bool)
func (m *TreeStore[T]) Modify(func(*TreeStore[T]))
```

内部使用 ID 到记录的映射，以及每个父级的有序子 ID。实现 TreeData，供普通应用
直接使用；业务已经有树结构时可自行实现模型，不必复制到 Store。

- Insert 只插入新 ID；重复 ID、不存在的父级和越界索引属于编程错误。
- Set 只替换业务值，不改变父子关系。T 为任意类型，不通过比较 T 推断变化。
- Remove 递归移除子树；删除不存在的 ID 不产生变化或通知。
- Expandable 为“显式可展开标志，或实际拥有子节点”。将标志设为 false 不隐藏已有子项。
- Modify 支持嵌套，最外层结束时最多发出一次通知；这是通知合并，不是事务回滚。
- 不承诺并发安全。首版不为尚未实现的拖放重排增加 Move 等接口。

示例：

```go
store := widgets.NewTreeStore[Entry]()
store.Modify(func(m *widgets.TreeStore[Entry]) {
    m.Append("", "project", Entry{Name: "项目"})
    m.Append("project", "src", Entry{Name: "src"})
    m.SetExpandable("src", true)
    m.Append("project", "readme", Entry{Name: "README.md"})
})
```

### 2.3 按需加载

可选模型能力：

```go
type TreeChildrenRequester interface {
    RequestChildren(id string)
}
```

加载请求不能只依赖应用监听 OnExpanded：声明式初始展开或受控展开在协调时不应
回送 UI 状态回调，但仍然必须触发数据加载。因此请求由 TreeView 依据模型能力发起。

- 节点自身和全部祖先均展开，且 TreeView 已挂载时，激活该节点的加载请求。
- 每次激活周期请求一次；Measure、Paint、滚动和普通数据刷新不重复请求。
- 折叠后再次展开、从祖先折叠中重新显现、卸载后重新挂载可以再次请求。
  模型通过 Loaded／Loading 状态保证幂等，不把重复请求等同于强制重新加载。
- 未挂载时允许配置 Expanded，但不执行加载。请求发生在状态提交后，不在模型查询、
  Measure 或 Paint 内发起。
- RequestChildren 可以同步补充内存数据，或启动异步操作，但不能阻塞 GUI 线程。
- 异步任务、取消、加载状态、错误和重试由模型／应用管理，不由 TreeView 启动或持有
  业务 goroutine。结果通过模型通知刷新行。
- 折叠一个 TreeView 不自动取消共享模型的加载。删除节点或复用 ID 时，模型用请求代次
  或记录身份校验结果，避免旧请求污染新节点。

加载中／失败可以由业务数据和行构建器呈现；需要占位子项时，它是普通模型节点。
不在首版额外添加 LoadingWidget／ErrorWidget 或另一套错误弹窗策略。
GTK4 对叶子与暂时为空分支的区分可参考
[TreeListModelCreateModelFunc](https://docs.gtk.org/gtk4/callback.TreeListModelCreateModelFunc.html)。

## 3. TreeView 公共接口与状态

### 3.1 接口

```go
type SelectionMode uint8

const (
    SelectionSingle SelectionMode = iota
    SelectionMultiple
)

func NewTreeView() *TreeView
func (v *TreeView) Model() TreeModel
func (v *TreeView) SetModel(TreeModel)
func (v *TreeView) Delegate() TreeItemDelegate
func (v *TreeView) SetDelegate(TreeItemDelegate)
func (v *TreeView) SelectionMode() SelectionMode
func (v *TreeView) SetSelectionMode(SelectionMode)
func (v *TreeView) Selection() []string
func (v *TreeView) SetSelection([]string)
func (v *TreeView) Current() string
func (v *TreeView) SetCurrent(string)
func (v *TreeView) Expanded(id string) bool
func (v *TreeView) SetExpanded(id string, expanded bool)
func (v *TreeView) ExpandedIDs() []string
func (v *TreeView) SetExpandedIDs([]string)
func (v *TreeView) Indentation() float32
func (v *TreeView) SetIndentation(float32)
func (v *TreeView) Reveal(id string)
func (v *TreeView) Refresh()
func (v *TreeView) RowAt(geometry.Point) (TreeRow, geometry.Rectangle, bool)
```

默认单选、无选择、无当前项、全部折叠，缩进为 16 DIP。

| 操作 | 含义 |
| --- | --- |
| SetSelection | 只改变选择，不隐式改变当前项、展开状态或焦点 |
| SetCurrent | 只改变当前项，不隐式选择、展开或抢焦点 |
| SetExpanded | 改变一个节点的展开偏好 |
| SetExpandedIDs | 替换完整展开集合，不自动展开祖先 |
| Reveal | 展开已知祖先并请求垂直滚动，不改变选择或当前项 |
| Refresh | 请求重新绑定已实现的行，不改变模型、展开、选择或当前项 |

选择和当前项只指向展开平铺中的逻辑行，可以在视口外，但不能位于折叠分支内。

- SetSelection 过滤未知、隐藏和重复 ID；单选取输入中第一个有效 ID。
  Selection 按逻辑行顺序返回副本。
- 多选切换为单选时保留逻辑顺序中的第一个已选项，不额外改变当前项。
- SetCurrent 空字符串表示清空，其他无效 ID 不改变现状。
- ExpandedIDs 保留折叠祖先内后代的展开偏好，返回按 ID 字典序排列的副本。
  SetExpandedIDs 忽略未知和不可展开节点，不为后续尚未加载的 ID 预存状态。
- Reveal 仅定位模型已知的节点，不搜索尚未加载的后代。未知 ID 不执行操作。
- Indentation 是局部 DIP；负数归零，非有限数忽略。
- SetModel 是显式安装：即使传入同一实例，也断开旧订阅、重新连接，清空展开、
  选择、当前项和待定位状态并关闭菜单。nil 清空模型，不比较接口值。
  数据变化使用模型 Items 信号，表现变化使用 Refresh，不通过 SetModel 隐式刷新。
- SetDelegate 即使传入同一实例，也解绑旧行并清理旧行壳池，但保留展开、选择和
  Current。普通内容刷新使用 Refresh。生命周期、外部回调重入及状态实际变化通知
  的规则不变；不把此次约定推广到所有 GUI setter。

### 3.2 内部状态

TreeView 保存模型、delegate、展开集合、选择集合、current ID、范围选择 anchor ID，
以及展开平铺数组、ID 到索引的映射、已实现行、待 Reveal ID 和状态 revision。

平铺行只缓存 ID、父 ID、深度、兄弟序号及兄弟数量，不复制业务数据。区分：

- 存在：模型能定位该节点。
- 逻辑可见：祖先已展开，节点进入平铺数组。
- 已实现：当前虚拟化范围内实际创建／绑定了行 Widget。

索引仅用于布局和虚拟化，公共状态、信号、异步操作和待定位目标均使用 ID。
选择和当前项变化不重建平铺；完整模型通知可以重建平铺，但不创建全部行 Widget。

### 3.3 折叠、删除和数据更新

折叠节点时：

1. 从平铺中移除后代，保留其展开偏好。
2. 移除隐藏后代的选择。
3. 若当前项被隐藏，将当前项移到被折叠的节点并选择该节点。多选保留分支外选择；
   单选只保留该节点。
4. 若范围选择锚点失效，改为新的当前项。

模型更新时：

- 同 ID 的数据替换、排序和换父级保留状态，再按新的逻辑可见性归一化。
- 删除的节点从选择和展开集合移除。
- 当前项删除后，依据更新前保存的祖先信息，优先定位最近仍存在且逻辑可见的祖先，
  否则取旧索引附近的逻辑行；空树则清空。
- 若被删除的当前项原本已选中，选择随之迁移到回退项；否则只调整当前项。
- 换父级导致当前项隐藏时也遵循可见性约束，不偷偷展开新祖先。
- 空树清空选择、当前项和失效锚点。

### 3.4 信号与重入

```go
func (v *TreeView) ConnectSelection(func([]string)) signal.Handle
func (v *TreeView) ConnectCurrent(func(string)) signal.Handle
func (v *TreeView) ConnectExpanded(func(string, bool)) signal.Handle
func (v *TreeView) ConnectActivate(func(string)) signal.Handle
func (v *TreeView) ConnectContextMenu(func(string, *gui.MenuModel)) signal.Handle
func (v *TreeView) ConnectContextMenuError(func(error)) signal.Handle
```

GUI setter 在状态实际变化时通知，无变化不通知。先完整提交一致状态，再依次通知
Expanded、Selection、Current；接收者可以立即读取新的完整状态。

完整展开集合更新对每个变化 ID 至多通知一次，采用稳定顺序。模型删除展开节点时
允许发出该 ID 的 false，回调不能假设节点仍然存在。更换模型不逐个发送旧模型的
折叠事件，但选择和当前项清空产生的实际变化仍通知。

Selection 参数不暴露内部切片，每个接收者获得独立副本。回调可能改模型、再调用
setter、卸载或销毁控件；revision 只代表生命周期、模型／平铺或绑定代次，Selection
与 Current 另外核对通知值是否仍然有效。只改变 Current 不应中断仍有效的 Selection
通知；同种状态已被回调改变时，跳过旧值。外部调用后停止基于旧索引／Widget 的处理，
不能通过清空信号规避问题。

模型在通知过程中再次变更时立即失效旧代次、标记待同步，并在当前提交结束后处理最新状态，不丢弃
重入通知，也不继续发送已经失效的旧状态通知。

## 4. 输入和上下文菜单

### 4.1 焦点与手势

TreeView 持有真实键盘焦点，当前项保存为 ID。普通行和展开器不分别进入焦点序列，
虚拟化回收行不会造成焦点丢失。自定义行内的按钮、编辑器等仍可正常获取焦点；当焦点
在这些子控件上时，树不抢占其键盘输入。

复用现有事件控制器、点击／双击和拖动竞争，不修改 EventDispatcher 增加树特例。
行内交互控件消费的点击不再选中或激活整行；拖动胜出后不补发点击。

| 输入 | 行为 |
| --- | --- |
| 单击行体 | 设置当前项、单选该项、设置范围锚点 |
| Primary＋单击 | 多选模式切换该项选择，更新当前项和锚点 |
| Shift＋单击 | 从锚点到目标的逻辑行替换范围选择 |
| Primary＋Shift＋单击 | 将该范围加入已有选择 |
| 单击展开器 | 只展开／折叠，不因这次点击额外选中或激活 |
| 双击分支行体 | 展开／折叠，不发 Activate |
| 双击叶子行体 | 发 Activate |
| Enter | 激活当前项，分支也允许发 Activate |
| Up／Down、Home／End | 导航当前项并单选 |
| Primary＋上述导航键 | 只移动当前项，不改变选择 |
| Shift＋上述导航键 | 以原锚点替换范围选择 |
| Primary＋Shift＋上述导航键 | 添加范围选择 |
| Left | 已展开则折叠，否则定位并选择父节点 |
| Right | 可展开且未展开则展开，否则定位并选择首个子项 |
| PageUp／PageDown | 按当前视口可见行数减一导航，最少一步 |
| Space | 单选当前项 |
| Primary＋Space | 多选模式切换当前项选择 |
| Primary＋A | 多选模式选择全部逻辑可见行，不含折叠后代 |
| 单击空白 | 清空选择，保留当前项 |
| Shift＋F10 | 请求当前项上下文菜单 |

范围选择按当前平铺顺序计算，不受 Widget 是否实现影响。连续 Shift 导航保持锚点，
反向移动可以缩小范围；普通选择重置锚点。Primary＋Shift 是叠加，不因反向移动删除
此前选择。单选模式不启用多选修饰行为。

无当前项时首次导航以首个已选项或首行作为起点；End 定位末行。用户键盘导航请求
Reveal；纯 SetCurrent 不自动滚动。修饰键复用 `gui.KeyGesture` 和 `ModPrimary`，
由既有 GUI 转换区分 macOS Command 和其他平台 Control，不自行读取平台按键。

### 4.2 上下文菜单

TreeView 拥有用于本控件的 PopoverMenu，沿用 TabBar／TabView 的查询信号模式，
不将菜单静态写死在模型里。

- 右击已选行：保留多选，更新当前项。
- 右击未选行：单选并设为当前项。
- 右击空白：查询 ID 为空字符串，保留现有选择。
- 行内容优先处理右键；TreeView 在冒泡阶段处理未消费的事件，不在捕获阶段抢走
  TextInput 或其他交互子控件的菜单。子控件停止传播时不改变树选择，也不查询树菜单。
- macOS Control＋单击沿用已有上下文菜单手势，不与 Primary 多选混淆。
- 查询结果初始化为 nil，连接顺序决定覆盖顺序；nil 或空菜单表示不显示。
- 指针触发以 TreeView 为锚，使用局部坐标。键盘触发先 Reveal，等待布局完成后用
  目标行位置弹出，不用尚未更新的矩形。
- 目标被折叠、滚出实现范围、模型替换或控件卸载时关闭菜单，避免旧目标操作。
  首版模型为整树 Items 通知，没有变更范围；任何 Items 通知都关闭已显示菜单，
  包括只修改标题的通知，不能据此猜测旧菜单动作仍然有效。
- 展示失败通过 ContextMenuError 通知，不重复日志或自动弹出错误对话框。

## 5. 行构建、布局与虚拟化

### 5.1 Delegate

```go
type TreeRow struct {
    ID         string
    Depth      int
    Expandable bool
    Expanded   bool
    Selected   bool
    Current    bool
}

type TreeItemDelegate interface {
    Setup() gui.Widget
    Bind(row TreeRow, widget gui.Widget)
    Unbind(row TreeRow, widget gui.Widget)
}
```

2026-10-04：delegate 的 Setup 改为提供整行内容，使用公开 TreeExpander 组合
缩进、展开器和 Child；内部行容器仅负责选择背景、行级输入与层级语义。
widgets/ui.TreeView 的声明形式不变，由适配层组合 TreeExpander。详细迁移、
RowAt 命中契约和验证见 [交互扩展设计](DesignInteractionExtensions.md)。

- Setup 对池内内容 Widget 创建一次；固定的控制器和连接在此建立。
- Bind 可以对同一 ID 多次调用，用于数据、选择、当前项或展开状态更新。
- 换绑到其他 ID 前必须 Unbind。解绑参数采用上次绑定时保存的 TreeRow，不能用新模型
  的旧索引重新查询。
- Bind 内建立的动态连接必须在重新绑定／解绑时释放，不能每次累积。
- 离屏回收不保证保留内容 Widget 的临时状态；需要持久化的业务状态放模型。
- delegate 只更新自己的行内容，不在 Bind、Measure 或 Paint 中修改模型或重建祖先。
- 行保存已绑定 ID、delegate 和版本；调用外部 Unbind 前先使旧绑定失效，避免重入重复清理。

更换 delegate 时，旧行由旧 delegate 完成 Unbind，再清理旧池并启用新 delegate。
ListView.SetDelegate 已先用旧 delegate 解绑旧行、清旧池，再安装新 delegate；
这一前置修正已落地，不再列为待实现。回归仍防止新 delegate 处理旧壳。

### 5.2 尺寸与绘制

TreeExpander 默认布局参数：根深度为 0，水平内边距 4 DIP，每层缩进 16 DIP，展开器槽宽 20 DIP，
叶子也保留同宽占位；行最小高度 28 DIP，上下内边距各 4 DIP。内容和展开图标垂直居中，
行高允许随字体、多行内容及自定义控件增长。

行选中／悬停背景覆盖包括缩进在内的完整行宽。缩进是布局状态，不是向子树传递样式。
测量、排列通过现有布局辅助机制执行，不能直接调用子控件 Measure 绕过样式和测量缓存。

保留内容自然宽度，允许 ScrollView 水平滚动；不强制所有内容按视口宽度换行。
应用可通过内容的宽度约束控制换行。颜色、圆角等按第 7 节解析，不能从行向 Label
隐式继承样式。

展开平铺重建复杂度为 O(展开后的逻辑行数)，首版不实现专用平衡树。
Widget 数量受虚拟化实现范围和复用池约束，一万个节点不能创建一万个行 Widget。

刷新区分三类，不将状态变化一律当成结构变化：

- Selection／Current：布局时只重新绑定 TreeRow 快照改变的已实现行，保留其他行的
  内容子树和尺寸缓存。
- 模型 Items 通知但平铺结构不变，或显式 Refresh：重新绑定可见内容，不卸载原行；
  ListView 保留离屏尺寸作为估计，重新实现时重新测量，避免暂时丢失最宽行而横向回跳。
- 平铺结构改变：旧索引解绑、清理索引尺寸缓存，但同一 delegate 的行壳进入复用池，
  不丢弃整池再创建。更换 delegate 不复用旧 delegate 创建的行壳。

2026-10-04 后续设计补充：ListView 的历史尺寸属于其内部虚拟化缓存，不向 TreeView
或其他调用方增加 InvalidateMeasurements 等公开失效接口。局部内容变化继续使用
RequestLayout，整体表现变化使用 Refresh；历史值失效后只能作为估计，实际行重新
实现时必须修正。缓存有效性与定位的一致性将由独立回归确认，具体契约见
[TableView §6](DesignTableView.md#6-listview-稳定契约不新增测量失效接口)。
此项为待验证的契约完善，不表示本节既有实现已完成后续修正。

### 5.3 目标定位与既有滚动机制

ScrollView 已有 `ScrollIntoView(rect)` 和 `ScrollIntoViewRequester` 接入机制，负责
偏移计算、边界钳制、布局期间暂存请求，以及内容尺寸更新后的执行。TreeView 不重做
这些机制，也不直接修改 ScrollView 的偏移。

缺少的是“数据项到内容坐标”的定位能力，而不是滚动能力。职责分工为：

```text
TreeView.Reveal(id)       节点 ID → 展开已知祖先 → 平铺行索引
ListView.Reveal(index)    行索引 → 内容坐标矩形（先估算，测量后纠正）
ScrollView               内容坐标矩形 → 实际滚动偏移
```

给 ListView 补充目标可见性请求，统一使用 Reveal 命名，不增加 ScrollTo：

```go
func (v *ListView) Reveal(index int)
func (v *ListView) ConnectRevealed(func(index int)) signal.Handle
func (v *ListView) ConnectScrollIntoView(func(geometry.Rectangle)) signal.Handle
func (v *TreeView) ConnectScrollIntoView(func(geometry.Rectangle)) signal.Handle
```

ListView 掌握行高缓存及估计值，只负责生成目标矩形。TreeView 是 ScrollView 的直接
内容，需要连接内部 ListView 的请求并转发为自身的可见性请求，遵守未减去滚动偏移的
内容局部 DIP 坐标约定；内部布局存在位移时在这一边界换算。ScrollView 仍只连接自己的
直接内容，不为 TreeView 穿透查询内部子树。转发连接随控件生命周期清理。

沿用 ScrollIntoViewRequester／ScrollView 路径的定位规则：

1. 首次布局前可以记录待定位索引。
2. 未实现目标先发送估计矩形，请 ScrollView 移动到附近；目标实现并测量后发送纠正矩形。
3. 定位完成后清除请求，不能持续把用户滚动拉回原处。新请求覆盖旧请求；无效索引无操作。
4. 超过视口高度的行沿用 ScrollView 的顶部对齐规则，不在列表内复制偏移计算。
5. 目标矩形的水平范围按当前视口选取，不要求将整条宽行横向滚入；保留水平滚动位置。
   模型替换或目标失效时取消旧请求。

TreeView 保存待 Reveal 的 ID，平铺变化后重新解析索引，不能把旧索引当成新节点。
完成条件（精确测量、超高行、末行不足 1 DIP 的滚动舍入余量）只由 ListView 判断，
通过 Revealed 通知 TreeView 清除待定位 ID；不在 TreeView 复制相同几何算法。
普通滚动及模型替换取消请求均不发送完成通知。
列表只保存待定位目标和现有 LayoutVisible 提供的视口信息，不成为第二个滚动状态所有者。
模型更新后的数值偏移保留及范围钳制仍由 ScrollView 完成；首版不承诺大规模重排时
精确保持原视口节点的像素锚点。

## 6. UI 声明绑定

### 6.1 使用方式与接口

```go
func TreeView[T any](
    model widgets.TreeData[T],
    item func(widgets.TreeRow, T) ui.View,
) *TreeViewView[T]
```

使用链式方法，与既有声明式控件保持一致：

- 数据与构建：`Model(...)`、`Item(...)`。
- 状态：`SelectionMode(...)`、`Selection(...)`、`Current(...)`、`Expanded(...)`。
- 布局：`Indentation(...)`，以及通用 View 修饰方法。
- 通知：`OnSelection(...)`、`OnCurrent(...)`、`OnExpanded(...)`、`OnActivate(...)`。
- 菜单：`ContextMenu(func(id string) []*ui.MenuItemView)`、`OnContextMenuError(...)`。

示意代码，回调中的状态和业务函数由应用提供：

```go
ui.ScrollView(
    wui.TreeView(store, func(row widgets.TreeRow, entry Entry) ui.View {
        return ui.Label(entry.Name).Style("tree-item-text")
    }).
        SelectionMode(widgets.SelectionMultiple).
        Selection(selected).
        OnSelection(func(ids []string) {
            selected = ids
            app.RequestUpdate()
        }).
        OnActivate(openEntry).
        ContextMenu(menuForEntry),
)
```

这里 `wui` 指 `widgets/ui`。不引入另一套声明式树节点对象；模型决定关系，构建器
只把一行数据映射为 View。菜单复用现有 MenuItem 声明协调和连接管理。

### 6.2 受控与非受控状态

Selection、Current、Expanded、Indentation 用 `bits.Bitmap` 区分未声明和显式零值，
不为每项重复添加 bool 字段。

- 未声明 Selection／Current／Expanded 时保留 Widget 内部状态。
- 显式空切片／空字符串清空对应状态；声明过后再移除，转为保留现状的非受控模式。
- SelectionMode／Indentation 未声明时采用默认值，移除声明时恢复默认。
- 协调受控 setter 时抑制 UI 回调，避免重建回送；实际输入和模型变化引发的状态通知
  正常传递。不能因此抑制按需加载请求。
- OnExpanded 是单节点变化通知。应用维护受控展开集合时据此增删 ID，不由 UI 另建状态机。

协调顺序：先更新构建器和回调，再配置 Model／Delegate、选择模式／缩进，随后依次
应用 Expanded、Selection、Current，最后 Refresh。先展开再设置选择，避免将合法的
受控选择当成隐藏节点过滤。

重复安装由声明协调层避免：通过 TreeView.Model() 读取实际已安装模型，同类型的
同一个非空指针不再调用 SetModel；新指针即使内容相同，也重新安装模型。GUI 不为
声明层猜测对象等价性。Delegate 在 Mount 时建立并随协调状态复用，不在每次 Update
重新设置，首次声明也只安装一次模型。无需增加 appliedModel 副本或 Root 特例。

需要跨重建保留状态的模型，应在重建之外创建并复用指针实例。非指针值实现仍受支持，
但每次 Update 都按重新安装处理，不做字段／深比较；未声明的树状态因此会重置，
受控 Expanded／Selection／Current 则在安装后重新应用。nil 接口清空模型；typed-nil
指针不是有效模型实例，不将其隐式转换为 nil。两个绑定共用仓库内部
internal/identity.SamePointer，仅识别上述引用关系，不作为公开 GUI 身份 API。

构建器通过当前模型和 ID 读取数据，不捕获旧索引。模型对象不变但闭包或外部显示状态
变化时，也必须重绑已实现行；不比较函数值，不用 View 对象身份替代节点身份。

### 6.3 延迟行构建与 BuildContext

`UpdateChild`／`UpdateChildren` 已属于 BuildContext，继续作为声明式子树构建的唯一
入口。Coordinator 保留已有显式子树转移职责，不添加同名构建方法，也不扩展为通用
构建器。本设计撤回之前的 Coordinator 接口扩展。

虚拟化列表在后续布局／滚动中调用 delegate.Bind，此时普通 View.Update 已经结束，
行内容仍需要通过 BuildContext 协调。原实现保存创建时的 Root，既与禁止保留 Context
的旧注释冲突，也会在显式子树转移后访问旧 Root。

**已实施的通用方案**：BuildContext 是所属 View 节点的借用上下文，可由虚拟化适配器
保留，不延长节点生命周期、不增加公共句柄类型或第二套协调入口：

- Context 只保存所属 node。每次 UpdateChild／UpdateChildren 根据 `node.root` 获取
  当前 Root；显式移交后跟随节点，节点释放后更新是 no-op，不能重新挂载旧内容。
  目标仍必须是该 View 注册和管理的稳定 Bin／Container，不取得任意树的修改权。
- 普通 View.Update 内的子协调复用当前协调范围。延迟 Bind 内的子协调由既有
  UpdateChild／UpdateChildren 在内部进入并退出本次协调范围；期间禁止嵌套整树更新，
  也禁止在显式移交过程中构建。用户状态变化继续 RequestUpdate，不同步递归重建。
- 普通窗口批次的 AfterUpdate 时序保持不变。延迟构建若产生完成回调，在子协调结束后
  通过 GUI 主循环 Post 执行，不在 Bind 内 flush，因此外层行已有机会完成挂载。
  执行前仍检查 owner 的 Root、释放状态和版本；卸载、替换或移交后的旧回调跳过。
  这不把 AfterUpdate 扩大为“所有布局或绘制已完成”的承诺。
- Bind 使用当前构建器和当前模型数据协调行内容；Refresh 可更新同 ID 的可见行。
  Unbind 清空行目标，释放声明子树和连接。行构建器可以销毁宿主，返回后只使用已借用
  的 Context，由其生命周期检查拒绝迟到构建，不再次读取可能已清空的适配器字段。
- Root 清理协调状态不夺走 Window 对 Widget 销毁的最终所有权。

ui.ListView 与 widgets/ui.TreeView 共用上述机制。回归覆盖同模型新构建器刷新、显式
移交后延迟构建、所属节点释放后 no-op，以及完成回调只在行挂载后执行。
不向 Root 添加 TreeView 特例，不扩大成属性／响应式系统。

## 7. 样式和语义快照

### 7.1 样式

| StyleName／Part | 用途 |
| --- | --- |
| `tree-view` | 树背景，默认透明 |
| `tree-item` | 行背景、边框、圆角、普通／悬停／按下状态 |
| `tree-item` 的 `selected` Part | 选中外观 |
| `tree-item` 的 `current` Part | 当前项指示 |
| `tree-expander` | 展开图标和命中槽 |
| `tree-item-text` | 默认文本内容显式使用的样式 |
| `tree-item-icon` | 图标内容显式使用的样式 |

兜底样式在 `theme/basic`，Modern 样式在 `theme/modern`。父行不隐式为 Label／Icon
提供前景色；内容选择自己的 StyleName，状态需要区别时通过行构建器明确表达。

Modern 使用中性悬停色、可辨认的中性灰选中背景、4 DIP 行圆角；树选中背景和当前项
边框只随亮暗模式切换，不随主题色变化。选中底色复用菜单按下色，悬停／按下分别向
文字色混入 4%／8%，形成轻微加深（亮色）或提亮（暗色）。默认只有树持有真实
焦点且 FocusVisible 生效时，当前项显示 1 DIP 中性色边框，用于区分“当前但未选中”。不把 Current 伪装为
焦点状态，不因父行被选中自动修改自定义子控件。首版不画树连接线、不引入展开动画。
样式变更走既有 StyleChanged 和布局／绘制失效路径。

亮色 selected 的 Normal／Hovered／Pressed 为 `#D1D1D1`／`#CACACA`／`#C3C3C3`，
暗色为 `#4F4F53`／`#555559`／`#5C5C5F`。current 边框复用中性次要文字色。
应用仍可按 Part 和 State 覆盖这些规则；此次调整不改变文本编辑选区、Primary 按钮
或 Basic 兜底主题。

selected Part 显式提供 Normal／Hovered／Pressed；展开箭头使用自己的事件控制器
决定悬停／按下状态，不拿整行悬停冒充箭头悬停。current Part 只画边框，即使自定义
样式继承到不透明背景也不能覆盖已选行。行最小高度 28 DIP、内边距 4 DIP、展开槽
20 DIP 是集中定义的布局常量，不新增样式尺寸系统。盒子绘制辅助放在 widgets 私有
通用 helper 中，TreeView 不再依赖以 Tab 命名的绘制函数；TabBar 绘制语义不变。

### 7.2 Snapshot

Tree／TreeItem 角色由 widgets 定义，复用开放角色机制，不把全部复杂控件角色塞进 gui。
详细层级数据归 widgets/info.go，以结构体值写入 gui.WidgetInfo.Attributes，
不要求 GUI 内核增加 TreeView 专用字段。契约见 [Widget Snapshot](DesignWidgetSnapshot.md)：

```go
// widgets 包，JSON 字段与首版保持。
const HierarchyInfoKey = "goui.hierarchy"

type HierarchyInfo struct {
    NodeID        string
    ParentID      string
    Level         int
    PositionInSet int
    SetSize       int
    Expandable    bool
    Expanded      bool
    Current       bool
}

// 行 Snapshot 中：
info.SetAttribute(HierarchyInfoKey, HierarchyInfo{ /* 当前行的已采集数据 */ })
```

- Level 和 PositionInSet 从 1 开始；SetSize 为模型当前已知的同父节点数量，不伪造尚未
  加载的总数。根节点的 ParentID 为空。
- 选择状态复用 Selected。Current 不等于 Focused，真实焦点仍在树或交互子控件上。
- 树的 ItemCount 为展开平铺中的逻辑行数，不是模型全部节点数。
- 子树只包含已实现的行，按逻辑顺序组织；不会为了 Snapshot 实例化离屏节点。
- 行保留真实内容子树，展开器具有真实按钮边界和“展开／折叠”名称。叶子的占位不伪造
  按钮、展开或激活动作。
- Snapshot 边界转换为窗口局部绝对坐标，绘制仍使用控件局部坐标。
- 自动化操作仍派发正常输入事件，不新增绕过 EventDispatcher 的语义执行捷径。
- 原顶层 hierarchy 字段和 gui.HierarchyInfo 不保留；直接 Go 调用方断言
  widgets.HierarchyInfo，JSON 消费者按 attributes 的具名数据解析。

## 8. 生命周期与资源归属

TreeView 管理模型信号 handle、内部 ListView、行绑定和本控件的上下文菜单；不拥有
业务模型的生命周期。多个视图共享模型时，只断开本视图的连接。

更换模型先使旧订阅及待操作失效，再连接新模型；卸载时断开本视图的模型订阅、关闭
菜单、撤销待显示／定位完成任务。每行卸载时使用最后绑定快照调用 delegate.Unbind，
普通滚动回收已解绑的行不会再次解绑。重新挂载先同步模型最新状态，再建立新行绑定。
销毁由现有 Widget 生命周期和 Window 兜底完成。
异步模型加载不是控件资源，控件卸载不默认取消共享模型任务。

对外信号、delegate、模型请求和 UI 构建器都是潜在重入边界。任何依赖旧状态继续执行的
地方都要检查对象有效性及版本；不能假设调用回调后窗口、模型或绑定还保持原样。

## 9. 实施切片与验证

### 9.1 实施顺序

1. **通用前置能力**：ListView.Reveal 目标定位接入既有 ScrollIntoView 请求机制，以及
   旧 delegate 解绑顺序修正；按 §6.3 的借用上下文同步修正 ui.ListView，
   不扩展 Coordinator 的构建职责。各自有独立回归测试。
2. **模型与状态**：TreeModel／TreeStore、展开平铺、ID 状态、按需请求和变化归一化。
3. **命令式控件**：行池、布局、输入／焦点、滚动、上下文菜单和生命周期保护。
4. **上层接入**：UI 链式声明、兜底／Modern 样式、Snapshot 及其一致性测试。
5. **窗口验收**：GUI／UI 共用独立例程，验证真实输入和显示，不以 DevServer 代替验收。

计划涉及 `widgets/tree_model.go`、`widgets/tree_view.go` 及必要的内部行实现，
`widgets/ui/tree_view.go`、`widgets/style`、`theme/modern`，以及上述 gui／ui 通用能力
所在文件。按职责组织实现，不把一次性辅助对象拆成大量微小文件。

### 9.2 包内逻辑测试

主要集中在 `widgets/tree_model_test.go`、`widgets/tree_view_test.go` 和
`widgets/ui/tree_view_test.go`；通用前置能力的回归放所属包现有对应测试文件。
不为每一个边界单独新建测试文件。

- 多根、深层、空树、空分支和叶子；非法 ID、父子关系及批量通知。
- 同一模型的多个视图拥有独立展开、选择、当前项和生命周期。
- 插入、删除、排序、换父级、折叠后的状态保留、可见性过滤和当前项回退。
- Shift 范围反向缩小、Primary 多选、折叠后代不进入选择。
- 行内交互控件不触发行选择，拖动胜出后不产生点击／双击。
- 首末行、首次布局前、可变行高和超高行 Reveal，确认估计定位最终收敛且不抢后续滚动。
- TreeView 转发内部列表请求时坐标正确；实际偏移只由 ScrollView 更新，垂直定位不改变
  水平偏移，不为目标尚未实现而创建全部前置行。
- 更换 delegate 使用旧 delegate 解绑；回调修改模型、卸载或销毁时没有失效访问。
- 异步结果到达已折叠节点不自动展开；删除和 ID 复用后的旧结果由模型正确丢弃。
- 初始受控展开触发加载但不回送 UI 状态；非受控重建保留状态，同模型新构建器刷新内容。
- 行复用不残留旧内容、选择、连接或双击状态；解绑参数使用旧绑定快照。
- 延迟构建仍通过有效 BuildContext；显式转移后不访问旧 Root，所属节点释放后不再构建。
  完成回调在行挂载后执行，跳过已失效的 owner／版本。
- 一万个节点时已实现 Widget 数量有界；折叠后代不被查询或创建，仅选中变化不重建平铺。
- Snapshot 的节点身份、层级、兄弟关系、选择／当前／焦点、实际子树和展开器边界准确。

测试使用独立预期和明确断言。性能结论需要同环境、同数据量的对照；不能以一次计时
或只有日志输出代替正确性和性能证据。

### 9.3 桌面例程

新增 `tests/window/widgets/tree_view`，默认 UI 模式，`-gui` 验证命令式组合。
入口使用中文说明环境、启动方式、初始状态、操作步骤、逐项预期、平台差异、副作用、
复位和退出方式，遵守 [测试约定](Testing.md)。

例程包含目录树、可控延迟加载、一万个节点分支、长文本／多行内容、行内按钮、上下文
菜单、数据更新和亮暗主题切换。重点操作展开折叠、多选范围、键盘导航、跨视口定位、
滚动复用、菜单目标删除和关闭窗口。

实施后执行 `go test ./...`，补充相关包 race 检查和 Windows amd64／macOS arm64
受影响目标编译；在本机真实桌面验证 1x／2x、亮暗样式和输入场景。分别记录包内测试、
交叉编译、原生运行和窗口操作验收结果，不将编译通过写成真机通过。

首版验收不以树拖放重排、行内重命名或 TableView 完成为前提。进度同步到
[Plan](Plan.md)／[Roadmap](Roadmap.md)，后续实现偏离本设计时先明确原因并更新文档。

### 9.4 本轮实现与验证记录（2026-10-03）

- 代码：TreeModel／TreeStore、TreeView／行池、widgets/ui 链式绑定、两套样式和
  HierarchyInfo 已接入。平台窗口、绘制和输入绑定没有改动。
- 通用回归：ListView.Refresh／Reveal、旧 delegate 解绑、向下跳转时先回收旧行、
  Snapshot 行顺序，以及 UI 借用上下文、移交／释放、延迟 AfterUpdate。
- 定位边界：ScrollView 使用整数 DIP 偏移，最后一行的不足 1 DIP 底部余量不能继续
  滚动。ListView 仅在该终止边界接受余量，并通知 TreeView 清理目标，回归验证完成后
  用户滚动和状态更新不会被旧 Reveal 拉回；超高行定位头部，纵向定位保留水平偏移。
- 包内回归另覆盖：多视图隔离、折叠／删除回退、稳定 ID 改名／排序／换父级、
  通知副本和重入、不可比较模型、行内按钮／拖动竞争、卸载解绑及宿主销毁时跳过
  旧通知、初始受控展开／加载、声明重建、可变行高和一万逻辑节点的有界行实例。
- 本机桌面：Linux X11／Xfwm4，Software 1x UI＋Modern 亮暗；选择和 Shift／Primary
  导航、行内按钮、500 ms 延迟加载、右键／Shift+F10 菜单、重建、跨视口 Reveal、
  用户继续滚动及 Snapshot 通过。展开后 10010 个逻辑行，观察到 18–19 个实现行。
  最终版本追加验证分支双击只切换展开、叶子双击仅一次 Activate、菜单删除后的当前项
  回退及窗口从 920×640 缩到 600×450 DIP 的布局／裁剪，均通过。
  OpenGL 2x GUI＋显式兜底样式验证展开、键盘多选、行内按钮、延迟加载、Refresh、
  万行定位与 Snapshot，通过；该视口观察到 10 个实现行。数量是各自实际视口结果，
  不是统一性能基准。例程窗口均已关闭，无残留后台进程。
- Windows amd64／macOS arm64 例程交叉编译通过；未进行远端或原生桌面验收。
  尚需补平台原生菜单／Command 与 Control-click、窗口 resize 和原生文字外观验收。
- Linux arm64 例程交叉编译未通过：`platform/linux/libs/libc/syscall_linux.go:29,61`
  引用 arm64 没有的 `syscall.SYS_PIPE`／`SYS_POLL`。该文件本轮未修改，
  DesignSplitView §验证记录已有同一失败，不在 TreeView 切片中扩大到平台 syscall 修复。
  命令为 `GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build ./tests/window/widgets/tree_view`
  （实际构建加 `-o` 写仓库外临时目录）。这不是 TreeView 的 arm64 编译通过证明。
- 全仓 `go test ./...`、`go test -race ./gui ./ui ./widgets/... ./theme/modern`
  及改动差异检查均通过。Windows／macOS 的 gui、ui、widgets、widgets/ui 和
  theme/modern 测试包另经 `go test -c` 编译通过，未运行其原生测试或桌面程序。

已有文字／像素测试会在源码目录输出 PNG；本轮未迁移这些既有测试，也未将产物加入
TreeView 的测试资源。新增包内测试不写源码目录，窗口截图和编译产物位于仓库外临时目录。

### 9.5 质量调整验证（2026-10-03）

- 回归用例覆盖：离屏最宽行下的 Selection／Current／Refresh 不改变水平偏移；
  状态变化只绑定受影响行；同结构数据更新保留行实例；结构更新解绑后复用行壳；
  Current 回调不吞 Selection 通知、同状态重入跳过旧值、同步期间再次删除节点
  不向后续监听者发送已删除 ID；右键由子控件优先消费；current 只绘制边框；
  展开器使用自身 hover／press 并在复用时复位；定位完成只通知一次，取消不通知。
- 同一个 200 行、200×100 DIP、无字体探针前后对照：选择操作的 Bind 次数从 4
  降至 1，水平偏移从错误的 80→0 改为 80→80；同结构内容更新新建行壳从 4
  降至 0，Unbind 从 4 降至 0。这是调用次数和状态对照，不是耗时性能基准。
- `go test ./...`、`go test -race ./gui ./ui ./widgets/... ./theme/modern` 通过；
  对相同相关包追加 `-count=1` 重跑及 `git diff --check` 通过。Windows amd64、
  macOS arm64 在 `CGO_ENABLED=0` 下交叉编译 TreeView 窗口例程通过，未做远端
  原生运行。Linux arm64 本轮未重测，前节记录的既有 libc 编译缺口仍未处理。
- Linux X11／Xfwm4，Software 1x UI＋Modern 亮暗：选择、展开、行内按钮、行菜单、
  一万行末项定位、用户继续滚动、Rebuild 和 Snapshot 通过。OpenGL 2x GUI＋
  兜底样式：选择、箭头按下／折叠复位、Reveal、滚动、Refresh 和 Snapshot 通过。
  当前两个实际视口分别观察到 18、11 个实现行，不作为统一基准。程序正常退出，
  无残留验证窗口；截图和可执行文件留在仓库外 `/tmp/goui-tree-opt-3owBr5`。
- 未更改 platform、公共绘制契约或 UI 协调机制。TabBar 只将同一盒子绘制函数
  迁移到 widgets 的私有通用 helper，没有改变标签行为或样式。已有测试的源码目录
  PNG 输出规则未在本轮迁移；不将测试跳过或交叉编译视为原生桌面验收。

### 9.6 Modern 中性高亮（2026-10-04）

- 仅修改 Modern 的 tree-item selected／current 样式；选择行为、Snapshot、文本
  编辑选区和兜底主题均不变。新增测试在旧主题色规则下失败，替换规则后通过。
- `go test ./theme/modern ./widgets/... -count=1`、`go test ./...` 和
  `git diff --check` 通过。回归覆盖亮暗模式、默认／黑／白／红／蓝主题色、六种
  确定的背景颜色、1 DIP 中性边框、4 DIP 圆角、文字对比度及应用覆盖规则。
- 本机 Linux X11／Xfwm4、Software 1x 的 UI 例程验证亮暗切换和选中三态。
  截图在选中行内部无文字、无边框处取屏幕坐标 (500,233)，不透明 sRGB RGB 值
  与 §7.1 六种预期完全一致，误差为零。首次 hover 截图在重绘前采样，仍为 Normal；
  等待事件绘制完成后重测正确，未放宽断言。截图位于 `/tmp/goui-tree-neutral-yRFxLn`，
  测试窗口已正常关闭。本轮未重复其他后端、缩放比例或 Windows／macOS 原生验收。

### 9.7 模型安装边界（2026-10-04）

- 删除命令式层 sameListObject／treeSameObject，显式 SetModel／SetDelegate 即使
  实例相同也重新安装。UI 仅以同类型的非空模型指针跳过重复调用，并复用 Delegate。
  首次 Mount／Update 只连接一次；无额外 appliedModel 状态或 Root／BuildContext 特例。
- 包内测试以连接次数、实际通知交付次数、Setup／Unbind 次数验证重连和订阅释放；
  覆盖同实例显式重装、同指针声明重建、不同指针相同数据替换、构建器更新、受控值
  应用顺序、nil 清理及值模型每次重装。接口字段包含 slice／map／func 均不做比较，
  不再触发此前 Type.Comparable 防护不完整导致的 panic。原有同模型不重置的
  GUI 断言按新契约改为重置，保留声明层同指针不重置的覆盖。
- `go test ./...`、`go test -race ./internal/identity ./gui ./ui ./widgets/...`
  和差异检查通过。Windows amd64／macOS arm64 在 CGO_ENABLED=0 下交叉编译
  TreeView 窗口例程通过，未做远端或原生桌面验收；Linux arm64 的既有编译缺口
  本轮未涉及也未重测。
- 本机 Linux X11／Xfwm4、Software 1x UI 例程验证：展开 src，Shift 多选后 Rebuild
  保留展开、选择和 Current，行文字更新为 v1；右键菜单删除当前项后状态回退，再
  Rebuild 更新至 v2 并保留回退后的状态。两次 Snapshot 均 ASSERT PASS。窗口正常
  关闭，截图及构建产物位于 `/tmp/goui-model-install-oFGpFu`，未使用 DevServer。
- 图片、菜单、文本编辑 setter 及样式／资源比较契约未改。菜单作为 ListView 调用方
  经现有包内测试和上述窗口操作回归；其开放接口直接比较问题仍需单独处理。

## 10. 文件管理例程与应用拖放边界

2026-10-04 新增 `example/filemanager`，先以现有公开 API 编写内存文件管理应用，
再判断树拖放是否真的缺少基础能力。本轮不增加 TreeView／ListView／GUI／UI API，
也不将应用数据移动策略放入控件。该例程不访问真实文件系统，不实现外部文件拖放。

### 10.1 实现与职责

- 应用模型保存稳定节点 ID、父 ID 与有序 Children。先归一化多选源，去掉重复节点
  和已选祖先的后代，再校验只读、文件目标、循环关系和稳定插入锚点。
- Motion 仅校验和显示反馈。Drop 再次校验，完整提交所有父子关系后才发一次 Items；
  移动保留节点 ID 和整棵子树，DragEnd 不再次删除源。取消及拒绝不修改模型。
- UI 的 TreeView 显示左侧目录树，既有 ListView 显示当前目录，组合 HeaderBar、
  SplitView、ScrollView 与 Modern 样式。模型指针稳定，重建不重复安装模型。
- 自定义内容 Widget 使用公开 Pick／Parent 识别应用自己的行，用局部坐标绘制
  before／after 插入线和 into 目录边框；不根据 TreeView 私有 children 顺序猜节点。
- DragSource／DropTarget 挂在稳定的 TreeView／ListView 上。行回收或重建会卸载
  内容 Widget，不能把长于该内容生命周期的源会话归属给它。预览通过 RenderWidget
  重绘当前内容，不包含私有缩进、展开器或整个行背景。
- 当前目录和顶层追加采用明确的接收按钮。图标是纯装饰；自定义列表内容的 Snapshot
  报告文字、选中状态和实际 Click 动作，树层级语义继续由 TreeView 负责。

### 10.2 实测结论与限制

内容区域内的同级排序、跨目录移动、整棵子树转移和多选移动，已经可以由用户实现。
使用自定义内容绘制插入线／目录边框，也不需要新增 DropActive 主题状态或内置树移动 API。

但这不能等同于完整的整行树拖放能力：

1. 委托只拥有内容；缩进、展开器和其余行空白仍是私有区域。当前例程不接受这些
   区域的拖放，内容边框也不能自然包住整行。稳定树控制器虽能接收事件，但缺少
   与应用内容一致的公开整行组合对象；不能用 Snapshot 代替精确 Pick。
2. 内容上／下缘表达同级 before／after，展开父节点的 after 指整个子树之后。
   悬停自动展开、插入到首个子节点等更细策略未在本例程实现。
3. 当前没有组合悬停展开和拖动边缘自动滚动。普通展开和滚动功能存在，并不代表
   DnD 期间的组合已经验收；不能靠例程中的显式按钮宣称这些问题都已解决。

### 10.3 对照与后续建议（未实现）

官方 [GtkTreeExpander 文档](https://docs.gtk.org/gtk4/class.TreeExpander.html)
（API 4.0，2026-10-04 页面为 GTK 4.23.4）明确其是 Widget，可绑定 TreeListRow、
设置 child，并提供展开操作与键盘绑定。应用能直接持有这一行组合部件，是 GTK4
灵活性的来源之一；不能只凭公共类型名称推断 GOUI 必须照搬其模型和事件实现。

结合上述应用观察，后续优先评估公开、可组合的 TreeExpander 或等价行外壳：
它负责缩进、展开器与内容布局，TreeView 仍负责模型平铺、选择、行回收和层级语义。
应先确定委托怎样获取／组合它，以及回收绑定与事件归属，再决定接口；本轮不直接
导出私有 treeRow，不让应用遍历私有 children，也不增加特定文件管理 Move API。
拖放期间自动滚动应单独审视 ScrollView 的通用能力，而非向 ListView 添加树特例。

### 10.4 验证

- `go test ./...`、`go test -race ./example/filemanager -count=1` 通过；模型测试
  有独立父子可达性断言、顺序预期、单次通知、无操作不通知、拒绝原子性、过期目标
  再校验和父子多选去重。命中几何及内容 Snapshot 另有包内断言。
- Ubuntu 22.04.5／X11／Xfwm4 4.16.1、Software 1x：排序、子树、多选、双向拖放、
  拒绝、Esc、亮暗、重置、重建与 500 项展开／滚动通过；OpenGL 1x 验证同级排序、
  预览和亮暗切换，未重复全部场景。未使用 DevServer，窗口正常关闭。
- Windows amd64／macOS arm64 在 `CGO_ENABLED=0` 下交叉编译例程通过；未执行
  原生桌面验收。2x、外部文件数据及拖动自动展开／滚动不在本轮通过范围。
- 截图、测试日志和二进制位于 `/tmp/goui-filemanager.riYEFQ`。没有更改既有测试
  的源码目录 PNG 输出规则或清理无关用户文件。

### 10.5 文件管理例程右键菜单

同日补齐应用菜单，不修改框架公共 API：树直接使用 widgets/ui.TreeView 的
ContextMenu／OnContextMenuError；右侧是例程专属 fileListView，复用 ui.ListView
的 Mount／Update／Unmount，在稳定 ListView 上附加 Bubble 阶段事件控制器和
PopoverMenu。不为单个应用向 ViewBase、Root 或 ListView 加菜单特例。控制器通过
公开 Pick 识别应用内容，右键及 macOS Control+左键走既有事件分发路径。

菜单查询时根据现状生成描述值；右键未选中项改为单选，已选中项保留多选。
打开／查看信息针对右键节点，树目录另有展开／折叠；移动命令使用归一化的选择集合：
同父级可按模型顺序整体上移／下移，跨父级仅可移到顶层。首尾边界、只读祖先和
已处于顶层决定禁用状态。打开文件仅写模拟状态，不访问真实文件或剪贴板。

命令捕获模型对象及节点 ID，不捕获会回收的行 Widget，也不使用之后变化的选择。
执行时检查模型仍相同、目标仍存在，并重新计算移动目标和校验权限；重置后的同 ID
新模型不能被旧菜单操作。菜单和 DnD 共用应用 commitMove；只有明确执行命令或
成功 Drop 才提交结构，菜单查询、取消和拒绝不改变数据。

右侧菜单锚在稳定列表上，避免选择重建导致内容卸载时弹窗提前关闭；关闭／Unmount
清空菜单模型，既有 Popover 锚点 unmount／窗口 destroy 机制负责释放原生资源。
不引入菜单对象的全局归属、公共 ContextMenu Controller 或新的声明式生命周期机制。

验证结果：

- 菜单包内测试验证选择、禁用状态、移动顺序／子树、原集合捕获、展开／信息、
  权限变化与模型替换后的再校验；这属于命令逻辑，不替代真实鼠标事件验收。
- Ubuntu 22.04.5／X11／Xfwm4 4.16.1、Software 1x 验证：两侧右键、单选／多选、
  上移／移到顶层、信息、目录折叠、边界与只读禁用、禁用点击、Esc／外部取消、
  树 Shift+F10、重置／重建、亮暗，以及菜单取消后再次拖放。提交结构断言通过，
  窗口正常退出。OpenGL 1x 另验证两侧菜单弹出、信息命令、亮暗／重建和取消，
  未重复 Software 的全部移动场景。
- `go test ./...`、`go test -race ./example/filemanager -count=1` 通过，Windows
  amd64／macOS arm64 的 CGO_ENABLED=0 交叉编译通过；未做远端／原生桌面或 2x
  验收。截图和产物位于 `/tmp/goui-filemanager-menu.0zkNoZ`，未使用 DevServer。

### 10.6 例程分割宽度状态

例程原先每次声明 `HSplit(...).StartSize(390)`，但没有接收 `OnResize`。
`StartSize` 是每次重建应用的受控值，不是只用一次的初始尺寸；选择树节点引起
UI 重建时，拖动后的宽度因此被固定值覆盖。这是应用状态绑定缺失，不是树测量
或 SplitView 约束错误，保持 [SplitView 受控语义](DesignSplitView.md) 不变。

修正仅在例程：`browser.sidebarWidth` 初始为 390 DIP，声明使用该状态，
`OnResize` 保存用户调整值。SplitView 自身已请求布局，无需在每次拖动时重建 UI。
数据重置不重置宽度；窗口约束造成的临时分配变化不发 Resize，也不改写偏好。

`main_test.go` 通过 EventDispatcher 和实际拖动控制器验证 390 → 210 DIP，
再验证选择、菜单查询、重建、亮暗、数据重置和窗口约束恢复。旧绑定不能保存
拖动值，回归失败；修正后通过，例程 race 和 `go test ./...` 通过。
Ubuntu 22.04.5／X11／Xfwm4 4.16.1、Software／OpenGL 1x 的真实窗口选择、
菜单、重建、亮暗和重置验证通过；Software 另验证窗口缩小／恢复。逐步截图采样
分隔线两侧及中线，物理位置不变。Windows amd64／macOS arm64 的 CGO_ENABLED=0
交叉编译通过，未做其桌面或 2x 验收。产物在 `/tmp/goui-filemanager-width.QSfX5Z`，
本轮创建的窗口已正常关闭，未使用 DevServer，未修改框架公共 API 或 Snapshot。

### 10.7 例程标题栏与正文留白

例程原先让根 VBox 的 8 DIP Padding 同时包住 HeaderBar 和正文，导致标题栏
不在窗口 Y=0、也不覆盖控制按钮侧的外部锚点，未参与既定的高度查询；Linux
圆形控制按钮因此按默认 40 DIP 行居中，与实际 HeaderBar 内容不在同一中心。

修正仅调整应用组合：根 VBox 无 Padding／Spacing，CrossStretch；第一项为
HeaderBar，第二项为独立正文 VBox，保留 Padding(8)／Spacing(6) 并设置
MainWeight(1)。正文内的 SplitView、状态栏和说明不变。标题栏仍自行管理内容
Padding 和按钮避让，不修改 GUI／platform 的查询、窗口按钮大小或坐标契约。

`main_test.go` 检查实际容器声明、根无留白和正文布局参数，旧结构下失败；公共
modifier 协调不在例程重复实现，由既有 UI 包测试及实际窗口验证覆盖。
Ubuntu 22.04.5／X11／Xfwm4 4.16.1、Software／OpenGL 1x 的标题栏居中、
亮暗、重建、窗口缩小、正文充满剩余高度和分割宽度保持通过。截图按同一扫描行
的空白标题背景识别按钮边界（不以顶部抗锯齿像素作背景），工具按钮与三个圆形
按钮的垂直像素中心一致；没有放宽位置误差来掩盖偏移。
全仓、例程 race 和 Windows amd64／macOS arm64 的 CGO_ENABLED=0 交叉编译
通过；未做其桌面／2x 验收，未使用 DevServer。本轮窗口正常关闭，产物在
`/tmp/goui-filemanager-header.gPv3CU`。
