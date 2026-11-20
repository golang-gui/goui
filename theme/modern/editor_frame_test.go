package modern

import (
	"image/color"
	"reflect"
	"testing"

	"github.com/golang-gui/goui/style"
)

func TestEditorFrameParts(t *testing.T) {
	for _, dark := range []bool{false, true} {
		sheet := Sheet(Options{Dark: dark})
		for _, state := range []style.State{style.Normal, style.Focused, style.FocusVisible, style.Disabled} {
			body := sheet.Resolve(style.Sel{Name: "text-view", State: state})
			frame := sheet.Resolve(style.Sel{Name: "text-view", Part: "frame", State: state})
			if width, _ := body.BorderWidth(); width != 0 {
				t.Fatal("body retained an inner frame")
			}
			if width, _ := frame.BorderWidth(); width <= 0 {
				t.Fatal("missing outer editor frame")
			}
			input := sheet.Resolve(style.Sel{Name: "text-input", State: state})
			if width, _ := input.BorderWidth(); width <= 0 {
				t.Fatal("TextInput lost its own frame")
			}
		}
		for _, part := range []string{"trough", "thumb", "corner"} {
			for _, state := range []style.State{style.Normal, style.Hovered, style.Pressed} {
				got := sheet.Resolve(style.Sel{Name: "text-view", Part: part, State: state})
				want := sheet.Resolve(style.Sel{Name: "scroll-view", Part: part, State: state})
				gotBG, _ := got.BackgroundColor()
				wantBG, _ := want.BackgroundColor()
				gotRadius, _ := got.CornerRadius()
				wantRadius, _ := want.CornerRadius()
				width, _ := got.BorderWidth()
				if !reflect.DeepEqual(gotBG, wantBG) || gotRadius != wantRadius || width != 0 {
					t.Fatalf("bar style differs: %s/%v got=%#v want=%#v", part, state, got, want)
				}
			}
		}
		for _, name := range []string{"scroll-view", "text-view"} {
			for _, state := range []style.State{style.Normal, style.Focused, style.FocusVisible, style.Disabled} {
				corner := sheet.Resolve(style.Sel{Name: name, Part: "corner", State: state})
				bg, _ := corner.BackgroundColor()
				width, _ := corner.BorderWidth()
				radius, _ := corner.CornerRadius()
				if bg == nil || color.RGBAModel.Convert(bg).(color.RGBA).A != 0 || width != 0 || radius.Max() != 0 {
					t.Fatalf("默认角区未保持透明／无装饰：name=%s state=%v style=%+v", name, state, corner)
				}
			}
		}

		plain := sheet.Resolve(style.Sel{Name: "scroll-view", Part: "frame"})
		if width, _ := plain.BorderWidth(); width != 0 {
			t.Fatal("ordinary scroll view acquired an editor frame")
		}
	}
}
