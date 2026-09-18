package svg

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"

	"github.com/golang-gui/oksvg"
)

// oksvg exposes a per-path winding field, but its parser ignores the
// fill-rule attribute. Compile each shape with its ancestor styles and shared
// definitions, then set that public field. This keeps geometry parsing in the
// dependency without guessing which source element produced each path (empty
// shapes can produce none), mutating global defaults, or using sentinel colors.
// Compilation happens only at load time, never on a style/size change.
func compile(data []byte) (*oksvg.SvgIcon, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var stack []xml.StartElement
	var root xml.StartElement
	var defs []xml.Token
	var shapes [][]xml.StartElement
	defsDepth := 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		token = xml.CopyToken(token)
		switch t := token.(type) {
		case xml.StartElement:
			stack = append(stack, t)
			parent := ""
			if len(stack) > 1 {
				parent = stack[len(stack)-2].Name.Local
			}
			switch t.Name.Local {
			case "linearGradient", "radialGradient":
				if parent != "defs" {
					return nil, fmt.Errorf("svg: gradients must be direct children of defs")
				}
			case "stop":
				if parent != "linearGradient" && parent != "radialGradient" {
					return nil, fmt.Errorf("svg: stop must be inside a gradient")
				}
			case "g":
				if parent != "svg" && parent != "g" {
					return nil, fmt.Errorf("svg: groups must be inside svg/g")
				}
			}
			if len(stack) == 1 {
				root = t
			}
			if t.Name.Local == "defs" {
				if len(stack) != 2 {
					return nil, fmt.Errorf("svg: defs must be direct children of svg")
				}
				defsDepth = len(stack)
			}
			shape := false
			switch t.Name.Local {
			case "path", "rect", "circle", "ellipse", "line", "polyline", "polygon":
				shape = true
			}
			if shape {
				if defsDepth != 0 {
					return nil, fmt.Errorf("svg: only gradients may be defined in defs; use is unsupported")
				}
				for _, parent := range stack[1 : len(stack)-1] {
					if parent.Name.Local != "g" {
						return nil, fmt.Errorf("svg: shape must be inside svg/g")
					}
				}
				shapes = append(shapes, append([]xml.StartElement(nil), stack[1:]...))
			}
			if defsDepth != 0 {
				defs = append(defs, token)
			}
		case xml.EndElement:
			if defsDepth != 0 {
				defs = append(defs, token)
			}
			if len(stack) == defsDepth {
				defsDepth = 0
			}
			stack = stack[:len(stack)-1]
		default:
			if defsDepth != 0 {
				defs = append(defs, token)
			}
		}
	}
	base, err := compileShape(root, defs, nil)
	if err != nil {
		return nil, err
	}
	for _, ancestors := range shapes {
		part, err := compileShape(root, defs, ancestors)
		if err != nil {
			return nil, err
		}
		nonzero := true
		for _, a := range ancestors[len(ancestors)-1].Attr {
			if a.Name.Local == "fill-rule" {
				nonzero = a.Value != "evenodd"
			}
		}
		for _, p := range part.SVGPaths {
			p.UseNonZeroWinding = nonzero
			base.SVGPaths = append(base.SVGPaths, p)
		}
	}
	return base, nil
}

func compileShape(root xml.StartElement, defs []xml.Token, ancestors []xml.StartElement) (*oksvg.SvgIcon, error) {
	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	if err := enc.EncodeToken(root); err != nil {
		return nil, err
	}
	for _, token := range defs {
		if err := enc.EncodeToken(token); err != nil {
			return nil, err
		}
	}
	for _, start := range ancestors {
		if err := enc.EncodeToken(start); err != nil {
			return nil, err
		}
	}
	for j := len(ancestors) - 1; j >= 0; j-- {
		if err := enc.EncodeToken(ancestors[j].End()); err != nil {
			return nil, err
		}
	}
	if err := enc.EncodeToken(root.End()); err != nil {
		return nil, err
	}
	if err := enc.Flush(); err != nil {
		return nil, err
	}
	return oksvg.ReadIconStream(&buf, oksvg.StrictErrorMode)
}
