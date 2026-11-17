package gui

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/style"
)

type popoverStyleMeasureWidget struct {
	WidgetBase
	measures int
}

func (w *popoverStyleMeasureWidget) Measure(c layout.Constraint) layout.Measurement {
	w.measures++
	size, _ := WidgetStyle(w, "popover-invalidation-content", style.PartDefault).FontSize()
	return layout.Measured(c.Clamp(geometry.Size{Width: 4 * size, Height: 2 * size}))
}

func popoverInvalidationSheet(size float32) style.StyleSheet {
	return style.Sheet(style.Name("popover-invalidation-content").FontSize(size))
}

func capturePopoverLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(previous) })
	return &output
}

func TestHiddenPopoverDefersStyleLayout(t *testing.T) {
	for _, state := range []string{
		"ordinary-retained", "ordinary-detached", "ordinary-no-anchor", "ordinary-other-surface",
		"shared-retained", "shared-evicted", "shared-detached",
	} {
		t.Run(state, func(t *testing.T) {
			w, plat, makeClient := transientFixture(t)
			App.SetStyleSheet(popoverInvalidationSheet(10))
			anchor := w.Widget()
			anchorRect := anchor.Rect()
			content := new(popoverStyleMeasureWidget)
			var client *sharedPopover
			var p *popover
			shared := strings.HasPrefix(state, "shared-")
			if shared {
				client = makeClient(anchor)
				client.SetWidget(content)
				if err := client.Show(); err != nil {
					t.Fatal(err)
				}
				p = w.shared.popup
			} else {
				p = NewPopover(anchor, nil).(*popover)
				t.Cleanup(p.Destroy)
				p.SetWidget(content)
				if err := p.Show(); err != nil {
					t.Fatal(err)
				}
			}
			p.Hide()
			restore := func() {}
			queries := []*placementWindow{w.platformWindow.(*placementWindow)}
			migrated := false
			switch state {
			case "ordinary-detached", "shared-detached":
				w.SetWidget(nil)
				restore = func() { w.SetWidget(anchor); anchor.Arrange(anchorRect) }
			case "ordinary-no-anchor":
				p.SetAnchor(nil)
				restore = func() { p.SetAnchor(anchor) }
			case "ordinary-other-surface":
				native := &placementWindow{area: geometry.Rect(0, 0, 1000, 800)}
				other := testWindowWithSurface(&window{surfaceBase: surfaceBase{platformWindow: native}})
				t.Cleanup(other.Destroy)
				otherAnchor := newTestWidget()
				other.SetWidget(otherAnchor)
				otherAnchor.Arrange(geometry.Rect(40, 50, 80, 30))
				p.SetAnchor(otherAnchor)
				queries = append(queries, native)
				migrated = true
			case "shared-evicted":
				client.Destroy()
			}
			native, painter := p.platformPopup, p.painter
			beforeMeasures := content.measures
			beforeQueries := 0
			for _, host := range queries {
				beforeQueries += len(host.points)
			}
			beforeSizes, beforePositions, beforePaints := len(plat.natives[0].sizes), len(plat.natives[0].positions), len(plat.painters[0].calls)
			output := capturePopoverLog(t)
			App.SetStyleSheet(popoverInvalidationSheet(20))
			afterQueries := 0
			for _, host := range queries {
				afterQueries += len(host.points)
			}
			if output.Len() != 0 || content.measures != beforeMeasures || afterQueries != beforeQueries {
				t.Fatalf("隐藏样式更新不应定位或测量：queries=%d→%d measures=%d→%d log=%s", beforeQueries, afterQueries, beforeMeasures, content.measures, output)
			}
			if p.Visible() || !p.layoutDirty || p.platformPopup != native || p.painter != painter || plat.natives[0].destroyed ||
				len(plat.natives[0].sizes) != beforeSizes || len(plat.natives[0].positions) != beforePositions || len(plat.painters[0].calls) != beforePaints {
				t.Fatal("隐藏样式更新操作了原生资源，或未保留待布局状态")
			}
			restore()
			if shared {
				if client.destroyed {
					client = makeClient(anchor)
					client.SetWidget(content)
				}
				if err := client.Show(); err != nil {
					t.Fatal(err)
				}
			} else if err := p.Show(); err != nil {
				t.Fatal(err)
			}
			if !p.Visible() || p.requestedSize != (geometry.Size{Width: 80, Height: 40}) || content.Rect().Size != p.bodyRect().Size {
				t.Fatalf("重开未采用新样式布局：requested=%v content=%v", p.requestedSize, content.Rect())
			}
			if migrated {
				if len(plat.natives) != 2 || !plat.natives[0].destroyed || p.platformPopup == native || p.painter == painter {
					t.Fatal("跨 Surface 重开未正确迁移资源")
				}
			} else if len(plat.natives) != 1 || p.platformPopup != native || p.painter != painter {
				t.Fatal("同 Surface 重开未复用原生资源")
			}
		})
	}
}

func TestHiddenPopoverSizeEventDefersPosition(t *testing.T) {
	p, _, plat, _, _ := ordinaryReuseFixture(t)
	p.SetPosition(geometry.Point{X: 900, Y: 650})
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	p.Hide()
	p.SetPosition(geometry.Point{X: 20, Y: 30})
	native := plat.natives[0]
	beforePositions, beforePaints := len(native.positions), native.paints
	native.handler(events.SizeEvent{Width: 200, Height: 100, PixelWidth: 300, PixelHeight: 150})
	if p.width != 200 || p.height != 100 || p.pixelWidth != 300 || p.pixelHeight != 150 || !p.layoutDirty {
		t.Fatal("隐藏时没有记录原生尺寸和待布局状态")
	}
	if len(native.positions) != beforePositions || native.paints != beforePaints {
		t.Fatal("隐藏尺寸通知使用旧定位快照移动或绘制了原生窗口")
	}
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	if p.requestedPosition != (geometry.Point{X: 30, Y: 50}) {
		t.Fatal("重开未按当前锚点和请求位置定位", p.requestedPosition)
	}
}

func TestVisiblePopoverStyleLayoutPreservesQueryErrors(t *testing.T) {
	p, w, _, _, _ := ordinaryReuseFixture(t)
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	query := w.platformWindow.(*placementWindow)
	query.err = platform.ErrUnavailable
	before := len(query.points)
	output := capturePopoverLog(t)
	App.SetStyleSheet(popoverInvalidationSheet(20))
	if !p.Visible() || len(query.points) <= before || !strings.Contains(output.String(), "goui: popup layout: popover work area:") {
		t.Fatalf("可见状态未保留真实工作区查询失败：visible=%t queries=%d→%d log=%s", p.Visible(), before, len(query.points), output)
	}
}

func TestHiddenPopoverConfigurationDefersNativeWork(t *testing.T) {
	p, w, plat, a, _ := ordinaryReuseFixture(t)
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	w.Widget().(*testWidget).RemoveChild(a)
	native, painter := p.platformPopup, p.painter
	query := w.platformWindow.(*placementWindow)
	beforeQueries, beforePaints := len(query.points), len(plat.painters[0].calls)
	beforeSizes, beforePositions := len(plat.natives[0].sizes), len(plat.natives[0].positions)
	output := capturePopoverLog(t)
	content := new(popoverStyleMeasureWidget)
	p.SetWidget(content)
	p.SetStyleName("updated-popover")
	p.SetPosition(geometry.Point{X: 50, Y: 60})
	p.SetPlacement(PopoverPlacementBottom)
	content.RequestLayout()
	content.RequestPaint()
	App.SetStyleSheet(popoverInvalidationSheet(20))
	if output.Len() != 0 || content.measures != 0 || len(query.points) != beforeQueries || len(plat.painters[0].calls) != beforePaints ||
		len(plat.natives[0].sizes) != beforeSizes || len(plat.natives[0].positions) != beforePositions {
		t.Fatal("隐藏配置或内容失效提前执行了原生工作", output)
	}
	w.Widget().(*testWidget).AddChild(a)
	a.Arrange(geometry.Rect(70, 80, 50, 30))
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	if p.platformPopup != native || p.painter != painter || content.Rect().Size != (geometry.Size{Width: 80, Height: 40}) ||
		p.requestedPosition != (geometry.Point{X: 70, Y: 110}) || p.StyleName() != "updated-popover" {
		t.Fatal("重新挂载未应用隐藏期间的内容和配置")
	}
}

func TestVisiblePopoverStyleUpdateResizesRetainedResources(t *testing.T) {
	w, plat, makeClient := transientFixture(t)
	App.SetStyleSheet(popoverInvalidationSheet(10))
	client := makeClient(nil)
	content := new(popoverStyleMeasureWidget)
	client.SetWidget(content)
	if err := client.Show(); err != nil {
		t.Fatal(err)
	}
	p := w.shared.popup
	native, painter := p.platformPopup, p.painter
	output := capturePopoverLog(t)
	App.SetStyleSheet(popoverInvalidationSheet(20))
	p.paint()
	if output.Len() != 0 || !client.Visible() || p.platformPopup != native || p.painter != painter || len(plat.natives) != 1 ||
		p.requestedSize != (geometry.Size{Width: 80, Height: 40}) || content.Rect().Size != p.bodyRect().Size {
		t.Fatal("可见样式更新未重新布局，或改变了资源归属", output)
	}
}
