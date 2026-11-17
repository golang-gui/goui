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
		"shared_popover.go": {
			{"\treturn nil\n}\n\nfunc (s *sharedPopoverSlot) closeClient", `
            fmt.Printf("TRACE LEASE root=%p native=%p handle=%x painter=%p style=%s\n", s.popup, s.popup.platformPopup, s.popup.platformPopup.NativeHandle(), s.popup.painter, p.StyleName())
            return nil
}

func (s *sharedPopoverSlot) closeClient`},
		},
		"popover.go": {
			{`"math"`, `"math"
            "os"
            "strconv"
            "time"`},
			{`// Commit visibility and modal ownership before allowing frame callbacks.`, `if delay,err:=strconv.Atoi(os.Getenv("GOUI_VERIFY_SLOW_MS"));err==nil && delay>0 {
                fmt.Printf("TRACE SLOW root=%p delay=%dms\n", p,delay)
                time.Sleep(time.Duration(delay)*time.Millisecond)
            }
            // Commit visibility and modal ownership before allowing frame callbacks.`},
			{`p.platformPopup = pp`, `kind := "independent"
            if owner := surfaceCore(p.owner); owner != nil && owner.shared != nil && owner.shared.popup == p { kind = "shared" }
            fmt.Printf("TRACE CREATE %s root=%p native=%p handle=%x painter=%p backend=%s\n", kind, p, pp, pp.NativeHandle(), painter, painter.Name())
            p.platformPopup = pp`},
			{`p.painter.Destroy()`, `fmt.Printf("TRACE DESTROY root=%p native=%p painter=%p\n", p, p.platformPopup, p.painter)
            p.painter.Destroy()`},
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
