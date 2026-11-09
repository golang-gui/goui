# Android Typography

状态：2026-11-09，Android Layout 已通过平台工厂接入，API 29+，支持 Software/GLES
消费者。本轮按原生 SDK、GOUI 批量适配和 Go 资源归属整理；下方旧版本验收记录
保留其原范围。WrapChar、真机与更多系统版本仍未完成。

## 边界

新增 typography/androidtext，实现既有 Context/TextLayout；不增加 GUI 命中接口。
JNI 绑定在 android/libs/jni，Bitmap NDK 绑定在 android/libs/bitmap。Android
启动支撑管理 JavaVM、应用 ClassLoader 和固定 GOUI 线程的 attach/detach，
不让某个排版 Context detach 其他对象仍在使用的线程。

Java helper 是 Android 排版适配，不是 runtime/cgo 基座；维护者编译为 Dex，
消费端只复用，无需 Java/NDK。原生 host 仅提供进程 JavaVM/ClassLoader，不
包含文字方法。Go/汇编调用仍复用 goexlib/cgo，不在平台层复制 ABI 实现。

## 对象与数据

Context 拥有独立字体集合，不再持有 Java TextContext。每份 TextLayout 拥有独立
Java LayoutEngine、Spannable、TextPaint、StaticLayout、Bitmap/Canvas。Go 保留原文、公开格式/尺寸、UTF-8/
UTF-16 映射、度量缓存与 lifecycle.Layout；Java 保存对应原生状态。setter 成功
后同步失效、通知，连续设置不重复排版。颜色/装饰不重建度量；字体、尺寸、
Tab、占位和换行使布局失效。Destroy 主动释放全局引用和位图，不靠终结器。

交互边界采用公开 ICU 字素分段，同一 Layout 提供几何。批量返回数组，不在
每个字素跨 JNI，不使用实验 JSON 协议。原文始终不变；零宽范围保留，硬换行
不伪造有宽度单元。字体范围不能把字素自动拆开。

Rasterize 直接绘制请求的物理区域。清空可复用软件 Bitmap、平移 region、缩放、
Layout.draw，再经 AndroidBitmap_lockPixels 逐行复制到 buf 并解锁。使用预乘
RGBA，原生 ARGB 非预乘颜色只在边界转换。scale 不改变排版；负原点及透明
空白遵守既有契约。每份布局保有自己的可变绘制资源。

## 必须单独验收的映射

- StaticLayout、TabStopSpan 和 ReplacementSpan 含整数 API。验证统一内部
  子像素单位，显式记录量化与上限；不各自取整、混入设备 density 或 fontScale。
- 字号沿 FontInfo 的 point 语义映射到 Android 逻辑单位，设备缩放仅在 raster
  阶段出现；不通过 SP 自动重复应用系统字体缩放。
- WrapWordChar 使用公开原生简单换行，禁用自动连字符；WrapNone 保留原文
  硬换行，不拿布局高度裁掉逻辑文本。WrapChar 没有直接对应的公开策略，须
  专项验证；不能将 SIMPLE 当成字符换行，也不修改原文插入隐藏字符。
- AddFont 保留族名语义，支持 TTF/TTC 与系统 fallback，注册失败不能污染旧
  集合。字体样式尽量请求原生实际 face，不通过画笔倾斜或横向拉伸模拟。
- 创建、AddFont、Rasterize 返回错误。没有 error 返回值的方法若遇到不可恢复
  原生失败，必须明确失败，不能提供旧度量/空图片伪装成功。

## 实施与验收

1. JNI/启动支撑、基础 Context/TextLayout、度量与 Rasterize。
2. 字体、范围格式、Tab、占位、对齐、小数单位和资源失效。
3. 换行专项、共同 Typography 回归、18 组交互对照、缓存/分配观测。

以 Android 实际执行为准。先验证 API 30 ARM64，再扩充版本/真机；未完成项
记录在 Plan/Roadmap，不将部分可运行后端描述为完整支持。所有产物在仓库外，
不改主工作区或 FFI 已验证分支，不修改用户签名/安全配置。

## 当前实现

入口为 androidtext.NewContext，名称为 Android Layout。当前支持 WrapNone/
WrapWordChar、开始/结束/居中/两端对齐、TTF/TTC 族匹配及原生 fallback、完整
字体范围替换、颜色/装饰、重复 Tab、占位、Changed/Destroy 与区域栅格化。
通过 platform.NewTypography 接入，沿用已有移动 Platform，不扩大为另一套宿主。

内部使用 64 个原生单位对应一个逻辑单位：宽度向内取整，Tab/占位宽度按该
网格量化，行高/基线上取整保证容纳内容。量化步长 1/64 DIP，最终还受 float32
精度限制。字号采用现有 Windows/Pango 的 96/72 逻辑单位/pt 约定；density
只进入窗口/raster scale，fontScale 不在后端再次自动应用。不能把这个约定
称为 Android TypedValue.PT 的物理英寸换算。原生坐标不能表示的请求明确失败。

每个硬段落保存原生 Layout；单份原文/UTF-16 索引不变。度量为批量原始数组，
不逐字素跨 JNI；Go 缓存原生结果，MeasureMetrics 返回独立切片，不能因缓存
让调用方修改污染后续度量。字体/方法标识可共享，可变 TextPaint、Spannable、
Bitmap/Canvas 均按布局独立。更换/销毁 Bitmap 前先解除 Canvas 绑定并 recycle；
Typeface/Font 没有这里使用的显式释放接口，移除强引用后的最终释放由系统管理。

Tab 只生成实际需要的原生 stop，不按窗口最大宽度铺设成千上万个 span。对齐
后的距离从本行实际原点计算，不把居中/靠右的位移计入 Tab 间隔。

### 两端对齐的兼容处理

API 30 实测发现直接启用原生 justification 后，画面与光标/选区不一致。
[Android 11 Layout.getHorizontal](https://github.com/aosp-mirror/platform_frameworks_base/blob/android-11.0.0_r1/core/java/android/text/Layout.java#L1103)
没有应用 getLineExtent/draw 所用的 justify，getSelectionPath 又依赖该路径。
首轮专项失败保留，不能使用不自洽的 Cluster，也不能在 Go 侧偏移来补画面。

实现统一改用公开 TextPaint.wordSpacing 的 MetricAffectingSpan，原文不变、
每行的测量和实际绘制读取同一状态。扩展词间距在原生细网格向下量化以保持
拟合，重建后逐行核对换行边界，改变则明确失败。末行不扩展，带 Tab 的行
保留 stop 布局。此路径不访问私有 TextLine/Minikin，也不按版本号盲目分支。
独立 Canvas.drawText + wordSpacing 的像素及行末几何均验证通过。

### 验证记录

- Linux Go 1.25.8 制作 Java/Dex、测试包装与当前宿主；无原生编译器消费 APK
  通过，消费原生编译器调用为 0。Java 属于独立组件，runtime/cgo/entry 未扩大。
- Mac ARM64 上专用 Android 11/API 30 ARM64 ATD，Software 1x、CheckJNI：
  170 项后端检查通过，4 组原生行/光标参考及 15 张完整 RGBA 位图精确一致。
  包含 TTC 与各 TTF 的三种实际 face、跨样式字素、对齐 Tab、占位、切片隔离、
  缓冲复用、半透明颜色及资源通知。CheckJNI 没有检测错误或 native fatal。
- 原有 36 项基础检查和 18 组/3814 项交互检查继续通过；9 项 shaping 等价
  差异仍保留，不把它们算成交互失败或改为等价成功。
- 旋转至 800×480、恢复 480×800、后台恢复，帧数从 1 递增至 3/5/6；进程
  与 GOUI 线程不变，Activity 重建没有使排版引用失效。实际画面通过新包装的
  DrawTextLayout 绘制，多语言与彩色 emoji 已截图检查。
- 缓存命中 MeasureMetrics 的本轮观测为每次 2 个 Go 分配、约 257 ns（10000
  次平均），用于独立结果切片；没有重新跨 JNI。不同运行的耗时有波动，这不是
  跨引擎、真机或长期内存性能结论。
- 隔离已跟踪源码副本，仅替换到 FFI 分支，GOWORK=off go test ./...：3509
  通过、10 项既有跳过、0 失败；相关 Linux race/vet 通过。直接在原工作区带
  GOWORK 运行仍因四个未跟踪旧例程及 cmd/goui 临时模块失败，日志保留。
- Android 目标 vet **未通过**：新 JNI 表访问 2 处、锁定 Bitmap 像素 1 处和
  既有 mobile 回调包 1 处报告 possible misuse of unsafe.Pointer。已核对这些
  地址均来自原生对象，JNI 表在 attached 线程有效，Bitmap 在局部引用及 lock/
  unlock 范围内复制；没有 Go 堆地址脱离 pin 后保留。NDK 头文件静态断言和
  CheckJNI 均通过；不以位重解释或关闭分析器隐藏这些静态诊断。

原始实验在 Linux/Mac 的 /tmp/goui-android-typography-1kdxtqrn，包含首轮断言、
原生 justification 失败及最终报告、截图/生命周期、构建日志和收据。最终 APK
SHA256：8a8fa6a7690d0550e371b08702d10a54cb00809a741a6adf4e436541897bb7af。
首轮半透明测试错误地对装饰叠加应用单次覆盖 alpha 上限，已改为分别独立
验证纯文字和装饰的原生像素，没有放宽颜色容差。第三轮旧 runtime-result
属于上一次运行，其专项失败以 backend-go.json/退出码为准；最终运行已先清除
旧收据，不能用残留文件判定成功。专用模拟器/ADB 已停止，旋转设置已复位。

### 剩余范围

WrapChar 创建返回错误、setter 明确失败；没有用 SIMPLE 冒充字符换行。平台工厂
已接入，不代表移动 GUI 能力已经完整。共同 typographytest
尚未通过 Android 的标准 Go 测试运行器执行，本轮是独立的原生窗口验收程序。
API 29/31+、真机、16 KiB、长时间内存/性能、系统字体缩放策略、移动 GPU 及
组件分发仍待验收，不将单版本通过推广为全部设备通过。

参考：[JNI](https://developer.android.com/ndk/guides/jni-tips)、
[StaticLayout](https://developer.android.com/reference/android/text/StaticLayout)、
[LineBreaker](https://developer.android.com/reference/android/graphics/text/LineBreaker)、
[Bitmap NDK](https://developer.android.com/ndk/reference/group/bitmap)。


## 2026-11-09：三批职责整理

本轮不以消除 Java 为目标。遵循 Android SDK/JNI 规范，原生类绑定与 GOUI 排版
适配分开；保留原生批量循环，避免每个字素/段落跨 JNI。公共 Typography 接口不变。

1. **SDK/JNI。** `android/sdk` 的 text/fonts/graphics 按原生类声明类型和方法；InitText
   独立按需初始化、进程级保留 Class，JNIEnv 显式传入。实例/静态方法分别绑定，
   复用既有 SDK 错误边界。JNI 增加 jboolean、静态对象字段等必要原语；不反向依赖
   androidruntime 或 Typography。SDK 不绑定 org.golanggui 的自定义类。
2. **字体与布局。** Go Context 拥有字体注册入口，fontCollection 保存原生 Font/Typeface
   的拥有引用与匹配缓存，由 Context 和存活 Layout 共同持有。每个 Layout 独立持有
   原生可变状态；Context.Destroy 只结束自身使用权，不关闭已有 Layout 的字体设施。
   私有 bridge_android.go 统一 GOUI Java 类初始化、ABI 和批量结果解析；class/method
   元数据只读共享，实例与 JNIEnv 不共享。SDK 绑定和组件桥接不能混在一起。
3. **栅格化与组件。** 独立 raster_android.go 管理每个 Layout 的 Bitmap/Canvas 缓存、
   请求区域、缩放、清空与 NDK 像素复制。先解除 Canvas 对旧 Bitmap 的使用再回收；
   不重排已有效的 Layout。生成器按 LayoutEngine、TextSpans、LayoutMetrics 分开
   Java 职责；批量段落、Tab、两端对齐和 ICU/Cluster 几何仍在 Java。它们接收原生
   单位和 Typeface，不管理字体集合、DIP 或最终位图缓存。

Go 保留 64 原生单位/DIP 与字号 96/72 的转换；批量结果为 UTF-16 范围和原生坐标，
经具名 nativeMetrics 统一解码，再映射 UTF-8 和公开 Typography 类型。SDK 标准类
通过 JNI 直接调用，必要的 Span 子类保留 Java，不引入 Go↔Java 的逐字回调系统。

Java 为维护者生成的 Dex 组件，调用方不需要 Java/NDK。移动运行时基座不变；组件
ABI、源码和内置 Android target 必须一起更新。维护者可使用 mobileassets 生成器的
`-android-dex` 模式，仅替换已验证 Dex，核验原有 target 文件并更新摘要，不重制
其他平台和 Go 版本的 native-support 对象。

### 实施与验证进度

- [x] 第一批 SDK/JNI，实际 ART/CheckJNI 的 StaticLayout、字体、boolean、常量和
  预乘 Bitmap 像素检查通过；提交 1495c7a。
- [x] 第二批字体/Layout：Context 销毁后继续修改已有 Layout 的回归与多 Layout 隔离，
  中间版本单独 APK/ART 验证通过；提交 d508908。
- [x] 第三批栅格缓存、Java/Dex ABI 3 同步及正式默认组件构建。旧 ABI 2 Dex 与新
  Go 混用会明确返回组件 ABI 错误，消费并清除 JNI 异常；提交 d05480a。
- [x] 既有独立原生参考、两后端生命周期、本机全仓/race、适用目标编译及同环境耗时对照。

### 本轮验证结果

- **Android 原生通过：** 远端 macOS15.7.7 ARM64 的专用 Android11/API30 ARM64
  AOSP ATD，480×800、160dpi/1x，SwiftShader GLES3，Go1.25.8、CheckJNI。
  `android_bindings` 的 SDK 专项通过；最终 `text` 连续三轮通过，含 Context 先
  销毁、两个 Layout 的字体/颜色隔离、1x/2x/1x 栅格化和重复销毁。旧实现同一
  生命周期检查确实报 TextContext is closed，不能用进程存活代替这项回归。
- **独立参考通过：** `android_text` 170 项后端检查，4 组行/光标与 15 张完整 RGBA
  参考包含 1x/1.5x/2x、TTF/TTC、装饰与半透明；基础 54 项及18组/3814项交互
  通过。历史9项 shaping 等价差异继续保留，不算成交互失败，也不宣称等价成功。
- **两后端通过：** `mobile_dispatch` Software/GLES 分别四次旋转、两次后台停绘/
  恢复与 UI 回调最终释放；nonce/UI线程不变、交付始终一次，后台帧数不增长。
  `android_rendering` 的多 Layout 隔离、重复 EGLSurface 创建失败回滚与离屏像素
  继续通过；无 CheckJNI warning/error。实际 GPU context loss 不在此范围内。
- **本机通过：** Linux amd64/Go1.25.8，已跟踪源码和本轮变更的独立副本，在
  Xvnc :190/Xfwm4 会话运行 `CGO_ENABLED=0 go test -json -count=1 ./...`：4558项
  通过、11项按原条件跳过、0失败；JNI/NDK、androidtext 与 mobile_build 的 race
  通过。后续仅调整独立参考的旧测试符号，相关包测试和 Android 实跑再次通过。
  Windows amd64/macOS arm64 的 platform/gui/ui/widgets/demo 交叉编译通过。
  它们不是 Windows/macOS 桌面操作验收，本轮未修改这两个后端。
- **原工作区失败：** `go test ./...` 仍被开始时已存在的未跟踪旧例程/测试阻断：
  example/plat_test、transparency、window_chrome、platform/test、旧 image_lifetime/
  drag_end/dragdrop，以及 ui/window_lookup_test 的旧接口。独立副本不含这些文件；
  没有删除它们、扩大 skip 或把原工作区结果写成通过。
- **测试维护：** 原独立参考仍查已删除的 goui_android_java_vm，首次报告167项、
  JNI子项失败；改用测试包装自己保留的 NativeActivity JavaVM 后170项全过。
  mobile_build 的旧归档日志断言也不识别现有 format2 recipe；现分格式校验入口，
  format2 另要求 runtime/cgo，始终拒绝 cgo 编译器，并使用测试自有冷缓存取证。
  保留负向单测，不恢复生产测试符号或放宽原生像素断言。

同环境耗时对照使用 `android_bindings` 的固定八段多语言文本，40次冷构造/度量/
销毁、1000次缓存度量、各20次1x/2x预热后栅格化；两版各3轮取中位数：

| 范围 | 整理前 | 最终版本 |
| --- | ---: | ---: |
| 创建+首次度量+销毁 | 2.996 ms/op | 2.896 ms/op |
| 命中缓存的度量 | 1.902 µs/op | 1.556 µs/op |
| 1x 区域栅格化 | 151.152 µs/op | 140.506 µs/op |
| 2x 区域栅格化 | 246.650 µs/op | 252.577 µs/op |

Context 创建不在计时中，栅格化复用调用方缓冲；各轮有明显波动，样本不足以给出
统计显著的提速结论，没有显示数量级退化。它不是帧率、真机或长时间内存测量。
本轮收益是明确绑定、批量算法与资源归属，不以 Java 文件数量作为性能依据。

材料位于两端仓库外 `/tmp/goui-android-text-refactor-05rmxhoz`，含分批APK、JSON、
完整日志、独立参考图像、构建与失败记录。本轮只使用专用只读AVD/ADB端口，保持
共享设备、个人签名与 SSH 不变；已卸载本轮四个测试应用，停止专用模拟器与ADB。
API29/31+、真机、16KiB、长期内存、标准 Android
Go测试运行器仍未执行；WrapChar 与 PlanMobileAPI 第10节既有全NULL EGL查询问题
均未扩为本批任务。

源码依据与原生契约：[StaticLayout.Builder](https://developer.android.com/reference/android/text/StaticLayout.Builder)、
[ReplacementSpan](https://developer.android.com/reference/android/text/style/ReplacementSpan)、
[Canvas](https://developer.android.com/reference/android/graphics/Canvas)、
[JNI tips](https://developer.android.com/ndk/guides/jni-tips)。Builder 构建后不复用；
JNI 参数/返回与引用有效期按原生 API 保留。性能结论必须区分首次构造、缓存读取和
栅格化，不以绑定层减少 Java 文件数作为性能证据。
