# Typography：原生文字排版与栅格化设计

状态：以 2026-10-15 的当前接口与实现为基线，包含 `d67e89d` 的字体、删除线和
占位能力。本文维护长期设计；诊断、实施顺序、提交及具体环境的验证结果见
[TextLayout 实施记录](PlanTextLayout.md)。实现完成不等于全部平台／后端／DPI 已验收。

## 1. 职责与边界

`platform/typography` 提供原生字体匹配、文字 shaping、断行、排版度量、范围格式
和离屏栅格化。Linux 使用 Pango／Cairo，Windows 使用 DirectWrite，macOS 使用
CoreText／CoreGraphics；不依赖 `gui`、`ui` 或 `platform/graphics`。

Typography 不负责文档模型、Markdown／HTML 解析、编辑历史、选区状态、IME、
Widget 排布、链接交互或主题。它可以为这些能力提供排版依据，但不实现上层行为。
文字编辑使用方的职责见 [文本编辑设计](DesignTextEditing.md)。

公共模型表达跨后端需要的概念，不直接暴露 Pango 属性、CoreText attributed
string 或 DirectWrite COM 对象。原生能力通过既有 libs／frameworks／sdk 绑定接入，
具体资源与适配留在后端，不在 GUI 或 Painter 中补偿排版后端错误。

接口定义见 [Context／字体](../platform/typography/context.go)、
[TextLayout／度量](../platform/typography/layout.go) 和
[TextBitmap](../platform/typography/bitmap.go)。源码注释中的前置条件也是契约的一部分。

### 1.1 TextLayout 的定位与接口完整性

TextLayout 对调用方提供的一段固定 UTF-8 文本进行原生排版，允许修改布局约束和
格式，提供度量与绘制结果，并管理自身原生资源。

当前接口已覆盖这一定位的主要能力，职责边界基本成立：

| 职责 | 已有接口能力 |
| --- | --- |
| 输入与约束 | 固定文本、基础格式、布局尺寸 |
| 排版控制 | 字体、换行、水平对齐、Tab、范围格式、占位度量 |
| 排版结果 | 实际占用尺寸、显示行与 Cluster 几何 |
| 绘制 | Rasterize，以及 Painter 对布局的使用 |
| 资源管理 | 显式销毁、变化与销毁通知、线程亲和契约 |

这是接口职责完整性的判断，不代表全部契约和平台组合均已验证。没有字距或自定义
行距，不妨碍它作为基础排版接口使用；不以补齐原生引擎的功能清单作为完善标准。

### 1.2 文本规模与上层拆分

调用方负责文本拆分、可见范围调度、段落组织和布局缓存预算。按段落维护多个
TextLayout 符合这一边界；拆分必须保留所需排版上下文，不能按任意字节数切开文本。
文档模型、选区和编辑历史继续由上层管理。

单个布局的工作量仍需单独评估。MeasureMetrics 导出整个布局的行和 Cluster 度量，
Rasterize 也可能产生较大的位图；上层按逻辑行拆分并不能约束超长单行的工作量。
应明确相关操作的成本、约束和失败行为，分别测量 shaping、度量导出、上层几何
构建与栅格化；不能仅凭可见段落布局或源文本缓存预算承诺大文本延迟和原生内存上限。

### 1.3 接口演进原则

保留当前接口骨架，优先兑现已有契约，再按明确需求评估扩展：

- 核对已有公共语义在各后端的实现，包括 Begin／End 的 RTL 对齐、范围 setter
  的输入校验和失败语义。当前限制及待复核项见第 11 节，不把它们当作已修正问题。
- 明确布局约束、实际占用与字形墨迹外伸的关系，验证斜体、重音等内容的栅格覆盖
  和绘制位置；先检查现有表示是否足够，再决定是否需要新增边界或位图定位接口。
- SetPlaceholder 继续表达参与排版的纯度量，不管理实际 Widget。已有占位能自然
  满足的需求优先复用；不因此把 TextLayout 扩展为富文本文档引擎，也不将占位
  等同于所有显示行的行高或行距控制。
- 新增字距、行高等能力前，说明真实使用场景、现有接口难以正确或低成本实现的
  原因，以及三端原生依赖和维护成本。仅因原生引擎或其他框架提供同名能力，不足以
  构成扩展依据；影响排版的能力须同时覆盖度量、绘制、缓存失效和验证契约。

## 2. 对象与所有权

### 2.1 Context

`Context` 是创建布局的原生设施，不是绘制上下文或布局集合管理器：

```go
type Context interface {
    Name() string
    AddFont(fontFile string) error
    NewTextLayout(text string, format TextFormat, width, height float32) (TextLayout, error)
    Destroy()
}
```

- `Name` 返回诊断用后端名称，不用于应用策略分支。
- 调用方拥有创建成功的 TextLayout；销毁 Context 不隐式销毁布局。
- 必须先销毁全部所属布局，再调用一次 `Context.Destroy`。
- Context 销毁后不再调用任何方法。

### 2.2 TextLayout

TextLayout 表示固定文本在可变布局盒子内的排版结果：

- `Text()` 返回创建时的 UTF-8 文本；生命周期内不修改内容。
- 编辑内容时由使用方创建新布局；上层可以按段落维护多个布局，不要求全文共用一个。
- 布局尺寸、段落对齐、换行方式、范围格式和占位度量可以修改。
- 每个布局独立持有可变原生排版状态和私有 `textPainter`。
- `Destroy` 幂等；销毁后除再次调用 Destroy 外，不再调用其他方法。

Context 与 TextLayout 均线程亲和，不支持并发调用。同一 Context 创建的布局不能
因为各自拥有 textPainter 就在不同线程同时操作；不同布局的隔离保证是：在所属
线程交错测量、改格式、栅格化时，不污染彼此状态。信号句柄的同步机制不放宽这一约束。

### 2.3 资源分层

| 所有者 | 持有的资源 | 不承担的职责 |
| --- | --- | --- |
| Context | 字体设施、原生 factory、字体集合或注册记录 | 不兜底销毁布局，不共享可变 textPainter |
| TextLayout | 原生布局、范围格式、占位度量、私有临时栅格工作区 | 不管理 Widget，不持有 Painter 图片 |
| Rasterize 调用方 | 返回的 TextBitmap 像素及复用缓冲区 | 不借用布局内部 surface |
| graphics Painter | 自身使用的文字 Image 与缓存订阅 | 不成为排版对象的所有者 |

布局销毁时先通知使用方清理关联缓存，再释放私有栅格资源和原生排版资源。
原生资源使用其规定的引用计数、释放函数与线程，不以 Go GC 代替原生生命周期。

## 3. 格式、约束与坐标

`TextFormat` 保存基础字体、基础颜色、换行方式和水平对齐。
`Format()` 返回创建时的基础格式及当前段落对齐／换行设置，不汇总后续范围格式，
也不是对当前文本每个位置有效样式的查询接口。

### 3.1 单位

| 数据 | 单位与原点 |
| --- | --- |
| FontInfo.Size | pt，1/72 英寸；不是布局宽高或物理像素 |
| NewTextLayout／SetSize／Size | 布局局部逻辑单位；GUI 使用时为 DIP |
| TextLine／TextCluster 的几何 | 布局局部逻辑单位，左上原点，Y 向下 |
| TextLine.Baseline | 相对布局顶部的基线 Y 坐标，不是相对行顶部的距离 |
| PlaceholderMetrics.Baseline | 相对占位顶部向下的基线距离 |
| Rasterize 的 scale | 每逻辑单位对应的物理像素数 |
| TextBitmap.Width／Height／Stride | 物理像素宽高及每行字节数 |

字号在后端边界转换为原生要求：Pango 使用 point 字体描述，DirectWrite 使用
pt 到 DIP 的转换，CoreText 使用其 point 字号。不能把桌面缩放乘入 FontInfo，
或用不同默认字体的截图差异判断换算错误。

### 3.2 布局盒子与实际占用

- `Size()` 返回请求的最大布局尺寸；`MeasureSize()` 返回当前文本实际占用尺寸。
- `SetSize` 可以重排已有文本，不必为每次窗口大小变化创建新布局。
- 尺寸约束不等同于墨迹边界；粗体、斜体及某些字形可能外伸。
- 不提供统一省略号、最大行数或自定义行距能力，不由上层猜测这些能力已随约束存在。

换行枚举：`WrapNone` 不自动断行，`WrapChar` 允许按字符断行，`WrapWordChar`
优先词边界，无法容纳时允许字符边界。零值为 WrapNone；实际合法断点由原生排版决定。

水平对齐枚举为 `TextAlignBegin`、`TextAlignEnd`、`TextAlignCenter`、`TextAlignFill`，
零值为 Begin。公共起止边语义与 RTL 原生映射的核对范围见第 11 节，不能据此承诺
三端所有双向段落具有完全相同的对齐与换行结果。

## 4. 字体请求与注册

### 4.1 FontInfo

```go
type FontInfo struct {
    Family string
    Size   float32
    Weight FontWeight
    Width  FontWidth
    Style  FontStyle
}

type FontWeight uint16
type FontWidth uint8
type FontStyle uint8
```

| 字段 | 有效值 | 零值 |
| --- | --- | --- |
| Family | 平台字体族名称 | 平台默认字体 |
| Size | 有限正数，单位 pt | 12 pt |
| Weight | FontWeight，100～900，允许中间值 | Regular（400） |
| Width | FontWidth，1～9 | Normal（5） |
| Style | FontStyleNormal／Italic／Oblique | Normal |

Weight 提供 Thin、ExtraLight、Light、Regular、Medium、Semibold、Bold、ExtraBold、
Black 常量；Width 从 UltraCondensed 到 UltraExpanded，表示字体宽度等级，不是缩放比例。

FontInfo 是匹配请求。缺少所请求 face 时允许原生最接近匹配或 fallback，不保证所有
平台都存在相同字体。属性映射集中在后端；例如 CoreText weight trait 不是直接使用
公共权重 700。不在 Painter 或 GUI 中通过加粗描边、倾斜矩阵模拟字体样式；这也不表示
禁止原生引擎自身的字体匹配策略。

### 4.2 AddFont

`AddFont(fontFile string) error` 成功表示文件中的字体族可供后续布局匹配，而非仅仅
某个原生注册调用成功。已有布局无需重新匹配；重复注册幂等，失败不破坏既有可用字体。
调用方必须在字体使用期间保留文件，不承诺将全部字体数据复制进内存。

当前注册范围与实现：

- **Pango**：Context 创建独立 Fontconfig 配置，关联自己的 PangoFontMap；添加文件后
  通知字体映射配置变化。PangoFc 入口由 libpangoft2 加载，不修改全局应用字体配置。
- **DirectWrite**：Factory3 分析文件中的全部 face，组合应用字体与已有应用／系统
  font set，完整创建新 collection 后发布。失败保留旧集合；已有原生布局保留旧集合
  所需引用。不使用 GDI 注册代替 DirectWrite 集合，不安装系统字体。
- **CoreText**：使用进程级注册，接受已注册结果；Context 不注销其他布局、Context
  或外部代码仍可能使用的共享注册。不能把进程注册理解为 Context 独占资源。

DirectWrite 的 font set／collection 机制见
[Custom Font Sets](https://learn.microsoft.com/en-us/windows/win32/directwrite/custom-font-sets-win10)。
GOUI 当前选择 Windows 10 的 Factory3 路径，不为注册方便引入 Factory5 或内存字体加载器。

## 5. 范围格式

范围统一使用原文本的 UTF-8 字节偏移，`start` 和 `start+length` 必须是合法 rune
边界，长度为正。后端负责转换 UTF-16 索引，调用者不需要理解 surrogate pair。

支持 `SetTextFont`、`SetTextColor`、`SetUnderline`、`SetStrikethrough`。
后续设置可以覆盖已有范围，不构建公开 span 树或隐式样式继承。

### 5.1 完整字体替换

`SetTextFont` 完整替换指定范围的字体请求，零字段使用默认值，不保留该范围上一次
设置的属性。只想改一个字段时，使用方应保留完整请求：

```go
font := layout.Format().Font
font.Weight = typography.FontWeightBold
layout.SetTextFont(start, length, font)
```

这段示例以基础格式为起点；若 span 已有独立字体，上层应修改自己保存的 span 字体，
不能用 Format 反查范围样式。恢复普通字体也通过再次完整设置完成。

### 5.2 颜色与装饰线

颜色使用标准 Go `color.Color`，`TextColor == nil` 使用 `DefaultTextColor()`，当前为
黑色；Typography 不跟随系统亮暗或主题色，GUI 应传入自身解析后的颜色。
`color.Color.RGBA()` 与输出文字位图均按预乘约定处理；原生需要非预乘分量时只在
后端边界转换。详见 [绘制颜色契约](DesignGraphicsColor.md#文字颜色)。

下划线保留原生单线布尔能力。删除线的设计契约为：

- 依据实际字体 run 的度量和基线，而不是字形墨迹或整行矩形的中点。
- 混合字号／字体可以有不同位置和厚度，颜色与透明度跟随对应文字范围。
- 开关只改变像素，不改变文本、测量、换行或 Cluster。
- 保留分数逻辑坐标与抗锯齿，不强制三端逐像素一致。

Windows／macOS 保留原生绘制。Pango 文字仍由原生引擎绘制，但删除线由 Typography
根据已 shaping 的 run、字体度量、advance、范围与颜色绘制；不重复绘制原生删除线，
不重新 shape 子字符串，也不通过修改无关属性强迫分段。

该选择针对 Pango 1.50.6 的相邻删除线合并行为，参考
[pango-renderer.c](https://github.com/GNOME/pango/blob/1.50.6/pango/pango-renderer.c#L284)。
下划线不因此改成同一套自绘策略，也不扩展波浪线、双线等装饰样式。

## 6. TextLine、Cluster 与基线

`MeasureMetrics` 返回行与 shaping cluster，作为上层光标、命中、选区和视觉导航的
几何依据；不增加另一套文字索引，也不在 Typography 内部保存编辑器光标。

- TextLine 包含原文本范围、行矩形、布局局部基线和本行的 Clusters。
- TextCluster 包含原文本范围、矩形、LineIndex 和已解析的 Direction。
- Cluster 可能包含多个 Unicode code point、UTF-8 字节或定位字形，不等同于单 rune。
- Start／Length 使用逻辑文本顺序；视觉几何及 Direction 保留原生双向排版结果。
- 上层按 Cluster 处理不可分隔的排版单元；不能按字节比例推算文字位置。
- 修改约束、字体或占位后应重新获取度量，不继续依赖旧的几何信息。

Cluster 是排版结果，不声称与 Unicode grapheme segmentation 完全等价。编辑器的
逻辑移动、视觉移动、命令选择与文档索引映射仍由上层负责。

## 7. 行内占位

```go
type PlaceholderMetrics struct {
    Width    float32
    Height   float32
    Baseline float32
}

// TextLayout 的方法。
SetPlaceholder(start int, metrics *PlaceholderMetrics)
```

这里只为已有的 U+FFFC 对象替换字符设置排版空间，不插入或替换文本，不创建、接收、
持有或绘制实际对象。因此公共名称是 Placeholder，而不是 InlineObject。

- start 必须指向 U+FFFC 的 UTF-8 起始位置。
- 三个度量均为有限逻辑单位；Width 非负，Height 为正，Baseline 在 `[0, Height]` 内。
- 布局复制 metrics；调用方可以随后修改或释放自身度量存储。
- `nil` 清除占位设置，恢复该字符的原生排版，不删除字符。
- 占位形成不可拆分 Cluster，参与换行、行高、基线与双向文字排版。
- MeasureMetrics 返回其排版矩形，上层通过 Start 关联实际内容；Rasterize 不绘制
  占位内容、替代字形或调试框。

| 后端 | 排版机制 | 保存的关联数据 |
| --- | --- | --- |
| Pango | AttrShape，空 ink 矩形与指定 logical 矩形 | 原生复制的 shape 度量 |
| DirectWrite | IDWriteInlineObject，提供度量和换行条件，Draw 为空 | C 堆纯数值 COM 对象，原生引用计数释放 |
| CoreText | CTRunDelegate，提供 ascent／descent／width | C 堆纯数值 refCon，由 delegate deallocate 释放 |

原生 `dwrite.SetInlineObject`／`InlineObjectMetrics` 继续保持原名，不反向决定 GOUI
公共命名。参考 [Pango AttrShape](https://docs.gtk.org/Pango/type_func.AttrShape.new.html)、
[DirectWrite InlineObject](https://learn.microsoft.com/en-us/windows/win32/api/dwrite/nn-dwrite-idwriteinlineobject)、
[CoreText RunDelegate](https://developer.apple.com/documentation/coretext/ctrundelegatecallbacks?language=objc)。
Flutter 的 [PlaceholderDimensions](https://api.flutter.dev/flutter/painting/PlaceholderDimensions-class.html)
也区分排版占位与实际 Widget；这里只采用职责区分，不引入其 Span 模型。

未来 RichTextView 可测量子控件、写入占位度量、按结果安排子控件，再走普通控件树
绘制与事件分发；Typography 不接收 Widget、图片或 GUI 绘制回调。

## 8. 栅格化与绘制缓存

`Rasterize(scale float32, buf []byte) (TextBitmap, error)` 将布局输出为调用方拥有的
CPU 位图。scale 必须有限且大于零；空结果可以返回空位图，不代表系统错误。

- TextBitmap 保存 Width、Height、Stride、Pixels，并实现标准 `image.Image`。
- 像素为 8-bit 预乘 RGBA；按 Stride 逐行访问，不假设原生 surface 的格式或存储方向。
- 传入 buf 容量足够时，返回 Pixels 可以与之共享存储。使用方若继续复用该 buf，
  必须自行保留仍需使用的旧结果；Context／TextLayout 不保留返回 slice。
- 后端可复用布局私有 surface／bitmap，但必须将本次结果复制到调用方像素存储。
  连续使用不同倍率不得累计 transform，也不得污染其他布局的结果。
- TextLayout 不缓存某个 scale 的最终 TextBitmap，不持有 graphics.Image、公开
  revision 或通用 AssociatedData 注册表。

Painter 的位置、clip、Transform 与 HiDPI 不进入 Typography。OpenGL／Software
在绘制边界根据设备倍率及变换的最大拉伸计算有效栅格倍率，再将物理像素尺寸换回
布局逻辑尺寸绘制；平移和裁剪本身不改变栅格内容。

OpenGL／Software 的文字 Image 缓存由各 Painter 独立管理，按布局身份与有效倍率
复用，并订阅布局通知。Direct2D 直接绘制原生 DirectWrite layout；其 Rasterize
能力仍可供离屏输出或其他 Painter 使用。缓存容量、淘汰及绘制路径的详细设计见
[绘制缓存设计](DesignPaint.md#31-textlayout-光栅能力与-painter-图片缓存核心收益)。

## 9. 通知、失效与错误

### 9.1 信号与失效

使用方通过 `ConnectChanged(func())` 和 `ConnectDestroy(func())` 订阅通知，拥有并
管理返回的 `signal.Handle`，不为同一变化另设回调 setter。

- Changed 同步在布局所属线程触发，发生于修改方法返回前；无变化的设置不必通知。
- 字体、约束、段落行为和占位变化可能使度量与像素失效；颜色和装饰线只使像素失效。
  当前 Changed 不区分二者，不增加公开 dirty flags 或 revision。
- Destroy 通知只发送一次，在布局标记销毁后、原生资源释放前触发；用于释放外部
  关联缓存。回调不能重新调用布局方法。
- 缓存使用方必须处理布局销毁、Painter 销毁和连接解除，不以清空所有信号掩盖重入
  或访问已销毁对象的问题。

通知实现见 [layout 生命周期 helper](../platform/typography/internal/lifecycle/lifecycle.go)，
Painter 的订阅与缓存管理见 [ImageCache](../platform/graphics/internal/textbitmap/cache.go)。

### 9.2 错误契约

- 原生设施创建、AddFont 注册失败、NewTextLayout 无效基础字体请求及 Rasterize
  失败通过 error 返回，由调用方决定兜底或反馈。
- SetTextFont 的无效字体请求／范围，以及 SetPlaceholder 的无效位置／度量属于
  调用前置条件错误，panic；不以错误输入换取隐含“设置到结尾”等扩展语义。
- 其他范围 setter 同样要求有效非空 UTF-8 范围，但当前历史实现的无效输入处理
  不完全一致，不承诺这些方法都以相同方式 panic 或返回。
- 不支持的方法与参数不通过新增日志、默认错误弹窗或上层坐标补偿伪装成功。

跨层反馈原则见 [错误约定](DesignConventions.md#错误反馈规则)。

## 10. 后端实现中的资源安全

具体实现见 [Pango](../platform/typography/pango/context_linux.go)、
[DirectWrite](../platform/typography/directwrite/context_windows.go)、
[CoreText](../platform/typography/coretext/context_darwin.go)。

- Pango 每个布局持有独立 PangoContext／PangoLayout、Cairo context 和 image surface；
  共享字体映射不能变成共享可变排版／栅格状态。
- DirectWrite 的 factory 在 Context 中共享，IDWriteTextLayout 与离屏 WIC bitmap／
  D2D render target 属于布局。新增原生回调后，输出地址必须在 Go→C→Go 栈增长
  期间稳定；仅对相关查询输出固定内存，不机械替换全部整数／句柄绑定。
- CoreText 每个布局持有 attributed string、CTFrame、CGBitmapContext 与像素工作区。
  CFArray／CFDictionary 必须使用正确 callbacks 保留成员；导出结构体地址与导出
  指针变量的值不能混淆，也不能靠字体代码额外 retain 掩盖绑定错误。

CoreFoundation 集合成员管理依据
[Apple 集合创建说明](https://developer.apple.com/library/archive/documentation/CoreFoundation/Conceptual/CFCollections/Articles/creating.html)。
具体 Const 值／地址输出的修复过程留在实施记录，不作为另一套 Typography API。

## 11. 测试要求与当前差异

测试组织遵循 [Testing](Testing.md)。无需桌面的原生排版／离屏测试留在包内；真实
窗口、显示连接、桌面合成、输入法或 GPU 呈现另行验收，不为每项排版测试创建窗口。

必要覆盖包括：

- 实际字体匹配、Regular／Bold／Italic、完整范围替换、默认值、无效请求与布局隔离。
- 字体注册失败／重复、多 face 文件、旧布局引用和失败后的资源可用性。
- UTF-8／UTF-16、emoji／组合字符、空行／纯空白、末尾换行、RTL 与宽度往返。
- 混合字号删除线的实际字体度量、范围颜色／alpha、开关前后度量不变。
- 占位矩形／基线、相邻占位不合并、换行、更新／清除、零宽及资源重复释放回归。
- 预乘 RGBA、1x／2x／分数栅格倍率、连续变换、调用方缓冲复用和多布局隔离。
- CF／COM／Pango 属性 ABI、成员保留和原生回调期间的存储稳定性。

像素断言使用有依据的采样和容差；字体实际匹配不能仅比较两张可能都使用 fallback
的图片。字体夹具在原生注册使用期间保持有效，不用跨平台统一金图掩盖字体差异。

当前应明确区分的限制与待复核项：

- TextAlignBegin／End 的公共意图是段落起止边；DirectWrite 当前映射 leading／trailing，
  Pango／CoreText 使用原生 left／right 枚举。原生自动方向处理与段落基方向也会影响
  结果；仅凭枚举映射不能认定缺陷已复现，RTL 对齐需专项核对，不能宣称三端一致。
- 原生段落基方向、font fallback、hinting 和抗锯齿允许差异；尾部中性字符不强制
  服从另一后端的方向策略。
- 墨迹外伸边界、不同系统 DPI 和硬件 GPU 仍需独立验证；不由本文补出 InkBounds
  或位图 Origin API，也不把当前排版矩形当作完整墨迹边界承诺。
- 所有范围 setter 的无效输入防护尚未统一；这与合法输入下的格式语义分开记录。

具体通过／失败／跳过、平台版本、既有 Linux arm64 构建限制和未执行场景见
[实施与验收记录](PlanTextLayout.md)，不得用本文的设计要求替代测试结果。

## 12. 后续能力边界

当前不提供行距自定义、上／下标、OpenType features、竖排、复杂文字背景、富文本
文档模型或 RichTextView。它们是按真实应用需求评估的后续方向，不是本文隐含承诺。

布局私有工作区的内存优化不能恢复共享可变 textPainter；Measure／Rasterize 的
性能结论必须来自同环境、后端与规模的对照。未来范围和验证缺口统一记录在
[Roadmap](Roadmap.md#textlayout-富文本基础本轮已实现应用层按需求推进)，不重新扩张本轮切片。

## 13. 可配置 Tab 间隔（2026-10-16）

TextFormat.TabWidth 与 TextLayout.SetTabWidth 使用 DIP；0 恢复后端默认。
正值必须有限且不超过 1048575，以保留两个 Pango-unit 站点的 int32 表示范围。
NewTextLayout 遇到非法值返回 error，setter 遇到非法值 panic；有效改变发 Changed，度量与 raster 缓存均失效。
Format 返回当前间隔；文本与 UTF-8 range 不改写，Tab 保持原生 cluster。

Pango 原生绑定只公开 TabArrayNew / SetTab / Free 与 Layout.SetTabs；创建两个左对齐站点的策略与临时资源所有权在 typography/pango。布局拷贝数组后释放临时资源；DirectWrite 在布局对象设置 repeating interval，
并保存原生默认值供 0 恢复；CoreText 同时清空显式 TabStops 并设置 DefaultTabInterval，
0 则使用未覆盖的默认段落样式。资源所有权沿用既有 native layout。

Pango / DirectWrite / CoreText 的 TestTabWidth 已分别在 Linux / Windows / macOS 实际执行，验证重复站点、连续 Tab、通知、恢复默认、颜色后的 Tab 坐标及非法创建参数。环境、全仓结果与既有失败见 [通用文本扩展验收](PlanSourceViewExtensions.md)；RTL / 折行 / 跨屏仍待专项验收。

[Pango Layout.set_tabs](https://docs.gtk.org/Pango/method.Layout.set_tabs.html) 明确数组会被复制。重复站点采用两个末尾站点的间距，依据 [Pango 1.50.6 的 get_tab_pos 源码](https://github.com/GNOME/pango/blob/1.50.6/pango/pango-layout.c) 及本机该版本实测；未引入需要更高版本的对齐类型。


## 14. 完整前景覆盖的记录回收（2026-10-16 工作副本）

长寿命 TextLayout 可反复进行范围着色。DirectWrite 后端在 SetTextColor 完整覆盖文本时，
丢弃已经被覆盖的全部旧颜色范围，再保存本次范围；部分范围仍遵循既有覆盖/绘制顺序。
这保持外部颜色语义及 UTF-8→UTF-16 转换，避免完整重着色永久累积无效记录。
不由调用方按后端定期重建布局来补偿资源增长，也不新增 API。

`TestFullColorReplacementDropsObsoleteRanges` 在 Windows 10 19045 / Go 1.24.0 原生执行：
修复前第二轮已有 4 条颜色记录而失败；修复后 1000 轮保持完整前景与局部覆盖的 2 条，
同时检查多字节源范围转换。DirectWrite 所属包全仓测试通过；本机 GOUI 全仓测试通过。
本修正不承诺任意局部更新序列都已合并为最小区间，原生内部 shaping/raster 工作仍由后端决定。
