# 触摸输入与 GUI 手势设计

状态：2026-11-09 更新既有草案，触摸事件与 GUI 适配尚未实现。平台原生 UI
主线程、GUI/UI Surface 已完成先前切片；当前先稳固 Android/iOS 平台实现，
再验证原生触摸事件，验收后才推进 GUI 手势和控件行为。本轮仅更新设计。

关联：[手势竞争](DesignGesture.md)、[平台稳固计划](PlanMobileAPI.md#7-平台实现稳固优先2026-11-09)、
[呈现宿主](DesignPresentationHosts.md)、[IME](DesignIME.md)、
[移动平台](DesignMobilePlatform.md)、[移动生命周期](DesignMobileLifecycle.md)。

## 1. 目标与分层

沿用此前确定的方向：扩展 PointerEvent 区分输入来源，复用现有 EventController
与 GTK4 风格的 Gesture 竞争机制。platform 报告原生输入事实；gui 负责命中、
传播、竞争、焦点和交互信号；ui 继续协调 Widget，不增加移动专属点击系统。

首轮覆盖手指直接触摸、多个接触的独立路由及单序列控制器，不处理多指联合
缩放/旋转、惯性/回弹、长按菜单、选择手柄、放大镜、触摸原生 DnD 或窗口移动。
笔与间接输入以后独立接入，不把尚未支持的来源伪装为鼠标或手指。
桌面原生触屏接入另行验证；事件模型可扩展不等于各桌面后端已经支持触摸。

## 2. 平台事件

拟扩展现有 PointerEvent，旧草案的 PointerKind/Kind 统一改称 PointerType。
以下为待实现 API；不表示源码已经提供这些字段：

```go
type PointerType uint8

const (
    PointerTypeMouse PointerType = iota
    PointerTypeTouch
)

type PointerID uint64

type PointerEvent struct {
    EventType   EventType
    PointerType PointerType
    ID          PointerID
    Timestamp   time.Duration

    Position  geometry.Point
    Button    PointerButton
    Buttons   PointerButtons
    Modifiers Modifiers
}
```

字段使用 PointerType，避免与现有 Type() EventType 方法冲突。PointerEvent
仍实现相同的 Event 接口，不另建触摸事件总线，不改变 EventHandler 的公开签名。

- 新增 PointerCancel，追加事件枚举以保留已有数值。PointerType 零值为 Mouse，
  现有归并鼠标使用 ID 0，已有桌面事件构造与输入路径保持兼容。
- Touch 使用非零宿主内不透明标识，从 Down 到 Up/Cancel 稳定。后端映射原生
  身份，保证活动接触不冲突；不要求连续，不承诺等于原生 ID，也不表示设备编号。
- Timestamp 来自原生单调时间线；0 表示未提供。只比较来源一致、时钟域相同的
  时间差，不与墙上时间混算。GUI 的兼容时钟兜底必须标明时间来源，不能因来源
  切换误判双击。Position 仍是宿主局部 DIP，控制器通过 EventContext 获取局部坐标。
- Touch 的 Button=None、Buttons=0，接触状态由 Down/Move/Up/Cancel 表达。
  控制器解释主操作，平台不伪造鼠标左键、点击、hover 或滚轮。
- 触摸板按系统提供的鼠标、滚轮或手势事实处理，不因由手指操作就归为 Touch。
  笔的类型及压力/倾角以后根据原生接入需求追加，不提前承诺。

## 3. 分发与生命周期（GUI 后续实现）

Dispatcher 按输入来源和 ID 区分活动序列，保存初始命中路径、竞争参与者、
获胜者和捕获归属。鼠标保持现有路径，触摸存储按需创建，不要求普通鼠标移动
构造多触点集合。

触摸 Down 命中一次；Move/Up/Cancel 沿初始路径和竞争结果分发，不随手指移动
重新命中相邻控件。即使没有识别器加入竞争，也保留普通 EventController 所需的
连续路由。控件局部坐标仍按当前有效布局换算，初始路径不是冻结布局坐标。
目标失效就取消，不把仍按住的手指移交给新目标。

鼠标 hover/光标单独维护。触摸不覆盖鼠标位置、不产生 Hover、不启动 Tooltip
等待；触摸按下可关闭已有 Tooltip。逻辑焦点与键盘状态继续属于宿主。

隐藏、原生取消、模态接管、参与者失效或宿主销毁时按既有 GUI 规则取消，完成
和取消互斥且至多一次。遍历活动序列时保存稳定快照，每次外部回调后复核对象、
模态边界和序列代次，不能重入删除新序列。结束后迟到的 Move/Up 不得重新命中。
平台报告原生输入撤销；GUI 对宿主可交互状态的策略性取消仍留在 GUI。

## 4. 手势协议（沿用现有机制）

复用 GestureController、JoinGesture、Claim、Reject、GestureAccepted 和
GestureCanceled，不新增公共 Arena。沿用同事件深层 Widget 优先、同 Widget
控制器注册顺序优先的同步裁决规则。StopPropagation 仍只影响当前事件传播，
不替代整条接触序列的竞争归属。

首版每个 GestureController 实例同时只处理一条接触序列，与现有 Pressed/
Dragging 及单次 Begin/Update/End 信号一致。Dispatcher 在交付前隔离忙碌
控制器的其他触点，不能让第二个 Down 覆盖内部状态：

- 两根手指可分别操作不同按钮。
- 同一个按钮忙时不接纳另一根手指；鼠标与触摸同时使用同一控制器也遵守此规则。
- 当前接触结束后，不接管另一根已经按住的手指；必须有新的 Down。
- 普通 EventController 可观察原始多触点，由自定义实现管理自己的状态。
  多指联合识别器及组内合作协议以后按实际需求设计。

默认 Click 主操作接受鼠标左键及 Touch；右键配置不匹配 Touch。触摸点击初始
移动容差为 8 DIP，属于 GUI 策略；鼠标保持当前行为。超过容差后放弃点击，
返回起点也不恢复资格。确认抬起前只更新可撤销的按压反馈，不执行业务点击。
触摸按压通过自身序列和位置判断，不依赖鼠标 crossing 通知。

## 5. 滚动、焦点与控件（GUI 后续实现）

ScrollView 增加内部触摸控制器，复用当前范围、偏移和布局。按钮与 ScrollView
在按下后等待；移动越过阈值后 ScrollView 申请，按钮撤销反馈，内容跟手移动，
抬起不能再点击按钮。不提前公开另一套 Pan 容器或复制滚动布局机制。

按 ScrollDirection 和可滚动范围识别方向。同方向内层可滚动时优先；内层无该
方向范围时允许父层识别。内层有范围时即使在边界也保持已经获得的交互，获胜后
不移交；与既有滚轮归属一致。首轮不实现惯性、回弹或滚动接力。

鼠标继续按下聚焦、拖动选择。触摸 Down 不执行 Dispatcher 现有的立即聚焦；
轻点确认后由交互控制器沿已有焦点路径聚焦，滚动本身不转移焦点。文本轻点确认
后再定位光标；滑过编辑框不改变光标/选区，不抢 ScrollView 的交互。不新增
Widget.FocusOnClick，不按具体 Widget 类型在 Dispatcher 中分支。
双击选词复用已有文字边界，长按、选择手柄和系统键盘不属于这一切片。

DragController 的明确阈值设置继续有效，不能把用户设置的立即拖动偷偷改成延迟
拖动。ScrollBar 等直接拖动区域与 ScrollView 内容滑动分别判断。控件迁移时同步
检查 Switch 的水平拖动与纵向滚动、SplitView、可选择 Label、列表/树/表格行、
TabBar 和 Dialog 遮罩；不能仅修改 Button 后就宣称全部控件支持触摸。
原生 DnD/窗口接管继续按实际支持来源处理，不因通用 Drag 接受触摸就自动启用。

Snapshot 复用现有 Pressed、Focused、Hovered 和动作信息；触摸按压不伪造 Hover。
UI 沿用现有控件声明，上层新增 API 要由具体缺口证明必要，不在平台验证时提前加入。

## 6. 平台入口与线程

原生 UI 主线程已经承载 GOUI 调度，旧草案的“复制后投递另一个 GOUI 线程”作废。
Android 主 Looper 与 iOS UIKit 回调在其所属线程提取普通 Go 事件值，再同步交付
当前已绑定宿主；不为输入额外建立 goroutine、消息循环或同步等待桥。

同一原生消息有多个触点时，先提取这批事实，再逐个交付；每次外部回调后复核
宿主与原生输入流有效期。原生事件、JNI 局部引用、UIKit 回调对象只按原生契约
借用，不能把未取得所有权的指针留给稍后执行的任务。未绑定或已经失效的宿主
不积压输入，恢复后不重放旧 Down。

普通绘制目标代次变化不等于原生触摸取消。平台在原生输入流被取消、撤销或失去
归属时正确结束接触；GUI 再按可见性、焦点、几何和模态事实决定手势是否中断。
原生绑定只表达 API；事件解码、ID 映射与单位换算属于 Android/iOS 适配包。

触摸入口使用现有 Go/汇编 FFI、公开 NDK 与 UIKit 方法，不要求新增 C/Objective-C/
Java 宿主协议，也不预设修改基座导出、链接器或原生预编译组件。若验证发现真实
缺口，先记录证据，再单独设计基座调整。首轮不做重采样、预测或历史事件合并。

## 7. Android 原生输入（平台先行）

复用 NativeActivity 的 AInputQueue 和主 Looper，补齐 libandroid 的事件读取绑定。
当前实现只取出并结束队列事件，尚未交付触摸，不能将已有 GUI 合成输入当作该
入口已经工作。

- Down/PointerDown 只建立新接触，Move 更新活动接触，Up/PointerUp 只结束
  对应接触；Cancel 结束对应原生流的全部活动接触。
- 使用 pointer ID 追踪身份，action index 只定位本次变化的条目；数组 index 可以
  变化，不能作为公开接触 ID。Up/Cancel 后结束 ID 映射。
- 核对取消标记和支持的 SDK 范围，不能把被取消的结束报告为正常 Up。
- 坐标取原生内容坐标，结合已有尺寸/缩放事实转换为 DIP；核对 NativeActivity
  的实际内容原点，不用桌面 raw 坐标或写死的状态栏偏移补偿。
- 遵守 preDispatch/finishEvent 契约。原生 handled 表示接管了支持的输入流，
  不等同于 GUI StopPropagation，也不新增要求 Widget 回传 handled 的协议。
  未支持的输入按原生默认路径处理，不统一吞掉系统按键。
- 队列销毁、Activity 更换和释放期间核对事件及队列有效期；旧队列事件不能进入
  新宿主。结束与清理必须能应对事件处理期间宿主被释放。

先用 platform-only 程序观察原生事件、绘制接触标记并断言单位和生命周期。
平台不识别按钮点击、滚动阈值或触摸聚焦。

## 8. iOS 原生输入（平台先行）

内容 UIView 通过 Go 注册触摸回调，复用 frameworks/uikit 的类、selector 与方法
绑定规范；开启 multipleTouchEnabled，接收 touchesBegan/Moved/Ended/Cancelled。
当前内容视图尚未注册这些处理，不以 UIKit 自动提供点击作为实现依据。

- 用 UITouch 的原生身份映射不透明 ID；只对本次变化的触点交付事件，稳定保留
  从开始到结束的对应关系。不要从 NSSet 枚举顺序推导接触身份或公共优先级。
- 使用相对内容 UIView 的位置；UIKit points 与 GOUI 逻辑单位的关系以现有缩放
  契约验证，不能再次乘屏幕倍率。timestamp 转换为同源单调时间差。
- Cancelled 必须结束对应接触；后台、失活、视图/Scene 脱离时检查系统回调与
  活动接触的清理关系，不从一次 drawable 更换推断所有 UITouch 已结束。
- 手指、Pencil 与间接指针按实际来源区分，首版不伪装尚未接入的来源。
- Widget 手势策略不放入 UIKit 绑定；GOUI 的竞争与点击仍由 GUI 解释。

先用 platform-only App 验证 UIKit 回调、坐标、取消与原生线程，保持 Software/
GLES 的目标和状态恢复检查。模拟器注入与物理设备的支持范围分别记录。

## 9. IME 后续切片

IME 单独设计，仍按“平台验证 → GUI 集成”的顺序推进：输入快照（文本/选区/
组合/版本）、提交/范围替换/删除/选择请求、输入连接与软件键盘显隐、焦点/会话
代次及原生同步查询。已有 Commit/Preedit 复用，但不能独立替代 Android
InputConnection 或 UIKit 文本输入协议。

原生 UI 线程不能与自身同步等待。重复轻点已聚焦输入框可重新请求键盘；键盘
出现后的区域变化复用原生事实与 ScrollIntoView，不任意平移窗口。本轮不扩展
IME API，也不把程序化 CommitText 通过记为系统键盘/输入法通过。

## 10. 实施顺序与验收边界

| 阶段 | 工作范围 | 进入下一阶段的条件 |
| --- | --- | --- |
| P0 当前优先 | 稳固现有平台结构、绑定、主线程、宿主和绘制目标/资源生命周期 | 复核记录与平台回归通过，已知缺口明确；见 PlanMobileAPI 第 7 节 |
| P1 平台原生触摸 | PointerEvent 契约、Android/iOS 原生解码、ID/取消/坐标、platform-only 例程 | 两端原生输入链验证通过；多指/真机缺口单列，不以 GUI 合成输入替代 |
| P2 GUI 触摸 | 按触点路由、控制器隔离、点击/滚动/聚焦、现有控件检查 | 逻辑回归、桌面鼠标回归和已支持的原生操作场景通过 |
| P3 后续专题 | IME、长按/选择、惯性、笔和联合手势等按需求拆分 | 每项先设计和验证平台事实，再接入上层 |

P0/P1 期间暂不扩展 GUI 手势、控件行为或 UI 声明。本轮不要求把尚未支持的
Popup、系统服务、安全区域等全部补齐；不支持就按既有契约报告，避免扩大稳固
工作的边界。既有 GUI/UI Surface 保留，不回退已验收成果。

不依赖桌面的 ID 映射、动作解码和取消状态测试放所属包；真实事件循环、输入
队列或 UIKit 视图用例放 tests/window/platform，按 Testing.md 写中文操作说明。
平台例程覆盖单指、多指、指针索引重排、结束/取消、旋转、前后台、宿主释放与
重新绑定，并对比原生坐标、DIP 和屏幕标记。按设备能力分别记录通过与未执行。

后续 GUI 包内覆盖双按钮并行、同控件第二指、鼠标/触摸并行、迟到事件、初始
路径保持、滚动取消点击、滑过编辑框不改焦点、Hover/Tooltip 隔离、模态/销毁
重入。通过 Surface.DispatchEvent 验证状态机，再验证原生输入到同一路径。
验证 Snapshot 的状态与角色，不增加语义动作执行捷径。

执行 go test ./...，按风险补充 race 和受影响目标编译。分别记录平台事实、GUI
合成测试、真实原生输入、模拟器与真机；平台编译或 App 启动不等于输入可用。
桌面鼠标回归不能因新增触摸跳过，未验证桌面触屏硬件时不得声明已支持。

## 11. 参考与取舍

2026-11-08/09 复核以下公开契约；Flutter 分发还核对了固定版本源码。它们为设计
依据，以上触摸行为是 GOUI 的待实现方案，不是运行结果。

- [GTK4 4.24.1 GestureSingle](https://docs.gtk.org/gtk4/class.GestureSingle.html)：
  同时支持主鼠标按钮与触摸，交互期间坚持当前序列。采用单实例单序列的控制器
  模型，不直接照搬 GObject、原生事件重放或识别器分组体系。
- [GTK4 Gesture](https://docs.gtk.org/gtk4/class.Gesture.html)：
  识别器参与序列竞争与取消。复用 GOUI 已实现的参与协议和同步裁决规则。
- [Flutter 3.29.0 binding.dart](https://github.com/flutter/flutter/blob/3.29.0/packages/flutter/lib/src/gestures/binding.dart)：
  保存 Down 的命中结果，后续按 pointer 路由，在结束或取消时移除。采用初始
  路径与序列生命周期，不照搬 Flutter 的事件类层次或全部 Arena 策略。
- [Android NDK Input](https://developer.android.com/ndk/reference/group/input)、
  [MotionEvent](https://developer.android.com/reference/android/view/MotionEvent)：
  核对动作、pointer ID/index、时间、坐标及队列确认；具体 SDK 差异在绑定时复核。
- [Apple Handling touches](https://developer.apple.com/documentation/uikit/handling-touches-in-your-view)：
  UIView 通过触摸阶段处理接触，取消是独立结束路径。具体 UIKit 绑定和多点
  回调生命周期须在平台例程验证，不从 API 名称推断运行结果。
- [Android InputConnection](https://developer.android.com/reference/android/view/inputmethod/InputConnection)：
  只作为后续 IME 设计入口，不混入本轮原生触摸或平台稳固的完成条件。
