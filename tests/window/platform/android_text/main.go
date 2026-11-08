//go:build android

// Android 系统排版能力验证，环境、构建和逐项预期见 README.md。
// Go 固定线程经 JNI 调用公开排版 API，再由正式移动 Window/Software 显示结果。
// 历史 shaping 对照与当前交互验证独立报告；程序启动不等于排版后端通过。
package main

import (
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"path/filepath"

	ffi "github.com/goexlib/cgo"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/platform/typography"
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
	ffi.Call(symbol("probe_free"), address)
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
	backend := verifyBackend(dir)
	data, err = json.MarshalIndent(backend, "", "  ")
	if err != nil {
		panic(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "backend-go.json"), data, 0600); err != nil {
		panic(err)
	}
	backendPlatform, err := platform.NewPlatform(platform.DefaultName(), "org.golanggui.androidtext")
	if err != nil {
		panic(err)
	}
	defer backendPlatform.Destroy()
	context, err := backendPlatform.NewTypography()
	if err != nil {
		panic(err)
	}
	defer context.Destroy()
	text := fmt.Sprintf("GOUI Android Typography\nNative Layout + Go Painter\n\nLatin: office affinity ffi\n中文：窗口内的 Go\n日本語：かなと漢字\n한국어: 안녕하세요\nالعربية: السلام عليكم\nעברית: ABC אבג DEF\nहिन्दी: नमस्ते\nEmoji: 😀 👩‍💻 🇨🇳\n\nBackend failures: %d\nInteraction failures: %d", backend.Failures, interactions.Failures)
	layout, err := context.NewTextLayout(text, typography.TextFormat{Font: typography.FontInfo{Size: 18}, TextColor: color.NRGBA{R: 32, G: 64, B: 96, A: 255}, WrapMode: typography.WrapWordChar}, 420, 1000)
	if err != nil {
		panic(err)
	}
	defer layout.Destroy()
	loop, err := backendPlatform.NewEventLoop()
	if err != nil {
		panic(err)
	}
	defer loop.Destroy()
	var painter graphics.Painter
	var size events.SizeEvent
	frames := 0
	win, err := backendPlatform.NewWindow(geometry.Size{Width: 320, Height: 480}, func(event events.Event) {
		if ffi.CallRet[int32](thread) != int32(os.Getpid()) {
			panic("event did not enter the Android UI thread")
		}
		switch event := event.(type) {
		case events.SizeEvent:
			size = event
			layout.SetSize(max(1, size.Width-32), size.Height)
		case events.PaintEvent:
			painter.Begin(size.PixelWidth, size.PixelHeight, size.PixelWidth/size.Width)
			painter.Clear(graphics.RGB(238, 244, 250))
			painter.DrawTextLayout(graphics.Point{X: 16, Y: 16}, layout)
			painter.End()
			frames++
			data, _ := json.Marshal(map[string]any{"frames": frames, "owner": ffi.CallRet[int32](thread), "startupThread": owner, "nativeThread": b.Thread, "width": size.Width, "height": size.Height, "interactionFailures": interactions.Failures, "backendFailures": backend.Failures})
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
	if err := win.(interface{ Show() error }).Show(); err != nil {
		panic(err)
	}
	loop.Run()
}
