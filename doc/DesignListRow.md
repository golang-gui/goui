# ListRow 设置与操作条目

状态：2026-11-11 实现；同日调整默认主题表面与示例层次。2026-11-17 增加
[ListRowGroup](DesignListRowGroup.md) 连接分组，行本身行为不变。验证范围记录在
[Plan](Plan.md)。

## 契约与 API

`widgets.ListRow` 是普通 Widget，可放入 VBox、ScrollView，或由 ListView delegate
复用。它持有标题/说明和两个独立插槽；设置值、导航与业务动作仍由应用持有。
不增加 ListView 模型、选择状态或窗口管理能力。设置页的分组标题、间距与额外
装饰使用现有容器及样式组合；默认行自身提供表面和边界。

需要视觉连成一体的多行时使用 [ListRowGroup](DesignListRowGroup.md)：它把直接
包含的行零间距堆叠，只保留外侧圆角并让相邻行共用一条边框。连接几何由行在
绘制时从所在分组推导，行的样式、状态、输入与语义不变。

```go
row := widgets.NewListRow("Automatic updates")
row.SetSubtitle("Notify when an update is ready")
row.SetPrefix(icon)
row.SetSuffix(toggle)
row.SetActivatable(true)
handle := row.ConnectActivated(func() { /* update application state */ })
```

对应 getter 为 Title、Subtitle、Prefix、Suffix、Activatable、Padding、Spacing。
标题和说明沿用 Label 的显式格式前缀：普通字符串原样显示，只有 `\{format\}`
开头的有效来源启用格式。Snapshot 使用显示文本，不包含格式源。空标题/说明
不参与布局，前后插槽均可为 nil。每个插槽接受一个 Widget，多项内容由 HBox 组合。
同一 Widget 不可同时占用两个插槽；不能把行本身或内部布局/Label 作为插槽。

## 布局与外观

默认 MinSize.Height=48 DIP，内边距左右 12、上下 10 DIP，插槽间隔 12 DIP，
标题与说明间隔 4 DIP。调用者可用普通 MinSize/MaxSize 修改约束；父硬约束优先。
有效有限宽度下填满可用宽度，无限宽度下使用自然内容宽度。行高由文字、控件与
边距决定，不用固定高度截断大字号或多行说明。

先为尾部控件测量宽度，再约束前置内容，剩余宽度交给内部文字 VBox；文字使用
WrapWordChar。窄到无法容纳全部内容时缩小插槽约束和间隔，不使槽位互相覆盖；
父控件可继续执行结构裁剪。前后内容垂直居中，其 MainWeight 不改变槽位策略。
这里只定义三个槽位的宽度分配，文字排版与列布局仍使用现有 Label/LinearBox。
滚动设置页通常应显式使用纵向 ScrollView；允许水平滚动的容器可按自然宽度呈现。

独立样式名为 `list-row`、`list-row-title`、`list-row-subtitle`，焦点使用行的
`focus` part。Basic 和 Modern 都提供完整规则，作为 widgets 包的内置主题规则。
`list-row-group` 没有默认规则：连接外观由各行自己的表面与边框组成，组不叠加
背景。内部 Label 通过自己的 StyleName 解析；前后插槽保持自己的样式，行的
StyleName 不向子树隐式继承。默认外观采用独立行表面，说明字号为标题的 90%；
应用仍可通过这些独立样式名覆盖背景、边界与排版。非激活行保持常态表面，
不绘制悬停/按下状态；禁用、键盘焦点和主题变化沿用 GUI 的状态与失效机制。

- Basic 只提供清晰兜底：白色表面、1 DIP 灰色边框、2 DIP 黑色键盘焦点框及
  2 DIP 圆角。颜色与字体从现有基础规则取得；主次文字共用黑色，通过字号区分。
  悬停/按下复用 Button 的灰阶，不增加专用强调色，也不随系统明暗变化。
- Modern 保持平台中立：复用当前调色板的 Button 表面、主/次文字色，1 DIP
  中性边界与 8 DIP 圆角。悬停/按下与普通 Button 一样向前景混合 4%/8%，边界
  混合 12%。禁用保留表面，只让文字和子控件按既有禁用样式呈现。焦点复用
  focusColor，根据页面、常态、悬停及按下背景检查对比度，不新增全局色板。

Demo 设置区使用一个 [ListRowGroup](DesignListRowGroup.md) 把四行连成一体，
共享接缝代替原来的 8 DIP 行间距；标题区间距 12 DIP，不再用两层同色 Card
包围行。连接仍由显式容器组合，不新增设置页容器或主题匹配机制。

## 输入与生命周期

默认 Activatable=false、自身不可聚焦，尾部控件仍可操作。SetActivatable(true)
接入已有 Motion/Click/Key 控制器，并设置自身 Focusable=true；关闭时移除行的
控制器、清理按压并取消自身焦点。之后可用 SetFocusable(false) 单独退出行的 Tab
导航。整行默认鼠标点击、无修饰的 Space/Enter 在 key-down 激活一次，重复键不
重复执行；键盘处理要求焦点在行本身，编辑框等后代保留自己的键盘输入。

点击子控件时复用现有 gesture 竞争和事件传播，子控件赢得点击后不再激活行。
行不调用子控件的语义动作，不合成点击，也不自动推断 Switch/DropDown 的含义。
整行切换设置时，应用将 OnActivate 与控件 OnChange 连接到同一业务状态；从行
或从控件各操作一次，应只更新一次。所有输入仍走 Surface.DispatchEvent 路径。
Setter 不发送 Activated。通知期间检查存活、Root 归属、有效禁用和 Activatable；
前一个回调释放宿主或改变准入后，后续旧通知不继续执行。

SetPrefix/SetSuffix 替换时先解除旧内容，不主动销毁可复用的旧 Widget。挂载/卸载
回调允许重入；setter 检查代次和 Root 归属，不能覆盖回调中设置的新内容，或在
宿主释放后继续挂载。Window 最终兜底销毁控件树；内部 Label 在 unmount 释放
文字资源。Popover 的释放与 Widget 最终销毁不同，不以 Root=nil 伪造 Destroyed。

## UI 与语义

```go
// import wui "github.com/golang-gui/goui/widgets/ui"
wui.ListRow("Automatic updates").
    Subtitle("Notify when an update is ready").
    Suffix(wui.Switch().Checked(enabled).OnChange(setEnabled))

wui.ListRow("Download folder").Subtitle(folder).
    Activatable(true).OnActivate(chooseFolder)
```

UI 提供 Title、Subtitle、Prefix、Suffix、Activatable、OnActivate、Padding、
Paddings、Spacing；移除 modifier 恢复构造默认，显式零值生效。Activatable 同时
声明默认 Focusable，之后的 Focusable(false) 可覆盖；遵守链式声明的最后写入顺序。
适配器用两个稳定 Bin target 和既有 BuildContext.UpdateChild 协调插槽，不向
Root/BuildContext 添加控件特例。重建保留 Widget 和各槽位的身份，更新回调闭包，
Unmount 断开信号。OnActivate 本身不会隐式开启 Activatable。

Snapshot.Role=listitem，Text 为显示标题和说明，以换行分隔；不重复暴露内部
布局/Label。Children 保留前后插槽的真实角色、状态与窗口局部边界；只在行可
激活且有效启用时提供 click 动作。没有隐含 selected/checked；Switch/输入框等
后代保留自己的状态和动作。Name 仍由应用显式设置，不承担格式源或业务设置键。

## 参考与验证

- [libadwaita 1.6.0 ActionRow 源码](https://github.com/GNOME/libadwaita/blob/1.6.0/src/adw-action-row.c#L20)
  与 [ActionRow 官方文档](https://gnome.pages.gitlab.gnome.org/libadwaita/doc/1-latest/class.ActionRow.html)
  （查阅时库版本 1.10.0）：参考标题/说明与前后内容的组合。其源实现通过
  mnemonic-activate 激活目标 Widget；GOUI 使用显式业务回调和现有事件竞争，
  不引入 GTK 的类继承、CSS 或目标激活转发。
- libadwaita **1.6.0** 的
  [默认表面色](https://github.com/GNOME/libadwaita/blob/1.6.0/src/stylesheet/_defaults.scss)、
  [列表样式](https://github.com/GNOME/libadwaita/blob/1.6.0/src/stylesheet/widgets/_lists.scss)
  与 [状态色](https://github.com/GNOME/libadwaita/blob/1.6.0/src/stylesheet/_colors.scss)
  是源码层面的外观参考：通过页面/条目表面、留白、主次排版和中性状态变化建立
  层次。GOUI 采用这些设计方法，但复用自己的 Basic/Modern 颜色角色和 Button
  状态比例，不复制 Adwaita 色值、CSS 类体系或特定平台外观。
- [Flutter 3.35.0 ListTile 源码](https://github.com/flutter/flutter/blob/3.35.0/packages/flutter/lib/src/material/list_tile.dart)
  及 [ListTile 官方文档](https://api.flutter.dev/flutter/material/ListTile-class.html)：
  参考 leading/title/subtitle/trailing 的结构；GOUI 的行高由内容测量决定，不采用
  Material 的行数预设或 Material 祖先绘制策略。
- [SwitchListTile 官方契约](https://api.flutter.dev/flutter/material/SwitchListTile-class.html)
  把整行视为单一开关语义。GOUI 通用行保留独立子控件语义和业务状态；目前不为
  通用容器合并/伪造 Switch 的状态，也不把单个控件需求扩为通用动作执行系统。

以上分别是文档/源码参考与 GOUI 的设计取舍，不能代替本项目实测。包内测试覆盖
约束、空槽位、语义、有效禁用、事件隔离、键盘重复/移出取消、资源回调重入、
声明身份/默认恢复，以及 Basic/Modern 常态、悬停、按下、禁用的 1x/2x 软件表面/
直边像素（预乘 sRGB RGBA8，允许 1 字节量化误差）。Modern 明暗主题在 125 种
accent 下检查各行状态的文字对比度至少 4.5:1、焦点至少 3:1；这不表示边界或
悬停色差也需达到文字的对比度。Linux Pango 实测换行
和 14/26pt 字号不需要桌面，执行原生调用的子测试锁定 OS 线程。
`tests/window/widgets/list_row` 使用正常桌面输入和只读 Snapshot 验收，不使用
DevServer。Demo 基础页提供开关、整行动作、下拉与只读值的八语言设置样例。
桌面缩放、原生后端和移动触摸的实际验证边界见 Plan。
