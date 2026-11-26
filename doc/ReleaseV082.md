# GOUI v0.8.2 发布说明

状态：**发布准备中**。比较基线为 v0.8.1；源码 Demo 为 **0.8.2 / build 1**。
正式发布前，公开下载继续提供 [v0.8.1](https://github.com/golang-gui/goui/releases/tag/v0.8.1)。
本页描述当前源码能力，正式标签、附件与签名公证结果在 main 候选冻结后确认。

本版本补充菜单、命令、滚动关联和窗口／文件选择能力，并修复菜单动作、悬停与
绘制问题。当前仍处于 v0.x，升级前请检查下方 API 迁移。

## 主要变化

- **菜单栏与级联菜单：** gui.MenuBar／ui.MenuBar 支持 Auto 和 InWindow。
  Auto 在 macOS 使用系统菜单栏，在其他桌面使用窗口内菜单；原生呈现保留控件与
  命令生命周期，并退出窗口内布局和绘制。MenuItem.Submenu 声明递归菜单，支持
  鼠标和键盘导航、屏幕边缘翻转、重叠与跨层轨迹保护。
- **菜单交互修复：** 当前项高亮即时跟随指针，展开路径使用独立提示；离开整条
  菜单链清除活动高亮并保留展开路径。修正子菜单阴影处 Hover 与展开背景来回切换。
  选择动作在关闭菜单、释放模态输入后执行一次，常规菜单模型清理不会丢弃动作；
  重新显示、替换或销毁导致的过期动作继续失效。
- **可勾选命令：** Command 提供 Checkable／Checked 与对应通知。菜单和快捷键共享
  同一操作的可用性与实际状态；Activate 不隐式改变 Checked，由业务处理提交结果。
  声明式支持 Checkable、Checked 和 BindChecked，快照提供 menuitemcheckbox 角色。
- **外部滚动条与持久关联：** ScrollView 和 TableView 通过 BindScroll 关联独立的
  ScrollClient，内置条可隐藏且滚动仍有效。声明式 ScrollBar 和 TabBar 通过 Target
  跟随目标控件的局部替换；ui.BindTarget 可供自定义 View 使用。关联不增加布局
  容器，外部滚动条由应用自行布局。
- **窗口与文件选择：** NoKeepRunning 辅助窗口不阻止最后一个普通窗口关闭后的
  自动退出，应用退出时仍统一清理。文件过滤使用单段扩展名，由 GUI 统一校验和
  适配 ASCII 大小写；Linux Portal 的取消和其他未选择结束响应正常返回空结果。
- **绘制：** Direct2D 曲线使用原生椭圆、圆角及 GeometrySink 弧段，保持抗锯齿；
  样式盒形的矩形、统一圆角、混合圆角和选边路径共用对齐后的外框。
  原尺寸图片由绘制后端统一对齐物理像素起点，避免小数布局位置带来的再次滤波；
  图片缩放和其他用户变换继续正常采样。
  Demo 的 ListRow 操作箭头使用矢量图标，不依赖字体字形。
- **macOS 分发工具：** 独立 PKCS#12 证书与 Notary API 密钥可用于跨平台 Developer ID
  签名、Apple 公证、票据装订及 ZIP 归档。已有 macOS 钥匙串入口保留；正式分发仍
  须使用有效凭据，并检查最终 .app 的签名、公证与运行结果。

## Demo 更新

保留九页导航、八种语言和现有主题切换。

- 菜单与覆盖层页增加 MenuBar 的 Auto／InWindow 切换、三级子菜单及命令勾选。
  菜单栏与弹出菜单共享 Command，直接开关、菜单操作和可用状态保持同步。
- 布局页增加独立布局的外部滚动条，直接通过 Target 控制 ScrollView；原有独立
  数值滚动条仍保留。
- 系统页的打开／保存选择器提供 txt／md 与全部文件过滤；保存只选择目标，不写入。
- 基础页末尾保留设置行展示，版本值与构建配置统一为 0.8.2。

操作、预期与复位方式见 [Demo 使用说明](../example/demo/README.md#v082-展示与发布准备)。

<a id="api-migration"></a>

## 从 v0.8.1 升级

### 文件扩展名过滤

GUI 和声明式 FileFilter 的 `Patterns` 改为 `Extensions`：

```go
ui.DialogOptions{
    DefaultName: "notes.txt",
    Filters: []ui.FileFilter{
        {Name: "Text", Extensions: []string{"txt", "md"}},
        {Name: "All files"},
    },
}
```

扩展名不带点或通配符，ASCII 字母大小写不敏感；同组取“或”，空 Extensions
允许全部文件。当前不支持文件名 glob 或复合后缀，如 `*.txt`、`.txt`、`tar.gz`。
DefaultName 是完整建议文件名；框架不据过滤器擅自改写或补齐后缀。
过滤器辅助原生选择，不替代应用对所选文件的验证。

gui.FileFilter／DialogOptions 现在由 GUI 定义，ui 继续使用 GUI 类型，不能再直接
与 platform 的同名结构体互换。自定义 platform.FileDialog 实现须增加 FilterFormat，
按报告的格式接收 FileFilter.Values；GUI 负责把 Extensions 转成对应原生值。
未自定义平台适配的应用只需修改过滤器声明。

### 命令范围与 Target

| v0.8.1 | v0.8.2 |
| --- | --- |
| `ui.CommandGroup(content).Commands(...)` | 改为 `ui.CommandScope(content).Commands(...)`；范围解析和局部优先语义保留。 |
| `wui.TabBar("pages")` | 构造用法保留；关联现在持久跟随目标，可用 `.Target("other-pages")` 改绑。 |
| AfterUpdate 中手工查找滚动目标并连接 | 声明式外部滚动条可用 `ui.ScrollBar(axis, targetID)`；自定义 View 可使用 `ui.BindTarget`，并管理自己的连接。 |

Target 表示关联，Group 表示成员之间的行为／外观联动，Scope 表示命名和管理范围。
Window／Surface 提供默认命令范围；显式 CommandScope 保留局部覆盖。
缺失目标会解除关联，目标重新出现后再绑定；目标 ID 在同一 UI Root 中解析。

命令勾选值需由激活处理提交，例如：

```go
ui.Command("show-details").Checkable(true).BindChecked(&showDetails).
    OnActivate(func() { showDetails.Set(!showDetails.Get()) })
```

直接 SetChecked 或 BindChecked 更新只发布状态，不再次激活命令。
自定义快照消费者需要识别新增的 menuitemcheckbox 角色。

### 菜单栏、子菜单及平台适配

```go
ui.MenuBar(
    ui.MenuItem("View").Submenu(
        ui.MenuItem("Details").Command("show-details"),
    ),
).Mode(gui.MenuBarAuto)
```

顶层菜单项应使用 Submenu；分支不同时执行叶项动作。Auto 原生呈现只覆盖所属
桌面窗口的业务菜单，不表示提供应用菜单角色、移动导航或 Linux 全局菜单。
平台可通过可选的 WindowMenuBar.SetMenu 能力接入；不支持时使用窗口内呈现，
不要求所有 Window 实现增加必选方法。events 的按键与修饰键入口继续可用。

NoKeepRunning 是窗口创建选项，默认 false；声明式同 ID 重建不能修改创建选项。
应用退出时这些窗口仍被清理，不等同于脱离应用生命周期。

### 原尺寸图片的绘制行为

DrawImage 在总变换只有平移、目标物理宽高分别等于源像素宽高时，自动对齐最终起点。
尺寸、长宽比与裁剪不变，原尺寸平移按物理像素步进；缩放、旋转、错切和镜像保留
原坐标及线性采样。Icon／Image 的用法不变，无须新增设置。自定义绘制若依赖
原尺寸图片的小数平移，应核对这一行为变化，规则见
[图片绘制契约](DesignGraphicsColor.md#原尺寸图片的设备像素对齐2026-11-26)。

## English release notes

Status: **preparing v0.8.2**, compared with v0.8.1. The source demo is **0.8.2 / build 1**;
published downloads remain v0.8.1 until the new release is available.

- Add adaptive menu bars: use the macOS system menu bar or an in-window presentation.
  Cascading submenus support pointer and keyboard navigation, edge placement and
  independent active-item and expanded-path feedback.
- Fix hover flicker across submenu shadows and retain an already selected action
  through ordinary menu cleanup. Release modal input before executing it once.
- Share checked command state across menu entries and shortcuts. Activation handlers
  commit actual state; publishing state does not activate the command.
- Bind independently laid out external scrollbars to ScrollView or TableView.
  Persistent declarative Target associations follow local widget replacement and
  also serve TabBar and custom views.
- Add NoKeepRunning windows and extension-based native file filters. Treat known
  Linux Portal non-selection responses as dismissal while preserving protocol and
  communication errors.
- Use native Direct2D curves and consistent styled-box outer contours. Use a vector
  arrow for the Demo's actionable ListRow. Align one-to-one images to physical-pixel
  origins across renderers, preserving filtered geometry for scaling and other transforms.
- Support portable Developer ID signing, Apple notarization, ticket stapling and ZIP
  creation with PKCS#12 and API-key files.

Migration: replace FileFilter.Patterns with bare Extensions; GUI dialog options are
no longer aliases of platform types. Custom platform.FileDialog implementations
must report FilterFormat and accept native FileFilter.Values. Rename
ui.CommandGroup to ui.CommandScope. The TabBar target-ID constructor is unchanged;
external scrollbars use ui.ScrollBar(axis, targetID). Checkable commands must publish
their state in the activation handler. Existing events key/modifier APIs remain.
DrawImage now snaps one-to-one images under pure translation to physical-pixel
origins; their placement advances in pixel steps. Icon and Image need no new options.

The demo retains nine pages and eight languages, adding menu-bar presentation,
cascading menus, shared checks, external scrolling and file-filter examples in the
existing pages. Current README screenshots are retained.

## 发布与维护范围

计划同步分发 Windows amd64、Linux amd64／X11 和 macOS arm64 Demo。
本轮准备不扩大架构、窗口系统或移动平台的正式支持范围。
正式发布后从对应 Release 下载 EXE、Linux 可执行文件或 macOS .app ZIP；Linux
文件需具有执行权限，macOS 解压后打开 GOUI Demo.app。

0.8.x 为当前维护系列，重点维护最新迭代；不单独承诺对旧迭代回补，也不保证迭代
之间无修改升级。版本和维护规则见 [Release](Release.md#1-版本与发布范围)。
本次准备、候选冻结与最终发布进度见 [Plan](Plan.md#v082-preparation)。
