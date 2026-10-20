// Cached long-paragraph caret regression. Requires a desktop and native fonts.
// Run: GOUI_PLAT_PAINTER=software GOUI_PLAT_SCALE=1 go run ./tests/window/gui/text_caret
// Also run at scale 2; use software on all platforms for the typography wrapper.
// Initial state: Modern, integrated window, empty TextView in ScrollView.
// Automatic steps use Window.DispatchEvent:
//  1. Load 32 KiB of an original ASCII word and wait for its wrapped layout.
//  2. Scroll, then click three distinct native cluster interiors. Each caret
//     must equal that cluster's leading UTF-8 offset.
//  3. Navigate to document end/start. Selection and scrolling must follow.
//
// After each navigation, a frame and a subsequent timer must complete without
// creating, resizing or exporting metrics for the long paragraph. This is a
// deterministic reuse assertion, not a hardware-specific latency threshold.
// Logged latency spans event dispatch through the first timer after a frame
// (20 ms timer resolution), not display/compositor presentation. Initial layout
// is excluded and can still block. The native metrics supply click expectations;
// byte/cell geometry and reentrant callbacks have separate package tests.
// Platform font differences can change wrapping. The command modifier is
// resolved by GOUI; no native IME or physical mouse acceptance is claimed.
// No clipboard/settings/files are modified. PASS closes the window; closing
// early fails. Restart resets the sample.
package main

import (
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/theme/modern"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func check(ok bool, message string) {
	if !ok {
		panic(message)
	}
}

type observedApp struct {
	gui.Application
	context typography.Context
}

func (a observedApp) Typography() typography.Context { return a.context }

type counters struct{ created, resized, metrics int }

type observedContext struct {
	typography.Context
	counters
	lines         []typography.TextLine
	clusters      []typography.TextCluster
	fieldBaseline float32
}

func (c *observedContext) NewTextLayout(s string, f typography.TextFormat, w, h float32) (typography.TextLayout, error) {
	l, err := c.Context.NewTextLayout(s, f, w, h)
	if err != nil || len(s) < 1024 && s != "Ag中" {
		return l, err
	}
	if len(s) >= 1024 {
		c.created++
	}
	return &observedLayout{TextLayout: l, context: c}, nil
}

type observedLayout struct {
	typography.TextLayout
	context *observedContext
}

func (l *observedLayout) SetSize(w, h float32) {
	oldW, oldH := l.Size()
	if len(l.Text()) >= 1024 && (w != oldW || h != oldH) {
		l.context.resized++
	}
	l.TextLayout.SetSize(w, h)
}

func (l *observedLayout) MeasureMetrics() ([]typography.TextLine, []typography.TextCluster) {
	lines, clusters := l.TextLayout.MeasureMetrics()
	if len(l.Text()) >= 1024 {
		l.context.metrics++
		l.context.lines, l.context.clusters = lines, clusters
	} else if len(lines) > 0 {
		l.context.fieldBaseline = lines[0].Baseline
	}
	return lines, clusters
}

func main() {
	app, err := gui.NewApplication("org.golanggui.TextCaretRegression")
	must(err)
	ctx := &observedContext{Context: app.Typography()}
	gui.App = observedApp{app, ctx}
	app.SetStyleSheet(modern.Sheet(modern.Options{}))
	win, err := app.NewWindow(&gui.WindowOptions{
		Size:   geometry.Size{Width: 760, Height: 360},
		Chrome: gui.WindowChromeIntegrated,
	})
	must(err)
	defer win.Destroy()
	must(win.SetTitle("Wrapped text caret reuse"))
	v := gui.NewTextView()
	scroll := gui.NewScrollView()
	scroll.SetChild(v)
	win.SetWidget(scroll)
	must(win.Show())
	win.SetFocusedWidget(v)

	const length = 32768
	primary, ok := gui.ModPrimary.Resolve()
	check(ok, "primary modifier unavailable")
	frame, actionFrame := 0, 0
	frameHandle := win.ConnectFrame(func(time.Time) { frame++ })
	defer frameHandle.Disconnect()
	var baseline counters
	var started time.Time
	stage, action, expected := 0, 0, 0
	complete := false
	deadline := time.Now().Add(time.Minute)
	timer := app.NewTimer()
	defer timer.Stop()
	timer.ConnectTimeout(func() {
		check(time.Now().Before(deadline), "caret verification timed out")
		if v.Rect().Width <= 0 || !win.Focused() {
			return
		}
		switch stage {
		case 0:
			v.Model().SetText(strings.Repeat("a", length))
			actionFrame, stage = frame, 1
		case 1:
			if frame <= actionFrame || len(ctx.lines) < 20 {
				return
			}
			scroll.SetScrollY(300)
			actionFrame, stage = frame, 2
		case 2:
			if frame <= actionFrame {
				return
			}
			baseline, started, actionFrame = ctx.counters, time.Now(), frame
			if action < 3 {
				line := 0
				for line+1 < len(ctx.lines) && ctx.lines[line].Y < scroll.ScrollY()+40 {
					line++
				}
				var target typography.TextCluster
				found, cell := false, 0
				for _, c := range ctx.clusters {
					if c.LineIndex != line || c.Width <= 0 {
						continue
					}
					if cell == 10+6*action {
						target, found = c, true
						break
					}
					cell++
				}
				check(found, "missing native click target")
				expected = target.Start
				bounds := v.Snapshot().Bounds
				point := geometry.Point{
					X: bounds.X + v.Padding() + target.X + target.Width/4,
					Y: bounds.Y + v.Padding() + max(0, ctx.fieldBaseline-ctx.lines[0].Baseline) +
						target.Y + target.Height/2 - scroll.ScrollY(),
				}
				for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
					must(win.DispatchEvent(events.PointerEvent{
						EventType: kind, Position: point, Button: events.PointerButtonLeft,
					}))
				}
			} else {
				key := events.KeyEnd
				expected = length
				if action == 4 {
					key, expected = events.KeyHome, 0
				}
				for _, kind := range []events.EventType{events.KeyDown, events.KeyUp} {
					must(win.DispatchEvent(events.KeyEvent{EventType: kind, Key: key, Modifiers: primary}))
				}
			}
			check(v.Selection() == (gui.TextSelection{Anchor: expected, Caret: expected}),
				fmt.Sprintf("action %d: caret=%+v want=%d", action, v.Selection(), expected))
			stage = 3
		case 3:
			if frame <= actionFrame {
				return
			}
			log.Printf("action=%d event-to-post-frame-timer=%v scrollY=%g native-before=%+v native-after=%+v",
				action, time.Since(started), scroll.ScrollY(), baseline, ctx.counters)
			check(ctx.counters == baseline, "caret navigation repeated whole-paragraph layout")
			if action == 3 {
				check(scroll.ScrollY() > 300, "document end did not scroll")
			}
			if action == 4 {
				// The mixed-script field baseline can place ASCII below the
				// native layout origin (4 DIP on the Linux test font). Reveal
				// aligns that actual caret with the padded viewport, not Y=0.
				top := float32(math.Floor(float64(max(0, ctx.fieldBaseline-ctx.lines[0].Baseline))))
				check(scroll.ScrollY() == top, fmt.Sprintf("document start: scroll=%g want=%g", scroll.ScrollY(), top))
				complete = true
				log.Printf("PASS cached text caret bytes=%d typography=%s", length, ctx.Name())
				win.Destroy()
				return
			}
			action++
			stage = 2
		}
	})
	must(timer.Start(20 * time.Millisecond))
	app.Run()
	check(complete, "caret verification aborted")
}
