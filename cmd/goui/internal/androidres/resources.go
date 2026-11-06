// Package androidres encodes the small Android resource set used by GOUI apps.
// It does not compile Java or implement a general Android resource compiler.
// The binary layouts follow AOSP androidfw/ResourceTypes.h.
package androidres

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
)

const androidNS = "http://schemas.android.com/apk/res/android"
const noIndex = ^uint32(0)
const iconID = 0x7f010000

// Only public framework attributes used by the generated manifest are needed.
// IDs are stable public Android resource IDs, not Java class definitions.
var attributes = []struct {
	name string
	id   uint32
	kind byte
}{
	{"theme", 0x01010000, 1}, {"label", 0x01010001, 3},
	{"icon", 0x01010002, 1}, {"name", 0x01010003, 3},
	{"hasCode", 0x0101000c, 18}, {"debuggable", 0x0101000f, 18},
	{"exported", 0x01010010, 18}, {"value", 0x01010024, 3},
	{"minSdkVersion", 0x0101020c, 16}, {"versionCode", 0x0101021b, 16},
	{"versionName", 0x0101021c, 3}, {"targetSdkVersion", 0x01010270, 16},
	{"allowBackup", 0x01010280, 18}, {"extractNativeLibs", 0x010104ea, 18},
}

type pool struct {
	values []string
	index  map[string]uint32
}

func (p *pool) add(s string) uint32 {
	if p.index == nil {
		p.index = make(map[string]uint32)
	}
	if n, ok := p.index[s]; ok {
		return n
	}
	n := uint32(len(p.values))
	p.values = append(p.values, s)
	p.index[s] = n
	return n
}

func put16(b []byte, offset int, value uint16) { binary.LittleEndian.PutUint16(b[offset:], value) }
func put32(b []byte, offset int, value uint32) { binary.LittleEndian.PutUint32(b[offset:], value) }

func chunk(kind, header uint16, size int) []byte {
	b := make([]byte, size)
	put16(b, 0, kind)
	put16(b, 2, header)
	put32(b, 4, uint32(size))
	return b
}

func (p *pool) encode() []byte {
	// UTF-16 pools avoid separate byte-count and UTF-16-length limits for UTF-8.
	start := 28 + 4*len(p.values)
	b := chunk(1, 28, start)
	put32(b, 8, uint32(len(p.values)))
	put32(b, 20, uint32(start))
	for i, s := range p.values {
		put32(b, 28+4*i, uint32(len(b)-start))
		u := utf16.Encode([]rune(s))
		if len(u) >= 0x8000 {
			b = binary.LittleEndian.AppendUint16(b, uint16(len(u)>>16)|0x8000)
		}
		b = binary.LittleEndian.AppendUint16(b, uint16(len(u)))
		for _, c := range u {
			b = binary.LittleEndian.AppendUint16(b, c)
		}
		b = append(b, 0, 0)
	}
	for len(b)%4 != 0 {
		b = append(b, 0)
	}
	put32(b, 4, uint32(len(b)))
	return b
}

func node(kind uint16, size int) []byte {
	b := chunk(kind, 16, size)
	put32(b, 8, 1)
	put32(b, 12, noIndex)
	return b
}

// Manifest converts the framework-generated XML into Android binary XML.
// Unsupported attributes and resource references fail instead of guessing IDs.
func Manifest(source []byte) ([]byte, error) {
	var p pool
	for _, a := range attributes {
		p.add(a.name)
	}
	prefix, ns := p.add("android"), p.add(androidNS)
	start := node(0x100, 24)
	put32(start, 16, prefix)
	put32(start, 20, ns)
	var nodes bytes.Buffer
	nodes.Write(start)
	d := xml.NewDecoder(bytes.NewReader(source))
	depth, roots := 0, 0
	for {
		token, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if depth == 0 {
				roots++
				if roots != 1 || t.Name.Local != "manifest" {
					return nil, fmt.Errorf("expected one manifest root")
				}
			}
			depth++
			var attrs [][]byte
			for _, a := range t.Attr {
				if a.Name.Space == "xmlns" || a.Name.Local == "xmlns" {
					continue
				}
				value := make([]byte, 20)
				put32(value, 0, noIndex)
				put32(value, 4, p.add(a.Name.Local))
				put32(value, 8, noIndex)
				put16(value, 12, 8)
				kind := byte(3)
				if a.Name.Space != "" {
					if a.Name.Space != androidNS {
						return nil, fmt.Errorf("unsupported manifest namespace %q", a.Name.Space)
					}
					put32(value, 0, ns)
					found := false
					for _, defined := range attributes {
						if defined.name == a.Name.Local {
							kind, found = defined.kind, true
							break
						}
					}
					if !found {
						return nil, fmt.Errorf("unsupported Android attribute %q", a.Name.Local)
					}
				}
				var data uint32
				switch kind {
				case 3:
					data = p.add(a.Value)
					put32(value, 8, data)
				case 16:
					n, err := strconv.ParseUint(a.Value, 10, 32)
					if err != nil {
						return nil, fmt.Errorf("manifest %s: %w", a.Name.Local, err)
					}
					data = uint32(n)
				case 18:
					if a.Value != "true" && a.Value != "false" {
						return nil, fmt.Errorf("manifest %s: invalid boolean", a.Name.Local)
					}
					if a.Value == "true" {
						data = noIndex
					}
				case 1:
					switch a.Value {
					case "@mipmap/app_icon":
						data = iconID
					case "@android:style/Theme.Material.Light.NoActionBar":
						data = 0x01030241
					default:
						return nil, fmt.Errorf("unsupported manifest reference %q", a.Value)
					}
				}
				value[15] = kind
				put32(value, 16, data)
				attrs = append(attrs, value)
			}
			// Resource-backed attributes precede unnamespaced attributes, in ID
			// order. The reserved leading pool entries have that same order.
			sort.SliceStable(attrs, func(i, j int) bool {
				a, b := binary.LittleEndian.Uint32(attrs[i]), binary.LittleEndian.Uint32(attrs[j])
				if a != b {
					return a != noIndex
				}
				return binary.LittleEndian.Uint32(attrs[i][4:]) < binary.LittleEndian.Uint32(attrs[j][4:])
			})
			b := node(0x102, 36+20*len(attrs))
			put32(b, 16, noIndex)
			put32(b, 20, p.add(t.Name.Local))
			put16(b, 24, 20)
			put16(b, 26, 20)
			put16(b, 28, uint16(len(attrs)))
			for i, a := range attrs {
				copy(b[36+i*20:], a)
			}
			nodes.Write(b)
		case xml.EndElement:
			depth--
			b := node(0x103, 24)
			put32(b, 16, noIndex)
			put32(b, 20, p.add(t.Name.Local))
			nodes.Write(b)
		case xml.CharData:
			if strings.TrimSpace(string(t)) != "" {
				return nil, fmt.Errorf("manifest text nodes are unsupported")
			}
		}
	}
	if roots != 1 || depth != 0 {
		return nil, fmt.Errorf("incomplete manifest document")
	}
	end := node(0x101, 24)
	put32(end, 16, prefix)
	put32(end, 20, ns)
	nodes.Write(end)
	resourceMap := chunk(0x180, 8, 8+len(attributes)*4)
	for i, a := range attributes {
		put32(resourceMap, 8+4*i, a.id)
	}
	b := chunk(3, 8, 8)
	b = append(b, p.encode()...)
	b = append(b, resourceMap...)
	b = append(b, nodes.Bytes()...)
	put32(b, 4, uint32(len(b)))
	return b, nil
}

// Icon identifies a density-qualified PNG and its resource-table entry.
type Icon struct {
	Density string
	DPI     uint16
	Size    int
}

var Icons = [...]Icon{{"mdpi", 160, 48}, {"hdpi", 240, 72}, {"xhdpi", 320, 96}, {"xxhdpi", 480, 144}, {"xxxhdpi", 640, 192}}

func (i Icon) Path() string { return "res/mipmap-" + i.Density + "/app_icon.png" }

// Resources builds one mipmap resource with the five supported icon densities.
func Resources(packageName string) ([]byte, error) {
	name := utf16.Encode([]rune(packageName))
	if len(name) == 0 || len(name) > 127 {
		return nil, fmt.Errorf("Android resource package name must fit 127 UTF-16 units")
	}
	var paths, types, keys pool
	for _, i := range Icons {
		paths.add(i.Path())
	}
	types.add("mipmap")
	keys.add("app_icon")
	pkg := chunk(0x200, 288, 288)
	put32(pkg, 8, 0x7f)
	for i, c := range name {
		put16(pkg, 12+2*i, c)
	}
	put32(pkg, 268, uint32(len(pkg)))
	put32(pkg, 272, 1)
	pkg = append(pkg, types.encode()...)
	put32(pkg, 276, uint32(len(pkg)))
	put32(pkg, 280, 1)
	pkg = append(pkg, keys.encode()...)
	spec := chunk(0x202, 16, 20)
	spec[8] = 1
	put32(spec, 12, 1)
	put32(spec, 16, 0x100) // CONFIG_DENSITY
	pkg = append(pkg, spec...)
	for index, i := range Icons {
		b := chunk(0x201, 84, 104)
		b[8] = 1
		put32(b, 12, 1)
		put32(b, 16, 88)
		put32(b, 20, 64)
		put16(b, 34, i.DPI)
		put16(b, 44, 4) // The density resource qualifier starts at API 4.
		put16(b, 88, 8)
		put16(b, 96, 8)
		b[99] = 3
		put32(b, 100, uint32(index))
		pkg = append(pkg, b...)
	}
	put32(pkg, 4, uint32(len(pkg)))
	b := chunk(2, 12, 12)
	put32(b, 8, 1)
	b = append(b, paths.encode()...)
	b = append(b, pkg...)
	put32(b, 4, uint32(len(b)))
	return b, nil
}
