# 综合演示例程

## 范围与组织

`example/demo` 以“分类导航 + 可操作页面”展示 v0.1 前的既有控件和机制，不取代
`tests/window` 中的专项回归。初始实现不增加框架能力；后续透明背景隔离修复
仅补充与 Popover 一致的 Window 样式名选择，不引入例程特例。参考 GTK 4.24.1 的
[Widget Factory](https://github.com/GNOME/gtk/blob/4.24.1/demos/widget-factory/widget-factory.c)
把控件放入可操作场景；仅采用展示组织方式，不复制 GTK 的全局资源/模板体系。

九类：基础控件、文本编辑、布局滚动、数据浏览、多标签、菜单覆盖层、图形动画、
系统拖放、窗口检查。内容和窗口交互均经过现有 ui / widgets/ui / gui 公共接口。
仅例程自己的背景 Bin、自绘图形和 ScrollBar WidgetView 作为扩展示范，不向
Root/BuildContext/Painter 添加特例。表单现改用普通 ScrollView 的纵向方向，
删除临时 flowWidget；框架完善与验证见 [滚动布局](DesignScrollLayout.md)。

## 状态、资源与生命周期

后续滚动切片的本机复验：Software 1x / OpenGL 配置 2x 的表单、布局页、
820×600 DIP 窄窗与 Wrap 折行通过，Software 1x 系统页末尾完整可达。
专项窗口的四组 1x/2x 和全仓/race/目标编译记录见 DesignScrollLayout；
本轮不重复宣称所有九类场景已重新验收。

- 持久的文档、树、表、列表、图标和位图只初始化一次，不在 Build 中创建模型。
- 分类切换保留业务数据，但只挂载活动分类；编辑视图局部的焦点/滚动可重置。
- 动画和计时器默认关闭；分类切换停止演示任务、销毁手工 Popover。
- 标签跨窗调用现有 Coordinator 子树移交，提交后更新源/目标页面 ID 集合；脱离
  先声明空目标，再 AfterUpdate 定位/TransferTo，最后由 UI 正常显示。
- 额外标签窗口由 UI 管理；装饰、SkipTaskbar 和 BorderBox 示范窗由 GUI Application
  管理，无额外引用表。
  主窗口关闭退出应用，最终清理由 Application 完成。
- 未接入 DevServer，不在演示或验收中使用语义动作捷径。

### 文本占位提示

文本页在原有 TextInput / TextView 上设置公开的 Placeholder 属性，分别展示单行与
两行提示；保留初始示例，页面说明引导用户清空后查看，不另建一份演示编辑状态。
提示及说明跟随八语言资源，外观使用当前 Modern 的 placeholder 部件。
语言、主题或分类重建不重载用户清空后的文本，也不重置文档历史。只读开关继续
控制原有多行编辑器；空文档仍可显示提示，输入受只读约束。
操作与逐项预期见 example/demo/README，真实输入和主题切换按平台分别验收。

### Logo 与构建资源

采用用户提供的 `doc/logo/goui-mark.svg`（512×512 方形标志）和
`doc/logo/goui-logo.svg`（1220×512 横版标志），不修改图形或配色。
方形标志用于 HeaderBar、带图标按钮、标签页、尺寸动画和平台应用图标；横版标志
在图形页以 305×128 的固定 Image 展示。所有 Logo 必须使用 `WithRawColors`，
始终保留原色，不因主题色、亮暗模式或禁用状态染色，也不提供 Logo 单色版本。
方形 Logo 独立存放于 `d.logo`，不加入 `d.icons`。可着色集合中的三枚 SVG 改为
普通文档、文件夹和勾选图标，保留 Software 图标和原色文件夹样例；图标数量与
八语言提示不变。语义色和普通图标示范使用这个集合，不复用品牌资源。
资源只在 `newDemo` 解析/栅格化一次，不在 Build/Paint 加载文件。

按钮、标签页和动画必须绑定独立的原色 `d.logo`，不能复用可着色样例；移除动画
图标的 `demo-swatch` 样式，避免把品牌展示与主题色示范混为一谈。大小仍由
`motionValue` 驱动，沿用原有布局与动画，不修改 Icon/SVG 或任何绘制后端。
绑定回归检查按钮、标签页和动画声明，像素回归独立验证原色资源和展示尺寸下的
颜色稳定性；另外比较 alpha 掩码，禁止把 Logo 的单色副本加入普通图标集合。
资源和绑定验证不能互相代替，真实窗口仍检查小尺寸、禁用与亮暗切换的外观。

`go:embed` 不允许父目录引用，因此 `example/demo/resource` 存放逐字节一致
的副本；测试防止副本与设计源漂移。运行文件不依赖源码或启动目录。
`example/demo/goui.toml` 的 icon 直接引用原始方形 SVG，ID 与 `ui.Run` 一致。
复用 `cmd/goui build` 生成资源与二进制，不增加框架 API、专用图标系统或打包逻辑。
macOS 的 `.app` 和 Linux 桌面目录部署继续用现有 `package` 命令显式执行；
裸二进制构建不承诺安装图标或显示 bundle 图标。操作步骤见例程 README。

#### 2026-10-12 验证

- 无桌面的 `env -u DISPLAY go test ./example/demo/... ./cmd/goui -count=1`
  通过；嵌入副本与设计源逐字节一致。像素回归覆盖透明外角、蓝色窗框、白眼、
  鼻子、横版灰色字母以及不同前景色下的原色保留，不使用抗锯齿边缘金图。
- `go test -race ./example/demo/... -count=1`（无 DISPLAY）与
  `go vet ./example/demo/...` 通过。
- `DISPLAY=:84 XMODIFIERS=@im=none go test -json ./... -count=1`：
  2341 个测试/子测试通过、15 项跳过、零失败。跳过为 14 个既有窗口/手动项
  和只适用于 macOS 的签名测试；不是窗口程序的场景验收。
- 通过 `go run ./cmd/goui build -config example/demo/goui.toml` 构建
  Linux amd64，另使用 `CGO_ENABLED=0` 及 `-target windows/amd64`、
  `-target darwin/arm64` 构建两端二进制。输出于例程的 dist；Windows 同时
  生成 COFF 图标/版本资源，macOS 生成 plist/ICNS，Linux 生成 desktop/hicolor。
- 本机 Linux amd64，隔离 Xvnc 1.12.0 + Xfwm4 合成器，Software 1x 与 OpenGL
  配置 2x：从仓库外临时目录直接启动所构建二进制，实际点击图形页、亮暗及
  主题色按钮，截图观察方形标题栏标志和横版比例、原色、透明外缘；正常关闭。
  未使用 DevServer。OpenGL 环境为 llvmpipe，保留既有后端回退边界，不宣称
  硬件 GPU 或真实 HiDPI 验收；横版是固定像素 Image 样例，不是动态 DPI 图标。
- 未执行 Windows/macOS 原生桌面、macOS `.app` 打包/签名或 Linux 桌面资源
  安装；本轮没有改动平台或绘制后端。临时截图/日志留在仓库外，构建输出及
  生成的 Windows syso/归属标记由例程的 `.gitignore` 排除，不提交二进制。

#### 动画原色绑定补充验证

- `env -u DISPLAY go test ./example/demo -run '^TestAnimatedIconUsesRawLogo$'
  -count=1` 在旧声明下失败：`animated-icon uses a monochrome sample instead of
  the original-color logo`；修正为 `d.logo` 后，例程无桌面测试/race/vet 通过。
- 修正后非缓存全仓测试：2342 个测试/子测试通过、15 项既有跳过、零失败；
  Linux amd64、Windows amd64、macOS arm64 的 `cmd/goui build` 均重新完成。
- 本机仍用隔离 Xvnc/Xfwm4，Software 1x 和 OpenGL 配置 2x：实际滚动到
  动画区域，观察 32 DIP 初态、动画中间帧、80 DIP 终值，均能看到窗框、眼睛
  和鼻子的原色细节；亮暗/主题色切换不把 logo 染成实心方块。Software 还复验
  反向动画回到 32 DIP。两组正常退出，截图/日志在仓库外临时目录。
  验证未使用 DevServer，沿用上一节后端/缩放范围，未做远端原生桌面验收。

#### Logo 原色统一补充验证（2026-10-12）

- 新增 `TestDemoTintedIconsExcludeLogo` 和 `TestDemoBrandIconBindings` 后，
  执行以下无桌面回归命令在旧实现失败：

  ```sh
  env -u DISPLAY CGO_ENABLED=0 go test ./example/demo -run '^TestDemo(LogoSizesPreserveColors|TintedIconsExcludeLogo|BrandIconBindings)$' -count=1
  ```

  图标 0 复用了 Logo alpha 掩码，按钮和标签页绑定
  可着色资源。改为独立原色 Logo／普通文档图标后通过，不改 Icon 或 Painter。
- 保留既有 512 像素的独立原色采样；新增 16、18、24、32、36、48、64、80、
  160 像素下的完整 RGBA8 比较，覆盖亮暗、主题色、禁用灰及预乘半透明前景。
  同一尺寸的 Logo 像素必须完全一致，不用颜色容差隐藏着色或缩放错误。
- 无 DISPLAY 的 `CGO_ENABLED=0 go test ./example/demo/... -count=1`、
  `go test -race ./example/demo/... -count=1` 与例程 vet 通过。隔离桌面的
  `CGO_ENABLED=0 go test -json ./... -count=1`：2357 项通过、15 项既有跳过、
  零失败；保留既有测试的原生窗口和图片输出副作用。
- 本机 Linux amd64、Xvnc 1.12/Xfwm4 合成器：Software 1x 与 OpenGL 配置 2x
  实际检查按钮初态／禁用、亮暗／紫色主题、选中与未选中标签及图形页；普通
  图标从紫色切到绿色，Logo 和原色文件夹不变。OpenGL 配置 2x 另检查动画
  初态、中间帧及终态；窗框、眼睛和鼻子保持原色。不使用 DevServer。
- 截图中的按钮区域（1x 的 `[469,487) × [332,350)`，2x 坐标乘 2）检查
  不透明窗框的 RGB8 `#00ADD8`：亮色、禁用＋紫色、暗色＋紫色均保留原色，
  每种后端内部的像素数不变。跨后端不要求抗锯齿像素数量相等。
- `CGO_ENABLED=0 go build` 的 Linux amd64、Windows amd64、macOS arm64 目标
  通过；后两端仅交叉编译，未做原生桌面／Direct2D 验证。OpenGL 环境为
  llvmpipe，应用配置 2x 不是实际 Retina 或混合 DPI 验证。二进制、截图和日志
  位于仓库外 `/tmp/goui-demo-logo.WFHwcw`，窗口和隔离桌面正常清理。

## 样式、系统操作与契约

### 窗口框架的紧凑化

2026-10-12 用户确认先收紧三处视觉占用，不重排九类页面、不更改 Modern 公共
样式、窗口装饰策略或任何框架接口：

- 标题栏 Logo 从 28 调至 24 DIP，HeaderBar Padding 从 8 调至 6 DIP。
  标题不再复用 16pt 的 `demo-subheading`，改用独立完整文本规则 `demo-title`，
  采用 Settings 的正文字号/字体和既有前景色；字号无效时沿用 14pt 兜底。
  常规内容高度约 36 DIP，但不设置固定或最大高度，大字体依测量自然撑开。
  标题文字、原生控制按钮及其既有占位/垂直居中机制不变。
- 移除侧栏整段 Tab / Shift+Tab、F5 和系统操作说明；操作语义不变，说明移至
  README，侧栏只展示分类。
- 状态栏 Padding 从 10 调至 6 DIP，MinHeight 从 44 调至 28 DIP。
  保留原有 `status` ID、MutedText、MainWeight 和 WrapWordChar；28 是最小值，
  不是固定高度，长操作结果/错误和大字号仍能自然增加高度，不截断信息。

回归直接检查例程真实声明，避免为测试导出 UI 私有状态或建立第二套协调器；
桌面另检查实际字体、短/长结果、标题拖动和窗口按钮，不能把声明检查当作原生
外观验收。操作流程补充在 README。

#### 紧凑化验证

- 临时恢复本轮之前的三处声明，新回归失败，分别指出标题尺寸/样式、侧栏
  快捷键说明和状态栏留白/最小高度；恢复调整后回归通过。
- 无 DISPLAY 的 `go test ./example/demo/... -count=1`、例程 race/vet 通过；
  `DISPLAY=:84 XMODIFIERS=@im=none go test -json ./... -count=1` 非缓存全仓
  2343 个测试/子测试通过、15 项既有跳过、零失败。跳过范围同 Logo 验证记录；
  全仓既有测试的原生窗口/图片产物副作用未迁移或删除。
- `cmd/goui build` 构建 Linux amd64、Windows amd64、macOS arm64 demo 通过。
  后两端仅交叉构建，未执行原生桌面或图标安装/打包验收。
- 本机 Linux amd64、隔离 Xvnc 1.12.0 + Xfwm4 合成器：Software 1x 检查亮暗、
  1120×800 与 820×600 DIP、德文标题、长操作结果、标题双击最大化/恢复及
  连续拖动；OpenGL 配置 2x 检查亮暗、窄窗和短消息反馈。标题、Logo、窗口
  控制按钮居中，侧栏不再显示详细快捷键说明，短消息状态栏紧凑；在文本页
  通过真实键入和 Enter 提交长句，状态栏多行显示直至末尾 END，未截断结果。
- 另用临时目录的 GTK 配置和进程级 XDG_CONFIG_HOME 启动 Software 1x，字号
  为 18pt。标题和状态栏依内容自然增高，窗口控制按钮保持居中并可最大化；
  不修改用户的系统字体设置。该检查只覆盖本轮两个区域，不宣称所有页面
  已完成大字号适配。
- 输入经 XTest 和正常事件分发，不使用 DevServer。沿用 llvmpipe、OpenGL
  配置/fallback 和应用缩放覆盖的既有边界，不宣称硬件 GPU、真实混合 DPI
  或 Windows/macOS 原生验收。截图/日志在仓库外，验证进程和隔离桌面正常清理。

### 既有样式与操作契约

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

主窗口声明使用 `ui.Window("main").Center(true)`，按既有 1120×800 DIP 初始尺寸
请求工作区居中。只影响首次创建；主题、语言和页面重建不重新定位，辅助窗口的
相对定位及标签拖出后的显式定位保持不变。包内声明回归检查真实主窗口启用 Center，
不导出 UI 私有状态。窗口居中机制已完成三平台验收；按用户要求，此次例程调整
仅复跑代码测试，不重复桌面验收。Demo/resource 的非缓存测试及
`CGO_ENABLED=0 go test ./...` 均通过；全仓结果包含缓存，不作为再次窗口验收。

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

### 2026-10-07 深度体验反馈与暂缓优化

用户确认当前优先保证功能正确，高频 UI 绑定优化仅记录，不实施。候选范围、
契约保护和性能对照要求见 [DemoFrameworkNotes §10](DemoFrameworkNotes.md#10-高频-ui-绑定优化记录暂缓实施)。

用户新增九项反馈及源码核对见 [DemoFrameworkNotes §11](DemoFrameworkNotes.md#11-深度体验反馈功能优先的待修复清单)：
拖动光标跳变、TreeView 外观、排序恢复列宽、Linux 跨应用标签拖放、macOS
脱离位置、移交后关闭确认归属、空白 TabBar 拖动窗口、按钮键盘激活、滚动条
视觉留白。该轮只记录，尚未修复或桌面验收；既往通过的指定场景不覆盖这些问题。

后续实施先修已确认的功能错误，再核对原生会话/坐标，最后调整视觉。列宽的
Width 保持受控语义；确认弹层由各工作区拥有；Tab 导航只移动焦点，激活仍是
按钮责任。页面新增的视觉间隔与已移除的窗口装饰补偿不同，不逆转 Linux
Integrated 的自动布局避让。macOS 位置偏差与光标跳变暂保留为待验证原因，
不据此引入平台经验偏移或公共预布局接口。

### 2026-10-07 深度体验反馈修复

用户授权逐项分析、修复、验证和提交后，九项修改已实施。表格通过 OnResize
回写声明的 Width；各标签窗口拥有自己的关闭确认；滚动页面在视口外增加
对称的 8 DIP 视觉留白，不恢复主体的窗口装饰补偿。

脱离窗口沿用源内容尺寸和 HeaderBar 高度，但标签起点按目标 Chrome 报告的
原生控制区计算，不复制源页中的嵌套偏移。GUI Chrome 仅根据 HeaderBar 高度
垂直居中原生按钮，保留原生水平锚点；没有新增隐藏窗口预布局、客户区尺寸
或像素缩放的布局 API。控制区与释放 Hotspot 的包内断言、macOS 实际脱离
位置和 Linux/Windows 窗口行为已复验。

框架修改限于捕获手势期间的光标归属、Button/MenuButton 键盘激活、HeaderBar
识别已知 DropTarget、Modern 树行外观及 X11 拖放协商/终止清理。各项原因、
测试命令、平台配置与未执行范围见
[DemoFrameworkNotes §12](DemoFrameworkNotes.md#12-深度体验反馈修复与验证记录)。
高频 UI 绑定和样式动画仍未实施。

### 2026-10-10 补齐默认样例本地化

用户反馈编辑区和拖放区遗漏翻译后，仅修改例程和 resource，没有改变
platform/gui/ui API。新增 17 个八语言资源，覆盖单行/多行、基线/分割输入、
初始快照提示、默认标签文档、树项目、列表行、表格样例、计时次数和拖放说明。
编辑样例与用户数据的具体归属规则见
[DesignLocalization](DesignLocalization.md#demo-样例与用户数据的边界2026-10-10)。

单行示例复用 ui.State/BindText；多行使用 TextModel Revision 判断是否仍由
应用持有，不用文字相等猜测是否编辑。已编辑内容、历史、选区、模型、排序、
列宽和选择保持；标签关闭清理默认样例的 revision 记录。布局用户名也改为
持久 State，避免语言重建时把用户输入覆盖回品牌名。拖放原始数据和格式与
展示说明分开，空文本不等于初始状态，路径与实际文本不翻译。

验证（Go 1.25.8，Linux amd64）：

- 将 HEAD 的旧 demo 提取到仓库外目录运行英文回归，编辑器、初始快照、拖放
  邀请、树项目和备注五处均报告未翻译；这是预期的旧缺陷复现，不是修正后失败。
- 八语言初值与动态切换、同值赋值归属、编辑后撤销回原文再重做、备注重置、
  数据/列宽/排序保留、四种拖入及不支持格式、关闭文档记录清理均通过。
  既有记录测试把固定中文默认备注改为当前 resource.RecordNote，仍断言只有
  目标行被编辑。资源完整性测试验证 258 个资源及八语言格式参数一致。
- `env -u DISPLAY go test ./example/demo/... -count=1`、
  `go test -race ./example/demo/... ./mlang -count=1`、
  `go vet ./example/demo/...`、`git diff --check` 通过；最后的例程调整后再次
  race/vet 通过。
- 全仓非缓存复跑 2218 测试/子测试通过，14 既有跳过；既有测试会创建原生
  窗口，另在独立显示 `:89` 非缓存复跑通过，最后代码调整后 `DISPLAY=:89
  go test ./...` 也通过（含缓存）。第一次全仓沿用环境 `:3.0`；交互验收全部
  在独立桌面完成，未操作用户桌面。
- `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./example/demo` 与
  macOS arm64 目标通过；产物指定仓库外路径，不等于原生桌面验收。
- 本机 Ubuntu 22.04/X11、Xvnc、Xfwm4 合成环境、`XMODIFIERS=@im=none`：
  Software 1x 实际通过八个语言菜单切换，指定文本、数据、标签、布局、拖放和
  初始快照页面观察。英文默认样例随切换更新；单行/多行实际输入、选区、撤销
  到原英文样例及重做保持，隔离剪贴板复制断言原始输入；日文新标签切到韩文后
  显示正确样例；表格备注、列宽在阿拉伯文切换后保留；用户名切到英文不重置。
  实际将文本拖入接收区，再切换德/日语言，接收说明更新且 payload 不变。
- OpenGL 请求配置 2x 在同一桌面复验中文/英文默认样例和实际输入/选区保留，
  复制断言 `scale USER`。GLX/Mesa 23.2.1 llvmpipe 非硬件加速；应用 2x 不是
  真实 HiDPI，配置值不作为每帧实际后端的强制断言。

日志、截图和构建产物在 `/tmp/goui-demo-localization.41MRK9`；旧版复现目录
在 `/tmp/goui-demo-baseline.nYTouo`。临时例程与独立桌面退出后清理进程，保留
证据文件。本轮未执行 Windows/macOS 原生桌面、Direct2D、跨应用文件拖放、
真实 IME/混合 DPI 或母语校对；不承诺 RTL 整窗镜像。Switch 已按用户要求单独
提交；本轮 demo 代码、回归测试和对应设计说明也按用户确认独立提交。

## 2026-10-13 SkipTaskbar 与 BorderBox 窗口示例

在“9.窗口与检查”增加两个入口，不改变主窗口的 Integrated/背景样式：

- “不占任务栏窗口”：创建 Native + SkipTaskbar 的普通窗口，不使用 Popup、
  置顶或禁止激活策略。说明 Windows Alt+Tab、X11 WM 请求与 macOS 应用 Dock
  的差异；保留原生拖动/缩放、窗口内输入和关闭。
- “BorderBox 自绘窗口”：创建 None + Transparent，选择此窗口独立的
  `demo-transparent-window` 背景样式，以 BorderBox 为实际根，显式使用 modern.Card
  的背景/圆角/描边。BorderBox 默认无外观规则不变，不修改 Basic/Modern 或 GUI。
  HeaderBar 在安全内区域负责移动；6 DIP 四边/四角由 BorderBox 和既有 Chrome
  机制缩放，不能再包入旧透明例的 16 DIP 外层容器。

继续复用 openWindow 的创建错误反馈、显示前定位、语言信号和销毁清理。窗口
创建选项与内容组成分开：前者直接使用 gui.WindowOptions，后者使用例程私有
auxWindowContent，不引入窗口管理器、持久状态表或 UI 特例。原三种装饰例仍保持
440x300 初始尺寸，新例使用 500x380 创建偏好，并显式设置 280x220 最小窗口尺寸；
不把长说明在无界测量时的自然宽度当作窗口最小宽度。两者的文本输入归各自 GUI Widget 所有，
应用声明重建与语言更新只改固定标题/说明/按钮，不改用户输入。

长说明按词换行，正文置于垂直 ScrollView，HeaderBar 不随正文滚动；此规则也
适用于原辅助窗，缩窄或切换长文案语言后仍可滚动到关闭按钮。所有新增固定文本
按现有八语言 resource 方式维护，不改主窗口共享 window 背景或隐式继承样式。

验证：无桌面测试检查真实入口选项、根类型/Card/default resize width、内容安全
布局/Snapshot 与旧透明例的外层保留；现有资源测试自动检查八种语言完整性。
真实桌面按 example/demo/README 第 9 项检查任务栏/激活、输入、拖动/缩放、
主题/语言更新、用户文本保留、多个透明窗口逐个关闭与主背景隔离。
本机验收完成（2026-10-13，Go 1.25.8，Linux amd64）：

- `env -u DISPLAY go test ./example/demo/... -count=1` 与 demo 的 race 测试通过；
  `go vet ./example/demo/...`、`git diff --check` 通过。
- 最终在独立本机 X11 显示上执行 `go test ./... -count=1 -json`：2481 测试/子测试
  通过、15 既有跳过、0 失败。保留原有测试的窗口副作用，不算无桌面验收。
- Windows amd64 / macOS arm64 的 `CGO_ENABLED=0 go build ./example/demo` 通过，
  产物输出到仓库外；这只证明目标编译，不代表远端桌面运行。
- 本机 Xvnc 1.12 / Xfwm4 合成环境，Software 和 OpenGL 配置 1x：两个入口可打开，
  500x380 创建尺寸及 280x220 原生最小尺寸提示正确；BorderBox 的 Card 表面、
  透明圆角、安全留白和主窗口背景隔离可见。SkipTaskbar 原生属性包含
  `_NET_WM_STATE_SKIP_TASKBAR`，仍能激活和接收输入。
- OpenGL 下实际拖动原生标题栏和 BorderBox HeaderBar、收窄右边框及降低底边框，
  尺寸从 500x380 变为 352x282；滚动能到达关闭按钮，HeaderBar 保持原位。
  浅色切暗色、中文切英文后，两个辅助窗说明和标题更新；复制输入框内容分别
  断言为 `keep-123` / `frame-456`，没有被声明重建覆盖。逐个关闭后主背景不变，
  再切德文无销毁访问异常，关闭主窗口退出码 0。

隔离桌面未启动任务栏面板，因此本轮仅验证 X11 请求属性，未把任务栏实际呈现
算作通过。OpenGL 为 Mesa 23.2.1 llvmpipe；未覆盖硬件 GPU、真实 HiDPI、真实 IME
或所有语言的桌面布局。第一次隔离会话沿用了无法连接的 fcitx 配置，模拟逐键输入
未产生文字；粘贴正常，随后以 `XMODIFIERS=@im=none GTK_IM_MODULE=simple` 重启后
逐键输入及复制断言通过，不据此改动输入框架。

按用户要求未执行远端桌面验收。日志、截图和构建产物保留于
`/tmp/goui-demo-windows.oUWHYj`；已退出例程并清理自建桌面进程。本轮 demo 代码、
测试及对应设计说明独立提交，未推送。
