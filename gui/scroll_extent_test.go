package gui

import (
	"fmt"
	"math"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/events"
)

// 小数内容/视口的末端必须可达；快照、滚动条、setter、Reveal和滚轮共享同一范围。
func TestScrollViewFractionalEndIsReachable(t *testing.T) {
	for _, virtual := range []bool{false, true} {
		for _, direction := range []ScrollDirection{ScrollBoth, ScrollVertical, ScrollHorizontal} {
			for _, method := range []string{"setter", "reveal", "wheel"} {
				t.Run(fmt.Sprintf("virtual=%t/direction=%v/%s", virtual, direction, method), func(t *testing.T) {
					size := geometry.Size{Width: 200.5, Height: 300.25}
					sv := NewScrollView()
					sv.SetDirection(direction)
					if virtual {
						sv.SetChild(&extentScrollContent{size: size})
					} else {
						sv.SetChild(&mockWidget{size: size})
					}
					win := testWindowWithSurface(&window{})
					win.width, win.height = 99.25, 100.75
					win.SetWidget(sv)
					defer win.SetWidget(nil)
					win.layoutContent()
					switch method {
					case "setter":
						sv.SetScrollX(1000)
						sv.SetScrollY(1000)
					case "reveal":
						sv.ScrollIntoView(geometry.Rect(size.Width-1, size.Height-1, 1, 1))
					case "wheel":
						if err := win.DispatchEvent(events.WheelEvent{Position: geometry.Point{X: 10, Y: 10}, DeltaX: 1000, DeltaY: 1000, Mode: events.WheelDeltaPixel}); err != nil {
							t.Fatal(err)
						}
					}
					area, info := sv.contentAreaSize(), sv.Snapshot()
					for _, axis := range []struct {
						enabled                                               bool
						content, viewport, offset, reported, barMax, barValue float32
					}{
						{sv.canScrollX(), sv.contentWidth, area.Width, sv.ScrollX(), info.MaxScrollX, sv.hbar.max, sv.hbar.value},
						{sv.canScrollY(), sv.contentHeight, area.Height, sv.ScrollY(), info.MaxScrollY, sv.vbar.max, sv.vbar.value},
					} {
						if !axis.enabled {
							if axis.offset != 0 || axis.reported != 0 {
								t.Fatal("禁用轴仍可滚动")
							}
							continue
						}
						end := float64(axis.offset) + float64(axis.viewport)
						if end < float64(axis.content) || end-float64(axis.content) >= 1 || axis.offset != float32(math.Trunc(float64(axis.offset))) || axis.offset != axis.reported || axis.offset != axis.barMax || axis.offset != axis.barValue {
							t.Fatalf("末端被裁切、过量留白或范围不一致：%+v", axis)
						}
					}
				})
			}
		}
	}
}

func TestScrollViewSubDIPOverflowRemainsReachable(t *testing.T) {
	sv := NewScrollView()
	sv.SetDirection(ScrollVertical)
	sv.SetChild(&mockWidget{size: geometry.Size{Width: 20, Height: 100.25}})
	sv.Arrange(geometry.Rect(0, 0, 80, 100))
	sv.SetScrollY(1000)
	if sv.ScrollY() != 1 || sv.Snapshot().MaxScrollY != 1 || sv.vbar.max != 1 {
		t.Fatal("不足1 DIP的溢出仍被向下取整成不可滚动")
	}
}
