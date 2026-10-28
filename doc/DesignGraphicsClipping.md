# 路径擦除与完整窗口内容

状态：2026-10-28 已实现并完成本轮验收。本文替代 Linux Integrated 安全矩形的旧方案。
实现与验收进度见 [Plan](Plan.md)，通用路径裁剪仍为独立的未来需求。

## 公共接口与契约

`graphics.Painter` 和 `gui.Painter` 增加 `ClearPath(path graphics.Path)`。
路径的填充区域擦除为透明黑；覆盖率为 c 时，预乘目标 RGBA 四通道均乘以 1-c。
完全覆盖归零，外部不变；边缘保留未覆盖部分。不是透明色的 SourceOver 填充。

- 路径是局部 DIP，随当前 Transform、HiDPI 和既有矩形 Clip 生效。
- 不改变 Transform、Clip 或后续绘制的合成状态，正常绘制可以重新填满已擦区域。
- 填充采用非零绕数规则，支持现有单轮廓路径；未闭合轮廓隐式闭合。空路径及无交集区域不绘制。
- 不扩展多轮廓、孔洞、公开 FillRule 或任意合成模式接口。
- 在正常帧或 RenderImage 的回调内使用；不改变窗口透明配置及鼠标输入区域。
- 半覆盖边缘重复擦除会继续衰减，不能按幂等裁剪使用。

语义参考 [Qt CompositionMode_Clear](https://doc.qt.io/qt-6/qpainter.html#CompositionMode-enum)
及 [Cairo CLEAR](https://www.cairographics.org/operators/#cairo_operator_clear)。

## 三后端

- Software：复用路径扫描，生成包围盒与 Clip 交集内的覆盖率，再修改目标 BGRA。
  复用临时遮罩缓冲；不读回或复制整帧。scanFT 的透明 draw.Src 不保留未覆盖部分，
  不能直接作为擦除。
- OpenGL：nanovgo 补齐 SourceOver / DestinationOut 状态，随 Save/Restore 保存、
  BeginFrame 重置，逐条命令记录并按顺序提交。ClearPath 用不透明填充和
  ZERO / ONE_MINUS_SRC_ALPHA 混合。不能在延迟命令外临时切换 GL 状态，
  不新增窗口 shader。参考上游 [NanoVG](https://github.com/memononen/nanovg/blob/master/src/nanovg_gl.h)。
- Direct2D 1.1：保存 primitive blend，透明黑 COPY 填充 Geometry，恢复原状态。
  COPY 的边缘公式为 S*c+D*(1-c)，见 [官方定义](https://learn.microsoft.com/en-us/windows/win32/api/d2d1_1/ne-d2d1_1-d2d1_primitive_blend)。

## 宿主绘制收尾

`Root.ConnectAfterPaint(func(Painter, geometry.Rectangle)) signal.Handle` 在 Widget、
窗口控件及宿主边框完成后、提交前触发；Linux Integrated 最终形状处理在其后。
Window / Popover 共用 rootBase 实现。

回调 bounds 是完整表面的局部 DIP 矩形。每位订阅者从单位变换、完整 Clip 开始，
绘制状态独立，Save/Restore 必须配对。Painter 只在同步回调内有效。
与 Widget.Paint 一样不能修改控件树、销毁宿主或嵌套开启绘制。
连接不驱动连续帧；连接、阻断、断开后由调用方 RequestPaint。
RenderWidget 不触发宿主 AfterPaint。窗口销毁使连接失效；Popover 隐藏保留连接。

UI 通过 `WindowView.OnAfterPaint` 薄绑定、保存最新回调，重建不重复连接；
`WindowView.Transparent(bool)` 只传递创建选项。用户仍需显式 RequestUpdate。
自定义 None+Transparent 窗口在 AfterPaint 构造四角外侧路径并调用 ClearPath，
不在父控件 Paint 中擦除后任由子控件重新覆盖。例程提供四角函数，不增加窗口
专用 Painter 方法。透明像素不会自动穿透输入。

## Linux GUI Integrated

根布局和结构 Clip 使用完整客户区，不再保留四边布局空白，不插入隐藏 Widget。
自动内容最小尺寸不再附加边缘宽度；显式 SetMinSize 保持客户区含义。
HeaderBar 及控制按钮以完整根分配定位，窗口局部输入、Snapshot、IME 坐标不变。

顺序：透明清空 → 矩形背景 → 完整 Widget 树 → 控制按钮及边框 → AfterPaint
→ 四角擦除 → 提交。12 DIP 半径钳制到短边一半，半径为零跳过，四块不重叠。
最大化、全屏关闭圆角；全屏隐藏控制按钮。边框保持内轮廓，外轮廓扩展到最终
擦除曲线外，避免外缘重复抗锯齿。无需 SurfaceClipper 或 Painter.End 的窗口行为。

四角路径的闭合直边放在表面外侧 2 个物理像素处；圆弧坐标和半径不移动。
不能直接沿窗口边缘闭合：圆弧在切点与闭合边形成零宽尖端，NanoVG 的 AA
条带会侵入保留区域，表现为直边末端整颗像素消失。外延由 PixelScale 换算为 DIP，
在普通矩形 Clip 内只留下需要擦除的圆角外区域。这是遮罩几何的构造要求，
不改变 ClearPath 的合成、后端或 nanovgo 依赖。自定义透明窗口例程采用相同构造。

切点回归同时覆盖四角的水平／垂直边缘、1/1.25/1.5/2x 和有无边框。
参考独立圆面积采样，要求切点附近的覆盖率处于圆轮廓内外各 1/4 物理像素
对应的面积区间；采样及量化边界仅留 2/255。包内还独立检查闭合边位于表面外、
圆弧切点／半径不变、偏移矩形和小窗口钳制。不能仅以圆内部／外部采样代替此项。

缩放范围独立保留：约 5 DIP 边缘，16 DIP 角部延伸。宿主控制器在控件获得按下、
焦点或手势前接管缩放，防止铺满后的按钮、滚动条抢占。已有指针捕获、模态及
显式 Client 覆盖继续有效；不改变全局手势竞争优先级。光标来自最终 Chrome 查询。
原生输入面保持矩形，透明角部仍可用于缩放。

本切片不迁移 Popover / BorderBox 的布局与装饰策略，不新增窗口形状或输入穿透 API。
ClearPath 会同时去除下面的内容与阴影，不替代圆角容器裁剪。

## 实施与验证

1. nanovgo 合成状态和延迟命令；三绘制后端及像素回归。
2. Root AfterPaint、GUI 状态隔离、自定义透明窗口和 UI 薄绑定。
3. Linux Integrated 铺满及缩放输入隔离，更新原有回归与文档。

包内验证完全擦除、部分覆盖的独立预期、预乘、Clip、Transform、HiDPI、
擦除后绘制、连续帧、资源生命周期及 AfterPaint 隔离。桌面程序置于 tests/window，
使用中文说明。独立 X11 会话验证 Software/OpenGL；Windows 验证 Direct2D，
macOS 验证 OpenGL，并记录实际环境与未验证项。执行全仓测试、相关 race/vet
及目标构建；不能把编译通过或窗口启动记为操作验收。

nanovgo 先在本地仓库提交，测试无误后发布新 tag；GOUI 直接依赖已推送的 tag，
不提交本机 replace。用户无关文件及侧会话文档不纳入提交。

实现复核发现：Direct2D 旧 FillPath 默认使用偶奇规则，与其他后端的自交轮廓结果
可能不同。本轮仅 ClearPath 显式选择非零绕数，旧接口保留行为；后续统一 FillPath
时需单独设计并增加三后端自交路径回归。
