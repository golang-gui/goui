package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"unsafe"

	"github.com/goexlib/cgo"
)

// 独立参考只在维护者阶段运行，既不链接也不探测 Android 系统私有库。
func (b *builder) shapingReference() error {
	fontBytes, err := os.ReadFile(b.config.Font)
	if err != nil {
		return err
	}
	names := []string{"libharfbuzz.so.0"}
	if runtime.GOOS == "darwin" {
		names = []string{"libharfbuzz.0.dylib", "/opt/homebrew/lib/libharfbuzz.0.dylib"}
	}
	library := cgo.NewLazyLibrary(names[0], names[1:]...)
	if err = library.Load(); err != nil {
		return fmt.Errorf("native shaping reference: %w", err)
	}
	symbol := func(name string) uintptr { return library.NewSymbol(name).Addr() }
	blob := cgo.CallRet[uintptr](symbol("hb_blob_create"), cgo.CSlice(fontBytes), uint32(len(fontBytes)), int32(0), uintptr(0), uintptr(0))
	if blob == 0 {
		return fmt.Errorf("create HarfBuzz blob")
	}
	defer cgo.Call(symbol("hb_blob_destroy"), blob)
	face := cgo.CallRet[uintptr](symbol("hb_face_create"), blob, uint32(0))
	defer cgo.Call(symbol("hb_face_destroy"), face)
	font := cgo.CallRet[uintptr](symbol("hb_font_create"), face)
	defer cgo.Call(symbol("hb_font_destroy"), font)
	cgo.Call(symbol("hb_ot_font_set_funcs"), font)
	type glyph struct {
		Glyph   uint32 `json:"glyph"`
		Cluster uint32 `json:"cluster"`
	}
	type shape struct {
		Text       string  `json:"text"`
		RTL        bool    `json:"rtl"`
		Glyphs     []glyph `json:"glyphs"`
		Boundaries []int   `json:"boundaries"`
	}
	type info struct{ Glyph, Mask, Cluster, Var1, Var2 uint32 }
	var cases []shape
	for _, text := range []string{"ffi", "office", "a\u0301", "لا", "سلام", "אבג", "a\u200bB", "\u200b", "a\u200b\u200bB"} {
		buffer := cgo.CallRet[uintptr](symbol("hb_buffer_create"))
		if buffer == 0 {
			return fmt.Errorf("create HarfBuzz buffer")
		}
		encoded := []byte(text)
		cgo.Call(symbol("hb_buffer_add_utf8"), buffer, cgo.CSlice(encoded), int32(len(encoded)), uint32(0), int32(len(encoded)))
		cgo.Call(symbol("hb_buffer_guess_segment_properties"), buffer)
		cgo.Call(symbol("hb_shape"), font, buffer, uintptr(0), uint32(0))
		var count uint32
		var pin runtime.Pinner
		pin.Pin(&count)
		address := cgo.CallRet[uintptr](symbol("hb_buffer_get_glyph_infos"), buffer, &count)
		pin.Unpin()
		current := shape{Text: text, RTL: cgo.CallRet[int32](symbol("hb_buffer_get_direction"), buffer) == 5}
		boundaries := map[int]bool{len(encoded): true}
		for _, g := range unsafe.Slice((*info)(cgo.Pointer(address)), count) {
			if g.Glyph == 0 {
				cgo.Call(symbol("hb_buffer_destroy"), buffer)
				return fmt.Errorf("reference font is missing glyphs for %q", text)
			}
			current.Glyphs = append(current.Glyphs, glyph{g.Glyph, g.Cluster})
			boundaries[int(g.Cluster)] = true
		}
		for n := range boundaries {
			current.Boundaries = append(current.Boundaries, n)
		}
		sort.Ints(current.Boundaries)
		cases = append(cases, current)
		cgo.Call(symbol("hb_buffer_destroy"), buffer)
	}
	if len(cases[0].Glyphs) != 1 {
		return fmt.Errorf("reference font must contain default ffi ligature")
	}
	var presentation uint32
	var pin runtime.Pinner
	pin.Pin(&presentation)
	ok := cgo.CallRet[int32](symbol("hb_font_get_nominal_glyph"), font, uint32(0xfb03), &presentation)
	pin.Unpin()
	if ok == 0 || presentation != cases[0].Glyphs[0].Glyph {
		return fmt.Errorf("reference U+FB03 must map to the same ffi glyph")
	}
	assets := filepath.Join(b.root, "assets")
	if err = os.MkdirAll(assets, 0755); err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(assets, "shape.ttf"), fontBytes, 0644); err != nil {
		return err
	}
	return writeJSON(filepath.Join(assets, "shape.json"), map[string]any{"engine": "HarfBuzz", "version": cgo.GoString(cgo.Pointer(cgo.CallRet[uintptr](symbol("hb_version_string")))), "fontSha256": fmt.Sprintf("%x", sha256.Sum256(fontBytes)), "fontFile": filepath.Base(b.config.Font), "ffiPresentationGlyph": presentation, "cases": cases})
}
