# Window Chrome and HeaderBar

Current status (2026-10-14): transparent Window/Popup rendering on all three
platforms, GUI Chrome collaboration and HeaderBar UI binding are implemented.
Menu and DropDown retain native Popover hosts; Overlay is an ordinary container,
not their planned replacement. Explicit input-region support was withdrawn.

Platform Chrome is a decoration preference/observation. GUI Window.Chrome is the
active signal service specified in [DesignGUIWindowChrome](DesignGUIWindowChrome.md).
Controls are owned by Window, not application-composed Widgets; UI is a thin
adapter. Native defaults stay Native, Integrated stays opt-in.

Windows 10 uses popup-style chrome with a one-physical-pixel non-client top;
Windows 11+ uses overlapped style without that top pixel. Both extend DWM.
macOS keeps native traffic lights, disables implicit content dragging and supports
controls positioning. Linux platform Integrated remains unsupported; GUI adapts
through a transparent None surface with a safe content allocation.
Dated verification below is evidence for its stated version/backend/scale only.

## Responsibilities

GOUI keeps three separate concerns:

| Concern | Owner | Not responsible for |
| --- | --- | --- |
| WindowChrome | platform decoration preference and observable native result | HeaderBar creation, button configuration, management state |
| HeaderBar | gui composition and layout | native handles and external frame thickness |
| WindowControls | gui button presentation or native-button layout adaptation | restricting system commands |
| Window | native host and Widget-tree boundary | becoming a Widget |
| DesktopWindow | desktop queries, requests and native hit testing | imposing desktop capabilities on mobile hosts |

The dependency direction remains ui -> gui -> platform. Native resources and
calls belong to platform, including the AppKit calls required by a native
WindowControls adapter. There is no general native-widget embedding framework.

The frameworks/libs/sdk packages contain native bindings organized by their
existing API/type conventions, not a GOUI chrome adapter. AppKit delegates only
bridge native arguments. Decoration/state interpretation, property-read policy,
button geometry and DIP conversion belong to cocoa/x11/win32. WinAPI exposes
GetWindowLong, GetKeyState and DwmGetWindowAttribute rather than purpose-specific
WindowStyle, KeyDown or CaptionButtonBounds helpers. A zero GetWindowLong result
is not by itself an error; preserve the native error distinction described in
[GetWindowLongW](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getwindowlongw).

## Decoration modes and creation

WindowChrome has three creation-time preferences: Native, Integrated and None.
Native is the zero/default value. Auto is removed: platform does not choose a
GUI title-bar policy. Integrated remains opt-in.

- Native requests system/window-manager decoration.
- Integrated requests application content in the title-bar area.
- None requests an undecorated, still normally managed top-level window.

The native factory takes initial Size in DIP separately from WindowOptions,
which contains Chrome. GUI WindowOptions still groups Size and Chrome. There is
no WindowBehavior mask. Hiding a GUI button does not disable native shortcuts,
system-menu commands, resizing, or the window's close-request mechanism.
Separate size constraints remain separate APIs. Initial and minimum size follow
the backend conventions below; native SizeEvent always reports the actual client.

A backend that has not implemented Integrated returns ErrUnsupported before
allocating a window; it must not silently return a Native window. A window
manager may still treat decoration requests as hints. DesktopWindow.Chrome
reports an observable result, or WindowChromeUnknown when it cannot be established.
Requested preferences are not observations.

Native defaults never automatically become integrated after a backend gains
support. gui decides fallback policy; ui only reconciles gui declarations.
Creation options cannot change for an existing ui window identity.

## Explicit Widget composition

Chrome does not insert an application root or a HeaderBar. Applications compose
a top HeaderBar with one child and the body below it. HeaderBar has no mandatory
title Label or start/title/end slots; ordinary containers express that composition.
Multiple HeaderBars may share a window.

Window owns its custom-control decoration tree, or reserves observed native
traffic-light bounds. HeaderBar consumes those bounds and automatically avoids
controls. Its arranged height answers ChromeControls.Height; no creation-time
TitleBarHeight or freely laid-out WindowControls is exposed.

Linux GUI Integrated reserves its perimeter in root Measure/Arrange and structural
clip. This is a narrow GUI adaptation, not universal FrameMetrics or mobile safe
area propagation. See [GUI chrome](DesignGUIWindowChrome.md).

## WindowControls

### GUI-painted mode

This is a private Window-owned decoration group. Glyphs, hover and pressed
states use GUI's isolated decoration policy, not application Button styles.
Buttons issue requests through the window and do not steal document focus.

Showing/hiding a button is presentation, not an OS permission. Close uses the
existing vetoable RequestClose path. Maximize-related visuals change only
after querying an authoritative native observation, not after issuing a request.
Unsupported and unknown observations must not be rendered as confirmed false.

### Native mode

The native window retains ownership of its real buttons. WindowControls is a
layout adapter, not three pretend GUI Buttons and not a wrapper exposing handles.

ControlsRect() reports the native button group's actual occupied rectangle in
client DIP. Query during layout and after existing native size/state events;
there is no separate occupation-change event. Placement is a separate optional
DesktopWindow request, implemented first in platform, not yet exposed by GUI/UI:

```go
SetControlsPosition(position *geometry.Point) error
```

- Non-nil requests the button union's top-left origin in client DIP, not the
  close button's center. The value is copied; `(0, 0)` is a real position.
- Nil clears the preference and restores the native layout recorded before the
  first custom placement. No arbitrary default margin is hard-coded.
- Only macOS Integrated supports it. Other backends and modes return
  ErrUnsupported, including for nil. Destroyed windows return ErrUnavailable.
- Coordinates must be finite/non-negative. Unavailable native controls and
  positions that cannot fit the native titlebar container in the client return
  errors without replacing the previous preference; no silent clamping.
- It may be called before Show once AppKit has created the controls. Resize,
  backing-scale, show and relevant native layout changes reapply the copied DIP
  preference. Native point defaults remain separate from that scaled preference.
- Fullscreen and its transitions suspend custom placement. Non-nil requests
  return ErrUnavailable there; nil may clear the preference without moving
  fullscreen controls. Native will/did/failure callbacks restore normal placement
  when appropriate, without changing State/RequestState semantics.
- ControlsRect always reads the actual native geometry. A later small resize or
  temporary native hierarchy change can prevent placement; the query does not
  invent the requested rectangle or perform a hidden reposition.

Visibility and per-button customization remain separate future requirements.

Do not publish each button rectangle merely to reserve layout space. Do not
publish external frame thickness. Only gui.Window and the integration controls
consume this information; ordinary widgets and ui do not.

The adapter must reserve the actual native location. Measuring a width on the
right while the native buttons remain on the left is not a valid integration.

Initial restrictions:

- normal top-level windows, in a top HeaderBar;
- at most one native group per window;
- no arbitrary transform, clipping, z-order or duplication guarantees;
- no unsupported per-button customization silently ignored;
- an explicit native request fails visibly if its requirements cannot be met.

Automatic native/custom selection, if offered, is gui policy. Explicit native
selection must not silently turn into GUI-painted buttons. Native button
actions retain their platform semantics rather than being forced into a fixed
minimize/maximize/close interpretation.

### Lifetime and accessibility

Window creation establishes the group and Chrome subscriptions; destruction
releases them. HeaderBars independently connect/disconnect on mount/unmount.
The native-controls adapter never destroys AppKit's system buttons.

DPI, fullscreen and temporary native-query failures require explicit tests.
No application-facing controls Widget can be reparented or duplicated.

Native buttons retain native input and accessibility. The GUI adapter describes
the native occupied region without inventing actionable GUI Button nodes.
A future native accessibility bridge is distinct work.

## No general FrameMetrics API

WindowFrameMetrics, WindowControlRects and the metrics-change event are removed
from the public window contract.

- Resolved decoration is an independent native observation.
- A GUI HeaderBar's height is a layout result.
- No universal SafeInsets are subtracted from application content.
- Native button occupation is a narrow, optional integration fact.

Mobile system bars, cutouts and safe-area behavior require a separate design.
They must not be encoded as desktop title-bar metrics.

## Input and native hit regions

GUI-painted controls use the existing path:

    platform event -> Window.DispatchEvent -> EventDispatcher
    -> EventController -> Widget state/signal -> layout/paint

Native controls keep native input processing. Close requests and management
notifications still enter the Window event path; no duplicate Widget click is
manufactured.

DesktopWindow.SetHitTest replaces WindowFrameRegions. GUI answers a synchronous,
read-only query using completed layout and current policy. Default delegates to
native behavior; Client explicitly keeps customizable regions in client input.
The callback can change a region's role without republishing rectangle snapshots.
Native controls remain native-owned; custom resize roles must not be unconditionally
overridden by a default edge test. See [DesignWindowHitTest.md](DesignWindowHitTest.md)
for callback lifetime, native input routing, platform limitations and ControlsRect.
Locking these hit regions is not a permission system for OS commands.

SetHitTest is supported only on Windows. X11/macOS return ErrUnsupported rather
than simulating a query on pointer events. GUI may then use BeginMove/BeginResize
from ordinary synchronous native PointerDown dispatch; Windows returns
ErrUnsupported for those requests; macOS supports limited native resize.
Unsupported is a
capability result, not a reason to emulate window geometry or edge cursors in
platform. Native AppKit resize borders remain system-owned.

GouiContentView overrides mouseDownCanMoveWindow to false, including content
under the transparent Integrated titlebar. It does not opt application content
into implicit AppKit window dragging while forwarding the same down/up to GUI.
Blank-header dragging explicitly requests BeginMove during native press or
left-held motion; GUI Chrome uses a movement threshold. The
caller must not also start a click/capture for that press; a native drag may
consume PointerUp. Platform does not swallow a real release or synthesize a GUI
cancellation. Native titlebar controls, native decoration outside content and
resize borders retain their system behavior; NSWindow.isMovable is not disabled.

## Size semantics

Size preferences describe the drawable client area in DIP in every Chrome
mode, including Win32 None and Integrated. Native decoration/frame offsets
remain backend details; GOUI does not expose an outer-window size or add a
second client-size API. CreateWindowEx takes the outer extent, so Win32 must
convert the request according to the non-client calculation it installs.

Native uses AdjustWindowRectExForDpi: this is precisely the standard conversion
from requested client area to outer window. None has no frame to add. Integrated
uses DefWindowProc's actual left/right/bottom frame calculation for the HWND
and its version-specific physical top inset; it must not include a caption height,
even though its Windows 11 style contains WS_CAPTION. There is no corrective
resize after Show and no frame/pixel-scale information exposed for GUI layout.

SizeEvent always reports the complete, actual drawable client surface in DIP
and physical pixels. The integrated HeaderBar participates in that surface's
ordinary root layout. Window must not add its height again or derive the client
size from the initial preference.

Applications that require a minimum body size set it on the body Widget.
SetMinSize describes the client area too. Win32 converts the hinted axes into
native outer tracking extents using the same mode-specific frame calculation;
an unset axis does not replace the native default.
Do not change or duplicate the existing logical/physical rendering boundary.

## Desktop window presentation

See [DesignWindowManagement.md](DesignWindowManagement.md). DesktopWindow exposes
one observed State and RequestState. Normal means ordinary presentation, not
restoration of a previous mode. Native recovery bookkeeping is not exposed as
combined states. Unsupported operations return errors; unavailable observations
are Unknown. There is no public Restore.

No native-title-bar button visibility option is used to reject system commands.
No mobile lifecycle or visibility event is normalized into a desktop state.

## Platform implementation direction

- Windows: retain DWM frame/shadow/rounding and shell behavior. GUI paints
  integrated caption content/buttons following the application theme; these are
  not three native HWND wrappers. Native buttons are not required by Integrated.
  A future optional native-button mode must state its theme/layout limitations.
  Validate non-client sizing, input, mode-specific system commands and Snap on
  native desktops.
  Do not disturb the existing resize/repaint synchronization.
- macOS: retain NSWindow traffic lights and their native actions. Full-size
  content and transparent title bars are now available in platform; GUI still
  requires a narrow native-button layout adapter.
  Use supported AppKit APIs; do not copy GTK private titlebar-height APIs.
- X11: native WM decoration stays default. Opt-in CSD remains a managed window,
  never override_redirect. Motif hints are requests, not a guaranteed observed
  decoration result. Preserve existing GLX/resize-sync behavior. Native shadow
  availability is WM/compositor-dependent.
- Wayland: future work. Respect compositor decoration configuration and native
  serial requirements for interactive move/resize.
- Mobile: no desktop chrome or window-management capability is assumed.

### Windows Integrated mechanism

- Require active DWM composition before allocation; an unavailable composition
  query returns its error, while observed disabled composition is unsupported.
- Query CurrentBuildNumber from HKLM's Windows NT CurrentVersion key, following
  ModernWindow's mechanism. Cache a successful build observation on Platform;
  do not add a public version enum or guess Windows 10 on query failure. Native
  and None do not depend on this query.
- Create initially with WS_OVERLAPPEDWINDOW and CW_USEDEFAULT positioning.
  Before Windows 11 (build 22000), change GWL_STYLE to exactly
  WS_POPUP | WS_THICKFRAME | WS_MAXIMIZEBOX. On Windows 11 and newer, retain
  WS_OVERLAPPEDWINDOW (not bare WS_OVERLAPPED, which is zero). User32 only applies
  default positioning to overlapped windows. All frame setup completes hidden.
- Extend DWM's top margin one physical pixel into the client with
  DwmExtendFrameIntoClientArea, install the non-client calculation, and apply
  SWP_FRAMECHANGED, regardless of whether GWL_STYLE changed. In particular the
  Windows 11 unchanged style must still install the custom client calculation
  and extend DWM. Reapply the extension on WM_DWMCOMPOSITIONCHANGED. A failed
  initial setup returns an error and destroys the window; Chrome reports Unknown
  if the installed extension or current composition can no longer be confirmed.
- WM_NCCALCSIZE with wParam == 0 delegates entirely to DefWindowProc. With
  wParam != 0 in the normal state, save the proposed outer top, call
  DefWindowProc, then set client.Top to outer.Top + 1 on Windows 10, or outer.Top
  on Windows 11 and newer. Preserve its left/right/bottom boundaries, other
  rectangles and return flags. The Windows 10 non-client pixel and the DWM
  client extension are separate parts of the frame, neither scaled by GOUI's
  logical-scale override. Windows 11 must not retain that top non-client pixel:
  the user observed bright pixels at the two rounded top corners in dark mode.
- In the maximized branch, use MonitorFromWindow/GetMonitorInfo to copy the
  current monitor's rcWork directly into rgrc[0], then return 0. Do not call
  DefWindowProc again or retain the normal top inset. The work area already uses
  native screen pixels, including negative multi-monitor coordinates. If the
  query fails, retain the proposed rectangle rather than inventing a work area.
- WM_GETMINMAXINFO uses the same work area: MaxSize is its extent, MaxPosition
  is its origin relative to rcMonitor. This preserves top/left taskbar offsets
  as well as secondary-monitor placement. Minimum tracking hints are independent;
  a zero hint leaves the native default untouched. Taskbar auto-hide behavior
  still requires native acceptance; no guessed reserved gap is introduced.
- None and Integrated do not call AdjustWindowRectExForDpi for creation or
  minimum size. None uses the requested physical client extent directly.
  Integrated obtains left/right/bottom insets with DefWindowProc's ordinary
  WM_NCCALCSIZE RECT calculation after installing the final Style, then adds
  the version-specific top inset. This is necessary: the Win10 Popup frame
  observed at 96 DPI is 7 pixels per side/bottom, while the standard overlapped
  resize metric is 8. A metric guess or hardcoded subtraction is not used.
  The HWND calculation uses native DPI independently of the rendering override.
  The same FRAMECHANGED that installs the custom calculation sets the final
  outer size before Show; no visible corrective resize is performed.
  Native mode retains AdjustWindowRectExForDpi and reports calculation failure.
- Default hit testing keeps native side/bottom edges. The top resize band uses
  GetSystemMetricsForDpi, independently of GOUI's logical scale override. Test
  the band in client pixels and correct native top corners extending too far
  down the sides, following ModernWindow's hit-test behavior.
  Explicit SetHitTest roles still take precedence; stale native caption-button
  hit regions do not cover the new client. Windows ControlsRect is empty in
  Integrated because there is no visible native caption-button group.
- Custom caption-button input retains the existing pointer route and native
  HTMAXBUTTON hover role for shell Snap support. System keys are also forwarded
  to DefWindowProc. The Windows 10 popup style does not include WS_SYSMENU or
  WS_MINIMIZEBOX, so it does not promise Native mode's default Alt+Space menu.
  Do not silently re-add caption styles or emulate a system menu in platform.
  Explicit state/close requests and the existing close-veto path remain intact.
- Invalidate and update the client on WM_ACTIVATE and WM_NCLBUTTONDOWN as in
  ModernWindow, with a lifetime recheck after synchronous painting. Preserve
  WM_SIZE-before-paint and synchronous painting during resizing. Do not invent
  WVR flags, stretching, timer repaint loops, layered windows, window regions,
  a full glass surface or changes to the painter backends.

The baseline is the user's device-tested
[ModernWindow implementation](https://github.com/xuges/ModernWindow/blob/812139e5e227b24c6627c531d37c257654b55bc7/ModernWindow/ModernWindow.cpp),
using the public [DWM frame extension](https://learn.microsoft.com/en-us/windows/win32/api/dwmapi/nf-dwmapi-dwmextendframeintoclientarea)
and [WM_NCCALCSIZE](https://learn.microsoft.com/en-us/windows/win32/winmsg/wm-nccalcsize)
mechanisms. The version split is based on the user's Windows 10/11 device
observations: Windows 10's overlapped style with the retained top pixel exposes
a native titlebar; Windows 11's retained top pixel leaks at rounded corners.
This is a GOUI compatibility policy, not a Microsoft promise that the two
styles guarantee identical compositor output. Prior ModernWindow validation
does not substitute for testing this GOUI implementation.
Native border/shadow/corner appearance remains an OS decision, not a guaranteed
radius; see Microsoft's [rounding guidance](https://learn.microsoft.com/en-us/windows/apps/desktop/modernize/ui/apply-rounded-corners).
Windows 10 is not promised Windows 11 rounding; maximized/snapped windows and
remote/virtualized desktops may receive different native treatment.

The size contract follows [CreateWindowEx](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-createwindowexw),
[AdjustWindowRectExForDpi](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-adjustwindowrectexfordpi)
and [MINMAXINFO](https://learn.microsoft.com/en-us/windows/win32/api/winuser/ns-winuser-minmaxinfo).
The [DPI-aware resize-hit metrics](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getsystemmetricsfordpi)
stay entirely in Win32; sizing uses the installed frame's native calculation.
Package tests cover the build boundary, frame geometry,
scale overrides and work-area math. The independent window program
`tests/window/platform/window_chrome` checks actual client/minimum geometry and
provides a flat bright/dark surface for corner inspection; desktop compositor
pixels still require native Windows 10 and Windows 11 acceptance.

#### 本轮验证（2026-10-12）

- 本机 Linux Xvnc/Xfwm4 隔离桌面，Go 1.25.8、CGO_ENABLED=0，非缓存
  `go test -json ./... -count=1`：2357 测试/子测试通过、15 既有跳过、无失败；
  另执行 `go test ./...` 通过。保留既有测试的窗口和图片输出副作用。
- GUI/UI race 通过。Windows amd64、macOS arm64 全仓构建及全部测试目标
  交叉编译通过；`-exec=/bin/true -run='^$'` 仅用于编译，不计为原生执行。
- Windows 10 19045 x64 原生：版本/缓存、客户区换算、NCCALCSIZE 数学、
  最大化工作区和顶部命中六组逻辑测试通过。Session 0 的 Native/None 几何
  回归通过，Integrated 因 DWM 未启用跳过；这不是桌面验收。
- RDP Session 1、系统 DPI 96：Native/Integrated/None 初始 640×400 DIP 与
  最小客户区 320×200 DIP、普通/最大化/恢复及 NCCALCSIZE 两种参数分支
  在渲染缩放 1 / 1.25 / 2 下通过。Integrated 普通顶部均为 1 物理像素，
  最大化客户区等于 1600×860 工作区；原生 Native 顶部为 31 物理像素。
- 同一 Windows 10 桌面、1x，实际 Software/Direct2D 的纯暗/纯亮、最大化/
  恢复与客户区一致性通过，Software 的右边缘真实拖动缩放通过。
  OpenGL 初始化失败：`can not choose useful pixel format`，未进行该后端
  桌面验收，不据此更改 WGL 或声称绘制验证通过。
- 旧几何测试将非队列系统消息交给 DispatchMessage，在实测中产生错误的
  NCCALCSIZE / MINMAXINFO 结果；改为按 [SendMessage 的同步契约](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-sendmessagew)
  送达原生窗口，并固定 LPARAM 所指 Go 内存。隐藏 Native 的 WM_SIZE 可以
  延至 Show；测试仍检查请求客户区，且检查实际收到的 SizeEvent，不伪造事件。
- 普通 Windows vet 报告既有 native-address/unsafe.Pointer 告警；受影响包
  `go vet -unsafeptr=false` 通过，未改写这些原生回调或删除检查来修复功能。
- **待验收**：Windows 11 暗色顶部两角；系统主题切换、激活/失活、OpenGL
  可用桌面、真实系统 HiDPI 和跨 DPI 显示器。渲染缩放覆盖不等于系统 DPI 验收。

### macOS Integrated mechanism

- Keep the ordinary titled, closable, miniaturizable, resizable NSWindow and its
  native close/minimize/zoom buttons. Add FullSizeContentView, hide the title,
  and make the titlebar transparent before publishing creation events.
- Explicitly set the content size through AppKit, including logical-scale
  conversion. The full-size view, not contentLayoutRect below the titlebar,
  determines SizeEvent and the painter's physical backing extent. Content
  minimum size includes the integrated titlebar area exactly once.
- Chrome queries actual style, title visibility and titlebar transparency.
  ControlsRect converts the real visible NSButton union to top-left client DIP.
  It never mutates layout. No fixed button sizes or extra notifications.
- SetControlsPosition follows Electron's native-button placement approach:
  rediscover the three standard buttons and their shared titlebar hierarchy,
  resize the container anchored to its top edge, and translate the saved button
  frames together. Sizes, gaps, native actions and ownership are retained. Only
  public NSView frame/conversion methods are bound; no private selectors, class
  names, reparenting or replacement buttons. The hierarchy is still an AppKit
  implementation detail: invalid/unavailable structure returns an error, and
  native OS versions require testing rather than a universal layout guarantee.
- GouiContentView returns false from mouseDownCanMoveWindow, following GTK's
  content-view integration. Application header input is not an implicit drag
  region. No click policy or simulated HitTest is introduced in platform.
- AppKit owns the buttons, accessibility, visible frame/shadow and native resize
  edges. BeginMove accepts native press/left-held motion, BeginResize uses limited
  backend-managed tracking, and SetHitTest remains unsupported. Fullscreen requests are still a separate
  unimplemented state transition; native green-button actions are retained.

These are the public [full-size content view](https://developer.apple.com/documentation/appkit/nswindow/stylemask-swift.struct/fullsizecontentview)
and [content size](https://developer.apple.com/documentation/appkit/nswindow/setcontentsize(_:))
mechanisms. FullSizeContentView opts into AppKit layer backing, so both Software
and OpenGL need native visual testing with the retained traffic lights.

## Implementation slices (historical chronology, not current tasks)

### A. Desktop contract migration (implemented)

- Write this revision and the separate management design.
- Retain the removal of Auto, WindowBehavior, FrameMetrics and Restore.
- Replace WindowManagement/WindowDecoration with DesktopWindow.
- Expose one observed State and explicit RequestState, not independent booleans.
- Implement SetHitTest and the separate ControlsRect observation.
- Preserve native creation defaults and the existing close-veto path.
- Add native state events with a GUI signal adapter; ControlsRect is query-only.
- Report unsupported Integrated rather than silently falling back.
- Migrate GUI/UI creation and tests without injecting a Widget root.

At completion of slice A, all backends rejected Integrated before allocation;
slice C now implements it on Windows/macOS. X11 still rejects Integrated and
reports Unknown chrome instead of treating Motif hints as observed results,
and provides CSD through None plus GUI content. Windows fullscreen requests and
macOS fullscreen transitions remain unsupported; native AppKit fullscreen
observations and traffic-light actions are preserved. ControlsRect is a geometry
query, not a completed native-controls Widget adapter.

The previous Phase 1 implementation is historical groundwork, not completion
of this revised contract or any integrated backend.

### B. Native interaction capability split (implemented)

Keep native SetHitTest on Windows, expose EWMH move/resize on X11 and AppKit move
on macOS, and forward explicit requests in GUI. Update the platform sample to
try native hit testing before selecting client-input requests. This slice does
not enable Integrated or promise complete None visuals.

### C. Integrated platform support (implemented; native acceptance pending)

Validate each native mechanism in platform before GUI integration: window
decoration, actual native-button occupation, supported placement, native hit
regions and management behavior. Do not add successful no-op capabilities.
Prioritize half-custom chrome: Windows retains DWM borders/shadow/rounding;
macOS retains AppKit borders/shadow/traffic lights. X11 has no corresponding
portable integrated-titlebar facility and continues to report unsupported.

Implementation is limited to platform plus `example/window_chrome`; no automatic
GUI root/HeaderBar, button policy or public frame metrics were introduced.
The existing `example/plat_test` remains the None interaction prototype.

### D. GUI composition

Implement HeaderBar and GUI-painted WindowControls, then the native adapter.
Test layout, placement restrictions, lifecycle, input, style and semantic
snapshots using fake platform capabilities and real native prototypes.

### E. UI and examples

Add thin declarative adapters and explicit HeaderBar examples. Do not add
automatic window roots or an alternative input/lifetime system.

### F. Fully custom None chrome (later)

Treat transparency, compositor shadow, visible shape/input region, resize cursor
policy and full GUI controls as a separate slice. None alone does not promise
transparent corners or shadows. Do not retrofit an alpha surface or simulate
HitTest to complete the current half-custom slice.

Scheduling update: explicit Linux transparency and pointer input regions are now the
next platform foundation, specified separately in DesignLinuxTransparency.md. Full CSD
geometry, shadow margins and GUI Chrome integration remain later work. Native Integrated
continues to mean native integration, not a successful alpha-surface request.

## Verification

- go test ./...
- Platform and GUI tests distinguish unsupported/unavailable observations.
- Requests do not fabricate observed states; Normal is not historical Restore.
- Unsupported Integrated fails before native allocation.
- Native defaults, system commands and close veto do not regress.
- GUI content never needs external frame metrics; header height counts once.
- Native occupied rectangles agree with actual button placement at different
  DPI, visibility and fullscreen configurations.
- Mount/unmount and destruction leave no stale native association.
- Cross-compile Windows amd64 and macOS amd64/arm64 tests; compilation does not
  substitute for runtime, accessibility or shell verification.
- Run native acceptance on Windows 10/11 and macOS before claiming complete
  Integrated behavior. Representative X11 desktops must continue to reject it.
  Keep defaults native throughout.

Verification of this migration:

- go test ./...;
- go test -race ./core/signal ./platform/common ./gui/... ./ui/...;
- Windows amd64 and macOS amd64/arm64 full test-target cross-compilation;
- native X11 creation, minimize/normal, maximize/minimize/normal, hide/normal
  and fullscreen/maximize/normal transitions at the default scale and 2x.

Integrated regression coverage:

- Creation tests distinguish Windows/macOS success from X11 rejection, verify
  authoritative client size reporting and queries after destruction. Windows
  custom modes test the unadjusted outer size separately from the actual client.
- Windows unit tests cover custom size/minimum calculations, normal non-client
  geometry at 1x/2x, monitor work-area offsets, empty bounds, top/corner hit regions
  and native ABI. Hidden-window tests exercise style/monitor bindings, the native
  top pixel, both normal WM_NCCALCSIZE forms and clearing minimum hints.
- macOS unit tests cover native-button DIP conversion, top-anchored container
  resizing and invalid coordinates. Native tests check mouseDownCanMoveWindow
  for all chrome modes, group placement without changing sizes/gaps, repeated
  requests, copied values, reset, resize, destruction and per-window isolation.
  Opt-in visible coverage also checks show/hide and actual ControlsRect queries.
- Opt-in `TestWindowChromeIntegrated` paints normal/maximized/normal on Windows
  and normal on macOS (portable maximization requests remain unsupported there),
  queries actual button occupation and exercises close veto and cleanup.
  The log reports the actual painter (a platform preference may fall back).
- Windows-only opt-in TestWindowChromeIntegratedNativeGeometry checks that the
  maximized client equals rcWork, WM_NCCALCSIZE returns 0 for that branch, and
  normal/maximized/normal restores the original client extent.

On a Windows/macOS test desktop, run the following with each supported painter
and both 1x/2x logical scale settings (PowerShell uses `$env:NAME="value"`):

```sh
GOUI_TEST_WINDOW_CHROME=1 GOUI_PLAT_PAINTER=software GOUI_PLAT_SCALE=1 go test ./platform/test -run 'TestWindow(Chrome|ControlsPosition)' -count=1 -v
GOUI_PLAT_PAINTER=opengl GOUI_PLAT_SCALE=2 go run ./example/window_chrome
```

Windows additionally uses `GOUI_PLAT_PAINTER=direct2d` (check the reported actual
painter). `-chrome native` provides a baseline without changing the default mode
of existing applications. Click the colored tile to change client colors; check
that Windows custom controls follow them while macOS native controls stay native.
The green outline marks the actual macOS controls occupation, not GUI padding.
Manual acceptance covers native corners/shadow and inactive appearance, minimum
size, all resize edges, maximization/restoration, top/left/bottom taskbars and
auto-hide, HiDPI monitor changes, opaque rendering on every backend, Windows
Alt+F4 and Snap hover (Alt+Space remains mode-dependent), and AppKit traffic-light
actions/accessibility. Cross-compilation and geometry tests do not prove these
compositor or shell behaviors.

For the macOS input/placement regression, use the standalone sample (the user's
`example/plat_test` is unchanged):

```sh
go run ./example/window_chrome -move=false -trace-input
go run ./example/window_chrome -controls-position 16,18 -trace-input
```

With `-move=false`, drag the old titlebar area and the colored tile: application
content must not move the window. A normal tile down/up changes the theme once.
With the default explicit movement enabled, only blank header space calls
BeginMove; clicking the tile must not move the window. The green outline must
match the actual native buttons at the requested DIP origin. Press R to reset
their position. Repeat after resize, show/hide, activation, HiDPI monitor changes,
native fullscreen entry/exit and failed/cancelled transitions. Fullscreen controls
keep the native layout; the custom preference resumes after returning. Check all
three native actions, hover tracking and accessibility after repositioning.

The user has verified the basic macOS Integrated appearance. The new input and
placement changes still need that native acceptance; Linux tests and Darwin
cross-compilation cannot validate AppKit dispatch or on-screen button hit areas.

Windows/macOS native interaction and button geometry remain pending real-device
validation; cross-compilation is not runtime validation. ControlsRect has no
dedicated event. Its unit tests cover client-relative conversion at 1x/2x.

Run the opt-in native transitions on another test desktop with:

~~~sh
GOUI_TEST_WINDOW_STATE=1 go test ./platform/test -run 'TestWindowState.*Transitions' -count=1 -v
~~~

## macOS None 的放大状态（2026-10-13）

None 创建不含 NSWindowStyleMaskResizable，现行 RequestState(Maximized) 不支持
原生 zoom。该类窗口的 isZoomed 可在初始小尺寸上返回 true，不能直接映射为 Maximized。
State 仅在有 Resizable mask 时采用 isZoomed；Normal 请求也只在有该能力时调用 Zoom。
隐藏、最小化、全屏的观察顺序保持不变，不用工作区几何或 GUI 补偿伪造 zoom。
真实 macOS 15.7.7 / arm64 / OpenGL / 1x 的旧误报、修正及缩放验收证据见
[DesignBorderBox](DesignBorderBox.md#macos-状态查询修正)，独立窗口 -auto 对刚显示的
Normal 状态作回归断言。Native/Integrated zoom 的契约没有改变。

## References

- [GTK native macOS controls](https://docs.gtk.org/gtk4/osx.html#native-window-controls)
- [GTK native-controls adapter source](https://github.com/GNOME/gtk/blob/4.18.6/gtk/gtkwindowbuttonsquartz.c)
- [GTK macOS content view](https://github.com/GNOME/gtk/blob/main/gdk/macos/GdkMacosView.c)
- [AppKit mouseDownCanMoveWindow](https://developer.apple.com/documentation/appkit/nsview/mousedowncanmovewindow)
- [Flutter title-bar guidance](https://docs.flutter.dev/ui/adaptive-responsive/idioms#title-bars)
- [Electron custom title bars and controls overlay](https://www.electronjs.org/docs/latest/tutorial/custom-title-bar)
- [Electron Windows caption button source](https://github.com/electron/electron/blob/main/shell/browser/ui/views/win_caption_button.cc)
- [Electron macOS native-button positioning](https://github.com/electron/electron/blob/main/shell/browser/ui/cocoa/window_buttons_proxy.mm)
