//go:build linux || windows || darwin

package typography_test

import (
	"fmt"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui/textedit"
	"github.com/golang-gui/goui/platform/typography"
	"runtime"
	"strings"
	"testing"
)

// 原生字体度量与 textedit 跨层集成：共享边界的浮点舍入不能产生重复光标停靠点，
// 普通 ASCII 行每次向左必须恰好前进一个字符；无需桌面，不与上层包内单元测试混放。
func TestNativeCharacterNavigation(t *testing.T) {
	for _, family := range nativeFamilies {
		for _, width := range []float32{173, 1090, 1 << 24} {
			t.Run(fmt.Sprintf("%s/%v", family, width), func(t *testing.T) {
				runtime.LockOSThread()
				defer runtime.UnlockOSThread()
				ctx, err := newContext()
				if err != nil {
					t.Fatal(err)
				}
				defer ctx.Destroy()
				text := strings.Repeat("a9+Z/Bh4q2AZbng7 latencyword ", 1600)
				l, err := ctx.NewTextLayout(text, typography.TextFormat{Font: typography.FontInfo{Family: family, Size: 12}, WrapMode: typography.WrapWordChar}, width, 1<<24)
				if err != nil {
					t.Fatal(err)
				}
				defer l.Destroy()
				w, h := l.MeasureSize()
				lines, cs := l.MeasureMetrics()
				f, err := textedit.NewFragment(textedit.Range{End: len(text)}, geometry.Size{Width: w, Height: h}, lines, cs)
				if err != nil {
					t.Fatal(err)
				}
				g := f.Geometry(20)
				for i, line := range lines {
					if line.Length < 14 {
						continue
					}
					for _, at := range []int{12, max(12, line.Length/2), line.Length - 1} {
						pos := textedit.Position{Offset: line.Start + at}
						for step := 0; step < 11; step++ {
							next, ok := g.Visual(pos, false)
							if !ok || next.Offset != pos.Offset-1 {
								t.Fatalf("line %d step %d: %+v -> %+v/%v", i, step, pos, next, ok)
							}
							pos = next
						}
					}
				}
			})
		}
	}
}
