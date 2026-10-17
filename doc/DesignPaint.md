# DesignPaint — 文本图片缓存 + Widget 绘制作用域

绘制时机：
- 布局变化
- 控件视觉状态变化

保留绘制的内容：
- OpenGL/Software Painter 所属的文字 `graphics.Image`

## 1. 一帧绘制与缓存边界

当前路径包括 TextLayout shaping／断行、栅格化、CPU 图片转换／上传、图元提交与
桌面呈现。Label／编辑器复用布局，OpenGL／Software 的 ImageCache 按 layout 和
有效 raster scale 复用最终图片；Direct2D 直接绘制原生 DirectWrite layout。

本文不再把 quad 吞吐量、Go Paint 耗时或 GPU 工作量直接当作“不是瓶颈”的证据。
旧稿的微秒／毫秒级排序没有同环境对照数据，不能作为性能承诺。缓存的确定性
收益是命中时省去对应的 Rasterize／NewImage；端到端耗时仍需按后端、内容和缩放测量。

## 2. 历史决策与修正

### 2.1 record-replay 跨帧缓存 → 撤掉

**决策**：撤掉 `recordingPainter` / `RenderNode` / `replay` 的跨帧缓存模型。

**原因**：
- 引入 stale 缓存、生命周期双轨、整树重录等已验证的 bug
- 记录／回放不能直接代替文字光栅图片缓存；实际收益未经此文测量
- 与 Painter 图片缓存正交；未证明收益前不恢复该方案

**保留的部分**：
- `Widget.RequestPaint()` 接口（Phase 0）—— 接口小、失效精确，与绘制后端无关
- `RenderNode` 结构作为单帧合批的候选（收益未验证，暂不引入）

### 2.2 子树缓存 / 脏区 → 暂不实现

- **子树缓存**：需要明确失效和资源归属；当前未实现，不推定其收益已被其他缓存覆盖
- **脏区重绘**：需要后端目标保留或 render-to-texture；先按真实场景测量，属于待评估优化

### 2.3 ScrollView 滚动

不再依赖 `RenderNode` transform 改 node。ScrollView 将 content 挂在内部
viewport Widget 下，并通过 content 的负布局位置表达滚动。GUI 自动遍历时，
content 的结构 clip 与 viewport 边界相交；内容原点随滚动变化，viewport
本身的 clip 不跟随内容移动。

Painter 的 clip 是绘制约束，不是 Widget 可见范围查询接口。若将来需要
跳过视口外子项，应由 ScrollView 或滚动布局根据 viewport、scroll offset
和子项布局矩形判断，而不是从 Painter 反查可见范围。

## 3. 当前决策

### 3.1 TextLayout 光栅能力与 Painter 图片缓存（核心收益）

`TextLayout` 负责排版和把自身光栅化。它拥有后端私有的临时光栅工作区，但不缓存
某个 scale 对应的最终 `TextBitmap`，也不持有任何 Painter 的 `graphics.Image`：

```go
type TextLayout interface {
    Rasterize(scale float32, buf []byte) (TextBitmap, error)
    ConnectChanged(func()) signal.Handle
    ConnectDestroy(func()) signal.Handle
}
```

- `Rasterize` 的 scale 是每 DIP 的物理像素数；返回紧密排列的预乘 RGBA。
- 返回像素属于调用方；`TextLayout` 和 `Context` 均不保留返回 slice。调用方传入的
  `buf` 容量足够时可以复用，但内部光栅工作区不会与返回结果共享生命周期。
- 每个 `TextLayout` 持有自己的 `textPainter` 和可增长的临时 surface。`Destroy`
  先释放这些光栅资源，再释放原生排版对象。
- 所有影响排版或像素的 setter 同步发出 changed；`Destroy` 同步发出一次 destroy。
- 不公开 revision、`CacheGen` 或通用 AssociatedData。

三个排版后端的具体归属如下：

- Pango：`Context` 只共享 `PangoFontMap`；每个 `TextLayout` 独立持有
  `PangoContext`、`PangoLayout`、Cairo context 和 image surface。不同 scale 的
  `pango_cairo_update_layout` 因此只会更新当前 layout 的排版环境。
- DirectWrite：`Context` 共享 DirectWrite、Direct2D 和 WIC factory；每个
  `TextLayout` 持有自己的 `IDWriteTextLayout`、WIC bitmap 和 D2D render target。
- CoreText：`Context` 负责框架初始化和注册字体；每个 `TextLayout` 持有自己的
  attributed string、CTFrame、CGBitmapContext 和像素 surface。

不同 `TextLayout` 的操作必须相互隔离：在所属线程上交错执行测量、样式修改和
`Rasterize`，不得改变其他 layout 的测量结果、光栅状态或输出像素。该隔离保证
不改变线程模型；`Context` 和 `TextLayout` 仍然线程亲和，不支持并发调用。同一个
`Context` 创建的所有 layout 必须在该 Context 之前销毁。

OpenGL 和 Software Painter 以 `(TextLayout 对象身份, rasterScale)` 为 key 缓存
`graphics.Image`。每个 layout 最多四个精确 scale，使用 LRU；平移、origin、clip
不进入 key。缓存 miss 时执行一次 `Rasterize -> NewImage`，命中时直接 `DrawImage`。
changed 释放该 layout 的全部 scale，destroy 额外断开订阅并删除记录。同一 layout
可以被多个 Painter 使用，各 Painter 独立持有资源。

Direct2D 继续直接绘制 DirectWrite layout，不经过图片缓存；DirectWrite layout
仍实现 `Rasterize`，以便用于 OpenGL、Software 或独立位图输出。

`Platform.NewPainter` 因此只接收 Surface，Painter 不再持有 Typography Context。

### 3.2 控件 TextLayout 字段化（已实现）

Label 的 cachedLayout 由 Measure／Paint 复用，SetText／换行／对齐及 StyleChanged
沿现有失效流程更新，卸载释放。TextInput／TextView 的布局资源由共享编辑核心和
段落缓存管理；不能用示意 Label 的销毁策略代替编辑器的局部失效。

具体实现见 [Label](../gui/label.go)、[编辑核心](../gui/text_editor.go) 与
[文本编辑设计](DesignTextEditing.md)。Paint 可维护内部派生缓存，但不修改业务状态
或控件树；布局／视觉变化分别通过 RequestLayout／RequestPaint 上报。

### 3.3 gui.Painter 与 Widget 子树遍历

`platform/graphics.Painter` 仍是 Surface 帧绘制器；`gui.Painter` 则是只在
一次 `Widget.Paint` 中有效的局部绘图上下文。GUI 内核使用一个扁平 Painter
自动遍历 Widget 树，不再创建 `SubPainter` 链，也不再让父 Widget 调用
`PaintChildren`。

`gui.Painter` 使用 `Save/Restore` 保存当前 Widget 内的 clip 和 transform。
父 Widget 的显式绘图状态会在其 `Paint` 返回后恢复，再进入子 Widget，因此
不会影响子树。Widget-local 原点、祖先结构 clip、空 clip 抑制和 transform
组合仍由 GUI Painter 统一处理。

ScrollView 使用内部 `scrollViewport` Widget 表达内容裁剪；content 是
viewport 的子节点，滚动条是后置兄弟节点。绘制层级、输入命中和布局边界因而
来自同一棵真实 Widget 树，不需要滚动容器专用绘制分支。

完整接口、状态模型、调用示例、诊断和遍历契约见
[`DesignWidgetPainter.md`](DesignWidgetPainter.md)。stroke 位于路径中心，控件
若要在自身边界内绘制 border，仍应将路径向内收缩半个 stroke width。

## 4. 三层缓存的关系

| 层 | 缓存什么 | 不知道什么 | 关联方式 |
|---|---|---|---|
| typography | TextLayout 的原生 shaping/断行数据和私有临时光栅 surface | Painter 及其图片资源 | `Rasterize` + changed/destroy signal |
| graphics | Painter 所属的文字 `Image` | Widget 状态 | `(layout identity, rasterScale)` |
| 控件 | TextLayout 对象 | 缓存内部结构 | 字段持有，setter 失效 |

typography 只返回一次性的 CPU 像素，不依赖 graphics 包的类型。graphics 把它快照为
自己的 `Image`，并通过生命周期通知精确失效；TextLayout 不成为多 Painter 资源注册表。

## 5. 实施路线

### Phase 1：TextLayout 光栅接口与 Painter 图片缓存（已实现）

**改动**：
- `Rasterize` 从 Context 移到 TextLayout；TextLayout 持有自己的临时光栅资源，
  Context 只保留 FontMap、factory 和注册字体等共享设施。
- Pango 为每个 TextLayout 创建独立 PangoContext，避免光栅 transform 和 font
  options 经共享 Context 污染其他 layout。
- TextLayout 提供 changed/destroy 通知，不缓存最终 CPU 位图，也不持有 Painter 图片。
- OpenGL/Software Painter 缓存最终 `graphics.Image`，每 layout 最多四个 scale。
- Direct2D 保持原生 DirectWrite 绘制；Painter 构造不再接收 Context。

**测试**：
- Rasterize buffer 复用、不同 scale、setter 后像素变化和生命周期通知。
- 同 layout 多帧/多位置只光栅化一次，scale LRU、帧内失效和多 Painter 独立释放。

### Phase 2：控件 TextLayout 字段化（已实现）

**改动**：
- `Label` 持有 cachedLayout，setter 通过私有失效流程更新，并请求布局
- `TextInput` 同理（已有的 `ensureLayout` 改为字段持有）
- Measure 和 Paint 共享同一 layout

### Phase 3：gui.Painter 与自动 Widget 遍历（已实现）

**改动**：
- `gui.Painter` 使用 `Save/Restore`；`graphics.Painter` 的帧接口不变。
- Window 和原生 Popup host 通过同一入口自动遍历完整 Widget 子树。
- 删除 `SubPainter` 与 `PaintChildren`，Widget 的 `Paint` 只画自身。
- GUI Painter 分离 Widget 结构状态和局部绘图状态，支持嵌套恢复、错误诊断
  和空 clip 绘制抑制。
- ScrollView 使用真实结构 viewport 裁剪内容。

**验证**：局部坐标正确转换，ScrollView 内容偏移不移动 viewport clip，父
Widget 的 clip/transform 不泄漏给子节点，兄弟 Widget 隔离，空 clip 不产生
底层绘制，Save/Restore 失衡和 Paint panic 均能恢复状态。

## 6. 不做的事

| 事项 | 不做原因 |
|---|---|
| **record-replay 跨帧缓存** | 旧方案已撤销；恢复前须证明收益并解决失效与生命周期问题 |
| **RenderNode 单帧合批** | 收益不明确；需要 Painter 批量接口和额外生命周期 |
| **命令合批** | 未测量证明收益；不能凭图元数量推定后端提交不是瓶颈 |
| **TextLayout 通用 AssociatedData 槽** | 会把多 Painter 资源注册表塞进 typography；专用 signal + Painter 私有缓存更清楚 |
| **Glyph atlas** | Pango/DirectWrite/CoreText 内部已有 glyph cache；atlas 是后端内部概念，与 GUI 层正交 |
| **脏区重绘** | 需要后端目标保留／离屏能力及完整失效契约，待测量 |

## 7. 正确性契约

保留式缓存把一条隐式约定升级为**正确性前提**：

> Widget.Paint 不修改业务状态或控件树；可维护局部派生布局／图片缓存。影响输出的业务状态变化仍通过 RequestPaint（纯视觉）或 RequestLayout（影响布局）显式上报，不靠缓存掩盖失效。

现有控件（Button/TextInput/Label 的 setter）已遵守这一契约；新增自定义控件时必须同样遵守。

## 一句话

TextLayout 以 layout 私有工作区提供无结果缓存的光栅能力和
生命周期通知，OpenGL/Software Painter 缓存最终图片，Direct2D 保持原生绘制；
控件字段持有 TextLayout，gui.Painter 为每个 Widget 提供自动管理的局部绘制作用域。

## 8. TextLayout 光栅工作区的内存权衡

layout 私有 `textPainter` 让资源归属和隔离边界直接对应，但会使已经执行过
`Rasterize` 的每个 layout 保留一块按其历史最大光栅尺寸增长的临时 surface。
这块内存与 Painter 缓存的最终 `graphics.Image` 用途不同，却可能在稳定文字进入
图片缓存后长期闲置。该问题属于工作区内存管理，不改变上述所有权设计。

此优化尚未实施，见 [未来路线](Roadmap.md#future-performance)。后续必须保持以下边界：

- `TextLayout` 仍是其可变原生光栅状态的所有者，不能重新引入跨 layout 的
  PangoContext 或绘制状态共享。
- 优化可以按需创建、主动收缩或释放像素 backing store；也可以让 layout 私有的
  painter 借用受控的像素存储，但不得让一个 layout 的绘制改变另一个 layout。
- 不在 typography 中缓存最终 `TextBitmap`，不把 `graphics.Image` 或 Painter
  关联表放进 TextLayout。
- 是否引入回收阈值、尺寸分级或池化，需要用大量短文本、少量大文本和多 scale
  场景分别测量峰值内存与 cache miss 耗时后再决定。

## 9. 实用化阶段新增目标：渐变与阴影

当前 graphics.Brush 已支持 Color 和双色 LinearGradient；三后端 BoxShadow 已实现。
以下记录现有边界，不把已完成能力继续列为待实施。

### 9.1 分层与 API 原则

- 渐变的 color stop 和几何参数属于 `platform/graphics` 的小值模型；不把 D2D
  brush、NanoVG paint 或原生句柄暴露到公共接口。
- 先实现真实控件需要的线性渐变；径向渐变由实际场景决定，不提前扩展完整效果系统。
- 阴影只描述颜色、偏移、模糊和必要的扩散/轮廓参数，不引入 CSS filter 或通用 shader graph。
- Painter 负责如何画，`gui`/style 负责何时使用以及是否为阴影预留布局空间。
- Direct2D 与 OpenGL 必须共享坐标、透明度、Transform、clip 和 HiDPI 语义；
  后端不能对不认识的公共 Brush 或效果静默跳过。
- Software Painter 应作为确定性参考实现或提供文档明确的降级路径，具体方案在
  公共 API 定型前确定。

### 9.1.1 已实现：双色线性渐变

```go
type LinearGradient struct {
    Start, End           graphics.Point
    StartColor, EndColor graphics.Color
}
```

`LinearGradient` 与 `Color` 一样实现 `graphics.Brush`，故无需给
`graphics.Painter` 或 `gui.Painter` 增加另一组绘制方法。它适用于全部已有的
Fill/Draw 图元。Start/End 是图元所在局部逻辑坐标；范围外分别 clamp 为端点
颜色，端点重合时绘制 StartColor。颜色采用预乘 Alpha 插值，避免透明端点产生
颜色边缘。

OpenGL 映射为 NanoVG `LinearGradient`；Direct2D 使用两项 stop collection 和
linear gradient brush；Software Painter 使用扫描器颜色函数作为确定性参考。三者
共享 Transform、clip 和 HiDPI 语义。多 stop、径向渐变、repeat/reflect 均不属于
本轮范围。

### 9.2 后端方向

- Direct2D：线性渐变使用原生 gradient stop collection/brush；阴影使用
  Direct2D 1.1 DeviceContext 的原生 Shadow effect。
- OpenGL：优先复用 NanoVG gradient/paint 和 GPU 路径；阴影要避免每次绘制产生
  不受控的临时纹理或跨帧资源泄漏。
- 两个后端允许抗锯齿和模糊核存在细微视觉差异，但效果边界、偏移、颜色和裁剪结果必须可对照。

### 9.2.1 已实现：BoxShadow 外阴影

`graphics.BoxShadow` 是效果值而不是 Brush，通过
`graphics.Painter.DrawBoxShadow(rect, radius, shadow)` 绘制。模型只包含 Color、
Offset、BlurRadius 和 SpreadRadius；归一顺序、CSS 高斯映射、圆角卷积、
单边组合配方和三后端实现见 `doc/DesignShadow.md`。`gui.Painter.DrawBoxShadow` 转发同一效果值，坐标、半径、偏移均为控件局部 DIP，
沿用普通 transform 和结构裁剪。阴影需要在控件分配范围内预留空间，不获得越界绘制权限。

### 9.3 验证

- 线性渐变至少覆盖水平、垂直、斜向、透明 stop、超出边界和 Transform。
- 阴影至少覆盖矩形、圆角矩形、偏移、模糊、半透明和 clip。
- 在 1x/2x scale 下对照 Direct2D/OpenGL 输出，并在 Windows、macOS、Linux 真机运行综合示例。
- 资源在 Painter Destroy、窗口 resize 和重复绘制后正确释放或复用。
