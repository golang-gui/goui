package widgets

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/pango"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
)

type tabTypographyApp struct {
	gui.Application
	context typography.Context
	sheet   style.StyleSheet
}

func (a *tabTypographyApp) Typography() typography.Context { return a.context }
func (a *tabTypographyApp) StyleSheet() style.StyleSheet   { return a.sheet }

// Pango's font map and layout measurement require native libraries, not a
// desktop connection. Keep native calls and destruction on this test's thread.
func TestTabLabelRetainsRealLineHeight(t *testing.T) {
	for _, size := range []float32{14, 24} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			context, err := pango.NewContext()
			if err != nil {
				t.Fatal(err)
			}
			defer context.Destroy()
			oldApp := gui.App
			gui.App = &tabTypographyApp{context: context, sheet: modern.Sheet(modern.Options{FontSize: size})}
			defer func() { gui.App = oldApp }()
			view := NewTabView()
			page := NewTabPage("Document typography gjpq", nil)
			page.SetClosable(true)
			view.AppendPage(page)
			bar := NewTabBar()
			bar.SetTabWidthRange(96, 600) // room for the complete 24pt title
			bar.SetView(view)
			defer releaseTabLayouts(bar)
			m := bar.Measure(layout.Loose(geometry.Size{Width: 800, Height: 200}))
			bar.Arrange(geometry.Rect(0, 0, m.Width, m.Height))
			item := bar.items[page]
			label := item.body.label
			natural := label.Measure(layout.Unbounded())
			if natural.Height <= 16 {
				t.Fatalf("fixture needs a real line taller than the old 16 DIP box: %v", natural)
			}
			if label.Rect().Height < natural.Height || label.Rect().Width < natural.Width {
				t.Fatalf("font=%g clipped label: natural=%v allocated=%v", size, natural, label.Rect())
			}
			if label.Rect().Y < 4 || label.Rect().Y+label.Rect().Height+4 > item.body.Rect().Height {
				t.Fatalf("line or vertical padding escaped tab body: label=%v body=%v", label.Rect(), item.body.Rect())
			}
			if label.Rect().X != 10 || item.close.Rect().X+24 != item.Rect().Width-4 {
				t.Fatalf("lost title/close breathing room: label=%v close=%v item=%v", label.Rect(), item.close.Rect(), item.Rect())
			}
			// Closability changes reserved content space, not the equal slot.
			closableWidth := item.Rect().Width
			bodyWidth := item.body.Measure(layout.Unbounded()).Width
			page.SetClosable(false)
			bar.Arrange(geometry.Rect(0, 0, m.Width, m.Height))
			if item.Rect().Width != closableWidth || item.body.Measure(layout.Unbounded()).Width != bodyWidth-28 || item.close.Visible() {
				t.Fatal("removing close retained stale reserved width")
			}
			page.SetClosable(true)
			bar.Arrange(geometry.Rect(0, 0, m.Width, m.Height))
			if item.Rect().Width != closableWidth || !item.close.Visible() {
				t.Fatal("restoring close lost its reserved width")
			}
			// A narrow tab may clip text horizontally, but must preserve its line
			// height and reserve the close button's 24 DIP hit target.
			bar.Arrange(geometry.Rect(0, 0, 110, m.Height))
			if label.Rect().Height < natural.Height || item.close.Rect().Width != 24 || item.close.Rect().X+24 != item.Rect().Width-4 {
				t.Fatalf("narrow layout lost text height or close target: item=%v label=%v close=%v", item.Rect(), label.Rect(), item.close.Rect())
			}
			if label.Rect().Width >= natural.Width || label.Text() != page.Title() || item.Snapshot().Text != page.Title() {
				t.Fatal("narrow title must be visually clipped, not truncated or replaced")
			}
		})
	}
}

func releaseTabLayouts(widget gui.Widget) {
	for _, child := range widget.Children() {
		releaseTabLayouts(child)
	}
	widget.StyleChanged() // release unattached labels' cached native layouts
}
