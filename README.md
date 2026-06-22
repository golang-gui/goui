# goui

[中文文档](./doc/README_CN.md)

goui is a cross-platform desktop GUI framework for Go under active development.

The project aims to build a modern GUI runtime that is natural to use from Go and straightforward for AI models to inspect and drive. It relies on the windowing, graphics, and text-layout facilities already provided by each operating system, without requiring a CGO toolchain or third-party native runtime libraries.

> [!IMPORTANT]
> goui is still at an early stage. Its platform graphics and typography foundations are usable, but the high-level UI, stable API, and complete event system are not finished. It is not ready for production use.

## Why Go

Go does not have a mature, unified desktop GUI ecosystem, but it has several properties that make it suitable for building one:

- A simple language and engineering model make framework and application code easier to read, maintain, and generate.
- Native compilation and single-binary distribution provide a practical deployment model without an additional language runtime.
- Low GC latency is a good fit for interactive applications.
- Goroutines work well for I/O, computation, and business tasks, while UI state can be committed serially through the event loop.
- The standard toolchain supports testing, profiling, cross-compilation, and automation.

goui does not attempt to copy an existing framework or reimplement an entire graphics stack in Go. Heavy tasks such as window management, hardware-accelerated drawing, font shaping, and text layout are delegated to mature platform facilities. A Go-native UI abstraction is built on top of them.

## Core Goals

### No CGO Toolchain

goui calls system APIs at runtime through [`purego`](https://github.com/ebitengine/purego) and [`goexlib/cgo`](https://github.com/goexlib/cgo). Despite its package name, `goexlib/cgo` is a dynamic-call bridge and does not require Go's CGO build mode.

The module currently requires Go 1.24. The intended repository-wide build command is:

```bash
CGO_ENABLED=0 go build ./...
```

Building an application should not require a C/C++ compiler, native headers, or custom linker scripts.

### No Bundled Third-Party Dynamic Libraries

Apart from libraries supplied by the target operating system, goui does not require applications to distribute additional third-party dynamic libraries. Go module dependencies are compiled into the application rather than shipped as native runtime components.

Linux systems must still provide system shared libraries such as X11, OpenGL, Pango, Cairo, Fontconfig, and GLib. Minimal distributions and containers may need to install these runtime packages through their system package manager.

### Platform-Appropriate Backends

goui does not pursue pixel-identical output across platforms. It aims for highly similar structure, layout, and interaction while preserving each platform's strengths in font rendering, rasterization, and window behavior.

| Platform | Window System | Primary Graphics Backend | Typography |
| --- | --- | --- | --- |
| Windows | Win32 | Direct2D | DirectWrite |
| macOS | Cocoa | nanovgo + OpenGL | Core Text |
| Linux | X11 | nanovgo + OpenGL | Pango |

The current implementation also provides software-rendering fallback when Direct2D is unavailable on Windows or OpenGL initialization fails on macOS and Linux.

### Designed for Automation and AI

Inspectability and controllability are core requirements rather than later additions. UI elements should support:

- Stable textual or structured descriptions containing hierarchy, identity, text, state, geometry, and available actions.
- Simulation of pointer, keyboard, text-input, and focus events.
- UI queries, waits, assertions, and event replay.
- Reusable skills that allow AI models to write, run, and debug goui applications.

The same foundation will support conventional automated testing, accessibility semantics, and debugging tools.

## Architecture Direction

```text
Applications and high-level UI
              │
              ├── Imperative widget API (first stage)
              └── Declarative View / Element runtime (later)
              │
Layout, input, focus, semantics, and scene graph
              │
Unified Graphics / Typography interfaces
              │
Platform: windows, event loop, and dynamic system bindings
              │
┌──────────────┬────────────────┬────────────────┐
│ Windows      │ macOS          │ Linux          │
│ Win32        │ Cocoa          │ X11            │
│ Direct2D     │ OpenGL/nanovgo │ OpenGL/nanovgo │
│ DirectWrite  │ Core Text      │ Pango          │
└──────────────┴────────────────┴────────────────┘
```

The `platform` layer contains platform-specific capabilities and bindings, but no high-level widget semantics. Drawing and text layout are exposed through the unified `platform/graphics` and `platform/typography` interfaces, so upper layers do not depend directly on Win32, Cocoa, X11, DirectWrite, or Pango types.

Event processing will use a unified event-loop abstraction while respecting each platform's UI-thread requirements. Expensive work may run in goroutines, but UI tree updates, layout, event dispatch, and frame submission should be coordinated serially by the UI event loop.

## Current Status

Implemented or available as a working foundation:

- Basic platform bindings for Windows, macOS, and Linux.
- Win32, Cocoa, and X11 window implementations.
- Direct2D and nanovgo/OpenGL graphics backends.
- DirectWrite, Core Text, and Pango text-layout backends.
- Bitmap, brush, path, clipping, primitive shape, image, and text drawing interfaces.
- Cross-platform graphics and typography tests.

Being revised:

- The current `EventQueue` does not have fully consistent semantics across platforms and does not naturally match the macOS application event model.
- The event system will be replaced with an event-loop abstraction that can support input dispatch, UI scheduling, and frame lifecycle management.
- Existing public APIs may still change substantially.

Not yet implemented:

- A complete high-level UI API for applications.
- Basic widgets, layouts, focus handling, keyboard input, and IME integration.
- A scene graph, declarative UI, and stable state-update model.
- A complete inspection, simulation, and replay protocol for AI and automated tests.

## Roadmap

### 1. Event Loop and Imperative UI Foundation

- Replace the platform event queues with a unified event-loop abstraction.
- Establish the minimum complete path for windows, UI scheduling, input dispatch, invalidation, and repainting.
- Implement `Label`, `Button`, `TextInput`, `ImageView`, and `ListView`.
- Implement `BoxLayout` and `FlexBoxLayout`.
- Validate APIs, lifecycle, layout, and cross-platform behavior with small but complete examples.

The first stage prioritizes validating the core design over building a large widget library.

### 2. Inspectable, Simulatable, and Debuggable UI

- Define stable identity, semantics, and structured descriptions for UI elements.
- Establish a unified pointer, keyboard, text, and focus event model.
- Support input simulation, UI queries, waits, assertions, and event replay.
- Provide application inspection and control interfaces suitable for skills.
- Allow AI models to diagnose applications from actual runtime state rather than relying only on source code and screenshots.

These requirements will be designed alongside the basic widgets instead of being retrofitted later.

### 3. Scene Graph and Declarative UI

- Represent drawing output as an inspectable and cacheable scene graph.
- Separate declarative descriptions, persistent elements, layout objects, and drawing nodes.
- Implement state-driven UI construction, updates, and localized invalidation.
- Support more precise repainting, caching, hit testing, and frame scheduling.
- Build a modern declarative GUI framework without sacrificing Go readability.

## Design Principles

- **Prefer platform capabilities:** Reuse mature system graphics, typography, and windowing facilities.
- **Consistent, not pixel-identical:** Unify framework semantics while preserving the best platform-specific presentation.
- **Prefer the Go toolchain:** Do not require CGO, a C compiler, or an additional native build process.
- **Keep platform boundaries explicit:** Contain platform differences within `platform` rather than leaking them into widgets.
- **Let the event loop own UI state:** Background goroutines must not mutate the UI tree concurrently.
- **Treat testability as a core capability:** UI descriptions, event simulation, and state inspection are part of the framework protocol.
- **Validate before expanding:** Prove the design with a minimal widget set before building the declarative runtime and broader ecosystem.

## Repository Layout

```text
core/
  geometry/              Basic geometry types

platform/
  common/                Shared platform interfaces
  events/                Current event types
  graphics/              Unified graphics API and backends
  typography/            Unified typography API and platform implementations
  windows/               Win32 and Windows SDK dynamic bindings
  darwin/                Cocoa and macOS framework dynamic bindings
  linux/                 X11 and Linux system-library dynamic bindings
```

## Contributing

The immediate priorities are stabilizing the platform event loop and implementing the first imperative widgets. Changes should preserve the following constraints:

- Keep `CGO_ENABLED=0` builds working.
- Do not add third-party dynamic libraries that applications must distribute.
- Keep platform differences behind clear backend boundaries.
- Design new UI capabilities together with inspectability and input simulation.
- Do not sacrifice platform text and interaction quality for superficial pixel consistency.

## License

MIT License.
