// Package typographytest contains shared native typography test fixtures.
package typographytest

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang-gui/goui/platform/typography"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goitalic"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/sfnt"
)

var fixtureDirectory string

// Main retains registered files until every test/layout is finished. CoreText
// registration is process-wide: a per-test TempDir can disappear while a later
// test still resolves that registered PostScript name.
func Main(m *testing.M) int {
	dir, err := os.MkdirTemp("", "goui-typography-fonts-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fixtureDirectory = dir
	defer os.RemoveAll(dir)
	return m.Run()
}

// FontFile writes the Go font (Go project BSD license, distributed by x/image)
// outside the source tree. Its family is "Go", not a generic fallback alias.
func FontFile(t *testing.T) string {
	t.Helper()
	if fixtureDirectory == "" {
		t.Fatal("typographytest.Main must own the font fixture lifetime")
	}
	path := filepath.Join(fixtureDirectory, "Go-Regular.ttf")
	if _, err := os.Stat(path); err == nil {
		return path
	}
	if err := os.WriteFile(path, goregular.TTF, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func InvalidFontFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "invalid.ttf")
	if err := os.WriteFile(path, []byte("not an OpenType font"), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

// FontCollectionFile packs the licensed fixtures into a TTC whose table offsets
// are relative to the entire file, not each face.
func FontCollectionFile(t *testing.T) string {
	t.Helper()
	if fixtureDirectory == "" {
		t.Fatal("typographytest.Main must own the font fixture lifetime")
	}
	faces := [][]byte{goregular.TTF, gobold.TTF, goitalic.TTF}
	data := make([]byte, 12+4*len(faces))
	copy(data, "ttcf")
	binary.BigEndian.PutUint32(data[4:8], 0x00010000)
	binary.BigEndian.PutUint32(data[8:12], uint32(len(faces)))
	for i, face := range faces {
		for len(data)%4 != 0 {
			data = append(data, 0)
		}
		base := len(data)
		binary.BigEndian.PutUint32(data[12+i*4:16+i*4], uint32(base))
		data = append(data, face...)
		for table := 0; table < int(binary.BigEndian.Uint16(face[4:6])); table++ {
			at := base + 12 + table*16 + 8
			offset := binary.BigEndian.Uint32(data[at : at+4])
			binary.BigEndian.PutUint32(data[at:at+4], offset+uint32(base))
		}
	}
	collection, err := sfnt.ParseCollection(data)
	if err != nil || collection.NumFonts() != 3 {
		t.Fatalf("invalid TTC fixture: %v", err)
	}
	path := filepath.Join(fixtureDirectory, "Go.ttc")
	if _, err := os.Stat(path); err == nil {
		return path
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

// RegisterFonts supplies actual Regular/Bold/Italic faces from one collection.
func RegisterFonts(t *testing.T, ctx typography.Context) {
	t.Helper()
	if err := ctx.AddFont(FontCollectionFile(t)); err != nil {
		t.Fatal(err)
	}
}
