# Android Typography

状态：2026-11-04，基础包装已实现并通过 API 30 ARM64 验收；WrapChar、完整共同
测试运行器及平台工厂接入仍待完成，不表示完整移动 Typography 已交付。前置实验见
[DesignMobilePlatform](DesignMobilePlatform.md#95-最小交互单元实测2026-11-04)。

## 边界

新增 typography/androidtext，实现既有 Context/TextLayout；不增加 GUI 命中接口。
JNI 绑定在 android/libs/jni，Bitmap NDK 绑定在 android/libs/bitmap。Android
启动支撑管理 JavaVM、应用 ClassLoader 和固定 GOUI 线程的 attach/detach，
不让某个排版 Context detach 其他对象仍在使用的线程。

Java helper 是 Android 排版适配，不是 runtime/cgo 基座；维护者编译为 Dex，
消费端只复用，无需 Java/NDK。原生 host 仅提供进程 JavaVM/ClassLoader，不
包含文字方法。Go/汇编调用仍复用 goexlib/cgo，不在平台层复制 ABI 实现。

## 对象与数据

Context 拥有独立字体集合和 Java peer，每份 TextLayout 拥有独立 Spannable、
TextPaint、StaticLayout、Bitmap/Canvas。Go 保留原文、公开格式/尺寸、UTF-8/
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
不接入仍待迁移的 platform.NewTypography，不扩大成另一套移动 Platform。

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

WrapChar 创建返回错误、setter 明确失败；没有用 SIMPLE 冒充字符换行。不
启用该后端为完整 GUI 工厂，直到此契约及平台接入确定。共同 typographytest
尚未通过 Android 的标准 Go 测试运行器执行，本轮是独立的原生窗口验收程序。
API 29/31+、真机、16 KiB、长时间内存/性能、系统字体缩放策略、移动 GPU 及
组件分发仍待验收，不将单版本通过推广为全部设备通过。

参考：[JNI](https://developer.android.com/ndk/guides/jni-tips)、
[StaticLayout](https://developer.android.com/reference/android/text/StaticLayout)、
[LineBreaker](https://developer.android.com/reference/android/graphics/text/LineBreaker)、
[Bitmap NDK](https://developer.android.com/ndk/reference/group/bitmap)。
