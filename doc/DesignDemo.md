# 综合演示例程

## 范围与组织

`example/demo` 以“分类导航 + 可操作页面”展示 v0.1 前的既有控件和机制，不取代
`tests/window` 中的专项回归。初始实现不增加框架能力；后续透明背景隔离修复
仅补充与 Popover 一致的 Window 样式名选择，不引入例程特例。参考 GTK 4.24.1 的
[Widget Factory](https://github.com/GNOME/gtk/blob/4.24.1/demos/widget-factory/widget-factory.c)
把控件放入可操作场景；仅采用展示组织方式，不复制 GTK 的全局资源/模板体系。

九类：基础控件、文本编辑、布局滚动、数据浏览、多标签、菜单覆盖层、图形动画、
系统拖放、窗口检查。内容和窗口交互均经过现有 ui / widgets/ui / gui 公共接口。
仅例程自己的背景 Bin、有限宽纵向 Scrollable、自绘图形和 ScrollBar WidgetView
作为扩展示范，不向 Root/BuildContext/Painter 添加特例。

## 状态、资源与生命周期

- 持久的文档、树、表、列表、图标和位图只初始化一次，不在 Build 中创建模型。
- 分类切换保留业务数据，但只挂载活动分类；编辑视图局部的焦点/滚动可重置。
- 动画和计时器默认关闭；分类切换停止演示任务、销毁手工 Popover。
- 标签跨窗调用现有 Coordinator 子树移交，提交后更新源/目标页面 ID 集合；脱离
  先声明空目标，再 AfterUpdate 定位/TransferTo，最后由 UI 正常显示。
- 额外标签窗口由 UI 管理；三种装饰示范窗由 GUI Application 管理，无额外引用表。
  主窗口关闭退出应用，最终清理由 Application 完成。
- 未接入 DevServer，不在演示或验收中使用语义动作捷径。

## 样式、系统操作与契约

使用 Modern，顶部显式选择系统/浅/深和主题色。每个子控件以自己的 StyleName
解析，不依赖父样式继承；表格编辑器具有专用完整文本样式和直角焦点边框。

透明能力不能代替清空背景。透明 None 窗口用 Window.SetStyleName 选择
`demo-transparent-window`，其 BackgroundColor 为透明；其他窗口沿用默认
`window`。样式表仍为应用级，但背景选择属于每个窗口，不修改共享规则。
Linux Integrated 的圆角也依赖 alpha surface，不能因此修改其他窗口的背景。
不再维护辅助窗透明标记或关闭时恢复背景的逻辑，生命周期由 Application 管理。
遮罩 Dialog 只承诺指针遮挡，不冒充完整键盘模态。

Window 统一分配根布局的可用内容区，demo 主体不再额外添加 8 DIP 外侧留白，
ScrollView 填满导航以外的区域，滚动条贴合实际内容区的右边界。例程不读取
Chrome 的私有边缘宽度，也不按平台补偿。Linux 自绘 Integrated 普通状态由
Window 自动避让四边缩放区域，HeaderBar 同样跟随根布局内缩；其他平台/模式
和最大化/全屏不重复扣除边缘。导航的 12 DIP、页面的 16 DIP 以及卡片、主题栏、
状态栏自身间距仍保留。透明 None 示例外围留白用于透出桌面，与边缘补偿无关。

剪贴板、文件对话框、打开 URL/路径均由点击触发；不自动读写文件。不隐瞒
取消/不可用的空对话框结果，不把请求成功说成外部应用已完成操作。

## 验证

包内测试覆盖内存模型、稳定行 ID 编辑/排序、页面顺序、树展开集合、图标像素
非空、Scrollable 的宽度约束/偏移/子树及主体无重复留白的分配、滚动条完整
宽度/命中/Snapshot 坐标。桌面步骤见例程 README，使用真实
输入验证；跨编译不能替代原生窗口验收。执行范围与缺口更新在 Plan 和本节。

### 2026-10-07 实现与验证记录

初始实现限于例程、说明和路线记录，未改 gui/ui/widgets/platform 的接口或实现。
拖动 SplitView 的受控值由 OnResize 保存；动画图标用外层容器预留高度，内层
保留自然尺寸；分类根 ID 区分不同页面，避免共享原本应独立的滚动偏移。

通过：

- `DISPLAY=:83 XMODIFIERS=@im=none go test -json ./...`：2050 个测试/子测试
  通过，14 个既有窗口测试跳过，没有失败；包含缓存结果，不宣称全部重新运行。
- `go test -race ./example/demo`、`go vet ./example/demo`、
  `env -u DISPLAY go test ./example/demo -count=1` 通过。
- `CGO_ENABLED=0` 交叉编译例程：Windows amd64、macOS arm64 通过；没有在
  Windows/macOS 启动程序，不能视为两端原生验收。

本机窗口：Linux amd64，隔离 Xvnc 1.12.0 + Xfwm4 合成器；通过 XTest 实际鼠标/
键盘操作与截图观察，未使用 DevServer。XIM 使用进程级 `@im=none`。

| 配置 | 已操作范围 |
| --- | --- |
| Software 1x | 九类页面；普通按钮/禁用/选择、ASCII 编辑/提交/跨页文本保留、树展开/右键、表格备注与排序、列表滚动/点击、菜单与页内遮罩、Timer 开始/停止、应用内原生 DnD 预览/投递、标签跨窗与脱离、Snapshot/ID 查询 |
| OpenGL 配置 1x | 分类滚动独立性；窄窗换行、基线和布局；亮暗/主题色、图形/RenderWidget、尺寸动画中间值、忙等待；通用 Popover；Native/Integrated/透明 None 窗口、None 标题拖动及相对定位；脱离目标尺寸沿用 |
| Software 2x | 基础外观、DropDown 展开/键盘选择、分类切换、主题重绘、图形与尺寸动画、两种忙等待、深色页内遮罩 Dialog、带任务退出 |

最后的 SplitView 受控值补充在 Software 1x 复验：分隔线从 x=422 拖至 x=510，
内部分隔线从 y=556 拖至 y=605；F5 重建后保持两条分隔线位置。例程关闭主窗
能够结束事件循环并清理已打开的演示窗。
最后的表单检查还观察到内层 HBox.CrossStretch 的测量/排列高度差异；例程不需
该拉伸，移除后接收区为 110 DIP 且末尾说明可完整滚到，应用内投递仍返回操作
1。计时器离页后回到该页计数保持不再递增。两个发现的区别见开发记录。

OpenGL 环境为 llvmpipe；平台既有创建失败后回退 Software 的路径没有增加诊断，
故只报告 OpenGL **配置**，不把它扩大为确认的硬件 GPU 验收。应用 2x 覆盖值不
等于真实高 DPI、Retina 或混合 DPI。

未执行：Windows/macOS 原生桌面、真实中文预编辑/候选窗、跨应用文本/文件拖入
拖出、原生文件对话框、OpenURL/OpenPath 外部应用结果及混合 DPI。没有为验证
关闭系统安全设置、读取真实文件或使用 DevServer。部分既有全仓测试仍生成
源码目录图片等产物，本轮不迁移或删除它们；新例程测试仅操作内存。

已发现的 XIM 环境问题和公共能力限制见 [DemoFrameworkNotes](DemoFrameworkNotes.md)，
没有把有契约依据的行为和例程约束/绑定错误写成框架缺陷。

### 2026-10-07 用户体验反馈修复

- 修复主体 ScrollView 铺至边缘导致 8 DIP 滚动条被 5 DIP 窗口安全 clip 裁掉
  的例程布局问题；窗口内核是否应自动避让另见 DemoFrameworkNotes §9。
- 修复辅助 Integrated 窗口的 Transparent() 触发共享透明背景的误判。真正显式
  打开透明 None 时仍有应用级背景副作用；这是第一轮遗留，下一节已彻底修复。
- 两项新无桌面回归：完整滚动条边界/留白，显式透明示范与多窗口关闭恢复。
  临时恢复原来的无留白与 Transparent() 判定后，两项均失败；恢复修复后通过。
- `DISPLAY=:83 XMODIFIERS=@im=none go test -json ./...`：2052 个测试/子测试
  通过、14 个既有跳过、零失败，含缓存。`env -u DISPLAY go test ./example/demo
  -count=1`、例程 race/vet、Windows amd64/macOS arm64 的 CGO_ENABLED=0 编译通过。
- 本机 Linux amd64、Xvnc 1.12.0 + Xfwm4 合成器：Software 1x 与 OpenGL 配置
  2x 的滑块拖动、完整宽度与 Integrated 辅助窗打开通过；Software 1x 的深色、
  820×600 DIP 窄窗、Integrated 关闭，以及显式透明 None 开启/关闭恢复也通过。
  OpenGL 配置 2x 的最大化/恢复后，滚动条仍为完整 16 物理像素。
  截图在滑块中段采样：1x 的连续 8 像素 / 2x 的连续 16 像素均位于裁剪区内，
  相邻像素为背景；窗口右边缘宿主背景在 Integrated 开启前后保持相同的不透明
  RGB。基于已知黑色桌面的软件合成结果验证，不作为原生 surface alpha 读回。
- 沿用上节的 OpenGL 配置/fallback 与应用缩放边界；本轮未执行 Windows/macOS
  原生窗口验证，也未启动外部应用或读写剪贴板。仅关闭验证进程和隔离桌面，
  不修改用户桌面或系统设置。其余历史未验收项不因此视为通过。

### 2026-10-07 透明 None 独立背景修复

- Window 新增与 Popover 一致的 StyleName / SetStyleName，空名仍解析 `window`。
  只用于本窗口背景及装饰亮暗配色；不继承给子 Widget，不改变布局、Snapshot
  或 GUI 管理的装饰几何。Setter 只请求自身重绘，同值和销毁后调用不生效。
- 例程始终声明 `demo-transparent-window` 规则，仅显式透明辅助窗选择该规则。
  删除修改共享 `window` 背景、辅助窗引用表及关闭时恢复背景的临时机制。
  原来验证透明标记/全局背景恢复的例程测试随过时逻辑移除，由真实窗口背景
  像素回归替代；已有滚动条、布局、模型和页面测试保留。
- 新回归覆盖独立选择、空名恢复、销毁保护、主题更新、多透明窗逐个关闭和
  不透明内容绘制；Software 1x/2x 精确比较预乘 RGBA，采样避开抗锯齿边界。
  临时恢复固定 `window` 解析后，两种缩放均失败，恢复修复后通过。
- `DISPLAY=:83 XMODIFIERS=@im=none go test -json ./...`：2055 个测试/子测试
  通过、14 个既有跳过、零失败，含缓存。初轮因 UI 的 Window 测试替身缺少
  新方法而编译失败，补齐替身后全仓和 `go test -race ./gui ./ui ./example/demo`
  均通过；UI 生产代码未修改。`go vet ./example/demo`、无 DISPLAY 指定回归、
  Windows amd64/macOS arm64 的 CGO_ENABLED=0 全仓构建和 GUI/UI 测试目标编译通过。
- 本机 Linux amd64、隔离 Xvnc 1.12.0 + Xfwm4 合成器，真实鼠标/键盘输入：
  Software 1x 验证打开两个透明 None、切换深色、逐个关闭及再次打开 Integrated；
  OpenGL 配置 2x 验证透明 None、主题切换与关闭。辅助窗外围透出黑色桌面，
  内容仍不透明，主窗边缘始终保持自己的亮/暗背景。两个配置均正常退出。
- 未执行 Windows/macOS 原生桌面；OpenGL 配置/fallback、应用缩放与此前未验收
  项的边界不变。未改平台/Painter、自动边缘避让、用户桌面或系统设置。

### 2026-10-07 Linux Integrated 内核布局修正

用户确认后，Linux GUI 自绘 Integrated 根布局与四边安全绘制区统一；HeaderBar
跟随内容起点，Chrome 按钮垂直居中且右侧距离不重复内缩。其他平台/模式不添加
避让。边缘没有 Widget 命中时，事件分发器仍交给既有窗口控制器和手势竞争。
该轮例程仍保留 8 DIP 外侧间距，随后按用户确认移除这段重复留白，见下节。
包内/像素、全仓、race、跨编译及独立窗口验收记录见
[DesignGUIWindowChrome](DesignGUIWindowChrome.md#2026-10-07-linux-integrated-布局修正与验证)。

### 2026-10-07 demo 边缘调整

用户确认后，仅移除 `demoBody` 的外层 `Padding(8)`；保留导航、页面、卡片、
主题栏、状态栏及透明 None 外围的既有间距，不再修改 GUI 内核或 HeaderBar。
回归测试使用宽/窄尺寸与零/非零分配起点，独立断言导航、ScrollView 填满各自
区域，滚动条为完整 8 DIP 且贴合内容右边界，Snapshot 为窗口坐标，左右两侧
均能命中。新断言在移除旧 Padding 前四种分配均失败，修正后复验。

通过：

- `env -u DISPLAY go test ./example/demo -count=1`、
  `go test -race ./example/demo`、`go vet ./example/demo`。
- `DISPLAY=:84 XMODIFIERS=@im=none go test -json ./...`：2076 个测试/子测试
  通过、14 个既有窗口测试跳过、零失败，含缓存；既有测试仍有源码目录图片
  产物与原生窗口副作用，本轮不迁移或删除。
- `CGO_ENABLED=0` 编译 `example/demo`：Windows amd64、macOS arm64 通过，
  不作为原生运行验收。
- 本机 Linux amd64、隔离 Xvnc 1.12.0 + Xfwm4 合成器：Software 配置 1x、
  OpenGL 配置 2x 的基础页滑块左右侧拖动、820×600 DIP 窄窗、标题双击最大化
  和恢复均通过；Software 1x 另检查浅/深色。两进程关闭主窗后正常退出。
  滑块中段截图分别连续 8/16 物理像素，贴合内容右边界，相邻像素为背景；
  外侧只剩 Window 管理的边缘，没有旧的 8 DIP 例程补偿。截图是桌面合成后
  的 RGB 观察，不宣称读回原生 surface alpha。

未执行：本轮全屏桌面操作、Windows/macOS 原生桌面、真实高 DPI/混合 DPI 和
其他场景复验。OpenGL 使用配置值与本机 llvmpipe 环境，应用 2x 覆盖值不等于
真实高 DPI；此前验证过的透明 None 外围布局本轮未修改，未因此重新标记验收。
无 DevServer、剪贴板写入、外部应用启动或用户桌面设置修改；仅清理本轮隔离
桌面和验证进程，测试产物保存在仓库外临时目录。
