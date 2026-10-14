# 样式驱动 Icon

## 统一资源契约

Icon 只通过公开的 IconSource 获取最终图片，不再分为直接绘制和图片两条路径。
不提供 NewIconSource、IconDraw、IconRasterizeFunc 或回调式 NewIconImage。

```go
type IconSource interface {
    Image(width, height int, foreground Color) image.Image
}

func NewIcon(source IconSource) *Icon
func (i *Icon) Source() IconSource
func (i *Icon) SetSource(source IconSource)
func (i *Icon) Size() float32
func (i *Icon) SetSize(size float32)
```

任何包都能直接实现 Image，不要求继承、注册、私有标记、公开 revision 或销毁接口。
具体资源实现属于可选 icon 包体系，不放在 gui 或 platform/graphics：

- gui：IconSource、Icon、最终图片及原生资源缓存。
- icon.Image：固定图片、多分辨率选择和着色策略。
- icon/svg、icon/draw：SVG 与 Software 离屏资源。
- icon/internal/pixels：资源实现共享的着色算法，不作为 graphics 公共 API。

width/height 是期望物理像素尺寸，包含实际分配区域、HiDPI 与当前变换最大伸长。
foreground 是 Icon 自身样式前景（预乘颜色），不一定是主题色，也不是整个 Settings。

Image 同步运行在 GUI 线程，不进行文件/网络加载、不修改 GUI、不保存 Painter。
资源应在创建时完成加载和有效性检查。Image 不返回 error；有效请求范围为每边
1..4096 像素。返回图片不可变，尺寸不必等于请求尺寸；nil/空图片表示内容缺失，
由 Icon 显示占位符，完全透明的有效图片则正常显示。图片按标准 image.Image
颜色约定提供像素，上传边界负责格式转换。Icon 不再进行二次着色。

Source 内容在使用期间保持稳定；自定义内容变更通过 SetSource 显式告知 Icon。
如果 Source 自己有 CPU 缓存，必须自行保证内容更新时缓存有效；Icon 不调用隐藏的
资源失效方法。已返回的图片像素始终不可变，即使 SetSource 也不能使原地修改像素
成为有效的刷新方式；内容改变时提供新图片。内置资源的内容不可变，替换内容时创建新实例。

## 资源实现

### 固定图片和多分辨率图片

```go
source, err := icon.NewImage(image16, image32, image64)
if err != nil {
    return err
}
// 若需要原色，创建共享图片集合的独立配置。
// source = source.WithRawColors()
```

多张图片表示相同内容和比例的不同分辨率，不是动画帧、不同主题或状态集合。
构造函数要求至少一张图片，每张必须非 nil 且非空，任意无效输入都会返回错误。
图片集合为私有字段，构造函数复制切片；不能在创建后替换其中的图片。
选择足够大的最小版本，不足时选择最大版本。
不强制把选中的图片重采样到请求尺寸。

默认由 icon.Image 将图片 alpha 遮罩按 foreground 着色，缓存最近的版本/颜色结果。
WithRawColors 返回保留原始 RGBA 的独立配置，共享不可变图片集合。
构造函数不复制全部像素，Source 共享传入的不可变图片；调用方不得修改像素。
零值 Image 不包含资源，Image 方法返回 nil。Source 含内部缓存和锁，不应复制已经使用的值。

pixels.Tint 对 RGBA/NRGBA 直接读取第四字节的 alpha，按源 Stride 逐行处理；
其他 image.Image 保留通用 At/RGBA 路径，不降低 16 位 alpha 的精度。
两条快速路径共用内部循环，不新增缓存、资源池或公共接口；输出仍独立、原点为零，
保留既有预乘与舍入方式。SubImage 的 Pix 已从 Rect.Min 开始，不重复偏移。

### SVG

`icon/svg.Parse(io.Reader) (*svg.Source, error)` 直接返回实现 IconSource 的资源。
svg.Source 负责解析结果、最近一次原始栅格和着色结果；仅换前景色不重新栅格化。
WithRawColors 返回共享解析内容的独立配置，适合在 UI build 外创建。
支持范围与限制见 [DesignSVG](DesignSVG.md)。

### 自定义绘制

应用可以自己实现 Image，也可以使用可选 `icon/draw.Source`：

```go
source := &draw.Source{
    Draw: func(p graphics.Painter, size graphics.Size, fg graphics.Color) {
        center := graphics.Point{X: size.Width / 2, Y: size.Height / 2}
        p.FillEllipse(center, size.Width/2, size.Height/2, fg)
    },
}
```

回调使用独立 Software 离屏 Painter，坐标是物理像素，结果为最终不可变图片。
size 是请求的物理像素宽高，绘制原点固定为 (0, 0)，不再传入 Rectangle。
包内回归同时覆盖正方形与非正方形请求、尺寸传递、裁剪、半透明颜色和旧图片不变。
不借用窗口 Painter、不模拟 Widget 树、不在 Icon 中保留旧的直接绘制入口。
回调不能持有 Painter 或调用 Begin/End/Destroy；适配器负责帧和资源清理。
返回前复制 Software 输出，避免销毁或下一次绘制覆盖图片。
持续动态内容仍适合普通自定义 Widget，不要求 Icon 承担动画系统。

## Icon 的职责

- 默认期望边长 16 DIP；非法尺寸归零。nil 接口 Source 是空内容，不画占位符。
  采用普通 Go 接口语义，不额外归一化 typed nil；非 nil Source 需履行 Image 契约。
  Source 返回缺失内容时，在相同布局区域用前景色绘制方框加叉，不上传替代图片。
- 空内容自然尺寸为零；其余自然尺寸为 Size × Size，父约束/minmax 决定分配。
- 使用分配区域的居中正方形作为目标；返回图片再等比居中放入，不变形拉伸。
- 从自己的 StyleName（默认 icon）、Normal 状态取前景色；缺失为透明色。
  Modern 的普通 icon 是正文色，AccentIcon 是主题色，不继承父控件样式/状态。
- 复用 StyleChanged/RequestPaint 失效流程，不直接监听全局 Settings。
- Snapshot 保持 RoleImage、边界、ID、可见性，无伪造交互；点击时组合 Button。

## 缓存和生命周期

| 层次 | 所有者 | 契约 |
| --- | --- | --- |
| 解析结果、原始位图、遮罩、中间 CPU 缓存 | Source 实现 | 不可变输出；可供多个 Icon 共享 |
| 最近一次最终图片及上传重试结果 | Icon | 持有引用，不额外着色 |
| graphics.Image | Icon；所属原生 Painter 最终兜底 | 不跨窗口共享，卸载释放 |

Icon 的请求键只有物理尺寸和前景色；SetSource 使请求失效，不将接口
放进可比较的键。相同请求跨帧直接复用原生图片。新请求仍可返回原来的不可变图片，
安全可比较且相等时直接复用原生图片；不可比较的返回值正常上传，不猜测内部身份。

SetSource 每次显式调用都刷新，包括同一个实例。Icon 只缓存最近请求的最终图片
（包含缺失结果）及其原生图片，不再维护错误对象、revision 或异步通知。
非 nil Source 的设置保留旧图片作为复用候选，但下次 Paint 必须先重新取图：
相同不可变图片复用上传结果，不同图片或缺失输出释放旧资源，绝不先画旧图再刷新。
候选资源至多保留到下次有效取图、SetSource(nil) 或卸载；设置 nil 和卸载立即清理。
同一缺失请求不会因重绘反复调用 Source；尺寸、前景色变化或 SetSource 后重新获取。
超出请求尺寸预算时直接画占位符，不调用 Source。图片上传失败暂时画占位符，
保留 CPU 图片供后续重绘重试上传，不重新调用 Source，也不主动触发重绘循环。
新请求不再保留旧语义图标作为视觉替代。GPU 设备失效时不保证占位符能显示。

加载/解码/解析错误在 NewImage、svg.Parse 等创建入口返回；Image 是已准备资源的
取图接口，不承担 I/O 错误交付。删除 ConnectRenderError / OnRenderError；
特殊处理由调用方在资源创建阶段或自定义 Source 内完成，不在 Paint 中发送业务回调。

Paint 中替换或卸载时直接调用旧图片 Destroy；平台后端负责立即逻辑失效、帧内
延迟物理释放。Icon 不维护 inFrame / retired 列表，gui.Painter 不提供 AfterFrame。
跨窗口重新挂载不带走旧窗口的原生图片。

## UI 薄适配

```go
ui.Icon(source).Size(20).Style(modern.AccentIcon)
ui.Icon(original).Size(32)
```

ui 暴露 IconSource 别名、Source/Size modifier，不保留 ui.IconImage 别名。
使用者通过 icon.NewImage 创建位图资源，gui/ui 不反向依赖 icon。没有资源构造
框架，也不向 Root/BuildContext 添加 Icon 特例。

IconView 每次 Update 直接调用 SetSource，不比较 IconSource，也不记录上一轮 Source。
UI 只保存恢复默认 Size 的状态，不推断资源身份或缓存有效性。
推荐在 build 外准备并复用 Source，使其内部缓存持续有效。UI 重建后重新调用 Image，
不等于重新栅格化或上传：Source 复用生成结果，gui.Icon 复用相同不可变图片的原生资源。
普通重绘仍直接使用 Icon 的请求缓存。自定义内容变化可以通过 UI 更新或 GUI SetSource
显式刷新。Size modifier 移除恢复默认值；IconView 不管理渲染错误信号。

## 验证与参考

包内测试覆盖：布局/样式/快照、公开接口与不可比较值、空内容、不同返回尺寸、
等比居中、最终颜色不被二次处理、1x/2x 和变换、构造时验证、切片隔离/像素共享、多位图选择/上传复用、占位符/上传重试、
跨窗口重新挂载与 End 后销毁。保留旧像素/样式测试的有效覆盖，原来的直接绘制/
零上传断言调整为离屏生成和图片缓存语义。

独立窗口入口：tests/window/ui/icon、tests/window/ui/svgicon；不依赖 DevServer。
资源选择/着色测试位于 icon，通用控件缓存/生命周期测试留在 gui。icon 的外部测试
通过公开 gui.Painter 验证真实 Image Source 的上传复用，不让 gui 测试依赖具体资源。
全仓测试、针对性 race、Windows/amd64 和 macOS/arm64 交叉编译用于代码验证，
三平台视觉/操作体验仍需实际验收，编译不替代桌面运行。

资源归属迁移验证：`go test ./...`、`go test -race ./gui ./ui ./icon/... -count=1`
通过；gui/ui/icon 及资源子包测试、两个窗口例程的 Windows/amd64、macOS/arm64
交叉编译通过。依赖检查确认 gui/ui 不导入 icon，graphics 不再提供 TintImage。

参考 [Qt QIcon](https://doc.qt.io/qt-6/qicon.html) 的按需图片选择，但不照搬其
engine、主题搜索或 mode/state 系统。GOUI 使用普通公开 Go 接口和明确对象生命周期。

## 2026-09-18 创建与取图接口调整

- 采用 Image(width, height, foreground) 返回最终 image.Image；不返回运行期渲染错误。
- icon.NewImage 在创建时验证并固定图片集合，原色配置已同步当前 WithRawColors 接口。
- 旧错误通知测试改为缺失结果缓存、刷新、占位符和上传重试测试；保留原有像素、
  生命周期、布局及样式覆盖。typed nil 归一化的旧断言改为当前明确的 nil 接口契约。
- 参考 Qt 6 [QIcon.pixmap](https://doc.qt.io/qt-6/qicon.html#pixmap) 的结果命名方式，
  以及 GTK 4.18.6 [GtkImage 缺失资源替代](https://github.com/GNOME/gtk/blob/4.18.6/gtk/gtkimage.c#L547)；
  不引入图标主题查找、资源注册或新的绘图后端接口。

本轮验证（Linux/amd64，CGO_ENABLED=1）：

- `go test ./...` 通过；包含已有缓存结果，需显式启用的桌面测试不作为本轮验收。
- `go test -race ./gui ./ui ./icon/... -count=1` 通过。
- Software 离屏占位符像素测试覆盖 1x/2x、半透明预乘色、旋转和矩形裁剪；
  区分缺失图片、合法全透明图片与 nil Source，并验证无图片上传。
- `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go test -exec=true ./...`、
  `GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go test -exec=true ./...` 通过；
  仅交叉编译，不执行目标平台测试。
- `tests/window/ui/svgicon` 已补位图资源、缺失占位和空 Source 的操作说明；
  本轮未运行三平台桌面视觉/交互验收。已有绘图测试仍会输出其原有 PNG 产物，
  本轮不迁移这些旧测试；新增测试不向源码目录写文件。

### UI 协调修正

删除 IconView 的 Source 比较与副本状态；缓存判断只针对 gui.Icon 已取到的不可变
图片，不针对 Source。新增回归验证每次声明协调重新取图、相同像素只上传一次、
相同 Source 提供新图片、替换上传失败不绘制旧图，以及待刷新资源的 nil/卸载清理。
旧帧内释放测试调整为重新取图后释放，仍断言 End 前不能物理释放、End 后不得泄漏。
位图旧测试同步当前 WithRawColors 接口，保留原色及像素共享断言。

Linux/amd64 全仓测试和 GUI/UI/icon 的 race 检查通过。受影响包及 svgicon 窗口例程
的 Windows/amd64、macOS/arm64 交叉编译通过（CGO_ENABLED=0、-exec=true，仅编译）。
三平台桌面体验未重新执行；窗口例程说明已改为区分取图次数与栅格化/上传次数。
