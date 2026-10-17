package modern

import (
	"reflect"
	"testing"

	"github.com/golang-gui/goui/style"
)

func mustPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Error("expected registration panic")
		}
	}()
	fn()
}
func TestRegistrationContract(t *testing.T) {
	var r modernRegistry
	provider := func(options Options, base style.StyleSheet) []style.Rule { return nil }
	mustPanic(t, func() { r.add("", provider) })
	mustPanic(t, func() { r.add("nil", nil) })
	r.add("z", provider)
	r.add("a", provider)
	mustPanic(t, func() { r.add("a", provider) })
	entries := r.snapshot()
	if entries[0].name != "a" || entries[1].name != "z" {
		t.Fatal("unstable order")
	}
	entries[0].name = "changed"
	if r.snapshot()[0].name != "a" {
		t.Fatal("mutable registration snapshot")
	}
	mustPanic(t, func() { r.add("late", provider) })
}

func TestRegisteredOptionsAndSnapshotIsolation(t *testing.T) {
	var r modernRegistry
	var dark bool
	r.add("a", func(options Options, base style.StyleSheet) []style.Rule {
		if options.FontSize != 14 || options.AccentColor == nil {
			t.Fatal("unnormalized options")
		}
		dark = options.Dark
		return []style.Rule{{Sel: style.Sel{Name: "test.third"}, Style: base.Resolve(style.Sel{Name: "label"})}}
	})
	entries := r.snapshot()
	build := func(d bool) []style.Rule {
		o := normalizedOptions(Options{Dark: d, FontSize: -1})
		own := builtInRules(o)
		return registeredRules(o, style.Sheet(own...), own, entries)
	}
	light := build(false)
	old := style.Sheet(light...)
	darkRules := build(true)
	if !dark || reflect.DeepEqual(light, darkRules) {
		t.Fatal("options not passed to extension")
	}
	if !reflect.DeepEqual(light, build(false)) {
		t.Fatal("nondeterministic sheet")
	}
	before, _ := old.Resolve(style.Sel{Name: "test.third"}).ForegroundColor()
	after, _ := style.Sheet(darkRules...).Resolve(style.Sel{Name: "test.third"}).ForegroundColor()
	if reflect.DeepEqual(before, after) {
		t.Fatal("wrong palette snapshot")
	}
	if _, ok := old.Resolve(style.Sel{Name: "test.third"}).FontSize(); !ok {
		t.Fatal("old sheet changed")
	}
}
