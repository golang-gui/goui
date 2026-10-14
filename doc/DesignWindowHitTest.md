# Native Window Hit Testing and Interactive Requests

This document describes platform capabilities. GUI integration now uses the
[active Chrome signal service](DesignGUIWindowChrome.md), not public GUI
SetHitTest/BeginMove/BeginResize forwarding or a Widget HitTester interface.

Status: capability split implemented. SetHitTest is a native query,
not a cross-platform pointer-event policy. This replaces the previous X11/macOS
simulated query, as well as WindowFrameRegions and its rectangle snapshots.

## API and ownership

~~~go
SetHitTest(func(p geometry.Point) WindowHit) error
BeginMove() error
BeginResize(edge WindowEdge) error
~~~

platform asks what native role applies at a point; gui reads its existing Widget
tree and policy. No Widget references or GUI hit semantics enter platform.
SetHitTest is a synchronous query, not an event signal or an action callback.

| Backend | SetHitTest | BeginMove | BeginResize |
| --- | --- | --- | --- |
| Windows | WM_NCHITTEST | ErrUnsupported | ErrUnsupported |
| X11 | ErrUnsupported | _NET_WM_MOVERESIZE, when advertised by WM | same, eight edges/corners |
| macOS | ErrUnsupported | performWindowDragWithEvent | explicit backend-managed eight-direction resize |

Windows does not implement a second client-input move/resize path. X11 and
macOS do not store a SetHitTest callback, including when passed nil. GUI first
tries SetHitTest; only errors.Is(err, ErrUnsupported) selects explicit input
requests. Other errors are reported, not interpreted as an absent capability.

- Coordinates are window-client DIP, possibly outside the client bounds.
- Default defers to native handling. Client explicitly keeps a customizable
  region in client input; it must not fall back to Caption/resize afterwards.
- On a supported backend nil unregisters. Replacement does not accumulate
  callbacks. Destroy releases the callback; GUI detaches before destroying its
  tree. A failed installation leaves the previous callback untouched.
- Callbacks use the owning thread, can be frequent/reentrant, and read completed
  layout/state only. No layout, drawing, event dispatch, mutation, modal loops or
  destruction. Reentrant queries safely defer to Default rather than recursively
  entering the same callback. Invalid WindowHit values also defer to Default.

GUI reuses normal visible-child ordering and local-coordinate hit logic.
HeaderBar blank space can return Caption; interactive children return Client;
WindowControls may expose caption roles. Mere Widget presence or focusability
does not distinguish draggable from interactive content. Modal, overlay and
capture policy remain GUI decisions.

## Native input, not synthesized button behavior

Returning a role never triggers an action during the query. Native drag/resize
uses the original input context. GUI-painted buttons receive input through
platform events -> Window.DispatchEvent -> EventDispatcher -> controller ->
signal. Do not execute both a native caption command and a GUI click.

- Windows: map roles to WM_NCHITTEST results. Preserve system-owned controls.
  Route custom caption-button pointer input to GUI, retaining native hover
  handling needed for HTMAXBUTTON/Snap. Moving/resizing stays with the WM. No
  blanket native-edge precedence may override Client in a customizable region.
- X11: forward client input normally. Only an explicit BeginMove/BeginResize
  request submits _NET_WM_MOVERESIZE, using the original root pixel coordinates,
  button and timestamp. Release the implicit grab first. If the client receives
  the release before the WM takes over, send EWMH CANCEL and forward the real
  release. Do not implement a manual move/resize loop.
- macOS: BeginMove delegates the original press to performWindowDragWithEvent.
  BeginResize is a deliberately limited adaptation: capture the initial frame
  and screen-point position, track native left drag/up, and update the frame
  with minimum-size constraints while keeping the opposite edge fixed. Screen
  coordinates and positioning remain private. No nested event loop, cursor
  changes or native edge double-click emulation is introduced. Once accepted,
  resize owns the drag/up; hide, focus loss and destruction cancel tracking.
  None omits AppKit's Resizable flag so native border tracking cannot compete
  with the caller's client regions. Native/Integrated keep their native borders.
  GouiContentView still returns false from mouseDownCanMoveWindow. SetHitTest
  remains unsupported, not simulated.

BeginResize must be called synchronously from the original native left
PointerDown dispatch. BeginMove also accepts a native left-held drag dispatch
of the same window, allowing GUI drag thresholds without retaining an event.
X11 validates the original press and current native motion; macOS uses the
current leftMouseDragged event. Resize remains unavailable from motion.
WindowEdge has eight distinct edge/
corner values (not flags and not WindowHit caption/button roles). No public
coordinates, timestamp, retained input token or GUI action bypass is introduced.
Normal GUI use goes through Window.DispatchEvent -> EventDispatcher -> controller;
the controller decides whether blank header space or an edge initiates a request.
SetHitTest itself never installs this GUI fallback.

The native press context expires on handler return, nested native event/task
dispatch, destruction or one successful request. Missing/expired input and a
destroyed window return ErrUnavailable. Unsupported mechanisms/WM support return
ErrUnsupported; other native failures are preserved. Nil means submitted, not
accepted or completed. macOS retains native objects across the modal drag call.

On success the controller consumes the down event without starting a click or
pointer capture: the system may consume PointerUp. The platform does not invent
a release, click, hover, resize cursor or GUI lock policy. Fallback cursor choice
belongs to GUI and is separate from starting system interaction. Native OS input
injection may follow the native path; constructing a PointerEvent or posting a
task does not manufacture native input context. Move supports GUI thresholds
through native left-held motion; Resize is press-only. Right-button and touch
initiation remain unsupported.

Locking affects only interactions initiated through customizable regions. It is
not a permission system: keyboard, system menu and WM commands are not all
intercepted. Close remains the vetoable close request.

## Native controls are a separate observation

~~~go
ControlsRect() (geometry.Rectangle, error)
SetControlsPosition(position *geometry.Point) error
~~~

ControlsRect returns the union of visible native caption buttons in window-client DIP. No GUI
padding, external border metrics or handles are included; this is not a placement
request. Native chrome may put the rectangle outside the client area. Empty means
an observed absence; unsupported/unavailable observations return errors, never
guessed fixed button sizes.

macOS converts actual standard NSButton bounds into the content view, then GOUI's
top-left DIP coordinates. Windows converts DWM caption-button area bounds from
window-relative physical coordinates. X11 has no portable WM-button query and
returns ErrUnsupported.

SetControlsPosition is a separate request, currently only supported by macOS
Integrated. It copies a finite, non-negative top-left group origin in client DIP;
nil restores the original native placement, while `(0, 0)` is an actual position.
Windows/X11 and other chrome modes return ErrUnsupported. Buttons remain native;
their sizes, gaps, actions and accessibility do not become GUI state. There is
no new event or public GUI/UI forwarding method in this platform slice.
Resize/scale/show reapply the preference; fullscreen transitions suspend it and
keep AppKit's fullscreen layout. Non-nil requests are unavailable then, while
nil can clear the preference. ControlsRect continues to report actual geometry,
never the remembered preference. See [DesignWindowChrome.md](DesignWindowChrome.md)
for placement constraints, lifecycle and native verification.

Windows Integrated uses version-specific DWM chrome (Win10 popup style plus one
top pixel, Win11+ overlapped style without that pixel) and
reports an empty native caption-button rectangle; cached DWM bounds must not
become ghost button hit regions. Its remaining native border is the Default
fallback, with a DPI-aware client-side top resize band and correction of overly
tall native top corners. Explicit Client/caption roles can override customizable
regions; maximized windows do not get that normal top band. macOS Integrated
retains the actual traffic lights in the full-size client; it does not gain
simulated SetHitTest.

There is no dedicated controls-rectangle notification. GUI queries during layout
and after existing size/presentation changes; DIP alone does not imply invariant
geometry. Add a notification only if a real native change cannot be covered by
those paths. Ordinary content Widgets do not consume this query. AppKit owns
button lifetime, input and accessibility. Integrated still requires actual
placement/visibility validation on native desktops and a GUI layout adapter.

## Tests

Tests cover backend capability results (including nil), original X11 message
data and all eight directions, current-press expiry/reentrancy/one-shot use,
ordinary client-input delivery and window lifetime. GUI tests cover pass-through
errors, native-first selection and detachment before tree destruction. No
controls-geometry event or subscription is introduced.

macOS tests additionally cover the content view's mouseDownCanMoveWindow override
and native button placement/reset, resize, geometry preservation and window
isolation. `example/window_chrome -move=false -trace-input` separates absence of
implicit dragging from explicit BeginMove. `-controls-position 16,18` exercises
placement; R resets to native layout. Actual drag, click and relocated native
button hit areas still require a macOS desktop.

Runtime follow-up on Windows/macOS must cover native ControlsRect at 1x/2x and
while hidden/minimized/fullscreen, caption-button input, AppKit dragging, Snap,
native accessibility and all painters before claiming integrated chrome. X11 WM
drag/resize has been exercised on Xfce through XTEST at 1x/2x (native move and
bottom-right resize, duplicate and posted-request rejection); other WMs and
full CSD visuals still require runtime verification.
Never enable unimplemented Integrated as a successful no-op.

~~~sh
go test ./...
go test -race ./gui ./ui ./platform/linux/x11
# Opt in only on a test desktop: moves a test window and restores the pointer.
GOUI_TEST_WINDOW_INTERACTION=1 GOUI_PLAT_SCALE=1 go test ./platform/test -run TestX11NativeWindowInteraction -count=1 -v
GOUI_TEST_WINDOW_INTERACTION=1 GOUI_PLAT_SCALE=2 go test ./platform/test -run TestX11NativeWindowInteraction -count=1 -v
~~~

Windows amd64 and macOS amd64/arm64 test targets cross-compile. Native Windows
and macOS interaction testing is still required; cross compilation is not that
verification. `example/plat_test` demonstrates native-first capability selection
without changing its existing None rendering into full custom chrome.
`example/window_chrome` separately exercises Windows/macOS Integrated, including
custom Windows buttons and the actual macOS occupied region; see
[DesignWindowChrome.md](DesignWindowChrome.md) for the native acceptance matrix.

## References

- [WM_NCHITTEST](https://learn.microsoft.com/en-us/windows/win32/inputdev/wm-nchittest)
- [Snap layouts](https://learn.microsoft.com/en-us/windows/apps/desktop/modernize/ui/apply-snap-layout-menu)
- [EWMH move/resize](https://specifications.freedesktop.org/wm/latest/ar01s04.html)
- [AppKit window drag](https://developer.apple.com/documentation/appkit/nswindow/performdrag(with:))
- [DWM button bounds](https://learn.microsoft.com/en-us/windows/win32/api/dwmapi/ne-dwmapi-dwmwindowattribute)
