package modern

import (
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
		for _, part := range []string{"trough", "thumb"} {
			for _, state := range []style.State{style.Normal, style.Hovered, style.Pressed} {
				got := sheet.Resolve(style.Sel{Name: "text-view", Part: part, State: state})
				want := sheet.Resolve(style.Sel{Name: "scroll-view", Part: part, State: state})
				gotBG, _ := got.BackgroundColor()
				wantBG, _ := want.BackgroundColor()
				gotRadius, _ := got.Radius()
				wantRadius, _ := want.Radius()
				width, _ := got.BorderWidth()
				if !reflect.DeepEqual(gotBG, wantBG) || gotRadius != wantRadius || width != 0 {
					t.Fatalf("bar style differs: %s/%v got=%#v want=%#v", part, state, got, want)
				}
			}
		}
		plain := sheet.Resolve(style.Sel{Name: "scroll-view", Part: "frame"})
		if width, _ := plain.BorderWidth(); width != 0 {
			t.Fatal("ordinary scroll view acquired an editor frame")
		}
	}
}
