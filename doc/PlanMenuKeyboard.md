# 单层菜单键盘验收（2026-10-16）

实现提交 `ffc59f3`；基于 `fd9f84e`，不增加公开 API、原生焦点宿主或子菜单。
设计见 [DesignMenu](DesignMenu.md#4-单层键盘导航)。

## 本机

Ubuntu 22.04.5 amd64、Go 1.25.8、X11/Xtigervnc + Xfwm4。
在源码树外的提交快照覆盖本次菜单文件，独立桌面中执行 `go test ./... -count=1` 通过；
既有测试生成的图片留在临时快照，没有写回工作树或删除用户文件。
`go test ./gui -run 'TestMenu|TestPopoverMenu' -count=1` 通过，覆盖正常 Window.DispatchEvent、
禁用/隐藏/分隔项跳过、循环、Home/End、Enter/Space、动态状态和虚拟行 Selected 清理。

SourceView 的 `go run ./tests/window/experience` 在 Software 1x/2x、OpenGL 1x 通过：
真实菜单表面中等待行实现，再以 Window.DispatchEvent 发送 End、Enter，断言活动项
快照、Select All 结果及关闭；指针删除和撤销也通过。绘图后端及实际缩放由伴随的
`tests/window/editor -auto` 记录。测试使用独立显示，不改日常桌面焦点或剪贴板。

## 远端

- Windows 10 19045 amd64 / Go 1.24.0 / SSH Session 0：
  `go test ./gui -run 'TestMenu|TestPopoverMenu' -count=1` 通过。
- macOS 15.7.7 arm64 / Go 1.24.9：同一命令通过。
- 两端 SourceView `go test ./... -count=1` 通过（包含原生 DirectWrite/CoreText 排版）。
  上述 GUI 包测试是逻辑/模拟宿主验证，不是两端原生菜单窗口验收，显示缩放不适用。
- 未执行 Windows/macOS 新菜单桌面交互、物理快捷键、跨屏和长菜单原生滚动验收。

远端使用任务临时目录快照，没有更新已有检出、全局 Go 配置、系统设置或剪贴板。
未推送、发布版本；用户原有 Linux arm64 文档改动和未跟踪文件留在原处。

## 鼠标与键盘双高亮回归（2026-10-16）

修正提交 `e5d9321`，基于 `e345c5c`。旧实现将行的物理 hovered 与 keyboardSelected
分别用于绘制；行的 Motion 仍在 Target 阶段，鼠标直接命中内部 Label 时不会更新活动项。
从 First 的留白跳到 Second 的文字区域，能同时绘制两行高亮，而 Selected 仍只有 First。
问题不要求先使用键盘，普通鼠标移动也会设置原来的 keyboardSelected。

现在鼠标与键盘共享唯一活动项，行仅保存其选中投影；Contains-enter 与 Bubble Motion
分别处理原生 Enter 和文字子控件上的移动。指针按下也接管选择，按压样式仅作用于当前活动项。
禁用项和分隔线不保留指针选择；模型变更、关闭与虚拟行重用沿用既有失效边界。
没有改动公共 API、通用事件分发器、平台实现或 SourceView 源码。

### 复现与验证

- `gui/menu_pointer_test.go` 经 `Window.DispatchEvent` 验证 100 次跨 Label 往返、文字/留白
  切换、静止指针上的键盘切换及再移动、原生 PointerLeave/PointerEnter、禁用项/分隔线、
  鼠标按住期间的键盘导航、静止指针按下接管与 Enter 激活。独立比较绘制样式和 Selected，
  不只检查 Snapshot。修改前快照中的回归失败明确报告 painted=2、selected=1。
- 新增显式窗口程序 `go run ./tests/window/gui/menu_pointer [-auto]`，环境、步骤和副作用见入口。
  `-auto` 在真实 Popover 中执行输入，逐步用 `RenderWidget(1)` 采样每行左侧 2 DIP、
  垂直中点的背景，避开文字和圆角边缘；转换为非预乘 NRGBA 后，精确比较不透明 sRGB
  普通色 #f0f0f0 与高亮色 #2468ac，容差为 0。同时核验 Selected、动作结果与重开/关闭。
  不用字体金图，不为执行输入导出内部状态。
- 用完全相同的窗口程序对修改前 `e345c5c` 构建：移动到 Second 后，First 的采样值仍为
  `{36 104 172 255}` 且 Selected=true，按预期失败。新实现通过相同像素和语义断言。
- Ubuntu 22.04.5 amd64 / Go 1.25.8 / X11（独立 Xtigervnc + Xfwm4，关闭 compositor）/
  Pango 1.50.6：Software 1x/2x、OpenGL 1x 的自动窗口回归均通过。
  painter 由实际图像资源类型记录，缩放读取 Painter.PixelScale；并抓取最终桌面 RGB 截图，
  高亮专用色的像素仅形成一条连续纵向区域。实际查看截图，确认只有 Second 高亮。
  RenderWidget 的离屏采样与桌面截图分别验证，没有将离屏结果当作桌面合成证明。
- 最终代码在源码树外的干净 HEAD 快照加本次文件中，独立显示下执行
  `go test ./... -count=1` 通过；测试生成的图片留在快照，不修改工作副本既有产物。
- `go test -race ./gui -run 'TestMenu|TestPopoverMenu' -count=1`、`git diff --check` 通过。
- `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go test ./gui -c -o <临时目录>/gui-windows.exe`
  及对应的 `GOOS=darwin GOARCH=arm64` GUI 测试二进制交叉编译通过。
  本次未执行 Windows/macOS 原生包测试或菜单窗口；交叉编译不记作原生验收。
- SourceView `37ebc3a` 使用本地工作区联调：`go test ./... -count=1` 与独立 Software 1x
  `go run ./tests/window/experience` 通过，包含菜单键盘/指针激活、关闭及编辑历史回归。

本轮未操作日常桌面、真实剪贴板或系统设置，没有推送。原有 `doc/Plan.md`、`doc/Roadmap.md`
和未跟踪文件保持原状。窗口程序及本验收记录单独提交；更多平台桌面、物理输入和混合 DPI 仍待验证。
