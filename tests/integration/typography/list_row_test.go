//go:build linux || windows || darwin

package typography_test

import (
	"fmt"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/widgets"
	"strings"
	"testing"
)

// 真实排版无需桌面；原生调用与释放都在当前子测试锁定的线程。
func TestListRowNativeWrapping(t *testing.T) {
	for _, font := range []float32{14, 26} {
		t.Run(fmt.Sprint(font), func(t *testing.T) {
			context := nativeContext(t)
			useTypography(t, context, modern.Sheet(modern.Options{FontSize: font}))
			r := widgets.NewListRow("Application updates")
			r.SetSubtitle(strings.Repeat("下载完成后通知，preserve user choices. ", 4))
			r.SetPrefix(fixedWidget(24, 24))
			r.SetSuffix(fixedWidget(60, 32))
			owner := gui.NewPopover(nil, nil)
			owner.SetWidget(r)
			defer owner.Destroy()
			wide := r.Measure(layout.Loose(geometry.Size{Width: 560, Height: layout.Inf}))
			narrow := r.Measure(layout.Loose(geometry.Size{Width: 260, Height: layout.Inf}))
			if wide.Width != 560 || narrow.Width != 260 || narrow.Height <= wide.Height || wide.Height <= 48 {
				t.Fatalf("字号 %g 没有随真实文字换行增高：wide=%v narrow=%v", font, wide, narrow)
			}
			r.Arrange(geometry.Rect(0, 0, narrow.Width, narrow.Height))
			title := descendants(r, func(w gui.Widget) bool { return w.StyleName() == "list-row-title" })[0]
			subtitle := descendants(r, func(w gui.Widget) bool { return w.StyleName() == "list-row-subtitle" })[0]
			text := title.Parent()
			if r.Suffix().Rect().X+60 != 248 || text.Rect().Width != 128 || title.Rect().Height <= 16 || subtitle.Rect().Y != title.Rect().Height+4 || subtitle.Rect().Y+subtitle.Rect().Height > text.Rect().Height {
				t.Fatalf("文字或插槽位置错误：text=%v title=%v subtitle=%v suffix=%v", text.Rect(), title.Rect(), subtitle.Rect(), r.Suffix().Rect())
			}
		})
	}
}
