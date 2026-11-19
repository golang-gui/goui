# 应用反馈缺口修复计划

本轮来自 SourceView、ApiBox、MarkDoc 的应用反馈核查。四项修复分别建立分支，
共同基线为本文档提交；不自动合并回 main。应用仓库只接入确有需要的公开契约，
不扩大为应用重构。现有未跟踪文件不在本轮范围内。

## 分支与交付

| 项目 | 修复分支 | 交付提交 |
| --- | --- | --- |
| 显式滚动覆盖旧 Reveal | `fix/scroll-intent-order` | `5871f608` 实现；`50d954cd` 窗口用例 |
| 聚焦控件输入法重绑定 | `fix/ime-context-refresh` | `462096b3`；SourceView 配套 `7695230` / `d017aaa` |
| 分隔条与滚动条输入冲突 | `fix/split-scrollbar-input` | `8233578a` 实现；`b1a699cb` 窗口用例 |
| Software 图片缩放采样 | `fix/software-image-sampling` | `d4ca809d` 实现；`3661ec4b` 窗口用例 |

每项的实现、相关逻辑回归一起提交；窗口用例和设计文档按实际独立性拆分。
本文末尾统一记录分支交付情况。生产代码保留在各自分支，应用集成按依赖顺序验证。

## 1. 显式滚动覆盖旧 Reveal

**问题与复现**：布局期间请求显示远处矩形，再显式设置偏移到原位，布局结束后旧请求
仍把视口拉回远处。公共接口回归中，期望 Y=0，实际 Y=601。

**契约**：有效的 SetScrollX/Y 是新的显式位置意图，即使位置相同或被边界钳制，
也取消更早的整个矩形 Reveal。后来的 Reveal 仍然有效。无效数值、禁用轴不取消导航。
布局引起的偏移钳制及内容重测不属于显式滚动，不取消待处理 Reveal。

**修改**：`gui/scroll_view.go` 使用私有请求代次隔离延后及重入请求；刷新内容尺寸等
外部调用后检查请求是否过期、内容是否替换或销毁。更新滚动设计文档。
不增加公共滚动模型，不修改自动分配和外部滚动条路线。

**验证**：扩展 `gui/scroll_view_test.go`，覆盖双轴、同值、边界、无效输入、
布局延后、新旧 Reveal 顺序和回调重入；已有 `tests/window/gui/scroll_layout`
验证滚轮、滚动条与内容导航交替。检查原有焦点可见性、末端及虚拟内容回归。

**参考**：Flutter 3.38.1 的 [jumpTo 实现](https://github.com/flutter/flutter/blob/3.38.1/packages/flutter/lib/src/widgets/scroll_position_with_single_context.dart#L197)
先 goIdle 再判断偏移是否相等。采用“新位置请求终止旧活动”的顺序，不引入其滚动活动体系。

## 2. 聚焦控件输入法重绑定

**问题**：第三方文本控件在保持焦点时改变只读状态，无法请求 GUI 重新查询 IMContext。
直接 CommitText 成功不代表原生输入法绑定有效。

**API 与契约**：增加 `gui.RefreshInputMethod(widget Widget)`，在 GUI 线程同步重新查询
当前已挂载且聚焦的控件。不移动焦点、不激活窗口；nil、卸载、销毁、非当前焦点及不支持
输入法的宿主均无操作。同一底层上下文重复刷新不取消组合输入；切换上下文或禁用输入时
结束旧绑定。外部查询、Reset 回调引起的焦点切换、销毁或再次刷新不得覆盖新绑定。

**修改**：`gui/im_context.go`、`gui/surface.go`、`gui/text_editor.go`，内部编辑器复用
公开刷新路径；保持宿主边界，不借此扩展 Popover 输入法能力。更新 `doc/DesignIME.md`。
SourceView 在同名配套分支中由 SetReadOnly 调用该入口；不提交机器路径或工作区配置。
新入口未发布前，SourceView 明确记录所需 GOUI 分支，不声称当前发布依赖已支持。

**验证**：扩展现有 `gui/im_context_test.go` 的宿主替身，验证绑定开关、相同上下文幂等、
上下文替换、不可比较包装值、焦点和 Reset 回调重入、卸载及销毁。
窗口验证通过正常平台输入检查聚焦只读→可编辑、反向切换及焦点保持；SourceView 同步验证
自身状态和两仓联调。原生输入法平台未实测的部分单独列出。

**参考**：Qt 6.8.3 的 [QInputMethod::update](https://github.com/qt/qtbase/blob/v6.8.3/src/gui/kernel/qinputmethod.cpp#L279)
在 ImEnabled 更新时重新查询焦点对象。采用显式刷新入口，不复制 Qt 查询标志体系。

## 3. 分隔条与滚动条输入冲突

**问题与复现**：SplitView 实际分隔只占 1 DIP，顶层 handle 却覆盖相邻面板各 4 DIP。
相邻滚动条内部点命中 separator，滚动条无法正常接收输入。

**契约**：保持原有面板占位和分隔线外观；实际分隔线优先拖动，扩展区域作为父级拖动
候选，与子控件通过现有手势仲裁竞争。滚动条、文本拖选等子级拖动优先；普通点击不得
在按下时被父级取消。空白处仍支持扩展区域拖动，不通过全局类型优先级修补 Pick。

**修改**：集中在 `widgets/split_layout.go`、`split_handle.go`、`split_view.go`。
handle 的真实边界与占位一致；SplitView 的 bubble 候选仅在扩展区域按下后、移动达到阈值
时认领。复用分隔条调整、取消及键盘逻辑，生命周期变化清理候选；语义边界反映真实控件。
更新 `doc/DesignSplitView.md`。

**验证**：扩展 `widgets/split_view_test.go`，通过正常事件分发验证相邻滚动条、按钮点击、
子级拖动、空白扩展拖动、双向布局、嵌套、取消回滚、键盘和语义。
扩展 `tests/window/widgets/split_view` 完成代表性窗口操作，不修改滚动条默认尺寸。

**参考**：Qt 6.8.3 [QSplitterHandle](https://github.com/qt/qtbase/blob/v6.8.3/src/widgets/widgets/qsplitter.cpp#L204)
同样存在 0/1 像素 handle 扩展抓取区，但不能据此断言其保护相邻滚动条。
Flutter 3.38.1 [手势竞争说明](https://github.com/flutter/flutter/blob/3.38.1/packages/flutter/lib/src/widgets/gesture_detector.dart#L163)
说明子级竞争；采用 GOUI 既有按深度仲裁，不建设新的手势系统。

## 4. Software 图片缩放采样

**问题与复现**：整数对齐快路径缩放采用最近邻，而带小数偏移走双线性。
2×1 黑白图放大到 32×8，中心像素在整数位置为白色，小数位置却为灰色。

**契约**：缩放时的双线性采样不因整数对齐改变；对齐的 1:1 绘制保留精确拷贝。
沿用预乘 BGRA、像素中心映射、边缘钳制、裁剪和镜像语义，不增加采样模式 API。

**修改**：`platform/graphics/software/painter.go` 的轴对齐缩放路径与通用路径统一映射。
检查 OpenGL、Direct2D 的实际采样契约；更新相关绘制文档，不改主题或控件补偿。

**验证**：扩展现有 painter 测试，按独立公式断言黑白中间值、透明预乘颜色、边缘、
镜像、分数位置、裁剪、变换及 1x/1.25x/1.5x/2x。代表性图片窗口分别检查
Software/OpenGL；Direct2D 未运行时只记录源码核对和编译，不算原生通过。

**参考**：Qt 6.8.3 [SmoothPixmapTransform 说明](https://github.com/qt/qtbase/blob/v6.8.3/src/gui/painting/qpainter.cpp#L1391)
区分平滑与最近邻。此处修复 GOUI 现有路径不一致，不推断 Qt 默认也采用双线性。

## 暂缓：macOS 文件过滤

`platform/darwin/cocoa/dialog_darwin.go` 将 FileFilter.Patterns 直接传给 allowedFileTypes。
Apple 的 [allowedFileTypes 契约](https://developer.apple.com/documentation/appkit/nssavepanel/allowedfiletypes)
接受扩展名或 UTI，不接受通配表达式。后续需明确 `*`、多段后缀及不可表达模式的策略，
再在 macOS 打开／保存面板验证。本轮只记录，不修改实现。

## 验收与状态

各分支交付前执行 `go test ./...`、相关 race 和受影响目标交叉编译。
共享逻辑优先无桌面回归，原生输入和后端风险追加针对性窗口验证；
通过、失败、跳过、未执行分别记录。临时日志和诊断脚本保留在仓库外。

### 交付结果（2026-11-19）

四项均已按上述契约实现并独立提交；GOUI main 仅维护总计划，生产修复留在各分支。
SourceView 的同名配套分支负责调用新入口及原生输入验收，不改变应用业务代码。

| 分支 | `go test ./... -count=1 -json` 测试／子测试通过数 | 失败 | 条件跳过 |
| --- | ---: | ---: | ---: |
| `fix/scroll-intent-order` | 6157 | 0 | 0 |
| `fix/ime-context-refresh` | 6149 | 0 | 0 |
| `fix/split-scrollbar-input` | 6149 | 0 | 0 |
| `fix/software-image-sampling` | 6160 | 0 | 0 |

每次有 81 个测试包通过；另有 155 个无测试文件的包，不算条件跳过。
全仓运行环境为 Linux amd64 / X11 / Xfwm4 的独立桌面会话。
两项既有 X11 包内测试创建私有窗口及 selection，不改系统主题；窗口验收只修改本例内存内容。

- 无显示会话的 race：滚动及输入法分支 `go test -race ./gui -count=1`，
  分隔条分支 `go test -race ./widgets -count=1`（补充嵌套／Esc 后再次运行 TestSplitView），
  图片分支 `go test -race ./platform/graphics/software -count=1`，均通过。
- 滚动窗口：Software 1x，通过布局中的新旧意图断言；真实滚轮移动表单、双轴滑块到末端，
  然后再次运行意图回归，偏移及实际槽位检查通过。未把用例注释中的其余完整矩阵计为通过。
- 输入法：SourceView 在 Software 1x 上先以只读状态取得焦点，再保持焦点恢复编辑，
  普通 XIM 键入恢复。相同窗口用例连接未修改的 SourceView 时重现原生键入无效；配套分支接受文字。
  同时验证可编辑→只读拒绝输入→恢复键入。OpenGL 1x 的既有自动编辑、焦点、重挂载、
  导航及语义检查通过；这些自动 CommitText 检查不算原生组合输入验证。
- 分隔条窗口：Software 1x，分隔线旁滚动条的末侧 4 DIP 内真实拖动使偏移从 0 到 774，
  分栏尺寸不变；空白扩展区域拖动后 Esc 恢复偏好，Snapshot 几何检查通过。
  双向、子级拖动、按钮点击、嵌套优先级及键盘由正常事件分发的逻辑断言补充。
- 图片窗口：Software / OpenGL（llvmpipe）分别在原生 1x、2x 提交帧并通过断言；
  离屏采样逐项检查 1x / 1.25x / 1.5x / 2x 的整数和小数位置。
  Software 1x 实际窗口的两条渐变均平滑。预乘透明、镜像、shear、裁剪及精确拷贝由独立像素预期验证。
- Windows amd64 / macOS arm64：受影响的 GUI、widgets、绘图包及窗口入口交叉构建通过；
  Direct2D 仅适用于 Windows。SourceView 配套编辑器及 ui 同样通过两目标构建。

**独立依赖限制**：SourceView 的临时双仓工作区 `go test ./... -count=1` 及
`go test -race ./sourceview` 通过；未提交工作区或 replace。
`GOWORK=off go test ./...` 失败：声明的 GOUI v0.8.0 缺少 RefreshInputMethod，
并已有 SharedPopover、NewSharedPopover、ErrPopoverSuperseded 的依赖缺口。
发布或独立消费前需更新到实际包含这些 API 的 GOUI 版本，不能把本地联调算作独立发布通过。

**未执行**：Windows / macOS 原生输入及窗口操作、Direct2D 桌面采样、真实 CJK 服务的组合／候选窗回归。
本轮没有改动三平台 IME 或 Direct2D/OpenGL 的实现，源码核对与交叉构建不代替这些验收。
macOS 文件过滤继续暂缓；没有顺带修复该项。
