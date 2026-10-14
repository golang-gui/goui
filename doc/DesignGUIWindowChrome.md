# GUI Window Chrome: window-owned controls

This contract supersedes the freely composed WindowControls design. The existing
platform Chrome implementation passed the user's three-platform acceptance.
This slice changes GUI ownership and layout, not native chrome mechanisms.

## Ownership

Window owns one private controls group in Integrated mode. Applications neither
construct this group nor insert it into their Widget trees. Native controls keep
OS input and accessibility; custom controls use ordinary Button controllers.
The custom group is a separate window decoration tree, painted after application
content and hit-tested first by the same EventDispatcher. Window.Widget remains
exactly the application's root; no hidden root or reduced client rectangle.
Window snapshots expose custom controls separately, without inventing native
button children. Window destruction releases both trees.

WindowChrome remains a non-nil, window-owned signal service. It communicates
through ConnectInfo, ConnectQueryRegion and ConnectQueryControls. Queries use
borrowed pointer results, run synchronously in connection order, and may be
overridden by later subscribers. They must not mutate layout, dispatch events,
destroy the window or retain result pointers. Native input never invokes GUI
button commands directly. Modal input and existing pointer capture still apply.

## Controls geometry

ChromeInfo's only geometry field is ControlsBounds, the occupied button union
in window-client DIP. Native query availability is private to Chrome. A temporary
failure preserves the last reported rectangle and does not emit a new notification;
disabled chrome reports an empty rectangle. Native input remains with the OS.

ChromeControls contains only Height (float32, DIP). It asks how tall the available
top row is, not how large the buttons should be. No WindowOptions/Window/Chrome
TitleBarHeight property exists. Zero means no row answered: native positioning
returns to its default, and custom controls use their own height. Invalid
negative/non-finite answers are ignored.

Windows custom buttons have a 46-by-32 DIP intrinsic size and stay on the right. Native
buttons keep their observed size and horizontal anchor; Chrome does not replace
the native left inset with a fixed value. This also preserves the reservation
observed before first Show. Chrome
centers the group vertically in the answered height; a row shorter than the
intrinsic group cannot move native buttons above the client origin, so the origin
is clamped to zero. Custom painting remains clipped to the client area.
Fullscreen hides custom controls; unavailable native controls are not replaced.

Frame order is explicit:

1. Refresh native bounds / seed custom bounds at the current window width.
2. Measure and arrange application content normally.
3. Emit QueryControls once, reading completed allocations. The relevant HeaderBar
   answers its actual height; later application subscribers can override it.
4. Resolve the centered controls bounds and publish only changed information.
5. If changed bounds invalidate content reservations, run at most one additional
   content layout in this frame, without querying height again.
6. Arrange the window-owned custom tree and paint both trees in one frame.

Native position requests are deduplicated, including failed requests; a changed
position or native change permits retry. Actual observations remain authoritative.
Chrome never directly arranges the controls Widget. HeaderBar measurement does
not depend on the controls' vertical position, preventing a height feedback loop.
Notifications only invalidate layout; they never recursively measure/arrange.

## HeaderBar

HeaderBar is a single-child container, not a built-in title or horizontal box.
Use SetChild/Child and ordinary LinearBox/Label/Button composition. Multiple
HeaderBars can share a window. Each connects on mount and disconnects on unmount.
40 DIP is a default minimum on the HeaderBar Widget, not a window property; content,
SetMinSize/SetMaxSize and parent constraints determine its actual height.

A visible, arranged HeaderBar answers QueryControls only if it touches the top of
the root content allocation and covers the controls' outer horizontal anchor (left for native,
right for custom). Covered headers do not answer. Other split segments and lower
rows only consume ControlsBounds for avoidance. All callbacks still run in
connection order; this selection rule belongs to HeaderBar, not a Chrome registry.

The HeaderBar supplying this height must remain a full-width top row (or the
relevant split segment), not be displaced by padding on a surrounding content
box. Put application margins on a separate body container below it; use
HeaderBar.Padding for its own content. A lower/application-inset HeaderBar must not
silently reposition the window controls. Linux GUI-owned perimeter allocation is
not application padding: the HeaderBar follows that root origin automatically.
The filemanager example follows this composition.

The background fills the HeaderBar's complete rectangle. During Arrange it
intersects ControlsBounds with its own window rectangle and reserves space on
the button side for its child. Existing padding and occupied extent combine by
max, not addition. Other segments and rows without intersection keep ordinary
padding. Arrange remeasures the child with the remaining constraint before
arranging it; measurement does not guess an as-yet unknown absolute position.
The parent's constraints always win, even in very narrow windows. This is a
header-row rule, not arbitrary obstacle packing. Arbitrary transforms, central
obstacles and automatic safe-area propagation are outside this slice.

Custom widgets can subscribe to ConnectInfo and consume the same rectangle.
No global padding, clip, SafeArea service or new TitleBar container is added.

## Drag regions and picking

HeaderBar connects to Chrome's existing region query. It rejects covered content
and window controls, then classifies the actual hit path up to and including
itself. Padding is part of its background, not a synthetic resize border.

- Any focusable widget on the path keeps client input.
- No controllers, or only the concrete built-in MotionEventController,
  KeyEventController and DropTarget types, permit dragging. DropTarget handles
  native drag offers rather than ordinary pointer presses; registering a drop
  destination must not disable an otherwise passive caption region.
- Click, Drag, Wheel and unknown/custom controllers conservatively keep client
  input, regardless of callback registration or current gesture state.
- Only the hit path matters: a neighboring button does not block a passive label,
  but a label inside a button cannot bypass its parent's controllers.

This is a HeaderBar-local default policy for common cases, not a universal
interaction detector. It neither executes controllers nor reads Snapshot, and
adds no public Widget/EventController metadata or window-drag properties.
ConnectDragRegion(point, *drag) is emitted after classification in HeaderBar-local
DIP. Applications may allow or deny special/custom regions; later callbacks
override earlier ones. Queries are synchronous/read-only and never execute input
handlers to predict their behavior.
gui.Pick(widget, point) queries the same child-order, visibility and clipping rules as real
input, using widget-local coordinates; it needs no hidden WidgetBase self pointer.
Subtree picking cannot detect an external
covering sibling. HeaderBar checks the window-level hit before its local query.

## Declarative binding

ui.HeaderBar(child) is a thin single-child binding with Child, Padding and
Captions modifiers, plus the existing shared ViewBase modifiers. Nil removes
the child. GUI owns controls avoidance, width/height layout and automatic drag
classification. Unset Padding/MinHeight restore the GUI constructor defaults;
there is no UI title, controls slot or window titlebar-height setting.

Captions additionally treats exact, nonempty IDs of actually hit widgets as
caption regions (ui.ID maps to gui.Widget.ID). It does not match ancestors or
look up another HeaderBar's subtree. Matching an interactive target explicitly
gives that region to the caption behavior, including window dragging. Empty or
unset lists leave automatic classification intact; repeated calls replace the list.

```go
ui.HeaderBar(
    ui.HBox(
        ui.Label("Project"), // automatically draggable
        customView.ID("extra-caption"),
        ui.Button("Save").OnClick(save),
    ),
).Captions("extra-caption")
```

The binding connects to ConnectDragRegion once on mount, reads the current IDs
from its persistent state, and disconnects/clears them on unmount. ID matching
uses gui.Pick inside that signal, after GUI has checked occlusion and controls.
It needs no cached widgets, rectangles or snapshot invalidation. There is no
UI OnDragRegion coordinate callback or additional GUI caption registration API.

## Scope and verification

### Special-purpose None windows

Prefer Native and Integrated for normal application windows. None is intended
for special requirements such as transparent custom surfaces, not a promise of
complete native decoration behavior. On desktop None, Chrome enables region
queries without creating controls or reserving a titlebar. HeaderBar can supply
drag regions; callers can answer Top/Bottom/Left/Right and four corner roles via
ConnectQueryRegion. Callers own cursor feedback and the shape of these regions.
Transparency never determines input regions automatically.

Chrome uses native SetHitTest on Windows; after ErrUnsupported it uses explicit
BeginMove/BeginResize on X11/macOS. Resize starts on the original press; Caption
owns its press and waits for native held motion beyond 4 DIP before BeginMove.
Even a rejected move does not leak an unmatched release into content. Resize
failure still leaves ordinary input intact. No screen-coordinate or arbitrary
position API is added to GUI.
macOS BeginResize is the bounded backend adaptation documented in
DesignWindowHitTest.md, not a simulated native HitTest service.

Menu keeps GOUI's common style. Special Popover appearances use Transparent and
custom content painting; neither transparency nor None inserts round corners,
shadows, automatic resize zones or input passthrough.

Linux GUI-drawn Integrated is implemented as described below. Caption double-click
now works on the non-native query path, including HeaderBar and custom Caption
regions in None. It requests Normal -> Maximized or Maximized -> Normal; other
states are ignored. macOS uses the existing zoom mapping. Windows retains native
caption behavior without a duplicate GUI state request.

The private GUI gesture policy is 500ms between downs and 4 DIP per axis. The
first press must have completed without dragging/holding too long; the second
down must also hit Caption. A double-click consumes the second press/release
and clears the sequence. Other buttons, Client regions, focus/size/state changes,
modal input and leaving the window cancel the candidate. No public double-click
signal or settings API is introduced. Native Windows thresholds remain system-owned.

Example: `go run ./example/gui_window_chrome -none -transparent`. The example
registers eight resize zones after HeaderBars, and deliberately leaves cursor
feedback unchanged. The alpha surface still has its normal styled background.
The public cursor set includes horizontal, vertical and both diagonal resize
shapes. None callers may select them explicitly; automatic feedback is confined
to GUI-drawn Integrated.

Tests cover all eight region mappings, explicit resize routing, no duplicate
native request, no automatic None controls, failed requests, and reentrant
destruction. Portable Cocoa geometry tests cover all edges, minimums, negative
screen origins and fractional points. macOS native drag/render acceptance must
still be performed on hardware, including crossing screens and outside-window up.

Native remains the default. Integrated uses native support on Windows/macOS
and the GUI-owned transparent frame on Linux/X11, falling back as documented
below. State reports facts and requests remain best effort. No changes to Windows
native hit testing or macOS BeginMove. No public WindowControls constructor or
automatic title.

Tests cover ownership without content, content replacement, button hit/click and
close veto, hover crossing between trees, snapshots and destruction, native
position deduplication/failure, custom resize/fullscreen, multiple HeaderBars,
left/right/no intersection, narrow widths, remount, drag query ordering and Pick.
Height tests cover dynamic header layout, unequal split rows, removal/fallback,
invalid answers, centering with unchanged button size, and bounded layout passes.
Drag tests cover the controller policy, full ancestor paths, live controller
changes and ordered overrides. UI tests cover child reconciliation, default
restoration, exact-name matching, rebuilt names and signal cleanup.
Run go test ./..., race checks and Windows/macOS cross-compilation. Repeat real
Windows Snap/button and macOS traffic-light/drag/HiDPI acceptance for this GUI
slice; the prior platform acceptance does not verify these new layout rules.

## Example

Run go run ./example/gui_window_chrome. The application creates two HeaderBars
with ordinary child rows and never constructs or positions controls. Passive
labels/backgrounds use automatic dragging, without explicit query callbacks.
Buttons retain ordinary client input. The example supports -native
and -quit-after=500ms for fallback and startup/shutdown checks.

Automated checks include 1x/2x Software frames proving that controls paint above
full-client content in the same frame, without an application root. Cross-platform
compilation does not substitute for the pending GUI interaction acceptance.

## Linux GUI-drawn Integrated (implemented)

### Creation and platform facts

`gui.WindowChromeIntegrated` first requests native Integrated. On Linux/X11 its
`ErrUnsupported` leads to a `None + Transparent` creation request. GUI then owns
the frame and the one custom controls group. `Window.Transparent()` reports the
actual alpha-surface configuration (true even if only Integrated was requested).
The X11 `Chrome()` observation remains `Unknown`: Motif decoration hints are a
request to the window manager, not a reason to fabricate native Integrated support.

If the ARGB/compositor capability is unavailable, an implicit alpha request may
fall back to Native using the caller's original transparency preference. Explicit
`Transparent: true` is never downgraded to opaque. Other creation failures are
returned. GUI drawing uses only existing Painter operations; there is no
surface-mask capability check or shader allocation during window creation.

### Appearance and layout

- Normal floating presentation: 12 DIP rounded corners and a 1 DIP solid border.
  There is no GOUI shadow or shadow padding. The root occupies the safe inner
  rectangle, leaving all four resize strips to Window. This automatic allocation
  is limited to Linux GUI-drawn Integrated; native Integrated, Native, None and
  Popover retain their existing layout, and no wrapper Widget is inserted.
- The body background uses the Window's explicit StyleName / default part,
  or `window` when unset. Changing the name only repaints this Window and never
  supplies a style to its Widgets. Radius, border
  width and border color are GUI-owned, not `window` / `frame` application rules.
  Private light/dark palettes follow the window background; the light border is
  opaque RGB(165,165,165). Replacing the sheet cannot remove or reshape the frame.
- The three GTK4-inspired controls use 24 DIP visible circles centered in 28 DIP
  square hit boxes, 8 DIP gaps between hit boxes and a 6 DIP
  right inset (8 DIP to the visible circle), measured from the complete window
  edge, not added a second time to the root inset. They are vertically centered
  in the queried HeaderBar row using the root content's Y origin; an
  absent row uses a 40 DIP placement height, not a Window height property.
  GUI-private normal/hover/pressed colors replace the former `window-control`
  rules. Neither circular nor rectangular caption buttons read application
  Button fonts or appearance. Icons use the private foreground color and the
  existing vector minus, maximize/restore and close shapes, not font glyphs.
  Linux circular controls use an 8 DIP glyph span within the unchanged 12 DIP
  icon allocation; strokes remain 1 DIP. Rectangular Windows controls retain
  their 10 DIP glyph span, and native macOS controls are unchanged.
- Maximized and fullscreen windows lose the frame border, rounded background
  and the content allocation/paint inset.
  Fullscreen also hides controls. Restoring Normal restores the GUI-owned frame.
  Only observed platform state drives these changes.
- HeaderBar avoidance uses the existing ControlsBounds signal; custom controls
  retain ordinary controllers, close vetoes, snapshots and window ownership.

### Rendering

Clear transparent → styled rounded background → safe rectangular clip →
application tree and controls → restore clip → inset border → present.

The safe rectangle is both the root allocation and a structural paint boundary.
Root Measure receives its tight size; Arrange receives its window-local origin
and size. Descendant local coordinates, input conversion, IME caret positioning,
Snapshot and RenderWidget use the existing Rect traversal; the native window
origin, Position/WorkArea and complete surface size do not change. HeaderBar
selects the row at the root's top rather than requiring window Y=0, and Chrome
includes that origin in circular-button vertical placement. Each Widget's
explicit clip intersects this boundary and cannot cancel it. Empty safe
rectangles still allocate zero-sized content and suppress drawing.

Automatic content-derived desktop minimum hints include twice the current
perimeter in each axis, and refresh when that perimeter changes with state or
scale. Unchanged hints retain the previous one-time behavior. Explicit SetMinSize
keeps its window-size semantics and is never increased by the GUI perimeter.

The default 12 DIP radius uses a 5 DIP inset. Larger radii use at least
r*(1-1/sqrt(2)) plus one physical pixel of AA clearance; the inset also covers
the border width and is rounded inward to a physical-pixel boundary. The same
inset defines the resize strips. The complete surface size remains unchanged.

Clipping Widget content at the window-owned perimeter is intentional. Matching
background colors blend naturally; differing Widget backgrounds leave the
window background visible. No extra Widget padding or per-HeaderBar rounding is added.
SurfaceClipper, its Software/OpenGL resources and the implicit End mask have
been removed. All rendering uses the existing Clear, FillRoundRect, SetClipRect
and border primitives. General clipping/erasure is deferred in
[DesignGraphicsClipping.md](DesignGraphicsClipping.md).

### Interaction and limits

The frame supplies eight resize roles (default 5 DIP edge strips, 16 DIP corner reach)
as the first region-query subscriber. HeaderBar's automatic classification leaves
these roles intact; later application subscribers may override them. Maximize and
fullscreen suppress these automatic resize regions. Caption drag/double-click
and native EWMH resize continue through DispatchEvent and the host controller.
The host pointer controller runs even when neither Widget tree is hit. A resize
strip is owned by Window, not an invisible Widget; it uses the same gesture
sequence and native takeover as input delivered inside content. Disabled or
empty content does not disable window resizing; callback destruction cancels
the sequence safely.

No input-region API, ShapeInput mask or transparent-corner passthrough is added.
The native input surface remains rectangular. GUI-drawn Integrated resolves
horizontal, vertical or diagonal resize cursors from the final ordered region
query, not a second edge hit test. Client overrides, modal input and pointer
capture suppress this feedback. Leave/focus loss and state changes clear it;
completed layout re-resolves it for a stationary pointer. Platform only maps the
requested cursor to a system resource. None and Native behavior, Windows rectangular caption
buttons and macOS native traffic lights remain unchanged. This is a bounded GUI
decoration implementation, not a promise of identical window-manager behavior.

### Verification

- `go test ./...`.
- GUI Software pixel tests at 1x/1.5x/2x: opaque children cannot refill the four
  corners; border width, antialiasing, circular buttons and state transitions.
- Creation/fallback, factual native Unknown, signal overrides and ordinary
  resize/button request routing tests.
- GUI tests verify safe root allocation and local/window coordinate conversion,
  structural clip enforcement for children/custom clip overrides/decorations,
  empty safe rectangles, border-state restoration and maximized/fullscreen transitions.
- `go run ./example/integrated`; repeat with `GOUI_PLAT_PAINTER=software` and
  `GOUI_PLAT_SCALE=2`. Native and GUI policies are logged. Software/OpenGL have
  been visually checked on the available composited X11 desktop; this does not
  replace testing other window managers or physical multi-monitor transitions.
- Native XTest input on the available X11 desktop verified the maximize button,
  Caption double-click restore, Caption drag, right-edge resize and close button.
  These inputs entered through the native event path, not direct GUI commands.

### 2026-10-07 Linux Integrated 布局修正与验证

仅 Linux GUI 自绘 Integrated 的根布局进入既有安全矩形；四边仍用于缩放。
HeaderBar 以根内容顶部查询高度，Chrome 计入内容 Y 起点，WindowControls 本身
及完整窗口右側距离不变。不添加包装 Widget、公共 API 或 Painter/平台特例。
派生最小尺寸含边缘，显式 MinSize 不变，最大化/全屏取消内缩。

验收发现的必要输入补齐：旧 EventDispatcher 无 Widget 命中时提前返回，内缩
后 host controller 因而收不到边缘按下。现在仍经 DispatchEvent/手势竞争
交给窗口控制器，不直接调用 BeginResize，不设置隐藏命中 Widget。普通、禁用、
空内容的八方向缩放及原生回调中销毁均有回归；无内容/无装饰的 None 自定义
区域也保留正常分发，但不自动内缩。

回归证据：新无 Padding 根布局/完整滚动条/最小尺寸在旧实现失败；仅完成布局
内缩但未补输入时，三个内容状态的 resizeRequests 均为空，新输入回归失败。
修复后通过。原 paint-only 测试改为新布局合同，保留圆角、clip 越界防护、边框
及状态转换像素断言。分数缩放的 Intersect 浮点几何预期同步调整，未放宽像素
容差。新窗口程序初轮误用不存在的 Size.Empty，编译失败后改为宽高检查；
失败日志保留在任务临时目录，最终构建通过。

通过：

- `DISPLAY=:83 XMODIFIERS=@im=none go test -json ./...`：2072 个测试/子测试
  通过、14 个既有跳过、零失败，包含缓存。无 DISPLAY 的指定回归与 GUI/UI/
  widgets/widgets/ui/demo 完整逻辑回归、五包 race 通过。
- `go vet -composites=false ./gui ./tests/window/gui/integrated_layout` 与
  `git diff --check` 通过；composites 的既有警告未修正，不宣称完整 vet 通过。
- Windows amd64 / macOS arm64 的 CGO_ENABLED=0 全仓构建及 GUI 测试目标编译
  通过，没有原生远端运行，不以编译替代桌面验收。
- Linux amd64、隔离 Xvnc 1.12.0 + Xfwm4 合成器，XTest 真实输入；独立程序
  `tests/window/gui/integrated_layout` 确认 Software 1x / OpenGL 2x 后端实例，
  分别输出对应 imageResource 类型。无根 Padding、贴边滚动条、标题高度变化、
  按钮居中、最大化/全屏/恢复、亮暗重绘及 F6 公开边界断言通过，均退出 0。
- 胶囊中段桌面采样显示连续 8/16 物理像素，相邻像素仍为窗口背景；包内
  Software 双向滑块精确比较不透明预乘 RGBA，1x/2x 无像素容差。
- 输入补齐后 Software 1x 右侧拖动使宽度 760→806；标题持续拖动使原点
  (100,80)→(180,120)，双击标题最大化通过。OpenGL 2x 右侧使物理宽度
  1520→1581，顶部使 Y=80→40、高度 1120→1160，确认是缩放而非 Caption 移动。

未执行真实中文 IME、其他 WM、真实高 DPI/混合 DPI、多显示器、Windows/macOS
原生桌面及其他后端组合。OpenGL 使用 llvmpipe，2x 是应用缩放覆盖，不宣称
硬件 GPU 或 Retina。只关闭本任务程序及隔离桌面，未改用户桌面、系统设置或
剪贴板；既有全仓像素/排版测试仍按原行为写图片，本轮不迁移或删除这些产物。
