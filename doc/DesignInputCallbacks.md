# 控制器输入回调

2026-11-14：本契约统一现有鼠标、键盘、滚轮和菜单请求回调，不改变平台事件、
EventDispatcher 的命中和传播、当前手势裁决、捕获或原生拖放接管规则。
真实与模拟输入继续经过 Window.DispatchEvent。完整手势扩展仍按
[DesignGesture](DesignGesture.md) 与 [触摸计划](DesignTouchInput.md) 推进。

## 数据与上下文

控制器发出的信号使用普通 Go 值表达本次输入信息。ClickInfo、MotionInfo、DragInfo、
WheelInfo 的 Position 均为当前控件局部 DIP；原始 events.PointerEvent／WheelEvent
中的 Position 保持宿主局部 DIP，不原地修改，避免父子控制器重复转换。
按钮掩码、modifier、滚轮 delta／mode 保留原始含义，只有位置转换到控件坐标。

EventContext.Event／Position 继续用于自定义 EventController 的原始输入处理；
使用控制器信号时直接读取 Info，无须对 ctx.Event 做类型断言。ctx.StopPropagation
只影响当前尚在进行的传播，不取得手势所有权，也不能撤销已经执行的传播。
EventContext 仅在同步回调内有效，不保存为异步输入响应。

## Click

```go
type ClickInfo struct {
    Position  geometry.Point
    Button    events.PointerButton
    Buttons   events.PointerButtons
    Modifiers events.Modifiers
    Count     int
}
```

- ConnectPressed(func(EventContext, ClickInfo))：真实按下时立即通知。在手势裁决前
  提供次数、位置与 modifier；不等释放，不保证以后获得 Click 或 Drag 所有权。
- ConnectReleased(func(EventContext, ClickInfo))：活跃按下收到匹配的真实抬起。
  在控件外抬起也可通知；这不表示点击成功。同一按下最多通知一次释放或取消。
- ConnectClicked(func(EventContext, ClickInfo))：符合既有点击条件并完成当前竞争后
  通知一次。取代旧 ConnectClick(ctx,count) 和 ConnectClicked(ctx) 两条完成信号。
- ConnectCanceled(func(GestureCancelReason))：按下在抬起前被现有竞争或生命周期路径
  取消；不伪造 Released、不携带虚构坐标。已报告 Released 后的清理不再次报告取消。
- ConnectPressedChanged(func(bool))：只报告按压反馈状态。Down、抬起、移出、Reset
  均可改变它，不再混用带空 EventContext 的输入通知。Pressed() 与通知的值一致。

先更新状态再通知。正常顺序为 state-on、Pressed、state-off、Released、Clicked；
移出可以提前 state-off。取消清理为 state-off（若需要）、Canceled。任一外部回调
可能 Reset、卸载、销毁或开始另一序列，后续通知逐个检查代次、对象及参与者有效性，
不能让旧发射继续调用后来才适用的业务处理。Released 回调也可以终止后续 Clicked。
SetButton 中的取消回调若再次改变按钮，较晚修改保留，较早 setter 不覆盖它。

Count 在 Down 时确定，并由对应的 Released／Clicked 沿用；其他输入字段来自本次
阶段，例如 Shift 按下后 Ctrl 抬起，两个阶段保留各自的 modifier。次数是控制器局部
序列信息，不是平台事实：同一按钮相邻按下相隔不超过 500 ms，宿主局部位置的
欧氏距离不超过 4 DIP 时递增；超时、时间回退、距离超限或按钮变化从 1 开始。
取消、Reset、失败释放等沿既有清理路径复位。计数依据 GUI 处理输入时的单调时间，
本轮不新增原生事件时间戳，也不声称还原原生队列中尚未分发的时间间隔。

通用计数可为 1、2、3、4……，第二击后不自动清空；已经完成的第一击立即通知，
不会因未来多击被撤回。需要“单击与双击绝对互斥”的业务仍不能直接据此实现。
选词、选逻辑行、选段落或全选由控件决定，次数本身不触发这些操作。

TextInput／TextView 保留自己的选择控制器，使用同一私有计数算法；三击时选逻辑行，
第四击重新开始。模型／revision 变化、键盘／滚轮、实际拖动等原有失效规则保留，
不通过额外挂一个 Click 控制器引入竞争。不存在公开 ClickSequence 或 PressCount getter。
Button 等 Widget 的无参数业务点击信号保持现状；其内部改为订阅带 ClickInfo 的完成
通知，按压外观改为订阅 PressedChanged。UI OnClick 无须额外接收输入数据。

## 其他控制器

| 回调 | 类型化信息与约束 |
| --- | --- |
| Motion.ConnectMotion | func(EventContext, MotionInfo)；增加 Buttons，位置局部化 |
| Drag.ConnectBegin／Update／End | func(EventContext, DragInfo)；含 Position、Button、Buttons、Modifiers |
| Drag.ConnectCanceled | func(GestureCancelReason)，取代无参数 ConnectCancel；只有已开始的拖动报告取消 |
| Wheel.ConnectScroll | func(EventContext, WheelInfo)；位置局部化，delta／mode 不变 |
| Key.ConnectKeyDown／KeyUp | 保留 func(EventContext, events.KeyEvent)；native PreventDefault 的同步响应契约不变 |
| ContextMenu.ConnectRequest | func(EventContext, ContextMenuInfo) |
| ui.OnContextMenu | 薄转发同一 EventContext 与 ContextMenuInfo，保留声明式连接替换及卸载清理 |

Drag Begin 报告原 Down 的位置、Buttons、Modifiers；阈值触发的当前 Move 仍在同一次
分发紧接着产生 Update，不丢失越过阈值的位移。Update／End 报告当前输入；Button
始终标识驱动拖动的按钮，不能因为 Move 的原始 Button=None 而丢失来源。
拖动完成与取消互斥；原生 DragSource.Begin 成功接管 GUI 路由后仍不再发 pointer
End／Canceled。接管入口继续要求在已获胜 Drag 的 Update 同步范围内调用。

ContextMenuInfo 的 Position、HasPosition、Modifiers 明确区分指针与键盘请求。
键盘请求 HasPosition=false，控件选择光标、当前行或选择范围作为锚点；不是把 (0,0)
当作实际请求位置。Windows 仍在次键释放时请求，Linux／macOS 仍按下时请求，
macOS Control+左键分类与 Shift+F10／Menu 键语义保持现状。

## 迁移与验证

直接迁移已有调用，不保留第二套旧控制器接口。旧 ConnectPressed(ctx,bool) 若只更新
外观，改为 ConnectPressedChanged(bool)；若处理真实 Down，则改为 ConnectPressed
并读取 ClickInfo。旧 ConnectClick 的 count 改为 info.Count，modifier 改读 info.Modifiers。
CancelPointer 不再接收无意义的 EventContext。拖动、滚轮及 UI 菜单回调同步迁移。

包内用正常窗口分发验证即时次数、阶段顺序、局部坐标、modifier／Buttons、边界、
第三次按下转拖动、Reset／取消、回调重入及控件状态。原 UI 点击夹具原来对指针输入
固定返回“没有位置”，现改为经过 EventDispatcher 的有布局点击，保留原绑定替换与
卸载断言。树／表格完成通知不再解包原始 PointerEvent；已有测试继续覆盖选区、
激活、排序、滚动、拖拽接管及语义快照。源指针事件的坐标与公共平台接口不变。

`tests/window/gui/input_callbacks` 的真实桌面流程补充：第三次按下保持未释放时的次数，
Click 取消后继续 Drag，真实释放与 Reset 的区别，Shift、滚轮、指针／键盘菜单请求，
以及文本逻辑行上下拖选。窗口启动不视为通过；逐项检查日志、状态断言和截图。

参考 GTK **4.18.6** [GtkGestureClick 的 pressed](https://github.com/GNOME/gtk/blob/4.18.6/gtk/gtkgestureclick.c#L228)
和 released 两阶段表达，以及 Avalonia **11.3.0**
[PointerPressedEventArgs.ClickCount](https://github.com/AvaloniaUI/Avalonia/blob/11.3.0/src/Avalonia.Base/Input/PointerEventArgs.cs#L158)
的类型化数据。Qt **6.8.3**
[文本控件私有三击识别](https://github.com/qt/qtbase/blob/v6.8.3/src/widgets/widgets/qwidgettextcontrol.cpp#L1599)
说明选择单位仍由控件决定。以上为源码参考，不复制对象层次、输入压缩或手势仲裁，
也未进行跨框架桌面实测；GOUI 的公共承诺以本文及源码为准。
