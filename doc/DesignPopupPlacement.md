# Popup 工作区约束与桌面边缘退避

> 状态：**代码及自动测试已实现；三平台真机外观验收待完成**。
>
> 本设计针对当前由原生 Popup 承载的 GUI Popover／PopoverMenu，补齐工作区查询、
> 内容尺寸约束及位置退避；普通 Overlay 已实现，但 Menu／DropDown 不迁移承载。
>
> 相关：[Popup](DesignPopup.md)、[PopoverMenu 外观](DesignPopoverMenuAppearance.md)、
> [菜单](DesignMenu.md)、[HiDPI](HiDPI.md)、[实用化路线](Roadmap.md)。

## 1. 目标与边界

采用“平台报告工作区，GUI 决定退避”的方案，不在平台 `Popup.SetPosition`
中隐式调整位置。

- `platform`：查询原生显示器及工作区事实，转换坐标，执行尺寸和位置请求。
- `gui.Popover`：测量内容，计算阴影占位，选择翻转、平移或收缩，提交原生请求。
- `gui.PopoverMenu`：在最终内容约束下决定是否滚动，并预留滚动条和间隔。
- `gui.MenuButton`：明确表达矩形锚点及首选方向，不自行查询桌面或处理屏幕边缘。
- `ui`：继续复用 GUI 行为，不直接查询平台或建立另一套定位逻辑。

公共坐标仍以窗口客户区为原点、使用 DIP，不引入全局屏幕坐标、显示器管理系统、
绝对桌面坐标设置接口或额外 Painter 能力。窗口相对定位已由 [窗口管理](DesignWindowManagement.md) 接入。首版 Linux 范围为现有 X11 后端。

不同时实现 Overlay、子菜单、箭头气泡、避开任意障碍物、鼠标穿透、额外键盘导航、
通用安全区域系统或显示环境变化通知。

## 2. 当前实现与待补能力

当前菜单按内容撑开，主体默认最小 160 DIP（包含内部留白，排除阴影），不设默认高度上限；工作区硬约束优先。
`SetMaxHeight` 仅是用户可选的显式限制；发生垂直滚动时预留滚动条及 6 DIP 间隔。

当前三个平台的 Popup 只负责窗口创建、坐标转换及位置／尺寸设置，并不自动退避。
实施前 GUI 不能查询可用工作区，Popover 自然测量使用 `1 << 14` 内部暂定边界。
本切片已移除此边界，改用工作区约束；仅在平台明确不支持查询时保留无退避降级。

实现以真实工作区约束替代任意尺寸上限，不重新引入固定的菜单宽高。

## 3. 平台公共接口

在 `platform/common.DesktopWindow` 增加：

```go
// WorkAreaAt returns the available work area of the display containing point.
// Coordinates are relative to this window's client origin, in DIP.
// If point is outside all displays, the nearest display is selected.
WorkAreaAt(point geometry.Point) (geometry.Rectangle, error)
```

### 3.1 归属

放在 `DesktopWindow` 上，而不是 Popup 上：GUI 必须在创建 Popup、分配绘制资源之前
取得尺寸约束。移动端 Window 不因此获得桌面专有能力。

`platform.Popup.SetPosition`、`SetSize` 保持现有语义，不自动翻转、平移或裁剪。
gui.Window.WorkAreaAt 已作为薄委托公开；Popover 使用所属窗口的桌面能力，UI 可通过 App.FindWindow 取得命令式窗口，不自行实现定位算法。

macOS 与 Windows 的 Popup 在原生尺寸转换处向上对齐到能容纳请求大小的物理像素，
创建和 SetSize 使用同一转换；本轮只修正 macOS，不顺带改变 X11 实现。
GUI 仍以实际 SizeEvent 为准，不以请求值覆盖
系统报告。macOS 使用 Popup 自身的 backing scale，将整数像素尺寸转回 AppKit
point，并正确处理 GOUI_PLAT_SCALE；普通窗口的原生尺寸行为保持不变。
避免文字排版的小数高度被原生取整压缩后产生没有有效滚动范围的滚动条。
不通过修改字体、增加任意留白或 ScrollView 容差掩盖尺寸不足。
参见 Apple 的[backing 坐标与像素对齐说明](https://developer.apple.com/library/archive/documentation/GraphicsAnimation/Conceptual/HighResolutionOSX/APIs/APIs.html)。

### 3.2 坐标和选屏

- 输入点及返回矩形都使用该窗口客户区 DIP，与 Popup 定位的坐标约定一致。
- 返回坐标允许为负，也允许超出主窗口范围；主窗口客户区不是弹出窗口的布局边界。
- 选择包含参考点的显示器；参考点落在屏幕之间的空隙或所有屏幕之外时，选择最近显示器。
- 返回单个目标显示器内的工作区，不返回所有显示器的联合包围盒。
- 参考点不能固定为主窗口中心。主窗口跨屏时，弹出锚点可能在另一块屏幕上。
- 工作区是操作系统／窗口管理器报告的可用区域，不是扣除其他普通应用窗口后的可见区域。
- 转换必须使用与现有窗口内 DIP、Popup 定位一致的缩放约定，包括项目自定义逻辑缩放。

### 3.3 查询与错误

- 方法是即时查询。主窗口移动后，相同输入点可能得到不同返回矩形。
- 方法要求在窗口所属事件线程调用；调用本身不移动或调整任何窗口。
- 不支持相应能力时返回 `ErrUnsupported`。
- 窗口已销毁、显示环境暂不可用或必要数据暂时无法取得时返回 `ErrUnavailable`。
- 其他原生错误保留；非有限输入返回参数错误。
- 空区域、无效矩形不能作为成功结果返回。
- 查询失败不能返回虚构工作区，也不能用任意固定尺寸冒充平台事实。

首版不增加专用通知；查询结果只作为一轮求解中的快照，不跨多次 Show 长期缓存。

## 4. 三平台实现

### 4.1 Windows

1. 将窗口客户区参考点转换为原生屏幕坐标。
2. 使用 `MonitorFromPoint(..., MONITOR_DEFAULTTONEAREST)` 选择显示器。
3. 使用 `GetMonitorInfo` 取得 `rcWork`。
4. 将矩形转换回所属窗口客户区 DIP。

不使用 `MonitorFromWindow` 替代按锚点选屏，也不使用主屏工作区代替目标显示器。

原生接口分别提供按点选择显示器及读取显示器工作区的能力：
[MonitorFromPoint](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-monitorfrompoint)、
[MONITORINFO](https://learn.microsoft.com/en-us/windows/win32/api/winuser/ns-winuser-monitorinfo)。

### 4.2 macOS

1. 根据现有窗口坐标转换关系，得到参考点的屏幕位置。
2. 枚举 `NSScreen`，选择包含参考点的屏幕；否则选择最近屏幕。
3. 查询目标屏幕的 `visibleFrame`。
4. 处理原点方向和项目逻辑缩放，返回窗口客户区 DIP。

`visibleFrame` 排除当前 Dock、菜单栏占用区域，并随界面设置变化，不应长期缓存。
参见 [Apple visibleFrame 文档](https://developer.apple.com/documentation/appkit/nsscreen/visibleframe)。

### 4.3 Linux/X11

必须区分显示器几何与桌面工作区：

- RandR 提供显示器／输出相关的几何信息。
- EWMH `_NET_WORKAREA` 是按虚拟桌面报告的，不是按显示器报告的精确工作区。

首版处理方式：

1. 查询当前有效显示器几何，按参考点选择目标显示器。
2. 查询当前虚拟桌面的工作区，正确处理该属性及桌面 viewport 的坐标约定。
3. 返回工作区与目标显示器的交集，并转换为 owner 客户区 DIP。
4. 缺少必要能力或数据时返回明确错误，不把根窗口大小直接当成工作区。

实现使用 RandR 1.5 `XRRGetMonitors` 查询活动显示器；缺少该版本或必要 EWMH
能力时返回 `ErrUnsupported`，声明支持但数据缺失／无效时返回错误。
EWMH §3.9 已规定 `_NET_WORKAREA` 相对各桌面的 viewport，故与 RandR 根坐标
和 `XTranslateCoordinates` 合并时**不再减去 `_NET_DESKTOP_VIEWPORT`**。

这是依据窗口管理器公布信息得到的**保守区域**，不承诺等同于精确的每显示器工作区。
某些多屏面板配置可能让其他屏幕也损失一部分可用空间。实现和验收必须记录这一限制，
不能将这种情况宣传为精确的单屏面板避让。

首版不引入发行版分支、不依赖私有 WM 协议，也不扫描所有面板并重建窗口管理器的
保留区域策略。后续如确有需求，再独立设计更精确的区域报告能力。

原生依据：[RandR 协议](https://xorg.freedesktop.org/archive/current/doc/randrproto/randrproto.txt)、
[EWMH 工作区与保留区域规范](https://specifications.freedesktop.org/wm/latest-single/)。

### 4.4 原生绑定规范

SDK／frameworks／libs 仅包含原生类型、常量和调用绑定；工作区查询组合、选屏及
坐标适配放在对应平台窗口后端。遵循现有 Windows、AppKit、X11 绑定风格，
不将应用层适配逻辑放进绑定目录。

## 5. GUI 定位请求

Popover 公开定位策略；显示操作只使用当前配置，不隐式改变定位方式。
MenuButton、PopoverMenu 和 widgets.DropDown 均通过公开接口提出请求，
不访问 `popover` 私有定位字段，也不根据 `Position.Y == anchor.Height` 猜测语义。

```go
type PopoverPlacement uint8

const (
    PopoverPlacementPoint PopoverPlacement = iota
    PopoverPlacementBottom
)

// Popover 接口中的定位部分
Placement() PopoverPlacement
SetPlacement(PopoverPlacement)
Position() geometry.Point
SetPosition(geometry.Point)
Show() error
```

| 模式 | 输入含义 | 首版使用者 |
|---|---|---|
| Point（默认） | Position 为主体首选左上角，使用 anchor 局部 DIP | 普通 Popover、PopoverMenu.ShowAt |
| Bottom | 完整 anchor 矩形，下方左对齐优先，允许翻转和平移 | MenuButton、widgets.DropDown |

### 5.1 配置契约

- `Placement()` 和 `Position()` 返回用户请求，不返回实际退避结果。
- `SetPlacement` 保留 Position；未知枚举值归一为默认 Point。
- `SetPosition` 保留 Placement；Bottom 模式保存但忽略该坐标，切回 Point 后恢复使用。
- 两个 setter 重复设置相同有效值时不请求更新；对象销毁后不再修改配置。
- 隐藏时仅保存配置，不创建原生窗口或查询工作区。
- 可见期间有效配置变化通过现有 RequestLayout 请求下一次布局，连续配置可合并；
  Bottom 模式仅修改 Position 不触发布局，不重新 Show 或重建原生 Popup。
- `Show` 每次重新解析锚点和工作区，使用最新配置；失败、Hide 及原生资源释放均保留配置。
- Bottom 是首选方向，实际位置可以在锚点上方；本轮不增加实际方向查询。

定位配置与显示动作的分离参考 GTK4 文档：
[set_position](https://docs.gtk.org/gtk4/method.Popover.set_position.html)（查阅版本 4.23.3）
将位置解释为可翻转的偏好；[popup](https://docs.gtk.org/gtk4/method.Popover.popup.html)
（查阅版本 4.23.4）负责显示。采用其职责分离，不引入完整方向、锚点矩形、对齐和偏移接口。

### 5.2 调用方与内部状态

MenuButton 明确请求 Bottom，PopoverMenu.ShowAt 明确请求 Point；透明创建失败后
替换为不透明 Popover 时重新应用同一策略和坐标。DropDown 打开时确保其 Popover
为 Bottom，然后统一调用 Show；重复配置已有策略是无副作用的。

删除 ShowBelow，不保留别名；该方法尚未发布。UI 继续复用控件行为，不增加
定位状态机或修改 Root / BuildContext。

普通 Popover 点定位继续有效，可进行工作区内平移及尺寸约束；没有明确方向语义时，
不猜测向上或向下翻转。

内部必须分别保存：

- 用户请求的点位置或矩形锚定配置。
- 本轮计算出的主体目标矩形。
- 原生 SizeEvent 报告的实际表面尺寸。

内部 `placement` 保存公开枚举，`placementSnapshot` 保存本轮求解的快照；
快照包含策略、窗口局部锚点和工作区，不维护另一份私有矩形模式开关。
测量回调若修改有效定位配置，本轮不提交旧结果；已有布局请求保留，下一轮应用新配置。
原生 SizeEvent 仍使用最后完成的快照与实际尺寸定位，不重新查询工作区。

退避结果不得写回用户配置，不能让后一次 Show 以之前退避过的位置作为新的原始锚点。

### 5.3 回归验证

包内覆盖默认策略、配置独立、完整矩形、模式切换、未知枚举、可见时延迟更新及
同值无副作用；Hide / 显示失败后保留配置，模式变化不重建、不关闭、不改变模态或焦点。
测量回调修改配置时旧结果不能提交或吞掉新请求。菜单透明降级保留策略与坐标，
ShowAt 不沿用先前的 Bottom 模式。已有四候选、阴影、负工作区及原生尺寸取整断言保留。
窗口验证继续使用菜单和 DropDown 例程检查下缘翻转、右缘退避和移动主窗口后重开。

### 5.4 2026-10-06 定位接口整理验证记录

- 定向 `go test ./gui ./widgets/... -run 'Popover|MenuPlacement|DropDown' -count=1`
  与 GUI / widgets / Modern 包测试通过；默认值、配置独立、可见时合并、模态/焦点保持、
  重入取消旧求解、透明降级和菜单模式切换断言通过。
- `DISPLAY=:99 GOUI_PLAT_PAINTER=software go test -json ./...` 全仓通过：
  1997 个测试 / 子测试通过、14 个既有测试跳过。全仓旧测试仍会短暂创建窗口，
  部分既有图片测试会输出源码目录 PNG；本轮新增测试不创建原生窗口、不写图片。
- `go test -race ./gui ./widgets/... ./theme/modern` 通过；
  `go vet -composites=false ./gui ./widgets/... ./theme/modern` 通过。
  未将既有非具名字面量警告视为默认 vet 通过。
- Windows amd64 / macOS arm64：`CGO_ENABLED=0 go test -exec=true ./gui ./widgets/...
  ./ui ./theme/modern ./tests/window/widgets/drop_down` 均编译通过，未执行目标程序。
- 本机 Linux amd64 / X11 / Xfwm4，隔离 Xvnc :99、Software 1x：DropDown 实际鼠标
  打开后下方对齐；移动主窗口后重开使用新位置；屏幕下缘向上翻转，右下缘必要平移
  后整个表面仍在工作区。F6 状态/快照检查通过，关闭输出 DESTROY PASS、退出码 0。
  例程不使用 DevServer；截图和全仓日志保留在 `/tmp/goui-popover-placement.wueazl/`。
- 本轮未执行 Windows/macOS 远端桌面、OpenGL/Direct2D、2x、真实 Retina 或混合 DPI
  交互验收；菜单降级及 Point/Bottom 切换仅由包内替身与独立坐标断言覆盖。

## 6. 内容尺寸与退避算法

```text
锚点确定目标工作区
        ↓
测量自然尺寸及阴影占位
        ↓
尝试首选位置与翻转位置
        ↓
保持尺寸，在工作区内平移
        ↓
确实超过工作区时，约束内容并重新布局
```

### 6.1 自然尺寸

菜单宽度由最宽项目和内部留白决定，高度由可见项目高度之和和内部留白决定。
主体默认最小宽度 160 DIP，不设固定最大高度；自然内容可撑得更宽，工作区不足时可收缩。用户显式 SetMaxHeight 仍是额外约束。

主体与阴影分别计算。候选位置是否能完整容纳，按照包含阴影的整个原生表面判断；
锚点对齐仍以主体边缘为基准，不因阴影改变视觉对齐。

### 6.2 矩形锚定的候选位置

对于“下方、左侧对齐”的 MenuButton，依次尝试：

1. 下方、左侧对齐。
2. 下方、右侧对齐。
3. 上方、左侧对齐。
4. 上方、右侧对齐。

采用第一个能完整容纳弹出表面的候选。候选顺序固定，不能受上次退避结果影响。

若所有候选都越界，但表面尺寸能放进工作区，则保持尺寸并平移进去。
此时允许覆盖部分锚点。不能因为按钮下方只有 200 DIP，就将一个本来能在屏幕内
完整显示的菜单限高为 200 DIP。

点定位没有明确方向，直接对首选位置做最小必要平移。

### 6.3 收缩和重新布局

只有完整表面超过工作区尺寸时，才约束主体尺寸：

```text
主体最大宽度 = 工作区宽度 − 左右阴影占位
主体最大高度 = 工作区高度 − 上下阴影占位
```

用所得约束重新测量内容，然后以最终尺寸重新计算候选位置与必要平移。

- 菜单按最终可用高度决定是否出现垂直滚动条。
- 出现滚动条时计入其宽度及 6 DIP 间隔，再检查宽度。
- 宽度仍不足时沿用现有水平滚动能力，本轮不增加自动换行或省略号策略。
- 普通 Popover 只约束用户内容，不擅自把 Widget 包进 ScrollView。
- 不把屏幕约束写入用户的 `SetMaxHeight`，也不覆盖控件固有的自然尺寸缓存。

求解应保证确定性，并处理尺寸取整、空可用区域及阴影导致主体无可用空间的情况；
不能提交非法尺寸、无限重测或依靠固定菜单大小掩盖求解失败。

实现每次先测自然尺寸，只有超过工作区才用主体上限重测一次。收缩时保留自然尺寸
算出的保守阴影占位，避免负 SpreadRadius 在小尺寸下使遮罩消失而引发尺寸振荡。
扣除阴影后任一轴不足 1 DIP 时返回错误。实际原生尺寸因取整仍大于整个工作区时，
将该轴贴工作区起点，保留原生尺寸事实，不循环请求缩小。

菜单在最终 Arrange 时将 6 DIP 间隔计入行的滚动占位，再从行绘制边界移除；
自然尺寸统计不包含这段附加间隔。这样横向滚到末尾时不会裁掉 Label 尾部文字。

### 6.4 绘制与输入

继续复用现有 `bodyRect`、阴影占位和内容安全 ClipRect：

- 原生表面的位置为退避后主体位置减去阴影左上占位。
- 根 Widget 仍安排在表面内的 bodyRect。
- Widget 输入沿用现有局部坐标转换，不重复减去桌面退避偏移。
- 阴影／圆角外侧的模态关闭行为保持不变。
- 不增加 Painter 接口，不改变阴影算法或应用样式。

## 7. 创建、更新与原生尺寸反馈

### 7.1 首次显示

创建前完成工作区查询、内容测量和首轮求解，直接以受约束尺寸创建原生 Popup，
避免先创建超大绘制表面再缩小。

### 7.2 再次显示

每次 Show 都重新查询工作区、计算位置并提交原生定位，覆盖主窗口移动、锚点移动、
跨屏以及两次显示之间工作区发生变化的情况。

保留上一切片已经修复的规则：同一个 owner 内坐标不能作为跨多次 Show 跳过
原生定位的依据。

### 7.3 已显示期间

内容、样式、显式限高和现有尺寸变化触发布局时重新求解；普通重绘不查询工作区。
每轮求解使用同一个工作区快照，不在尝试候选位置的过程中重新选屏。

首版不定时轮询，也不承诺静止显示期间立即响应所有显示器配置变化。
专门的显示环境变化通知留作后续独立能力。

### 7.4 SizeEvent 与取整

- SizeEvent 继续是实际逻辑／物理表面尺寸的事实来源，请求尺寸不能覆盖它。
- 根据实际表面尺寸复核边缘位置，必要时修正位置。
- 保留同一次显示期间的请求去重，原生像素取整不能引起尺寸请求循环。
- 不因普通取整差异反复缩放内容。
- 同步与异步 SizeEvent 都必须安全，布局期间新发起的布局请求不能丢失。
- 关闭、卸载或销毁发生后，不继续使用旧求解结果操作已释放资源。

## 8. 查询失败与降级

平台只返回事实或错误，以下策略属于 GUI：

| 情况 | GUI 行为 |
|---|---|
| WorkAreaAt 返回 ErrUnsupported | 保留现有无退避行为，提供可诊断日志，不虚构工作区 |
| Show 时返回 ErrUnavailable 或其他查询错误 | 本次 Show 返回错误，不使用上次显示缓存的旧区域 |
| 已显示期间重新查询失败 | 保留最后完成的布局，不提交部分计算结果 |

工作区查询错误不属于透明表面创建错误，不触发 PopoverMenu 的不透明 Popup 重试。
生命周期、失败资源清理和原有透明降级路径继续保持独立。

## 9. 代码组织与实施顺序

计划涉及：

- `platform/common/window.go`：DesktopWindow 查询契约。
- 三平台窗口后端：查询、选屏、坐标转换及错误处理。
- 三平台 SDK／frameworks／libs：必要的薄原生绑定。
- `gui/popover_placement.go`：私有纯几何求解，不依赖原生对象或 Widget。
- `gui/popover.go`：工作区快照、显示流程、尺寸反馈及位置提交。
- `gui/menu.go`：MenuButton 矩形锚定，以及基于最终约束的滚动条占位。
- 对应平台测试、GUI 几何与生命周期测试、边缘弹出样例。

实施按以下顺序分段验证：

1. 三平台 WorkAreaAt 查询，在 platform 层验证事实及坐标。
2. GUI 私有纯几何算法，用确定性用例验证候选、平移和收缩。
3. Popover 尺寸约束及原生反馈接入。
4. MenuButton 矩形锚定和菜单滚动布局接入。
5. 全仓库回归、跨平台编译及三平台真机验证。

## 10. 验证与验收

### 平台查询

- 单屏、多屏、显示器间空隙和不规则排列。
- 主窗口跨屏而锚点位于另一显示器。
- 主窗口移动后相同输入点的返回区域变化。
- 返回负坐标及超出 owner 客户区的合法矩形。
- 任务栏／Dock／面板占位和 X11 保守交集限制。
- 不同 DPI、自定义逻辑缩放、无效参数、能力缺失、无效原生对象及暂时查询失败。

### 几何、布局与输入

- 工作区四边、四角及完整可容纳的正常位置。
- 点定位与矩形锚定采用各自明确的退避规则。
- 翻转优先于平移，平移优先于收缩。
- 菜单超过锚点一侧空间但未超过整个工作区时，不被过早限高。
- 阴影不改变主体锚点，完整表面的边界参与容纳判断。
- 限高后滚动条及间隔不会意外挤出水平滚动条；确实过宽时水平滚动有效。
- 请求位置不变，反复显示不产生累积偏移。
- 实际表面取整后的边缘校正，不出现重复请求或布局循环。
- 退避后项目命中、空白区域和阴影区域输入坐标正确。
- 工作区查询失败不会触发透明降级，回调关闭／销毁后不使用过期资源。

实现阶段运行 `go test ./...`，交叉编译 Windows／macOS 相关测试目标。
三平台真机验证不同 DPI 下的边缘弹出、跨屏和超长菜单，并记录 X11 工作区报告的限制。
编译、自动测试和真机验收分别记录，不互相替代。

### 当前进度

- [x] 独立设计文档。
- [x] 三平台工作区查询代码、共享几何／X11 属性与 ABI 测试、本机 X11 原生查询。
- [x] 纯几何退避求解与测试。
- [x] Popover、MenuButton 和菜单布局接入。
- [x] 全仓库回归及 Windows amd64／macOS arm64 GUI、platform/test 测试目标交叉编译。
- [ ] 三平台真机验收。

### 实施验证记录

- `go test ./...` 通过。
- 本机 X11 `TestWindowWorkArea`：1x 返回 `{X:0, Y:41, Width:2560, Height:1336}`；
  `GOUI_PLAT_SCALE=2` 返回 `{X:0, Y:20.5, Width:1280, Height:668}`。均为新建
  owner 客户区坐标；测试同时覆盖屏幕外最近显示器查询、无效点与销毁后查询。
- GUI 测试覆盖候选顺序、阴影、负坐标、再次显示刷新、实际尺寸取整、普通帧不查询、
  查询失败不触发透明降级、测量／缩放期间关闭、工作区限高、横纵滚动与局部输入命中。
- `example/modern` 底部增加 `Edge / long menu`（120 项）。移动主窗口至屏幕四边
  和四角后测试，再跨屏、切换字号／亮暗样式重复；不需要手工设置菜单最大高度。
- Windows／macOS 编译通过不代表真机定位验收；多屏不规则排列、不同 DPI、任务栏／
  Dock／面板配置及真实弹出外观仍需分别验证。X11 始终保留前述保守交集限制。
