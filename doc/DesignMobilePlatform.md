# 移动端平台、绘图与文字排版

状态：2026-11-04 更新的实施计划。基座、启动、逻辑窗口及 Software 已完成的
范围见 [PlanMobileBuild](PlanMobileBuild.md) 与
[DesignMobileLifecycle](DesignMobileLifecycle.md)。本篇规定后续结构与顺序，
不表示完整目录迁移或全部移动能力已经实现。两端排版/绘图的最小切片见第 10 节；
它不等同于完整 Platform 工厂、GUI/UI 或真机支持。Android 文字基础包装已实现，
当前限制与验收见 [DesignAndroidTypography](DesignAndroidTypography.md)。

## 1. 目标与边界

应用只编写普通 Go，通过 goui 工具构建 Android/iOS。消费端不安装或配置
原生编译链；维护者可以预编译必要的 C、Objective-C、Java 和绘图组件。
复用 stock Go 编译器/runtime、已有链接驱动及 FFI，不重新实现 GC、线程或 TLS。

内部允许 CGO_ENABLED=1。最小 runtime/cgo 基座、FFI 归档、固定 ABI 入口、
平台宿主、绘图/排版组件和打包工具分别归属、版本化及校验。普通 Go 应用、
platform、gui、ui 从源码编译；平台 UI 变化不能扩大通用 runtime 基座。

platform 报告原生事实；GUI 状态与策略归 gui；ui 只协调已有 Widget。
第一批目标为 Android ARM64、iOS ARM64，模拟器使用独立组件身份。当前已验证
Android 11 ARM64 ATD、iOS ARM64 Simulator，不以链接成功代替真机运行。

## 2. 平台包结构

按应用环境、公开原生 API 与可复用基础设施分组，不仅按内核血缘归类：

```text
platform/
  common/                       公共接口和值类型
  events/                       原生事实事件
  mobile/                       main 注册与固定 ABI 启动入口
  internal/mobile/              共享队列、逻辑窗口及绘制许可
  windows/{win32,sdk}/
  linux/
    x11/
    wayland/                    后续按需实现，与 x11 平级
    libs/
  darwin/
    cocoa/                      既有 macOS AppKit 后端
    ios/                        iOS 平台后端，包名 ios
      native/                   必需的 Objective-C 宿主
    frameworks/
      core_foundation/
      core_graphics/
      core_text/
      foundation/
      appkit/
      uikit/
      quartzcore/
  android/                      Android 平台后端
    libs/                       按公开原生 API 组织绑定
    native/                     必需的 NativeActivity/JNI 支持
  graphics/
    software/
    opengl/                     桌面 GL 与移动 GLES Painter
    direct2d/
  typography/
    coretext/                   macOS 与 iOS 共用
    directwrite/
    pango/
    androidtext/                Android 系统文字适配，基础包装已实现
```

这是一份目标结构，不预先创建空包。iOS 的 UIKit 绑定是 frameworks/uikit，
平台适配是 darwin/ios；不将平台包命名为 uikit。Apple 共享 CF/CG/CT 和
Objective-C 支持，所以归于 darwin；Android 的 Bionic、Activity、JNI、生命周期
不同于 Linux 桌面，独立于 linux/x11 与未来的 wayland。

当前 platform/mobile 中的共享 Go 窗口/队列移至 internal/mobile；Android 与
iOS 的宿主分别回到对应平台。mobile 保留构建工具生成文件使用的 RegisterMain
和最小固定入口，不再成为应用需要直接选择的另一套平台 API。正式使用仍从
platform.NewPlatform/NewWindow/NewPainter/NewTypography 进入。

## 3. 构建选择与初始化

Go 的 Android 同时匹配 linux 文件，iOS 同时匹配 darwin 文件。平台工厂、GLX、
AppKit/NSGL 等桌面入口显式排除移动目标；CF/CG/CT 等共享实现保留。DefaultName
增加对应事实，错误/不支持行为沿用 common，不把移动后端伪装为 DesktopWindow。

frameworks.Init 当前会加载 AppKit 与桌面 OpenGL；CoreText.NewContext 不得在
iOS 通过这个总入口加载桌面依赖。明确每个后端所需初始化子集；保持初始化
幂等，不为解决 iOS 而改变 macOS 已验证的字体、窗口及绘制行为。

main 注册、固定 goui_run ABI、GOUI 固定 OS 线程和原生系统主线程分工不变。
移动端首次创建逻辑 Window 不等待 surface；Activity/Scene 和目标替换不重复
执行 main、不卸载 Widget 树。同步撤销绘制许可不得反向等待系统主线程。

## 4. 两端共用 OpenGL ES

首版 Android 与 iOS 共同适配 GLES，复用 graphics/opengl 和 NanoVGo。Android
使用 EGL/ANativeWindow，iOS 使用 EAGLContext/CAEAGLLayer；不把 EGL 强套到 iOS。
Metal 是后续路线，不作为当前移动端接入的前置条件。

Android 的 EGL/GLES 是公开系统能力。iOS GLES 已被 Apple 弃用，作为首版明确
接受的兼容选择；支持范围以实际 SDK、模拟器和真机验证为准，不承诺长期可用。
NanoVGo v0.0.10 虽有移动 shader，却混用 GLSL ES 100 与强制加载的 GL3 函数；
不能只补上下文便宣称完成。第一份 GPU 验证优先固定 GLES 3.0，核对 shader、
VAO、纹理格式、stencil、ClearPath、透明合成与离屏输出。是否支持 ES2 另行按
真实需求设计，不默默改变最低设备要求，也不因 GPU 不可用隐瞒错误。
本轮独立 NanoVGo 分支已完成这组修正，实际覆盖见第 10 节。

绘图资源归 Painter，窗口/原生视图生命周期归平台后端。GPU 许可保护绑定和
提交等原生目标访问；表面替换尽量保留 context 与图片，context 真正丢失时明确
报告和重建。graphics.Image 不保留原始源图，不能承诺任意资源自动恢复。
Software 保持确定性参考和独立可选后端。

不在本轮引入 Skia 二维绘制后端。C++ 可通过维护者预编译加小型 C ABI 接入，
但不意味着它比复用 NanoVGo 更合适。后续以功能缺口、维护、包体和实测性能
决定，不能假定 Android 内部的 Skia 是应用可调用的公共动态库。

## 5. iOS 文字：复用 CoreText

复用 typography/coretext，解除其 AppKit/桌面 OpenGL 初始化依赖。核对 iOS
实际可用符号、系统默认字体、字体注册、沙箱字体路径、位图朝向及 HiDPI。
栅格化继续使用离屏 CoreGraphics，输出现有预乘 RGBA TextBitmap；每个
TextLayout 保持独立可变资源。GPU 上传缓存继续复用当前 textbitmap 能力。
不新增与现有 TextLayout 平行的 iOS 排版接口，不混淆系统字体缩放与 raster scale。

## 6. Android 文字：先验证系统公开接口

首选验证 JNI 调用 StaticLayout、TextPaint、Typeface、Spanned、Canvas/Bitmap。
只调用公开 Java/NDK API，不能通过 dlopen/dlsym 依赖系统私有 HarfBuzz、Skia、
Minikin 或 libandroid_runtime 符号。系统使用某库不等于向应用提供稳定 ABI，
Skia 还可能以静态库并入系统模块。

NDK AFontMatcher/AFont 提供字体匹配和字体信息，不是完整段落排版。HarfBuzz
负责 shaping，不独立提供 bidi、断行、字体回退和字形位图。若公开系统 API
无法满足契约，再评估自带 HarfBuzz/其他排版与栅格化组件；它们属于排版后端，
不是 runtime 基座。当前不提前引入完整 Skia 或自研文字引擎。

### 6.1 必须满足的现有契约

- 文字不可变，布局尺寸及范围格式可变；UTF-8 字节范围与 Java UTF-16 索引
  明确转换。Cluster 的语义为最小文本交互单元，遵循 DesignTypography 第 6 节，
  不能将 glyph ID 或 code unit 直接当作交互单元。
- MeasureMetrics 返回行、baseline、方向、Cluster 的文字范围与几何，和实际
  Rasterize 使用的同一布局一致；不能用每字符宽度猜测复杂文字几何。
- 支持 wrapping/alignment、Tab、颜色/字体/下划线/删除线与占位 metrics。
- Rasterize(scale, region, buf) 不改变 shaping/布局，精确返回指定像素区域，
  含负原点、透明空白和可复用输出缓冲，输出预乘 RGBA。
- 每个 TextLayout 独立持有可变对象。JNIEnv 仅所属线程使用；Go GUI 线程通过
  JavaVM 正式 attach，必要时 detach；局部/全局引用、Bitmap 和缓冲归属明确。
  不在每个字符或每帧重复创建 JNI 排版对象。
- 明确 minimum API、字体/DIP 与系统 fontScale 的转换，不因引入新接口悄悄
  提高最低版本。当前原生窗口验收 APK 的 minSdkVersion 为 29。

### 6.2 本轮最小验证

先使用独立 tests/window/platform/android_text 程序与维护者脚本，不修改生产
排版实现或公共 TextLayout。在正式 mobile Window/Software 上显示系统生成的
位图；JNI 调用从 GOUI 线程发出，不用主线程原生 TextView 替代 Go 接入。
测试所需 helper 可以由维护者预编译，消费端复用原生对象/Dex，不要求 javac/NDK。

验收分开记录通过、能力缺口及未执行：

| 项目 | 独立预期/检查 |
| --- | --- |
| 启动/线程 | 正常 APK、GOUI 固定线程经 JNI 调用；不在 Android UI 主线程运行排版 |
| 索引/Cluster | 验证原生交互边界、逻辑范围、光标/选择与画面一致；HarfBuzz 只用于区分 shaping 与编辑分组的对照，不要求分组等价；覆盖代理对、组合音标、连字、RTL、印度文字、ZWJ emoji |
| 行与 bidi | 文本范围覆盖、方向与 baseline；混合 RTL 运行的视觉位置，换行及末尾空行；不以内存中的任意 RTL 标志代替系统度量 |
| 范围格式/占位/Tab | 确定颜色及占位宽高/baseline；格式改变不能污染其他 Layout；Tab 的位置由明确间隔决定 |
| 区域栅格化 | 直接绘制与平移/clip 后区域逐像素相等；负原点/空白透明；1x/2x 不改变布局度量；半透明像素满足预乘约束 |
| 实际画面 | 原生截图检查文字、色块和布局；颜色采样避开抗锯齿，字体形状不使用跨平台统一金图 |
| 构建 | Go 应用使用现有基座与预编译宿主/helper 消费，原生编译器调用为零；维护者制作步骤单独记录 |

公开 cursor API 返回的合法编辑位置不一定就是 shaping cluster。glyph 数量、
字形位置或零 advance 也不能单独证明原始文字到 glyph 的对应关系。若存在
无法满足的交互几何，应记录失败样本和 API 缺口，不访问私有 API，也不把原生
交互粒度与 shaping 粒度的合理差异当作失败。
这轮能力验证通过也不等于完整 Android TextLayout 后端已经实现。

## 7. 实施顺序

2026-11-04 最新执行顺序：Android 文字与 GUI 启动切片完成后，暂停 GUI 扩展，
先完成两端 Typography/Graphics 的平台层最小验证。先接 iOS CoreText/Software，
再适配 GLES 3 与 Android EGL/iOS EAGL；全部通过后再实施
[触摸设计](DesignTouchInput.md) 与后续 IME。以下列表保留总体能力依赖，不表示
GUI 输入先于平台绘制验证。

1. Android 的公开系统文字路线和正式 TextLayout 包装，保留能力边界。
2. iOS CoreText 解除桌面初始化依赖，先用 Software 验证相同 API 与位图契约。
3. GLES 3 NanoVGo、Android EGL 与 iOS EAGL；验证绘图、目标替换和后台保护。
4. 不依赖 GUI 的双端排版/绘图验收通过后，再整理目标包结构、补齐 iOS 工厂和
   两端绘图选择；目录迁移与行为变化分开验证，不在原型上扩大另一套平台 API。
5. 按 DesignTouchInput 接入原生触摸、GUI 指针/竞争/滚动/编辑，再单独接移动 IME。
   UI、生命周期查询与退出按实际需求推进，不在移动端模拟桌面工作区/位置。
6. cmd/goui 移动构建、组包签名与组件分发；真机、16 KiB Android 内核与正式
   APK/IPA 另做发布验收。Metal、多 Scene、更广设备版本留在未来路线。

## 8. 参考与证据

以下为官方契约/源码参考，不等于本项目实测：

- [Go build constraints](https://pkg.go.dev/cmd/go#hdr-Build_constraints)：Android/
  Linux、iOS/Darwin 的构建选择。
- [Android NDK API](https://developer.android.com/ndk/guides/stable_apis)：EGL/GLES
  等公开原生接口。
- [Apple GLES 说明](https://developer.apple.com/library/archive/documentation/3DDrawing/Conceptual/OpenGLES_ProgrammingGuide/Introduction/Introduction.html)：iOS 12 起弃用。
- [CoreText 平台差异](https://developer.apple.com/library/archive/documentation/StringsTextFonts/Conceptual/CoreText_Programming/Overview/Overview.html)：两端 API 共性与字体/上下文差异。
- [StaticLayout](https://developer.android.com/reference/android/text/StaticLayout)、
  [Paint](https://developer.android.com/reference/android/graphics/Paint)、
  [TextRunShaper](https://developer.android.com/reference/android/graphics/text/TextRunShaper)：公开系统能力；后者自 API 31 提供，不能假定 API 29/30 可用。
- [AOSP Android 11 StaticLayout](https://android.googlesource.com/platform/frameworks/base/+/refs/tags/android-11.0.0_r1/core/java/android/text/StaticLayout.java)：系统布局实现参考。
- [私有原生库限制](https://developer.android.com/about/versions/nougat/android-7.0-changes#ndk)、
  [NDK Font](https://developer.android.com/ndk/reference/group/font)、
  [HarfBuzz 职责](https://harfbuzz.github.io/what-harfbuzz-doesnt-do.html)。
- [Qt 6.8 图形架构](https://doc.qt.io/qt-6.8/topics-graphics.html)、
  [Flutter Impeller](https://docs.flutter.dev/perf/impeller)：参考平台接入与绘图后端
  分离，不引入其完整 RHI、场景图或框架规模。

实测结论、环境和剩余缺口写入后续执行记录，并同步 Plan/Roadmap。

## 9. Android 系统排版首轮实测（2026-11-04）

### 9.1 环境与执行边界

- 构建宿主 Linux amd64，Go 1.25.8，使用已验证的 purego/cgo 移动分支。
  维护者使用 Java 11、Android SDK 35/D8、Android 29 NDK 输入制作 helper；
  Go 消费阶段 PATH 无原生编译器，复用原有正式 Android 宿主及入口归档，
  没有 cgo 源码生成或原生编译器调用。测试 APK 签名经官方 apksigner 校验。
- 运行宿主为 macOS 15.7.7 ARM64；专用 Android 11/API 30 ARM64 ATD，480×800、
  Software 1x。APK 是普通 NativeActivity 应用，不是在 shell 的 app_process
  中运行，未使用主线程 TextView 代替测试。JNI 在固定 GOUI 线程 attach/call/
  detach，返回的线程 ID 与 Go 所有者相同，实际 Window Paint 已发生。
- 参考使用维护者 Linux 的公共 HarfBuzz 2.7.4，与 APK 内完全相同的 DejaVuSans
  字体。字体 SHA256 为 690243adfefe0ce154b547db6205794bd30ac4277275179517a90994f4980648。
  没有加载或查询 Android 系统私有 HarfBuzz/Skia 符号，也不推断系统内部版本。
- 复现程序、原生 helper、维护者构建工具和中文操作说明位于
  tests/window/platform/android_text；正式生产后端、GUI/UI 和 runtime 基座未修改。

### 9.2 通过的能力

最终 36 项基础能力检查通过。另有 18 项严格 Cluster 等价的候选检查，其中
9 项通过、9 项失败；这两类结果分开记录，不能宣称整个 TextLayout 已支持。

基础检查包括：

- 同一线程 JNI 调用、UTF-16 代理对、组合音标、ZWJ emoji、旗帜与印度文字的
  内部光标保护，单词换行、末尾换行/空行、连续行范围、baseline 和 RTL 运行。
- 明确 Tab stop、相邻占位的 37 单位宽度/42 高度、占位不绘制，两个 Layout
  的颜色和度量隔离。
- 同一布局的 1x/2x 区域栅格化、负原点和透明空白，与完整图像对应区域逐像素
  相等；栅格化不改变布局。原始半透明 RGBA 为独立预期 (32,16,8,128)。
- 真实截图已检查 Latin、中、日、韩、阿拉伯、希伯来、印地语及彩色 emoji，
  由 GOUI Software Painter 绘制系统生成的 graphics.Image。这里只检查实际
  字形/顺序/裁剪，不使用跨平台统一金图或宣称已接入 GOUI 文字后端。

第一轮小数度量断言失败：19.25 字号的 WWW 在初始 hinted 配置中宽度为 60。
按公开 Paint 配置启用 LINEAR_TEXT_FLAG 后，原断言不变，结果为 57.09375，
最终通过；报告同时保留 hinted=60 的对照。这是测试配置调整，不是平台缺陷
修复。该标志涉及 hinting/原生 glyph cache 的取舍；没有测量性能，不据此确定
生产后端的最终绘制标志。字体位图缓存和布局度量仍应分别设计。

### 9.3 被反例否定的两个直接映射方案

同字体、同文字范围，HarfBuzz 的 shaping cluster 边界以 UTF-8 字节记录。
Android 原生 ffi 的像素与字体同 glyph 的 U+FB03 像素精确相等，独立确认
Android 实际也形成连字，而非仅据另一套引擎推断：

| 输入 | 参考 shaping 边界 | Android 光标边界 | 非零 advance 分组 |
| --- | --- | --- | --- |
| ffi | [0,3] | [0,1,2,3] | [0,3] |
| office | [0,1,4,5,6] | [0,1,2,3,4,5,6] | [0,1,4,5,6] |
| لا | [0,4] | [0,2,4] | [0,4] |
| a + U+200B + B | [0,1,4,5] | [0,4,5] | [0,4,5] |
| U+200B | [0,3] | [0,3] | [3] |

系统允许在部分连字内部编辑；零宽字符又可能被光标导航跳过。它们是合法的
系统行为。非零 advance 能识别此处的连字，却会丢失零宽 shaping cluster。
这些反例否定的是严格 shaping 等价，不表示 Android 的编辑能力不符合 GOUI。
本轮之后已明确 Cluster 为最小交互单元，取代此前过窄的“GOUI shaping Cluster”
表述；原始对照断言与结果保留，不把合理的交互粒度差异改记为 shaping 等价。

实测结论只否定这两种直接映射；后续先按更新的交互原则验证 Android Layout，
不以 shaping 等价不成立为理由优先更换排版引擎。正式后端尚未实现。

### 9.3.1 本轮交互验证方案

使用公开 Android ICU 字素分段和同一 StaticLayout 的方向、行、选区路径及
光标坐标构造现有 TextLine/TextCluster，传给真实 gui/textedit.Geometry 验证。
不增加公共 Run、HitTest 或选区绘制接口，不实现生产 androidtext。

验收包括组合字符/emoji 不可内部停靠、ffi 可独立编辑、样式边界不拆字素、零宽
范围不丢失、RTL/软换行的坐标和亲和性；选区与同一 Layout 的 getSelectionPath
在实际文字行范围内一致。GOUI 不必复制原生选区延伸到整行空白的高亮策略。
几何比较使用独立原生查询，不能只把同一批 Cluster 互相比对。

原生选区路径通过 Region 转为像素覆盖参考，Go 根据现有选区矩形计算覆盖；
由于 Region 整数栅格与 float32 逻辑几何的量化，最多允许原生边界一个 1x
物理像素内的差异，内部和外部必须严格一致。重合的零宽/bidi 边界允许返回不同
合法偏移，但必须具有相同视觉坐标；必须单独报告此类差异，不能吞掉文字范围。

不要求复制 Android 的命中策略：GOUI 按 Cluster 边界及 Position 亲和性命中。
原生 getOffsetForHorizontal 的原始值必须保留；测试参考若吸附字素内部位置，
或保留软换行上游行尾，必须以原生端点坐标独立计算并标明原因，不能把任意
字节差异算作通过。生产后端只需输出布局事实，GUI 仍使用自己的 Geometry。

### 9.4 验证范围、保留材料及下一步

Go 回归：相关 race/vet 通过；已跟踪 Go 源码及本例的隔离副本，显式 replace
到 FFI 分支，GOWORK=off go test -count=1 ./... 为 3505 通过、10 既有跳过、
0 失败。原工作区四个未跟踪旧 example 目录的过期接口未修改；不将它们的
编译失败与本轮混合，也不把隔离副本描述成原目录全仓通过。

未执行：API 29 和 API 31+ 设备、Android 真机/16 KiB 内核、iOS、移动 GPU、
性能/内存长期测试。SDK 35 的 PositionedGlyphs 公共方法检查不是 API 31+
原生运行；它提供 glyph/位置/字体，却未提供可直接使用的原文字索引方法。
完整换行模式、对齐、范围字体、字体注册、变更/销毁通知、缓存等仍须在正式
后端中逐项实现和复用现有 TextLayout 回归，不由本次基础检查推断。

首轮及最终报告、反例、截图、消费日志、测试包哈希和基座/宿主复用记录在
本机 /tmp/goui-android-text-40_at_fu，Mac 同名目录。最终 APK SHA256 为
b88b6ead6c1f2b35e76f8ffc9ecb1198f238e3623f428a9a9ca5fbc4632abfe0。
首轮未通过的小数断言和失败结果一并保留。专用模拟器与 ADB 已停止，未修改
用户签名、安全配置或其他设备；字体和产物没有加入仓库。

### 9.5 最小交互单元实测（2026-11-04）

结论：公开 ICU + StaticLayout 足以表达本轮样本的 TextLine/TextCluster，现有
gui/textedit.Geometry 可以消费这些值，无需新增公共 HitTest 或 Run，也无需
为这轮需求改用 go-text。此结论是能力验证，不是完整 Android 排版后端交付。

环境沿用 Android 11/API 30 ARM64 ATD、Software 1x；Linux Go 1.25.8 无原生
编译器消费，Mac ARM64 上运行专用模拟器。Java helper 与原生测试包装属于
验证程序，未并入 runtime/cgo 基座，也没有改变平台与 GUI 的依赖方向。

- 18 组：ffi/office、组合字符、Emoji/ZWJ/旗帜、印度元音/合字、阿拉伯文、
  混合 bidi、连续零宽/纯零宽/RTL 零宽、软换行、空行、LTR/RTL 末尾换行、
  跨颜色/字号的组合字符与 Tab。40 项原生边界/行断言全部通过。
- 导出 28 行、94 个交互单元、116 个原生光标、847 个命中点、525 个完整选区。
  实际 Go Geometry 的 3814 项检查全部通过：UTF-8 覆盖、字素内部 Snap、原生
  主光标、命中、视觉前进/终止及选区像素覆盖。所选样本的边界像素差异为 0；
  测试仍保留有依据的 1 px 整数 Region 边界量化上限。
- 原始 getOffsetForHorizontal 与 GOUI 相差 59 次，全部保留：30 次零宽/bidi
  重合视觉点，25 次软换行上游行尾，4 次字号范围切进组合字符后的内部命中。
  后两类用原生端点坐标建立显式契约参考，现有 GUI 算法未修改。

软换行差异有源码依据：Android 11 的
[Layout.getOffsetForHorizontal](https://github.com/aosp-mirror/platform_frameworks_base/blob/android-11.0.0_r1/core/java/android/text/Layout.java#L1426)
限制非末行的最大命中偏移；GOUI Position 已支持同一偏移的上游/下游位置。
字号切进字素后的内部命中是此版本实测，不推断所有版本相同。不能直接把
原生命中结果当成 GOUI Cluster 边界；以 ICU 分段、同一 Layout 的方向和几何
导出交互单元，再由既有 Geometry 处理即可保持完整字素。

首轮 17 组有 37 项不一致，原始证据保留。除了上述命中策略差异，首轮把
getLineWidth 中的换行符宽度当成实际文字范围，导致选区多出约 12.5 DIP。
测试适配已改为使用硬换行前的原生光标确定文字边界，不伪造可见换行 Cluster；
新增 RTL 末尾换行复验。旧 shaping 对照仍有 9 项不等价，没有删除或改为通过。
原有 36 项基础检查、1x/2x 区域栅格化、预乘像素及真实 Software 呈现继续通过。

材料在 Linux/Mac 的 /tmp/goui-android-interaction-ywqhwcai：first-evidence 与
final-evidence 归档、report.json、interaction-go.json、Go 帧/线程记录、截图及
消费构建日志。最终 APK SHA256：
2345a9b01c6aacc5acc111991e00be298747a8812eb0151cb431bbdad6c675a6。
专用模拟器与 ADB 已退出；没有修改桌面主分支、cgo/purego、签名或安全配置。

本机校验：隔离的已跟踪源码副本仅替换到本轮 FFI 分支，GOWORK=off
go test ./... 为 3507 项通过、10 项既有跳过、0 失败；跳过为 9 项需人工操作的
窗口测试及本机不适用的 macOS 签名测试。相关比较器/textedit 的 race 与 vet
通过。原工作区直接带 GOWORK 执行全仓仍失败：四个未跟踪旧 example 使用过期
API，cmd/goui 的临时模块又不属于该 go.work。两类原始日志保留，未删除例程、
放宽测试或把环境错误算作通过；隔离副本通过不等于原目录全仓通过。
验证程序另提供非 Android 提示入口，Linux amd64、Windows amd64、Darwin arm64
的 CGO_ENABLED=0 编译通过，不因新增可移植逻辑比较器破坏桌面 go build；
这不是 Windows/macOS 原生排版验收。

仍待：API 29/31+、真机、生产对象/失效/销毁、完整字体与对齐/换行能力、性能
与内存对照。LINEAR_TEXT_FLAG 仍只是本轮小数度量配置，不能据此承诺生产
hinting/cache 性能；桌面后端的交互粒度审视单列，不纳入本轮通过结论。

## 10. 两端 Typography / Graphics 最小实现（2026-11-04）

### 10.1 实现边界

本轮只改平台层和独立窗口验证，不增加 gui/ui 触摸实现。触摸、滚动竞争和 IME
设计已记录在 DesignTouchInput，待平台切片完成再推进。

CoreText.NewContext 改用 frameworks.InitTypography，只初始化 CF/CG/CT，幂等
状态与已有总入口共用锁。macOS 原生窗口仍可调用完整 Init；不以 iOS 需求改变
既有字体单位、每 Layout 状态隔离或 Rasterize 契约。Android 继续复用正式
androidtext，不另写一套测量/栅格化，也不访问系统私有 Skia/HarfBuzz。

移动 OpenGL Context 通过独立宿主的私有 GLES ABI 创建、绑定、解绑、提交和释放。
Android 使用 EGL ES3 context + window surface + 1x1 pbuffer；iOS 使用 EAGL ES3
context + CAEAGLLayer drawable FBO + 私有 scratch FBO。所有 GL 调用和 context
资源归 GOUI 固定线程，Activity/Scene、窗口、图层属性及目标替换归原生 UI 线程。
宿主按 GOUI_GLES=0/1 制作独立 Software/GLES 组件，不能在运行中混用呈现方式。
这些组件仍属于 platform，不加入最小 runtime/cgo 基座。

common.NativeTarget 表达可释放的原生目标借用。mobile.Window.AcquireTarget
在绑定前取得同代目标；Context.ClearCurrent 后释放。原生撤销先拒绝新借用，
再等待已开始的 GPU 帧或 Software present；不等待 GOUI 消息循环处理丢失事件。
绑定期间不得同步等待原生 UI 线程，也必须先结束绘制再 Destroy Window。
TryRLock 拒绝撤销中的嵌套借用/Draw，避免持有旧借用时被排队 writer 阻塞。

窗口目标替换重新建立 EGL surface / EAGL drawable，不重建逻辑窗口、context
或 graphics.Image。Painter 构造结束主动解绑，避免首帧之前仍占用目标借用。
iOS 另有原生执行锁，覆盖没有窗口目标的 scratch 操作；进入后台和 Scene 断开
关闭执行许可并等待当前操作结束。首版每次解绑用 glFinish 排空 GPU 后清除
当前 context，确保后台回调返回后没有遗留命令。这是明确的保守实现，尚未做
性能结论；后续可设计专门的停绘屏障，不能直接移除同步保证。

GLES 固定 RGBA8/depth24/stencil8，不支持 MSAA、共享 context、stereo 或 sRGB
请求；iOS swap interval 返回 Unsupported。iOS 首轮在前台 target Ready 后创建
Painter，首个 Scene 前的 GPU 构造仍待工厂切片设计。真正的 EGL context loss
报告原生失败，不承诺自动恢复未保留源像素的图片。

### 10.2 NanoVGo 修正与失败证据

独立 nanovgo-mobile 的 feature/mobile-base 分支基于 v0.0.10；主会话 nanovgo
仓库未修改。当前消费使用显式 workspace，不把未发布分支写成正式 tag 依赖。

- 移动 shader 使用 GLSL ES 300 与 GL3 路径，版本声明位于首行。Android 实际
  编译器拒绝原先的前置换行；旧 createInternal 忽略 renderCreate 错误，导致
  返回没有 VBO/VAO 的半成品，后续 BufferData 才报 0x502。现在原错误返回，
  清理部分 shader/program，并补初始化失败回归；临时诊断未进入生产代码。
- iOS 首轮在字体图集创建后留下 0x502。GLES 3 的单通道纹理采用 R8 内部存储，
  源格式仍为 RED/UNSIGNED_BYTE；两者不能像原 wrapper 一样总传相同常量。
  增加显式内部格式入口并保留旧 wrapper，修正图集创建。错误不能靠日志读取
  glGetError 后被吞掉，最终使用无诊断构建重跑像素断言。
- 删除 Context 时同时释放原来遗漏的 VAO 和默认白色纹理。普通图片所有权与
  预乘契约不变，没有将系统字体排版改成 NanoVGo 字体引擎。

规范依据为 [GLES 3.0.6](https://registry.khronos.org/OpenGL/specs/es/3.0/es_spec_3.0.pdf)
第 3.8 节的存储/上传格式表；后台约束见
[Apple 多任务 GLES 指南](https://developer.apple.com/library/archive/documentation/3DDrawing/Conceptual/OpenGLES_ProgrammingGuide/ImplementingaMultitasking-awareOpenGLESApplication/ImplementingaMultitasking-awareOpenGLESApplication.html)，
context 线程约束见 [Apple 并发指南](https://developer.apple.com/library/archive/documentation/3DDrawing/Conceptual/OpenGLES_ProgrammingGuide/ConcurrencyandOpenGLES/ConcurrencyandOpenGLES.html)。
Android 销毁回调的有效期仍以 [NDK 文档](https://developer.android.com/ndk/reference/struct/a-native-activity-callbacks) 为准。
这些为原生契约；项目实际执行范围在下一节单列。

### 10.3 验收范围与后续

正式例程 `tests/window/platform/mobile_paint` 不导入 gui/ui。共同检查字体注册、
行与交互单元、范围格式/布局隔离、占位、1x/2x/3x 区域栅格和复用缓冲；Painter
检查图片更新、预乘透明合成、clip、Transform、ClearPath、文字上传及离屏读回。
具体启动、维护者制作组件、像素预期、旋转/Home 和清理见该例程 README。

本轮最终实测（Linux Go 1.25.8 消费，消费阶段没有原生编译器调用）：

| 原生环境 | 路径 | 自动断言 | 生命周期 |
| --- | --- | --- | --- |
| Android 11/API 30 ARM64 ATD，1x，CheckJNI | Android Layout + Software | 264 项通过 | 横/竖屏、一次 Home/恢复，后台帧数不变，同 PID/nonce |
| 同上，SwiftShader GLES 3 | Android Layout + OpenGL | 266 项通过 | 同上，目标 generation 更新，context/图片持续有效 |
| iOS 26.3 ARM64 Simulator，3x | CoreText + Software | 265 项通过 | 八次旋转、三次 Home/恢复，后台帧数不变，nonce 不变 |
| 同上，EAGL GLES 3 | CoreText + OpenGL | 267 项通过 | 同上，三个实际后台窗口均超过一秒，复用布局和 GPU 图片 |

自动数目不同包含 CoreText 不加载桌面库、GLES 版本等目标专属断言，不能当作
后端性能比较。四条路径还各自执行 1x/2x RenderImage；文字 Rasterize 执行
1x/2x/3x。iOS device ARM64 的 Software/GLES 均完成链接，没有签名或真机运行。

Linux amd64 独立 Xvnc :77 / Xfwm4 合成会话，已跟踪源码加本轮新增文件、显式
替换到三个实验依赖的隔离副本：`GOWORK=off go test -json -count=1 ./...` 为
3516 项通过、10 项既有跳过、0 失败。未跟踪旧 example 的过期接口没有纳入
副本，也没有修改或删除；这不等于原工作区直接全仓通过。mobile/opengl/software
的 race 通过；image_update、clear_path 原生例程的 Software/OpenGL 1x/2x 共
八次通过，覆盖构造解绑后的普通帧、离屏、图片更新和资源销毁。Windows amd64、
macOS arm64 的全仓 CGO_ENABLED=0 交叉编译通过。macOS 15.7.7 ARM64 的
CoreText 原生测试 51 个通过项；这是排版测试，不是本轮 macOS 窗口 GPU 验收。
NanoVGo 全包测试通过，独立分支提交 3a93aef，未推送/打 tag。

最终四条路径的竖屏、iOS 两路径的横屏截图及三块纯色内部采样独立记录，源像素、
实际 scale 和原生截图坐标一致；iOS 横屏按已知 LandscapeLeft 方向归一。
不使用 GUI DevServer，不以文字日志代替画面采样。
原生错误的首轮材料保留：Android shader 初始化失败、iOS R8 图集错误，以及
一次 iOS Home 动画后观察间隔不足的脚本失败。后者延长真实后台观察时间后
保留原停绘断言复验，没有放宽条件或在框架中加延迟。
另有横屏截图在新尺寸画面提交前取样的失败；尺寸事件不代表完成呈现。
等待至少两个新尺寸完成帧，核对截图前后尺寸并归一原生物理方向后，像素通过，
没有修改框架坐标、增加绘制补偿或根据颜色猜测采样点。

当前 iOS 26.3 模拟器截图有 Emoji LastResort 占位，系统日志请求的
AppleColorEmoji.ttc 文件路径确实不存在；没有用自带字体掩盖，也没有证明是
GOUI 缺陷或所有 iOS 版本都如此。字体资产/原生参考及真机另验。当前内容使用
完整原生表面，顶部可能处于状态栏/刘海下方；safe area 的报告/GUI 使用尚未实现。
不能把本轮文字基础检查描述成全部 Emoji 和系统字体通过。

原生材料与消费日志保留在 Linux/Mac 的 `/tmp/goui-mobile-paint-7d58902l`，
最终报告/截图在 gles/final-evidence.tar.gz。签名、SDK、对象和截图不提交仓库。
本轮专用模拟器和私有 ADB 已清理，没有修改用户设备、签名身份、安全设置、
主会话仓库或 cgo/purego 源码。iOS Scene 断开/重连、真机/16 KiB 内核、GPU
性能/内存、真正的 context loss 恢复、正式工厂/绘图选择、目录迁移、触摸/IME
及发布组件均未完成。后续先收敛平台工厂和组件选择，再按设计接 GUI 输入。
