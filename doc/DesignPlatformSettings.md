# GOUI Platform Settings Design

This document records the platform boundary for system settings used by the GUI and style.
It is intentionally narrow. Platform settings are platform facts, not GUI
policy.

## Core Thesis

The platform layer should expose the ability to query operating-system settings.
It should not invent defaults, merge application preferences, or decide whether
the GUI should use light or dark styling when a system value is unavailable.

If a platform setting cannot be queried, the platform method returns an error.
The gui/style layer owns fallback policy.

## Interface

`platform.Platform` exposes a settings provider:

```go
type Platform interface {
	NewSettings() (common.Settings, error)
}
```

The settings provider is an interface because each field is an independent
capability. Some platforms may support only a subset:

```go
type Settings interface {
	Composited() (bool, error)
	ColorScheme() (ColorScheme, error)
	AccentColor() (color.Color, error)
	Language() (string, error)
	FontFamily() (string, error)
	FontSize() (float32, error)
}
```

The common unsupported error is:

```go
var ErrSettingUnsupported = errors.New("platform setting unsupported")
```

Callers should use `errors.Is(err, common.ErrSettingUnsupported)` when they need
to distinguish unsupported settings from query failures.

## Color Scheme

Color scheme describes the system preference when the platform provides one:

```go
type ColorScheme int

const (
	ColorSchemeLight ColorScheme = iota
	ColorSchemeDark
)
```

If the platform has no system color scheme concept, or GOUI has not implemented
that query for the platform, `ColorScheme` returns
`ErrSettingUnsupported`.

The platform layer must not assume light mode as a fallback.

## Accent Color

Accent color is the operating-system accent color when one exists and can be
queried. If there is no platform concept or the query is unavailable,
`AccentColor` returns `ErrSettingUnsupported`.

The platform layer must not provide GOUI's default accent color.

## Font Settings

`FontFamily` and `FontSize` expose the system UI font family and size when the
platform provides them.

If only one can be queried, only that method should succeed. The other method
should return an error. Partial failure is represented at the field method
level, not by failing a whole settings snapshot.

## Ownership

`color.Color` follows the standard-library premultiplied `RGBA()` contract.
Native straight-alpha values must be converted before constructing a
premultiplied `graphics.Color`, including the X11 GTK accent-color query.
`Language` reports the first preferred UI language as a BCP 47 tag, not regional
formatting preferences. Unsupported or failed queries return an error; GUI
normalization and the fallback `"en"` are described in
[System Language](DesignSystemLanguage.md). Native change notifications are not
implemented; GUI currently polls through its application timer.

The split is:

```text
platform:
  query system settings
  return exact values or errors

gui/style:
  merge user style configuration
  choose framework defaults
  decide fallback when platform values are unsupported or fail
```

This keeps platform factual and keeps style policy in the layer that owns
styling.

## Composition

`Composited` queries whether the current environment can composite surface alpha.
It is not the alpha configuration of a particular surface or a guarantee that
transparent resource creation will succeed. X11 reads the compositor selection
owner on each call; Windows queries DWM with its documented manifest compatibility
handling. Supported Cocoa, UIKit and Android environments use system compositors.
Android/iOS currently expose only this setting; other fields report unsupported.

The GUI caches this fact in its normal settings snapshot, falls back to false,
and uses the existing shared polling and notification cycle. Native transparent
surface creation still uses an immediate backend query, independent of that cache.
See [Settings](Settings.md) for notification and lifetime semantics.

## X11 desktop settings without a GTK event loop

The display owns one native XSETTINGS client, independent of libXcursor and the
number of windows or cursors. Settings getters read the current selection owner
and property synchronously, including before GOUI's event loop starts. GOUI's
existing X11 event dispatch also handles MANAGER, PropertyNotify and
DestroyNotify for cursor refresh. No getter or event handler iterates a GTK or
GLib main context. The GUI still polls its settings every two seconds; this is
not a new public native-change notification API.

| Field | X11 source and limits |
| --- | --- |
| ColorScheme | `Net/ThemeName`; the existing case-insensitive `dark` theme-name convention. XSETTINGS defines no separate standard light/dark preference. |
| AccentColor | The same theme name selects a borrowed `gtk_css_provider_get_named` provider. A new local GtkStyleContext looks up `theme_selected_bg_color`; it does not change GtkSettings or install a screen-wide provider. |
| FontFamily / FontSize | `Gtk/FontName`, parsed by Pango. Missing/unreadable settings retain the native Pango font-map fallback. Sizes expressed in absolute device units do not masquerade as points: family remains available, FontSize reports unsupported. |
| Startup scale | `GOUI_PLAT_SCALE`, then `GDK_SCALE`, then XSETTINGS `Xft/DPI` (1024 units per DPI), then X Resources `Xft.dpi`, then the existing 96-DPI baseline. Window geometry remains a startup snapshot. |

Missing manager/field, wrong types and malformed properties do not retain an old
theme. Color queries return errors and GUI applies its existing per-field
fallback. Font fallback is a native font query, not a platform-invented family or
point size. This source contract does not reproduce GTK's complete settings.ini,
GTK_THEME or portal precedence. CSS lookup still requires GTK 3; a missing named
theme follows GTK's own provider fallback. Editing the files of a cached theme
in place is outside this runtime theme-name switching contract.

The client grabs the X server only for owner discovery, subscription and bounded
property reads (at most 1 MiB). It releases the grab before parsing, CSS loading
or cursor reloading. The parser validates a complete typed snapshot before
publishing it; errors discard the whole snapshot. Cursor resources remain owned
by each cursor. Releasing the last cursor keeps the display's shared settings
subscription alive; platform destruction restores the root/current-owner masks.

References and their role:

- [XSETTINGS v0.5](https://specifications.freedesktop.org/xsettings/latest/)
  specifies selection discovery, server-grab protection, manager changes and
  complete property snapshots.
- [Qt 6.8.3 XCB client](https://github.com/qt/qtbase/blob/v6.8.3/src/plugins/platforms/xcb/qxcbxsettings.cpp)
  demonstrates a toolkit-owned native XSETTINGS reader. GOUI reuses its existing
  Xlib connection rather than adding Qt's abstraction or a second event loop.
- GTK 3.24.33's
  [X11 settings client](https://gitlab.gnome.org/GNOME/gtk/-/blob/3.24.33/gdk/x11/xsettings-client.c)
  updates its cache from native events. The original GOUI query observed the
  startup GtkSettings cache when GTK dispatch was absent; a diagnostic GLib pump
  confirmed the cause but is not part of the implementation.
- GTK 3 documents the borrowed ownership of
  [CssProvider.get_named](https://docs.gtk.org/gtk3/type_func.CssProvider.get_named.html)
  and the context-local scope of
  [StyleContext.add_provider](https://docs.gtk.org/gtk3/method.StyleContext.add_provider.html).
  This narrowly preserves GTK CSS color interpretation without adopting GTK
  widgets, global styling or its event loop.

The executable regression is `tests/window/platform/xsettings`; its entry
comment describes source errors, manager replacement, automatic UI rebuilds,
focus/selection retention and startup-scale checks. Measured environment and
verification limits are recorded in [Plan](Plan.md).

The fixed GTK CSS themes are Go strings in `themes_linux.go`. The executable
writes them to its private temporary data directory; it does not require a
generation step. For independent inspection, set `GOUI_GENERATE_DIR` to an
absolute directory outside the repository and run
`go generate ./tests/window/platform/xsettings`. The generator uses the same
strings and writes the CSS beneath that output directory through `nativegen`.
