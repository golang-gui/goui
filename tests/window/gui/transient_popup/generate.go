//go:build ignore

// 仅为桌面验收生成带资源身份日志的 Go overlay，不导出生产私有状态。
package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang-gui/goui/internal/nativegen"
)

func main() {
	cwd, err := os.Getwd()
	must(err)
	repo := filepath.Clean(filepath.Join(cwd, "../../../.."))
	out := os.Getenv("GOUI_GENERATE_DIR")
	if !filepath.IsAbs(out) {
		log.Fatal("GOUI_GENERATE_DIR 必须为仓库外的绝对路径")
	}
	must(nativegen.CheckOutput(out, repo))
	component := "tests/window/gui/transient_popup"
	sources := make(map[string]string)
	replace := make(map[string]string)
	for file, patches := range map[string][][2]string{
		"transient_popup.go": {
			{`h.native, h.painter, h.transparent = native, painter, transparent`, `h.native, h.painter, h.transparent = native, painter, transparent
                fmt.Printf("TRACE CREATE shared native=%p handle=%x painter=%p backend=%s\n", native, native.NativeHandle(), painter, painter.Name())`},
			{`p.platformPopup, p.surface, p.painter = h.native, h.native, h.painter`, `p.platformPopup, p.surface, p.painter = h.native, h.native, h.painter
            fmt.Printf("TRACE LEASE native=%p handle=%x painter=%p style=%s\n", h.native, h.native.NativeHandle(), h.painter, p.StyleName())`},
			{"h.closed = true", `h.closed = true
            fmt.Printf("TRACE DESTROY native=%p painter=%p\n", h.native, h.painter)`},
		},
		"popover.go": {
			{`p.platformPopup = pp`, `fmt.Printf("TRACE CREATE independent native=%p handle=%x painter=%p backend=%s\n", pp, pp.NativeHandle(), painter, painter.Name())
            p.platformPopup = pp`},
		},
	} {
		data, err := os.ReadFile(filepath.Join(repo, "gui", file))
		must(err)
		text := string(data)
		for _, patch := range patches {
			if strings.Count(text, patch[0]) != 1 {
				log.Fatalf("%s 注入点不唯一：%s", file, patch[0])
			}
			text = strings.Replace(text, patch[0], patch[1], 1)
		}
		sources[file] = text
		replace[filepath.Join(repo, "gui", file)] = filepath.Join(out, component, file)
	}
	overlay, err := json.MarshalIndent(map[string]any{"Replace": replace}, "", "  ")
	must(err)
	sources["overlay.json"] = string(overlay)
	nativegen.Main(component, sources)
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
