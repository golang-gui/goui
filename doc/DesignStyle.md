# GOUI Style System Design

This document records the current direction for GOUI's style system. It is a
design note, not an implementation checklist. The goal is a small style model
that fits GOUI's architecture: style is ordinary value data, widgets remain in
charge of behavior, and style resolution is explicit enough to explain.

## Core Thesis

Local style is a **semantic reference, not concrete values**. The declarative
layer names an *intent*; the theme decides what that intent looks like:

```go
Button("Delete")                  // default style
Button("Delete").Style(Danger)    // "Danger" is a name defined by the theme; to ui it is a string
```

Three rules define the whole system:

1. **Local code may only name a style — never set concrete values.** No color,
   no font at the call site. Without this, theming / dark mode cannot be
   guaranteed: a single hard-coded color escapes the theme. **There is no escape
   hatch.** Concrete values live only in the stylesheet; shared colors are just
   ordinary Go variables reused across rules (there is no separate token system —
   see Theme & Tokens).
2. **Style serves purely visual properties: color, font, border (radius, …).**
   Layout — padding, alignment, size, weight — is **not** style; it lives on
   layout fields (see DesignLayout). Style produces **values (nouns)**; it never
   *executes* layout, paint, or animation (**verbs**).
3. **Style is static.** It does not animate or recompute at runtime. For dynamic
   appearance, rebuild the UI declaratively (an application concern); the style
   system stays a plain lookup.

Two sources, as before:

- **global / theme** (framework default sheet + application, or an explicit
  `gui.StyleScope` subtree): an ordinary
  stylesheet defining what each semantic name looks like.
- **local** (`.Style(name)` on a widget/view): names intent only.

The mechanism below (`Sel` / `Rule` / `Resolve`, parts, states) is the
**sheet-author's** machinery and is unchanged. What this finalization changes is
the *local* surface (semantic names, not values) and the boundary (see Non-Goals).

> Status: the basic value/selector model is implemented. `padding` is a layout field (not style);
> `ViewBase.Style` takes a single semantic name; `gui.Widget` no longer carries
> local rules; `StyleSheet.Resolve` takes only a `Sel`. Rule values are ordinary
> colors — no token/theme system. The opt-in Modern sheet and runtime invalidation
> are specified in [Modern style](DesignModernStyle.md), in six ordered slices.
> A nil custom sheet keeps the minimal GUI fallback; it does not enable Modern.

### Box shadow and runtime updates

`style.Shadow` is one optional, atomic style field containing a `color.Color`, a
DIP offset, blur radius and spread radius. `Rule.Shadow` sets the complete field;
`Style.Shadow` returns its value and presence. An explicit zero shadow overrides
an earlier shadow rather than merging individual members. `SameRules` compares
colors by RGBA value. The style package has no platform dependency and does not
allocate rendering resources or grant widgets automatic out-of-bounds painting.

`Widget.StyleChanged` now invalidates derived resources before subsequent
framework layout/paint. Changes are marked on sheet replacement, local style-name
changes, effective enabled-state changes and attachment, then coalesced until
framework use. Invalidations need not result in different resolved style values.
ScrollView also
delivers pending notifications before querying specialized Scrollable geometry.
The base implementation is a no-op; the framework handles measurement invalidation
and scheduling independently of overrides. Detached widgets are refreshed on
attachment, not kept in a global registry. This is a framework hook, not another
application signal. Hover/pressed state remains the widget's responsibility.

UI subscribes to system
Settings changes and requests a rebuild internally; user state still uses explicit
updates. Window decoration remains GUI/platform-owned: application sheets can
change the window background, not Native/Integrated frame geometry or controls.
Window-decoration isolation is implemented: private GUI geometry and palettes
replace the former `window/frame` and `window-control` application rules.
Custom caption buttons retain Button input behavior, but do not use application
Button colors, fonts or box effects. Their private light/dark palette follows
the window background; native decorations remain OS-owned. UI Settings automatic
updates are implemented: one run-scoped subscription coalesces rebuilds, with
cleanup on initialization failure and native-loop exit. GUI getters read the
resolved snapshot; polling only runs during Application.Run. Equal sheets still
skip application-wide style invalidation.

The opt-in `theme/modern` package is implemented. Its Options/Sheet/Rules are plain
values and functions with no settings subscriptions. The `FromSettings(gui.Settings)`
convenience entry depends on GUI types, but GUI/UI never depend on Modern. It supplies
light/dark palettes, accent tints, fonts and the complete menu/shadow rules, including
the `primary` button and `muted-text` label names. See `theme/modern/README.md` and
`example/modern` for explicit enablement and system-following usage. PopoverMenu's
full shell and row-text consumption are implemented. Complete fallback rules live
in `theme/basic`; `widgets/style` is only a compatibility forwarding entry.

Button and MenuButton retain an independent `focus` paint pass over their
hover/pressed background. `Focused` is logical focus; `FocusVisible` is a
GUI-managed keyboard-navigation hint, also exposed read-only on Widget and in
Snapshot. Successful default Tab enables hints, pointer presses hide them;
programmatic focus preserves the host's mode. Window and Popover modes are isolated.
GUI fallback and Modern use a transparent background and BorderWidth 0 for ordinary
Focused; FocusVisible uses a transparent background and a 2 DIP inset border in
a softened accent color for ordinary buttons, keeping hover/pressed fill and
layout unchanged. Modern Primary alone uses dark grey in light mode and light
grey in dark mode, as documented in [Modern Presets](DesignModernPresets.md).
The pass paints optional background and border before child widgets. Applications
can override either state; FocusVisible inherits fields not explicitly replaced
from the same part's Focused rules. Its default background/color/width are explicit,
so customize keyboard borders with FocusVisible, not a less-specific Focused rule.
Custom complete sheets should explicitly
set a transparent focus background when they only want a border. TextInput retains
its ordinary focused border. See DesignKeyboardNavigation §7.

### Theme & Tokens are not separate systems

There is **one concept: the stylesheet**. A "theme" is just a complete stylesheet;
re-skinning (light/dark) is `App.SetStyleSheet(other)` or explicitly replacing a
subtree sheet — swapping the sheet, not a
distinct `Theme` type. "Semantic color tokens" are just Go variables reused when
building a sheet:

```go
danger := color.RGB(0xC1, 0x1, 0x1) // an ordinary variable, reused across rules
sheet := style.Sheet(
    style.Name("danger").BackgroundColor(danger),        // the "danger" button/box style
    style.Name("danger-text").ForegroundColor(danger),   // reuse the same red for text
)
```

No framework-level token table, no `Theme` abstraction, no semantic-color layer —
only *style name* (local) and *stylesheet* (author). If a real need appears later
(runtime token override), a token/Theme layer can be added on
top without reworking this. The chain builders below (`BackgroundColor(...)`,
`Radius(...)`, …) are **kept** as the way to author a `StyleSheet`.

## Non-Goals — the boundary that stops style becoming a paint system

CSS bloated because "presentation" had no edge and its property bag accepted
anything. GOUI's style has a **closed vocabulary** (`.Style(name)` only)
and one admission test:

> **Both yes → it is style:** ① is it a static *semantic → value* mapping? ②
> does the value *change with the theme*?

Everything that bloats CSS fails the test and has another home:

| Concern | In style? | Home |
|---|---|---|
| color / font / border / radius / box shadow | ✅ | **style** |
| alignment / direction / padding / size / weight | ❌ (algorithm/scalar, not themed) | **layout** (fields) |
| animation / transition | ❌ (time process; for dynamic, rebuild UI) | **animation** |
| transform / clip / opacity | ❌ (compositing) | **scene** (DesignPaint) |
| icon / image | ❌ (asset/content) | **asset** |

Style holds **nouns**, never **verbs**: layout arranges, scene draws, animation
interpolates, assets supply icons — each *reads values from* style. Because the
local vocabulary is closed, there is no entry point for those to leak in.

## Core Types

The core model is intentionally small:

```go
package style

type State int

const (
	Normal State = iota
	Hovered
	Pressed
	Focused
	Disabled
	FocusVisible
)

type Sel struct {
	Name  string
	Part  string
	State State
}

type Style struct {
	// Optional style fields.
}

type Rule struct {
	Sel   Sel
	Style Style
}

type StyleSheet interface {
	Resolve(sel Sel) Style
}
```

`Style` is pure style data. It does not contain selector data. `Rule` is the
combination of selector data and style data. `Resolve` takes only a selector —
there is no local-override list, because local code names a style, it does not
supply rules (see Non-Goals and Declarative UI Responsibilities).

`State` is an integer because the supported widget states are fixed and should
be fast to compare. States are not treated as a bitset in the MVP. A widget asks
for one effective state at a time, such as default, hovered, pressed, focused,
or disabled.

`Name` and `Part` are strings because both need to be open to user-defined
values.

## Selector Meaning

`Sel.Name` is the widget's style name. It is not the widget identity.

By default, a widget should use a stable built-in style name:

```text
button
text-input
label
```

Applications override the style name via `.Style(name)` (= `SetStyleName`), which
affects style lookup only. It is separate from identity: `.Name(id)` sets the
reconciliation ID, not the style name.

`Sel.Part` identifies a paintable or layout-relevant part of one widget. The
default part is the empty string:

```go
const PartDefault = ""
```

Examples of parts include:

```text
selection
caret
placeholder
focus-ring
icon
```

Parts are not child widgets. They are named style targets inside a widget. A
text input can resolve different styles for its root box, normal text,
placeholder text, selected text, selection background, and caret without
creating extra widgets.

`Sel.State` identifies the widget state for this style query. `Normal`
means the base style for that name and part.

## Rule Builders

Rules are how a **style-sheet author** builds a sheet — the chain API sets visual
fields on a selector:

```go
sheet := style.Sheet(
	style.Name("button").BackgroundColor(color.Gray).Radius(4),
	style.Name("button").State(style.Hovered).BackgroundColor(color.LightGray),
	style.Name("text-input").Part("selection").BackgroundColor(color.Blue),
	style.Name("text-input").Part("selection").State(style.Pressed).BackgroundColor(color.Gray),
)
```

There is **no local-rule form**: local code never authors rules, it only names a
style (`.Style(name)`), so the builders exist purely to author sheets. Builder
names stay direct:

```go
style.Default()          // default part, default state
style.Name("text-input") // a named style
style.Part("selection")  // a part of the current style
```

`style.Default()` builds a rule for the default part and the base (`Normal`)
state. Only **visual** fields exist (color / font / border / radius) — padding
and other layout scalars are **not** style (see DesignLayout).

## Style Sheet & Resolution Pipeline

A sheet is built from rules:

```go
func Sheet(rules ...Rule) StyleSheet
```

Where the sheet comes from:

- `App.StyleSheet()` — the application's **custom** sheet, or nil when none is set
  (explicit; `SetStyleSheet(nil)` reverts).
- `DefaultStyleSheet()` — the complete **Basic fallback** used when there is no
  custom sheet, including controls in `widgets` and their independently styled
  text. Values live in `theme/basic`; GUI owns only the resolution entry point.
  Modern remains an explicitly selected appearance. See
  [Theme fallback and convenience entry](DesignThemeFallback.md).

`StyleSheet.Resolve` owns only the merge + fallback-chain policy of one sheet:

```go
type StyleSheet interface {
	Resolve(sel Sel) Style
}
```

Resolution is **one seam, three layers**, and no widget is threaded into the
resolver:

```go
// 1. control → name: the explicit override, else the control's own constant.
//    The control inlines this (a single if) using the public StyleName();
//    there is deliberately no WidgetBase helper — a private method could not
//    serve controls defined in other packages.
name := b.StyleName()
if name == "" {
	name = styleNameButton
}

// 2. name → Style: the single exported entry point, gui.ResolveStyle; it knows
//    nothing about widgets, so third-party controls call the very same seam.
ResolveStyle(name, style.PartDefault, state)

// 3. inside ResolveStyle: pick the current sheet, then Resolve.
// A custom sheet still replaces Basic; there is no implicit per-field overlay.
sheet := DefaultStyleSheet()
if App != nil {
	if s := App.StyleSheet(); s != nil {
		sheet = s
	}
}
return sheet.Resolve(style.Sel{Name: name, Part: part, State: state})
```

`StyleName()` stays the raw override (`""` when unset); the control inlines the
fallback to its own default constant, and the fallback to `DefaultStyleSheet`
lives in `ResolveStyle` — resolver jobs, not getter jobs (see DesignConventions
§5: explicit getter + named resolver). **`ResolveStyle` is exported on purpose:**
resolving a style is not a `gui`-internal privilege, so a control in another
package uses the same public seam — `Widget.StyleName()` + its own name constant
+ `gui.ResolveStyle` — with no private `WidgetBase` helper to lock control
authorship inside `gui`.

## Resolution Rules

Within one sheet, resolution matches intuition:

- more-specific matches override less-specific ones, field by field.
- state-specific fields override default-state fields.
- part-specific fields override default-part fields.
- fields a rule did not set keep the previously resolved value.

The fallback chain for a selector (later overrides earlier, field-wise):

```text
Name + default part + Normal
Name + Part         + Normal
Name + Part         + Focused      (only when exact state is FocusVisible)
Name + Part         + exact state
```

Every rule whose `Sel` equals a step is merged in order. This gives useful
same-widget defaults without parent inheritance: a text input sets a foreground
on its default part, and its placeholder part overrides the foreground only when
it needs a different one — a same-widget fallback, not a parent-to-child cascade.

Duplicate steps collapse. A non-default part does not inherit another part's
state-specific fields; the Focused step above always uses the requested Part.

There is **no local layer** and no cross-widget cascade: a widget resolves one
name against one sheet. Different appearance = a different name (`.Style(name)`)
or a different sheet (`App.SetStyleSheet`).

## Optional Fields

`Style` fields must preserve whether a value was explicitly set. Plain zero
values are not enough because zero can be a valid user choice:

```go
style.Default().Radius(0)
style.Default().BackgroundColor(color.Transparent)
```

The implementation can use per-field option values or a bitset of explicitly
set fields. The public API should remain value-oriented and chainable.

Merge is field-wise:

```text
if override has field X set, replace base field X
otherwise keep base field X
```

## No Automatic Inheritance

The MVP style system has no automatic parent style inheritance. A parent
background, border, padding, or text color does not silently affect child
widgets.

If users want consistent styling across a subtree, they name the same style on
each widget (`.Style(name)`) or install a custom application sheet or explicit StyleScope. That is a
user decision, not a hidden framework behavior.

This keeps ownership clear:

- the application sheet (or the bare `DefaultStyleSheet`) provides the values.
- each widget names which style it wants (`SetStyleName`).
- widgets define which parts they support and how styles affect layout and
  painting.

## Widget Responsibilities

Each widget should define:

- its default style name.
- the parts it supports.
- how to compute its current effective state.
- which style fields affect measurement.
- which style fields affect painting.

The style package resolves values. The widget interprets those values.

For example, a text input resolves through the single exported entry, applying
its own default name inline:

```go
name := t.StyleName()
if name == "" {
	name = styleNameTextInput
}

rootStyle      := ResolveStyle(name, style.PartDefault, state)
selectionStyle := ResolveStyle(name, "selection", state)
caretStyle     := ResolveStyle(name, "caret", state)
```

The widget then uses those resolved styles during measure and paint. Its
"default style name" is a compile-time constant applied at the call site — not a
field it stores or an interface method it exposes.

## Declarative UI Responsibilities

The declarative layer exposes exactly one visual entry point — a single semantic
name, no values, no variants:

```go
func (b *ViewBase[T]) Style(name string) *T
```

```go
Button("Delete").Style(Danger)   // Danger etc. are string consts from the theme's package
Label("Title").Style(Heading)
VBox(...).Style(Card)
```

- `name` — the semantic style name, *defined by the theme / style package*; to ui
  it is just a string. It maps 1:1 to `Sel.Name`, i.e. `.Style(name)` is exactly
  `widget.SetStyleName(name)`.
- **A variant is just another name.** `outlined` / `filled` are the same kind of
  thing as `primary` / `danger` — each selects one look. Want "outlined primary"?
  That is one semantic name (the theme builds it by copying a base). The style
  combinations are role × variant only (size/density already left style for layout
  scalars), so they are finite and defined on demand — no explosion, no `sel`.
- **Overwrite semantics**: the last `.Style(name)` wins, like `.Padding()` /
  `.MinWidth()` — a single name backs a single value.
- **No `.Style(rule...)`, no `.Background()/.Foreground()/.Font()`, no part
  constructors in local code, no local rules at all.** Parts, states, and concrete
  values live in the theme. A plain `Button()` uses its default style name
  (`"button"`).
- Layout is **separate and never style**: `.Padding()`, `.MinWidth()`,
  `.MainWeight()` are layout fields on the same view (see DesignLayout).

Semantic name constants belong to the application/theme's own package
(`mystyle.Danger = "danger"`), not to `ui`. The ui layer forwards the string to
`SetStyleName`; the theme's sheet resolves `(name, part, state)` to concrete
values via the mechanism above (no local override list; `gui.Widget` no longer
carries `StyleRules`).

> Future: if freely composing several orthogonal styles *without* pre-defining
> the combination becomes a real need, `.Style(name)` can grow to
> `.Style(names ...string)` (CSS-class-style multi-name merge) — a pure addition,
> no rework.

Parts and states have not gone away — they moved entirely to the **theme side**.
A multi-part widget (text input's selection/caret/placeholder) still resolves per
part and state, but only the theme names them; local code never touches a part.

## Deferred Evaluation (Not a Committed API)

The selector can grow later if there is a concrete need:

```go
type Sel struct {
	Name  string
	Class string
	ID    string
	Part  string
	State State
}
```

These should not be added until the simple `Name + Part + State` model proves
insufficient. `Name` should keep its current meaning as the widget style name,
even if class or ID are added later.

The current implementation uses complete Basic defaults, application-level
overrides, part/state styles and deterministic field-wise merge; no widget-local
rule lists or implicit inheritance. Multi-name selectors, Class/ID selectors and
style animation are unimplemented evaluation directions, not agreed APIs; see
[Future Roadmap](Roadmap.md#future-style).

## Explicit subtree sheets

`gui.StyleScope` is a transparent single-child container selecting a complete,
immutable `style.StyleSheet`. `ResolveWidgetStyle(widget, name, part, state)` uses
the nearest scope, then the application/Basic sheet when no scope supplies one.
An incomplete scope is not implicitly merged with outer sheets. Nil delegates;
ordinary parents still do not inherit font, color or other individual properties.
`ResolveStyle` remains explicitly application-wide. Built-in controls and compound
widgets use the widget-aware entry; custom widgets should do likewise.

This addresses embedded components with independent palettes in one window.
The reference is [Flutter 3.35.4 Theme source](https://github.com/flutter/flutter/blob/3.35.4/packages/flutter/lib/src/material/theme.dart):
it provides a nearest-ancestor theme. GOUI adopts explicit subtree ownership,
using existing StyleChanged invalidation rather than dependency tracking or automatic
rebuilds. No local concrete property setters or local rule lists are introduced.
Scope sheet replacement invalidates descendants before layout/paint, including
nested scopes conservatively. Reparenting uses the existing attachment invalidation.
Sheets own no native resources; consumers retain and release their own caches.
A native popup has a different root and does not implicitly capture the opener's
scope. A popup-specific palette must be installed explicitly.

Implementation verification and remaining platform gaps: [component styles](PlanComponentStyles.md).
