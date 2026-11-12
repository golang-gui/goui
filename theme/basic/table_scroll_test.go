package basic

import (
	"image/color"
	"reflect"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/style"
)

func TestTableScrollParts(t *testing.T) {
	sheet := Sheet()
	for _, state := range []style.State{style.Normal, style.Hovered, style.Pressed, style.Focused, style.Disabled} {
		frame := sheet.Resolve(style.Sel{Name: "table-view", Part: "frame", State: state})
		bg, _ := frame.BackgroundColor()
		width, _ := frame.BorderWidth()
		radius, _ := frame.CornerRadius()
		if bg == nil || color.RGBAModel.Convert(bg).(color.RGBA).A != 0 || width != 0 || radius != (geometry.CornerRadius{}) {
			t.Fatal("内部ScrollView继承了表格外框或不透明底色")
		}
		track := sheet.Resolve(style.Sel{Name: "table-view", Part: "trough", State: state})
		bg, _ = track.BackgroundColor()
		header, _ := sheet.Resolve(style.Sel{Name: "table-view", Part: "header"}).BackgroundColor()
		radius, _ = track.CornerRadius()
		width, _ = track.BorderWidth()
		if !reflect.DeepEqual(bg, header) || radius != (geometry.CornerRadius{}) || width != 0 {
			t.Fatal("表格轨道未与表头使用同色方形底，或保留了第二层边框")
		}
		thumb := sheet.Resolve(style.Sel{Name: "table-view", Part: "thumb", State: state})
		want := sheet.Resolve(style.Sel{Name: "scroll-view", Part: "thumb", State: state})
		bg, _ = thumb.BackgroundColor()
		wantBG, _ := want.BackgroundColor()
		radius, _ = thumb.CornerRadius()
		wantRadius, _ := want.CornerRadius()
		width, _ = thumb.BorderWidth()
		if !reflect.DeepEqual(bg, wantBG) || radius != wantRadius || width != 0 {
			t.Fatal("表格滑块丢失通用状态色/圆角，或继承了外框")
		}
	}
}
