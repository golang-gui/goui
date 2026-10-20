# 文本区域光栅化验收（2026-10-20）

实现契约见 [设计](DesignTextRasterization.md)。GOUI 修改前基线为 `809083e`，
SourceView 的联调基线和窗口记录见其 doc/PlanTextRasterization.md。

## 根因与修正

Linux / Pango 1.50.6 / Cairo 1.16.0，单行 157,862 字节的有效 JSON，测量约
1,578,396×24 DIP。旧 Rasterize 请求完整超宽 surface，首次返回 Cairo INVALID_SIZE；
失败却留下已公布尺寸，第二次跳过初始化并 cairo_save(NULL)，SIGSEGV。
独立原生调用与实际 example/temp 的 Open 文件路径都曾复现；用户文件不纳入仓库。
40,000 个 ASCII 字符同样可以复现，不依赖 JSON 内容。

修正覆盖三排版后端的候选资源提交、失败释放和 Destroy 状态清零，并直接修改
Rasterize 接口为显式像素 region。Software/OpenGL 只请求可见区域，在有限原生块中
绘制完整布局，拼接后合成一张区域图片。缓存加入 region；原点只对齐一次。
Pango/DirectWrite 的度量和绘制使用原布局坐标，CoreText 移除额外 frame 高度截断。
Direct2D 保留原生绘制，使用 Painter clip，不隐式裁剪至无折行布局约束。

## 包内回归

- `typographytest.RasterRegions`：负坐标、空/透明区域、buf 复用、分数倍率、
  center/end 对齐位置、与完整区域对应的每个 RGBA 像素、度量稳定、长文本尾部。
- Pango 实际 40000×1 surface 超限，首次/重复失败、成功后扩容失败、sticky error
  丢弃与重试；失败不能留下尺寸或替换原可用 context。
- 区域反变换、整数溢出和 64 MiB 存储边界，分块拼接的独立坐标颜色预期，
  scale/region 缓存区分、失效与对象隔离。
- GUI 离屏：80,000 字符 TextInput、无折行 TextView，以及 10,000 字符/48 DIP 窄
  视口折行 TextView；首尾、重复绘制、1×/1.25×/2×，均必须有真实文字颜色，
  原生请求不超过 1024×1024。

Linux amd64 / Go 1.25.8 上 `go test ./...` 和上述目标 race 回归通过。
既有全仓测试仍可能更新原有 PNG 产物，本轮不迁移这些测试，不提交产物。

macOS 15.7.7 arm64：交叉编译测试程序在原生主机执行，区域回归、重复倍率、
显式区域尺寸及 GUI 的三种文本场景通过。GUI 二进制传输前后 SHA-256 一致。
这不是 macOS 桌面窗口场景验收。

Windows amd64：DirectWrite、GUI 及 SourceView 测试目标交叉编译通过。
Windows 10 19045 目标机已接收并核对 DirectWrite 测试二进制哈希，但启动返回
“拒绝访问”，因此原生像素与 Direct2D 窗口行为未验证。没有修改目标机权限或执行策略。

Linux 独立 Xvnc :167 / Xfwm4 compositor on / llvmpipe：SourceView 窗口程序
在 Software 1×、1.25×、2× 及 OpenGL 1× 完成 160,011 字节 JSON 的八步骤检查；
用户原始 157,862 字节 JSON 另在 Software 1× 完成同组检查。原生区域请求均不超过
1024×1024，没有 Rasterize 错误。`example/temp` 的实际 Open 文件对话框也重新打开
同一用户文件，界面显示语法色与水平滚动条，未再触发 SIGSEGV；用户文件与截图
不纳入仓库。具体场景、等待条件和样本数见 SourceView 同名 Plan。原生文件选择器
可能刷新最近文件记录；独立桌面在验证完成后关闭，不操作用户原有桌面。


## 保留的边界

区域光栅化不会使完整 native shaping、逐字命中几何或范围着色变成有界工作。
SourceView 的 160 KiB 连续单词切换 WrapWordChar 时，Linux 的 Pango GetExtents
出现超过一分钟的同步排版；诊断主动 SIGQUIT，栈在 Pango 布局度量中，不是修复前
的 Cairo 空 context 崩溃。未将此探测计为通过，也未通过截短样本或放宽断言掩盖。
完整 frame 覆盖、极端字体墨迹外伸、硬件 GPU、混合 DPI 和近奇异变换仍需专项验证。

本轮只提交列明的实现、调用迁移、回归与文档。工作区原有未跟踪原生测试中的必要
Rasterize 调用也随契约迁移，但不将这些用户文件整体纳入提交。
