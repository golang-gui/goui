package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/example/demo/resource"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/gui/diagnostics"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	wui "github.com/golang-gui/goui/widgets/ui"
)

// Capture belongs to the gallery lifetime, so explicit recording can span
// navigation. Reading is manual: the observer never starts a refresh loop.
func (d *windowsDemo) diagnosticsView() ui.View {
	return ui.Build(func() ui.View {
		active := d.capturing.Get()
		return ui.VBox(
			ui.Label(resource.Diagnostics()).Style("demo-subheading"),
			ui.Label(resource.DiagnosticsHint()).WrapMode(gui.WrapWordChar).Style(modern.MutedText),
			ui.HWrap(
				ui.Button(resource.CaptureStart()).ID("capture-start").Enabled(!active).OnClick(d.startCapture),
				ui.Button(resource.CaptureRead()).ID("capture-read").Enabled(d.capture != nil).OnClick(d.readCapture),
				ui.Button(resource.CaptureStop()).ID("capture-stop").Enabled(active).OnClick(d.stopCapture),
				wui.CheckButton(resource.CaptureWidgets()).ID("capture-widgets").Checked(d.captureWidgets.Get()).Enabled(!active).
					OnChange(func(v wui.CheckState) { d.captureWidgets.Set(v == wui.CheckChecked) }),
				wui.CheckButton(resource.CaptureHUD()).ID("capture-hud").Checked(d.hudVisible.Get()).Enabled(active).
					OnChange(func(v wui.CheckState) { d.showHUD(v == wui.CheckChecked) }),
			).Spacing(10).LineSpacing(8),
			ui.Label(resource.CaptureHUDHint()).WrapMode(gui.WrapWordChar).Style(modern.MutedText),
			ui.Label(resource.TraceHint()).ID("trace-hint").WrapMode(gui.WrapWordChar).Style(modern.MutedText),
			ui.Build(func() ui.View {
				_ = d.captureRevision.Get()
				return ui.Label(captureSummary(d.captureResult)).ID("capture-summary").WrapMode(gui.WrapWordChar).Style(modern.MutedText)
			}),
		).Spacing(8)
	})
}

func (d *windowsDemo) showHUD(visible bool) {
	if !visible {
		d.hud.Destroy()
		d.hud = nil
		d.hudVisible.Set(false)
		return
	}
	if d.capture == nil {
		return
	}
	hud, err := d.capture.ShowHUD(d.host.app.FindWindow("main"))
	if err != nil {
		d.host.result(resource.CaptureHUD(), err)
		return
	}
	// Leave the title controls visible; the HUD never intercepts sidebar clicks.
	hud.SetPosition(geometry.Point{X: 12, Y: 52})
	d.hud = hud
	d.hudVisible.Set(true)
}

func (d *windowsDemo) startCapture() {
	if d.capturing.Get() {
		return
	}
	capture, err := diagnostics.Start(gui.App, diagnostics.Options{Widgets: d.captureWidgets.Get()})
	if err != nil {
		d.host.result(resource.CaptureStart(), err)
		return
	}
	d.capture = capture
	d.captureResult = diagnostics.Snapshot{}
	d.captureRevision.Update(func(value int) int { return value + 1 })
	d.capturing.Set(true)
	d.host.report(resource.CaptureActive())
}

func (d *windowsDemo) readCapture() {
	if d.capture == nil {
		return
	}
	result := d.capture.Snapshot()
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		d.host.result(resource.CaptureRead(), err)
		return
	}
	d.captureResult = result
	d.captureRevision.Update(func(value int) int { return value + 1 })
	d.inspection.SetText(string(data))
}

func (d *windowsDemo) stopCapture() {
	if d.capture != nil {
		d.capture.Stop()
		d.hud = nil
		d.hudVisible.Set(false)
		d.capturing.Set(false)
		d.readCapture()
		d.host.report(resource.CaptureStopped())
	}
}

func (d *windowsDemo) closeCapture() {
	if d.capture != nil {
		d.capture.Stop()
		d.capture = nil
	}
	d.hud = nil
}

func captureSummary(result diagnostics.Snapshot) string {
	if result.Started.IsZero() {
		return resource.CaptureInitial()
	}
	var frames uint64
	var fps float64
	var last time.Duration
	for _, root := range result.Roots {
		if root.ID != "main" {
			continue
		}
		frames, fps = root.Completed, root.FPS
		for _, frame := range result.Frames {
			if frame.RootRef == root.Ref && frame.Completed {
				last = frame.Total
			}
		}
		break
	}
	r := result.Resources
	return fmt.Sprintf(resource.CaptureFrameSummary(), frames, fps, float64(last)/float64(time.Millisecond)) + "\n" +
		fmt.Sprintf(resource.CaptureResourceSummary(), r.LayoutsCreated, r.TextRasterizations, r.ImagesCreated, r.ImagesUpdated, r.TextCacheHits, r.TextCacheMisses)
}
