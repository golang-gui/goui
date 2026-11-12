package modern

import (
	"image/color"
	"reflect"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/style"
)

func TestTableScrollParts(t *testing.T) {
	for _, dark := range []bool{false, true} {
		sheet := Sheet(Options{Dark: dark})
		for _, state := range []style.State{style.Normal, style.Hovered, style.Pressed, style.Focused, style.Disabled} {
			frame := sheet.Resolve(style.Sel{Name: "table-view", Part: "frame", State: state})
			bg, _ := frame.BackgroundColor()
			width, _ := frame.BorderWidth()
			radius, _ := frame.CornerRadius()
			if bg == nil || color.RGBAModel.Convert(bg).(color.RGBA).A != 0 || width != 0 || radius != (geometry.CornerRadius{}) {
				t.Fatal("内部滚动区域出现第二层框或覆盖表体背景")
			}
			track := sheet.Resolve(style.Sel{Name: "table-view", Part: "trough", State: state})
			bg, _ = track.BackgroundColor()
			header, _ := sheet.Resolve(style.Sel{Name: "table-view", Part: "header"}).BackgroundColor()
			radius, _ = track.CornerRadius()
			width, _ = track.BorderWidth()
			if !reflect.DeepEqual(bg, header) || radius != (geometry.CornerRadius{}) || width != 0 {
				t.Fatal("轨道未复用表头中性色或没有方形收口")
			}
			thumb := sheet.Resolve(style.Sel{Name: "table-view", Part: "thumb", State: state})
			want := sheet.Resolve(style.Sel{Name: "scroll-view", Part: "thumb", State: state})
			bg, _ = thumb.BackgroundColor()
			wantBG, _ := want.BackgroundColor()
			radius, _ = thumb.CornerRadius()
			wantRadius, _ := want.CornerRadius()
			width, _ = thumb.BorderWidth()
			if !reflect.DeepEqual(bg, wantBG) || radius != wantRadius || width != 0 {
				t.Fatal("滑块状态色/圆角或边框不符合通用滚动条契约")
			}
		}
	}
}
