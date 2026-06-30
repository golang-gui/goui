# GOUI Design Principles

This document records the core design principles of GOUI. It is not an API
reference and should not be treated as an implementation checklist. Its purpose
is to help human contributors and AI agents make consistent design decisions as
the project evolves.

## Core Thesis

GOUI is built around a clear three-layer architecture:

- platform: low-level platform facts and native integration.
- gui: an imperative widget kernel.
- ui: a declarative view layer built on top of gui.

The layers are intentionally separate. This increases short-term implementation
work, but it keeps responsibilities explicit and makes the system easier to
reason about, test, and extend.

The design favors clarity over cleverness. GOUI should not force Go to imitate
GTK, Qt, SwiftUI, Flutter, or the web. It may learn from them, but the final
shape must fit Go's language model: explicit data flow, simple interfaces,
ordinary values, and minimal hidden behavior.

## Layer Responsibilities

The platform layer exposes platform facts. It creates windows, event loops,
graphics backends, typography contexts, and native input events. It should avoid
GUI policy. For example, platform should report pointer, wheel, key, focus,
paint, size, and scale events, but it should not decide widget hover state,
button behavior, layout, or declarative updates.

The gui layer is the imperative widget kernel. It owns windows, widgets, layout,
painting, event dispatch, event controllers, focus state, lifecycle, signals,
and semantic snapshots. It is the stable middle layer that proves the platform
layer is usable and gives the declarative layer a reliable target.

The ui layer is declarative. It describes what the interface should look like and
updates the gui widget tree accordingly. It should remain thin. It should reuse
gui widgets, setters, layout, painting, event dispatch, and signals instead of
creating a parallel GUI framework.

## Platform Principles

Platform APIs should be minimal and factual. They should expose what the
operating system provides, normalize only what is necessary, and avoid guessing
high-level GUI meaning too early.

Platform objects are thread-affine unless explicitly documented otherwise.
Cross-thread interaction should go through the event loop posting mechanism.
This keeps platform integration predictable and avoids implicit queued behavior
in higher layers.

Platform changes should be verified before higher layers depend on them. A new
input event, rendering primitive, or window capability should first be validated
at the platform level, then integrated into gui, and only then exposed through
ui when needed.

## GUI Principles

The gui layer is not merely an implementation detail, but it is not the primary
application authoring surface either. Most application code should use ui.
Complex custom controls may use gui directly.

Window is not a Widget. A window is a host for a widget tree and a boundary to
the native platform window. It owns painting, event dispatch, lifecycle, and
resources that are scoped to the native window.

Widget and Container are separate concepts. A widget is a node in the visual and
semantic tree. A container is a widget that can own child widgets. Not every
widget should carry container semantics.

WidgetBase is a helper for implementing widgets. It may centralize common tree,
state, layout, painting, event-controller, focus, and lifecycle behavior, but it
should not become an inheritance system. GOUI should use Go embedding and small
interfaces pragmatically, without pretending Go has virtual classes.

GUI state should be ordinary Go state unless a stronger abstraction is proven
necessary. MVP does not need a property system. Setters are enough when they
perform the required layout, paint, semantic, or signal notifications.

## Signals

Signals are the unified notification mechanism of the gui layer.

GOUI should not provide both signal-style connections and SetOnXXX-style single
callback APIs for the same GUI event. Two parallel mechanisms create ambiguity:
which one should users choose, which one wins, and how do internal controls,
debuggers, automation tools, and external listeners coexist?

Signals are more general and therefore the preferred GUI mechanism. They support
multiple listeners, independent disconnection, temporary blocking, and lifecycle
management through handles.

The declarative ui layer may expose OnXXX chain methods, but those methods are
declarative syntax, not a separate event model. Internally they may connect to
gui signals and manage handles through the ui reconciliation mechanism.

Signals should be used where meaningful semantic notifications exist. GOUI
should avoid turning every field into a property-changed signal. A small number
of important notifications is better than a large reactive surface that is hard
to reason about.

## Event Dispatch Principles

Real input and simulated input must use the same path. Automation and AI-driven
interaction should dispatch platform events into windows, not bypass behavior
through helper methods such as direct click-by-id actions.

The event path is platform event, Window.DispatchEvent, EventDispatcher,
EventController, widget state or signal, layout or paint, semantic snapshot.
This path is central to correctness.

EventDispatcher owns hit testing, propagation paths, and propagation phases.
EventController interprets input into widget-specific behavior and semantic
signals. Widgets and applications should normally care about semantic signals,
not raw platform events.

Propagation should stay explicit. Hidden queued event handling, implicit async
delivery, and action side channels should be avoided unless there is a strong
reason to add them.

## Declarative UI Principles

The declarative layer should be a small adapter over gui, not a second widget
system.

View values describe UI structure. Root reconciles those values into a gui
widget tree. Reuse is based on parent position and concrete view type during
MVP. Stable identity for dynamic lists can be added later when the need is
concrete.

The declarative syntax should be Go-friendly. Chain methods are acceptable when
they produce clearer code than struct literals. The API should avoid unsafe
self-referential tricks, embedded Props structures, and abstractions that only
exist to mimic other languages.

The declarative layer should hide signal handles from normal application code.
Application authors should write OnXXX callbacks and request updates. The ui
runtime should manage signal connection lifetimes internally.

Declarative updates should be explicit. State can be ordinary Go variables.
When state changes, code requests an update. A full reactive state system,
automatic dependency tracking, and property binding are not required for MVP and
should not be introduced prematurely.

## Layout and Painting Principles

Layout algorithms should be separate from widgets where practical. A layout
manager should operate on measurable and arrangeable children, not on full
widget objects. This keeps layout reusable and limits dependencies.

Measure and arrange are distinct phases. Measure computes desired size under
constraints. Arrange assigns final rectangles. Painting uses the assigned
rectangles.

Widget rectangles are parent-local. Semantic snapshots expose window-local
absolute bounds, because automation and AI agents should not need to reconstruct
the widget tree geometry manually.

Painting should use widget-local coordinates. Parent painting should translate
and clip child painters so custom controls can draw relative to their own origin.

## Lifecycle and Resource Ownership

Window owns the final fallback lifecycle of its widget tree and window-scoped
resources. Widgets should release resources as early as they safely can, but
window destruction must be able to clean up what remains.

Mount and unmount describe tree attachment. They should be precise about when
Parent, Root, and Window are still valid. Resource cleanup should not depend on
ambiguous ordering.

Declarative roots should clean up their own signal handles and reconciliation
state. They should not steal ownership of gui widget destruction from the
window.

## Text and Input Principles

Text input is not just key handling. Key events, text editing, IME composition,
clipboard, selection, caret geometry, focus, platform keyboard state, and
accessibility all interact. These concerns should be added incrementally and
only at the layer where they belong.

Pure key and pointer events belong in platform and gui. Text input and IME
composition require a more careful design and should not be forced into the same
event model prematurely.

Declarative text controls must eventually distinguish controlled and
uncontrolled behavior. If a view explicitly provides text, the widget is driven
by external state. If it does not, the widget may preserve internal editing
state. This should be designed deliberately rather than accidentally.

## Style and Settings Principles

Style should affect drawing, not replace widget behavior. It should be small,
value-oriented, and easy to reason about.

System settings such as dark mode, accent colors, double-click interval, cursor
behavior, and platform conventions belong below or at the gui boundary, not in
application code. Platform-specific reading and change notification should be
implemented in platform and consumed by gui.

GOUI should learn from CSS, but it should not become CSS. Tree-level style
inheritance and local style overrides are useful. Browser compatibility,
selector complexity, and large implicit cascades are not MVP goals.

## Semantic and AI Integration Principles

Semantic snapshots are part of the core architecture, not an afterthought. A
widget tree should be inspectable in terms of roles, names, text, bounds,
visibility, focus, and supported actions.

AI and automation should interact with the application through the same event
path as users. They may use semantic snapshots to decide what to do, but the
operation itself should be expressed as platform input events dispatched to a
window.

Clear architecture improves AI correctness. When each layer has a narrow role,
AI agents are less likely to modify the wrong subsystem or create shortcuts that
work in one test but violate the design.

## Evolution Principles

Prefer small, verifiable steps. Platform additions, gui integration, and ui
exposure should be implemented and tested separately when possible.

Do not add abstractions because another toolkit has them. Add them when GOUI has
a concrete need and the abstraction reduces complexity in Go.

Avoid parallel APIs for the same concept. If one mechanism is more general and
fits the architecture, prefer one clear mechanism over two partially overlapping
ones.

Keep MVP conclusions stable. The project has already validated the core
architecture: platform adaptation works, the imperative gui layer is useful, and
Go can express a simple declarative UI through chainable view values. Future
work should build on this foundation rather than reopen settled decisions
without strong evidence.

When tradeoffs appear, choose the design that makes ownership, event flow,
threading, and lifecycle easier to explain. A design that is easy to explain is
usually easier for both humans and AI agents to implement correctly.
