package main

import (
	"bytes"
	"image/color"
	"io"
	"log"
	"sync"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
)

// 独立预期由样式字号确定，避免字体栅格差异影响几何断言。
type invalidationContent struct {
	gui.WidgetBase
	measures int
}

func (w *invalidationContent) Measure(c layout.Constraint) layout.Measurement {
	w.measures++
	size, _ := gui.WidgetStyle(w, "invalidation-content", style.PartDefault).FontSize()
	return layout.Measured(c.Clamp(geometry.Size{Width: size * 4, Height: size * 2}))
}

func (w *invalidationContent) Paint(p gui.Painter) {
	gui.PaintStyledBox(p, geometry.Rectangle{Size: w.Rect().Size}, gui.WidgetStyle(w, "invalidation-content", style.PartDefault))
}

type popupLayoutLog struct {
	sync.Mutex
	errors int
}

func (l *popupLayoutLog) Write(p []byte) (int, error) {
	l.Lock()
	defer l.Unlock()
	if bytes.Contains(p, []byte("goui: popup layout:")) {
		l.errors++
	}
	return len(p), nil
}

func (l *popupLayoutLog) count() int {
	l.Lock()
	defer l.Unlock()
	return l.errors
}

func startInvalidationVerification(app gui.Application, win gui.Window, ordinary gui.Popover, a, b gui.Widget) {
	previousLog := log.Writer()
	logs := new(popupLayoutLog)
	log.SetOutput(io.MultiWriter(previousLog, logs))
	win.ConnectDestroy(func() { log.SetOutput(previousLog) })
	setStyle := func(size float32) {
		rules := modern.Rules(modern.Options{})
		rules = append(rules, style.Name("invalidation-content").FontSize(size).BackgroundColor(color.NRGBA{R: 64, G: 128, B: 192, A: 255}))
		app.SetStyleSheet(style.Sheet(rules...))
	}
	setStyle(10)
	ordinary.SetAnchor(a)
	ordinary.SetPlacement(gui.PopoverPlacementBottom)
	shared := gui.NewSharedPopover(b)
	shared.SetPlacement(gui.PopoverPlacementBottom)
	x, y := new(invalidationContent), new(invalidationContent)
	ordinary.SetWidget(x)
	shared.SetWidget(y)
	var sharedRoot gui.Root
	parent := b.Parent().(interface {
		AddChild(gui.Widget)
		RemoveChild(gui.Widget)
	})
	stage, hiddenMeasures, hiddenFrames, frames := 0, 0, 0, 0
	check := func(ok bool, message string) {
		if !ok {
			log.Fatalf("INVALIDATION FAIL stage=%d %s", stage, message)
		}
	}
	checkSize := func(size float32) {
		want := geometry.Size{Width: size * 4, Height: size * 2}
		check(x.Rect().Size == want && y.Rect().Size == want, "内容未按当前样式分配尺寸")
		check(ordinary.Visible() && shared.Visible() && y.Root() == sharedRoot, "重开丢失显示或共享 Root 身份")
	}
	timer := app.NewTimer()
	timer.ConnectTimeout(func() {
		switch stage {
		case 0:
			must(ordinary.Show())
			must(shared.Show())
			sharedRoot = y.Root()
			checkSize(10)
			ordinary.ConnectAfterPaint(func(gui.Painter, geometry.Rectangle) { frames++ })
			sharedRoot.ConnectAfterPaint(func(gui.Painter, geometry.Rectangle) { frames++ })
			setStyle(20)
		case 1:
			checkSize(20)
			ordinary.Hide()
			shared.Hide()
			hiddenMeasures, hiddenFrames = x.measures+y.measures, frames
			setStyle(30)
		case 2:
			check(x.measures+y.measures == hiddenMeasures && frames == hiddenFrames, "隐藏更新仍执行测量或绘制")
			ordinary.SetAnchor(nil)
			shared.Destroy()
			check(sharedRoot.Widget() == nil, "归还客户端仍保留内容")
			setStyle(15)
		case 3:
			check(x.measures+y.measures == hiddenMeasures && frames == hiddenFrames, "空闲缓存更新仍执行测量或绘制")
			ordinary.SetAnchor(b)
			x, y = new(invalidationContent), new(invalidationContent)
			ordinary.SetWidget(x)
			shared = gui.NewSharedPopover(b)
			shared.SetWidget(y)
			must(ordinary.Show())
			must(shared.Show())
			checkSize(15)
		case 4:
			parent.RemoveChild(b)
			check(!ordinary.Visible() && !shared.Visible(), "锚点卸载没有结束显示")
			hiddenMeasures, hiddenFrames = x.measures+y.measures, frames
			setStyle(25)
		case 5:
			check(x.measures+y.measures == hiddenMeasures && frames == hiddenFrames, "卸载锚点更新仍执行测量或绘制")
			parent.AddChild(b)
		case 6:
			must(ordinary.Show())
			must(shared.Show())
			checkSize(25)
		case 7:
			ordinary.Destroy()
			shared.Destroy()
			setStyle(10)
		case 8:
			check(logs.count() == 0, "样式更新产生了 popup layout 错误")
			timer.Stop()
			log.Printf("INVALIDATION PASS stages=%d shared=%p hidden-paint-checks=3", stage+1, sharedRoot)
			win.Destroy()
		}
		check(logs.count() == 0, "样式更新产生了 popup layout 错误")
		stage++
	})
	must(timer.Start(200 * time.Millisecond))
}
