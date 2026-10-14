# PopoverMenu 多平台统一外观设计

> 状态：**第 6 步代码及本机自动测试已完成，三平台真机验收待执行**。
> 共享阴影样式先于 Modern 实施；完整顺序见 [Modern 样式](DesignModernStyle.md)。
>
> 本切片先完善现有原生 Popup 承载的 PopoverMenu，不同时实现 Overlay。
> Menu 与 DropDown 继续使用原生 Popover，不迁移 Overlay。普通 Overlay 已实现，
> 与本宿主独立；不恢复旧 PopupAppearance 平台圆角方案。
>
> 相关背景：[菜单模型与渲染](DesignMenu.md)、[Popover](DesignPopup.md)、
> [样式系统](DesignStyle.md)、[透明表面](DesignTransparency.md)、
> [阴影绘制](DesignShadow.md)、[实用化路线](Roadmap.md)。

## 1. 目标与职责

提供三平台统一的菜单圆角、柔和阴影、细边框和圆角菜单项。现代外观由显式启用的
`theme/modern` 样式提供，不是 GUI 兜底默认、控件写死的策略或平台查询出的系统半径。
GUI 使用独立的完整 Basic 兜底，包含菜单外壳和菜单项规则，不因 Modern 包增加而自动换肤。

- `style`：描述背景、边框、圆角、阴影、文字及状态颜色。
- `gui.Popover`：管理外壳绘制、阴影占位、原生表面尺寸和定位。
- `gui.PopoverMenu`：管理内容布局、菜单状态、激活和关闭。
- `platform`：提供透明表面和已有绘制能力，不读取菜单样式。
- `ui`：继续复用现有菜单薄适配，不增加第二套外观模型。

菜单外壳仅绘制一次：由 Popover 使用 `menu` 样式绘制，移除 menuContent
重复绘制的背景和边框。MenuItem 仍是数据模型，不增加外观参数。

子菜单与菜单专属键盘导航尚未实现，见 [未来路线](Roadmap.md#future-submenu)。
工作区尺寸约束和位置退避已接入；圆角 Clip／ClearPath 仅为待评估方案，不新增
鼠标穿透、窗口专用 Painter、隐式样式继承或第二套主题系统。

## 2. 样式与公共接口

### 2.1 通用阴影值

在 `style` 包增加如下值类型与访问接口：

```go
type Shadow struct {
    Color        color.Color
    Offset       geometry.Point
    BlurRadius   float32
    SpreadRadius float32
}

func (s Style) Shadow() (Shadow, bool)
func (r Rule) Shadow(shadow Shadow) Rule
```

- 几何参数使用 DIP，含义与现有 graphics.BoxShadow 一致。
- 首版支持一个阴影值，不增加阴影数组或 Elevation 等间接参数。
- 阴影作为完整字段覆盖，不分别合并其中的颜色和几何参数。
- 未设置表示不提供阴影；显式 `Shadow(style.Shadow{})` 可以覆盖并关闭已有阴影。
- nil 或全透明颜色不绘制，也不预留阴影空间。
- 补齐字段标记、样式合并和 SameRules；颜色采用现有颜色值比较机制。

style 只依赖底层几何和颜色值，不依赖 platform。GUI 将其转换为 graphics.BoxShadow。
本轮仅由 Popover 消费阴影样式，不让所有 Widget 自动获得越界绘制能力。

### 2.2 Popover 样式入口

在 Popover 接口增加：

```go
StyleName() string
SetStyleName(name string)
```

- StyleName 返回显式名称；空名称使用 `popover`，与 Widget 的命名习惯一致。
- PopoverMenu 内部将宿主名称设置为 `menu`。
- 修改样式名使自然尺寸、布局和绘制失效。
- Transparent 仍是不可变创建配置，不因样式变化而自动切换。
- 普通 Popover 默认继续使用白色矩形、无阴影，现有默认用法保持不变。
- 不透明表面不绘制外围圆角和外部阴影，也不预留阴影空间。

### 2.3 样式名和 Modern 默认值

| 样式名 | 负责内容 | 首版默认值 |
|---|---|---|
| `menu` | 背景、边框、圆角、阴影 | 浅色背景，8 DIP 圆角，1 DIP 细边框 |
| `menu-item` | 行背景、边框、圆角、交互状态 | 4 DIP 圆角，正常背景透明；悬停、按下使用固定中性色，不随主题色变化 |
| `menu-item-text` | 菜单项 Label 的字体和文字颜色 | 使用主题正文颜色和字号 |
| `menu-item-text-disabled` | 禁用菜单项 Label 的字体和文字颜色 | 使用主题禁用文字颜色和字号 |
| `menu-separator` | 分隔线 | 沿用现有分隔线颜色 |

以下是 Modern 的默认阴影，不自动加入 GUI 兜底规则：

```go
style.Shadow{
    Color:        color.NRGBA{A: 46},
    Offset:       geometry.Point{Y: 4},
    BlurRadius:   16,
    SpreadRadius: 0,
}
```

两个文字样式名各自显式设置默认字体和字号，不依赖父行或彼此的样式继承。字体大小仍沿用
既有样式的单位约定，不能与几何参数的 DIP 混淆。

外观通过应用 StyleSheet 调整，不增加 MenuItem 外观字段或 UI 层局部样式参数。
自定义 StyleSheet 保留现有替换语义，需要默认值时显式追加默认规则。

## 3. Popover 几何与定位

### 3.1 三个区域

```text
原生 Popup 表面
└─ 主体 bodyRect：背景、圆角、边框
   └─ 内部安全矩形：ScrollView、菜单项和滚动条

主体外围的表面空间仅用于阴影。
```

GUI 内部计算或保存：

- shadowInsets：四边阴影占位。
- bodyRect：主体在原生表面中的矩形。
- contentClip：允许内容子树绘制的安全矩形。

这些都是私有几何，不加入平台窗口接口，也不将 Popover 伪装成 Widget。

### 3.2 阴影占位

先测量主体，再计算阴影外扩。沿用现有 BoxShadow 归一化语义：

- BlurRadius <= 0 为清晰阴影；正值最小归一到 0.5。
- 模糊标准差为归一化 BlurRadius 的一半，主要影响范围为 3σ。
- Offset 平移轮廓，SpreadRadius 扩张或收缩轮廓，圆角相应调整并钳制。
- 非有限参数、透明颜色或收缩为空的轮廓不产生阴影占位。

设模糊外扩为 E、扩张半径为 S、偏移为 OX/OY：

```text
Left   = max(0, E + S - OX)
Right  = max(0, E + S + OX)
Top    = max(0, E + S - OY)
Bottom = max(0, E + S + OY)
```

有效阴影的上述结果分别增加 1 DIP 安全余量，并向上取整为整数 DIP。
GUI 只计算保守占位，不复制遮罩算法，不新增公开 graphics Bounds API。

Direct2D 阴影输出同样按每边 3σ 扩展，参见
[Direct2D Shadow effect](https://learn.microsoft.com/en-us/windows/win32/direct2d/drop-shadow)。

### 3.3 尺寸和锚点

Popover.Position 表示**主体左上角相对 anchor 的位置**，不包含阴影留白。

```text
原生表面位置 = anchor 原点 + Position - shadowInsets.LeftTop
原生表面尺寸 = 主体尺寸 + 四边 shadowInsets
```

- PopoverMenu.ShowAt 的视觉定位不因阴影参数改变而跳动。
- 无阴影的现有 Popover 定位不变。
- SetMaxHeight 是可选的显式主体高度限制，默认不限制；包含内部留白，不包含外部阴影。
- 根 Widget 安排在 bodyRect，以自己的局部坐标绘制；输入利用既有 Widget
  矩形换算局部坐标，不再额外重复减去阴影偏移。
- 原生 SizeEvent 是实际表面尺寸的事实来源，不用请求尺寸覆盖它。

内容或样式触发 RequestLayout 时重新计算自然尺寸和占位，仅在所需尺寸或位置变化
时提交原生更新。保留布局期间再次请求布局的语义，避免同步 SizeEvent 造成递归或
丢失更新。实际表面扣除占位后空间不足时，内容矩形宽高钳制为非负值。

定位去重仅用于同一次显示中的布局更新。每次 Show 必须重新提交原生位置：
主窗口移动后，窗口内坐标可能完全相同，但对应的屏幕坐标已改变。
由平台 SetPosition 按主窗口当前位置重新换算，不向 GUI 暴露屏幕坐标。

## 4. 绘制与输入

### 4.1 绘制顺序

1. 清空整个表面；透明宿主清为透明。
2. 在完整表面范围绘制主体阴影。
3. 绘制主体圆角背景。
4. 设置内部结构性 ClipRect，绘制内容子树。
5. 恢复裁剪，绘制主体边框。

安全内缩量取圆角内接矩形所需距离与边框宽度的较大值，并保留抗锯齿余量。
菜单内容内边距不小于该距离，默认至少 6 DIP。滚动内容和自定义背景不得覆盖
主体圆角；安全区为空时抑制内容绘制，不能把空 ClipRect 当作取消裁剪。

裁剪在宿主遍历边界建立，不依赖父 Widget 在 Paint 中设置、随后会被遍历恢复的
临时 ClipRect。阴影不受内容安全区裁剪，边框在恢复裁剪后绘制。

复用现有帧清空和绘制流程，通过私有主体矩形、内容裁剪参数支持 Popover。
Window 继续传入原有几何，不能改变已实现 Integrated 窗口的布局和裁剪行为。

### 4.2 阴影区域输入

透明不等于鼠标穿透，本切片不提供穿透能力。

- 模态 Popover 收到主体外鼠标按下时，包括阴影留白和圆角外侧，发出 DismissRequest。
- 主体内部留白不激活菜单项，也不关闭菜单。
- PopoverMenu 通过信号执行 Hide；宿主不直接执行菜单项动作。
- 原生输入与程序派发统一进入 Popover.DispatchEvent，不在平台事件回调中单独
  实现关闭逻辑。
- 阴影区域不伪装成菜单项，不向下层控件合成点击。
- 若原生系统将事件送到 owner，继续沿用 owner 的外部点击关闭路径。

关闭和重新显示需要清理过期的悬停、按下状态；关闭回调可能销毁宿主，事件处理
在发出通知后不得继续访问已释放原生资源。

## 5. 菜单内容与文字资源

### 5.1 布局

继续复用 ScrollView(ListView)，不更换模型或虚拟化方案。

| 参数 | 默认值 |
|---|---|
| 主体内部留白 | 至少 6 DIP，随外壳安全距离增大 |
| 菜单项水平文字留白 | 8 DIP |
| 菜单项高度 | max(28 DIP, 文字高度 + 8 DIP) |
| 文字垂直位置 | 居中 |
| 分隔项高度 | 9 DIP |
| 主体宽度 | 最宽菜单项加内部留白，默认最小 160 DIP，工作区硬约束优先 |
| 主体高度 | 可见菜单项高度之和加内部留白，无默认高度上限 |
| 显式高度限制 | 可通过 SetMaxHeight 设置，非正数或非有限值取消限制 |
| 菜单项与垂直滚动条间距 | 6 DIP，仅在垂直滚动条出现时预留 |

这些是控件内部布局默认值，不新增 Padding、行高等样式字段。
主体高度封顶后，从中扣除内部留白得到滚动视口；阴影不挤占视口配额。
发生垂直滚动的菜单额外预留现有滚动条宽度和 6 DIP 间距，避免又挤出水平滚动条。
菜单行 Arrange 将这段间距留在行的实际矩形之外，背景、点击和语义边界保持一致。
短菜单不保留该间距；不修改通用 ScrollView 默认布局或增加公共间距 API。

当前菜单在 160 DIP 最小主体宽度基础上按内容撑开；显式限高或工作区限高
而发生滚动时预留滚动条和间距。
工作区约束及位置退避现已按 [Popup 工作区设计](DesignPopupPlacement.md) 接入；
超长菜单受目标工作区约束而滚动，显式限高仍为用户附加约束，不设置默认固定尺寸。

#### 短菜单最小宽度（2026-10-10）

中文等短文案菜单仅靠文字撑宽时，操作区域过窄。采用与语言无关的 160 DIP
主体最小宽度，包含菜单内部留白，不包含阴影。这是 GOUI 的布局默认值，
不是特定语言分支、固定宽度或 Modern 专有样式字段。

`menuContent` 构造时调用既有 `SetMinSize`，继续使用通用约束机制：

- 长内容大于 160 DIP 时自然扩张，不引入默认最大宽度。
- 可用工作区扣除阴影后不足 160 DIP 时，硬约束覆盖最小宽度。
- 保留原始自然尺寸缓存；偏好的最小宽度不算内容溢出，不产生额外横向滚动条。
- 仅 PopoverMenu 使用该默认值；通用 Popover、Tooltip、DropDown 不变。
- 模型、文字和样式变化沿用已有布局失效，行的绘制、命中与快照边界随最终安排更新。

参考的是“内容驱动尺寸加最小操作区域”，不是照搬其他框架尺寸：
[Flutter PopupMenuButton.constraints 官方文档](https://api.flutter.dev/flutter/material/PopupMenuButton/constraints.html)
的 Material 默认值为 112–280 逻辑单位；GOUI 不采用它的固定最大值。
[GTK4 4.20.3 默认主题源码](https://github.com/GNOME/gtk/blob/4.20.3/gtk/theme/Default/_common.scss#L1975)
为 modelbutton 提供最小尺寸和水平留白，并非承诺菜单统一宽度 160。

回归覆盖无字体时的最小宽度、短中文/英文、长内容、父级约束、阴影占位、
窄工作区无多余滚动条、文字变化后的伸缩，以及正常菜单点击与语义边界。
测试字体替身每 UTF-8 字节占 10 DIP，仅用于确定性布局断言，不代表真实中文字体指标。
桌面验证结果记录在 Plan / Roadmap，不将替身测试当成真实字体或跨平台桌面验收。

本轮验证（2026-10-10）：

- `go test ./...`、`go test ./... -count=1` 通过；隔离桌面最后一次非缓存全仓
  JSON 统计为 2202 测试/子测试通过、14 个既有跳过。跳过为平台人工输入、
  原生 GLX/透明表面、窗口状态等显式启用项及 Linux 原生 Integrated 不适用项。
- `go test -race ./gui ./ui ./widgets -run 'Menu|Popover|DropDown' -count=1` 通过。
- `go vet ./gui ./ui ./widgets` 未通过，仍报告文本编辑及既有测试的未具名
  跨包结构体字段；未修改这些代码。`go vet -composites=false ./gui ./ui ./widgets` 通过。
- Windows amd64、macOS arm64 的 demo 可执行程序和 GUI 测试目标交叉编译通过；
  不代表两个平台的原生桌面验收。
- 本机 Linux X11：隔离 Xvnc 2560×1800、Xfwm4 合成器，Software / OpenGL
  （Mesa 23.2.1 llvmpipe）各请求配置 1x/2x。中文短菜单亮暗外观、扩宽区域的
  悬停/点击、关闭再打开通过；原主体约 80 DIP，修正后 160 DIP，高度不变。
  2x 长菜单受工作区限高，滚动至第 45 项，无多余水平滚动条。
  Software 1x 英文切换、OpenGL 1x 德语切换通过，德语长文案主体约 204 DIP，
  证明尺寸仍由内容决定。2x 是应用覆盖值，不算真实 HiDPI/混合 DPI 验收。
- 一次额外隔离全仓运行中，未修改的 Pango
  `TestTextAttributesReplaceNestedRanges/strikethrough` 报告位图尺寸
  `want=0x0/0 got=296x102/1184`，父测试也标失败；独立复测该组五次和再次
  非缓存全仓均通过。未修改排版实现、未跳过或放宽断言，保留偶发失败记录。
- 整仓包含既有平台测试，会短暂创建原生窗口；新增回归均为不依赖桌面的布局逻辑。
  手动操作仅在隔离桌面执行，demo、窗口管理器及 Xvnc 均在验收后关闭。
  Windows/macOS 原生桌面、Direct2D、真实 HiDPI 未执行。

验证日志、构建产物及前后截图留在仓库外 `/tmp/goui-menu-minwidth.UqVUsV/`。

### 5.2 菜单项组合 Label

menuItemRow 保留内部 Label；父控件不为子控件提供样式：

- 行解析 `menu-item`，负责背景、边框、圆角和交互；状态优先级为 Disabled > Pressed > Hovered > Normal。
- Label 独立解析 `menu-item-text`；绑定禁用项时改用 `menu-item-text-disabled`。
  两者都是普通 StyleName，不把行的样式或 Hovered、Pressed 状态传递给 Label。
- Label 自己管理 TextLayout，测量通过 Label.Measure，绘制由 Widget 树遍历完成。
- 行内保留水平内边距并垂直居中文字；滚动条间距仍处于行边界之外。
- 悬停、按下只重绘行背景，不改变文字排版资源和自然尺寸。
- 文字样式变化经既有 StyleChanged 钩子刷新 Label 缓存，同时更新行、列表和菜单自然尺寸。
- 禁用项无按下反馈、不执行动作；分隔项不参与激活。

不扩展公共 Label 接口，不引入隐式样式继承。行快照保留 RoleMenuItem、
RoleMenuSeparator、文字、启用状态及适用的点击动作；Label 作为真实子控件参与快照。

### 5.3 缓存与生命周期

- 行解绑或卸载时释放文字资源。
- 自然尺寸测量使用的临时文字资源必须显式释放。
- 菜单内容的模型信号连接成对清理，不能由模型长期持有已弃用的内容对象。
- 主题变化同时失效菜单自然尺寸和 ListView 行尺寸缓存。
- 同一菜单多帧绘制不重新创建文字布局，不新增第二套图片缓存。
- 替换菜单模型、重复显示、anchor 卸载和 owner 销毁均需覆盖资源释放与重新绑定。

## 6. 透明不可用时的降级

用户已选择**降级显示**，该策略属于 PopoverMenu，而不是平台事实层：

1. 优先尝试透明 Popup。
2. 创建阶段返回 ErrUnsupported 或 ErrUnavailable 时，清理失败资源，再尝试
   不透明 Popup。
3. 降级外壳为矩形，关闭外部阴影并清除阴影占位；保留菜单项圆角、颜色、文字和交互。
4. 其他错误直接返回，不把所有失败都解释为透明能力不足。
5. 不透明重试也失败时，返回包含两次失败原因的错误。

普通 Popover 显式请求 Transparent=true 时，仍然返回创建错误，不暗中改变配置。
样式本身不被降级逻辑改写；宿主根据实际表面能力选择有效外壳表现。

已创建的降级宿主在 Hide/Show 间复用；原生资源释放后，下次创建重新尝试透明。
重试不能泄漏 Painter、Popup、信号连接，或产生重复 Closed 通知。

## 7. 三后端与原生阴影

### 7.1 复用绘制能力

复用现有 BoxShadow 算法，不升级模糊实现，不增加 gui.Painter 方法。外壳由持有
底层 Painter 的 Popover 绘制。不得恢复 SurfaceClipper 或窗口专用遮罩。

### 7.2 阴影颜色适配

现有阴影路径需要校正颜色约定；黑色阴影会掩盖错误，测试必须包含半透明彩色阴影。

- GUI 通过 graphics.ColorOf 转换，graphics.Color 按预乘颜色解释。
- OpenGL/NanoVG 和 Direct2D 需要 straight alpha 的入口进行一次反预乘，alpha
  为零时不除零。
- Software 对预乘 RGB 与 alpha 同时乘覆盖率，不能再次乘 alpha。
- 清晰阴影以及软阴影失败后的清晰降级路径遵守相同约定。

Direct2D 原生阴影颜色明确要求 straight alpha，参见
[Shadow effect 的 Color 参数](https://learn.microsoft.com/en-us/windows/win32/direct2d/drop-shadow)。
本轮只校正阴影路径，不扩大为整个颜色系统的重构。

### 7.3 原生表面装饰

macOS Popup 明确配置为无原生阴影，避免与 GUI 阴影叠加。使用 AppKit 的
[hasShadow](https://developer.apple.com/documentation/appkit/nswindow/hasshadow) 设置；
绑定遵循现有规范，配置留在 Cocoa Popup 创建层，不放入 frameworks 绑定层。
普通顶层 Window 的原生阴影不受影响。

Windows、Linux 不主动申请额外原生装饰。不引入平台圆角半径查询或发行版分支。
统一外观指 GOUI 自身绘制结果一致，不承诺覆盖用户强制配置的合成器特效。

## 8. 后续实施顺序

本设计的前两项由 Modern 路线第 1 步实施，其余在第 6 步实施。
中间依次完成样式变化钩子、窗口装饰隔离、UI Settings 自动更新和 Modern 规则。
菜单行 Label 组合和独立文字样式接入位于第 6 步，不提前改变菜单内容实现。

- [x] 阴影样式值、覆盖与比较测试。
- [x] 三后端阴影颜色适配；Software／GLX 像素测试及 Windows／macOS 交叉编译。
- [ ] 阴影颜色的 Windows 原生像素测试和 Windows／macOS 样例运行验收。
- [x] Popover 样式入口、外壳几何、定位和绘制。
- [x] PopoverMenu 内容布局、文字资源与语义快照。
- [x] 透明失败降级、阴影区域输入及生命周期回归。
- [x] macOS Popup 原生阴影配置与绑定；已交叉编译，原生运行待验收。
- [x] Modern 菜单示例、几何／输入／资源测试、多倍缩放 Software 像素测试与 Pango 状态文字测试。
- [ ] 三平台真机验收。
- [ ] 验收后同步相关接口说明和路线进度，Overlay 保持为后续切片。

核心修改集中在 style、Popover 宿主及菜单内部实现，外加必要的阴影适配和
macOS Popup 配置。不借本切片重构不相关的绘制、布局或文字基础设施。

## 9. 验证与验收

### 自动化验证

- 样式覆盖、显式关闭阴影、等值比较、普通 Popover 默认兼容性。
- 正负 Offset、正负 Spread、无模糊、空轮廓、非有限参数及阴影占位。
- 修改阴影前后主体锚点不变，最大高度不包含阴影。
- 同步/异步 SizeEvent、重复布局和实际表面取整，不造成定位跳动或请求循环。
- Software 在 1x、1.25x、1.5x、2x 下的透明角、阴影、彩色阴影及边框像素测试。
- 首尾菜单项悬停、按下、长菜单滚动及滚动条出现时，圆角不被覆盖。
- 明暗配色、字体和字号变化、禁用状态、重复显示时缓存正确更新。
- 阴影区域点击、主体留白点击、菜单项激活、Esc 和外部点击关闭。
- 透明创建失败后的资源释放、矩形降级、重试失败和普通 Popover 错误语义。
- anchor 卸载、owner 销毁、模型替换和关闭回调销毁宿主。
- 现有 Window Integrated 裁剪、定位及生命周期回归。

各切片代码实施后运行 `go test ./...`，有平台变更时交叉编译 Windows/macOS 相关测试目标。

### 真机验收

三平台分别验证默认浅色、自定义深色、彩色阴影、无阴影、长菜单和不同 DPI。
允许后端渐隐曲线有细微差异；主体定位与外部影响范围误差控制在 1 个物理像素内。
外壳圆角、阴影和边框不得出现截断、覆盖或重复原生阴影。

自动化测试、交叉编译和三平台真机验收分别记录，不以文档写入或测试目标编译
代替功能完成。共享阴影值、颜色适配、宿主和菜单外观均已实现，跨平台目测仍待验收；
分阶段验证记录见 [Modern 样式](DesignModernStyle.md)。
