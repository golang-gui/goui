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
