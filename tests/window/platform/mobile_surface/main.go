//go:build android || ios

// 移动端逻辑窗口/Software 表面验收。构建与操作见 README.md；真实原生宿主
// 使用对应平台的正式宿主，不以测试宿主代替绘制实现。窗口和图片在 Run 前
// 创建；周期请求绘制，旋转、后台返回和 Android Activity 重建后 nonce、图片
// 与 Window 保持不变。表面暂时丢失不得发 Destroy，threadFailures 必须为零。
package main

import (
	"encoding/json"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"runtime"
	"time"

	ffi "github.com/goexlib/cgo"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/software"
)

type presentTarget struct {
	win                    common.Window
	submitted, unavailable int
}

func (*presentTarget) Transparent() bool { return false }
func (d *presentTarget) Draw(img image.Image) error {
	err := d.win.Draw(img)
	if err == nil {
		d.submitted++
	} else {
		d.unavailable++
	}
	return err
}

func symbol(name string) uintptr {
	p, err := ffi.Dlsym(ffi.RTLD_DEFAULT, name)
	if err != nil {
		panic(err)
	}
	return p
}

func main() {
	backend, err := platform.NewPlatform(platform.DefaultName(), "org.golanggui.mobilesurface")
	if err != nil {
		panic(err)
	}
	defer backend.Destroy()
	loop, err := backend.NewEventLoop()
	if err != nil {
		panic(err)
	}
	defer loop.Destroy()
	dir := ffi.GoString(ffi.Pointer(ffi.CallRet[uintptr](symbol("probe_data_dir"))))
	if err := os.MkdirAll(dir, 0700); err != nil {
		panic(err)
	}
	threadName := "gettid"
	if runtime.GOOS == "ios" {
		threadName = "pthread_self"
	}
	thread := symbol(threadName)
	startup := ffi.CallRet[uint64](thread)
	var owner uint64
	nonce := time.Now().UnixNano()
	var win common.Window
	var painter graphics.Painter
	var icon graphics.Image
	var size events.SizeEvent
	var state common.SurfaceState
	var target presentTarget
	frames, ready, lost, destroyed, visible, failures := 0, 0, 0, 0, false, 0
	initialReady := false
	save := func() {
		data, _ := json.Marshal(map[string]any{"nonce": nonce, "frames": frames, "submitted": target.submitted, "unavailable": target.unavailable, "ready": ready, "lost": lost, "destroyed": destroyed, "visible": visible, "generation": state.Generation, "surfaceReady": state.Ready, "threadFailures": failures, "initialReady": initialReady, "width": size.Width, "height": size.Height, "pixelWidth": size.PixelWidth, "pixelHeight": size.PixelHeight})
		path := filepath.Join(dir, "surface.json")
		if err := os.WriteFile(path+".tmp", data, 0600); err != nil {
			panic(err)
		}
		if err := os.Rename(path+".tmp", path); err != nil {
			panic(err)
		}
	}
	win, err = backend.NewWindow(geometry.Size{Width: 320, Height: 240}, func(e events.Event) {
		current := ffi.CallRet[uint64](thread)
		if owner == 0 {
			owner = current
		}
		main := false
		if runtime.GOOS == "android" {
			main = current == uint64(os.Getpid())
		} else {
			main = ffi.CallRet[int32](symbol("pthread_main_np")) != 0
		}
		if !main || current == startup || current != owner {
			failures++
		}
		switch e := e.(type) {
		case events.SurfaceEvent:
			state = e.State
			if state.Ready {
				ready++
			} else {
				lost++
			}
		case events.SizeEvent:
			size = e
		case events.VisibilityEvent:
			visible = e.Visible
		case events.DestroyEvent:
			destroyed++
		case events.PaintEvent:
			if size.PixelWidth <= 0 || painter == nil {
				panic("paint before dimensions/painter")
			}
			frames++
			scale := size.PixelWidth / size.Width
			painter.Begin(size.PixelWidth, size.PixelHeight, scale)
			painter.Clear(graphics.RGB(238, 244, 250))
			painter.FillRoundRect(graphics.Rect(20, 20, min(160, size.Width-40), 80), 12, graphics.RGB(30, 130, 220))
			painter.FillRect(graphics.Rect(size.Width-30, size.Height-30, 10, 10), graphics.RGB(230, 70, 50))
			painter.DrawImage(graphics.Rect(30, 120, 32, 32), icon)
			if os.Getenv("GOUI_SURFACE_DELAY") != "" {
				time.Sleep(35 * time.Millisecond)
			}
			painter.End()
		}
		save()
	}, common.WindowOptions{})
	if err != nil {
		panic(err)
	}
	initialReady = win.SurfaceState().Ready
	target.win = win
	painter, err = software.NewPainter(&target)
	if err != nil {
		panic(err)
	}
	defer painter.Destroy()
	bitmap := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			bitmap.SetRGBA(x, y, color.RGBA{R: 40, G: 170, B: 90, A: 255})
		}
	}
	icon, err = painter.NewImage(bitmap)
	if err != nil {
		panic(err)
	}
	defer icon.Destroy()
	if err = win.(interface{ Show() error }).Show(); err != nil {
		panic(err)
	}
	win.RequestPaint()
	save()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case <-ticker.C:
				loop.Post(func() { win.RequestPaint(); save() })
			case <-done:
				return
			}
		}
	}()
	loop.Run()
}
