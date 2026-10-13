# Basic 兜底与 Modern 便利入口

> 2026-10-13 确认。主题选择是可选外观，控件可用性不应依赖 Modern 是否启用。
> 本轮不修改绘制后端、控件行为、UI 协调机制或自定义样式表整体替换的契约。

## 1. 问题与范围

原先 gui.DefaultStyleSheet 只覆盖 GUI 基本控件；widgets/style 的指示器、
开关、表格、标签等样式要求应用手工拼接。tests/temp 中未应用 Modern 的
CheckButton／Switch 占有布局区域，但指示器没有绘制颜色，内部标签也缺少
专用样式的字体/颜色。回归已在旧实现复现透明像素和缺失字体。

现代样式还要求调用方重复把 Settings 的四项外观值写入 Options。
修正两项独立问题，不默认启用 Modern，不向 UI 添加具体主题 API。

## 2. 归属和默认契约

- theme/basic 保存 GUI 与 widgets 的全部兜底值，只依赖 style、core 与标准库。
  不导入 GUI、UI、Widget 或 platform 类型，不注册全局规则，不监听设置。
- basic.Rules 返回新切片，basic.Sheet 返回完整独立样式表；原有配色、字体、
  状态、圆角、描边、焦点和阴影原样迁移，不以搬迁为由改成 Modern 配色。
- gui.DefaultStyleSheet 缓存完整 Basic；gui.DefaultStyleRules 转发 basic.Rules。
  Application.StyleSheet 仍返回用户显式设置的值；未设及 SetStyleSheet(nil)
  返回 nil，实际解析使用 Basic。
- widgets/style.Rules 保留兼容，转发 basic.WidgetRules 子集；新调用方无需拼接。
  已有 DefaultStyleRules + widgets/style.Rules 的组合仍可用，重复同值规则不改外观。
- 自定义样式表仍整体替换 Basic；若需要只覆盖几个规则，显式追加到 basic.Rules
  或 modern.Rules。显式透明、零描边和空阴影按既有字段契约生效，不补回默认值。
- StyleName 不跨名称回退，父控件不向子控件提供样式。内部文本分别解析自己的名称。
- 平台窗口装饰与 Controls 仍由 GUI／平台管理，不迁入主题规则。

默认 Basic 表格按既有 widgets 兜底提供 1 DIP 水平网格，网格独立占位，不盖住
单元格内容。此前未拼接样式的测试默认无网格；相应期望必须改为内容高度 + 1 DIP，
并同时断言原内容高度完整保留，不能通过关掉网格掩盖迁移后的布局契约。
完整表头样式还暴露了 TableView 无界测量的既有缺陷：layout.Inf 是有限哨兵，
扣减边框／表头会把它变成巨大有限视口。仅在有限轴扣减占位，保留无界语义；
不修改 ScrollView/ListView 接口或控件行为，也不放宽虚拟化断言。

## 3. Modern 便利入口

```go
type Settings interface {
    ColorScheme() gui.ColorScheme
    AccentColor() color.Color
    FontFamily() string
    FontSize() float32
}

func FromSettings(settings Settings) style.StyleSheet
```

消费方的小接口，gui.Settings 与 ui.Settings 都满足；不要求 Language、
ConnectChanged 或 Application。现有颜色枚举由 GUI 导出，因此 Modern 便利
入口允许依赖 GUI；反向依赖不存在，GUI 只接入 Basic，UI 只使用普通 StyleSheet。

FromSettings 逐项读取当前值，调用已有 Sheet(Options)；不访问 gui.App、不保留
对象、不订阅信号、不安装样式、不引入隐式更新。传入非 nil 设置对象；调用遵循
该对象已有线程契约（GUI 主线程，UI 包装可同步投递）。源颜色由现有规则归一化
复制，之后修改源设置/颜色不会改动旧样式表。

声明层一行选择并跟随既有系统设置重建：

```go
return ui.Root().StyleSheet(modern.FromSettings(app.Settings())).Windows(/* windows */)
```

命令层一行应用当前快照：

```go
app.SetStyleSheet(modern.FromSettings(app.Settings()))
```

命令层若需实时跟随，仍显式连接 Settings.ConnectChanged 并管理 signal.Handle。
固定亮暗/主题色或追加规则继续使用 Sheet(Options)／Rules(Options)。不为一行
语法隐藏监听、生命周期或应用对象，也不增加 Root.Modern／ui.ModernStyle。

## 4. 参考与取舍

[Avalonia 官方 Themes 文档](https://docs.avaloniaui.net/docs/styling/themes)
（2026-10-13 查阅，页面更新 2026-04-20）区分轻量 Simple 与现代 Fluent，并通过
应用样式声明选择。采用“基本外观与可选现代外观分离”的定位；不采用其要求应用
显式安装主题才能提供控件模板的前置条件，GOUI 默认 Basic 必须可用。
这是官方文档层的参考，不是对应源码、平台后端或 GOUI 实测的证明。

## 5. 验证与边界

- 包内：Basic 原有 sRGB 值、DIP 几何、完整文字与状态、切片所有权和显式覆盖；
  GUI 默认入口、custom 整体替换/清空；兼容包装；无主题 CheckButton／Switch 的
  Software 1x/2x 像素（内部样点，预乘 RGBA8，远离抗锯齿边缘，精确字节断言）。
- Modern：GUI/UI 设置接口兼容、四项映射/读取次数、Options 归一化、颜色复制及
  旧快照隔离；原有 UI 设置变化合并/等值去重测试保持有效。
- 窗口：tests/window/ui/themes，中文入口包含默认/显式 Basic、系统/深色 Modern、
  恢复 nil、选中联动、禁用、键盘焦点、内部文字及身份/边界检查，不使用 DevServer。
  tests/temp 不改，直接验证原例程；example/modern 系统默认模式使用 FromSettings。
- 完成前执行全仓 go test ./...，相关 race/vet、Windows amd64／macOS arm64 编译，
  本机真实桌面 Software/OpenGL 1x/2x。远端未运行，不以编译代替 Windows/macOS 验收。

### 5.1 本轮执行结果（2026-10-13）

- 通过：Go 1.25.8 linux/amd64，`go test -json -count=1 ./...`：2387 个测试／
  子测试通过，15 个既有跳过，0 失败，57 个测试包通过。既有测试仍会创建原生
  窗口和输出图片，本轮不迁移它们；新回归不向源码目录输出产物。
- 通过：`go test -race -count=1 ./theme/... ./gui ./ui ./widgets/...`。
- TableView 旧测量实现复跑 `TestTableViewUnboundedMeasurement`：高 80 DIP 表头
  下两种无界高度用例均得到 1e9，而非 82 DIP；修正后四种有限／无界组合通过，
  不实现行控件。既有万行回收池上限、内容完整高度及同步表头断言继续保留。
- 普通 `go vet ./theme/... ./gui ./ui ./widgets/... ./example/modern
  ./tests/window/ui/themes` 未通过：GUI／UI 既有文字编辑与部分旧测试存在未具名
  外包结构体字段（composites）告警；本轮未修改这些文件，不计作 vet 通过。
  同命令加 `-composites=false` 后通过；新主题／widgets／两个例程的普通 vet 通过。
- 编译通过：`CGO_ENABLED=0` 的 Windows amd64／macOS arm64 全仓 `go build ./...`，
  及 theme/basic、theme/modern、gui、ui、widgets、widgets/style、widgets/ui 的
  `go test -c` 测试目标。仅编译，没有远端运行。
- 依赖检查：Basic 的仓库内依赖仅 core／style；GUI／UI 生产代码未导入 Modern。
  `git diff --check` 通过，未修改用户的 README 或 tests/temp。

本机真实桌面：Ubuntu 22.04.5 amd64 用户空间、X11 1.21.1.4、XFCE／Xfwm4
4.16.1；OpenGL 使用 Mesa 23.2.1 llvmpipe，无硬件 GPU 性能结论。

- tests/window/ui/themes 的 Software／OpenGL、应用 1x／2x 四组均通过：默认及
  显式 Basic、系统及强制深色 Modern、恢复 nil、鼠标切换、禁用项不改变计数、
  Tab／Space 切换、禁用项跳过及 F6 身份／文字／边界／状态检查；截图观察指示器、
  滑块、文字和焦点边框无裁剪。通过真实桌面输入，不使用 DevServer。
- 原 tests/temp 在修改前 Software 1x 只有 HeaderBar，两个控件不可见；修改后
  Software 1x／OpenGL 2x 均显示方框／勾、开关轨道／滑块及独立文本，原例程不改。
- 测试窗口已正常关闭，日志／截图／构建产物位于仓库外临时目录。
- 未执行：真实系统设置变更、原生系统 HiDPI、Windows／macOS 桌面和 Direct2D。
  设置映射与已有自动重建的包内测试通过，不用它们冒充系统变更实测。

## 6. Basic 边缘与配色优化

2026-10-13 用户确认：仅调整 theme/basic/check_button.go 与 switch.go 的外观值。
Basic 的搬迁先保留旧外观；此后独立进行以下修改，不混同为迁移契约。

- 选中／部分选中方框取消同色 1 DIP 描边，保留 18 DIP 指示器与 4 DIP 圆角；
  实底仍为 #3370CC，白色勾／横杠不变。未选中保留中性 1 DIP 轮廓。
- 单选改为同状态同色的实心蓝底及原有 8 DIP 白色圆点，不再使用 2 DIP 深色圆环。
  外径仍为 18 DIP；禁用选中与方框相同，为中性灰底和灰色标记。
- 开启开关取消同色 1 DIP 描边，实底改为 Basic 默认蓝 #4682DC，悬停／按下
  分别为 #427ACF／#3E72C2。关闭轨道、白色滑块、禁用轨道及滑块色值不变。
  36×20 DIP 胶囊、16 DIP 滑块、内外留白、拖动和动画不变。
- 按钮式外观、2 DIP 键盘焦点、字体、布局、事件、Snapshot 和 Modern 均不改。
  自定义样式表仍可显式设置自己的边框；绘制实现不会隐式消除用户描边。

从控件现有绘制路径可见，原选中方框／开启开关先填充再绘制同色轮廓；两次
抗锯齿 alpha 合成会强化边缘覆盖。这不是默认兜底缺少抗锯齿，也不需要调整
Painter。包内回归在旧样式下已复现：方框 (7,6) 的 G 为 34，单次几何覆盖预期
17.96；开启轨道多个边缘样点也超出原有误差范围。此为 Software 的实测证据，
不直接当作其它后端相同像素值的证明。

补充回归：选中状态统一的无描边规则与保留未选中轮廓；Basic 的 checked／mixed／
radio 在 1x／2x、整数／1/4 DIP 平移、正常／悬停／按下／禁用的独立几何边缘覆盖；
开启 Switch 复用已有半圆积分参考，覆盖 1x／1.25x／2x 与相同状态／平移组合。
比较透明底上的预乘 sRGB RGBA8；沿用既有 10 字节级误差，覆盖路径量化、圆弧
近似和取整，未放宽容差。内部颜色／白色圆点断言按已确认视觉契约调整，保留
完整几何和透明外部覆盖。复用 tests/window/ui/themes 和 widgets/check_button
的显式兜底模式，不新建例程或改变绘制／GUI 生产代码。

### 6.1 本轮执行结果（2026-10-13）

- 通过：Go 1.25.8 linux/amd64，`go test -json -count=1 ./...`，2464 个测试／
  子测试通过，15 个既有跳过，0 失败；保留既有窗口测试及图片输出的副作用。
- 通过：`go test -race -count=1 ./theme/... ./gui ./ui ./widgets/...`；
  `go vet ./theme/... ./widgets/... ./tests/window/ui/themes
  ./tests/window/widgets/check_button`。此限定范围的 vet 不代替 §5.1 所列
  GUI／UI 既有 composites 告警的全范围检查。
- 编译通过：`CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./...` 与
  `CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build ./...`。未执行远端运行。
- 旧样式下三个新增回归失败；新样式下 Basic 规则、默认指示器内部颜色、
  48 组方框／单选边缘和 24 组 Switch 胶囊覆盖通过；原有 Modern 覆盖保留。
  `git diff --check` 与格式检查通过，产物位于仓库外临时目录。

本机环境同 §5.1，使用真实 X11 桌面输入，不使用 DevServer。Software／OpenGL
各应用 1x／2x 共四组完成如下检查：

- `tests/window/ui/themes`：默认／显式 Basic 与恢复 nil；方框／开关同步切换、
  禁用项点击不增加计数；F6 的身份／状态／文字／边界检查和正常退出均通过。
  截图观察 18 DIP 方框及胶囊边缘、白色标记和滑块，外观无裁剪。
- `tests/window/widgets/check_button -fallback-style`：单选实色底和白色圆点可见；
  鼠标选 B、Tab／Space 选 C、组内取消／选中计数和 F6 身份检查通过。
  鼠标焦点无额外边框，键盘焦点仍可见，关闭后均输出 `DESTROY PASS`。
- 检查过程中一次使用了已失效的 X11 窗口 ID，自动化命令报 `BadWindow`；
  重新按标题解析实际 ID 后补测对应操作通过，不将失败命令计作验收通过。
- 未执行：真实系统 HiDPI／设置变化、硬件 GPU、Direct2D、Windows／macOS 桌面。
  此轮生产代码只调整 Basic 两个样式文件；未修改 Modern、GUI 或 Painter 实现，
  未提交／推送。
