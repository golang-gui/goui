# ScrollView 外框与编辑器组合

## 职责

ScrollView 拥有完整滚动区域的外框，正文视口、横纵滚动条和交汇角都在外框
内。正文控件只绘制正文背景、文字与领域装饰。无样式名的 ScrollView 保持
透明，不自动读取子控件的样式名或判断子控件类型。

采用已有 StyleName / part，没有新增控件、公开接口或主题匹配系统：

```go
scroll := gui.NewScrollView()
scroll.SetStyleName("text-view")
scroll.SetChild(gui.NewTextView())
// 声明式：ui.ScrollView(ui.TextView()).Style("text-view")
```

Basic / Modern 的 `text-view` 默认 part 保留背景、字体、前景及选区等文本
样式，BorderWidth 为 0；`frame` part 提供背景、边框和圆角。`trough`、
`thumb` part 明确提供滚动条外观，不把 frame 边框传播给滚动条。
TextInput 保留自身外框。直接使用 TextView 得到无框正文；需要统一外框时
显式组合带 `text-view` 样式的 ScrollView。已有自绘外框的查找框等组合保持
各自样式，不因正文使用 TextView 而自动增加外框。

## 内区、状态与失效

ScrollView 使用自身 Focused / ContainsFocus 绘制焦点外框；键盘焦点经
实际聚焦控件的 FocusVisible 选择对应状态，禁用时使用 Disabled。
ContainsFocus 的变化只请求绘制，不请求正文布局。

StyleChanged 从 Normal、Focused、FocusVisible、Disabled 四个状态计算
最大安全边距，缓存为控件私有普通值；样式名、StyleScope 或应用样式表变化
沿既有失效机制重算。边框宽度和圆角归一化为非负有限值。边距为
`border + max(0, radius-border)*(1-1/sqrt(2)) + 1`（有圆角时），
无圆角则为 border。额外 1 DIP 隔开圆角抗锯齿边缘。所有正文和滚动条使用
同一矩形内区，不需要新增圆角裁剪接口；交汇角由 frame 背景填充。

焦点或禁用状态变化不会改变安全边距，因此不会重新换行、改变滚动范围或
移动正文。主题实际变化可以改变内区，正常请求重排。

Measure 对原有约束减去内边距，测量正文，给最终外部尺寸和基线加回边距；
无界约束仍保持无界。Arrange 先扣除安全边距，再在内区联立计算两条滚动条
占位。视口和条的位置是 ScrollView 局部坐标；正文仍使用视口局部坐标，
ScrollIntoView 仍接收未减滚动偏移的正文坐标。滚轮、拖动自动滚动、命中、
语义最大范围共用同一内区。极小分配将各轴边距限制到该轴一半，不生成负
尺寸或越界子控件。语义树继续隐藏结构视口及滚动条，只暴露正文。

## 参考与验证边界

- [GTK4 GtkScrolledWindow](https://docs.gtk.org/gtk4/class.ScrolledWindow.html)
  的文档提供可选 frame，并把 viewport、scrollbar 放在同一滚动容器下。
- [Qt 6 QAbstractScrollArea](https://doc.qt.io/qt-6.10/qabstractscrollarea.html)
  继承 QFrame，在内部放置 viewport 和两条滚动条。

这些是文档中的职责参考，未对其运行时像素效果作对照测量。GOUI 使用自己的
显式 part 和普通矩形布局，不复制 CSS、QFrame 类型体系或主题继承。
回归覆盖普通 ScrollView 不变、双条与无条、极小尺寸、焦点/禁用稳定性、
内容坐标 reveal，以及 Software 1×/1.5×/2× 的圆角和边框像素。真实窗口
组合和滚动交互由 SourceView 的 `tests/window/editor/frame` 验证。
