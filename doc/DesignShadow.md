# BoxShadow 平台绘制设计

## 目标与边界

`platform/graphics` 提供可组合的圆角矩形外阴影，并在 Direct2D、OpenGL 和
Software 后端保持一致的几何、Transform、clip 与 HiDPI 语义。阴影是绘制效果，
不是 Brush；平台类型不进入 `gui.Painter` 或 `ui`，也不参与布局空间计算。
样式层现已增加独立的 `style.Shadow` 值，不依赖 platform；GUI 宿主消费和占位
计算已由 PopoverMenu 消费，见 [菜单外观](DesignPopoverMenuAppearance.md)。

本切片不支持 Path、内阴影、边缘掩码、公开 Bounds API、通用自定义 shader
或通用效果系统。不同方向、颜色或模糊参数通过多次绘制和正常 alpha 合成实现。

## 公共 API

```go
type BoxShadow struct {
    Color        Color
    Offset       Point
    BlurRadius   float32
    SpreadRadius float32
}

type Painter interface {
    DrawBoxShadow(rect Rectangle, radius float32, shadow BoxShadow)
}
```

所有参数均为图元所在的局部逻辑坐标，和图元一起受当前 Transform 与 HiDPI
缩放影响；clip 继续使用 Painter 既有的窗口逻辑坐标契约。常规顺序是先画阴影、
再画主体，因为柔化遮罩有意包含轮廓内部。

`BoxShadow.Color` 使用预乘 RGB，例如通过 `graphics.ColorOf(color.NRGBA{...})`
转换。OpenGL/NanoVG 和 Direct2D 的阴影入口反预乘一次，Software 将预乘 RGB
与 alpha 同时乘覆盖率；清晰阴影及软阴影失败后的清晰降级遵循同一约定。
零 alpha 直接跳过。普通 Brush、Clear 和图片遵循同一预乘契约，见 [绘制颜色](DesignGraphicsColor.md)。

## 归一与高斯语义

输入按以下顺序归一：

1. 原矩形按 `Offset` 平移；
2. `SpreadRadius` 同时扩张四边，负值表示向内收缩；
3. 圆角变为 `clamp(radius + SpreadRadius, 0, min(width,height)/2)`；
4. 正 `BlurRadius` 最小归一到 `0.5` 逻辑单位，非正值表示清晰轮廓。

透明颜色、空源矩形、非有限参数或 spread 后为空的轮廓不绘制。正模糊采用与
CSS box-shadow 对齐的映射：

```text
sigma  = BlurRadius / 2
extent = 3 * sigma = 1.5 * BlurRadius
```

高斯在数学上具有无限尾部，实际渲染在轮廓外 `3σ` 截断。对于足够长的直边，
轮廓处遮罩约为 50%，向外一个 `BlurRadius` 时约为 2.275%；小矩形、大模糊和
圆角附近由完整有限形状卷积决定，不强行保证轮廓处恰好 50%。清晰轮廓在内部
及边界为 100%，外部为 0%。

共享算法先解析积分完整矩形，再减去四个圆角缺口；每个缺口使用固定 16 点中点
积分。Go 与 NanoVG shader 使用同一 erf 近似和常量。Software 是确定性参考，
OpenGL 与它计算同一个解析近似。

常用的下边缘强调配方仍为：

```go
blur := float32(6)
p.DrawBoxShadow(rect, radius, graphics.BoxShadow{
    Color:        shadowColor,
    Offset:       graphics.Point{Y: blur},
    BlurRadius:   blur,
    SpreadRadius: -blur,
})
```

主体随后覆盖内部。高斯尾部使顶部和侧面不再数学上绝对透明，但其遮罩约低于
3%，也不会出现边缘掩码造成的硬截断或圆角接缝。

## 后端实现

- 共享实现位于 `platform/graphics/internal/boxshadow`，负责归一、`sigma`、
  `3σ` 影响范围、清晰轮廓 signed-distance 和高斯透明度。
- OpenGL 使用团队维护的 NanoVG `BoxShadow` Paint，在单个 fragment shader
  pass 中计算共享高斯近似；不创建临时纹理或 FBO。
- Software 将设备像素中心经设备 Transform 的逆矩阵映回局部坐标，逐像素运行
  共享实现并生成预乘 alpha。
- Direct2D 使用 D3D11 BGRA device、DXGI swap chain 和 `ID2D1DeviceContext`。
  圆角矩形录制为 CommandList，交给原生 `CLSID_D2D1Shadow`，其中
  `BlurStandardDeviation=BlurRadius/2` 且优化模式为 QUALITY。CommandList 按
  `{width,height,radius}` 缓存 16 项 LRU。
- Direct2D 首选硬件 D3D11，失败后尝试 WARP。透明 surface 使用 DirectComposition
  与 flip swap chain；不透明 HWND 使用 sequential，失败时尝试 discard，而不是
  将透明呈现路径套给所有窗口。D2D1.1 初始化失败时由 Windows 平台继续选择 OpenGL、
  Software，不保留 D2D1.0 九分区实现。
- resize 释放尺寸相关 target，不清空可复用的阴影轮廓缓存；DPI 改变清理阴影缓存，
  device loss 释放并重建设备资源；
  `D2DERR_RECREATE_TARGET`、DXGI removed/reset 在下一帧重建。单次 effect 失败时
  退化为完整的清晰阴影，不输出半幅结果。

## 验证

共享数学测试覆盖归一、`sigma`、`3σ` 边界、直边 CDF、对称性、单调性和 16 点
积分误差。Software 像素测试覆盖全边、下边缘、多层合成、透明色、clip、旋转、
非均匀缩放与 HiDPI。Windows 测试覆盖 D2D1.1/DXGI ABI、输出定位、LRU、设备
丢失分类和资源空释放。

阴影样式切片补充了 Software 和 OpenGL/GLX 的半透明彩色像素读回测试，覆盖
1x／1.25x／1.5x／2x、清晰／柔化轮廓和叠加，避免黑色测试掩盖重复预乘错误。
Windows 增加 WARP Direct2D 清晰／柔化／降级颜色测试及 OpenGL FBO 测试；
本机只完成 Linux 运行和跨平台编译，Windows／macOS 运行验证仍待执行。

综合样例 `platform/graphics/test` 用于三平台真机对照。外部影响边界允许至多一个
物理像素误差；Direct2D 原生核允许与共享近似有细微曲线差异，但不得出现圆角
接缝、可见硬截断或资源泄漏。

## GUI facade

`gui.Painter.DrawBoxShadow` forwards `graphics.BoxShadow` without converting local
DIP to physical pixels. The existing widget transform and structural clip apply;
empty clips suppress the call. Widgets reserve shadow room in their allocation.
The facade does not change backend algorithms or resource ownership.
