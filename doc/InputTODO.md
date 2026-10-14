# goui 输入后续事项

> 状态（2026-10-14）：基础 Pointer／Wheel／Key、IME、Clipboard 与手势竞争已实现。Touch／Pen 等规划集中在 [未来路线](Roadmap.md#future-input)。以下保留输入事实和扩展边界，不把早期顺序当作当前待办。\
> 适用范围：Pointer、Wheel、Key 后续迭代\
> 核心原则：每一步只实现当前阶段必须验证的语义，不提前扩大事件结构和平台查询范围。

## 1. Pointer 命名与触摸支持

当前事件名使用 `PointerEnter`、`PointerLeave`、`PointerMove`、`PointerDown`、`PointerUp` 是合适的。`Pointer` 比 `Mouse` 更适合作为长期公共命名，因为它可以覆盖鼠标、触摸和笔等指针类输入。

但现在不应提前加入触摸字段，例如：

- `PointerID`
- `PointerKind`
- `Primary`
- 压力、倾斜、接触面积
- 多点触控状态

原因：

- 第一阶段只验证鼠标输入；
- 触摸和笔的原生模型与鼠标差异较大；
- 过早加入字段会让测试范围扩大，但暂时没有平台映射验证；
- 多点触控还需要 GUI 层明确 capture、gesture、hit-test 和 cancel 语义。

当前 `PointerEvent` 保持鼠标可用的最小结构：

```go
type PointerEvent struct {
    EventType EventType
    Position  geometry.Point
    Button    PointerButton
    Buttons   PointerButtons
    Modifiers Modifiers
}
```

后续真正实现触摸时，再评估是否扩展为：

```go
type PointerEvent struct {
    EventType EventType
    PointerID PointerID
    Kind      PointerKind
    Primary   bool
    Position  geometry.Point
    Button    PointerButton
    Buttons   PointerButtons
    Modifiers Modifiers
}
```

这个扩展应和触摸平台映射、GUI capture、cancel 和 gesture 设计一起提交，不在鼠标 Pointer 阶段提前加入。

## 2. Pointer 阶段的修饰键来源

Pointer 事件可以携带 `Modifiers` 字段，但 Pointer 阶段不应为了填满所有修饰键而主动查询全局键盘状态。

### 2.1 Windows

Windows 鼠标消息的 `wParam` 已经携带事件发生时的按钮和部分修饰键状态：

- `MK_LBUTTON`
- `MK_RBUTTON`
- `MK_MBUTTON`
- `MK_XBUTTON1`
- `MK_XBUTTON2`
- `MK_SHIFT`
- `MK_CONTROL`

Pointer 阶段应直接从 `wParam` 解码：

```go
func pointerButtons(wParam winapi.WPARAM) events.PointerButtons
func pointerModifiers(wParam winapi.WPARAM) events.Modifiers
```

不要在 pointer hot path 中调用 `GetKeyState` 查询：

- Alt
- Super
- CapsLock
- NumLock
- 任意其他键盘状态

当前 Windows Pointer 仍只取事件携带状态，不因 KeyEvent 已实现而伪造鼠标修饰键。这样可以避免每个 pointer event 额外产生多次系统调用，也避免 pointer 阶段承担键盘状态一致性问题。

### 2.2 Linux/X11

X11 的 pointer 事件结构中已有 `state` 字段。Pointer 阶段应只使用这个事件自带状态：

- button mask 转为 `PointerButtons`
- `ShiftMask` 转为 `ModifierShift`
- `ControlMask` 转为 `ModifierControl`
- 可明确识别的 `Mod1Mask` 可暂时转为 `ModifierAlt`

不要为了补齐 Super、CapsLock、NumLock 等状态主动查询 keymap 或维护额外键盘状态。当前键盘映射已处理 modifier map 与 AltGraph；Pointer 仍从事件 state 读取。

### 2.3 macOS

macOS 的 `NSEvent` 已经携带 `modifierFlags`。Pointer 阶段可以直接使用该事件自带 flags。

不要为了 Pointer 阶段额外维护完整键盘状态。KeyEvent 已处理 flagsChanged 与 Option／Command 事实；侧别与锁定键的验证范围见 [快捷键](DesignShortcuts.md)。

## 3. 原阶段顺序与当前状态

以下前五项已实现，仅 Touch／Pen 尚需具体设计与平台映射验证：

1. Pointer：只完成鼠标移动、进入、离开、按钮 down/up、事件自带 modifier/button 状态。
2. Wheel：单独增加 `WheelEvent`，验证 line/pixel delta 和方向语义。
3. Key：单独增加 `KeyEvent`，建立统一键盘状态和更完整修饰键语义。
4. GUI TextInputContext：基于 KeyEvent 接入 IME，不加入 platform 通用事件。
5. Clipboard：作为 GUI 服务接口接入，不加入 platform 通用事件。
6. Touch/Pen：在 GUI capture、cancel、gesture 语义明确后，再扩展 Pointer 结构。

每一步都应保持提交范围和测试范围可控。

## 4. Key 阶段的锁定键语义

`CapsLock`、`NumLock`、`ScrollLock` 应作为按键事件上报，例如：

- `KeyCapsLock`
- `KeyNumLock`
- `KeyScrollLock`

它们不应作为 `Modifiers` 上报。`Modifiers` 只表示按住型修饰键：

- `ModifierShift`
- `ModifierControl`
- `ModifierAlt`
- Windows 的 ModifierWin、macOS 的 ModifierOption／ModifierCommand、X11 的 ModifierSuper；不同事实不互相别名化

原因：

- 锁定键是切换状态，不是按住期间改变输入语义的普通 modifier；
- 不同平台对锁定状态的事件时序和可见状态不同；
- GUI 层如果需要显示或查询锁定状态，应通过后续 KeyboardState/InputContext 设计处理，而不是混入每个 Pointer/Wheel/Key event 的 modifier bitmask。

### 4.1 X11 小键盘与 NumLock

X11 的 `XLookupKeysym(event, 0)` 不会按 NumLock 自动把 keypad navigation keysym 转为 keypad number keysym。例如小键盘 7 可能始终返回 `KP_Home`。

平台初始化时应通过 `XKeysymToKeycode(XK_Num_Lock)` 和 `XGetModifierMapping` 找到 NumLock 实际对应的 modifier mask。后续读取事件自带的 `state`，在该 mask 置位时，将 keypad navigation keysym 转为对应数字：

- `KP_Home` -> `KeyNumpad7`
- `KP_End` -> `KeyNumpad1`
- `KP_Insert` -> `KeyNumpad0`
- `KP_Delete` -> `KeyNumpadDecimal`

这只是 Key 映射规则，不代表 `NumLock` 重新成为 `Modifiers` 的一部分。
