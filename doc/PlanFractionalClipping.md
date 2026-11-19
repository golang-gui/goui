# 小数位置绘制裁剪：分析与待实施方案

状态：保存分析，暂缓实现。本轮先处理 SplitView，不修改绘制后端。

## 证据与影响范围

- 应用反馈的 Linux amd64 / X11 / Xfwm4 / OpenGL 1x 实测：48／64 DIP、4 DIP
  环宽的圆环在整数位置正常，顶部位于半物理像素时被截平；Software 保留边缘过渡。
  临时副本扩大裁剪后的改善仅证明问题与裁剪有关，不证明对其他控件安全。
- 源码：OpenGL SetClipRect 先用 pixelsnap.Rect 就近取整；任意 Path 继续保留
  原布局坐标。物理顶部 0.5 因而可被裁至 1。取整最初用于防止普通边框被 Scissor
  的抗锯齿再次衰减，不能简单恢复小数 Scissor。
- Software SetClipRect 使用左上 floor、右下 ceil；Direct2D 使用 pixelsnap.Edge
  就近取整和 ALIASED clip，存在同类风险，但没有本项原生实测，不能记为已复现。
- GUI 先在 DIP 中求祖先、控件与显式裁剪的交集。RenderWidget 会消除目标自身
  的位置偏移，因此只渲染单个圆环的既有回归可能掩盖本缺陷。

## 候选修复契约

1. 保留 GUI 原始逻辑交集；后端只对最终交集换算一次保守物理像素包络，左上
   floor、右下 ceil。不逐层扩大，不增加固定边距，不修改圆环或通用路径坐标。
2. OpenGL 的 Scissor 和文字缓存区域使用一致的整数包络，保留图元抗锯齿，
   避免第二次乘上小数裁剪覆盖率。Direct2D 和 Software 使用相同的私有换算辅助。
3. 明确硬裁剪的像素覆盖语义：相交的边界像素可以保留；这不是精确的亚像素蒙版。
   相邻内容、边框与视口是否仍满足约束，须独立验证后才能确认方案安全。
4. 区分正面积小裁剪、真正空裁剪和取消裁剪的零矩形哨兵，保留裁剪替换／恢复断言。

参考 Flutter 3.38.1 的 [Clip 定义](https://github.com/flutter/flutter/blob/3.38.1/engine/src/flutter/lib/ui/painting.dart#L1191)：
图元与裁剪的抗锯齿是不同选择，普通抗锯齿裁剪还可能出现合成边缘问题。
Qt 6.8.3 [矩形 clip 实现](https://github.com/qt/qtdeclarative/blob/v6.8.3/src/quick/scenegraph/coreapi/qsgbatchrenderer.cpp#L2392)
也有就近取整路径，不能据此推断所有框架都用向外取整。

## 实施前后验收

- 48／64 DIP 圆环、整数／四分之一／半物理像素位置，1／1.25／1.5／2x，
  亮暗、半透明以及固定动画角度；用独立圆面积预期检查预乘颜色和边缘覆盖。
- 渲染带小数位置子控件的父容器，并检查真实窗口帧；不只 RenderWidget(圆环)。
- 回归普通边框、嵌套裁剪、滚动视口、相邻控件、负坐标、小尺寸、空裁剪与恢复；
  完全位于有效像素包络外的像素不能改变，半透明绘制不能重复合成。
- 扩展已有 progress_regression、pixel_snap 及后端测试；原生显示依赖仍留在
  tests/window，Software 及纯换算回归留所属包。
- 覆盖 Software／OpenGL 与 Windows Direct2D、适用桌面平台。交叉构建不能
  代替原生操作；无法执行的平台须记录缺口。完成后运行 go test ./...。
