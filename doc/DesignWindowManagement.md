# Desktop Window Contract

Status: Window / DesktopWindow and State / RequestState migration implemented.
This contract supersedes the previous optional WindowManagement/WindowDecoration
interfaces. Windows/macOS Integrated chrome is implemented in the separate
[chrome slice](DesignWindowChrome.md); dated results and remaining gaps are recorded
separately there. Complete
desktop transition coverage remains follow-up work.

## Host and desktop responsibilities

Window remains a native paint/event/lifetime host. Title, Show, Hide and
RequestClose stay on it: metadata and lifetime are not desktop title-bar policy.
Show does not mean foregrounding a mobile application; RequestClose does not mean
terminating an application. MobileWindow is deferred until a real implementation
establishes its requirements.

DesktopWindow embeds Window and provides:

~~~go
WorkAreaAt(point geometry.Point) (geometry.Rectangle, error)
SetPosition(relativeTo Window, position geometry.Point) error
Position(relativeTo Window) (geometry.Point, error)
SetMinSize(width, height float32)
Chrome() WindowChrome
ControlsRect() (geometry.Rectangle, error)
SetControlsPosition(*geometry.Point) error
State() WindowState
RequestState(WindowState) error
SetHitTest(func(geometry.Point) WindowHit) error
BeginMove() error
BeginResize(WindowEdge) error
~~~

There is no separately owned management wrapper. Callers assert the interface on
the original native object. DesktopWindow does not guarantee every request is
available on every desktop backend. Calls and callbacks are thread-affine;
cross-thread work is posted through the event loop.

SetHitTest is native-only: Windows supports it; X11/macOS return ErrUnsupported.
GUI may then initiate BeginMove/BeginResize through ordinary input controllers.
Resize needs original native left PointerDown; move also accepts native left-held
motion for threshold initiation. A posted task cannot manufacture that context.
Windows returns ErrUnsupported for both. X11 submits EWMH requests; macOS
supports AppKit move and limited backend-managed native frame-tracking resize. See
[native interaction contract](DesignWindowHitTest.md) for input ownership and
error semantics. No platform backend simulates a GUI hit-test policy.

## Default initial placement

When no position is requested, each backend uses its native placement facility:
Win32 uses `CW_USEDEFAULT`, X11 leaves placement to the window manager, and AppKit
calls the existing `NSWindow.center` once when creating an ordinary window, after
its native content size is initialized and before it is shown. AppKit's native
placement is horizontally centered and vertically somewhat above center; this
is not a promise of mathematical work-area centering or identical coordinates
across platforms.

The AppKit call belongs to `cocoa.newWindow`, not the shared native constructor
used by Popup and not `Show`. It applies equally to Native, Integrated and None,
independently of transparency and SkipTaskbar. A later `SetPosition` overrides the
default; hiding/showing a window does not restore its initial placement. Popup
continues to use owner-local explicit positioning. Default placement adds no
screen API, remembered-position policy, fixed offset or cascading state.

Evidence: [AppKit center](https://developer.apple.com/documentation/appkit/nswindow/center())
documents native horizontal/vertical placement and does not show the window.
[Qt 6.8.3's initialGeometry](https://github.com/qt/qtbase/blob/v6.8.3/src/gui/kernel/qplatformwindow.cpp#L614)
also provides initial centering on systems without automatic placement while
preserving explicitly requested geometry. GOUI reuses AppKit's existing method
rather than introducing Qt's screen-selection and geometry abstractions.

Regression verification lives in the explicit macOS window program
`tests/window/platform/window_position`; it observes native frame/screen geometry,
checks hidden creation, explicit placement and Hide/Show, and verifies Popup is
not centered. Desktop execution and remaining validation gaps are recorded below.

### 2026-10-13 placement verification

- The same native probe was built against HEAD `33e8a84` in a separate temporary
  tree. On macOS 15.7.7 arm64 it failed as expected before Show: frame origin
  `(0,85)` remained at the left/bottom of the visible area. No baseline source or
  assertions were changed to make the corrected version pass.
- The corrected probe, with SHA-256 verified after transfer, passed Native,
  Integrated and None at application scales 1x/2x (six cases). Each checks hidden
  creation against native screen geometry, first Show preserving the frame,
  explicit client placement at `(120,80)`, two Hide/Show cycles, Popup creation
  without centering and the owner-local `(20,40)` anchor. The automatic geometry
  mode uses native windows, not a Painter, and is not a headless test.
- macOS VNC desktop: OpenGL 1x Native/Integrated and Software 1x None appeared
  horizontally centered and vertically above center. Real F5/F6 input confirmed
  explicit placement and redisplay; Native was also dragged with the mouse and
  redisplayed without reverting its position. F7 showed the correctly anchored
  Popup. Integrated/None exited with recorded code 0; Native was closed using
  its native close button, with no failure log. Screenshots are observations,
  not a measurement of initial-frame latency or a guarantee against all flicker.
- Existing `tests/window/gui/window_position -auto` passed all three Chrome
  modes on the same macOS desktop with OpenGL/application 1x, including pre-Show
  explicit positioning, nil/self/other references and independent WorkAreaAt
  checks. No GUI/UI code was changed.
- Local Go 1.25.8/Linux amd64: final `go test ./... -count=1 -json` on isolated
  Xvnc/Xfwm4 passed 2481 tests/subtests, with 15 existing skips and no failures.
  The original tests still have desktop side effects; this is not a headless
  full-suite result. Demo/resource tests with DISPLAY unset and race checks passed.
  CGO_ENABLED=0 Windows amd64/macOS arm64 full builds, the macOS Cocoa test binary
  compilation, scoped macOS vet and `git diff --check` passed.

Only this native default-placement call was changed. No Windows desktop testing,
multi-monitor/mixed DPI, real HiDPI, alternate Dock position, oversized windows,
Stage Manager or Spaces transitions were executed. Application scale overrides
are not real system DPI coverage. Desktop test processes were closed and the
temporary local display/client connection cleaned up; artifacts remain under
`/tmp/goui-mac-placement.AHf9qk` locally and `/tmp/goui-mac-placement.Qhhiwv` on macOS.
The native fix and regression program are committed as `90bdb83`
(`Fix macOS window initial position.`), not pushed. This pre-existing untracked
design document and the broader test index remain in the working tree.

## Explicit initial centering

`gui.WindowOptions.Center bool` requests initial client-area centering; default
false preserves the native default above. `ui.Window(...).Center(bool)` is a thin
creation-only declaration, like Size and SkipTaskbar. Changing it on an existing
window ID is rejected; rebuilding the same declaration never moves the window.

- After creating the hidden native window, GUI queries its existing
  `WorkAreaAt(Point{})` and submits `SetPosition(nil, point)`, with
  `point=((area.Width-Size.Width)/2, (area.Height-Size.Height)/2)`.
- Size is the normalized initial WindowOptions.Size, including default axes.
  It is a creation preference, not a fabricated observation of current size.
  This computation does not depend on a SizeEvent, Widget measurement, painter
  pixel scale or a pre-Show Snapshot. GUI still lays out using native SizeEvents.
- WorkAreaAt's origin is client-local; it is not added to the point because nil
  SetPosition already uses the work-area top-left. Existing backend positioning
  owns native frame offsets, scale conversion and pixel rounding.
- Center is applied once during NewWindow, before registration/Show. A later
  explicit SetPosition naturally overrides it. Show and Hide/Show never reapply
  it, and UI AfterUpdate may still position a newly created hidden window.
- Do not clamp oversized windows in GUI: positioning remains a native request.
  A WM may constrain size/position, including after applying a minimum-size hint.
  If actual size subsequently changes, Center does not become a persistent
  centering constraint. In particular this is not SizeToContent or autosizing.
- Work-area/positioning failures fail NewWindow, preserve the error for the
  caller and destroy the hidden window/painter. False performs neither request
  and does not require a desktop positioning capability.

The strategy belongs only to GUI; native WindowOptions and all three platform
implementations are unchanged. No public Window.Center/Bounds/Size, owner,
startup-location enum, screen model or new first-layout hook is introduced.
SkipTaskbar(bool) keeps its explicit boolean argument. Popover anchoring is
unchanged.

Reference: [Avalonia 11.3.0 ShowCore](https://github.com/AvaloniaUI/Avalonia/blob/11.3.0/src/Avalonia.Controls/Window.cs#L633)
resolves initial size and startup location before native Show. GOUI adopts the
pre-visibility placement order, not its startup enum, owner/screen abstractions
or SizeToContent layout machinery: GOUI's current initial size is already in
WindowOptions.

Package tests cover default/partial/fractional/oversized sizes, missing creation
SizeEvents, chrome fallback, errors/cleanup, explicit overrides, repeated Show
and declaration preservation/rejection. The desktop entry
`tests/window/ui/window_center` checks hidden creation, actual client/work-area
symmetry, rebuild identity, Hide/Show preservation and explicit overrides via
the public GUI/UI interfaces. It has Chinese procedures and an explicit -auto
mode, without DevServer. Native desktop validation and its limits are recorded
in Plan; initial size requests do not guarantee a WM-independent final geometry.

### 2026-10-13 Center verification

- Go 1.25.8/Linux amd64: final isolated Xvnc/Xfwm4 full suite
  `CGO_ENABLED=0 go test ./... -count=1 -json` passed 2492 tests/subtests, with
  15 existing test skips and no failures. Existing desktop/pixel-output side
  effects remain. DISPLAY-unset GUI/UI tests and `CGO_ENABLED=1 go test -race
  ./gui ./ui -count=1` passed. Race needs cgo for Go's race runtime only; the
  first CGO_ENABLED=0 race invocation was rejected, then rerun correctly.
- Windows amd64/macOS arm64 CGO_ENABLED=0 full builds passed. Scoped vet with
  the repository's existing composite-literal warnings excluded passed, as
  did diff checks. All new desktop probe instructions are Chinese.
- Local Ubuntu 22.04.5/X11/Xfwm4: Center=true passed Native/Integrated/None
  at application 1x/2x with Software and OpenGL preferences (12 cases), plus
  pre-Show explicit overrides and SkipTaskbar combinations for both preferences.
  Actual client size/work-area symmetry, explicit moves, window identity on
  rebuild and Hide/Show preservation are asserted. Real F5/F6/F7 input also
  preserved an explicitly moved Integrated window; titlebar drag was not
  independently confirmed in this local probe.
- Windows 10 19045 x64: the same six Chrome/scale cases plus override,
  Center=false and SkipTaskbar passed, with a Direct2D preference in desktop
  Session 1. Remote Go 1.25.8 GUI/UI Window logic tests passed. Screenshot and
  real keyboard/titlebar input verified initial appearance, moving, rebuild and
  Hide/Show; the first manual scheduled wrapper expired before recording its
  process exit, so only its input assertions are counted, not a clean exit.
- macOS 15.7.7 arm64: all nine corresponding cases passed with an OpenGL
  preference, as did remote Go 1.24.9 scoped GUI/UI logic tests. VNC screenshots
  and real F5/F6/F7/titlebar drag confirmed appearance and preservation after
  manual movement; Esc exited with recorded code 0.
- Backend labels above are requested preferences: this UI probe does not read
  the private painter's Name, so a factory fallback is not independently ruled
  out. Positioning assertions use native geometry, not Painter-specific output.
  Application 1x/2x overrides do not establish real system HiDPI or mixed-DPI
  coverage. Windows 11, other WMs, multi-monitor selection, oversized windows
  and changed post-creation minimum sizes were not desktop-tested. Screenshot
  observations are not a timing measurement or a universal no-flicker guarantee.

An additional `-auto -center=false` case on local Xfwm4 fails its final
Hide/Show preservation assertion: the explicitly moved `(100,60)` window is
repositioned by the WM to `(620,362)` because it had no initial position hint.
The identical failure was reproduced from HEAD `90bdb83` in an isolated temporary
tree (only the new probe's unavailable Center modifier was omitted). Software
and OpenGL preferences reproduce it; native code was not changed, the assertion
remains, and this existing behavior is not counted as passed or silently fixed.
Center=true, explicit pre-Show overrides and Windows/macOS Center=false cases
passed their own preservation assertions.

Artifacts: `/tmp/goui-window-center.ODhq2N` locally,
`/tmp/goui-window-center.XoLZyJ` on macOS and
`C:\Users\xuyang8\AppData\Local\Temp\goui-center-ODhq2N` on Windows. Transfers
were SHA-256 checked. Windows remote-shell's claimed PowerShell default needed
an explicit powershell.exe invocation; large stdin Base64 uploads stalled and
were stopped. Task files were transferred using verified SSH/SFTP with existing
protected credentials, never printed. No production workaround was added for
tooling or session issues. Test windows, the owned Windows scheduled task, two
stalled upload processes, the isolated local display and the desktop client
connections created for this task were cleaned up. Existing SSH connections
remain healthy; artifacts are retained. GUI/UI code, package tests and the desktop
probe are committed as `650f293` (`Add window initial centering option.`), not
pushed. These pre-existing untracked design/route documents and the test index
remain in the working tree rather than bringing unrelated history into the commit.

## One-shot relative positioning

`DesktopWindow.SetPosition(relativeTo, position)` requests the **client-area
top-left**. Both target and reference use their client origins; native frame
offsets are private backend details, not a second public positioning contract.
There is no public desktop coordinate space, screen object or monitor
enumeration, and positioning does not create ownership or a transient parent.

- Non-nil reference: client-local DIP of that reference, including its scale
  override. The target's scale is not used to interpret the offset.
- Nil reference: offset from the work area returned by the target's
  `WorkAreaAt(geometry.Point{})` at invocation, in the target's DIP. It is not
  selected by the pointer, largest window intersection or the reference window.
  Conceptually this adds the area's client-local origin and positions relative
  to the target itself. The work area and origin are sampled once, not followed.
- Self-reference is valid. Negative/outside-area positions are valid. GOUI does
  not clamp, resize, change stacking or request activation. Native WM policy can
  still constrain or ignore placement; success means request submission only.
- Before Show, use the native initial origin if observable. Missing native
  observations return Unavailable, never a fabricated primary-screen origin.
- Unsupported backend/reference returns Unsupported; destroyed target/reference
  or missing observations returns Unavailable. Non-finite coordinates and native
  coordinate overflow are errors. Native errors propagate unchanged where the
  API provides them. Calls remain GUI-thread-affine.

`gui.Window` exposes the same `SetPosition` contract with a `gui.Window`
reference, and a thin `WorkAreaAt` query. It returns errors rather than logging
or silently substituting positioning policy. No UI position declaration is
introduced in this slice: placement is a command, not a persistent constraint.

### Observing position

`Position(relativeTo)` is an immediate native observation of the client-area
top-left, in the same reference/DIP convention as SetPosition. `nil` uses the
target's current `WorkAreaAt(Point{})`; self-reference returns `(0, 0)`.
The query never returns a cached SetPosition value
or pre-maximize restoration geometry, and does not wait for moves or run a nested
event loop. An immediate query after a request can legitimately return the old
position. Independent native queries are not an atomic transaction with the WM.

`gui.Window.Position` delegates with the same lifecycle/error checks as the setter.
No position signal, UI binding, screen identity or persistence API is introduced.
In particular, Position(nil) alone cannot remember which monitor a window used.

- Win32 observes ClientToScreen(0, 0). Subtract the reference's
  ClientToScreen origin and divide by the reference scale.
- AppKit observes NSWindow.contentRectForFrameRect(frame), converts its upper-left relative to the
  reference content rect, and divides by the reference logical-to-point scale.
- X11 observes the actual client root origin through XTranslateCoordinates.
  It does not need `_NET_FRAME_EXTENTS`, a mapped WM frame or decoration guesses.
  Unmapped windows can be queried when their native client geometry exists;
  observations can still change when the WM maps/reparents them.
  Missing native geometry fails; no cached request/restoration position is substituted.

Evidence: [ClientToScreen's device-coordinate contract](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-clienttoscreen)
and [EWMH 1.5 client-origin gravity](https://specifications.freedesktop.org/wm/1.5/ar01s09.html).
These are API/specification contracts, not claims that every WM honors requests.
Native runtime coverage is recorded in Plan separately.

The fixed `_NET_MOVERESIZE_WINDOW` atom is owned by X11 Platform.atoms and
interned once with the other protocol atoms during platform initialization
(`onlyIfExists=false`). SetPosition uses this identifier directly.
Interning a name does not prove WM support: `_NET_SUPPORTED` is read per operation.
Only identifiers are retained, never geometry or capability observations. This
small ownership correction does not change WorkAreaAt or transparent-window code.

### Native mapping

- Win32: reference ClientToScreen + reference scale, then SetWindowPos with
  NOSIZE | NOZORDER | NOACTIVATE. Convert the desired client origin to the HWND
  origin using the actual ClientToScreen/GetWindowRect difference, including
  invisible resize borders. Do not hardcode chrome/title-bar metrics.
- AppKit: reference content rect + logical-to-point scale, invert top-left Y,
  then convert the desired client origin using the actual frame/content-rect
  difference and call NSWindow.setFrameTopLeftPoint. The target is retained across the native
  call because delegate callbacks can destroy it. AppKit constraints still apply.
- X11: TranslateCoordinates to root, then reference DIP to native pixels.
  Managed windows use `_NET_MOVERESIZE_WINDOW`, explicit StaticGravity,
  x/y-only flags and application source. Missing EWMH support returns Unsupported.
  Withdrawn windows use XMoveWindow with PPosition/PWinGravity normal hints;
  minimum-size updates preserve those hints. Initial core-protocol positions
  must fit signed 16-bit coordinates; EWMH positions use signed 32-bit values.
  No frame-extents compensation or interactive-move emulation is used.

There is no SetClientPosition or public frame-position API. Native, Integrated
and None share the same client-origin contract. X11 gravity and request fields follow
[EWMH 1.5 sections 4.3 and 9.7](https://specifications.freedesktop.org/wm/latest-single/):
NorthWestGravity references the frame corner; StaticGravity references the client
corner. These are specification contracts, not claims about every WM's compliance.

### Validation and boundaries

Package tests cover request delegation, lifecycle/errors, rounding/overflow,
EWMH payload and coexistence of initial-position/minimum-size hints. The explicit
desktop entry is `tests/window/gui/window_position`: `-auto` checks client positions
before Show and after real WM processing in None/Native/Integrated modes;
multi-monitor/mixed-DPI behavior require separate desktop checks. The test does
not claim WM-independent placement guarantees.

This is the prerequisite positioning slice only. TabView cross-window page
transfer, tear-off creation and UI subtree coordination remain separate work.

### UI access through mounted window identity

`ui.App.FindWindow(id string) gui.Window` borrows the currently mounted window.
It follows the existing FindWidget approach instead of adding a UI window wrapper
or forwarding every GUI command through App. Call Position, SetPosition and
WorkAreaAt on that GUI object; errors remain visible to application code.

- Empty/missing IDs, an unmounting window, or a stopped App return nil. Lookup
  never creates a window and does not transfer ownership.
- Lookup uses App.Sync. The returned object is still GUI-thread-affine: perform
  lookup and use together in an event callback or App.Post/App.Sync. A successful
  lookup is not a lifetime guarantee across later callbacks or queued tasks.
- Ordinary rebuilds preserve the window object. Removing and recreating the same
  ID produces a new object; previously borrowed references are not retargeted.
  Do not change the borrowed window's ID or its declaratively managed content.
- A missing reference ID must not become a nil reference accidentally: nil has
  the explicit work-area-origin meaning, not "reference lookup failed".
- Building a WindowView does not make it discoverable. The native host is
  registered before content mounting and Show, so that its AfterUpdate can
  perform pre-Show positioning and binding using ordinary lookup. This does not guarantee
  native SizeEvents or WM requests have completed. AfterUpdate follows the outer
  coordinated update batch, not an OS-readiness notification. Post is
  scheduling, not a WM-completion barrier.

Example inside a GUI-thread event callback:

```go
target := app.FindWindow("editor")
reference := app.FindWindow("main")
if target == nil || reference == nil {
    return // Do not accidentally use work-area positioning.
}
err := target.SetPosition(reference, geometry.Point{X: 200, Y: 100})
// Application code decides how to report or recover from err.
```

No WindowView.Position declaration, position cache, polling or move signal is
introduced. Declarative rebuilds do not reapply placement and therefore do not
reset user-moved windows. First-show preparation is described below;
lookup alone does not prepare geometry or wait for native requests.

Reference rationale: Qt Quick 6.11's [Window x/y properties](https://doc.qt.io/qt-6/qml-qtquick-window.html)
use screen/virtual-desktop coordinates and a property-binding model; GOUI does not
adopt those semantics. Avalonia's [window access documentation](https://docs.avaloniaui.net/docs/how-to/window-how-to)
illustrates accessing the imperative top-level object from UI code. GOUI instead
uses its existing application-scoped IDs and keeps its relative DIP contract.
These are API-design references, not evidence of native positioning behavior.

Logic tests in ui/window_lookup_test.go cover identity, removal/native close,
same-ID recreation, stopped applications, unmounting and worker dispatch/exit.
The desktop program tests/window/ui/window_position uses real buttons/shortcuts
to exercise positioning, observations, rebuild preservation and window recreation.
It does not extend the native acceptance claims of the GUI positioning program.

### First-show preparation: current contract

ClientSize／PrepareLayout and the separate SetClientPosition proposal were
withdrawn. They are not current Window APIs and are not future prerequisites.
Applications use the source TabView content allocation already carried in the
detach request, add their own decoration overhead, position and transfer while
the new window is hidden, then Show. No hidden layout or exposed layout pixel
scale is needed; see [Tab Transfer §31](DesignTabTransfer.md#31-按-tabview-内容区尺寸创建拖出窗口2026-10-02).

AfterUpdate runs after the outer update batch has coordinated its windows. It is
not a native WM-completion or geometry-readiness barrier. Position observes
client origins; SetPosition submits a request using that same contract.

## One presentation state

WindowState is mutually exclusive: Normal, Hidden, Minimized, Maximized or
Fullscreen. Unknown is an observation sentinel, not a request target. Backend
restoration bookkeeping and multiple EWMH flags are not public combinable states.

- Normal is ordinary window presentation, not a generic restore operation.
- Hidden is a hidden/withdrawn window, not occlusion or an inactive workspace.
- Minimized remains distinct even when no window pixels are visible.
- Maximized and Fullscreen describe their corresponding native presentation.
- Unknown means failed/unavailable observation, including a destroyed window.

RequestState(Normal) asks for ordinary presentation, including leaving
minimization, maximization or fullscreen where supported. It does not restore a
remembered maximized mode. Show requests display and can preserve the previous
native mode. RequestState(Hidden) uses the same hiding mechanism as Hide.

Requests reject invalid targets and unsupported mechanisms. Nil means submitted,
not completed or guaranteed. OS constraints and intermediate native states remain
observable. Never assign the requested value to the observation or synthesize a
successful state-change event.

State() has no error result. Failed native observation produces Unknown, never
Normal. Chrome() likewise returns WindowChromeUnknown rather than echoing an
unverified creation hint. Unknown chrome is not accepted as a creation option.

## Events and GUI

StateEvent carries an observation obtained in response to native events.
Backends deduplicate equal observations. GUI delivers ConnectState through
DispatchEvent. Queries and requests do not manufacture signals.

Window marks itself destroyed before notifying destruction. Each state, focus
and close-request callback checks the window's lifetime at its own entry: if an
earlier callback destroys the window, later ordinary callbacks are skipped.
DispatchEvent ignores events after destruction and rechecks lifetime after modal
and window-focus callbacks before continuing into the Widget tree.

Destroy notifications are different: existing callbacks run before resources
are released, even if a callback reenters Destroy. Normal Handle blocking and
disconnection still apply. All Connect methods return inert Handles once
destruction starts. This is GUI-thread reentrancy protection, not a cross-thread
execution barrier.

core/signal remains independent of owner lifetime. Window does not bulk-disconnect
subscriptions: subscribers retain control through their Handles. Skipping a
callback does not promise immediate release of the closure or captured objects;
callers should disconnect and release Handles when no longer needed.

gui.Window keeps convenience methods including SetMinSize. Hosts without
DesktopWindow receive no desktop minimum-size hints; Widget layout minimums still
work. State reports Unknown when unavailable; GUI RequestState is best effort
and has no error return, while platform requests continue to report errors.
GUI Chrome is now an active signal service, not a native decoration getter;
see [DesignGUIWindowChrome.md](DesignGUIWindowChrome.md). GUI no longer exposes
HitTest/BeginMove/BeginResize/native-button geometry forwarding. ui remains a
thin gui adapter without platform imports.

GUI WindowOptions includes initial Size. The native factory is
NewWindow(size, handler, options); native options contain Chrome, not Size.
Initial size is a DIP preference; native SizeEvent remains authoritative.
All current desktop backends request client size in every Chrome mode, including
minimum hints. Native frame conversion stays private to each backend. See the [size convention](DesignWindowChrome.md#size-semantics).

Native window implementations live in window_<os>.go. The GUI desktop
adapter lives in window_desktop.go because it also covers native controls and
hit testing, not only state.

## Backend boundaries

- Windows: observe visibility, iconification and maximization. Normal uses an
  explicit SW_SHOWNORMAL request rather than exposing a generic restore command.
  Fullscreen is not inferred from monitor-sized geometry; fullscreen requests
  remain unsupported until a managed transition is implemented.
- macOS: observe visibility, miniaturization, native fullscreen and AppKit
  isZoomed. Zoom maps to Maximized: this means the platform's enlarged state,
  not necessarily a frame filling the work area. RequestState(Maximized) only
  calls zoom when not already zoomed; Normal unzooms after unhiding/restoring.
  AppKit owns restoration geometry. Native resize callbacks and a final native
  observation publish actual state, never a predicted target. Reentrant state
  requests during the native operation return ErrUnavailable. None windows
  without native Resizable support reject Maximized rather than synthesizing it.
  Fullscreen remains distinct and its unsupported requests still return errors.

macOS zoom regression: `GOUI_TEST_WINDOW_CHROME=1 go test ./platform/test -run 'TestMacOSZoomState|TestWindowChromeIntegrated' -count=1`.
Native/Integrated requests, duplicate requests, native zoom observation, original
frame restoration, hidden zoom restoration and destruction during notification
are covered. Execution requires a macOS desktop; cross-compilation is not visual validation.
- X11: distinguish withdrawal from iconification using ICCCM WM_STATE and
  interpret EWMH presentation flags. UnmapNotify alone does not mean Hidden.
  Submit EWMH requests to the WM; never write WM-owned mapped-window properties.
  Show plus explicit presentation requests must not become an ambiguous restore.

## Verification

- Window-only fakes need no desktop methods.
- Normal/hidden/minimized/maximized transitions are explicit; requests do not
  optimistically update observations.
- Unknown differs from Normal/Native and is not a request/creation target.
- Explicit and automatically derived GUI minimum hints skip non-desktop hosts.
- State events follow DispatchEvent and stop after destruction.
- Destruction inside state, focus or close-request callbacks skips later ordinary
  callbacks without interrupting destroy notifications or changing core signals.
- Late connections are inert; Handle blocking, unblocking and disconnection keep
  their existing semantics.
- Run go test ./..., shared/GUI race tests and Windows/macOS test-target cross
  compilation. Cross compilation does not replace native runtime verification.

Native X11 tests cover creation and explicit minimize/normal, maximize/minimize/
normal, hide/normal, and fullscreen/maximize/normal transitions at 1x and 2x.
Fullscreen exit precedes the next maximize request so WM restoration does not
discard that target. The tests query native state, not a request cache.

## References

- [Win32 ShowWindow](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-showwindow)
- [EWMH window state](https://specifications.freedesktop.org/wm/latest/ar01s05.html)
- [AppKit window drag](https://developer.apple.com/documentation/appkit/nswindow/performdrag(with:))
