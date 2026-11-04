# 触摸输入与 GUI 手势设计

状态：2026-11-04 设计记录，尚未实现。本轮先完成 Android/iOS Typography、
Graphics 的平台层最小验证，再实施这里的事件与 GUI 调整。此前 Android GUI
启动切片保留；接下来不继续修改 GUI/UI。

关联：[DesignGesture](DesignGesture.md)、[DesignIME](DesignIME.md)、
[DesignMobilePlatform](DesignMobilePlatform.md)、[DesignMobileLifecycle](DesignMobileLifecycle.md)。

## 1. 目标与分层

让现有 Button、ScrollView、TextInput 等自然支持触摸，应用和 UI 声明沿用当前
接口。platform 报告输入事实；gui 负责命中、传播、竞争、焦点与交互信号；ui
继续协调 Widget，不增加移动专属点击系统。

首轮支持多触点的独立分发及单触点控制器。暂不实现联合缩放/旋转、惯性/回弹、
长按菜单、选择手柄、放大镜、触摸原生 DnD 或原生窗口移动。

## 2. 平台事件

拟扩展现有 PointerEvent，而不是另建一套触摸事件总线：

```go
type PointerKind uint8

const (
    PointerKindMouse PointerKind = iota
    PointerKindTouch
    PointerKindPen
)

type PointerID uint64

type PointerEvent struct {
    EventType EventType
    Kind PointerKind
    ID PointerID
    Timestamp time.Duration
    Position geometry.Point
    Button PointerButton
    Buttons PointerButtons
    Modifiers Modifiers
}
```

- 新增 PointerCancel，保持原有事件数值稳定。Kind 零值为 Mouse，现有归并鼠标
  使用 ID 0。触摸使用非零宿主内标识，从 Down 到 Up/Cancel 稳定；后端映射原生
  身份，保证活动触点不冲突，不要求编号连续，不将它当作物理设备 ID。
- Timestamp 为原生单调时间线上的时间，仅比较同一输入流内的时间差；0 表示
  未提供，GUI 沿用既有时钟兜底。Position 仍是窗口局部 DIP。
- Touch 的 Button=None、Buttons=0；接触状态由 Down/Move/Up/Cancel 表达，
  不伪装鼠标左键。Cancel 清理交互，不能生成点击或正常拖动完成。
- 首轮只接入 Mouse/Touch。Pen 保留来源表达，不提前增加压力/倾角等字段。
  触摸板按系统提供的鼠标、滚轮或手势事实处理，不因物理手指操作就归为 Touch。

## 3. 分发与生命周期

Dispatcher 按 ID 保存活动序列：初始命中路径、竞争参与者、获胜者、捕获目标。
触摸 Down 命中一次；Move/Up/Cancel 沿初始路径和竞争结果分发，不随手指移动
重新命中相邻控件。未加入手势的原始控制器也需得到连续路由。目标失效则取消
该序列，不把仍按住的手指移交给新目标。

鼠标 hover/光标单独维护。触摸不覆盖鼠标位置、不产生 Hover、不启动 Tooltip
等待；触摸按下可关闭已有 Tooltip。Focus、键盘状态继续属于宿主。

隐藏、原生取消、模态接管、参与者失效或宿主销毁时按既有规则取消，完成/取消
互斥且至多一次。快照遍历活动序列，并在每次外部回调后复核身份与生命周期，
避免卸载、销毁或重入删掉新序列。已结束接触不能被迟到 Move/Up 复活。

## 4. 手势协议

复用 JoinGesture/Claim/Reject，不新增公共 Arena。沿用同事件深层 Widget 优先、
同 Widget 控制器注册顺序优先的同步裁决规则。

首版 GestureController 每实例一次只处理一条接触序列，与 Pressed/Dragging 和
单次 Begin/Update/End 信号一致，保留现有接受/取消接口。Dispatcher 隔离忙碌
控制器的其他触点，不能让第二个 Down 覆盖内部状态：

- 两根手指可分别操作不同按钮。
- 同一个按钮忙时不接纳第二根手指。
- 第一根结束后，不接管仍按住的第二根；必须有新的 Down。
- 普通 EventController 可观察原始多触点，但多指联合仲裁以后另行设计。

默认 Click 主操作接受鼠标左键及 Touch；右键等配置不匹配 Touch。触摸点击
初始移动容差建议 8 DIP，作为 GUI 策略，鼠标保留当前行为。超过容差后放弃
点击，返回起点也不恢复资格。确认抬起前只更新可撤销按压反馈。

## 5. 滚动与编辑

ScrollView 增加内部触摸控制器，复用当前范围、偏移与布局，不提前公开 Pan
控制器。按钮和 ScrollView 都在按下后等待；移动越过阈值后 ScrollView 申请，
按钮撤销反馈，内容跟手移动，抬起不能再点击按钮。

按 ScrollDirection 识别方向。同方向可滚动时内层优先；内层无该方向范围时
允许父层识别。内层有范围时，即使在边界也持有已经获得的交互，获胜后不移交；
与既有滚轮归属保持一致。首轮不实现惯性、回弹或滚动接力。

鼠标编辑继续按下聚焦、拖动选择。触摸编辑先等待竞争，轻点完成后才聚焦并
定位光标；滑动放弃轻点，让 ScrollView 接管。焦点在确认交互的控制器路径
复用内部方法，不新增 Widget.FocusOnClick，不按 Widget 具体类型分支。
双击选词复用既有边界；长按、选择手柄等不作为首轮前置。

## 6. Android 适配与线程

MotionEvent 的 Down/PointerDown 只建立新触点，Move 更新活动触点，Up/
PointerUp 只结束对应触点，Cancel 结束该原生输入流所有活动触点。必须使用
pointer ID，不能使用可变化的数组 index。

原生回调立即复制数据再投递 GOUI；同一原生消息中的触点在一个任务连续处理。
输入带原生宿主有效期校验，旧 Activity 的排队消息不能进入新宿主。私有输入
ABI 随平台宿主版本化，不扩展 runtime/cgo 基座。首轮不做重采样、预测或合并。

## 7. IME 后续切片

触摸稳定后另行设计移动 IME：输入快照（文本/选区/组合/版本）、提交/范围替换/
删除/选择请求、输入连接与软件键盘显隐、焦点/会话代次、原生与 GOUI 线程同步。
已有 Commit/Preedit 复用，但不能独立替代 Android InputConnection 的编辑协议。
原生 UI 线程不能与 GOUI 双向同步等待。重复轻点已聚焦输入框可重新请求键盘；
键盘出现后的内容区域变化复用原生事实与 ScrollIntoView，不任意平移窗口。

## 8. 实施与验证

待平台 Typography/Graphics 最小切片通过后依次实施：

1. PointerEvent、按触点路由、控制器隔离、取消与桌面鼠标回归。
2. 触摸点击/滚动/文本轻点；先经 Window.DispatchEvent 验证纯 GUI 状态机。
3. Android 原生输入 APK，在专用模拟器验证真实 MotionEvent 路径和混合输入。
4. 单独确定 IME 接口，再验中文组合、删除、范围替换、焦点和键盘开合。

包内测试覆盖双按钮并行、同按钮第二指、迟到事件、移出/返回、嵌套滚动、
滚动取消点击、滑过编辑框不改焦点、Hover/Tooltip 隔离、重入和销毁。
窗口程序按 Testing.md 给出中文操作说明。平台原生输入验收不以 GUI 合成事件
代替；跨平台编译不等于原生运行，桌面触屏硬件未验证时明确标注。

## 9. 参考

2026-11-04 查阅：Flutter 公共 PointerEvent 与主分支分发源码、GTK4 4.24.1
Gesture 文档、Android MotionEvent 及 InputConnection 契约。采用统一指针事实、
按输入序列路由和竞争，不照搬完整设备体系、多指识别器或编辑框架。

- [Flutter PointerEvent](https://api.flutter.dev/flutter/gestures/PointerEvent-class.html)
- [Flutter binding.dart](https://github.com/flutter/flutter/blob/master/packages/flutter/lib/src/gestures/binding.dart)
- [GTK4 Gesture](https://docs.gtk.org/gtk4/class.Gesture.html)
- [Android MotionEvent](https://developer.android.com/reference/android/view/MotionEvent)
- [Android InputConnection](https://developer.android.com/reference/android/view/inputmethod/InputConnection)
