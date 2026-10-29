package style

import (
	"image/color"
	"math/bits"
	"slices"

	corebits "github.com/golang-gui/goui/core/bits"
	"github.com/golang-gui/goui/core/colors"
)

// FontWeight is an OpenType weight request. Zero uses the native regular face.
type FontWeight uint16

const (
	FontWeightNormal FontWeight = 400
	FontWeightBold   FontWeight = 700
)

// FontStyle requests an upright, italic or oblique font face.
type FontStyle uint8

const (
	FontStyleNormal FontStyle = iota
	FontStyleItalic
	FontStyleOblique
)

// State is a set of simultaneous widget states. Normal is the empty set.
// Numeric values are not a persistence contract; use the named flags.
type State uint32

const Normal State = 0

const (
	Hovered State = 1 << iota
	Pressed
	Focused
	Disabled
	// FocusVisible implies Focused when resolving a stylesheet.
	FocusVisible
	ContainsFocus
	Selected
	Checked
	Mixed
	DropActive
)

// Has reports whether all flags are present. Has(Normal) is always true.
func (s State) Has(flags State) bool { return s&flags == flags }

// Normalized returns the state used for stylesheet matching. Disabled removes
// transient interaction states; keyboard-indicated focus implies logical focus.
func (s State) Normalized() State {
	if s.Has(Disabled) {
		return s &^ (Hovered | Pressed | Focused | FocusVisible | ContainsFocus)
	}
	if s.Has(FocusVisible) {
		s |= Focused
	}
	return s
}

const PartDefault = ""

type Sel struct {
	Name  string
	Part  string
	State State
}

type Style struct {
	backgroundColor color.Color
	foregroundColor color.Color
	borderColor     color.Color
	borderWidth     float32
	radius          float32
	fontFamily      string
	fontSize        float32
	fontWeight      FontWeight
	fontStyle       FontStyle
	underline       bool
	strikethrough   bool
	shadow          Shadow
	fields          corebits.Bitmap[uint64]
}

const (
	fieldBackgroundColor = iota
	fieldForegroundColor
	fieldBorderColor
	fieldBorderWidth
	fieldRadius
	fieldFontFamily
	fieldFontSize
	fieldFontWeight
	fieldFontStyle
	fieldUnderline
	fieldStrikethrough
	fieldShadow
)

func (s Style) BackgroundColor() (color.Color, bool) {
	return s.backgroundColor, s.fields.Check(fieldBackgroundColor)
}

func (s Style) ForegroundColor() (color.Color, bool) {
	return s.foregroundColor, s.fields.Check(fieldForegroundColor)
}

func (s Style) BorderColor() (color.Color, bool) {
	return s.borderColor, s.fields.Check(fieldBorderColor)
}

func (s Style) BorderWidth() (float32, bool) {
	return s.borderWidth, s.fields.Check(fieldBorderWidth)
}

func (s Style) Radius() (float32, bool) {
	return s.radius, s.fields.Check(fieldRadius)
}

func (s Style) FontFamily() (string, bool) {
	return s.fontFamily, s.fields.Check(fieldFontFamily)
}

func (s Style) FontSize() (float32, bool) {
	return s.fontSize, s.fields.Check(fieldFontSize)
}

// FontWeight returns the font weight and whether it was explicitly set.
func (s Style) FontWeight() (FontWeight, bool) { return s.fontWeight, s.fields.Check(fieldFontWeight) }

// FontStyle returns the font slant and whether it was explicitly set.
func (s Style) FontStyle() (FontStyle, bool) { return s.fontStyle, s.fields.Check(fieldFontStyle) }

// Underline returns the decoration and its presence, including explicit false.
func (s Style) Underline() (bool, bool) { return s.underline, s.fields.Check(fieldUnderline) }

// Strikethrough returns the decoration and its presence, including explicit false.
func (s Style) Strikethrough() (bool, bool) {
	return s.strikethrough, s.fields.Check(fieldStrikethrough)
}

// Shadow returns the complete shadow value and whether it was explicitly set.
// An explicitly set zero Shadow disables a shadow supplied by an earlier rule.
func (s Style) Shadow() (Shadow, bool) {
	return s.shadow, s.fields.Check(fieldShadow)
}

func (s *Style) setBackgroundColor(c color.Color) {
	s.backgroundColor = c
	s.fields.Set(fieldBackgroundColor, true)
}

func (s *Style) setForegroundColor(c color.Color) {
	s.foregroundColor = c
	s.fields.Set(fieldForegroundColor, true)
}

func (s *Style) setBorderColor(c color.Color) {
	s.borderColor = c
	s.fields.Set(fieldBorderColor, true)
}

func (s *Style) setBorderWidth(width float32) {
	s.borderWidth = width
	s.fields.Set(fieldBorderWidth, true)
}

func (s *Style) setRadius(radius float32) {
	s.radius = radius
	s.fields.Set(fieldRadius, true)
}

func (s *Style) setFontFamily(family string) {
	s.fontFamily = family
	s.fields.Set(fieldFontFamily, true)
}

func (s *Style) setFontSize(size float32) {
	s.fontSize = size
	s.fields.Set(fieldFontSize, true)
}

func (s *Style) setShadow(shadow Shadow) {
	s.shadow = shadow
	s.fields.Set(fieldShadow, true)
}

func (s Style) merge(override Style) Style {
	if override.fields.Check(fieldBackgroundColor) {
		s.setBackgroundColor(override.backgroundColor)
	}
	if override.fields.Check(fieldForegroundColor) {
		s.setForegroundColor(override.foregroundColor)
	}
	if override.fields.Check(fieldBorderColor) {
		s.setBorderColor(override.borderColor)
	}
	if override.fields.Check(fieldBorderWidth) {
		s.setBorderWidth(override.borderWidth)
	}
	if override.fields.Check(fieldRadius) {
		s.setRadius(override.radius)
	}
	if override.fields.Check(fieldFontFamily) {
		s.setFontFamily(override.fontFamily)
	}
	if override.fields.Check(fieldFontSize) {
		s.setFontSize(override.fontSize)
	}
	if override.fields.Check(fieldFontWeight) {
		s.fontWeight = override.fontWeight
		s.fields.Set(fieldFontWeight, true)
	}
	if override.fields.Check(fieldFontStyle) {
		s.fontStyle = override.fontStyle
		s.fields.Set(fieldFontStyle, true)
	}
	if override.fields.Check(fieldUnderline) {
		s.underline = override.underline
		s.fields.Set(fieldUnderline, true)
	}
	if override.fields.Check(fieldStrikethrough) {
		s.strikethrough = override.strikethrough
		s.fields.Set(fieldStrikethrough, true)
	}
	if override.fields.Check(fieldShadow) {
		s.setShadow(override.shadow)
	}
	return s
}

type Rule struct {
	Sel   Sel
	Style Style
}

func Default() Rule {
	return Rule{
		Sel: Sel{
			Part:  PartDefault,
			State: Normal,
		},
	}
}

func Name(name string) Rule {
	rule := Default()
	rule.Sel.Name = name
	return rule
}

func Part(part string) Rule {
	rule := Default()
	rule.Sel.Part = part
	return rule
}

func (r Rule) Part(part string) Rule {
	r.Sel.Part = part
	return r
}

func (r Rule) State(state State) Rule {
	r.Sel.State = state
	return r
}

func (r Rule) BackgroundColor(c color.Color) Rule {
	r.Style.setBackgroundColor(c)
	return r
}

func (r Rule) ForegroundColor(c color.Color) Rule {
	r.Style.setForegroundColor(c)
	return r
}

func (r Rule) TextColor(c color.Color) Rule {
	return r.ForegroundColor(c)
}

func (r Rule) BorderColor(c color.Color) Rule {
	r.Style.setBorderColor(c)
	return r
}

func (r Rule) BorderWidth(width float32) Rule {
	r.Style.setBorderWidth(width)
	return r
}

func (r Rule) Radius(radius float32) Rule {
	r.Style.setRadius(radius)
	return r
}

func (r Rule) FontFamily(family string) Rule {
	r.Style.setFontFamily(family)
	return r
}

func (r Rule) FontSize(size float32) Rule {
	r.Style.setFontSize(size)
	return r
}

// FontWeight sets a font weight. Zero selects regular; other values must be 100-900.
func (r Rule) FontWeight(weight FontWeight) Rule {
	if weight != 0 && (weight < 100 || weight > 900) {
		panic("style: invalid font weight")
	}
	r.Style.fontWeight = weight
	r.Style.fields.Set(fieldFontWeight, true)
	return r
}

// FontStyle sets a font slant, including an explicit upright face.
func (r Rule) FontStyle(slant FontStyle) Rule {
	if slant > FontStyleOblique {
		panic("style: invalid font style")
	}
	r.Style.fontStyle = slant
	r.Style.fields.Set(fieldFontStyle, true)
	return r
}

// Underline enables or explicitly disables underlining for Label text.
func (r Rule) Underline(value bool) Rule {
	r.Style.underline = value
	r.Style.fields.Set(fieldUnderline, true)
	return r
}

// Strikethrough enables or explicitly disables strikethrough for Label text.
func (r Rule) Strikethrough(value bool) Rule {
	r.Style.strikethrough = value
	r.Style.fields.Set(fieldStrikethrough, true)
	return r
}

// Shadow sets an outer box shadow as one atomic field. Members are not merged
// with earlier shadows. Pass Shadow{} to explicitly turn off an earlier shadow.
func (r Rule) Shadow(shadow Shadow) Rule {
	r.Style.setShadow(shadow)
	return r
}

type StyleSheet interface {
	Resolve(sel Sel) Style
}

func Sheet(rules ...Rule) StyleSheet {
	ordered := slices.Clone(rules)
	slices.SortStableFunc(ordered, func(a, b Rule) int {
		ka, kb := stateOrder(a.Sel.State), stateOrder(b.Sel.State)
		if ka < kb {
			return -1
		}
		if ka > kb {
			return 1
		}
		return 0
	})
	return sheet{rules: ordered}
}

type sheet struct {
	rules []Rule
}

func (s sheet) Resolve(sel Sel) Style {
	return resolveRules(sel, s.rules, Style{})
}

// Rules are sorted once by Sheet. Normal defaults and the requested Part
// precede subset-matching state rules; states from other Parts never leak.
func resolveRules(sel Sel, rules []Rule, base Style) Style {
	state := sel.State.Normalized()
	for _, rule := range rules {
		if rule.Sel.Name == sel.Name && rule.Sel.Part == PartDefault && rule.Sel.State == Normal {
			base = base.merge(rule.Style)
		}
	}
	if sel.Part != PartDefault {
		for _, rule := range rules {
			if rule.Sel.Name == sel.Name && rule.Sel.Part == sel.Part && rule.Sel.State == Normal {
				base = base.merge(rule.Style)
			}
		}
	}
	for _, rule := range rules {
		if rule.Sel.Name == sel.Name && rule.Sel.Part == sel.Part && rule.Sel.State != Normal && state.Has(rule.Sel.State) {
			base = base.merge(rule.Style)
		}
	}
	return base
}

func stateOrder(s State) uint64 {
	// Specificity first, then a fixed order for equally specific conditions.
	// Disabled rules override every enabled-state condition.
	key := uint64(bits.OnesCount32(uint32(s))) << 32
	for i, flag := range [...]State{ContainsFocus, Selected, Checked, Mixed, DropActive, Focused, FocusVisible, Hovered, Pressed, Disabled} {
		if s.Has(flag) {
			key |= 1 << i
		}
	}
	if s.Has(Disabled) {
		key |= 1 << 48
	}
	return key
}

func SameRules(a, b []Rule) bool {
	return slices.EqualFunc(a, b, sameRule)
}

func sameRule(a, b Rule) bool {
	return a.Sel == b.Sel && sameStyle(a.Style, b.Style)
}

func sameStyle(a, b Style) bool {
	return a.fields == b.fields &&
		colors.Equal(a.backgroundColor, b.backgroundColor) &&
		colors.Equal(a.foregroundColor, b.foregroundColor) &&
		colors.Equal(a.borderColor, b.borderColor) &&
		a.borderWidth == b.borderWidth &&
		a.radius == b.radius &&
		a.fontFamily == b.fontFamily &&
		a.fontSize == b.fontSize &&
		a.fontWeight == b.fontWeight && a.fontStyle == b.fontStyle &&
		a.underline == b.underline && a.strikethrough == b.strikethrough &&
		colors.Equal(a.shadow.Color, b.shadow.Color) &&
		a.shadow.Offset == b.shadow.Offset &&
		a.shadow.BlurRadius == b.shadow.BlurRadius &&
		a.shadow.SpreadRadius == b.shadow.SpreadRadius
}
