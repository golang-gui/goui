package basic

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
	var r basicRegistry
	provider := func(base style.StyleSheet) []style.Rule { return nil }
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

func TestRegisteredRulesOwnershipAndBaseIsolation(t *testing.T) {
	var r basicRegistry
	shared := []style.Rule{style.Name("test.third").FontSize(7)}
	r.add("a", func(base style.StyleSheet) []style.Rule { return shared })
	r.add("b", func(base style.StyleSheet) []style.Rule {
		if _, ok := base.Resolve(style.Sel{Name: "test.third"}).FontSize(); ok {
			t.Fatal("provider saw preceding extension")
		}
		family, _ := base.Resolve(style.Sel{Name: "label"}).FontFamily()
		return []style.Rule{style.Name("test.other").FontFamily(family)}
	})
	entries := r.snapshot()
	a := registeredRules(entries)
	b := registeredRules(entries)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("nondeterministic result")
	}
	for i := range a {
		a[i] = style.Rule{}
	}
	if got, _ := style.Sheet(b...).Resolve(style.Sel{Name: "test.third"}).FontSize(); got != 7 {
		t.Fatal("shared returned rules")
	}
	if !reflect.DeepEqual(shared, []style.Rule{style.Name("test.third").FontSize(7)}) {
		t.Fatal("mutated provider data")
	}
}
