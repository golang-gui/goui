package gui

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"strings"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/software"
)

// 只替换窗口上传与呈现，绘制函数始终使用真实 Software Painter。
type drawingAreaTarget struct {
	Painter
	scale    float32
	uploads  int
	updates  int
	images   []*drawingAreaImage
	draws    []geometry.Rectangle
	fail     bool
	onUpload func()
	onUpdate func()
}
type drawingAreaImage struct {
	owner     *drawingAreaTarget
	pixels    *image.RGBA
	destroyed bool
}

func (i *drawingAreaImage) Size() (int, int) { return i.pixels.Bounds().Dx(), i.pixels.Bounds().Dy() }
func (i *drawingAreaImage) Update(src image.Image) error {
	p := i.owner
	p.updates++
	draw.Draw(i.pixels, i.pixels.Bounds(), src, src.Bounds().Min, draw.Src)
	if p.onUpdate != nil {
		p.onUpdate()
	}
	if p.fail {
		return errors.New("测试更新失败")
	}
	return nil
}
func (i *drawingAreaImage) Destroy()                      { i.destroyed = true }
func (p *drawingAreaTarget) PixelScale() float32          { return p.scale }
func (*drawingAreaTarget) Save()                          {}
func (*drawingAreaTarget) Restore()                       {}
func (*drawingAreaTarget) SetClipRect(geometry.Rectangle) {}
func (p *drawingAreaTarget) NewImage(src image.Image) (graphics.Image, error) {
	p.uploads++
	if p.fail {
		return nil, errors.New("测试上传失败")
	}
	b := src.Bounds()
	img := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(img, img.Bounds(), src, b.Min, draw.Src)
	result := &drawingAreaImage{owner: p, pixels: img}
	p.images = append(p.images, result)
	if p.onUpload != nil {
		p.onUpload()
	}
	return result, nil
}
func (p *drawingAreaTarget) DrawImage(rect geometry.Rectangle, img graphics.Image) {
	if img.(*drawingAreaImage).destroyed {
		panic("绘制已销毁图片")
	}
	p.draws = append(p.draws, rect)
}

func TestDrawingAreaRegenerationAndCache(t *testing.T) {
	calls, queries := 0, 0
	a := NewDrawingArea(func(p Painter, size geometry.Size) {
		calls++
		if p.PixelScale() != 1 && p.PixelScale() != 2 {
			t.Fatal("软件比例错误")
		}
		p.FillRect(geometry.Rectangle{Size: size}, graphics.RGBA(200, 40, 10, 128))
	})
	defer a.releaseDrawing()
	a.Arrange(geometry.Rect(10, 20, 8, 5))
	target := &drawingAreaTarget{scale: 1}
	a.ConnectShouldDraw(func(*bool) { queries++ })
	a.Paint(target)
	raster := a.raster
	a.Paint(target)
	if calls != 1 || queries != 2 || target.uploads != 1 || len(target.draws) != 2 {
		t.Fatal("普通 Paint 重新生成或上传", calls, queries, target.uploads)
	}
	a.Arrange(geometry.Rect(30, 40, 8, 5))
	a.Paint(target)
	if calls != 1 {
		t.Fatal("仅位置变化重新生成")
	}
	a.Arrange(geometry.Rect(30, 40, 10, 5))
	a.Paint(target)
	if calls != 2 || !target.images[0].destroyed || a.raster != raster {
		t.Fatal("尺寸改变没有重画、释放旧图或复用软件 Painter")
	}
	target.scale = 2
	a.Paint(target)
	w, h := target.images[2].Size()
	if calls != 3 || w != 20 || h != 10 {
		t.Fatal("HiDPI 缓存失效错误", calls, w, h)
	}
	a.releaseDrawing()
	if a.raster != nil || a.image != nil || !target.images[2].destroyed {
		t.Fatal("没有释放两类资源")
	}
}

func TestDrawingAreaQueryOrderAndVeto(t *testing.T) {
	calls := 0
	target := &drawingAreaTarget{scale: 1}
	a := NewDrawingArea(func(p Painter, size geometry.Size) {
		calls++
		p.FillRect(geometry.Rectangle{Size: size}, graphics.RGB(255, 0, 0))
	})
	defer a.releaseDrawing()
	a.Arrange(geometry.Rect(0, 0, 6, 4))
	veto := a.ConnectShouldDraw(func(v *bool) { *v = false })
	a.Paint(target)
	if calls != 0 || target.uploads != 0 || len(target.draws) != 0 {
		t.Fatal("首次否决仍绘图")
	}
	force := a.ConnectShouldDraw(func(v *bool) { *v = true })
	a.Paint(target)
	a.Paint(target)
	if calls != 2 || target.uploads != 1 || target.updates != 1 {
		t.Fatal("后连接者未覆盖、按帧重画失败或同尺寸重新创建了图片")
	}
	force.Disconnect()
	a.Arrange(geometry.Rect(0, 0, 12, 8))
	a.Paint(target)
	if calls != 2 || target.draws[len(target.draws)-1] != geometry.Rect(0, 0, 6, 4) {
		t.Fatal("否决后拉伸了旧图")
	}
	veto.Disconnect()
	a.Paint(target)
	if calls != 3 || target.draws[len(target.draws)-1] != geometry.Rect(0, 0, 12, 8) {
		t.Fatal("恢复默认后未更新")
	}
}

func TestDrawingAreaFunctionReplacementAndUploadRetry(t *testing.T) {
	calls := 0
	target := &drawingAreaTarget{scale: 1, fail: true}
	a := NewDrawingArea(func(Painter, geometry.Size) { calls++ })
	defer a.releaseDrawing()
	a.Arrange(geometry.Rect(0, 0, 8, 5))
	a.Paint(target)
	a.Paint(target)
	if calls != 1 || target.uploads != 2 {
		t.Fatal("上传失败重复执行绘图函数")
	}
	target.fail = false
	a.Paint(target)
	if calls != 1 || target.uploads != 3 {
		t.Fatal("没有直接重试已有像素")
	}
	fn := func(Painter, geometry.Size) { calls += 10 }
	a.SetDrawFunc(fn)
	a.Paint(target)
	a.SetDrawFunc(fn)
	a.Paint(target)
	if calls != 21 || target.uploads != 3 || target.updates != 2 {
		t.Fatal("显式 Set 没有重新生成或同尺寸未复用图片", calls, target.uploads, target.updates)
	}
	a.SetDrawFunc(nil)
	a.Paint(target)
	if a.image != nil || a.raster != nil || a.dirty {
		t.Fatal("nil 函数没有清空内容")
	}
}

func TestDrawingAreaCallbackLifecycle(t *testing.T) {
	for _, stage := range []string{"query", "draw", "upload"} {
		t.Run(stage, func(t *testing.T) {
			calls := 0
			target := &drawingAreaTarget{scale: 1}
			var a *DrawingArea
			a = NewDrawingArea(func(Painter, geometry.Size) {
				calls++
				if stage == "draw" {
					a.base().destroy(a)
				}
			})
			win := &window{}
			win.SetWidget(a)
			t.Cleanup(win.Destroy)
			a.Arrange(geometry.Rect(0, 0, 8, 5))
			if stage == "query" {
				a.ConnectShouldDraw(func(*bool) { a.base().destroy(a) })
				a.ConnectShouldDraw(func(*bool) { t.Fatal("销毁后仍通知后续监听者") })
			}
			if stage == "upload" {
				target.onUpload = func() { a.base().destroy(a) }
			}
			a.Paint(target)
			if !a.Destroyed() || a.image != nil || a.raster != nil || len(target.draws) != 0 {
				t.Fatal("销毁后提交了旧结果")
			}
			for _, img := range target.images {
				if !img.destroyed {
					t.Fatal("上传期间销毁泄漏图片")
				}
			}
			if stage == "query" && calls != 0 {
				t.Fatal("查询销毁后仍调用绘图")
			}
		})
	}
	for _, stage := range []string{"query", "draw"} {
		t.Run("replace/"+stage, func(t *testing.T) {
			target := &drawingAreaTarget{scale: 1}
			calls := 0
			var a *DrawingArea
			replacement := func(Painter, geometry.Size) { calls += 10 }
			a = NewDrawingArea(func(Painter, geometry.Size) {
				calls++
				if stage == "draw" {
					a.SetDrawFunc(replacement)
				}
			})
			defer a.releaseDrawing()
			a.Arrange(geometry.Rect(0, 0, 8, 5))
			if stage == "query" {
				a.ConnectShouldDraw(func(*bool) {
					if calls == 0 {
						calls++
						a.SetDrawFunc(replacement)
					}
				})
			}
			a.Paint(target)
			if target.uploads != 0 {
				t.Fatal("函数替换后仍上传过期结果")
			}
			a.Paint(target)
			if calls != 11 || target.uploads != 1 {
				t.Fatal("没有执行最新函数", calls, target.uploads)
			}
		})
	}
}

func TestDrawingAreaUnmountReentryAndPaintRequest(t *testing.T) {
	target := &drawingAreaTarget{scale: 1}
	win := &window{}
	calls := 0
	var a *DrawingArea
	a = NewDrawingArea(func(Painter, geometry.Size) { calls++; a.RequestPaint() })
	win.SetWidget(a)
	defer win.Destroy()
	a.Arrange(geometry.Rect(0, 0, 8, 5))
	a.ConnectShouldDraw(func(*bool) { a.Paint(target) })
	win.paintDirty = false
	a.Paint(target)
	if calls != 1 || !win.paintDirty {
		t.Fatal("重入或后续 RequestPaint 语义错误")
	}
	a.Paint(target)
	if calls != 1 {
		t.Fatal("RequestPaint 隐式使软件缓存失效")
	}
	old := a.image
	win.SetWidget(nil)
	if !old.(*drawingAreaImage).destroyed || a.raster != nil {
		t.Fatal("Unmount 没有释放资源")
	}
	win.SetWidget(a)
	a.Arrange(geometry.Rect(0, 0, 8, 5))
	a.Paint(target)
	if calls != 2 {
		t.Fatal("重挂没有重新生成")
	}
}

func TestDrawingAreaPaintStateAndPanicCleanup(t *testing.T) {
	a := NewDrawingArea(func(p Painter, _ geometry.Size) { p.Save() })
	defer a.releaseDrawing()
	a.Arrange(geometry.Rect(0, 0, 8, 5))
	target := &drawingAreaTarget{scale: 1}
	func() {
		defer func() {
			value := recover()
			if value == nil || !strings.Contains(value.(string), "DrawingArea.Draw") {
				t.Fatal("没有按绘图回调边界检查 Save", value)
			}
		}()
		a.Paint(target)
	}()
	if a.raster != nil || a.painting {
		t.Fatal("异常后仍保留正在绘制的资源")
	}
	a.SetDrawFunc(func(p Painter, _ geometry.Size) { p.Save(); p.SetTransform(geometry.Translate(1, 1)); p.Restore() })
	a.Paint(target)
	if target.uploads != 1 {
		t.Fatal("异常后不能继续使用")
	}
}

// 采样预乘 sRGB RGBA，内部整数边界没有部分覆盖，精确断言；分数末列仅检查
// 透明裁剪和逻辑区域，不用字体或圆角的后端金图。
func TestDrawingAreaPixelsClipScaleAndRetainedSize(t *testing.T) {
	for _, scale := range []float32{1, 1.5, 2} {
		t.Run(fmt.Sprint(scale), func(t *testing.T) {
			setTestApplication(t, nil)
			native, err := software.NewPainter(&widgetRenderSurface{})
			if err != nil {
				t.Fatal(err)
			}
			defer native.Destroy()
			calls := 0
			ink := graphics.RGBA(200, 40, 10, 128)
			a := NewDrawingArea(func(p Painter, size geometry.Size) {
				calls++
				p.FillRect(geometry.Rectangle{Size: size}, ink)
				p.Save()
				p.SetClipRect(geometry.Rect(2, 1, 2, 2))
				p.SetTransform(geometry.Translate(1, 0))
				p.FillRect(geometry.Rect(0, 0, 9, 9), graphics.RGB(0, 0, 255))
				p.Restore()
			})
			win := &window{rootBase: rootBase{painter: native}}
			win.SetWidget(a)
			defer win.SetWidget(nil)
			a.Arrange(geometry.Rect(0, 0, 6, 4))
			img, err := RenderWidget(a, scale)
			if err != nil {
				t.Fatal(err)
			}
			pixel := func(x, y int) color.RGBA { return color.RGBAModel.Convert(img.At(x, y)).(color.RGBA) }
			if got := pixel(0, 0); got != (color.RGBA{R: 100, G: 20, B: 5, A: 128}) {
				t.Fatal("预乘像素错误", scale, got)
			}
			if got := pixel(int(3*scale), int(2*scale)); got != (color.RGBA{B: 255, A: 255}) {
				t.Fatal("局部裁剪／变换错误", scale, got)
			}
			cached := a.image
			ink = graphics.RGBA(20, 160, 80, 128)
			a.SetDrawFunc(a.DrawFunc())
			img, err = RenderWidget(a, scale)
			if err != nil {
				t.Fatal(err)
			}
			if calls != 2 || a.image != cached || pixel(0, 0) != (color.RGBA{R: 10, G: 80, B: 40, A: 128}) || pixel(int(3*scale), int(2*scale)) != (color.RGBA{B: 255, A: 255}) {
				t.Fatal("真实 Software 更新未复用、像素未更新或破坏了 clip／Transform", scale)
			}
			a.ConnectShouldDraw(func(v *bool) { *v = false })
			a.Arrange(geometry.Rect(0, 0, 10, 8))
			img, err = RenderWidget(a, scale)
			if err != nil {
				t.Fatal(err)
			}
			if calls != 2 || pixel(int(8*scale), int(6*scale)) != (color.RGBA{}) {
				t.Fatal("否决尺寸变化后拉伸缓存或未清透明")
			}
		})
	}
	a := NewDrawingArea(nil)
	defer a.releaseDrawing()
	a.SetMinSize(geometry.Size{Width: 10, Height: 12})
	if a.Measure(layout.Unbounded()).Size != (geometry.Size{Width: 10, Height: 12}) {
		t.Fatal("没有服从布局约束")
	}
	info := a.Snapshot()
	if info.Role != RoleWidget || info.Focusable || len(info.Actions) != 0 {
		t.Fatal("伪造了焦点或交互语义")
	}
}

func TestDrawingAreaFractionalRasterExtent(t *testing.T) {
	a := NewDrawingArea(func(p Painter, size geometry.Size) {
		if size != (geometry.Size{Width: 6.25, Height: 2.2}) || p.PixelScale() != 1.5 {
			t.Fatal("回调不是局部 DIP／实际比例")
		}
	})
	defer a.releaseDrawing()
	a.Arrange(geometry.Rect(0, 0, 6.25, 2.2))
	target := &drawingAreaTarget{scale: 1.5}
	a.Paint(target)
	w, h := target.images[0].Size()
	if w != 10 || h != 4 || target.draws[0].Size != (geometry.Size{Width: 10 / float32(1.5), Height: 4 / float32(1.5)}) {
		t.Fatal("补齐像素被压缩到逻辑区域", w, h, target.draws)
	}
}

func TestDrawingAreaUpdatesSameSizeImage(t *testing.T) {
	calls := 0
	ink := graphics.RGBA(200, 40, 10, 128)
	a := NewDrawingArea(func(p Painter, size geometry.Size) {
		calls++
		p.FillRect(geometry.Rectangle{Size: size}, ink)
	})
	defer a.releaseDrawing()
	a.Arrange(geometry.Rect(0, 0, 8, 6))
	target := &drawingAreaTarget{scale: 1}
	a.Paint(target)
	cached, raster := a.image, a.raster
	force := a.ConnectShouldDraw(func(redraw *bool) { *redraw = true })
	ink = graphics.RGBA(20, 160, 80, 128)
	a.Paint(target)
	if a.image != cached || a.raster != raster || target.uploads != 1 || target.updates != 1 {
		t.Fatal("同尺寸重绘没有复用软件及原生图片", target.uploads, target.updates)
	}
	if got := target.images[0].pixels.RGBAAt(3, 3); got != (color.RGBA{R: 10, G: 80, B: 40, A: 128}) {
		t.Fatal("Update 未替换预乘像素", got)
	}
	force.Disconnect()
	// DIP 和比例均改变，物理尺寸仍为 8×6；复用图片并更新绘制范围。
	a.Arrange(geometry.Rect(0, 0, 4, 3))
	target.scale = 2
	a.Paint(target)
	if a.image != cached || target.uploads != 1 || target.updates != 2 || calls != 3 || a.imageSize != (geometry.Size{Width: 4, Height: 3}) {
		t.Fatal("物理尺寸相同仍重新创建图片，或逻辑范围未更新")
	}
}

func TestDrawingAreaUpdateFailureAndLifecycle(t *testing.T) {
	for _, stage := range []string{"failure", "destroy"} {
		t.Run(stage, func(t *testing.T) {
			calls := 0
			a := NewDrawingArea(func(p Painter, size geometry.Size) {
				calls++
				p.FillRect(geometry.Rectangle{Size: size}, graphics.RGB(uint8(calls*40), 0, 0))
			})
			win := &window{}
			win.SetWidget(a)
			t.Cleanup(win.Destroy)
			a.Arrange(geometry.Rect(0, 0, 8, 6))
			target := &drawingAreaTarget{scale: 1}
			a.Paint(target)
			cached := a.image
			a.SetDrawFunc(a.DrawFunc())
			if stage == "destroy" {
				target.onUpdate = func() { a.base().destroy(a) }
				a.Paint(target)
				if !a.Destroyed() || a.image != nil || a.raster != nil || !target.images[0].destroyed || len(target.draws) != 1 {
					t.Fatal("更新回调销毁后继续绘制或泄漏资源")
				}
				return
			}
			target.fail = true
			a.Paint(target)
			a.Paint(target)
			// 替身在失败前已经改写像素，禁止把它当作仍然可用的旧图呈现。
			if calls != 2 || target.updates != 2 || target.uploads != 1 || a.image != cached || len(target.draws) != 1 || !a.needsUpload {
				t.Fatal("失败后重复软件绘制、重建资源或显示未完成更新")
			}
			target.fail = false
			a.Paint(target)
			if calls != 2 || target.updates != 3 || target.uploads != 1 || a.image != cached || len(target.draws) != 2 || a.needsUpload {
				t.Fatal("没有复用同一图片重试已有软件结果")
			}
		})
	}
}
