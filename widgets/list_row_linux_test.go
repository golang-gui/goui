package widgets

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/typography/pango"
	"github.com/golang-gui/goui/theme/modern"
)

// 真实 Pango 排版不需要 DISPLAY；原生调用与释放都在当前子测试锁定的线程。
func TestListRowNativeWrapping(t *testing.T) {
	for _, font := range []float32{14, 26} {
		t.Run(fmt.Sprint(font), func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			context, err := pango.NewContext()
			if err != nil {
				t.Fatal(err)
			}
			defer context.Destroy()
			previous := gui.App
			gui.App = &tabTypographyApp{context: context, sheet: modern.Sheet(modern.Options{FontSize: font})}
			defer func() { gui.App = previous }()
			r := NewListRow("Application updates")
			r.SetSubtitle(strings.Repeat("下载完成后通知，preserve user choices. ", 4))
			r.SetPrefix(listRowFixed(24, 24))
			r.SetSuffix(listRowFixed(60, 32))
			owner := gui.NewPopover(nil, nil)
			owner.SetWidget(r)
			defer owner.Destroy()
			wide := r.Measure(layout.Loose(geometry.Size{Width: 560, Height: layout.Inf}))
			narrow := r.Measure(layout.Loose(geometry.Size{Width: 260, Height: layout.Inf}))
			if wide.Width != 560 || narrow.Width != 260 || narrow.Height <= wide.Height || wide.Height <= 48 {
				t.Fatalf("字号 %g 没有随真实文字换行增高：wide=%v narrow=%v", font, wide, narrow)
			}
			r.Arrange(geometry.Rect(0, 0, narrow.Width, narrow.Height))
			if r.Suffix().Rect().X+60 != 248 || r.text.Rect().Width != 128 || r.title.Rect().Height <= 16 || r.subtitle.Rect().Y != r.title.Rect().Height+4 || r.subtitle.Rect().Y+r.subtitle.Rect().Height > r.text.Rect().Height {
				t.Fatalf("文字或插槽位置错误：text=%v title=%v subtitle=%v suffix=%v", r.text.Rect(), r.title.Rect(), r.subtitle.Rect(), r.Suffix().Rect())
			}
		})
	}
}
