# DesignImage — 持久图片绘制资源

## 背景

旧的 `DrawImage(rect, image.Image)` 把 CPU 图片直接交给即时绘制接口。OpenGL
每次调用都创建并上传 NanoVG 纹理，Direct2D 每次调用都创建 `ID2D1Bitmap`，
Software 也会重复转换像素格式。静态图片因此在每一帧重复执行与内容无关的转换、
分配和上传，图片越大，帧率下降越明显。

图片内容和一次绘制的位置、Transform、clip 是两类生命周期。前者应成为 Painter
持有的长期资源，后者仍由每帧的 `DrawImage` 描述。

## 公共 API

```go
type Image interface {
    Size() (width, height int)
    Update(src image.Image) error
    Destroy()
}

type Painter interface {
    NewImage(src image.Image) (Image, error)
    DrawImage(rect Rectangle, img Image)
}
```

- `NewImage` 立即快照 `src`；调用返回后修改源图片不会影响资源。
- 返回资源只属于创建它的 Painter，和 Painter 一样线程亲和且不可并发使用。
- `Size` 只返回资源的物理像素宽高；图片资源没有坐标原点或 SubImage 语义。
- `Update` 同步替换整张图片，只接受相同像素尺寸，返回后同样与 `src` 脱钩；
  尺寸变化由调用方执行 `Destroy + NewImage`。首版不增加局部区域更新。
- `Destroy` 幂等。资源不再使用时由持有者调用；`Painter.Destroy` 对遗留资源兜底。
  调用后立即逻辑失效，不能再 DrawImage 或 Update，但 Size 保持不变。
  帧内调用时保留底层资源到 End 消费或丢弃已记录命令后释放，之前的绘制仍然有效。
  帧外调用立即释放。不增加 DestroyOnEnd、引用计数或调用方的退休资源队列。
  `Update` 和 `Painter.Destroy` 仍不得在有效帧内调用。
- `DrawImage` 只接受有效且属于当前 Painter 的资源。跨 Painter 或销毁后使用属于
  编程错误，后端确定性 panic，避免静默绘制错误纹理。
- `NewImage(nil)` 和空图片返回错误。`DrawImage` 的目标矩形为空时不绘制。
- `DrawImage` 必须位于有效帧内，目标矩形、Transform、clip 和 HiDPI 继续使用
  Painter 现有逻辑坐标语义。

`graphics.Image` 是 Painter 所属的绘制资源，不是 Widget。它与
`Platform.NewImage` 创建的 `common.Image` 并存：后者是调用方可修改的 CPU Bitmap，
可以作为 `NewImage`/`Update` 的源，但不表达 GPU 所有权，也不直接参与 `DrawImage`。

## 像素语义

创建资源时统一使用预乘 alpha：OpenGL 使用 RGBA，Direct2D 和 Software 使用
BGRA。原生创建函数会同步复制上传数据，因此 OpenGL 对紧密排列的兼容 RGBA 先用
`ToBitmap` 取得临时零拷贝视图，格式或 stride 不兼容时才调用 `CopyToBitmap`。
Software 必须用自己的像素缓冲作为底层资源；Direct2D 目前保留独立 CPU 快照用于
设备丢失恢复。转换必须正确处理非预乘的 `image.NRGBA`、Gray、非零原点和带父图
stride 的 sub-image；`NewImage` 返回后所有后端都与 `src` 脱钩。

## 后端设计

### OpenGL / NanoVG

`NewImage` 在当前 GL context 中调用一次 `CreateImageRGBA`，`DrawImage` 后续只提交
已有 image handle 的 textured quad。文字位图也通过 `NewImage` 成为 Painter 私有的
持久资源，不再创建逐帧临时纹理。普通图片与文字缓存统一调用 Destroy，帧内设置
pending 标记，在 EndFrame 消费命令后释放纹理；异常路径先 CancelFrame 再清理，
并保证解除当前 context。无需等待 GPU 完全空闲，已提交资源由原生驱动管理。
Painter 销毁时 NanoVG context 统一兜底。

### Direct2D 1.1

`NewImage` 创建并长期持有 `ID2D1Bitmap`，`DrawImage` 直接调用 `DrawBitmap`。资源还
保留一份规范化 CPU 快照：发生 `D2DERR_RECREATE_TARGET`、device removed 或 reset
时释放所有 device-dependent bitmap，但保留逻辑资源；新设备建立后在首次绘制时
惰性重建。`Update` 使用 `CopyFromMemory` 更新同尺寸 bitmap 和恢复快照。
帧内 Destroy 标记逻辑失效；EndDraw 后释放 bitmap 和 CPU 快照，包括 EndDraw 失败。
设备资源被丢弃时同样清理 pending 图片，不能在新设备上重新创建已销毁图片。

### Software

资源持有一次转换后的预乘 BGRA 快照，`Update` 复用其像素缓冲。无旋转/错切且目标边界落在物理像素上时，
使用直接最近邻缩放和 source-over 合成，避免为每个像素调用 `image.Image.At` 和
rasterizer；任意 Transform 继续走通用四边形逆映射路径，保证功能完整。
虽然像素绘制同步完成，仍遵守统一的帧内延迟释放契约；End 的呈现错误或 panic
也会清理 pending 图片和活动帧状态。

OpenGL 和 Software 的 `DrawTextLayout` 以 TextLayout 身份和最终 raster scale 缓存
`graphics.Image`；普通图片仍完全由调用方管理，不进入这一内部缓存。

## gui / ui 生命周期

`gui.Image` 继续接收普通 `image.Image` 作为应用数据，在第一次 `Paint` 时惰性创建
`graphics.Image`，后续帧复用。`SetImage` 先销毁旧资源，Widget unmount 时释放当前
资源；创建失败时本帧跳过，后续 Paint 可重试。

`ui.ImageView.Update` 对可比较且相同的源图片不重复调用 `SetImage`，避免声明式重建
把未变化的原生资源失效。自定义绘制控件若直接创建 `graphics.Image`，同样必须在
源变化和 unmount 时显式销毁。

## 验证

- Bitmap 测试覆盖 RGBA/BGRA 通道、NRGBA 预乘、非零原点和 sub-image stride。
- Software 像素测试覆盖快照、透明合成、clip、缩放、跨 Painter 和销毁后使用。
- Update 测试覆盖同尺寸存储复用、返回后与源脱钩、尺寸变化拒绝和活动帧约束。
- GUI 测试覆盖多帧复用、替换/unmount 释放和创建失败重试。
- 三后端测试覆盖帧内逻辑失效、End 后释放、重复销毁与 Painter 兜底生命周期。
- Software 覆盖呈现失败和 1x / 2x 旧图片像素；Direct2D 增加 WARP 离屏检查。
- 桌面回归入口：`tests/window/graphics/image_lifetime`，比较销毁前记录的图片与画刷参考。
- `BenchmarkDrawImageStatic1024` 用固定 1024×1024 资源观察静态图片热路径的分配与吞吐。
- 完成前运行 Linux `go test ./...`，并交叉编译 Windows 与 macOS 全部测试包。

### 2026-09-17 帧内销毁验证记录

- Linux：`go test ./...` 通过；gui、ui、icon/...、Software、OpenGL 的 race 检查通过。
- Linux/X11：`GOUI_TEST_OPENGL=1 go test ./platform/graphics/opengl -count=1` 通过，
  显式启用了依赖桌面的既有 GLX 像素测试。
- 新窗口例程（X11 / Xfwm4）：OpenGL 1x / 2x 均验证了图片帧内销毁；窗口截图的内部采样
  与画刷参考精确一致（白底红色 RGB 255/127/127、蓝色 127/127/255）。
  2x 下通过原生输入事件切换颜色、调整窗口大小后仍一致，Esc 正常退出。
- Windows amd64 / macOS arm64：`CGO_ENABLED=0 go test -exec=true ./...` 交叉编译通过。
  此命令不执行目标测试；Windows WARP、新窗口例程的 Direct2D 与 macOS 原生运行未验证。
- 既有全仓测试会产生排版图片产物；本次窗口截图和输入验证辅助文件仅放在仓库外临时目录。
