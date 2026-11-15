package main

import (
	"fmt"
	"image/color"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/ui"
)

// 单元格透明外框不能使文本选区也透明；各部件随现有主题切换。
func TestDemoCellEditSelectionStyles(t *testing.T) {
	for _, mode := range []int{1, 2} {
		for accent := 0; accent < 4; accent++ {
			d := &demo{mode: mode, accent: accent}
			sheet := style.Sheet(d.styleRules(ui.Settings{})...)
			for _, state := range []style.State{style.Normal, style.Focused, style.Focused | style.Hovered, style.Focused | style.Pressed, style.FocusVisible, style.Disabled} {
				t.Run(fmt.Sprintf("mode=%d/accent=%d/state=%d", mode, accent, state), func(t *testing.T) {
					resolve := func(name, part string) style.Style {
						return sheet.Resolve(style.Sel{Name: name, Part: part, State: state})
					}
					selection, ok := resolve("demo-cell-edit", "selection").BackgroundColor()
					if !ok || selection == nil || demoStyleColor(selection).A != 255 {
						t.Fatalf("文本选区必须有不透明背景，实际为 %v", selection)
					}
					want, _ := resolve("text-input", "selection").BackgroundColor()
					if demoStyleColor(selection) != demoStyleColor(want) {
						t.Fatalf("选区应跟随标准输入框主题：实际 %v，期望 %v", selection, want)
					}
					row, _ := sheet.Resolve(style.Sel{Name: "table-view", Part: "row-selected"}).BackgroundColor()
					if demoStyleColor(selection) == demoStyleColor(row) {
						t.Fatal("文本选区与表格行选中背景无法区分")
					}
					for _, part := range []string{"caret", "placeholder"} {
						got, ok := resolve("demo-cell-edit", part).ForegroundColor()
						want, _ := resolve("text-input", part).ForegroundColor()
						if !ok || got == nil || demoStyleColor(got) != demoStyleColor(want) {
							t.Fatalf("%s 部件没有保留标准输入框外观：%v，期望 %v", part, got, want)
						}
					}
					if state.Has(style.Disabled) {
						return
					}
					box := resolve("demo-cell-edit", style.PartDefault)
					bg, ok := box.BackgroundColor()
					if !ok || demoStyleColor(bg).A != 0 {
						t.Fatal("单元格外框失去透明背景")
					}
					width := float32(0)
					if state.Normalized().Has(style.Focused) {
						width = 1
					}
					if got, _ := box.BorderWidth(); got != width {
						t.Fatalf("单元格边框宽度改变：%g，期望 %g", got, width)
					}
					if radius, _ := box.CornerRadius(); radius != (geometry.CornerRadius{}) {
						t.Fatal("单元格外框不应增加圆角")
					}
				})
			}
		}
	}
}

func demoStyleColor(c color.Color) color.NRGBA {
	if c == nil {
		return color.NRGBA{}
	}
	return color.NRGBAModel.Convert(c).(color.NRGBA)
}

// 边线开关不能通过样式回退把外框复制到内部宿主；切换外观后表头、滚动条和选区仍完整。
func TestDemoTableBorderScopesAndOwnedParts(t *testing.T) {
	for _, mode := range []int{1, 2} {
		for _, tc := range []struct {
			look                        int
			outer, horizontal, vertical float32
			top, bottom, left, right    float32
			outline, separator          float32
		}{
			{tableLookDefault, 1, 1, 0, 0, 1, 0, 0, 0, 1},
			{tableLookGrid, 0, 1, 1, 1, 1, 1, 1, 0, 1},
			{tableLookCells, 0, 0, 0, 1, 1, 1, 1, 0, 0},
			{tableLookOutline, 0, 0, 0, 0, 0, 0, 0, 1, 0},
			{tableLookGridOutline, 0, 1, 1, 0, 0, 0, 0, 1, 1},
		} {
			t.Run(fmt.Sprintf("mode=%d/look=%d", mode, tc.look), func(t *testing.T) {
				d := &demo{mode: mode, data: &dataDemo{tableLook: ui.MakeState(tc.look)}}
				sheet := style.Sheet(d.styleRules(ui.Settings{})...)
				resolve := func(name, part string, state style.State) style.Style {
					return sheet.Resolve(style.Sel{Name: name, Part: part, State: state})
				}
				for part, want := range map[string]float32{
					"outer-horizontal": tc.outer, "outer-vertical": tc.outer,
					"grid-horizontal": tc.horizontal, "grid-vertical": tc.vertical,
					"grid-top": tc.top, "grid-bottom": tc.bottom, "grid-left": tc.left, "grid-right": tc.right,
					"outline-horizontal": tc.outline, "outline-vertical": tc.outline,
					"header-separator": tc.separator, "frame": 0, "trough": 0,
				} {
					if got, ok := resolve("demo-records", part, style.Normal).BorderWidth(); !ok || got != want {
						t.Errorf("%s 宽度 = %g/%v，期望 %g", part, got, ok, want)
					}
				}
				for _, state := range []style.State{style.Normal, style.Hovered, style.Pressed, style.Disabled, style.FocusVisible} {
					for _, part := range []string{"header", "header-text", "header-sort", "trough", "thumb", "row-selected", "row-current"} {
						got, want := resolve("demo-records", part, state), resolve("table-view", part, state)
						bg, _ := got.BackgroundColor()
						wantBG, _ := want.BackgroundColor()
						fg, _ := got.ForegroundColor()
						wantFG, _ := want.ForegroundColor()
						if demoStyleColor(bg) != demoStyleColor(wantBG) || demoStyleColor(fg) != demoStyleColor(wantFG) {
							t.Errorf("%s 状态 %d 丢失 Modern 部件颜色", part, state)
						}
					}
				}
				text := resolve("demo-records", "header-text", style.Normal)
				if size, ok := text.FontSize(); !ok || size <= 0 {
					t.Fatal("自定义表格名没有完整表头文字样式")
				}
				header, _ := resolve("demo-records", "header", style.Normal).BackgroundColor()
				trough, _ := resolve("demo-records", "trough", style.Normal).BackgroundColor()
				if demoStyleColor(header) != demoStyleColor(trough) {
					t.Fatal("滚动槽与表头失去统一底色")
				}
			})
		}
	}
}
