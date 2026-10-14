# 绘制系统变换（Transform）设计

## 目标

为 GOUI 绘制系统添加 2D 变换支持，使控件能够实现旋转、缩放、平移等效果。

## 业界方案对比

| 框架 | 变换位置 | 实现方式 |
|------|---------|---------|
| Qt（QPainter） | 绘制层 | `QTransform`，GPU 加速 |
| GTK（Cairo） | 绘制层 | `cairo_matrix_t`，CPU 变换 |
| Flutter（Canvas） | 绘制层 | `Float64List` 矩阵，GPU 加速 |

**结论**：变换应放在 `graphics.Painter` 层，利用后端原生支持。越接近底层，
越能利用 GPU 硬件加速；`gui.Painter` 只负责把 widget-local 偏移与用户变换
合成后透传给 `graphics.Painter`。

## 后端支持情况

| 后端 | 变换支持 | 实现方式 | 性能 |
|------|---------|---------|------|
| D2D | ✅ 原生 | `ID2D1RenderTarget::SetTransform` | GPU 加速 |
| OpenGL | ✅ 原生 | NanoVG `SetTransformByValue` | GPU 加速 |
| Software | ✅ 软件实现 | `deviceTransform` 变换路径，逆映射采样位图/渐变/阴影 | CPU，较慢 |

## 设计

### 1. `geometry.Transform` 类型

```go
// core/geometry/transform.go

// Transform 是行优先 3×2 仿射矩阵：
//	| A11 A12 TX |
//	| A21 A22 TY |
type Transform struct {
	A11, A12 float32 // row 1
	A21, A22 float32 // row 2
	TX, TY   float32 // translation
}
```

`geometry.Transform` 按列向量公式解释：`x' = A11*x + A12*y + TX`、
`y' = A21*x + A22*y + TY`。Direct2D 的 `Matrix3x2F` 使用行向量公式，虽然
两者都以六个标量保存 3×2 数据，但线性部分的非对角项必须在适配时转置，不能
按同名字段直接复制。

### 2. 构造函数

```go
func Identity() Transform
func Translate(dx, dy float32) Transform
func Scale(sx, sy float32) Transform
func Rotate(degrees float32) Transform  // 角度制，用户友好
```

`Rotate` 正值 = 顺时针（屏幕坐标系 Y 向下）：
```
|  cos  sin |
| -sin  cos |
```

### 3. 方法与链式语义

```go
// Multiply 返回 t * o（标准矩阵乘法，先应用 o，再应用 t）。
func (t Transform) Multiply(o Transform) Transform

// 链式方法在局部空间应用：先执行 t，再执行本次变换。
// 返回新值，不修改 t（Go 值语义）。
func (t Transform) Translate(dx, dy float32) Transform
func (t Transform) Scale(sx, sy float32) Transform
func (t Transform) Rotate(degrees float32) Transform

// TransformPoint 变换一个点。
func (t Transform) TransformPoint(p Point) Point

// Inverse 返回逆变换；不可逆（行列式 ≈ 0）时返回 Identity。
func (t Transform) Inverse() Transform
```

链式示例：`geometry.Translate(10, 20).Rotate(45)` 表示先旋转 45°、再平移
`(10, 20)`（局部空间语义，等价于 `Translate(10,20).Multiply(Rotate(45))`）。

旋转围绕原点 `(0, 0)`。围绕任意点 `(cx, cy)` 旋转由上层组合实现：
`Translate(cx, cy).Rotate(d).Translate(-cx, -cy)`。

### 4. `graphics.Painter` 接口扩展

```go
// graphics/painter.go

type Painter interface {
	// ... 现有方法

	// SetTransform 设置后续所有绘制操作的绝对变换矩阵（替换而非累积）。
	SetTransform(transform geometry.Transform)
}
```

### 5. 各后端实现

**D2D**（`platform/graphics/direct2d/painter_windows.go`）：

Windows 后端使用 D3D11/DXGI swap chain 上的 `ID2D1DeviceContext`；不再使用
`ID2D1HwndRenderTarget`。两种目标的矩阵约定相同，转换仍必须转置非对角项。

```go
func (p *Painter) SetTransform(t geometry.Transform) {
	p.matrix = d2d1.Matrix3x2F{
		M11: t.A11, M12: t.A21,
		M21: t.A12, M22: t.A22,
		M31: t.TX, M32: t.TY,
	}
	p.render.SetTransform(&p.matrix)
}
```

**OpenGL / NanoVG**（`platform/graphics/opengl/painter.go`）：
```go
func (p *Painter) SetTransform(t geometry.Transform) {
	// NanoVG 的 SetTransform 是预乘当前变换；先 Reset 再设置绝对变换。
	p.vg.ResetTransform()
	p.vg.SetTransformByValue(t.A11, t.A21, t.A12, t.A22, t.TX, t.TY)
}
```

**Software / rasterx**（`platform/graphics/software/painter.go`）：
```go
func (p *Painter) SetTransform(t geometry.Transform) {
	p.transform = t
}

// 点变换
func (p *Painter) deviceTransform() geometry.Transform {
	return geometry.Scale(p.scale, p.scale).Multiply(p.transform)
}
```

Software 后端的位图（`DrawImage`/`DrawTextLayout`）走 `drawBitmap`：把目标
矩形四角经 `deviceTransform().TransformPoint` 变换成四边形路径，再用逆变换 `ColorFunc` 把每个
设备像素映射回源位图采样，支持任意仿射（旋转/缩放/平移）。

### 6. 嵌套语义（gui.Painter 集成）

`gui.Painter` 的 `painterState` 保存当前 Widget 的窗口原点和用户变换。
`Save/Restore` 用于同一次 `Widget.Paint` 内的显式状态嵌套；Widget 返回后，
GUI 自动恢复入口状态，再遍历子节点：

```go
type painterState struct {
	scopeClip geometry.Rectangle
	clip      geometry.Rectangle
	offset    geometry.Point      // widget-local → window-local 的累计位移
	userXform geometry.Transform  // widget 通过 SetTransform 设置的变换
}

// 有效变换：先应用用户变换（widget 局部空间），再平移到窗口坐标。
func (p *painter) fullTransform() geometry.Transform {
	return geometry.Translate(p.state.offset.X, p.state.offset.Y).Multiply(p.state.userXform)
}

func (p *painter) applyTransform() {
	p.base.SetTransform(p.fullTransform())
}
```

- 所有绘制方法直接透传 widget-local 坐标，不手动加 offset；位移统一由
  effective transform 处理。
- GUI 使用单一扁平 Painter 自动遍历 Widget 树。进入子 Widget 时只累加结构
  offset，局部用户变换重置为 Identity，因此父 Paint transform 不传给子树。
- 需要同时影响子树绘制、命中测试和语义边界的变换属于未来 Widget/scene
  能力，不通过 Painter 状态隐式实现。

完整遍历和状态契约见 [`DesignWidgetPainter.md`](DesignWidgetPainter.md)。

### 7. clip 与 transform 的交互（重要）

D2D `PushAxisAlignedClip` 与 NanoVG `Scissor` 都会用当前 transform 变换
clip 矩形。但 `gui.Painter` 传给 `SetClipRect` 的 clip 已是**窗口坐标**
（offset 已包含），不能再被 transform 变换一次。两端都在 Identity transform
下设置 clip，再恢复原 transform：

**D2D**：
```go
func (p *Painter) SetClipRect(rect graphics.Rectangle) {
	prev := p.render.GetTransform()
	identity := d2d1.Matrix3x2F{M11: 1, M22: 1}
	p.render.SetTransform(&identity)
	// ... PopAxisAlignedClip / PushAxisAlignedClip ...
	p.render.SetTransform(&prev)
}
```

**OpenGL / NanoVG**：
```go
func (p *Painter) SetClipRect(rect graphics.Rectangle) {
	xform := p.vg.CurrentTransform()
	p.vg.ResetTransform()
	// ... ResetScissor / Scissor ...
	p.vg.SetTransformByValue(xform[0], xform[1], xform[2], xform[3], xform[4], xform[5])
}
```

注意：不能用 NanoVG `Save`/`Restore` 包住 clip 设置 —— `Save/Restore` 会
连同 scissor 一起恢复，导致刚设置的 clip 被丢弃。

**Software**：`scanner.SetClip` 直接作用于像素缓冲，不受 transform 影响，
无需处理。

### 8. 度量与文字栅格化

`strokeWidth`、`radius` 与图元坐标一样都是局部逻辑单位（DIP），随当前
Transform 和 surface HiDPI 生效，不保证固定物理像素宽度。文字的布局度量仍
以逻辑坐标描述；栅格分辨率另行跟随有效设备缩放。

文字的布局度量保持逻辑坐标不变，但 OpenGL 和 Software 后端的文字位图分辨率
必须跟随 `窗口 scale × Transform 最大伸缩率`。目标矩形仍使用原逻辑尺寸，并将
最终原点映射到设备空间后 snap 到物理像素网格，避免放大低分辨率位图导致发虚。
Painter 的文字图片缓存只以这一最终 raster scale 区分资源；origin、平移和 clip
不会改变源位图，因而不进入缓存 key。不同缩放使用独立项并受每 layout 四项 LRU
限制。
Software 的任意仿射位图路径使用预乘 Alpha 双线性采样，保留旋转文字的灰度
抗锯齿；轴对齐的 `DrawImage` 仍走直接合成快速路径。

## 改动范围

| 文件 | 改动 |
|------|------|
| `core/geometry/transform.go` | 新增 `Transform` 结构体及方法 |
| `core/geometry/transform_test.go` | 单测（构造/乘法/链式/旋转方向/逆变换） |
| `core/geometry/rectangle.go` | 新增 `Translate(pt)` 方法 |
| `platform/graphics/painter.go` | 接口新增 `SetTransform` |
| `platform/graphics/direct2d/painter_windows.go` | 实现 `SetTransform` + clip 交互处理 |
| `platform/graphics/opengl/painter.go` | 实现 `SetTransform` + clip 交互处理 |
| `platform/graphics/software/painter.go` | 实现 `SetTransform`，`deviceTransform`/`drawBitmap` 逆变换映射 |
| `gui/painter.go` | `painterState` 保存 `userXform`；`fullTransform` 合成；自动 Widget 遍历和 `Save/Restore` 隔离状态 |
| `platform/graphics/test/main.go` | 变换视觉测试用例 |

## 验证方式

```go
// 旋转 45 度
t := geometry.Rotate(45)
p.SetTransform(t)
p.FillRect(geometry.Rect(0, 0, 100, 100), graphics.RGB(255, 0, 0))

// 链式：先旋转再平移（局部空间）
t := geometry.Translate(10, 20).Rotate(45)
p.SetTransform(t)
p.FillRect(geometry.Rect(0, 0, 100, 100), graphics.RGB(255, 0, 0))
```
