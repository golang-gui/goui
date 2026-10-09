# Modern 通用样式预设

## 范围与参考

2026-10-09：在现有 `theme/modern` 中增加六个具名预设，不增加 Widget、
Style 字段、CSS class 组合、隐式父子继承或平台适配。

参考 [libadwaita 1.9.4 的样式约定](https://gnome.pages.gitlab.gnome.org/libadwaita/doc/1.9/style-classes.html)：
Flat 区分静止与交互表面，Card 表达内容分组，Warning/Error/Success 表达状态。
这里只采用视觉语义，不复制 GTK 的 class 组合、继承或自动修改子控件机制。
GOUI 的 Sidebar 表示背景表面，不等同于 libadwaita 的 navigation-sidebar 列表策略。

## API 与规则

| 常量 | StyleName | 消费方与默认值 |
| --- | --- | --- |
| `modern.Card` | `card` | 绘制背景的 Widget；中性表面、1 DIP 柔和边框、10 DIP 圆角、明确无阴影 |
| `modern.Sidebar` | `sidebar` | 绘制背景的 Widget；中性底色、直角、零边框、明确无阴影 |
| `modern.Flat` | `flat` | Button/MenuButton；静止和禁用透明、悬停/按下中性灰、6 DIP 圆角、零普通边框 |
| `modern.Warning` | `warning` | Label/Icon 等前景消费者；警告色，不添加背景、边框、圆角或阴影 |
| `modern.Error` | `error` | Label/Icon 等前景消费者；错误色，不表示危险操作按钮或输入校验状态 |
| `modern.Success` | `success` | Label/Icon 等前景消费者；成功色，不添加表面装饰 |

Sidebar 使用独立的调色板值：亮色 `#E2E2E8`，暗色 `#18181C`。
两种模式均比 Window/Card 更深，不再通过窗口背景向正文混合 2.5% 推导。
本轮只改变侧栏底色，不改变 Flat、普通按钮、Primary 或语义前景；
子控件的悬停/按下、当前分类与 Tab 焦点仍由各自原有规则绘制。

每个预设都有独立的正文前景与主题字体规则。禁用时使用既有中性禁用色；
不影响控件行为。Flat 复用按钮的独立 focus part：鼠标聚焦零边框，
FocusVisible 为 2 DIP 柔化主题色边框，不改变布局或悬停/按下底色。
应用仍可在 `modern.Rules` 后追加规则覆盖同一 selector。

Color 按现有标准库 sRGB 值表达，交给既有绘制边界转换。状态色只随 Dark 改变，
不随 AccentColor 改变：

| 前景 | 亮色 | 暗色 |
| --- | --- | --- |
| Warning | `#8A5900` | `#EAC16D` |
| Error | `#BA3038` | `#F3959A` |
| Success | `#276B43` | `#83CAA2` |

三种状态前景在默认 Window/Card/Sidebar 和普通按钮表面上保持至少 4.5:1
对比度；禁用色沿用既有弱化表现，不承诺正文对比度。

样式名是完整预设，而不是叠加 class；`Style("flat primary")` 没有组合语义。
父控件不把样式传给子 Label/Icon，调用方分别设置。
Card/Sidebar 不提供 Padding 或布局，也不让不绘制背景的 HBox/VBox 自动变成表面。
只有自绘表面的消费者才绘制这些字段。

## 示例集成

`example/demo` 继续使用现有应用自定义 surface Bin：全部 section 选用 Card，
侧栏外壳选用 Sidebar，未选中的导航按钮选用 Flat，当前页保留 Primary。
浮动查找面板、两种确认 Dialog、Toast、辅助窗口内容、权重布局表面及拖放接收区
同样选用 Card；遮罩保留独立的半透明规则，透明窗口的外围保留独立背景规则。
基础页增加可点击/禁用 Flat 以及独立设置样式的警告、错误、成功文字和图标；
新增文案在 resource 中提供现有八语言翻译，固定 ID 不参与翻译。
不修改 Window Integrated 避让、透明窗口背景、页面滚动边距或控件默认行为。

`example/modern` 增加 Flat、禁用 Flat 和三种状态文字。
GUI 兜底模式清空这些预设名，不依赖未启用的 Modern 规则。

## 验证

包内测试检查独立颜色预期、亮暗/极端主题色、字体完整性、状态前景、
禁用、Flat 的六种状态、focus part、对比度、应用覆盖及其他名字不受影响。
demo 布局回归使用实际 surface 承载侧栏，仍须填满分配空间且不增加边缘内缩；
资源测试覆盖新文本的八语言与格式参数。

本机窗口通过真实桌面输入检查：亮暗/主题色切换、Flat 悬停/按下/点击、
禁用不可激活、Tab 焦点、语义图标与文字、侧栏导航、卡片边缘、缩窄后滚动。
不使用 DevServer；后端和缩放覆盖值与实际桌面缩放分别记录。

## 实施与验收记录（2026-10-09）

- 六个预设、完整 Flat 交互/focus、状态色与独立字体规则已实现；不修改 gui、
  ui、style 或 platform。demo 已移除私有 demo-card 配色，采用公共 Card/Sidebar，
  新增七条具名八语言资源。导航当前页仍为 Primary，其他项使用 Flat。
- `go test ./...` 与非缓存 `go test -json -count=1 ./...` 通过：2138 项通过、
  14 项既有窗口/剪贴板/GLX 人工测试跳过，无新增失败或跳过。
- `go test -race ./theme/modern ./example/demo/...` 与
  `go vet ./theme/modern ./example/demo/... ./example/modern` 通过。
- Windows amd64、macOS arm64 的 demo 与 modern 示例交叉编译通过，
  `CGO_ENABLED=0`；没有把编译记录为原生运行或桌面外观验收。
- 本机 Linux amd64/X11，隔离 TigerVNC 1.12.0 / Xfwm4 合成桌面：
  Software 1x、OpenGL 请求配置 2x 的 demo 亮暗、主题切换和显示通过。
  2x 是 `GOUI_PLAT_SCALE` 应用覆盖，不代表物理显示器 HiDPI/混合 DPI；
  平台工厂仍允许 OpenGL 初始化失败后回退 Software，未插入私有后端探针。
- 真实桌面输入验证 Flat 悬停/按下/点击反馈、禁用点击不改变反馈、
  Shift+Tab/Tab 聚焦边框、侧栏切页；Software 1x 另检查 820×600 缩窄和滚动。
  截图采样用独立字节预期断言 Flat 悬停 `#EDEDF0`、按下 `#E5E5E7`，
  Card/Sidebar 两套背景以及三种语义色的实心文字/图标像素；
  改主题色后语义行像素保持相同。形状、文字抗锯齿不套跨平台金图。
- modern 示例 Software 1x 检查 Flat MenuButton 打开菜单、Flat action 更新
  输入框以及 Bare GUI 清空预设名；截图的透明 popup 黑色外侧是单独 drawable
  采集时丢弃 alpha 的结果，不当作桌面合成外观证据。
- 验证产物在 `/tmp/goui-modern-presets.9vRfgV/`，不写入仓库。
  程序正常关闭；隔离桌面随后停止，不操作用户原来的桌面会话。

未执行：Windows/macOS 原生桌面、本机 Software 2x/OpenGL 请求配置 1x 的
完整组合、物理显示器 HiDPI、八语言母语外观校对。新增文本的八语言定义、
实际解析与格式参数已由 resource 包测试覆盖。本轮没有平台或绘制契约变更。

## Sidebar 背景调整（2026-10-09）

侧栏旧背景通过窗口/正文混合 2.5% 得到，与 Flat 的悬停表面过近。
此次改为独立 palette 字段，亮色从 `#F1F1F3` 改为 `#E2E2E8`，
暗色从 `#252529` 改为 `#18181C`。不改变 Flat、普通按钮、Primary、
语义前景、布局、边缘避让或 demo 的声明。

新增回归检查侧栏比 Window/Card 和既有 Flat 交互表面更深，正文、MutedText、
Warning/Error/Success 在实际 Sidebar 背景上的对比度不低于 4.5:1。
原有表面颜色预期随契约调整，Flat 的交互颜色预期保持不变。

已通过：`go test ./...`、Modern/demo 包非缓存测试、相关 race/vet；
Windows amd64/macOS arm64 的 Modern 测试目标编译。
本机隔离 TigerVNC/X11/Xfwm4、Software 1x 的 demo 检查亮暗模式、分类切换、
Flat 悬停/按下及 Primary 选中项。截图按独立 RGB 字节断言新背景和原交互色，
并与上一版 Software 1x 截图比较选中项内部及右侧卡片内容，确认不受影响。
验证产物在 `/tmp/goui-sidebar.O79RsC/`；测试程序和隔离桌面已退出。

本轮未重新执行 OpenGL/Direct2D 原生窗口、远端桌面或物理 HiDPI 验收。
此前按用户要求保留工作区供体验，本轮用户已明确要求提交。

## demo 卡片引用修复（2026-10-09）

用户反馈的浮动查找面板与遮罩 Dialog 透明，原因是 demo 删除私有 `demo-card`
规则后，五个场景文件中仍有八处旧样式引用。具名样式不存在就不提供背景，
surface 不应自行猜测 fallback；这不是 Sidebar 配色或透明绘制契约的问题。

将查找面板、确认 Dialog、Toast、标签关闭确认框、辅助窗口内容、两个权重布局
表面及拖放接收区统一改为 `modern.Card`。保留三处同名 `demo-card` 拖放数据
格式标识，不添加兼容样式别名，不修改 gui/ui/style/platform 或 surface 绘制实现。
遮罩 alpha 140、透明辅助窗口的外围、现有布局与事件处理保持不变。

回归测试 `TestDemoCardStyleMigration` 只检查 Style/SetStyleName 调用：在修复前
明确报告八处遗留引用，修复后通过；不将应用内数据格式误判为样式。

验证结果：

- `go test -json -count=1 ./...` 通过：2140 项通过、14 项既有测试跳过，
  无新增失败或跳过；不把无测试文件的包计作测试跳过。
- `go test -count=1 ./example/demo/... ./theme/modern`、相关 race 与 vet 通过。
- Windows amd64/macOS arm64 demo 的 `CGO_ENABLED=0` 交叉编译通过；未原生运行。
- 本机 Linux amd64，隔离 TigerVNC 1.12.0/X11/Xfwm4 合成桌面、Software 1x，
  真实鼠标输入复现旧版面板/Dialog 背景缺失，再验证修复后的亮暗模式。
  查找面板、确认 Dialog、Toast、标签关闭确认框、两处权重表面、拖放接收区和
  透明辅助窗口内部卡片均恢复背景；切换模式时同步更新。
- 截图在避开文字、边框与抗锯齿的内部空白像素按独立 RGB 预期断言：
  Card 亮色 `(255,255,255)`、暗色 `(40,40,45)`。另采集完整桌面验证透明辅助
  窗口外围透出下方侧栏，而不是使用丢弃 alpha 的单窗口图片推断合成结果。
  初次诊断采样将 Dialog 下方的白色卡片误当窗口底色，修正采样基准后，
  旧版遮罩像素符合 `255 × (1 − 140/255) = 115`，未调整容差或绘制代码。

未执行：本轮 OpenGL/Direct2D 原生窗口、远端桌面与物理 HiDPI 验收。
产物位于 `/tmp/goui-demo-card.XW7jPy/`；测试应用正常关闭，隔离桌面停止。
