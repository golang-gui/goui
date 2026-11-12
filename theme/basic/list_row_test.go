package basic

import (
	"image/color"
	"testing"

	"github.com/golang-gui/goui/style"
)

func TestListRowFallbackBoundaries(t *testing.T) {
	sheet := Sheet()
	resolve := func(name string, state style.State) style.Style {
		return sheet.Resolve(style.Sel{Name: name, State: state})
	}
	normal := resolve("list-row", style.Normal)
	background, _ := normal.BackgroundColor()
	border, _ := normal.BorderColor()
	width, _ := normal.BorderWidth()
	if background == nil || color.RGBAModel.Convert(background).(color.RGBA).A != 255 || border == nil || width != 1 {
		t.Fatal("Basic 行必须有不透明表面和清晰的 1 DIP 边界")
	}
	label := resolve("label", style.Normal)
	foreground, _ := label.ForegroundColor()
	disabled, _ := resolve("label", style.Disabled).ForegroundColor()
	if border != disabled {
		t.Fatal("兜底边界没有复用已有中性灰")
	}
	for _, state := range []style.State{style.Hovered, style.Pressed} {
		rowBG, _ := resolve("list-row", state).BackgroundColor()
		buttonBG, _ := resolve("button", state).BackgroundColor()
		if rowBG != buttonBG {
			t.Fatal("Basic 行增加了独立的状态色")
		}
	}
	for _, name := range []string{"list-row-title", "list-row-subtitle"} {
		fg, _ := resolve(name, style.Normal).ForegroundColor()
		if fg != foreground {
			t.Fatal("Basic 主次文字应共用基础前景，通过排版区分")
		}
	}
	titleSize, _ := resolve("list-row-title", style.Normal).FontSize()
	subtitleSize, _ := resolve("list-row-subtitle", style.Normal).FontSize()
	if subtitleSize >= titleSize || subtitleSize <= 0 {
		t.Fatal("副标题字号没有体现层次")
	}
	focus := sheet.Resolve(style.Sel{Name: "list-row", Part: "focus", State: style.FocusVisible})
	focusColor, _ := focus.BorderColor()
	if focusColor != foreground {
		t.Fatal("Basic 焦点应使用已有前景，不增加强调色")
	}
	disabledBG, _ := resolve("list-row", style.Disabled).BackgroundColor()
	if disabledBG != background {
		t.Fatal("禁用行丢失了常态表面")
	}
}
