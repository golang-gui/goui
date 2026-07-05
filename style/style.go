package style

import (
	"image/color"
	"slices"

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
	padding         float32
	fields          fields
}

type fields uint64

const (
	fieldBackgroundColor fields = 1 << iota
	fieldForegroundColor
	fieldBorderColor
	fieldBorderWidth
	fieldRadius
	fieldFontFamily
	fieldFontSize
	fieldPadding
)

func (s Style) BackgroundColor() (color.Color, bool) {
	return s.backgroundColor, s.fields&fieldBackgroundColor != 0
}

func (s Style) ForegroundColor() (color.Color, bool) {
	return s.foregroundColor, s.fields&fieldForegroundColor != 0
}

func (s Style) BorderColor() (color.Color, bool) {
	return s.borderColor, s.fields&fieldBorderColor != 0
}

func (s Style) BorderWidth() (float32, bool) {
	return s.borderWidth, s.fields&fieldBorderWidth != 0
}

func (s Style) Radius() (float32, bool) {
	return s.radius, s.fields&fieldRadius != 0
}

func (s Style) FontFamily() (string, bool) {
	return s.fontFamily, s.fields&fieldFontFamily != 0
}

func (s Style) FontSize() (float32, bool) {
	return s.fontSize, s.fields&fieldFontSize != 0
}

func (s Style) Padding() (float32, bool) {
	return s.padding, s.fields&fieldPadding != 0
}

func (s *Style) setBackgroundColor(c color.Color) {
	s.backgroundColor = c
	s.fields |= fieldBackgroundColor
}

func (s *Style) setForegroundColor(c color.Color) {
	s.foregroundColor = c
	s.fields |= fieldForegroundColor
}

func (s *Style) setBorderColor(c color.Color) {
	s.borderColor = c
	s.fields |= fieldBorderColor
}

func (s *Style) setBorderWidth(width float32) {
	s.borderWidth = width
	s.fields |= fieldBorderWidth
}

func (s *Style) setRadius(radius float32) {
	s.radius = radius
	s.fields |= fieldRadius
}

func (s *Style) setFontFamily(family string) {
	s.fontFamily = family
	s.fields |= fieldFontFamily
}

func (s *Style) setFontSize(size float32) {
	s.fontSize = size
	s.fields |= fieldFontSize
}

func (s *Style) setPadding(padding float32) {
	s.padding = padding
	s.fields |= fieldPadding
}

func (s Style) merge(override Style) Style {
	if override.fields&fieldBackgroundColor != 0 {
		s.setBackgroundColor(override.backgroundColor)
	}
	if override.fields&fieldForegroundColor != 0 {
		s.setForegroundColor(override.foregroundColor)
	}
	if override.fields&fieldBorderColor != 0 {
		s.setBorderColor(override.borderColor)
	}
	if override.fields&fieldBorderWidth != 0 {
		s.setBorderWidth(override.borderWidth)
	}
	if override.fields&fieldRadius != 0 {
		s.setRadius(override.radius)
	}
	if override.fields&fieldFontFamily != 0 {
		s.setFontFamily(override.fontFamily)
	}
	if override.fields&fieldFontSize != 0 {
		s.setFontSize(override.fontSize)
	}
	if override.fields&fieldPadding != 0 {
		s.setPadding(override.padding)
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

func (r Rule) Padding(padding float32) Rule {
	r.Style.setPadding(padding)
	return r
}

type StyleSheet interface {
	Resolve(sel Sel, local []Rule) Style
}

func Sheet(rules ...Rule) StyleSheet {
	return sheet{rules: slices.Clone(rules)}
}

type sheet struct {
	rules []Rule
}

func (s sheet) Resolve(sel Sel, local []Rule) Style {
	base := resolveRules(sel, s.rules, false, Style{})
	return resolveRules(sel, local, true, base)
}

func resolveRules(sel Sel, rules []Rule, local bool, base Style) Style {
	for _, candidate := range fallbackChain(sel) {
		for _, rule := range rules {
			ruleSel := rule.Sel
			if local && ruleSel.Name == "" {
				ruleSel.Name = candidate.Name
			}
			if ruleSel == candidate {
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
		a.fontSize == b.fontSize &&
		a.padding == b.padding
}
