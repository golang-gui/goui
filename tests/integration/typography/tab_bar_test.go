//go:build linux || windows || darwin

package typography_test

import (
	"fmt"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/widgets"
	"testing"
)

// 真实排版跨层验证：较高文字、关闭按钮占位及窄标签裁剪；不创建原生窗口。
func TestTabLabelRetainsRealLineHeight(t *testing.T) {
	for _, size := range []float32{14, 24} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			context := nativeContext(t)
			useTypography(t, context, modern.Sheet(modern.Options{FontSize: size}))
			view := widgets.NewTabView()
			page := widgets.NewTabPage("Document typography gjpq", nil)
			page.SetClosable(true)
			view.AppendPage(page)
			bar := widgets.NewTabBar()
			bar.SetTabWidthRange(96, 600) // 为完整 24pt 标题留出空间
			bar.SetView(view)
			ownWidget(t, bar)
			m := bar.Measure(layout.Loose(geometry.Size{Width: 800, Height: 200}))
			bar.Arrange(geometry.Rect(0, 0, m.Width, m.Height))
			label := descendants(bar, func(w gui.Widget) bool { l, ok := w.(*gui.Label); return ok && l.Text() == page.Title() })[0].(*gui.Label)
			body := label.Parent()
			item := body.Parent()
			closeButton := descendants(item, func(w gui.Widget) bool { return w.StyleName() == "tab-close-button" })[0]
			natural := label.Measure(layout.Unbounded())
			if natural.Height <= 16 {
				t.Fatalf("fixture needs a real line taller than the old 16 DIP box: %v", natural)
			}
			if label.Rect().Height < natural.Height || label.Rect().Width < natural.Width {
				t.Fatalf("font=%g clipped label: natural=%v allocated=%v", size, natural, label.Rect())
			}
			if label.Rect().Y < 4 || label.Rect().Y+label.Rect().Height+4 > body.Rect().Height {
				t.Fatalf("line or vertical padding escaped tab body: label=%v body=%v", label.Rect(), body.Rect())
			}
			if label.Rect().X != 10 || closeButton.Rect().X+24 != item.Rect().Width-4 {
				t.Fatalf("lost title/close breathing room: label=%v close=%v item=%v", label.Rect(), closeButton.Rect(), item.Rect())
			}
			// 可关闭状态只改变内容占位，不改变等宽标签分配。
			closableWidth := item.Rect().Width
			bodyWidth := body.Measure(layout.Unbounded()).Width
			page.SetClosable(false)
			bar.Arrange(geometry.Rect(0, 0, m.Width, m.Height))
			if item.Rect().Width != closableWidth || body.Measure(layout.Unbounded()).Width != bodyWidth-28 || closeButton.Visible() {
				t.Fatal("removing close retained stale reserved width")
			}
			page.SetClosable(true)
			bar.Arrange(geometry.Rect(0, 0, m.Width, m.Height))
			if item.Rect().Width != closableWidth || !closeButton.Visible() {
				t.Fatal("restoring close lost its reserved width")
			}
			// 窄标签只在水平方向裁剪文字，保持完整行高与关闭按钮的 24 DIP 命中宽度。
			bar.Arrange(geometry.Rect(0, 0, 110, m.Height))
			if label.Rect().Height < natural.Height || closeButton.Rect().Width != 24 || closeButton.Rect().X+24 != item.Rect().Width-4 {
				t.Fatalf("narrow layout lost text height or close target: item=%v label=%v close=%v", item.Rect(), label.Rect(), closeButton.Rect())
			}
			if label.Rect().Width >= natural.Width || label.Text() != page.Title() || item.Snapshot().Text != page.Title() {
				t.Fatal("narrow title must be visually clipped, not truncated or replaced")
			}
		})
	}
}
