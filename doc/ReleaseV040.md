# GOUI v0.4.0 发布说明

本版本同步更新 Linux amd64/X11、Windows amd64、macOS arm64 Demo，版本为
0.4.0、build 1。最终构建、签名公证、验证环境及下载校验结果以 Release 附件
`BUILD-INFO.md` 和本仓库 Plan 的本轮记录为准；本文不将待验收项视为通过。

> 下文接口示例记录 v0.4.0；main 分支的按钮分组已改为局部容器／自动作用域，
> 当前用法见 [ButtonGroup 设计](DesignButtonGroup.md)。

## 主要变化

- Widget 增加普通文字 Tooltip，命令式使用 `SetTooltip`，声明式使用
  `.Tooltip(text)`。鼠标停留或键盘导航可触发，默认延迟 700 ms；禁用控件也能
  提供说明。Modern 亮暗样式采用无阴影提示表面。
- 长段落采用有界原生排版窗口，保留软折行、字节位置和双向文字上下文。几何
  先在局部坐标计算，再与高精度段落原点组合；文本读取、词边界查询、滚动范围
  和滚动条端点同步改进。Demo 增加大文本样本及切换入口。
- 声明式组合拥有独立节点与 State，不再需要无用途的包装 Widget。
  `ComposeBase` / `ComposeView` 负责构建内容，`ObjectView` 可向后代提供显式对象，
  `FindAncestor[T]` 解析对象作用域；跨窗口移交后可刷新依赖祖先的连接。
- 新增 `MenuButtonGroup`：点击打开菜单后，鼠标经过同组按钮即可切换；左右键
  切换成员，上下键操作菜单，Esc 关闭，Tab 关闭并继续焦点导航。普通命令按钮
  可与菜单按钮混排。Basic / Modern 提供展开状态外观，Demo 展示文件／编辑／视图组。
- Popover 自动关闭策略独立于模态设置；菜单按钮使用非模态菜单，保留所属窗口
  的逻辑焦点。修正键盘切换后静止指针收到重复原生通知时菜单跳回的问题。
- 修正 CoreText 长段落默认省略 Unicode 分析、导出坐标提前丢失精度的问题，
  以及相邻排版页重复计算首行间距的问题。分页拼接采用实际基线，保留混合字体
  的首行差异；不通过放宽回归断言掩盖接缝误差。

## 升级影响

### 自定义声明式 View

`View` 只表示声明身份。构建内容的组合实现 `ComposeView.Build(ctx)`；直接
挂载 Widget 的声明实现 `WidgetView` 的 Mount / Update / Unmount，不再保留
无用途的 Build。两者不能在同一声明中混用。

组合类型嵌入 `ui.ComposeBase[T]` 并初始化 Self；Widget 声明继续嵌入
`ui.ViewBase[T]`。需要对象作用域的组合实现 `Object(ctx)` 和 `Release(ctx)`，
后代通过显式 ID 查找最近祖先。完整例子见 [自定义 Widget 与 View](CustomWidgetView.md)。

### CheckButtonGroup

命令式 `widgets.CheckGroup` / `NewCheckGroup` 改名为
`widgets.CheckButtonGroup` / `NewCheckButtonGroup`，不保留旧名称别名。
声明式分组必须有明确的祖先声明，成员的 Group 引用其 ID：

```go
wui.CheckButtonGroup("options", ui.HBox(
    wui.CheckButton("A").Group("options"),
    wui.CheckButton("B").Group("options"),
))
```

这里 wui 表示 `github.com/golang-gui/goui/widgets/ui`。不同分支的同名组相互
独立；不再通过共享字符串隐式创建全局组。

### Popover.AutoDismiss

`SetModal` 仅决定模态输入，不再隐式决定外部点击、Esc 和失焦时关闭。
需要该策略的自定义 Popover 显式调用 `SetAutoDismiss(true)`；默认值为 false。
内置菜单和 DropDown 已配置自己的策略，普通使用者不需额外设置。

### 长文本扩展

`gui.ParagraphLayout` / `ParagraphGeometry` 提供按需排版与局部几何；使用方
必须遵守 GUI 线程、借用快照和显式 Destroy 契约。`HeightIndex.Top64` 及
`Projection.Slice` 支持大坐标和局部读取。它们不意味着任意大文件都能立即精确
测量：尚未访问的区域仍使用估算，原生单次排版仍可能占用较长时间。

## 范围与限制

MenuButtonGroup 不是原生系统菜单栏；子菜单、助记键仍未实现。SourceView 是
独立扩展，不包含在本模块或 Demo 中，使用新接口需更新其 GOUI 依赖。

Linux 正式附件仍为 amd64/X11，不扩展到尚未验收的 arm64 或其他窗口系统。
Windows EXE 不宣称经过 Authenticode 签名。macOS Demo 的 Developer ID 签名、
公证票据和 Gatekeeper 结果须以本轮最终 ZIP 的验证为准。
