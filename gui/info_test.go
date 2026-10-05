package gui_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
)

func TestWidgetInfoSetAttribute(t *testing.T) {
	var info gui.WidgetInfo
	info.SetAttribute("test.absent", nil)
	if info.Attributes != nil {
		t.Fatal("deletion allocated an empty map")
	}
	info.ID, info.Role, info.Selected = "widget-id", gui.RoleButton, true
	info.Bounds = geometry.Rect(1, 2, 30, 40)
	info.SetAttribute("test.first", map[string]int{"old": 1})
	info.SetAttribute("test.second", false)
	info.SetAttribute("test.first", map[string]int{"new": 2})
	if got := info.Attributes["test.first"].(map[string]int); len(got) != 1 || got["new"] != 2 {
		t.Fatalf("replacement merged old fields: %v", got)
	}
	if value, exists := info.Attributes["test.second"]; !exists || value != false {
		t.Fatal("false was treated as absence")
	}
	info.SetAttribute("test.first", nil)
	if _, exists := info.Attributes["test.first"]; exists || len(info.Attributes) != 1 {
		t.Fatal("deletion removed the wrong attribute")
	}
	info.SetAttribute("id", "not-the-widget-id")
	if info.ID != "widget-id" || info.Role != gui.RoleButton || !info.Selected || info.Bounds != geometry.Rect(1, 2, 30, 40) {
		t.Fatal("attribute changed common semantic fields")
	}
	info.SetAttribute("Test.second", 3)
	if len(info.Attributes) != 3 {
		t.Fatal("attribute names were normalized")
	}
	values := []int{1}
	info.SetAttribute("test.borrowed", values)
	values[0] = 2
	if info.Attributes["test.borrowed"].([]int)[0] != 2 {
		t.Fatal("helper must store values as provided, not silently deep-copy them")
	}
	t.Run("empty name", func(t *testing.T) {
		defer func() {
			if got := recover(); got != "gui: empty snapshot attribute name" {
				t.Fatalf("unexpected panic: %v", got)
			}
		}()
		info.SetAttribute("", 1)
	})
}

func TestWidgetInfoAttributesJSON(t *testing.T) {
	for _, info := range []gui.WidgetInfo{{}, {Attributes: map[string]any{}}} {
		data, err := json.Marshal(info)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(data, &fields); err != nil {
			t.Fatal(err)
		}
		if _, exists := fields["attributes"]; exists {
			t.Fatal("empty attributes were not omitted")
		}
	}
	info := gui.WidgetInfo{ID: "widget-id", Role: gui.RoleWidget}
	info.SetAttribute("test.values", snapshotAttributeData{Values: []int{1, 2}})
	var typedNil *snapshotAttributeData
	info.SetAttribute("test.nil", typedNil)
	if _, exists := info.Attributes["test.nil"]; !exists {
		t.Fatal("typed nil was treated as deletion")
	}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		ID         string                     `json:"id"`
		Attributes map[string]json.RawMessage `json:"attributes"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	var values snapshotAttributeData
	if err := json.Unmarshal(decoded.Attributes["test.values"], &values); err != nil {
		t.Fatal(err)
	}
	if decoded.ID != "widget-id" || len(decoded.Attributes) != 2 || !slices.Equal(values.Values, []int{1, 2}) || string(decoded.Attributes["test.nil"]) != "null" {
		t.Fatalf("unknown attributes lost their data: %s", data)
	}
}

type snapshotAttributeData struct {
	Values []int `json:"values"`
}

type snapshotAttributeWidget struct {
	gui.WidgetBase
	values []int
}

func (w *snapshotAttributeWidget) Snapshot() gui.WidgetInfo {
	info := w.WidgetBase.Snapshot()
	info.SetAttribute("test.values", snapshotAttributeData{Values: slices.Clone(w.values)})
	return info
}

// 外部控件无需注册；生产者复制内部可变数据，helper 不承担隐式深拷贝。
func TestWidgetInfoAttributeProducerIsolation(t *testing.T) {
	w := &snapshotAttributeWidget{values: []int{1, 2}}
	first := w.Snapshot()
	w.values[0] = 3
	second := w.Snapshot()
	second.SetAttribute("test.values", snapshotAttributeData{Values: []int{4}})
	if got := first.Attributes["test.values"].(snapshotAttributeData).Values; !slices.Equal(got, []int{1, 2}) {
		t.Fatalf("old snapshot changed with widget or later map: %v", got)
	}
	if got := w.Snapshot().Attributes["test.values"].(snapshotAttributeData).Values; !slices.Equal(got, []int{3, 2}) {
		t.Fatalf("consumer changed live widget state: %v", got)
	}
}
