package gui

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
)

type frameRequesterTest struct {
	requests int
	err      error
}

func (r *frameRequesterTest) RequestPaint() error { r.requests++; return r.err }

func TestRootFrameScheduling(t *testing.T) {
	now := time.Unix(100, 0)
	var posts []func()
	scheduler := newTimerScheduler(func(fn func()) { posts = append(posts, fn) }, func() time.Time { return now })
	app := &application{timers: scheduler}
	b := &rootBase{app: app}
	r := new(frameRequesterTest)
	frames := 0
	b.ConnectFrame(func(at time.Time) {
		frames++
		if at != now {
			t.Fatal("frame timestamp")
		}
	})
	if frames != 0 || len(scheduler.queue) != 0 || r.requests != 0 {
		t.Fatal("passive connection started drawing")
	}
	for range 10 {
		if err := b.requestPaint(r); err != nil {
			t.Fatal(err)
		}
	}
	if r.requests != 1 {
		t.Fatal("requests not coalesced")
	}
	if !b.beginFrame() {
		t.Fatal("frame rejected")
	}
	for range 10 {
		_ = b.requestPaint(r)
	}
	if r.requests != 1 || len(scheduler.queue) != 0 {
		t.Fatal("requested platform during frame")
	}
	b.endFrame()
	if len(scheduler.queue) != 1 || r.requests != 1 {
		t.Fatal("next frame not armed once")
	}
	_ = b.requestPaint(r)
	if len(scheduler.queue) != 1 || r.requests != 1 {
		t.Fatal("async UI invalidation bypassed pacing")
	}
	now = now.Add(frameInterval)
	scheduler.postDue()
	if len(posts) != 1 {
		t.Fatal("deadline did not dispatch")
	}
	posts[0]()
	if r.requests != 2 {
		t.Fatal("timer did not request paint")
	}
	if !b.beginFrame() {
		t.Fatal("second frame")
	}
	b.endFrame()
	if len(scheduler.queue) != 0 || b.framePending || frames != 2 {
		t.Fatal("idle retained work")
	}
}

func TestRootFrameEarlyPaintAndLifecycle(t *testing.T) {
	now := time.Unix(100, 0)
	s := newTimerScheduler(func(func()) {}, func() time.Time { return now })
	b := &rootBase{app: &application{timers: s}}
	r := new(frameRequesterTest)
	_ = b.requestPaint(r)
	b.beginFrame()
	_ = b.requestPaint(r)
	b.endFrame()
	if len(s.queue) != 1 {
		t.Fatal("timer missing")
	}
	now = now.Add(time.Millisecond)
	b.beginFrame() // Native expose satisfies a pending GUI request early.
	b.endFrame()
	if len(s.queue) != 0 || r.requests != 1 {
		t.Fatal("early native paint left stale timer")
	}
	_ = b.requestPaint(r)
	b.suspendFrames()
	now = now.Add(time.Second)
	_ = b.requestPaint(r)
	if len(s.queue) != 0 || r.requests != 1 || b.beginFrame() {
		t.Fatal("suspended root woke")
	}
	b.resumeFrames(r)
	if r.requests != 2 {
		t.Fatal("resume did not request frame")
	}
	called := false
	b.ConnectFrame(func(time.Time) { b.destroyFrames() })
	b.ConnectFrame(func(time.Time) { called = true })
	if b.beginFrame() || called {
		t.Fatal("destroyed root continued callbacks")
	}
	_ = b.requestPaint(r)
	if r.requests != 2 || len(s.queue) != 0 {
		t.Fatal("destroy retained work")
	}
}

func TestRootFrameReentrancyAndFailure(t *testing.T) {
	b := new(rootBase)
	r := &frameRequesterTest{err: errors.New("paint")}
	if b.requestPaint(r) != r.err || b.framePending {
		t.Fatal("submission failure lost")
	}
	r.err = nil
	if err := b.requestPaint(r); err != nil || r.requests != 2 {
		t.Fatal("explicit retry failed")
	}
	b.ConnectFrame(func(time.Time) {
		if b.beginFrame() {
			t.Fatal("reentered frame")
		}
	})
	if !b.beginFrame() {
		t.Fatal("outer frame")
	}
	b.endFrame()
	if r.requests != 3 {
		t.Fatal("reentrant request not preserved")
	}
}

type frameOrderWidget struct {
	WidgetBase
	trace *[]string
}

func (w *frameOrderWidget) Measure(c layout.Constraint) layout.Measurement {
	*w.trace = append(*w.trace, "measure")
	return layout.Measured(c.Min)
}
func (w *frameOrderWidget) Arrange(r geometry.Rectangle) {
	*w.trace = append(*w.trace, "arrange")
	w.WidgetBase.Arrange(r)
}
func (w *frameOrderWidget) Paint(Painter) { *w.trace = append(*w.trace, "paint") }

func TestWindowFrameUpdatesBeforeLayoutAndReplacesTree(t *testing.T) {
	var trace []string
	w := &window{rootBase: rootBase{painter: new(testGraphicsPainter), width: 100, height: 100}}
	old := newTestWidget()
	w.SetWidget(old)
	newContent := &frameOrderWidget{trace: &trace}
	w.ConnectFrame(func(time.Time) { trace = append(trace, "frame"); w.SetWidget(newContent) })
	w.paint()
	if !reflect.DeepEqual(trace, []string{"frame", "measure", "arrange", "paint"}) || newContent.Root() != w {
		t.Fatalf("frame order: %v", trace)
	}
}

type frameShowPopup struct {
	platform.Popup
	handler platform.EventHandler
}

func (p *frameShowPopup) Show() error {
	p.handler(events.PaintEvent{}) // Windows UpdateWindow during native Show.
	return p.Popup.Show()
}

type frameShowPlatform struct{ *placementPlatform }

func (p *frameShowPlatform) NewPopup(w platform.Window, size geometry.Size, handler platform.EventHandler, options platform.PopupOptions) (platform.Popup, error) {
	native, err := p.placementPlatform.NewPopup(w, size, handler, options)
	return &frameShowPopup{Popup: native, handler: handler}, err
}

func TestPopoverFirstShowPausesSynchronousPaint(t *testing.T) {
	p, _, plat, app := placementFixture(t, geometry.Rect(0, 0, 1000, 1000))
	app.platform = &frameShowPlatform{plat}
	p.anchor.Root().(*window).app = app
	frames := 0
	p.ConnectFrame(func(time.Time) {
		if !p.Visible() {
			t.Fatal("frame before Show committed visibility")
		}
		frames++
	})
	for range 2 { // First presentation and reopening must have identical order.
		before := frames
		if err := p.Show(); err != nil {
			t.Fatal(err)
		}
		if frames != before || !p.layoutDirty || p.app != app {
			t.Fatal("native Show consumed initial layout/frame or lost owner application")
		}
		p.paint()
		if frames != before+1 || p.layoutDirty {
			t.Fatal("visible frame did not lay out")
		}
		p.Hide()
	}
}
