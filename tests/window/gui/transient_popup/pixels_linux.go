//go:build !android

package main

import (
	"log"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"
)

func init() { nativePixelProbe = verifyNativeFirstPixels }

// 同步 Repaint 后、事件循环再次绘制前读取原生窗口，不依赖截图采样碰巧捕获。
// 像素是原生 ZPixmap；仅比较 RGB 掩码，不解释合成器未提供的 alpha。
// 独立预期：两个 8×8 不透明红/绿角标，每个像素的位置均明确，颜色判定阈值
// 200/40 只排除抗锯齿边缘，不放宽几何。丢失、偏移或残留任一角标均失败。
func verifyNativeFirstPixels(popup platform.Popup, pixels geometry.Size) {
	d := xlib.OpenDisplay("")
	if d == 0 {
		log.Fatal("PIXEL FAIL 无法打开只读 X11 连接")
	}
	defer d.Close()
	width, height := int(pixels.Width), int(pixels.Height)
	pic := d.GetImage(xlib.Drawable(popup.NativeHandle()), 0, 0, width, height, ^uint64(0), xlib.ImageFormatZPixmap)
	if pic == nil {
		log.Fatal("PIXEL FAIL XGetImage 没有返回原生像素")
	}
	defer pic.Destroy()
	counts := [2]int{}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixel := pic.Pixel(x, y)
			red := pixel & pic.RedMask
			green := pixel & pic.GreenMask
			blue := pixel & pic.BlueMask
			kind := -1
			if red > pic.RedMask*200/255 && green < pic.GreenMask*40/255 && blue < pic.BlueMask*40/255 {
				kind = 0
			}
			if green > pic.GreenMask*200/255 && red < pic.RedMask*40/255 && blue < pic.BlueMask*40/255 {
				kind = 1
			}
			expected := -1
			if x >= 9 && x < 17 && y >= 11 && y < 19 {
				expected = 0
			}
			if x >= width-19 && x < width-11 && y >= height-23 && y < height-15 {
				expected = 1
			}
			if kind != expected {
				log.Fatalf("PIXEL FAIL alpha=%v size=%dx%d at=(%d,%d) got=%d want=%d rgb=%x", popup.Transparent(), width, height, x, y, kind, expected, pixel)
			}
			if kind >= 0 {
				counts[kind]++
			}
		}
	}
	if counts != [2]int{64, 64} {
		log.Fatalf("PIXEL FAIL counts=%v", counts)
	}
	log.Printf("PIXEL PASS alpha=%v size=%dx%d markers=%v", popup.Transparent(), width, height, counts)
}
