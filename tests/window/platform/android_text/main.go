//go:build android

// Android 系统排版能力验证，环境、构建和逐项预期见 README.md。
// Go 固定线程经 JNI 调用公开排版 API，再由正式移动 Window/Software 显示结果。
// 历史 shaping 对照与当前交互验证独立报告；程序启动不等于排版后端通过。
package main

import (
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"unsafe"

	ffi "github.com/goexlib/cgo"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/platform/mobile"
)

type nativeBitmap struct {
	Width, Height, Stride, Length int32
	Pixels                        uintptr
	Thread                        int32
}

func symbol(name string) uintptr {
	p, err := ffi.Dlsym(ffi.RTLD_DEFAULT, name)
	if err != nil {
		panic(err)
	}
	return p
}

func main() {
	thread := symbol("gettid")
	owner := ffi.CallRet[int32](thread)
	address := ffi.CallRet[uintptr](symbol("probe_run"))
	if address == 0 {
		panic("Android text probe failed; inspect logcat")
	}
	b := *(*nativeBitmap)(ffi.Pointer(address))
	if b.Thread != owner || ffi.CallRet[int32](thread) != owner || b.Width != 800 || b.Height != 1100 || b.Stride != 3200 || b.Length != 800*1100*4 || b.Pixels == 0 {
		panic(fmt.Sprintf("invalid native result: %+v owner=%d", b, owner))
	}
	pixels := append([]byte(nil), unsafe.Slice((*byte)(ffi.Pointer(b.Pixels)), int(b.Length))...)
	ffi.Call(symbol("probe_free"), address)
	bitmap := &image.RGBA{Pix: pixels, Stride: int(b.Stride), Rect: image.Rect(0, 0, int(b.Width), int(b.Height))}
	dir := ffi.GoString(ffi.Pointer(ffi.CallRet[uintptr](symbol("probe_directory"))))
	var report struct {
		Interaction struct {
			Cases []interactionCase
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, "report.json"))
	if err != nil {
		panic(err)
	}
	if err = json.Unmarshal(data, &report); err != nil {
		panic(err)
	}
	interactions := verifyInteractions(report.Interaction.Cases)
	data, err = json.MarshalIndent(interactions, "", "  ")
	if err != nil {
		panic(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "interaction-go.json"), data, 0600); err != nil {
		panic(err)
	}
	loop := mobile.NewEventLoop()
	defer loop.Destroy()
	var painter graphics.Painter
	var img graphics.Image
	var size events.SizeEvent
	frames := 0
	win, err := mobile.NewWindow(loop, geometry.Size{Width: 320, Height: 480}, func(event events.Event) {
		if ffi.CallRet[int32](thread) != owner {
			panic("event changed GOUI thread")
		}
		switch event := event.(type) {
		case events.SizeEvent:
			size = event
		case events.PaintEvent:
			painter.Begin(size.PixelWidth, size.PixelHeight, size.PixelWidth/size.Width)
			painter.Clear(graphics.RGB(238, 244, 250))
			scale := min(size.Width/float32(b.Width), size.Height/float32(b.Height))
			painter.DrawImage(graphics.Rect(0, 0, float32(b.Width)*scale, float32(b.Height)*scale), img)
			painter.End()
			frames++
			data, _ := json.Marshal(map[string]any{"frames": frames, "owner": owner, "nativeThread": b.Thread, "width": size.Width, "height": size.Height, "interactionFailures": interactions.Failures})
			if err := os.WriteFile(filepath.Join(dir, "go.json"), data, 0600); err != nil {
				panic(err)
			}
		}
	}, common.WindowOptions{})
	if err != nil {
		panic(err)
	}
	painter, err = software.NewPainter(win)
	if err != nil {
		panic(err)
	}
	defer painter.Destroy()
	img, err = painter.NewImage(bitmap)
	if err != nil {
		panic(err)
	}
	defer img.Destroy()
	if err := win.Show(); err != nil {
		panic(err)
	}
	loop.Run()
}
