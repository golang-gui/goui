# DrawingArea 软件绘图区域

状态：2026-10-30 已确认，分阶段实现；进度与验证见 Plan。

## 边界

样式保持当前模型，不引入复杂边框软件回退、自动纹理缓存或异步渲染。
DrawingArea 是 gui 的普通叶子 Widget，软件生成像素，再用所在 Painter 的
Image 呈现。ui 只协调构造函数和查询信号，不修改 Root／BuildContext。
首版复用 gui.Painter 作为软件绘图上下文，局部 DIP 坐标；不增加 Canvas、
逐段画刷、复合路径或合成接口。复杂像素也可由用户生成 image.Image 后绘入。
不承诺现有 Software 自动具备正确的多色边框合成。

## 接口与更新

```go
type DrawFunc func(p Painter, size geometry.Size)
func NewDrawingArea(draw DrawFunc) *DrawingArea
func (*DrawingArea) SetDrawFunc(DrawFunc)
func (*DrawingArea) ConnectShouldDraw(func(redraw *bool)) signal.Handle
```

绘制函数是唯一操作，不是广播信号。ShouldDraw 是绘制前查询，默认在首次、
逻辑尺寸或有效像素比例变化时为 true，其他情况 false。用户可以覆盖结果；
多个监听者按连接顺序修改同一个 bool，后连接的可覆盖前面结果。
SetDrawFunc 是显式替换，默认请求重新生成；不比较函数指针。查询仍可否决。

每次区域实际参与 Paint 时查询，最终 true 才执行软件绘图。RequestPaint
仅请求画面更新，不清空缓存；没有单独 RequestDraw。每帧绘图应使用现有
帧机制请求画面，并在查询中设 true。没有更新时不自行申请帧或启动计时器。
用户否决尺寸／比例变化时，旧图按原逻辑尺寸显示并裁剪，不拉伸；无旧图则透明。

ui.DrawingArea(draw).OnShouldDraw(fn) 保存最新闭包，挂载时安装稳定代理函数，
普通重建不调用 SetDrawFunc、不生成新像素。业务闭包变化是否需更新内容，由
ShouldDraw 显式判断。卸载断开查询连接并清空声明闭包，GUI 保留最终销毁归属。

## 软件与缓存

每个 DrawingArea 延迟拥有独立 Software Painter，按实际绘制大小与 PixelScale
栅格化。每次重新生成先清透明黑，回调使用局部 DIP 与既有 Save／Restore 隔离。
图片的像素尺寸向上取整，呈现区域使用 pixelSize/rasterScale，避免把补齐像素
压缩到逻辑尺寸；Widget 自身边界继续裁剪。透明区域使用预乘 RGBA 契约。

软件输出可借用其 Painter 的缓冲；窗口 Image 在 NewImage 返回前已复制。
缓存命中只 DrawImage。上传失败保留软件结果，之后可重试上传，不因此反复调用
绘制函数；保持旧可用图或透明，不显示伪造内容。资源／尺寸失败属于呈现失败，
在新的失败状态记录一次诊断；不增加应用错误弹窗。

Unmount 清理软件与窗口 Image，重挂后重新生成。回调可能销毁／迁移对象、
替换函数或改变尺寸：检查生命周期和代次，丢弃过期结果；销毁后不继续通知查询
监听者。重入 Paint 不递归执行软件绘制；回调内 RequestPaint 安排后续更新。
绘图上下文及其中创建的软件原生资源不得逃逸回调，不与窗口 Image 混用。
不在绘图回调中修改控件树、布局或主动调用 Paint。

首版不增加异步任务、LRU、局部脏矩形或动画系统。软件缓冲按需复用；物理大小
沿用离屏绘图上限（单边 16384、总像素 6400 万），非有限／空尺寸不分配。
缓存只是避免重复执行，不能保证昂贵绘图在每帧更新时仍然流畅。

## 布局、样式与语义

自然尺寸为零，用户使用 MinSize／MaxSize／MainWeight 等已有布局属性。
绘制只覆盖自身，不增加 Child、前景绘制或自动装饰。样式变化会请求窗口更新，
但是否重画内容仍由查询决定；回调可以显式解析自身样式，不隐式继承父配色。
默认 RoleWidget，不添加 Click 等动作或焦点；事件控制器、名字、ID、边界与
状态沿用 WidgetBase。用户仍可实现自己的语义 Snapshot。

## 验证与参考

包内测试覆盖默认命中／失效、信号顺序／否决、显式函数替换、上传重试、
主题及普通重建不重新生成、原尺寸保留、1x／2x 与 Transform／clip、预乘颜色、
绘图状态隔离、回调销毁／迁移／重入以及资源释放。
独立窗口程序验证实际软件与 OpenGL 资源、缓存计数、尺寸变化、手动绘图请求、
按帧绘制与停止、主题重建、指针输入；不依赖 DevServer。执行全仓、相关 race
以及 Windows amd64／macOS arm64 编译；原生桌面未测范围单独记录。

参考 GTK4 的 [DrawingArea](https://docs.gtk.org/gtk4/class.DrawingArea.html)
提供绘图入口并复用事件控制器；采用其显式区域职责，不照搬 set_draw_func 的
通知方式。绘制函数是操作，ShouldDraw 是查询，属于不同契约，遵循用户确认。
Flutter 的 [CustomPainter.shouldRepaint](https://api.flutter.dev/flutter/rendering/CustomPainter/shouldRepaint.html)
用于理解内容变化与绘制的区分；GOUI 不比较 delegate 或函数身份，不引入隐式失效。
