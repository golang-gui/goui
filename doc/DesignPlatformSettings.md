# GOUI Platform Settings Design

This document records the platform boundary for system settings used by style.
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

`color.Color` is the standard-library color interface; conversion to the drawing
backend's premultiplied `graphics.Color` occurs at the rendering boundary.
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
