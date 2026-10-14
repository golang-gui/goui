# GOUI 菜单(Menu)设计

> 状态：**已实现**（切片：`PopoverMenu`（右键/任意点）+ `MenuButton`）。定位：记
> "菜单的模型/渲染分离、为何复用 `ListData`、为何内部用 `ScrollView(ListView)`"。
> 参考 GTK4 的 `GMenuModel`/`GtkPopoverMenu`/`GtkMenuButton` 模型与渲染分离，
> 吸取其架构价值，用 GOUI 的 Go 惯例简化。

---

## 0. 一句话

菜单 = **模型**（`Menu`/`MenuItem`，纯数据，复用 `ListData`）+ **渲染宿主**
（`PopoverMenu` = `Popover` + `ScrollView(ListView)`，命令式控件）。模型与渲染
分离，同一模型可喂多个宿主；渲染器是内部细节，不进公共 API。

## 1. 对照 GTK4，取什么、简化什么

| GTK4 | GOUI | 取舍 |
|---|---|---|
| `GMenu`（GMenuModel 实现） | `Menu`（内嵌 `SliceListModel[*MenuItem]`） | `MenuModel` 嵌入 `ListData[*MenuItem]`，不另建列表通知协议，见 §2 |
| `GActionMap`/`SimpleAction` | `MenuItem.Action() func()` | 动作直接是 Go 闭包，不引入字符串动作注册表 |
| `GtkPopoverMenu`（渲染器控件） | `PopoverMenu`（命令式控件） | 渲染器内部化：内部 `menuContent` 未导出 |
| `GtkMenuButton` | `MenuButton` | 对齐 `Button` 惯例（单 child + 点击弹出） |
| `GtkPopover::closed` | `gui.Popover.ConnectClosed` | 信号放宿主（Popover），控件转发 |

**尚未实现**：子菜单及菜单自身的 ↑↓/Enter/Home/End 导航，见
[未来路线](Roadmap.md#future-submenu)。图标、菜单栏、模型级关闭信号和富控件菜单项
不属于当前 Menu 契约；富内容可组合普通 Popover。工作区尺寸约束／退避、UI 菜单
声明、通用 `OnContextMenu` 和 TabBar 上下文菜单已经实现，不能再列为缺失能力。

**ui 层**：声明式 `MenuButton`/`MenuItem`/`MenuSeparator`（`ui/menu.go`）已实现，
只协调到 `gui.MenuButton`，不另建菜单控件体系。"文件 ▾" 用
`ui.MenuButton("File").Menu(ui.MenuItem("Open", onOpen), …)`，子菜单的未实施设计见
`doc/DesignMenuSubmenu.md` §6。

## 2. MenuModel 复用 ListData

`gui/list_model.go` 已把列表拆成两层：`ListModel`（计数 + `ConnectItems` 变更
通知）与 `ListData[T]`（再加 `ItemAt(i) T`）。菜单的模型需求逐字一致——条目数 +
按索引取值 + 变更通知 + 内建具体实现（`SliceListModel`）。

公开 `MenuModel` 接口嵌入 `ListData[*MenuItem]`，明确菜单用途但不重复列表协议。`Menu` 内嵌
`*SliceListModel[*MenuItem]`，免费获得 `ItemsCount/ItemAt/ConnectItems/Append/
Insert/Remove/Set/Modify` + 并发安全 + 变更通知：

```go
type Menu struct{ *SliceListModel[*MenuItem] }
type MenuModel interface { ListData[*MenuItem] }
func NewMenu() *Menu
func (m *Menu) Append(label string, action func()) *MenuItem // 薄糖：构造 item + 塞模型 + 接 onChange
func (m *Menu) AppendSeparator() *MenuItem
```

- **`MenuItem` 是具体结构体，不是接口**（对照 `SliceListModel[int]` 的 `int`）：
  菜单项是叶子数据记录，不存在第二种条目表示（分隔线是 `separator` 布尔字段），
  接口买不到任何东西。
- **变更通知的实时价值**：`PopoverMenu` 订阅 `ConnectItems` → 菜单打开期间
  `SetEnabled(false)`/`Append` 能实时重渲染（ListView 的既有机制），而不是打开
  那一刻的冻结快照。
- `onChanged` 接线：同包访问 `SliceListModel.changed` 信号，`Append` 时
  `mi.onChanged = m.changed.Emit`，使 `SetEnabled/SetVisible` 也触发模型变更通知。

## 3. 渲染宿主：`PopoverMenu = Popover + ScrollView(ListView)`

菜单需要滚动与虚拟化（长菜单有正收益），因此渲染宿主直接复用
`ScrollView(ListView)`，而不是手写一个纵向行列表。

**为什么 ListView 可行**：菜单项数量适中，`ScrollView(ListView)` 的行高缓存与
虚拟化直接生效。但 ListView 不自带"自然高度"——`ScrollView.Measure` 对
Scrollable 返回的是约束 max，不是内容高度。所以**必须由 `PopoverMenu` 自己封顶
尺寸**，否则 popover 会把菜单量成 16384 高的巨块。

**内部结构**（`gui/menu.go`，全部未导出）：

```text
PopoverMenu（命令式控件）
└─ Popover（anchor 锚定，modal）
   └─ menuContent（内部 widget：内容尺寸，可显式限高）
      └─ ScrollView
         └─ ListView（model = ListData[*MenuItem]，delegate = menuItemDelegate）
```

**`menuContent.Measure`**（决定 popover 的自然尺寸，最终服从传入约束）：
- `naturalSize()`：O(n) 逐行测量（用 delegate 的 `Setup`/`Bind` 造一次性行
  widget，累计宽高），并缓存、模型变更时失效；
- 自然尺寸包含菜单内部留白；主体默认最小宽度 160 DIP，不设置默认最大宽度或最大高度。
- 最小宽度复用 `WidgetBase.SetMinSize`，不改写缓存的自然宽度；长文字继续撑宽，父级工作区硬约束优先。
- 显式限高或工作区限高且发生滚动时，限制高度，并额外预留滚动条和 6 DIP 间距。

**`maxHeight`**（默认 0，不限制；`SetMaxHeight` 可设置）：仅显式正值限制主体高度，
视口小于内容时 ScrollView 滚动条/滚轮/虚拟化生效。取消限制后恢复自然尺寸。
屏幕工作区约束和位置退避已按 [Popup 工作区设计](DesignPopupPlacement.md) 接入；
可用空间不足 160 DIP 时允许收缩，仅实际内容溢出时出现滚动条。最小宽度包含内部
留白，不包含宿主阴影；不影响通用 Popover、Tooltip 或 DropDown。

**菜单激活**：点击可用行先保存动作、`Hide()`，再调用 `Action()`；避免回调关闭窗口或重新开菜单后继续访问旧宿主。disabled 行
无操作、不关闭（点击无反馈，与 GTK4 一致）。
