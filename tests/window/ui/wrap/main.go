// WrapBox desktop checks, without DevServer.
//
// Environment: Windows/macOS/Linux desktop. Record OS, backend and scale;
// repeat at 1x/2x. Run: go run ./tests/window/ui/wrap
// Initially a Native 780x650 window has explicit 400x600 minimum size, wrapped
// buttons above baseline, vertical-column and scroll samples.
// Steps and expected results:
//  1. Narrow/widen continuously: buttons wrap/merge in original order without
//     overlap; shorter buttons center within their row. Width reaches 400 DIP
//     with multiple rows, then merges again at 780 DIP. The window minimum must
//     not force the full single-row width.
//  2. Click an item: its name/count appear. After rebuild, each click still
//     increments only once.
//  3. Toggle last removes/restores the last item and reflows. Alignment cycles
//     within-row alignment; all rows together remain top-aligned.
//  4. Different font sizes share baselines within each Baseline row after wrapping.
//     V1-V8 flow top-to-bottom, then left-to-right into columns at the height
//     limit; all stay visible at 400 DIP width.
//  5. Scroll to every numbered item. Explicit MaxWidth(360) does not stretch to
//     viewport width under this bidirectional ScrollView constraint.
//  6. Close to exit; restart to reset. No files or system settings are changed.
//
// Platform differences: fonts affect wrap thresholds; order, spacing and hits
// must remain consistent.
package main

import (
	"fmt"
	"log"

	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
)

func main() {
	count, alignment := 0, 0
	selected := "none"
	showLast := true
	if err := ui.Run("org.golang-gui.Wrap", func(app ui.App) ui.RootView {
		items := []ui.View{}
		names := []string{"Go", "Windows", "Linux", "macOS", "A longer tool button", "Short", "Last"}
		for i, name := range names {
			if i == len(names)-1 && !showLast {
				continue
			}
			items = append(items, ui.Button(name).Name(name).Padding(float32(6+i%3*4)).OnClick(func() { selected = name; count++; app.RequestUpdate() }))
		}
		rows := make([]ui.View, 30)
		for i := range rows {
			rows[i] = ui.Button(fmt.Sprintf("Item %02d", i+1)).MinSize(90, 30)
		}
		columns := make([]ui.View, 8)
		for i := range columns {
			columns[i] = ui.Label(fmt.Sprintf("V%d", i+1)).MinSize(90, 24)
		}
		sheet := style.Sheet(append(modern.Rules(modern.Options{}),
			style.Name("wrap-small").FontSize(14), style.Name("wrap-large").FontSize(26),
		)...)
		// Unbounded measurement describes the preferred single-line width, not
		// the minimum width of a wrapping layout. Override the window's current
		// automatic size hint explicitly so the WM permits this demonstration.
		return ui.Root().StyleSheet(sheet).Windows(ui.Window("wrap").Title("GOUI WrapBox").Size(780, 650).MinSize(100, 600).Content(
			ui.VBox(
				ui.Label(fmt.Sprintf("Clicked: %s (%d); alignment=%d", selected, count, alignment)),
				ui.HWrap(ui.Button("Toggle last").OnClick(func() { showLast = !showLast; app.RequestUpdate() }), ui.Button("Alignment").OnClick(func() { alignment = (alignment + 1) % 4; app.RequestUpdate() })).Spacing(8),
				ui.HWrap(items...).Spacing(8).LineSpacing(6).MainAlign(layout.MainAlign(alignment)),
				ui.HWrap(ui.Label("Baseline").Style("wrap-small"), ui.Label("Large text").Style("wrap-large"), ui.Label("Another baseline").Style("wrap-small")).Spacing(12).LineSpacing(6).CrossAlign(layout.CrossBaseline),
				ui.VWrap(columns...).Spacing(4).LineSpacing(14).MinHeight(80).MaxHeight(80),
				ui.Label("Scroll: content max width 360 DIP"),
				ui.ScrollView(ui.HWrap(rows...).Spacing(8).LineSpacing(8).MaxWidth(360)),
			).Spacing(12).Padding(12).CrossAlign(layout.CrossStretch),
		))
	}); err != nil {
		log.Fatal(err)
	}
}
