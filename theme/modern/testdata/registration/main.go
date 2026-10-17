// Headless integration fixture: imports GUI before init registrations, then
// verifies lazy Basic, Basic-only controls in Modern, Modern overrides and snapshots.
package main

import (
	"fmt"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/basic"
	"github.com/golang-gui/goui/theme/modern"
)

func init() {
	basic.Register("example/basic-only", func(base style.StyleSheet) []style.Rule {
		return []style.Rule{style.Name("fixture.basic").FontSize(17)}
	})
	basic.Register("example/modern", func(base style.StyleSheet) []style.Rule {
		return []style.Rule{style.Name("fixture.modern").FontSize(18)}
	})
	modern.Register("example/modern", func(o modern.Options, base style.StyleSheet) []style.Rule {
		return []style.Rule{style.Name("fixture.modern").FontSize(o.FontSize)}
	})
}
func main() {
	def := gui.DefaultStyleSheet()
	n, _ := def.Resolve(style.Sel{Name: "fixture.basic"}).FontSize()
	if n != 17 {
		panic("eager Basic cache")
	}
	for _, o := range []modern.Options{{FontSize: 11}, {Dark: true, FontSize: 12}} {
		s := modern.Sheet(o)
		n, _ = s.Resolve(style.Sel{Name: "fixture.basic"}).FontSize()
		if n != 17 {
			panic("Basic-only control lost")
		}
		n, _ = s.Resolve(style.Sel{Name: "fixture.modern"}).FontSize()
		if n != o.FontSize {
			panic("Modern registration not applied")
		}
	}
	n, _ = def.Resolve(style.Sel{Name: "fixture.modern"}).FontSize()
	if n != 18 {
		panic("Basic snapshot changed")
	}
	fmt.Println("PASS registration startup, Basic/Modern composition and snapshots")
}
