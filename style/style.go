package style

import (
	"image/color"
	"slices"

	"github.com/golang-gui/goui/core/bits"
	"github.com/golang-gui/goui/core/colors"
)

type State int

const (
	Normal State = iota
	Hovered
	Pressed
	Focused
	Disabled
)

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
	fields          bits.Bitmap[uint64]
}

const (
	fieldBackgroundColor = iota
	fieldForegroundColor
	fieldBorderColor
	fieldBorderWidth
	fieldRadius
	fieldFontFamily
	fieldFontSize
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

type StyleSheet interface {
	Resolve(sel Sel) Style
}

func Sheet(rules ...Rule) StyleSheet {
	return sheet{rules: slices.Clone(rules)}
}

type sheet struct {
	rules []Rule
}

func (s sheet) Resolve(sel Sel) Style {
	return resolveRules(sel, s.rules, Style{})
}

// resolveRules accumulates every rule that matches a step of the fallback chain,
// later (more specific) matches merging over earlier ones. There is no local
// override list anymore: local code names a style, it does not supply rules.
func resolveRules(sel Sel, rules []Rule, base Style) Style {
	for _, candidate := range fallbackChain(sel) {
		for _, rule := range rules {
			if rule.Sel == candidate {
				base = base.merge(rule.Style)
			}
		}
	}
	return base
}

func fallbackChain(sel Sel) []Sel {
	normalDefault := Sel{
		Name:  sel.Name,
		Part:  PartDefault,
		State: Normal,
	}
	chain := []Sel{normalDefault}
	partNormal := Sel{
		Name:  sel.Name,
		Part:  sel.Part,
		State: Normal,
	}
	if partNormal != chain[len(chain)-1] {
		chain = append(chain, partNormal)
	}
	exact := Sel{
		Name:  sel.Name,
		Part:  sel.Part,
		State: sel.State,
	}
	if exact != chain[len(chain)-1] {
		chain = append(chain, exact)
	}
	return chain
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
		a.fontSize == b.fontSize
}
