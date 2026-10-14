# 通用裁剪与局部擦除：后续规划

状态：待评估，未实现，见 [未来路线](Roadmap.md#future-clipping)。当前 Linux GUI Integrated 采用圆角背景与内部安全矩形方案，
不为模拟窗口装饰增加 Painter 圆角裁剪、路径擦除或窗口专用遮罩。

## 职责与当前选择

- platform 的透明窗口能力负责提交、合成带 Alpha 的画面，不决定窗口外观。
- graphics.Painter 提供通用绘图操作，不持有 WindowChrome、HeaderBar 等 GUI 策略。
- gui.Window 决定窗口背景、内容布局、装饰和缩放区域。
- 透明绘制本身是完整能力。普通 SourceOver 填充透明色不会擦除目标；Clear 清空目标与
  透明色参与普通填充不是同一种操作。
- SurfaceClipper 及 Painter.End 隐式窗口遮罩已移除；不补齐 Direct2D 的专用遮罩。

当前 Linux 模拟 Integrated 同时限制布局和绘制：根控件的 Measure／Arrange 使用
四边安全内容区域，窗口局部事件与 Snapshot 坐标不变；派生最小尺寸计入边缘，
显式 MinSize 保持客户区语义。HeaderBar 查询高度及按钮定位计入内容起点。
Window 先绘制圆角背景，再将内部安全矩形作为控件树及自绘按钮的结构 ClipRect；
控件自己的裁剪不能扩大它。最后恢复裁剪、绘制窗口边框。
默认半径 12 DIP 时安全内缩为 5 DIP；较大圆角/边框按几何安全距离增大内缩，
预留一物理像素的抗锯齿余量并向内对齐物理像素。缩放热区使用同一内缩量。
最大化、全屏不内缩。框架以安全区域布局，避免把边缘控件先铺满再裁掉；
缩放区域属于窗口管理的装饰，契约见 [GUI WindowChrome](DesignGUIWindowChrome.md)。
窗口背景与控件背景一致时外围自然融合；背景不同时外围显示窗口背景。

## 方案一：通用圆角裁剪

适用需求：保留底层背景，只限制某组后续绘制，例如需要任意内容铺满圆角容器。

- GUI 决定几何边界，graphics 执行；不提供窗口专用接口。
- 必须明确裁剪坐标系、Transform、矩形裁剪相交、替换、恢复和跨帧重置。
- 当前 SetClipRect 会由 Widget 遍历反复设置，不能让它意外清除外层结构边界。
- 抗锯齿裁剪需明确组合语义：逐图元重复应用覆盖率与整组内容合成后应用一次并不等价。
- Direct2D 可利用带几何遮罩的 Layer；OpenGL/nanovgo 需要额外裁剪支持，Software
  需要覆盖率及适当的合成实现。通用能力应在三个后端共同实现和验证。
- 尚不确定采用 SetClipRoundRect 还是作用域式接口；不因本轮窗口需求提前定型。

## 方案二：ClearPath 局部擦除

适用需求：明确让已有画面的指定区域变透明，例如绘制结束后清除窗口四角。

候选接口：ClearPath(path Path)。这是一次性操作，不增加持久绘制状态；遵循当前
Transform 和矩形 Clip，不改变之后的绘制状态。命名和与 Clear 的契约仍需正式确认。

- 路径内部像素归零，边缘按覆盖率 a 将预乘 RGB 和 Alpha 同时乘以 1-a。
- GUI 构造四个圆角外部区域，决定调用时机；Painter 不认识窗口圆角。
- Direct2D 1.1 可临时设置 primitive COPY，透明黑填充几何后恢复状态；覆盖率公式为
  S*a + D*(1-a)，S=0 时就是所需擦除。
- Software 可复用路径扫描的覆盖率，但当前 scanFT 的透明 draw.Src 不能直接代替：
  其实现不保留部分覆盖像素的未覆盖目标分量，需要适当的擦除像素处理。
- nanovgo 当前延迟提交且采用固定混合方式，需要在绘制命令中记录擦除混合状态；
  不能仅在排队绘制前后切换 GL 状态。应复用路径栅格化，不另建窗口 shader。
- 局部擦除会同时删除下层内容，不等价于裁剪某个子控件；有窗口外阴影时，还需设计
  绘制顺序，不能误擦需要保留的阴影。

若将来仅需要局部擦除，ClearPath 比引入完整裁剪状态体系更小；若需要通用圆角容器，
则应按裁剪需求设计，不能用擦除偷换语义。

## 后续启用条件与验证

出现明确需求后再选择方案，不同时预建两套机制。实施前重新检查三个后端和 nanovgo。
至少覆盖路径/圆角边缘、预乘透明合成、Transform、矩形 Clip、后续状态不受污染、
HiDPI、资源生命周期和正常帧性能；运行 go test ./... 并完成三端验证。

参考：
[Direct2D primitive blend](https://learn.microsoft.com/en-us/windows/win32/api/d2d1_1/ne-d2d1_1-d2d1_primitive_blend)、
[Direct2D layers](https://learn.microsoft.com/en-us/windows/win32/direct2d/direct2d-layers-overview)。
