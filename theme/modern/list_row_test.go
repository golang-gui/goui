package modern

import (
	"image/color"
	"testing"

	"github.com/golang-gui/goui/style"
)

func TestListRowSurfaceHierarchy(t *testing.T) {
	for _, dark := range []bool{false, true} {
		sheet := Sheet(Options{Dark: dark})
		want := []color.RGBA{rgb(0xFFFFFF), rgb(0xF6F6F6), rgb(0xEDEDED)}
		if dark {
			want = []color.RGBA{rgb(0x323238), rgb(0x39393F), rgb(0x414147)}
		}
		for i, state := range []style.State{style.Normal, style.Hovered, style.Pressed} {
			row := resolved(sheet, "list-row", state)
			background, _ := row.BackgroundColor()
			if background == nil || rgba(background) != want[i] {
				t.Fatalf("dark=%v state=%v 背景=%v，预期已有中性表面 %v", dark, state, background, want[i])
			}
			button, _ := resolved(sheet, "button", state).BackgroundColor()
			if rgba(background) != rgba(button) {
				t.Fatal("行未复用既有按钮中性色阶")
			}
			border, _ := row.BorderColor()
			width, _ := row.BorderWidth()
			if width != 1 || border == nil || rgba(border).A != 255 {
				t.Fatal("Modern 行缺少克制的实色边界")
			}
		}
		normal, _ := resolved(sheet, "list-row", style.Normal).BackgroundColor()
		disabled, _ := resolved(sheet, "list-row", style.Disabled).BackgroundColor()
		if normal != disabled {
			t.Fatal("禁用行改变了表面层次")
		}
		for rowName, baseName := range map[string]string{"list-row-title": "label", "list-row-subtitle": MutedText} {
			row, _ := resolved(sheet, rowName, style.Normal).ForegroundColor()
			base, _ := resolved(sheet, baseName, style.Normal).ForegroundColor()
			if row != base {
				t.Fatal("文字新增了独立调色板", rowName)
			}
		}
		title, _ := resolved(sheet, "list-row-title", style.Normal).FontSize()
		subtitle, _ := resolved(sheet, "list-row-subtitle", style.Normal).FontSize()
		if subtitle >= title || subtitle <= 0 {
			t.Fatal("主次文字字号未区分")
		}
	}
}

func TestListRowStateContrastAcrossAccents(t *testing.T) {
	// 对量化后的 sRGB 配色计算 WCAG 相对亮度；文字 >=4.5:1，焦点 >=3:1。
	// 不把卡片/页面或悬停色差当作文字对比度要求。
	for _, dark := range []bool{false, true} {
		for _, r := range []uint8{0, 64, 128, 192, 255} {
			for _, g := range []uint8{0, 64, 128, 192, 255} {
				for _, b := range []uint8{0, 64, 128, 192, 255} {
					accent := color.RGBA{R: r, G: g, B: b, A: 255}
					sheet := Sheet(Options{Dark: dark, AccentColor: accent})
					focus, _ := sheet.Resolve(style.Sel{Name: "list-row", Part: "focus", State: style.FocusVisible}).BorderColor()
					for _, state := range []style.State{style.Normal, style.Hovered, style.Pressed} {
						background, _ := resolved(sheet, "list-row", state).BackgroundColor()
						if focus == nil || background == nil || contrast(rgba(focus), rgba(background)) < 3 {
							t.Fatalf("焦点不足：dark=%v accent=%v state=%v", dark, accent, state)
						}
						for _, name := range []string{"list-row-title", "list-row-subtitle"} {
							text, _ := resolved(sheet, name, state).ForegroundColor()
							if contrast(rgba(text), rgba(background)) < 4.5 {
								t.Fatalf("文字不足：dark=%v state=%v %s", dark, state, name)
							}
						}
						if state == style.Normal && dark {
							window, _ := resolved(sheet, "window", style.Normal).BackgroundColor()
							if contrast(rgba(window), rgba(background)) < 1.2 {
								t.Fatal("暗色行没有从页面背景中分离")
							}
						}
					}
				}
			}
		}
	}
}
