package gui

import (
	"fmt"
	"testing"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
)

type chromeLayoutBody struct {
	WidgetBase
	arranges  int
	again     bool
	onArrange func()
}

func (b *chromeLayoutBody) Arrange(rect geometry.Rectangle) {
	b.arranges++
	b.WidgetBase.Arrange(rect)
	if b.onArrange != nil {
		b.onArrange()
	}
	if b.again {
		b.RequestLayout()
	}
}

func pendingChromeLayoutFixture(t *testing.T, nativeHit bool) (*window, *desktopTestWindow, *HeaderBar, *chromeLayoutBody) {
	t.Helper()
	win, native := chromeFixture(t, false, nativeHit)
	header := NewHeaderBar()
	body := new(chromeLayoutBody)
	body.SetMinSize(geometry.Size{Height: 100})
	root := NewLinearBox(layout.DirectionVertical)
	root.SetCrossAlign(layout.CrossStretch)
	root.AddChild(header)
	root.AddChild(body)
	win.SetWidget(root)
	win.paint()
	return win, native, header, body
}

// 下一帧的布局请求不能禁用已经完成布局的标题栏；命中不能偷偷执行布局。
func TestChromePendingBodyLayoutPreservesCaptionInput(t *testing.T) {
	for _, nativeHit := range []bool{false, true} {
		for _, when := range []string{"before-press", "during-press", "arrange", "before-paint", "focus-callback"} {
			t.Run(fmt.Sprintf("native-hit=%t/%s", nativeHit, when), func(t *testing.T) {
				win, native, header, body := pendingChromeLayoutFixture(t, nativeHit)
				point := geometry.Point{X: 100, Y: header.windowRect().Center().Y}
				before := header.Rect()
				switch when {
				case "before-press":
					body.SetMinSize(geometry.Size{Height: 140})
				case "arrange":
					body.again = true
					body.RequestLayout()
					win.paint()
				case "before-paint":
					win.painter = new(testGraphicsPainter)
					win.ConnectBeforePaint(body.RequestLayout)
					win.paint()
				case "focus-callback":
					win.ConnectFocus(func(bool) { body.RequestLayout() })
					_ = win.DispatchEvent(events.FocusEvent{Focused: true})
				}
				if when != "during-press" && !win.layoutDirty {
					t.Fatal("复现输入前没有待布局请求")
				}
				arranges := body.arranges
				if nativeHit {
					if when == "during-press" {
						body.RequestLayout()
					}
					if got := native.hitTest(point); got != platform.WindowHitCaption {
						t.Fatalf("正文待布局时的原生命中=%v，期望 Caption", got)
					}
				} else {
					_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Button: events.PointerButtonLeft, Buttons: events.PointerButtonLeftDown, Position: point})
					if when == "during-press" {
						body.RequestLayout()
					}
					moved := geometry.Point{X: point.X + 24, Y: point.Y}
					_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Buttons: events.PointerButtonLeftDown, Position: moved})
					_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerUp, Button: events.PointerButtonLeft, Position: moved})
					if native.moveRequests != 1 || win.dispatcher.gesture != nil {
						t.Fatalf("正文待布局阻断拖动：move=%d gesture=%v", native.moveRequests, win.dispatcher.gesture)
					}
				}
				if !win.layoutDirty || body.arranges != arranges || header.Rect() != before {
					t.Fatal("输入消费了布局请求、同步布局或改变了标题栏几何")
				}
			})
		}
	}
}

// 真正失效的几何在下一次布局完成前仍不可使用；不是简单删除 dirty 检查。
func TestChromeGeometryInvalidationStillBlocksInput(t *testing.T) {
	for _, nativeHit := range []bool{false, true} {
		for _, change := range []string{"new-root", "size", "state", "native-controls"} {
			t.Run(fmt.Sprintf("native-hit=%t/%s", nativeHit, change), func(t *testing.T) {
				win, native, header, _ := pendingChromeLayoutFixture(t, nativeHit)
				point := geometry.Point{X: 100, Y: header.windowRect().Center().Y}
				switch change {
				case "new-root":
					replacement := NewHeaderBar()
					win.SetWidget(replacement)
					// 模拟复用控件已有的非空 Rect，不能将旧分配当作本窗口的新布局。
					replacement.Arrange(geometry.Rect(0, 0, 640, 40))
				case "size":
					_ = win.DispatchEvent(events.SizeEvent{Width: 700, Height: 400, PixelWidth: 700, PixelHeight: 400})
				case "state":
					native.state = WindowStateMaximized
					_ = win.DispatchEvent(events.StateEvent{State: WindowStateMaximized})
				case "native-controls":
					win.chrome.info.Controls = ChromeControlsNative
					native.controls = geometry.Rect(12, 12, 56, 14)
					win.chrome.focusChanged()
				}
				if nativeHit {
					if got := native.hitTest(point); got != platform.WindowHitDefault {
						t.Fatalf("失效几何仍返回原生命中：%v", got)
					}
				} else {
					dragCaptionAt(win, point)
					if native.moveRequests != 0 {
						t.Fatal("失效几何仍发起移动")
					}
				}
				win.paint()
				if nativeHit {
					if got := native.hitTest(point); got != platform.WindowHitCaption {
						t.Fatalf("布局完成后未恢复命中：%v", got)
					}
				} else {
					dragCaptionAt(win, point)
					if native.moveRequests != 1 {
						t.Fatal("布局完成后未恢复移动")
					}
				}
			})
		}
	}
}

// 无绘制帧的持续请求及双击历史也须保留，不只放行第一次按下。
func TestChromeRepeatedPendingLayoutCaptionGestures(t *testing.T) {
	win, native, header, body := pendingChromeLayoutFixture(t, false)
	point := geometry.Point{X: 100, Y: header.windowRect().Center().Y}
	now := time.Unix(100, 0)
	win.dispatcher.hostController.(*chromeInteractionController).now = func() time.Time { return now }
	for i := 0; i < 3; i++ {
		body.RequestLayout()
		dragCaptionAt(win, point)
	}
	if native.moveRequests != 3 {
		t.Fatalf("持续布局请求只允许了 %d 次移动", native.moveRequests)
	}
	body.RequestLayout()
	clickAt(win, point)
	now = now.Add(100 * time.Millisecond)
	body.RequestLayout()
	clickAt(win, point)
	if len(native.requests) != 1 || native.requests[0] != WindowStateMaximized {
		t.Fatalf("普通布局请求清除了标题栏双击历史：%v", native.requests)
	}
}

// 实际 Arrange 和 ChromeControls 查询期间仍不暴露部分更新的布局。
func TestChromePartialLayoutCannotHitCaption(t *testing.T) {
	win, native, header, body := pendingChromeLayoutFixture(t, true)
	point := geometry.Point{X: 100, Y: header.windowRect().Center().Y}
	checked := 0
	check := func() {
		checked++
		if got := native.hitTest(point); got != platform.WindowHitDefault {
			t.Fatalf("部分布局期间暴露命中：%v", got)
		}
	}
	body.onArrange = check
	body.again = true
	win.Chrome().ConnectQueryControls(func(*ChromeControls) { check() })
	body.RequestLayout()
	win.paint()
	if checked < 2 || !win.layoutDirty || native.hitTest(point) != platform.WindowHitCaption {
		t.Fatal("未覆盖布局重入，或未在完整布局后恢复命中")
	}
}

// 应用回调改变真正的几何时必须丢弃本次答案，普通 RequestLayout 则继续有效。
func TestChromeQueryRejectsInvalidatedGeometry(t *testing.T) {
	for _, change := range []string{"layout-request", "root", "size", "controls", "destroy"} {
		t.Run(change, func(t *testing.T) {
			win, native, header, body := pendingChromeLayoutFixture(t, true)
			point := geometry.Point{X: 100, Y: header.windowRect().Center().Y}
			query := native.hitTest
			win.Chrome().ConnectQueryRegion(func(_ geometry.Point, result *ChromeRegion) {
				switch change {
				case "layout-request":
					body.RequestLayout()
				case "root":
					win.SetWidget(NewHeaderBar())
				case "size":
					_ = win.DispatchEvent(events.SizeEvent{Width: 700, Height: 400, PixelWidth: 700, PixelHeight: 400})
				case "controls":
					win.chrome.info.Controls = ChromeControlsNative
					native.controls = geometry.Rect(12, 12, 56, 14)
					win.chrome.focusChanged()
				case "destroy":
					win.Destroy()
				}
				*result = ChromeRegionCaption
			})
			want := platform.WindowHitDefault
			if change == "layout-request" {
				want = platform.WindowHitCaption
			}
			if got := query(point); got != want {
				t.Fatalf("查询回调后命中=%v，期望 %v", got, want)
			}
		})
	}
}

// 控件重新挂载后即使保留非空 Rect，也必须经过本次挂载的 Arrange。
func TestChromeReattachedHeaderRequiresArrange(t *testing.T) {
	win, native, header, body := pendingChromeLayoutFixture(t, true)
	root := win.Widget().(*LinearBox)
	point := geometry.Point{X: 100, Y: header.windowRect().Center().Y}
	root.RemoveChild(body)
	root.RemoveChild(header)
	root.AddChild(header)
	root.AddChild(body)
	if got := native.hitTest(point); got != platform.WindowHitDefault {
		t.Fatalf("重新挂载的 HeaderBar 使用了旧分配：%v", got)
	}
	win.paint()
	if native.hitTest(point) != platform.WindowHitCaption {
		t.Fatal("重新布局后未恢复 HeaderBar 命中")
	}
}
