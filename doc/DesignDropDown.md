# DropDown

> 2026-10-06：代码、本机及 Windows Direct2D / macOS OpenGL 1x/2x 指定桌面场景验收完成；
> macOS 1x 短列表的原生尺寸取整回归已修复并复验。非编辑式单选控件位于 widgets，UI 使用
> Selected / OnSelected；编辑、搜索、多选、分组不属于本切片。

## 数据和契约

DropDownItem 包含 Text string、Disabled bool，复用 gui.ListData[DropDownItem]
及 SliceListModel，不引入 Value any 或另一套存储框架。Selected 为零基索引，
-1 表示未选，默认不自动选择。SetSelected 静默、越界清空，允许程序选择禁用项。
ConnectSelected / UI OnSelected 仅通知用户确认的实际改变，同项不重复通知。
索引不是稳定业务 ID：Items 保留有效索引、越界清空，重排后的业务对象保持由
应用同步 Selected，不猜测对象或文本身份。SetModel 总是重装并清空选择；
UI 对同一个非 nil 模型指针跳过重装，查询和 Items 发布在 GUI 线程进行。

公开接口：Model / SetModel、Selected / SetSelected、Placeholder / SetPlaceholder、
Enabled / SetEnabled、Padding / SetPadding、Open / Close / Opened、ConnectSelected、
ConnectOpenError、Delegate / SetDelegate、SelectedDelegate / SetSelectedDelegate、
Refresh、PopupMaxHeight / SetPopupMaxHeight。Open 返回失败；输入触发失败仅发出
OpenError，不重复报告或默认弹窗。

Enabled / SetEnabled 来自 Widget 公共接口；gui.IsEnabled 查询祖先限制后的结果。
禁用保留选择和 Focusable，不自动抢回焦点，也不增加启用变化的关闭监听。
已展开弹层遵循原有失焦／取消规则；下次输入先检查所属 DropDown，若不再
允许交互则消费操作并关闭，不触发自定义选项回调或提交选择。
收起内容继承实际启用限制，但由自身 StyleName 解析 Disabled 样式，不继承
父控件颜色。DropDownItem.Disabled 仍为独立数据语义，不等同于 Widget 的
自身 Enabled。详见 [统一禁用契约](DesignWidgetEnabled.md)。

## 内容和分层

控件管理收起展示和箭头；Popover 管理原生浮窗与退避；ScrollView 管理滚动；
ListView 仅管理可见行、测量和回收，不改变其公共接口。Popover 公开
Placement / SetPlacement，DropDown 打开时确保使用 PopoverPlacementBottom，
再统一调用 Show：完整锚点矩形优先向下，已有算法退避到上方或工作区内。
SetPosition 只保存点坐标，不隐式改变定位策略；MenuButton 共用 Bottom，
PopoverMenu.ShowAt 显式请求 Point。详见当前 [Popover 定位接口与实现](../gui/popover.go)。

SetDelegate 复用 ListItemDelegate，默认构建行和收起内容；SetSelectedDelegate
单独覆盖收起内容。两处创建独立 Widget，不共享或重挂同一实例。UI Item 返回
View，SelectedItem 覆盖收起展示，复用 BuildContext.UpdateChildren，不为此修改
Root / Coordinator / BuildContext。行壳负责背景、勾选、禁用和确认；内容是实际
Widget 子树。默认 Label 用自己的样式名称解析，父控件不隐式传递样式。

UI 使用示例（模型在重建之外创建）：

```go
model := ui.SliceList([]wui.DropDownItem{{Text: "系统"}, {Text: "亮色"}, {Text: "暗色"}})
selected := -1
// 在已有 Root 构建中使用；app 是 ui.App，wui 是 widgets/ui 的别名。
choice := wui.DropDown(model).Selected(selected).Placeholder("选择外观").OnSelected(func(index int) {
    selected = index
    app.RequestUpdate()
})
```

省略 Selected 时保留控件的用户选择；自定义 Item / SelectedItem 时自行为内容
设置 StyleName，Disabled 等数据不会通过父控件隐式传递子样式。

## 输入和生命周期

selected 为已提交值，current 为弹出期间浏览位置。打开时从 selected 或第一个
可用项开始。点击、Enter / Space 确认，先关闭再发送 Selected；Esc、外部点击、
失焦取消并保留 selected。关闭时 Up / Down 直接选邻项；打开时 Up / Down、
Home / End 仅浏览，跳过禁用项、不循环，通过 ListView.Reveal 滚入。Enter / Space /
Alt+Down 打开，重复打开键不反复切换。Tab / Shift+Tab 关闭后将该键重新交给
Window.DispatchEvent 导航，不绕过正常路径。关闭时滚轮不改值，打开时正常滚动。
空或全部禁用不打开。隐藏、卸载、模型重装关闭并取消输入；禁用后的独立弹层
按上述输入入口规则拒绝操作并关闭。相同 Selected
声明同步不打断浏览。信号、Delegate、原生操作可能重入或销毁，后续检查版本及
生命周期。控件负责关闭 / 释放其 Popover，UI 只释放自己的协调状态和连接。

## 尺寸、样式、快照

收起按当前内容 / Placeholder、箭头、Padding 测量，不扫描全部文本。弹出内容
等于控件宽度（工作区不足收窄），不随滚动改变宽度。短列表按实际高度撑开，
长列表只创建可见行并沿用 ListView 高度估计。PopupMaxHeight 默认 320 DIP，0
取消用户限高但仍受工作区限制；内部安全留白避让圆角和边框，不在阴影区绘制。

弹出主体的外围留白至少为 6 DIP，较大的圆角或边框要求更多安全空间时对称增大。
无垂直滚动条时，选项与主体左右边缘留白一致；有垂直滚动条时，左边缘到选项、
选项到滚动条之间保持同等留白，滚动条独立占列，外侧保留主体安全留白。
选项背景、勾选、子控件布局、命中及快照使用实际缩窄的选项边界；尾部间隔属于
ListView 的滚动占位，不通过增加文字 Padding 或只缩绘制背景制造空隙。
宽内容的自然宽度包含尾部间隔，横向滚动到底后其右边缘与勾选仍完整可见。
根据完成布局后的内容高度决定是否保留间隔，长短列表切换及行回收后重新计算。

macOS Popup 创建及调整尺寸在平台边界向上对齐物理像素，避免 1x 自定义双行
短列表的 180.058594 DIP 内容被压入 180 DIP 视口。保持原始测量与 ScrollView
溢出规则，不在 DropDown 增加补偿。转换测试覆盖原生 1x/Retina、缩放覆盖值、
整数及小数尺寸；窗口例程用 Shrink/Reset 验证原生 Popup 复用时尺寸更新。

独立样式：drop-down、drop-down-text、drop-down-text-disabled、drop-down-placeholder、
drop-down-item、drop-down-item-text、drop-down-item-text-disabled、drop-down-popup。
theme/basic 已提供 GUI 自动使用的完整兜底，Modern 提供亮暗统一的按钮密度、2 DIP FocusVisible、中性
灰行高亮、勾选及圆角阴影。Padding 是控件属性，不进入样式系统。
角色：combobox / listbox / option；goui.dropdown 属性携带索引、数量、展开或浏览
状态，不修改 gui.WidgetInfo。弹出为独立 Root 快照，不伪装主窗口子树，不实现
离屏行来制作快照。

## 参考

- [GTK4 DropDown 官方接口](https://docs.gtk.org/gtk4/class.DropDown.html)，查阅页面
  4.23.4；[4.20.3 源码](https://github.com/GNOME/gtk/blob/4.20.3/gtk/gtkdropdown.c)
  使用独立 selection / popup_selection。采用模型 / 内容工厂、浏览确认分离，
  不引入 GTK 属性表达式。
- [Avalonia 11.3.9 ComboBox](https://github.com/AvaloniaUI/Avalonia/blob/11.3.9/src/Avalonia.Controls/ComboBox.cs)：
  确认浏览项及 Tab 关闭后继续导航；不采用关闭时滚轮改变值。
- [Qt QComboBox](https://doc.qt.io/qt-6/qcombobox.html)：索引选择及用户激活与程序
  同步的区别。setter 静默、Selected 命名遵循 GOUI 已确认值控件约定。

## 验证

包内：默认值、静默 setter、模型变化、禁用跳过、浏览 / 确认 / 取消、有效释放、
重入、内容独立与回收、有限尺寸、快照；UI 模型保留、显式 / 非显式 Selected、
最新回调、独立内容、卸载连接；样式 / 像素涵盖中性背景、主题、焦点、clip 和
1x / 2x。中文窗口例程 tests/window/widgets/drop_down 明确键鼠、长列表、退避、
重建、主题及关闭步骤，不依赖 DevServer。运行全仓、相关 race / vet、Windows
amd64 / macOS arm64 编译；原生桌面与编译分开记录。

### 执行记录

- 选项留白调整：外围最小 6 DIP，出现垂直滚动条时增加等宽尾部间隔；仅调整
  widgets 的行测量/边界，不修改 GUI ScrollView/ListView/Painter 或 UI 接口。
  新增几何、长短切换、间隔输入、宽内容和 1x/2x 预乘像素回归；使用仓库外旧实现
  overlay 重跑三项测试均失败，分别复现 4 DIP 外边距、滚动末端间隔不足和高亮侵入。
- 本轮 `DISPLAY=:99 GOUI_PLAT_PAINTER=software go test -json ./...`：2000 个测试/
  子测试通过，14 个既有跳过。`go test -race ./widgets/...`、相关 widgets/Modern/
  窗口例程 vet 通过；Windows amd64/macOS arm64 的 widgets 与窗口例程目标编译通过。
  既有全仓测试的桌面及 PNG 输出副作用保持原状。四个 28 DIP 行加两侧各 6 DIP
  外围留白，自然高度由 120 调整为 124 DIP，对应测量断言同步更新。
- 本机 Linux amd64、隔离 Xvnc :99 / Xfwm4：Software 1x、OpenGL 2x（Mesa llvmpipe）
  的 Modern 长列表高亮/勾选与滚动条分隔、短列表左右留白实测通过；间隔点击后浮窗
  保持可见，Esc 后 F6 状态检查仍保持原选择/计数，关闭均 DESTROY PASS、退出码 0。
  截图和日志保存在 `/tmp/goui-dropdown-gutters.8vfq8S/`。后续远端复验见下一项；
  硬件 OpenGL 专项、真实 Retina 或混合 DPI 未验证。
- 2026-10-06 留白调整远端复验：Windows 10 19045 amd64（Direct2D 配置、RDP
  1600×900）与 macOS 15.7.7 arm64（OpenGL 配置、VNC 1650×1050），分别使用
  `GOUI_PLAT_SCALE=1/2`。通过真实鼠标/键盘输入确认：长列表高亮/勾选避让滚动条，
  左侧及滚动条前间隔为 6 DIP（2x 为 12 像素）；间隔点击保持浮窗打开和选择/计数
  不变；拖动滚动条显示约第 5000 行，Home/Enter 仅确认一次；Custom 四行、Shrink
  一行、Reset 四行均无滚动条且两侧留白一致。四次 F6 身份/状态与 DESTROY 断言
  均通过，无 panic；macOS 两次和 Windows 2x 退出码为 0，Windows 1x 原启动包装
  未成功记录退出码，仅记录正常销毁。2x 为应用缩放覆盖，不代表真实 Retina/
  系统 200% 或混合 DPI；后端按环境配置启动，未独立断言工厂的实际 fallback。
  产物 SHA-256 校验一致，截图和日志见 `/tmp/goui-dropdown-remote.6CMgmP/`。
  本轮仅验证指定 DropDown 场景，不代表菜单模式切换、透明降级或全部定位能力。
  测试窗口、临时 Windows 启动任务和本轮桌面连接已清理，保留已有用户程序及 SSH。
  工具问题：remote-shell 大文件 stdin 读取等待 EOF；macOS 改为明确长度读取，
  Windows 改为临时 HTTP 传输并校验哈希，服务及悬挂上传读取进程已关闭。
  桌面输入成功仅表示投递、截图可能仍为缓存帧；Windows Run 连续操作曾执行旧
  历史命令，已关闭误启动程序，改为分步截图及交互会话启动，不将旧帧当验收结果。
- 定位接口整理已迁移到公开 Placement / SetPlacement 与 Show；新增替身检查
  DropDown 实际请求 Bottom。当前全仓、race、Windows/macOS 编译及 Linux Software
  1x 定位回归结果见 [PopupPlacement](DesignPopupPlacement.md) 的 2026-10-06 记录。
  2026-10-14 已同步其过时章节，复核分类见 [文档审查记录](DesignDocumentAudit.md)。
  下列初次实现和 macOS 尺寸修复结果保留各自环境，不代表本轮远端桌面复验。
- 完成 widgets / widgets/ui、显式兜底 / Modern、具名快照，以及 Popover 定位
  通用入口；未改变平台、Painter、ListView 或 UI Root / BuildContext 的接口。
- `DISPLAY=:99 GOUI_PLAT_PAINTER=software go test -json ./...`：49 个有测试包，
  1987 个测试 / 子测试通过，14 个既有人工 / opt-in 测试跳过，无失败。现有平台
  测试会在指定桌面短暂创建窗口，现有像素测试可能覆写源码目录原有 PNG；本轮
  新增测试使用内存像素，不新增此类副作用。
- `go test -race ./widgets/... ./gui ./theme/modern` 通过。
  `go vet ./widgets/... ./theme/modern ./tests/window/widgets/drop_down` 及
  `go vet -composites=false ./gui` 通过。默认 `go vet ./gui` 未通过：既有
  text_editor.go、text_view_test.go、popover_test.go 等存在 unkeyed geometry / TextSelection
  字面量警告，已用 HEAD 核对；不借本切片改写无关代码或声称默认 vet 全部通过。
- Windows amd64 / macOS arm64：`CGO_ENABLED=0 go test -exec=true ./widgets/...
  ./gui ./ui ./theme/modern ./tests/window/widgets/drop_down` 各目标编译通过；
  `-exec=true` 只验证编译，不执行目标程序。
- 本机 Linux amd64，隔离 Xvnc :99、Xfwm4 合成器，Mesa llvmpipe LLVM 15：
  Software 1x / OpenGL 2x 的 UI Modern，Software 2x 的 UI 显式兜底，OpenGL 1x
  的命令式分支完成指定交互；不同后端 / 缩放不扩大为所有组合场景全部验收。
  已检查取消不提交、禁用跳过、确认一次、同项不通知、Tab 关闭继续导航、独立
  自定义内容、亮暗 / 蓝红、中性行背景、万项已选定位 / Home 确认 / 滚动条拖动，
  Set 静默、非受控值重建保留、Shrink 清空越界及控件身份 / 快照断言。
  Software 1x 移动窗口后浮窗随锚点，下缘退避到上方，宽度 / 高度约束正确。
  正常退出及 UI Software 1x 的 `-destroy-on-selected` 均输出 DESTROY PASS，
  回调销毁后后续连接未运行、浮窗被释放。截图和日志保存在仓库外临时目录。
- 初次实现时未执行：Windows Direct2D / macOS OpenGL 真实桌面，硬件 OpenGL、混合 DPI，
  其他 Linux WM。交叉编译、离屏像素和 llvmpipe 验证不替代这些验收。
- 后续远端验收：Windows 10 19045 amd64 Direct2D / macOS 15.7.7 arm64 OpenGL，
  GOUI_PLAT_SCALE=1/2；四组共 84 项状态断言通过，检查指定外观、长列表、退避
  与正常关闭及信号内销毁。macOS 1x 自定义短列表出现无有效滚动范围的滚动条，
  诊断为 238.058594 DIP 原生表面请求被取整为 238，180.058594 DIP 内容只能
  获得 180 DIP 视口；2x 此次实际表面为 238.5 DIP，未触发。
- 本轮修正 macOS Popup 创建和 SetSize 的物理像素向上对齐；不改 Painter、
  ScrollView、ListView 或公共接口。远端 `TestPopupSizeInPoints` 的 9 组转换断言
  通过；OpenGL 1x/2x 实际鼠标打开 Custom，四行无滚动条，Shrink 一行及 Reset
  恢复四行仍无滚动条；Long 的万项滚动条可拖动，Home/Enter 选择第 0 项后
  F6 状态/快照断言通过，关闭均输出 DESTROY PASS、退出码 0。裸可执行文件，
  未改变系统缩放；实际 backing scale 为 1，Retina 仅有转换逻辑覆盖，混合 DPI
  与 macOS Software 桌面回归未执行。二进制哈希、日志、截图保存在仓库外临时目录。
- 本轮 `DISPLAY=:99 GOUI_PLAT_PAINTER=software go test -json ./...`：1987 项通过、
  14 项既有跳过；首次运行因 :99 不存在，既有 TestTypography 报 can not open
  display，建立隔离 Xvnc 后重跑通过。相关 race/vet 通过，macOS arm64 原生测试
  编译与运行、Windows amd64 例程编译通过。全仓旧测试仍有临时窗口/PNG 副作用。
