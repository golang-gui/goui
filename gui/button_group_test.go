package gui

import (
	"fmt"
	"image/color"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/style"
)

func TestButtonGroupLayoutAndMembership(t *testing.T) {
	group := NewButtonGroup()
	children := []Widget{NewButton(), NewMenuButton(), NewButton()}
	for _, child := range children {
		child.SetMinSize(geometry.Size{Width: 40, Height: 40})
		group.AddChild(child)
	}
	if got := group.Measure(layout.Unbounded()).Size; got != (geometry.Size{Width: 120, Height: 40}) {
		t.Fatal("没有按子项自然尺寸测量", got)
	}
	group.Arrange(geometry.Rect(0, 0, 120, 50))
	for i, child := range children {
		if child.Parent() != group || child.Rect() != geometry.Rect(float32(i)*40, 0, 40, 50) {
			t.Fatal("没有零间距连排或交叉轴拉伸", child.Rect())
		}
	}
	children[1].SetVisible(false)
	group.MoveChildBefore(children[2], children[0])
	group.SetDirection(layout.DirectionVertical)
	group.Arrange(geometry.Rect(0, 0, 70, 80))
	if children[2].Rect() != geometry.Rect(0, 0, 70, 40) || children[0].Rect() != geometry.Rect(0, 40, 70, 40) {
		t.Fatal("隐藏或重排后布局错误")
	}
	corners, edge, ok := buttonSegment(children[2])
	if !ok || corners != buttonTopLeft|buttonTopRight || edge != 2 {
		t.Fatal("垂直首项几何错误", corners, edge)
	}
	group.RemoveChild(children[0])
	corners, edge, ok = buttonSegment(children[2])
	if !ok || corners != buttonAllCorners || edge != -1 {
		t.Fatal("单项没有完整轮廓")
	}
	if children[0].base().destroyed || children[0].Parent() != nil {
		t.Fatal("移除错误销毁了子项")
	}
	info := group.Snapshot()
	if info.Role != RoleVBox || info.Focusable || len(info.Actions) != 0 || len(info.Children) != 2 || info.Children[0].Role != RoleButton {
		t.Fatal("容器语义或子树错误", info)
	}
}

// 像素是预乘 sRGB RGBA。中段整数对齐的直线不含部分覆盖，逐字节精确断言；
// 圆角只采样确定在轮廓外的角点，不比较后端抗锯齿曲线。
func TestButtonGroupPixels(t *testing.T) {
	for _, vertical := range []bool{false, true} {
		for _, scale := range []float32{1, 2} {
			t.Run(fmt.Sprintf("vertical=%v/%gx", vertical, scale), func(t *testing.T) {
				setTestApplication(t, nil)
				backend, err := software.NewPainter(&widgetRenderSurface{})
				if err != nil {
					t.Fatal(err)
				}
				defer backend.Destroy()
				bg, line := color.RGBA{R: 40, G: 100, B: 150, A: 255}, color.RGBA{A: 255}
				scope := NewStyleScope()
				scope.SetStyleSheet(style.Sheet(style.Name("button").BackgroundColor(bg).BorderColor(line).BorderWidth(1).Radius(8), style.Name("button").Part("focus").BorderColor(color.RGBA{R: 255, A: 255}).BorderWidth(2).Radius(8)))
				group := NewButtonGroup()
				scope.SetChild(group)
				win := &window{rootBase: rootBase{painter: backend}}
				win.SetWidget(scope)
				defer win.Destroy()
				for range 3 {
					button := NewButton()
					button.SetMinSize(geometry.Size{Width: 40, Height: 30})
					group.AddChild(button)
				}
				size := geometry.Size{Width: 120, Height: 30}
				if vertical {
					group.SetDirection(layout.DirectionVertical)
					size = geometry.Size{Width: 40, Height: 90}
				}
				group.Arrange(geometry.Rectangle{Size: size})
				img, err := RenderWidget(group, scale)
				if err != nil {
					t.Fatal(err)
				}
				pixel := func(x, y int, want color.RGBA) {
					t.Helper()
					got := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
					if got != want {
						t.Fatalf("像素(%d,%d)=%v，预期%v", x, y, got, want)
					}
				}
				n := int(scale)
				pixel(0, 0, color.RGBA{})
				pixel(int(size.Width)*n-1, int(size.Height)*n-1, color.RGBA{})
				// 前项不画尾边，后项只拥有 1 DIP 的前边；每条接缝两侧都是背景。
				for _, seam := range []int{1, 2} {
					for depth := -2 * n; depth <= 2*n; depth++ {
						want := bg
						if depth >= 0 && depth < n {
							want = line
						}
						if vertical {
							pixel(20*n, seam*30*n+depth, want)
						} else {
							pixel(seam*40*n+depth, 15*n, want)
						}
					}
				}
				// 内侧角为直角；外侧角保留圆角。
				if vertical {
					pixel(3*n, 29*n, bg)
				} else {
					pixel(39*n, 3*n, bg)
				}
				if !vertical {
					_ = win.DispatchEvent(shortcutPress(events.KeyTab, 0))
					if !group.Children()[0].FocusVisible() {
						t.Fatal("焦点没有落到首按钮")
					}
					img, err = RenderWidget(group, scale)
					if err != nil {
						t.Fatal(err)
					}
					// 焦点必须补齐首项右侧边，不受共享接缝省略影响。
					pixel(39*n, 15*n, color.RGBA{R: 255, A: 255})
					pixel(37*n, 15*n, bg)
				}
			})
		}
	}
}
