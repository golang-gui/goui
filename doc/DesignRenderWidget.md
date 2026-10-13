# RenderWidget：按需重新绘制为位图

## 契约

```go
func gui.RenderWidget(widget Widget, scale float32) (image.Image, error)
```

GUI 线程、帧外调用；Widget 必须已挂载到 Window/Popover、可见且已有非空 allocation。
使用当前 allocation，不重新测量/排列，不改变位置、归属、焦点或语义快照。
有待处理的布局请求不阻止导出，也不会被导出提前执行；图片反映当前已分配尺寸，
不承诺反映下一轮布局。这允许拖动排序改变兄弟绘制顺序后同步准备原生拖放预览。
返回独立持有像素的预乘 RGBA 图片，透明背景，原点 (0,0)，物理尺寸 ceil(DIP×scale)。
scale=0 使用宿主当前比例；负数、非有限数、空尺寸、无绘制后端及嵌套绘制返回错误。

只绘制自身及可见子树，沿用各自变换和结构裁剪；不包含祖先背景、祖先裁剪、兄弟遮挡、
窗口装饰。范围为 Widget 自身布局矩形，不额外扩展阴影边界。Paint 会重新执行，
不保证等于窗口上次已显示的帧；Paint 不应推进动画或修改 Widget 树。
不叫 Snapshot，避免与语义快照混淆；不新增 UI 平行渲染机制。

## 绘制层

```go
graphics.Painter.RenderImage(width, height int, scale float32, draw func()) (image.Image, error)
```

宽高是物理像素，回调中绘制仍是 DIP；同步调用，禁止 Begin/End、Destroy 或嵌套导出。
同一 Painter 切换临时目标，所以控件已有 graphics.Image 可继续使用；回调新建的资源
仍由原 Painter 管理。临时目标结束只释放目标，不释放控件缓存。帧内 Destroy 延迟释放
仍有效。回调 panic 清理目标后继续传播，不伪装成普通错误。

Software：隔离像素缓冲及扫描器，不调用 Drawable.Draw。
OpenGL：同上下文 FBO（颜色及深度/模板附件），NanoVG 提交后读回并翻转行，不 SwapBuffers。
Direct2D：同 DeviceContext 临时目标，CPU_READ 位图复制/Map；透明目标文字使用灰度抗锯齿，
恢复原目标、DPI、变换及文本抗锯齿模式，不 Present。原生 SDK 绑定独立于适配代码。

每边最多 16384 像素，总像素最多 64×1024×1024（RGBA 256 MiB）；本限制只约束返回图，
不声称是总内存上限。后端设备可能限制更小，失败返回错误。没有持续缓存或每帧读回。
本能力返回的是重新绘制结果，不是屏幕截图，不承诺跨后端文字逐像素一致。

参考：[GTK4 WidgetPaintable](https://docs.gtk.org/gtk4/class.WidgetPaintable.html)、
[GSK render_texture](https://docs.gtk.org/gsk4/method.Renderer.render_texture.html)、
[Flutter RenderRepaintBoundary.toImage](https://api.flutter.dev/flutter/rendering/RenderRepaintBoundary/toImage.html)。
2026-09 核对官方文档；借鉴内容/目标分离和显式物理缩放，不引入 retained scene graph。

## 验证

无桌面测试覆盖 Software 与 GUI 子树：坐标归零、透明预乘、资源跨导出复用、缩放、裁剪、
变换、嵌套拒绝、panic 恢复、布局不变、图像独立以及下一帧继续绘制。
Direct2D 使用 WARP 无窗口测试；OpenGL 与完整 Widget 混合文字/图片验收放独立窗口例程。
三平台、三后端适用组合及 1x/2x 分别记录，交叉编译不等同原生运行验证。
