# ProgressBar

## 1. 范围与接口

控件放在 `widgets`，声明绑定放在 `widgets/ui`，不向 gui 内核增加具体控件。
一个 ProgressBar 支持长条／圆形、确定进度／
忙等待，圆形忙等待替代 Spinner，不增加第二套控件或平台能力；时间驱动复用已有 [帧更新机制](DesignAnimation.md)。

```go
type ProgressShape uint8
const (
    ProgressLinear ProgressShape = iota
    ProgressCircular
)
func NewProgressBar() *ProgressBar
// 对应 getter：Value、Shape、Indeterminate、Thickness。
func (*ProgressBar) SetValue(float32)
func (*ProgressBar) SetShape(ProgressShape)
func (*ProgressBar) SetIndeterminate(bool)
func (*ProgressBar) SetThickness(float32)
```

默认长条、Value=0、确定进度、Thickness=4 DIP。Value 为 [0,1] 的完成比例，
有限越界值钳制，非有限值归零；负数／非有限 Thickness 归零，无效 Shape 回到长条。
SetValue 不改变模式，忙等待保留 Value，退出后显示保存值。相同值 setter 无副作用。
控件不管理任务，不发完成／变化信号，不安装输入控制器，默认不接受焦点。

## 2. 布局与绘制

长条自然尺寸 160×Thickness DIP，圆形 24×24 DIP，遵循现有约束、Min/MaxSize、
MainWeight，无 baseline。长条使用分配宽度并垂直居中，圆形取居中的内接正方形。
实际厚度限制在绘制区域内；确定进度直接显示最新值，不做补间。
Value=0 只画轨道，Value=1 为完整闭合图形，不用接近一圈的弧模拟完整圆环。

使用现有 FillRoundRect／FillPath，不修改 Painter 或 platform。圆环及圆头弧段
由控件私有路径构造，单次填充同一前景，避免半透明端点被重复合成。
圆头在近完整圆周相接时合为完整环；完整圆环用外顺向／内反向的单个闭合轮廓。
极小尺寸允许厚度收缩，内半径归零时退化为实心图形。

长条忙等待为占轨道 30% 的单段胶囊，1.8 秒一周期。从左侧外部进入，始终
向右移动，完全从右侧移出后重新开始，不回头。归一化相位 q∈[0,1)，左端
位置为 `(1.3*q-0.3)*width`，长度为 `0.3*width`。保留完整胶囊形状，由已有
Widget 结构裁剪限制可见区域，不把截断片段重新圆角化。周期交界处不可见，
避免可见跳变。圆形忙等待为固定 90° 圆头弧段，顺时针匀速旋转，1.4 秒一圈；
起始角为 `-π/2 + 2π*q`，弧长角始终为 `π/2`。不伸缩弧段、不使用缓动，
两个端点等速前进。循环边界角度相差一整圈，几何连续，不停顿或跳变。
几何由单调经过时间计算，不按 tick 次数递增。

## 3. 样式

默认 StyleName="progress-bar"。BackgroundColor 为轨道，ForegroundColor 为
进度段，Radius 为长条圆角（不超过半厚度／半段宽）；圆形忽略 Radius。
无需新增 style 字段或 Part。Thickness 为绘制属性，不复用 BorderWidth。
兜底规则在 `theme/basic`，GUI 默认样式已包含 progress-bar，应用无需显式组合；
构造函数不自动注册样式。`theme/modern` 提供完整外观。
modern 使用中性轨道及可辨主题色，亮暗／主题变化通过既有 StyleChanged 和重绘
更新。自定义 StyleName 可使用透明轨道得到纯 Spinner。文本通过独立 Label／
Overlay 组合，不引入文本格式、任务绑定、取消按钮或父子样式继承。

## 4. 时间与生命周期

按需连接所属 Root 的 `ConnectFrame(func(time.Time))`，启动与每次活动帧只通过
`RequestPaint` 驱动下一帧，不为控件创建 Timer 或 goroutine。根据单调经过时间
计算姿态，落后的帧跳过而不补播；不请求布局、UI 更新或语义更新。Root 调度
不是原生 vsync 承诺，详见 [动画设计](DesignAnimation.md)。

切回确定进度、有效隐藏、零尺寸／零厚度、卸载／销毁时断开 frame handle；
重新得到可绘制布局时按需连接。回调检查 Root 归属，跨窗口不沿用旧连接；
重复声明不增加连接。请求绘制失败时停止，保留静态忙等待图形，不主动重试。
首版不增加原生窗口遮挡检测。
不能启动时，长条显示固定的可见胶囊（450ms 姿态），而不是循环开始处的空轨道；
圆形显示从顶部起始的固定 90° 静态弧段。布局由私有 LayoutManager 提供自然尺寸，沿用
WidgetBase 的可见性、Min/MaxSize 与父约束处理，不复制 GUI 的私有 constrain。

## 5. UI 与 Snapshot

```go
// import wui "github.com/golang-gui/goui/widgets/ui"
wui.ProgressBar(progress).MainWeight(1)
wui.ProgressBar(progress).Shape(wui.ProgressCircular).Thickness(3)
wui.ProgressBar(0).Shape(wui.ProgressCircular).Indeterminate(true)
```

View 为普通链式声明；移除 modifier 恢复 GUI 构造默认值，显式 Thickness(0)
不当作未设置。重建保留控件及忙等待相位，不增加 Root／BuildContext 特例。

Snapshot.Role="progressbar"，角色常量及 ProgressInfo 数据类型属于 widgets；
Attributes["goui.progress"] 为包定义的只读值：
形态、Indeterminate、确定模式下的 Value（忙等待时省略）。不报告动画相位，不
伪造可调节 Range 或点击动作。数据独立于控件可变状态。

## 6. 参考与取舍

- [GTK4 fraction](https://docs.gtk.org/gtk4/method.ProgressBar.set_fraction.html)：
  使用 0..1 比例；[ProgressBar](https://docs.gtk.org/gtk4/class.ProgressBar.html)
  将确定进度与 activity 区分。GOUI 不要求使用者周期调用 Pulse。
- [Avalonia ProgressBar](https://docs.avaloniaui.net/controls/feedback/progressbar)：
  独立 IsIndeterminate；采用显式模式，不引入 RangeBase／模板系统。
- [Avalonia 11.3 Fluent 源码](https://github.com/AvaloniaUI/Avalonia/blob/11.3.0/src/Avalonia.Themes.Fluent/Controls/ProgressBar.xaml)：
  参考水平方向动画从起点前进到终点再循环的方向设计；其双段、缓动与模板绑定
  不照搬。GOUI 使用单段等速和已有 Widget 裁剪，避免为了外观扩大 API。
- [Flutter 3.35 源码](https://github.com/flutter/flutter/blob/3.35.0/packages/flutter/lib/src/material/progress_indicator.dart)：
  参考确定／忙等待时钟启停，不采用 Material 的弧段伸缩，也不复制完整参数或
  AnimationController。其 vsync 是 Flutter 承诺，GOUI 普通 Timer 不具备该承诺。
- [Qt 6.8 Fusion BusyIndicator 源码](https://github.com/qt/qtdeclarative/blob/v6.8.0/src/quickcontrols/fusion/BusyIndicator.qml)：
  其外观使用循环 0°→360° 的 RotationAnimator。采用固定图形持续旋转的思路，
  但 GOUI 使用已有圆头弧段与 1.4 秒周期，不照搬 Qt 的具体外观、1 秒周期或动画设施。
- [Qt 6 ProgressBar](https://doc.qt.io/qt-6/qprogressbar.html)：不采用
  minimum=maximum=0 的模式哨兵，避免与普通零进度混淆。

以上为文档／源码参考，不能替代 GOUI 实测。首版不包含纵向长条、反向／缓冲
进度、平滑业务值、自动完成隐藏或系统减弱动画策略。

## 7. 测试与验收

以下带日期的验证为当时 Timer 版本的历史证据，不等于当前帧驱动已按同样范围
重新验收。当前迁移契约与验证范围见 [DesignAnimation](DesignAnimation.md)。

包内测试覆盖归一化、约束、四种模式、完整圆环、半透明合成、clip／Transform／
1x/2x、长条单周期位置单调与循环边界不可见、圆形固定弧长／等速顺时针旋转／
循环连续、隐藏／卸载／重挂载、定时失败和声明更新。
圆形回归先在旧实现上确认失败：弧长数学断言检测到 45°，350ms 的独立象限
像素采样检测到额外象限被绘制。新预期是 90° 固定弧段，不通过放宽容差消除差异。
包内逻辑只使用 GUI 公共接口，不借用私有调度器；真实 Timer 启停、身份与
声明重建、销毁停表的集成断言放在窗口例程中。已有 GUI Timer 测试继续覆盖
调度与已投递回调的取消，不为迁移导出私有状态。
窗口例程在 tests/window/widgets/progress_bar 和 tests/window/widgets/ui_progress_bar，
中文说明环境、步骤、预期、副作用、复位与退出。真实输入不依赖 DevServer。

### 7.1 迁移及长条单向循环基线

2026-10-06，迁移与长条单向循环实现完成，尚未提交。下表为上一轮迁移后的
验收记录，不作为新圆形动画的验收；圆形固定弧段旋转的复验见 7.2。

| 项目 | 结果 |
| --- | --- |
| `go test -count=1 -json ./...` | 通过；49 个包，1922 个测试／子测试通过，14 个既有测试跳过，53 个包无测试；无失败 |
| `go test -race ./widgets ./widgets/ui ./widgets/style ./theme/modern` | 通过 |
| `env -u DISPLAY go test ./widgets ./widgets/ui ./widgets/style ./theme/modern -run Progress -count=1` | 通过；新增包内测试不依赖桌面 |
| `go vet ./widgets ./widgets/ui ./widgets/style ./theme/modern` | 通过 |
| Windows amd64 / macOS arm64，`CGO_ENABLED=0` | widgets／widgets/ui 测试目标及两处窗口例程交叉编译通过；未原生运行 |
| GUI 例程，Software，1x | 13 次状态断言，真实 Timer 启停、活动图像变化、隐藏恢复及销毁前停表通过 |
| UI 例程，Software / OpenGL，各 1x / 2x | 每组 13 次状态、身份及真实 Timer 断言通过；确定进度、隐藏恢复、亮暗／主题色、声明重建不新增 Timer，以及销毁停表、正常退出通过 |
| 单向循环，以上五组窗口运行 | 每组连续采样 42 帧实际长条像素；同周期只向右，越过右端后从左端重入，没有中途反向 |
| UI 例程，OpenGL，1x，窗口调整 | 900×560 → 700×520 → 1000×600；长条按分配宽度变化，圆形不拉伸，身份与进度断言通过 |
| `git diff --check` | 通过 |

本机环境为 Linux amd64 / Ubuntu 22.04，独立 Xvnc 桌面，Xfwm4（关闭合成），
OpenGL 为 Mesa 23.2.1 / llvmpipe。缩放由 GOUI_PLAT_SCALE 指定。该桌面不能
满足透明窗口，例程请求 Integrated 后按既有契约回退 Native；本次不将其计为
Integrated 装饰验收。使用原生鼠标／窗口事件，不依赖 DevServer。像素比较检查
活动区域两帧变化／静止，连续帧检查单向运动与循环；截图另行检查轨道、圆头、
完整圆环和主题外观。销毁时先检查公开 Timer.Active 全部为 false，再退出事件
循环，避免应用退出的全局清理掩盖控件泄漏。
离屏 alpha 测试输出为预乘 sRGB RGBA8，半透明前景允许 1 字节舍入误差。

全仓既有测试仍可能写图片等产物，不将全仓运行描述为无桌面副作用；未顺带
改写这些测试。临时日志、截图和目标文件在 `/tmp/goui-progress-migrate.QferxL/`。

### 7.2 圆形固定弧段旋转复验

2026-10-06，固定 90° 弧段旋转实现与复验完成，尚未提交。只修改圆形忙等待
的弧长计算及相关测试、说明，不修改 Timer、公共接口、Painter 或平台后端。

| 项目 | 结果 |
| --- | --- |
| `env -u DISPLAY go test ./widgets -run 'TestProgressBar(Motion\|CircularRotationPixels)$' -count=1` | 旧实现数学及像素断言失败，新实现通过；覆盖固定弧长、等速顺时针、四象限、跨周期、圆心透明及半透明不重复合成 |
| `env -u DISPLAY go test ./widgets ./widgets/ui ./widgets/style ./theme/modern -run Progress -count=1` | 通过，含 Software 离屏 1x/2x |
| `go test -count=1 -json ./...` | 通过；49 个包、1923 个测试／子测试通过，14 个既有测试跳过，53 个包无测试，无失败 |
| `go test -race ./widgets ./widgets/ui ./widgets/style ./theme/modern` | 通过 |
| `go vet ./widgets ./widgets/ui ./widgets/style ./theme/modern` | 通过 |
| Windows amd64 / macOS arm64，`CGO_ENABLED=0` | widgets 测试目标及 UI 窗口例程交叉编译通过，未原生运行 |
| UI 例程，Software / OpenGL，各 1x / 2x | 每组 40 帧连续圆形像素采样及 8 次状态／身份／Timer 断言通过；旋转至少一圈、周期符合 1.4 秒，无反向或弧长伸缩；停表、祖先隐藏恢复、重建不新增 Timer、亮暗／主题色及销毁停表通过 |
| `git diff --check` | 通过 |

本轮窗口环境与 7.1 相同，仍为 Native 回退及 llvmpipe，不将其计为 Integrated
装饰或硬件 OpenGL 验收。48 DIP 圆环的中线半径为 22 DIP，4 DIP 厚度的两个
圆头使中线可见角约为 `90° + 2*asin(2/22) = 100.43°`。1° 间隔像素采样预设
±8° 容差覆盖 1x 像素定位及端点抗锯齿；四组实测为 99°～104°，不是弧长变化。
包内测试仍直接断言弧长角为 π/2，不以窗口取样容差代替数学契约。

首次外部采样脚本错误假设各行位置相同，报告 12°；核对截图后，改为根据完整
轨道像素校准圆心，保持原采样预期和容差，未为脚本定位问题修改产品代码。
本轮日志、截图、回归前后结果及目标文件在 `/tmp/goui-progress-spin.pLlZIs/`。
隔离桌面已关闭，未修改用户正在使用的桌面。

未执行：Windows Direct2D／macOS OpenGL 原生桌面验收、硬件 OpenGL 和混合
显示器 DPI 切换。交叉编译及本机 llvmpipe 不替代这些验收。首版动画不承诺
vsync／最小化时停止唤醒；后续由独立动画系统处理。
