# Component styles and local shadow verification (2026-10-17)

The independent SourceView find panel needs native TextInputs and buttons with
an editor-owned palette, including two different palettes in one window. GOUI
provides explicit StyleScope/ResolveWidgetStyle and exposes the existing backend
BoxShadow through the widget-local Painter. GOUI has no SourceView dependency.
Design and constraints are in DesignStyle.md, DesignPaint.md and DesignShadow.md.

- `go test ./...`: passed locally, Linux amd64. Includes the user's existing
  local tests; no desktop program is counted as executed by this command.
- `go test -race ./gui ./widgets`: passed.
- `go test ./gui ./widgets`: passed after final scope-slot/cycle checks.
- `go vet ./gui ./widgets ./ui`: failed on existing unkeyed TextSelection/
  TextRange literals. Comparing against pre-change HEAD 6f684b6 with the same
  user-owned untracked test files gives the same 167 diagnostic lines; no new
  diagnostic was introduced. Those files are outside this change.
- The first commit's shadow-only staged tree passed `go test ./gui ./widgets`
  independently. Painter tests assert local rect/radius/effect, translated clip,
  transform and suppression under an empty clip. Consumer painter mocks updated.
- Native macOS 15.7.7 arm64 / Go 1.24.9 and Windows 10 19045 amd64 / Go 1.24.0:
  `go test ./gui ./widgets ./ui -count=1` passed in isolated clean task copies.
  The consuming SourceView `go test ./... -count=1` and its find, shortcuts and
  appearance window program builds passed on both hosts.
- Linux X11 / Pango, task-owned Xtigervnc and Xfwm4 (compositing off): SourceView
  find interaction and appearance programs passed with Software 1x/2x and OpenGL
  1x. They check native input/panel solid colors for independent light/dark
  scopes, palette replacement, overlay allocation, narrowing/restoration, focus,
  hit testing and destruction. Real desktop captures were also visually checked
  at 1x, including an actual window-manager resize.

Native Windows/macOS desktop composition and interaction, fractional/mixed DPI,
real IME and additional fonts were not executed. Cross-platform package tests
and program builds do not replace those checks. No global desktop settings or
user files changed. Task-owned displays, processes and transfer server closed.
The full logs and temporary snapshots are outside the source tree.
