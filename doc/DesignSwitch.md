# Switch

## 范围与参考

`widgets.Switch` 是独立两态开关，接受一个可选 Child；状态使用 bool，不引入
Mixed、互斥组、异步生效状态或公开滑块位置。GUI 内核、Painter 和平台接口不变。

参考 GTK 4.20.3 [gtkswitch.c](https://github.com/GNOME/gtk/blob/4.20.3/gtk/gtkswitch.c)
的点击／拖动竞争与短过渡，以及 Qt 6.8.3
[qquickswitch.cpp](https://github.com/qt/qtdeclarative/blob/v6.8.3/src/quicktemplates/qquickswitch.cpp)
的 checked 与 position 分离。GTK 的 active/state 双状态、帧时钟和 Qt 模板／
继承体系不适用于此切片；异步业务由调用方管理，短过渡复用纯 animation 包和 Root 帧信号。
以上为对应版本源码行为参考，不代表 GOUI 已完成原生桌面验收。

## 接口与状态

```go
b := widgets.NewSwitch()
b.SetChild(gui.NewLabel("自动保存"))
b.SetChecked(true)
b.ConnectChange(func(checked bool) { /* 用户操作 */ })
```

提供 Checked/SetChecked、Child/SetChild、Enabled/SetEnabled、Padding/SetPadding、
Animated/SetAnimated 与 ConnectChange(func(bool))。默认关闭、启用、可聚焦、
动画开启，Padding 为 6 DIP。setter 静默；实际用户状态变化后立即发出 Change，
不等待动画。回调重入／销毁使后续过期通知失效。同值 setter 不打断手势或动画。

有效抬起才点击切换；移出抬起不切换。交互 Child 获胜时父项不切换。轨道内
按下后，水平移动超过 4 DIP 并占主导才参与拖动；纵向移动让出手势。获胜后
滑块按相对位移移动、不跳到指针中心，持续接收边界外移动。仅松手提交状态；
滑块位置大于／小于 0.5 分别为开／关，恰好中点保持原值。拖动不产生 Click。
Esc、手势取消、禁用、隐藏或卸载恢复当前状态对应端点。改变 Checked 的外部
setter 取消当前操作。同值声明更新保持当前操作。

Space/Enter 激活，Repeat 消费但不重复切换；Tab 沿用已有焦点导航。
Enabled 复用 Widget 的自身设置；gui.IsEnabled 查询祖先限制后的结果，限制
整棵 Child 子树，保留自身设置及 Focusable。通过既有 Cancel 恢复拖动外观，
短过渡沿既有动画路径收敛，不增加通用状态监听；恢复不自动抢回焦点。
见 [统一禁用契约](DesignWidgetEnabled.md)。

## 几何、样式与快照

轨道 36×20 DIP，滑块直径 16 DIP、端点间隙 2 DIP，Child 间隔 8 DIP。
无内容自然大小 48×32 DIP。Child 在右侧垂直居中；测量／安排使用相同约束，
上报 Child 的真实 baseline。轨道不随多余空间拉伸；硬约束不足时等比例缩小。
Padding 属于控件。可选 Child 独立解析自身样式。

样式名 `switch`；Parts 为 track、track-checked、thumb、thumb-checked、focus。
默认关闭轨道为中性灰色、弱轮廓，开启轨道为主题色，滑块为无描边的白色圆形；
Modern 校正过亮主题色以确保滑块可辨。不增加阴影。轨道为胶囊、滑块保持圆形。
Hover/Pressed 只调整轨道配色、不改变尺寸／边框宽度。Modern 的开启轨道仅填充，
不叠加同色描边，关闭轨道保留 1 DIP 弱轮廓；Basic 兜底也避免开启轨道重复描边。
Disabled 弱化轨道（开启时仅保留少量主题色）及滑块，保留可辨对比和左右位置区别。FocusVisible 使用
独立 2 DIP 柔和主题色边框；鼠标焦点没有额外装饰。文字使用 switch-text。
完整兜底属于 theme/basic 并由 GUI 自动使用，Modern 提供完整自身规则。

Snapshot 使用 widgets.RoleSwitch（"switch"）；Attributes["goui.switch"] 保存
SwitchInfo{Checked bool} 的值快照。保留 Child 子树、边界、启用和焦点状态，
启用时提供 Click。动画／拖动中上报已提交状态，不暴露显示进度，不扩展 gui.Info。

## 动画和声明协调

120ms ease-out 过渡，按单调经过时间计算位置；快速切换从当前显示位置续接。
只有短过渡存在时连接所属 Root 的 ConnectFrame，用 animation.Tween 计算位置，
通过 RequestPaint 请求下一帧；完成后断开连接，不创建控件专属 Timer，不承诺原生 vsync。
拖动直接跟随指针，松手滑向端点；关闭 Animated 只禁止自动过渡。
未挂载、未得到可绘制布局、有效不可见、禁用或请求绘制失败时直接显示端点。
完成／卸载／销毁后断开帧连接；回调检查归属和有效期。见 [帧驱动契约](DesignAnimation.md)。

```go
wui.Switch("自动保存").Checked(autoSave).OnChange(func(checked bool) {
    autoSave = checked
    app.RequestUpdate()
})
```

widgets/ui 提供 Text、Content/Child、Checked、Enabled、Padding、Animated 和
OnChange 链式声明，复用 ui.Bin。重建保留 Widget 身份与一个 Change 连接，
回调使用最新闭包；卸载断开声明连接，不改变窗口的最终资源所有权。

## 实施与验收

实现位于 widgets/switch.go、widgets/ui/switch.go、theme/basic/switch.go、
theme/modern/switch.go 与 widgets/info.go，相关测试集中于各包 switch_test.go。
窗口例程放 tests/window/widgets/switch，入口中文说明环境、操作及逐项预期。

逻辑覆盖 setter、输入取消／重入／子控件竞争、拖动终态／中点／纵向让出、
布局 baseline 和紧约束、语义 JSON、动画续接／生命周期、声明重建。像素覆盖
预乘合成、1x/2x、clip/Transform、胶囊轨道／圆形滑块及亮暗样式。
运行全仓测试、相关检查和 Windows amd64/macOS arm64 编译；本机窗口验证
Software/OpenGL、主题、点击／拖动／键盘／重建与关闭。跨平台编译只记为编译。

## 2026-10-06 实现与验证

本节为当时 Timer／widgets/style 版本的历史记录；当前驱动和兜底归属以以上章节、
[DesignAnimation](DesignAnimation.md) 和 [DesignThemeFallback](DesignThemeFallback.md) 为准。

- 已完成 widgets.Switch、widgets/ui.Switch 链式绑定、显式兜底与 Modern 样式、
  RoleSwitch／goui.switch 快照和中文窗口例程；GUI、Painter、平台无改动。
- `go test ./...` 通过：49 个有测试包、1973 个测试／子测试通过，14 个既有
  人工／opt-in 跳过，无新增 skip。逻辑验证沿正常 EventDispatcher 路径，
  覆盖中点保持原值、越界捕获、同值更新不取消、取消／子控件竞争／回调重入。
  软件像素覆盖预乘 sRGB、透明留白、1x/2x、窗口 clip、平移及 90° 旋转。
- Switch 相关 widgets、widgets/ui、widgets/style、theme/modern race 测试与
  上述包及窗口例程 vet 通过；Windows amd64／macOS arm64 的
  `CGO_ENABLED=0 go build ./...` 通过，仅证明编译。
- 本机隔离 TigerVNC/X11 + Xfwm4、无合成器，Software/OpenGL 各 1x/2x，
  以及 Software 1x 显式兜底：每组 28 次状态断言通过。检查点击／拖动／中点／
  Esc／越界捕获、子按钮、禁用、动画开关、静默 setter、重建、主题及键盘；
  动画结束 ActiveTimers 为零，关闭后控件与计时器清理通过。
- OpenGL 2x 的 -destroy-on-change 通过：状态提交后立即关闭窗口，后续连接
  不执行，无活跃 Timer。包内的惰性 Popover 销毁测试只证明卸载后的通知失效，
  控件最终销毁由窗口例程验证，不把 Popover 卸载当作 Widget 销毁。
- Software 1x 实际连续截图在开启／关闭两方向均捕获中间位置，位置单调趋向
  端点；显示期间使用真实 GUI Timer，结束后停止。此采样验证动画存在与方向，
  不声称帧同步或性能提升。OpenGL 为 Mesa 23.2.1 llvmpipe 软件驱动。
- 未执行：Windows/macOS 原生桌面、Direct2D、硬件 GPU、混合 DPI。
  测试桌面 Integrated 请求回退 Native，不作为窗口装饰验收；Linux arm64
  仍受 Plan 已记录的既有 libc 绑定编译阻塞影响，本切片未修改相关绑定。

日志、脚本、亮暗截图与动画采样位于 `/tmp/goui-switch.6rDMr3`；未提交 git。

### 外观精简

Modern 与显式兜底已同步改为无描边白色滑块、弱轮廓中性关闭轨道；开启轨道
边框随填充同步，禁用态降低饱和度但保留滑块对比。不改变几何、动画、输入、
Snapshot 或 UI 协调。只调整主题规则，不为此修改控件和绘图后端。

样式回归覆盖亮／暗、黑／白／红／黄色主题及 Hovered、Pressed、Disabled
组合：滑块零描边、启用时始终白色、开启轨道边框等于填充、关闭轮廓对比不超过
1.35、禁用滑块与轨道对比至少 1.2。后两项仅为本主题的视觉回归约束，不是无障碍
标准承诺；开启且可用时仍保留至少 3:1 的对比要求。

全仓 1973 个测试／子测试通过，14 个既有跳过；相关 vet 和 Windows amd64／
macOS arm64 编译通过。本机 TigerVNC/X11 + Xfwm4、无合成器，Software/OpenGL
各 1x/2x 及 Software 1x 显式兜底的正常输入复验各 28 项通过，亮暗截图检查
无描边滑块和弱化轨道。
OpenGL 使用 Mesa llvmpipe；不代表硬件 GPU 或远端原生桌面验收。
本轮日志与截图位于 `/tmp/goui-switch-style.RW8aPZ`。

### Windows／macOS 远端验收

2026-10-06 使用 remote-shell 传输并校验 SHA-256、remote-computer-use 通过
真实鼠标／键盘输入验证 `tests/window/widgets/switch`；未使用 DevServer。
Windows 10 19045 amd64 的交互 Session 1／RDP，Direct2D 1x、2x；macOS
15.7.7 arm64 的 VNC 桌面，OpenGL 1x、2x。缩放通过 GOUI_PLAT_SCALE 请求，
未更改系统缩放。2x 窗口尺寸由桌面可用范围限制，测试区域均可见。

四组各 27 项状态断言通过：点击、按下后抬起切换、双向水平拖动、中点保持、
越界捕获、Esc 取消、Child 独立点击、禁用、静默 setter、动画开关、重复重建
身份保持、隐藏恢复、Tab／Shift+Tab、Space／Enter、亮暗及蓝／红主题切换。
动画稳定后 ActiveTimers 为零；四组关闭均输出 DESTROY PASS、退出码为 0。
截图检查白色无描边圆形滑块、关闭轨道弱轮廓、开启轨道悬停／按下同色边框、
弱化禁用配色和独立键盘焦点。未测帧率、精确动画时长、混合 DPI 或 Windows
OpenGL；远程截图不作为精确像素／抗锯齿金图。

本轮没有发现需要修复的 Switch 问题。本次窗口已关闭，创建的 Windows
计划任务与远程桌面连接已清理；保留测试日志／截图。证据与工具调用问题记录
位于 `/tmp/goui-switch-remote.1EGpWW`，工具问题不归为 GOUI 功能失败。

### 2026-10-09 Modern 开启轨道抗锯齿修正

用户在 OpenGL／Direct2D 的亮色蓝主题、1x 下观察到胶囊边缘偏硬。
代码确认开启轨道先填充，再沿同一外轮廓绘制 1 DIP 同色描边；边缘覆盖率
重复执行 SourceOver，覆盖率约为 `a` 的两次绘制会得到 `a+a*(1-a)`，而不是 `a`。
Software 离屏实测可复现：理论覆盖率约 59% 的圆弧像素，填充得到约 60%，
叠加同色描边后约为 84%。亮色背景与蓝色的反差会放大这个过渡问题。

本轮只将 `theme/modern` 的 track-checked 默认 BorderWidth 改为 0，正常、
悬停、按下和禁用状态均沿用单次填充。保留原有 BorderColor 值供显式覆盖，
关闭轨道仍为 1 DIP 弱轮廓、键盘焦点仍为独立 2 DIP 边框；尺寸、圆角、
滑块、全部配色、动画、输入与 Snapshot 不变。显式兜底未调整，不为主题视觉
策略修改通用 Painter，也不通过降低主题色对比度补偿边缘覆盖率错误。

- 样式回归保留原有配色／滑块对比度／焦点／字体覆盖，分别断言开启描边
  0 DIP、关闭描边 1 DIP、轨道圆角 10 DIP。修正前明确失败。
- 新增实际 Switch + Modern 的 Software 像素回归：亮暗、正常／悬停／按下／
  禁用、1x／1.25x／2x、整数／0.25 DIP 平移，共 48 组。状态经正常输入派发
  或公开 setter 建立；采样开启状态滑块不会遮挡的左半圆，用 128×128 子采样
  积分的精确圆覆盖率作为独立预期。比较透明背景上的预乘 sRGB RGBA8，
  允许 10 字节级误差以覆盖 26.6 坐标量化、三次曲线近似与字节舍入；不修改
  现有像素测试容差。旧实现 48 组均失败，修正后均通过。
- `DISPLAY=:89 XMODIFIERS=@im=none go test -json -count=1 ./...` 通过：
  2190 项测试／子测试通过、14 项既有跳过，无新增失败／跳过。Modern、widgets、
  widgets/ui、widgets/style 的相关测试、完整包 race 及相关 vet 通过。
  全仓仍含既有原生离屏、opt-in 窗口测试及向源码目录输出图片的测试；本轮未迁移。
- `CGO_ENABLED=0` 的 Windows amd64／macOS arm64 Switch 窗口程序及 Modern／
  widgets 测试目标、Linux arm64 Modern 测试目标交叉编译通过，不计作原生运行。
- 本机 Linux amd64、TigerVNC 1.12.0/X11/Xfwm4 合成桌面，Software 与
  OpenGL（Mesa 23.2.1 llvmpipe）各 1x／2x：真实鼠标／键盘验证亮暗、按下、
  禁用和独立 Tab 焦点；F6 状态／身份／快照断言、关闭 DESTROY PASS 均通过。
  2x 为应用缩放请求，不代表物理 HiDPI 或混合 DPI。没有使用 DevServer。
- 同后端、同状态的 1x 截图前后对照：左半圆 34 个部分覆盖像素相对精确面积
  的平均覆盖率误差，Software 亮色约 13.8%→0.8%、暗色约 13.9%→0.9%；
  OpenGL 亮色约 8.7%→6.9%、暗色约 8.7%→6.8%。关闭轨道与白色滑块核心
  像素逐字节保持一致。统计仅用于前后对照，不要求不同后端共享同一抗锯齿金图。

未执行本轮 Windows／macOS 原生桌面、Direct2D、硬件 OpenGL、分数缩放窗口
与物理 HiDPI 验收。此前远端通过的结果不代替本轮复验。测试窗口正常退出，
隔离桌面已停止；日志、前后截图与采样脚本位于 `/tmp/goui-switch-fix.5fbec5/`。
