# 非显示区域的绘制与续帧

## 范围

本切片实现通用屏幕可绘制性查询、空结构裁剪子树剔除、确定离屏的 Widget 重绘请求过滤，
以及 ProgressBar／Demo 无限视觉动画的按需续帧。鼠标移动引起的额外重绘、固定 60 Hz
节拍和原生帧时钟留到后续，不在本切片改变。

参考 Qt Widgets 6.8.3 的
[子控件区域相交筛选](https://github.com/qt/qtbase/blob/v6.8.3/src/widgets/kernel/qwidget.cpp#L5742)
及 Flutter 3.35.7 的
[Sliver 可绘制范围检查](https://github.com/flutter/flutter/blob/3.35.7/packages/flutter/lib/src/rendering/sliver_multi_box_adaptor.dart#L749)。
GTK 4.20.3 通过 [Viewport overflow](https://github.com/GNOME/gtk/blob/4.20.3/gtk/gtkviewport.c#L435)
及 RenderNode 缓存组织绘制。GOUI 保持现有即时绘制，不引入节点缓存或 ScrollView 类型特判。

## 查询契约

```go
func IsDrawable(widget Widget) bool
```

这是 GUI 线程上的辅助查询，不扩展 Widget／Root／Painter 接口，不改变 Visible，
不增加 style.Invisible、属性或可见性变化信号。

- 未挂载、销毁／卸载中、自身或祖先隐藏、宿主暂停或失效时返回 false。
- 内置宿主尚未准备过有效屏幕绘制区域时返回 false。
- 以当前分配的 DIP 矩形，沿父链累加原点并与祖先结构边界、根裁剪求交。
  尺寸为空或交集为空时返回 false；部分可见为 true。
- true 表示可能绘制，不推断兄弟覆盖、窗口遮挡、透明度或最终 ClearPath 擦除。
- 不刷新布局、不构造 Snapshot、不查询原生系统，不分配临时集合。计算为 O(父链深度)。
- 内置 Root 缓存宿主提供的屏幕根裁剪；Popover 的安全内边距仍由 Popover 决定。
  非内置 Root 使用其根 Widget 的分配边界，以及可用的 Visible 查询，不猜测原生状态。

宿主绘制和查询共用矩形相交规则。ScrollView 的内容负偏移及内部 viewport Widget
已经表达滚动和裁剪，查询不读取 ScrollX／ScrollY，不判断容器具体类型。
查询只报告当前分配，不预测尚未执行的布局。

## 绘制遍历

计算 Widget 的窗口原点及结构裁剪，裁剪为空时跳过 Paint 和整个子树；部分可见时
仍绘制自身，由现有 Painter 限制输出。沿用既有样式失效通知及状态栈隔离。
控件 Paint 的临时 SetClipRect／Transform 不向子树传播，不能据此剔除子树。

可见区域像素不变：旧实现已在空有效 clip 时抑制图元，本次进一步避免 Paint 内的
文字／图片准备及 DrawingArea 栅格化。Measure、Arrange、事件、挂载和 Snapshot
仍按原契约处理，Snapshot.Visible 不改为“本帧绘制过”。
Paint 不保证每根帧执行；业务状态、计时及最终释放不能依赖 Paint。缓存和失效标记
保留到实际绘制，DrawingArea.ShouldDraw／DrawFunc 在重新可见后按既有规则执行。

## 重绘请求

WidgetBase.RequestPaint 先记录既有 Widget 请求意图，再决定是否转发：

1. 内置 Root 已暂停或销毁时不转发。
2. 布局或状态样式待处理、根裁剪尚未知时保守转发，不能用旧几何丢弃可能影响布局的更新。
3. 几何稳定且确定不可绘制时不转发；其余沿用 Root.RequestPaint。

不增加逐 Widget 待绘制队列。数据／资源失效标记仍由控件持有；滚动或重新显示引起
正常宿主绘制时读取最新状态。RequestLayout 始终保留，例如离屏 Label 换行变化仍需
更新内容高度和可见滚动条。Root.RequestPaint 与原生 PaintEvent 保持窗口级含义。
非内置 Root 继续由其自身管理请求转发。

## 无限视觉动画

ProgressBar 保留被动的 Root.ConnectFrame，回调只采样 elapsed，不无条件请求下一帧。
增加 Root.ConnectAfterPaint：使用最新布局及根裁剪查询 IsDrawable，仅可绘制时续帧。
开启／挂载时连接，关闭／卸载／Root 迁移时统一断开；裁出视口保留被动连接。
启动请求和绘制请求失败仍按现有行为停止连接，避免资源滞留。

滚动触发一帧 → 完成新布局 → 控件重新可绘制 → AfterPaint 请求下一帧，因此不需要
ScrollView 通知或滚动监听。离屏不改变 Indeterminate、Value、起始时间；恢复时使用
实际时间的循环相位，不补画历史帧。部分可见继续运行。

Demo DrawingArea 的逐帧开关采用相同的绘制后条件续帧，保留开关状态和资源所有权。
普通数值 Animation、业务任务和 Timer 不因裁剪自动停止。

## 离屏渲染

RenderWidget 使用目标图片自己的原点和结构裁剪，不能用屏幕 IsDrawable 阻断。
目标 Widget 即使在滚动视口之外，仍可渲染其当前分配子树；不继承目标之外的祖先裁剪。
RenderWidget 不发出 Root.Frame／AfterPaint，也不改变屏幕根裁剪缓存，不启动动画续帧。

## 诊断与验证

沿用 Widget.PaintRequests（请求意图）、Root.PaintRequests（实际转发）、PaintCalls、
DrawingGenerations 等计数，不增加常驻观测。HUD 的一次空闲清零帧作为独立预期保留。

- 包内：普通容器／嵌套 viewport／部分可见／空尺寸／祖先隐藏／宿主暂停，查询与实际
  绘制一致；查询零堆分配，不依赖 ScrollView；失效连接及初始根裁剪未知处理。
- 请求：离屏稳定状态不转发；待布局及待状态样式时保守处理；重新显示读取最新值。
- 像素：可见部分的颜色、透明、clip、Transform、1x／2x 保持；空裁剪不调用 Paint 子树。
- 离屏：屏幕不可绘制子树仍可 RenderWidget，图像非空，不产生屏幕 Frame／AfterPaint。
- 窗口：忙等待滚出后仅允许已排队帧与 HUD 清零帧，之后帧数／资源稳定；部分可见续帧，
  滚回恢复，业务状态不变；DrawingArea 逐帧模式及主窗口／Popover 生命周期同样验证。
- 运行 go test ./...、相关 race／vet、Windows amd64／macOS arm64 交叉构建；
  本机独立 X11 的 Software／OpenGL 1x／2x 验证。跨平台编译不等于原生验收。

实际结果记录到 Plan，按通用内核和视觉动画／Demo 接入拆分提交。
