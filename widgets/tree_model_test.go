package widgets

import (
	"slices"
	"testing"
)

func TestTreeStoreStructureBatchAndCopies(t *testing.T) {
	m := NewTreeStore[[]string]()
	notifications := 0
	m.ConnectItems(func() {
		notifications++
		if _, exists := m.Parent("folder"); !exists {
			t.Fatal("partial mutation notified")
		}
	})
	m.Modify(func(m *TreeStore[[]string]) {
		m.Append("", "folder", []string{"folder"})
		m.Modify(func(m *TreeStore[[]string]) {
			m.Append("folder", "a", []string{"a"})
			m.Insert("folder", 0, "b", []string{"b"})
		})
	})
	if notifications != 1 || !slices.Equal(m.Children("folder"), []string{"b", "a"}) {
		t.Fatal("ordered batch failed")
	}
	if parent, exists := m.Parent("folder"); parent != "" || !exists {
		t.Fatal("root contract")
	}
	if _, exists := m.Parent("missing"); exists {
		t.Fatal("missing node reported present")
	}
	children := m.Children("folder")
	children[0] = "mutated"
	if m.Children("folder")[0] != "b" {
		t.Fatal("children leaked storage")
	}
	m.Set("a", []string{"updated"})
	if m.Item("a")[0] != "updated" || notifications != 2 {
		t.Fatal("non-comparable data update failed")
	}
	m.SetExpandable("a", true)
	if !m.Expandable("a") {
		t.Fatal("empty expandable branch lost")
	}
	m.SetExpandable("folder", false)
	if !m.Expandable("folder") {
		t.Fatal("explicit false hid children")
	}
	m.Remove("b")
	m.Remove("missing")
	if !slices.Equal(m.Children("folder"), []string{"a"}) {
		t.Fatal("subtree removal failed")
	}
}
func TestTreeStoreInvalidMutationsAndRecursiveRemoval(t *testing.T) {
	for _, operation := range []func(*TreeStore[int]){
		func(m *TreeStore[int]) { m.Append("", "", 0) },
		func(m *TreeStore[int]) { m.Append("", "root", 0) },
		func(m *TreeStore[int]) { m.Append("missing", "new", 0) },
		func(m *TreeStore[int]) { m.Insert("root", 1, "new", 0) },
		func(m *TreeStore[int]) { m.Set("missing", 0) },
	} {
		m := NewTreeStore[int]()
		m.Append("", "root", 1)
		func() {
			defer func() {
				if recover() == nil {
					t.Error("invalid mutation did not panic")
				}
			}()
			operation(m)
		}()
	}
	m := NewTreeStore[int]()
	m.Append("", "root", 1)
	m.Append("root", "child", 2)
	m.Append("child", "grandchild", 3)
	m.Remove("root")
	if len(m.Children("")) != 0 || len(m.nodes) != 0 {
		t.Fatal("recursive removal retained descendants")
	}
}
