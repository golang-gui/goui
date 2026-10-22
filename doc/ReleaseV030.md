# GOUI v0.3.0 发布说明

本版本同步更新 Linux amd64/X11、Windows amd64、macOS arm64 Demo，版本为
0.3.0、build 1。构建、公证、实际验证及下载结果以 Release 附件 `BUILD-INFO.md`
和本仓库 Plan 的本轮记录为准；本文说明功能与升级影响，不将计划作为已实现能力。

## 主要变化

- TextView 按内容测量高度，支持在父约束和 MaxSize 下自然增长；与 ScrollView
  组合时保留完整正文范围。TextInput 的文字基线与垂直居中同步修正。
- 长段落编辑、命中、滚动及换宽时复用测量和几何片段；缓存最近宽度的结果，
  减少重复排版。首次复杂超长段落仍可能需要较长的原生排版，不能保证无阻塞。
- 文本绘制按指定像素区域光栅化，限制可见区域的位图开销；修正 Pango、
  DirectWrite、CoreText 度量、FFI 及像素缓冲的相关生命周期问题。
- 连续调宽时跨段保留经整数 DIP 取整的文本锚点；Direct2D 显式按预乘 sRGB
  插值，修正不同透明度端点的渐变颜色。清理既有 vet 诊断，不关闭检查。
- Image 可显式设置 DIP 内容尺寸；Demo 的 SVG Logo 按实际缩放生成资源，
  RenderWidget 预览保留控件逻辑尺寸。
- ScrollView 可通过 frame part 统一绘制包含滚动条的外框，TextView 的 Basic /
  Modern 样式及使用者同步迁移。X11 弹窗按物理像素向上取整，避免分数缩放
  将正常菜单挤出多余滚动条。
- 新增 `cmd/svgpng` SVG 转 PNG 工具；补充文本、原生排版和窗口交互验证入口。

## 升级影响

### TextLayout.Rasterize

`platform/typography.TextLayout` 的光栅化调用增加显式像素区域：

```go
Rasterize(scale float32, region image.Rectangle, buf []byte) (TextBitmap, error)
```

区域位于排版自身的物理像素平面。返回位图大小与区域完全一致，局部原点对应
region.Min；空区域返回空位图，无覆盖部分透明。自定义后端、TextLayout 包装器
及直接调用者需要同步签名和区域语义。普通 gui/ui 文本控件使用者不需直接调用。

### TextView 测量及外框

TextView 默认高度随内容测量，不再固定申请六行。需要固定或填满高度时使用
父约束、MainWeight 或 MinSize/MaxSize；紧凑字段显式设置 MainWeight 为 0。

TextView 正文默认无外框。需要滚动和标准编辑器外框时显式设置宿主样式：

```go
scroll := gui.NewScrollView()
scroll.SetStyleName("text-view")
scroll.SetChild(gui.NewTextView())
// ui.ScrollView(ui.TextView()).Style("text-view")
```

自定义输入组合请明确外框的所有者；主题可覆盖 frame、trough、thumb part。
TextInput 继续绘制自身外框。

### Image.ContentSize

默认仍以源图片的像素宽高作为 DIP。需要按逻辑尺寸展示高分辨率图片或
RenderWidget 截图时，调用 `SetContentSize(geometry.Size{...})`，声明式使用
`ui.Image(...).ContentSize(width, height)`。两维必须有限且大于 0，否则恢复默认。

## 范围

Linux 原生 Pango 绑定的 Attribute.Class 和 Analysis.ExtraAttrs 字段改用
`*AttrClass` 和 `*glib.GSList[Attribute]`，与 C 指针语义一致，原生 ABI 偏移不变。
直接构造这些底层结构的调用者需同步字段类型；普通 gui/ui 文本使用者不受影响。

SourceView 是独立可选扩展，不随本模块或 Demo 自动包含。Linux 发布目标为
amd64/X11；Linux arm64、其他窗口系统、完整 IME 和混合 DPI 能力不在本次
发布承诺内。macOS Demo 的签名及公证状态须以最终附件的验证记录为准。
