package gui

import (
	"image/color"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/style"
)

// 阴影只负责装饰；工作区边缘不能把仍可容纳的主体推离锚点。
func TestPopoverShadowDoesNotDisplaceBody(t *testing.T) {
	for _, tc := range []struct {
		name      string
		anchor    geometry.Rectangle
		placement PopoverPlacement
		want      geometry.Point
	}{
		{"point left", geometry.Rect(0, 40, 0, 0), PopoverPlacementPoint, geometry.Point{X: 0, Y: 40}},
		{"point right", geometry.Rect(100, 40, 0, 0), PopoverPlacementPoint, geometry.Point{X: 70, Y: 40}},
		{"point top", geometry.Rect(40, 0, 0, 0), PopoverPlacementPoint, geometry.Point{X: 40, Y: 0}},
		{"point bottom", geometry.Rect(40, 100, 0, 0), PopoverPlacementPoint, geometry.Point{X: 40, Y: 80}},
		{"below left", geometry.Rect(0, 10, 20, 10), PopoverPlacementBottom, geometry.Point{X: 0, Y: 20}},
		{"below right", geometry.Rect(80, 10, 20, 10), PopoverPlacementBottom, geometry.Point{X: 70, Y: 20}},
		{"below shadow clipped", geometry.Rect(40, 60, 20, 10), PopoverPlacementBottom, geometry.Point{X: 40, Y: 70}},
		{"above body", geometry.Rect(40, 85, 20, 10), PopoverPlacementBottom, geometry.Point{X: 40, Y: 65}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, _, _, app := placementFixture(t, geometry.Rect(0, 0, 100, 100))
			p.transparent = true
			app.style = style.Sheet(style.Name("popover").Shadow(style.Shadow{Color: color.Black, BlurRadius: 10}))
			p.anchor.Arrange(tc.anchor)
			p.SetPlacement(tc.placement)
			clicks := 0
			click := NewClickEventController()
			click.ConnectClicked(func(EventContext, ClickInfo) { clicks++ })
			p.Widget().AddEventController(click)
			if err := p.Show(); err != nil {
				t.Fatal(err)
			}
			settlePlacement(t, p)
			body := p.bodyRect()
			got := geometry.Point{X: p.requestedPosition.X + body.X, Y: p.requestedPosition.Y + body.Y}
			if got != tc.want || body.Size != (geometry.Size{Width: 30, Height: 20}) {
				t.Fatalf("主体位置/大小 got=%+v/%+v want=%+v/30x20", got, body.Size, tc.want)
			}
			if p.requestedPosition.X < 0 || p.requestedPosition.Y < 0 || p.requestedPosition.X+p.width > 100 || p.requestedPosition.Y+p.height > 100 {
				t.Fatal("不可见阴影仍占用工作区外的原生表面", p.requestedPosition, p.width, p.height)
			}
			if p.Widget().Snapshot().Bounds != body {
				t.Fatal("裁剪阴影后语义边界与主体不一致", p.Widget().Snapshot().Bounds, body)
			}
			for _, typ := range []events.EventType{events.PointerDown, events.PointerUp} {
				if err := p.DispatchEvent(events.PointerEvent{EventType: typ, Button: events.PointerButtonLeft, Position: body.Center()}); err != nil {
					t.Fatal(err)
				}
			}
			if clicks != 1 {
				t.Fatal("阴影裁剪改变了主体正常输入路径的命中", clicks)
			}
		})
	}
}

func TestPopoverShadowClippingUsesCurrentWorkArea(t *testing.T) {
	p, w, plat, app := placementFixture(t, geometry.Rect(-40, -20, 100, 100))
	p.transparent = true
	app.style = style.Sheet(style.Name("popover").Shadow(style.Shadow{Color: color.Black, BlurRadius: 10}))
	p.anchor.Arrange(geometry.Rect(-40, 0, 20, 10))
	p.SetPlacement(PopoverPlacementBottom)
	for _, left := range []float32{-40, -80, -40} {
		w.area.X = left
		if err := p.Show(); err != nil {
			t.Fatal(err)
		}
		settlePlacement(t, p)
		wantInset := float32(16)
		if left == -40 {
			wantInset = 0
		}
		if p.insets.left != wantInset || p.requestedPosition.X+p.bodyRect().X != -40 {
			t.Fatal("复用资源重开沿用了先前的阴影裁剪/退避", left, p.insets, p.requestedPosition)
		}
		p.Hide()
	}
	if len(plat.natives) != 1 {
		t.Fatal("工作区变化重建了原生表面", len(plat.natives))
	}
}

func TestPopoverClippedShadowNativeRounding(t *testing.T) {
	p, _, plat, app := placementFixture(t, geometry.Rect(0, 0, 100, 100))
	p.transparent = true
	app.style = style.Sheet(style.Name("popover").Shadow(style.Shadow{Color: color.Black, BlurRadius: 10}))
	p.anchor.Arrange(geometry.Rect(0, 10.25, 20, 10.5))
	p.SetPlacement(PopoverPlacementBottom)
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	settlePlacement(t, p)
	n := plat.natives[0]
	requests := len(n.sizes)
	p.onEvent(events.SizeEvent{Width: p.width + .5, Height: p.height + .5, PixelWidth: p.width + .5, PixelHeight: p.height + .5})
	settlePlacement(t, p)
	if p.requestedPosition.X+p.bodyRect().X != 0 || p.requestedPosition.Y+p.bodyRect().Y != 20.75 || len(n.sizes) != requests {
		t.Fatal("原生取整改变主体贴边或引发反复调整尺寸", p.requestedPosition, p.bodyRect(), n.sizes)
	}
}
