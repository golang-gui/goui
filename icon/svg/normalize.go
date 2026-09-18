package svg

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/srwiley/rasterx"
)

// Normalize only the supported static profile before handing geometry parsing
// to oksvg. In particular, do not trust StrictErrorMode to reject attributes
// that oksvg ignores. No CSS selectors, resource fetching or path parser here.
type scope struct {
	matrix                     rasterx.Matrix2D
	fillOpacity, strokeOpacity float64
	stroke, dash               string
	strokeWidth, dashOffset    float64
	fill                       string
	fillRule                   string
}

func normalize(data []byte) ([]byte, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var out bytes.Buffer
	encoder := xml.NewEncoder(&out)
	stack := []scope{{matrix: rasterx.Identity, fillOpacity: 1, strokeOpacity: 1, strokeWidth: 1, stroke: "none", fill: "black", fillRule: "nonzero"}}
	seenRoot, roots := false, 0
	gradients := make(map[string]bool)
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("svg: XML: %w", err)
		}
		switch t := token.(type) {
		case xml.StartElement:
			if len(stack) > 64 {
				return nil, fmt.Errorf("svg: nesting exceeds 64 elements")
			}
			name := t.Name.Local
			if t.Name.Space != "" && t.Name.Space != "http://www.w3.org/2000/svg" {
				return nil, fmt.Errorf("svg: unsupported namespace %q", t.Name.Space)
			}
			if len(stack) == 1 {
				roots++
				if roots != 1 || name != "svg" {
					return nil, fmt.Errorf("svg: exactly one svg root required")
				}
				seenRoot = true
			} else if name == "svg" {
				return nil, fmt.Errorf("svg: nested svg is unsupported")
			}
			switch name {
			case "svg", "g", "path", "rect", "circle", "ellipse", "line", "polyline", "polygon", "defs", "linearGradient", "radialGradient", "stop", "title", "desc":
			default:
				return nil, fmt.Errorf("svg: unsupported element %q", name)
			}
			attrs, err := attributes(t)
			if err != nil {
				return nil, err
			}
			current := stack[len(stack)-1]
			if err := normalizeAttributes(name, attrs, &current, gradients); err != nil {
				return nil, err
			}
			stack = append(stack, current)
			keys := make([]string, 0, len(attrs))
			for k := range attrs {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			t = xml.StartElement{Name: xml.Name{Local: name}}
			for _, k := range keys {
				t.Attr = append(t.Attr, xml.Attr{Name: xml.Name{Local: k}, Value: attrs[k]})
			}
			token = t
		case xml.EndElement:
			if len(stack) <= 1 {
				return nil, fmt.Errorf("svg: unbalanced elements")
			}
			stack = stack[:len(stack)-1]
			token = xml.EndElement{Name: xml.Name{Local: t.Name.Local}}
		case xml.Directive:
			return nil, fmt.Errorf("svg: XML directives/DOCTYPE are unsupported")
		case xml.CharData:
			if len(stack) == 1 && strings.TrimSpace(string(t)) != "" {
				return nil, fmt.Errorf("svg: text outside root")
			}
		case xml.ProcInst, xml.Comment:
			continue
		}
		if err := encoder.EncodeToken(token); err != nil {
			return nil, fmt.Errorf("svg: encode: %w", err)
		}
	}
	if !seenRoot || len(stack) != 1 {
		return nil, fmt.Errorf("svg: complete svg root required")
	}
	if err := encoder.Flush(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func attributes(t xml.StartElement) (map[string]string, error) {
	attrs := make(map[string]string)
	inline := ""
	for _, a := range t.Attr {
		k := a.Name.Local
		if k == "xmlns" || a.Name.Space == "xmlns" {
			continue
		}
		if a.Name.Space != "" {
			return nil, fmt.Errorf("svg: unsupported namespaced attribute %q", k)
		}
		if k == "style" {
			inline = a.Value
			continue
		}
		if _, exists := attrs[k]; exists {
			return nil, fmt.Errorf("svg: duplicate attribute %q", k)
		}
		attrs[k] = strings.TrimSpace(a.Value)
	}
	for _, declaration := range strings.Split(inline, ";") {
		if strings.TrimSpace(declaration) == "" {
			continue
		}
		k, v, ok := strings.Cut(declaration, ":")
		if !ok {
			return nil, fmt.Errorf("svg: invalid inline style")
		}
		attrs[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	for k := range attrs {
		switch k {
		case "viewBox", "width", "height", "x", "y", "cx", "cy", "r", "rx", "ry", "x1", "y1", "x2", "y2", "fx", "fy", "d", "points", "id", "fill", "fill-rule", "fill-opacity", "stroke", "stroke-width", "stroke-opacity", "stroke-linecap", "stroke-linejoin", "stroke-miterlimit", "stroke-dasharray", "stroke-dashoffset", "transform", "opacity", "preserveAspectRatio", "gradientUnits", "spreadMethod", "offset", "stop-color", "stop-opacity":
		case "version", "role":
			delete(attrs, k)
		default:
			if strings.HasPrefix(k, "aria-") || strings.HasPrefix(k, "data-") {
				delete(attrs, k)
				continue
			}
			return nil, fmt.Errorf("svg: unsupported attribute/style %q", k)
		}
	}
	return attrs, nil
}

func normalizeAttributes(name string, a map[string]string, s *scope, gradients map[string]bool) error {
	if v, ok := a["opacity"]; ok {
		n, err := number(v)
		if err != nil || n != 1 {
			return fmt.Errorf("svg: opacity requires unsupported offscreen compositing; use fill-opacity/stroke-opacity")
		}
		delete(a, "opacity")
	}
	if v, ok := a["preserveAspectRatio"]; ok {
		if v != "xMidYMid" && v != "xMidYMid meet" {
			return fmt.Errorf("svg: only centered aspect-preserving meet is supported")
		}
		delete(a, "preserveAspectRatio")
	}
	if v, ok := a["gradientUnits"]; ok && v != "objectBoundingBox" {
		return fmt.Errorf("svg: only objectBoundingBox gradients are supported")
	}
	if name == "linearGradient" || name == "radialGradient" {
		id := a["id"]
		if id == "" || gradients[id] {
			return fmt.Errorf("svg: gradient requires a unique id")
		}
		gradients[id] = true
	}
	for _, k := range []string{"fill", "stroke", "stop-color"} {
		if a[k] == "currentColor" {
			a[k] = "black"
		}
		if v := a[k]; strings.Contains(v, "url(") {
			if !strings.HasPrefix(v, "url(#") || !strings.HasSuffix(v, ")") || !gradients[v[5:len(v)-1]] {
				return fmt.Errorf("svg: %s requires an earlier local gradient definition", k)
			}
		}
	}
	if v, ok := a["fill"]; ok {
		s.fill = v
	}
	if v, ok := a["stroke"]; ok {
		s.stroke = v
	}
	if v, ok := a["fill-rule"]; ok {
		if v != "nonzero" && v != "evenodd" {
			return fmt.Errorf("svg: invalid fill-rule %q", v)
		}
		s.fillRule = v
	}
	for _, p := range []struct {
		k   string
		dst *float64
	}{
		{"fill-opacity", &s.fillOpacity}, {"stroke-opacity", &s.strokeOpacity},
		{"stroke-width", &s.strokeWidth}, {"stroke-dashoffset", &s.dashOffset},
	} {
		if v, ok := a[p.k]; ok {
			n, err := number(v)
			if err != nil {
				return err
			}
			if strings.HasSuffix(p.k, "opacity") && (n < 0 || n > 1) || p.k == "stroke-width" && n < 0 {
				return fmt.Errorf("svg: invalid %s", p.k)
			}
			*p.dst = n
		}
	}
	if v, ok := a["stroke-dasharray"]; ok {
		s.dash = v
	}
	if v, ok := a["transform"]; ok {
		m, err := transform(v)
		if err != nil {
			return err
		}
		s.matrix = s.matrix.Mult(m)
	}
	// Flatten transforms and opacity inheritance onto leaves. oksvg otherwise
	// multiplies inherited fill-opacity overrides and does not scale strokes.
	for _, k := range []string{"transform", "fill-opacity", "stroke-opacity", "stroke-width", "stroke-dashoffset", "stroke-dasharray"} {
		delete(a, k)
	}
	shape := name == "path" || name == "rect" || name == "circle" || name == "ellipse" || name == "line" || name == "polyline" || name == "polygon"
	if name == "svg" {
		if _, ok := a["stroke-linejoin"]; !ok {
			a["stroke-linejoin"] = "miter"
		}
	}
	if !shape {
		return nil
	}
	a["fill-rule"] = s.fillRule
	m := s.matrix
	strokeScale := math.Hypot(m.A, m.B)
	if s.stroke != "none" {
		otherScale := math.Hypot(m.C, m.D)
		if math.Abs(strokeScale-otherScale) > 1e-9*max(1, strokeScale, otherScale) || math.Abs(m.A*m.C+m.B*m.D) > 1e-9*max(1, strokeScale*otherScale) {
			return fmt.Errorf("svg: non-uniform/sheared stroke transforms are unsupported")
		}
	}
	if m != rasterx.Identity && (strings.HasPrefix(s.fill, "url(") || strings.HasPrefix(s.stroke, "url(")) {
		return fmt.Errorf("svg: transformed gradients are unsupported")
	}
	a["transform"] = fmt.Sprintf("matrix(%g,%g,%g,%g,%g,%g)", m.A, m.B, m.C, m.D, m.E, m.F)
	a["fill-opacity"], a["stroke-opacity"] = num(s.fillOpacity), num(s.strokeOpacity)
	a["stroke-width"], a["stroke-dashoffset"] = num(s.strokeWidth*strokeScale), num(s.dashOffset*strokeScale)
	if s.dash != "" && s.dash != "none" {
		numbers, err := numberList(s.dash)
		if err != nil {
			return err
		}
		parts := make([]string, len(numbers))
		for j, n := range numbers {
			if n < 0 {
				return fmt.Errorf("svg: negative dash")
			}
			parts[j] = num(n * strokeScale)
		}
		a["stroke-dasharray"] = strings.Join(parts, ",")
	}
	return nil
}

func number(v string) (float64, error) {
	n, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil || !finite(n) {
		return 0, fmt.Errorf("svg: invalid number %q", v)
	}
	return n, nil
}
func num(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }

var svgNumber = regexp.MustCompile(`[+-]?(?:[0-9]+\.?[0-9]*|\.[0-9]+)(?:[eE][+-]?[0-9]+)?`)

func numberList(v string) ([]float64, error) {
	var result []float64
	for strings.TrimSpace(v) != "" {
		v = strings.TrimLeft(v, " \t\r\n,")
		loc := svgNumber.FindStringIndex(v)
		if loc == nil || loc[0] != 0 {
			return nil, fmt.Errorf("svg: invalid number list %q", v)
		}
		n, err := number(v[:loc[1]])
		if err != nil {
			return nil, err
		}
		result = append(result, n)
		v = v[loc[1]:]
	}
	return result, nil
}

func transform(v string) (rasterx.Matrix2D, error) {
	m := rasterx.Identity
	for strings.TrimSpace(v) != "" {
		v = strings.TrimLeft(v, " \t\r\n,")
		name, rest, ok := strings.Cut(v, "(")
		if !ok {
			return m, fmt.Errorf("svg: invalid transform")
		}
		args, rest, ok := strings.Cut(rest, ")")
		if !ok {
			return m, fmt.Errorf("svg: invalid transform")
		}
		n, err := numberList(args)
		if err != nil {
			return m, err
		}
		switch strings.TrimSpace(name) {
		case "translate":
			if len(n) == 1 {
				n = append(n, 0)
			}
			if len(n) != 2 {
				return m, fmt.Errorf("svg: invalid translate")
			}
			m = m.Translate(n[0], n[1])
		case "scale":
			if len(n) == 1 {
				n = append(n, n[0])
			}
			if len(n) != 2 {
				return m, fmt.Errorf("svg: invalid scale")
			}
			m = m.Scale(n[0], n[1])
		case "rotate":
			if len(n) == 1 {
				m = m.Rotate(n[0] * math.Pi / 180)
			} else if len(n) == 3 {
				m = m.Translate(n[1], n[2]).Rotate(n[0]*math.Pi/180).Translate(-n[1], -n[2])
			} else {
				return m, fmt.Errorf("svg: invalid rotate")
			}
		case "matrix":
			if len(n) != 6 {
				return m, fmt.Errorf("svg: invalid matrix")
			}
			m = m.Mult(rasterx.Matrix2D{A: n[0], B: n[1], C: n[2], D: n[3], E: n[4], F: n[5]})
		default:
			return m, fmt.Errorf("svg: unsupported transform %q", name)
		}
		v = rest
	}
	return m, nil
}
