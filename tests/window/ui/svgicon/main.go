// SVG Icon 窗口验证（不使用 DevServer）。
//
// 环境：Linux/Windows/macOS 桌面；Linux 记录 WM，所有平台记录后端和 DPI。
// 启动：go run ./tests/window/ui/svgicon
// GOUI_PLAT_PAINTER=software/opengl，Windows 还可选 direct2d。
// GOUI_PLAT_SCALE=1/2 可分别验证；ui.Run 负责主线程。
// 初始：亮色、蓝色主题、24 DIP。前三行是 Brain、People、Dev tool；
// 各行依次为主题色、正文前景色、原色。下方保留孔洞、描边、渐变等辅助验证。
//
// 操作与预期：
//
//  1. 点击 Light/Dark、Accent：三个 SVG 的主题色列跟随主题色，正文列跟随亮暗；
//     原色列保持 SVG 默认黑色（暗背景上对比度低是预期），孔洞保持透明。
//
//  2. 点击 Size：16/24/40/64 DIP 循环；Brain 内部连线、People 头部孔洞、
//     Dev tool 扳手轮廓应完整；辅助描边的圆头和线粗同比例变化。内容过高时滚动查看。
//
//  3. Image calls 显示重建前的取图计数；点击 Rebuild 后下一次绘制会重新取图，
//     再次点击可看到增加的次数。没有 UI 重建、尺寸或颜色变化的普通重绘不增加。
//     这不是栅格化/上传计数；准备好的 Source 可返回缓存图片，上传复用由包内测试验证。
//     改颜色也应重新取图：Source 负责生成最终颜色，不由 Icon 二次着色。
//
//  4. 点击 Replace：辅助区第一张图在复合孔洞与圆头描边图之间切换，无旧图残留。
//
//  5. 点击 Second window：三个 SVG 的同一 Source 在第二个窗口以 48 DIP/正文色使用。
//     关闭该窗口后主窗口继续工作，不出现资源失效或颜色/尺寸污染。
//
//  6. 调整窗口大小，分别用 1x/2x 和三个后端启动，检查描边、渐变、透明边缘。
//     允许抗锯齿边缘细微差异；不允许方形背景、孔洞被填满或缩放后描边变细。
//
//  7. 最后一行显示多分辨率位图和缺失占位符；前者跟随尺寸/主题切换，后者为
//     方框加叉且随前景换色；旁边的 nil Source 不应出现占位符。
//
// SVG 创建失败会在启动时返回错误，不通过控件发送异步错误通知。
// 几何回归：Brain 连线、People 三个头部及孔洞应完整，原色与主题色轮廓一致。
// 连续弧线解析已在自有 oksvg fork 修复；离屏对照记录见 doc/DesignSVG.md。
// 副作用：只创建测试窗口，无文件/剪贴板/系统设置修改。
// 复位：重启程序。退出：关闭所有窗口。此程序不证明真实跨屏 DPI 切换已验收。
package main

import (
	"fmt"
	"image"
	"image/color"
	"strings"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/icon"
	drawicon "github.com/golang-gui/goui/icon/draw"
	"github.com/golang-gui/goui/icon/svg"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
)

const (
	brainSvg   = `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path d="M14.5 2c1.64 0 3 1.22 3.22 2.8a3.88 3.88 0 0 1 2.08 6.63A4 4 0 0 1 22 15v.25a4 4 0 0 1-3.37 3.95A3.75 3.75 0 0 1 12 20.5a3.75 3.75 0 0 1-6.63-1.3A4 4 0 0 1 2 15.25V15a4 4 0 0 1 2.2-3.57 3.86 3.86 0 0 1 2.08-6.64A3.25 3.25 0 0 1 12 3.17C12.6 2.46 13.5 2 14.5 2m-5 1.5c-.97 0-1.75.78-1.75 1.75v.25c0 .41-.34.75-.75.75h-.12a2.37 2.37 0 1 0 0 4.75H7c1.16 0 2.12.89 2.24 2.02l.01.23v.4a2 2 0 1 1-1.5 0v-.48A.75.75 0 0 0 7 12.5H6A2.5 2.5 0 0 0 3.5 15v.25a2.5 2.5 0 0 0 2.5 2.5c.34 0 .63.23.72.54l.03.14a2.26 2.26 0 0 0 4.5-.18v-9h-.9a2 2 0 1 1 0-1.5h.9v-2.5c0-.97-.78-1.75-1.75-1.75m8.23 9a2 2 0 0 1-.98.85v.9c0 1.24-1 2.25-2.25 2.25h-1.75v1.75a2.26 2.26 0 0 0 4.5.18l.03-.14c.1-.31.38-.54.72-.54a2.5 2.5 0 0 0 2.5-2.5V15a2.5 2.5 0 0 0-2.5-2.5zM8.5 15a.5.5 0 1 0 0 1 .5.5 0 0 0 0-1m6-11.5c-.96 0-1.75.78-1.75 1.74V15h1.75c.41 0 .75-.34.75-.75v-.9a2 2 0 1 1 2.65-2.48 2.37 2.37 0 0 0-.77-4.62H17a.75.75 0 0 1-.75-.75v-.25c0-.97-.78-1.75-1.75-1.75M16 11a.5.5 0 1 0 0 1 .5.5 0 0 0 0-1M8.5 8a.5.5 0 1 0 0 1 .5.5 0 0 0 0-1"/></svg>`
	peopleSvg  = `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path d="M14.75 10c.97 0 1.75.78 1.75 1.75v4.75a4.5 4.5 0 0 1-9 0v-4.75c0-.97.79-1.75 1.75-1.75zm0 1.5h-5.5a.25.25 0 0 0-.25.25v4.75a3 3 0 0 0 6 0v-4.75a.25.25 0 0 0-.25-.25m-11-1.5h3.38q-.53.63-.62 1.5H3.75a.25.25 0 0 0-.25.25V15a2.5 2.5 0 0 0 3.08 2.43q.13.76.45 1.43Q6.55 19 6 19a4 4 0 0 1-4-4v-3.25c0-.97.78-1.75 1.75-1.75m13.12 0h3.38c.97 0 1.75.78 1.75 1.75V15a4 4 0 0 1-5.03 3.87q.32-.68.46-1.44.27.07.57.07a2.5 2.5 0 0 0 2.5-2.5v-3.25a.25.25 0 0 0-.25-.25h-2.76a2.7 2.7 0 0 0-.62-1.5M12 3a3 3 0 1 1 0 6 3 3 0 0 1 0-6m6.5 1a2.5 2.5 0 1 1 0 5 2.5 2.5 0 0 1 0-5m-13 0a2.5 2.5 0 1 1 0 5 2.5 2.5 0 0 1 0-5m6.5.5a1.5 1.5 0 1 0 0 3 1.5 1.5 0 0 0 0-3m6.5 1a1 1 0 1 0 0 2 1 1 0 0 0 0-2m-13 0a1 1 0 1 0 0 2 1 1 0 0 0 0-2"/></svg>`
	devToolSvg = `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path d="M3 6.25C3 4.45 4.46 3 6.25 3h11.5C19.55 3 21 4.46 21 6.25v5.66a1.4 1.4 0 0 0-.99-.8l-.51-.08V8.5h-15v9.25c0 .97.78 1.75 1.75 1.75h5.68l-.19.19q-.55.57-.7 1.31H6.25A3.25 3.25 0 0 1 3 17.75zm13 5.68-1.72-1.71a.75.75 0 1 0-1.06 1.06l1.7 1.7a5 5 0 0 1 1.07-1.05M4.5 7h15v-.75c0-.97-.78-1.75-1.75-1.75H6.25c-.97 0-1.75.78-1.75 1.75zm6.28 4.28a.75.75 0 1 0-1.06-1.06l-3 3c-.3.3-.3.77 0 1.06l3 3a.75.75 0 1 0 1.06-1.06l-2.47-2.47zm9.02.81c.35.08.44.51.18.77l-1.9 1.9a1.53 1.53 0 0 0 2.16 2.16l1.9-1.9c.26-.26.69-.17.77.18a4.07 4.07 0 0 1-5.57 4.62l-2.73 2.73a1.53 1.53 0 0 1-2.16-2.16l2.73-2.73a4.07 4.07 0 0 1 4.62-5.57"/></svg>`
)

func main() {
	parseSVG := func(data string) *svg.Source {
		source, err := svg.Parse(strings.NewReader(data))
		if err != nil {
			panic(err)
		}
		return source
	}
	// Prepare the real resources once, outside the declarative build.
	brain, people, devTool := parseSVG(brainSvg), parseSVG(peopleSvg), parseSVG(devToolSvg)
	brainRaw, peopleRaw, devToolRaw := brain.WithRawColors(), people.WithRawColors(), devTool.WithRawColors()
	hole := parseSVG(`<svg viewBox="0 0 24 24"><path fill-rule="evenodd" d="M2 2H22V22H2Z M7 7H17V17H7Z" fill="#2878dc"/></svg>`)
	stroke := parseSVG(`<svg viewBox="0 0 24 24"><path d="M4 12L10 18L20 5" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"/></svg>`)
	gradient := parseSVG(`<svg viewBox="0 0 24 24"><defs><linearGradient id="g"><stop offset="0" stop-color="#2878dc"/><stop offset="1" stop-color="#d04090"/></linearGradient></defs><circle cx="12" cy="12" r="10" fill="url(#g)"/></svg>`)
	rasterCalls := 0
	probeFn := imageFunc(func(w, h int, foreground ui.Color) image.Image {
		rasterCalls++
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := h / 4; y < h*3/4; y++ {
			for x := w / 4; x < w*3/4; x++ {
				img.SetRGBA(x, y, color.RGBA{A: 128})
			}
		}
		source, err := icon.NewImage(img)
		if err != nil {
			panic(err)
		}
		return source.Image(w, h, foreground)
	})
	probe := &probeFn
	missingFn := imageFunc(func(int, int, ui.Color) image.Image { return nil })
	missing := &missingFn
	var resolutions []image.Image
	for _, edge := range []int{16, 32, 64} {
		img := image.NewRGBA(image.Rect(0, 0, edge, edge))
		for y := edge / 4; y < edge*3/4; y++ {
			for x := edge / 4; x < edge*3/4; x++ {
				img.SetRGBA(x, y, color.RGBA{A: 255})
			}
		}
		resolutions = append(resolutions, img)
	}
	bitmapSource, err := icon.NewImage(resolutions...)
	if err != nil {
		panic(err)
	}
	holeOriginal, strokeOriginal, gradientOriginal := hole.WithRawColors(), stroke.WithRawColors(), gradient.WithRawColors()
	draw := &drawicon.Source{Draw: func(p graphics.Painter, size graphics.Size, c graphics.Color) {
		p.FillEllipse(geometry.Point{X: size.Width / 2, Y: size.Height / 2}, size.Width*.4, size.Height*.4, c)
	}}
	dark, replaced, extra := false, false, false
	sizeIndex, accentIndex, rebuilds := 1, 0, 0
	sizes := []float32{16, 24, 40, 64}
	accents := []color.Color{color.RGBA{R: 40, G: 120, B: 220, A: 255}, color.RGBA{R: 160, G: 60, B: 200, A: 255}}
	err = ui.Run("org.golang-gui.SVGIconTest", func(app ui.App) ui.RootView {
		current, original := hole, holeOriginal
		if replaced {
			current, original = stroke, strokeOriginal
		}
		size := sizes[sizeIndex]
		windows := []ui.WindowView{
			ui.Window("svg").Title("GOUI SVG Icon validation").Size(780, 640).Chrome(ui.WindowChromeIntegrated).Content(
				ui.VBox(
					ui.HeaderBar(ui.Label("SVG IconSource")),
					ui.ScrollView(ui.VBox(
						ui.Label(fmt.Sprintf("Size %.0f DIP | Rebuilds %d | Image calls %d", size, rebuilds, rasterCalls)),
						ui.HBox(
							ui.Button("Light/Dark").OnClick(func() { dark = !dark; app.RequestUpdate() }),
							ui.Button("Accent").OnClick(func() { accentIndex = (accentIndex + 1) % len(accents); app.RequestUpdate() }),
							ui.Button("Size").OnClick(func() { sizeIndex = (sizeIndex + 1) % len(sizes); app.RequestUpdate() }),
						).Spacing(8),
						ui.HBox(
							ui.Button("Rebuild").OnClick(func() { rebuilds++; app.RequestUpdate() }),
							ui.Button("Replace").OnClick(func() { replaced = !replaced; app.RequestUpdate() }),
							ui.Button("Second window").OnClick(func() { extra = true; app.RequestUpdate() }),
						).Spacing(8),
						ui.Label("SVG | Accent | Foreground | Raw colors"),
						ui.HBox(ui.Label("Brain").MinWidth(100), ui.Icon(brain).Size(size).Style(modern.AccentIcon), ui.Icon(brain).Size(size), ui.Icon(brainRaw).Size(size)).Spacing(30),
						ui.HBox(ui.Label("People").MinWidth(100), ui.Icon(people).Size(size).Style(modern.AccentIcon), ui.Icon(people).Size(size), ui.Icon(peopleRaw).Size(size)).Spacing(30),
						ui.HBox(ui.Label("Dev tool").MinWidth(100), ui.Icon(devTool).Size(size).Style(modern.AccentIcon), ui.Icon(devTool).Size(size), ui.Icon(devToolRaw).Size(size)).Spacing(30),
						ui.Label("Additional checks: theme | raw colors | custom drawing / counter"),
						ui.HBox(ui.Icon(current).Size(size).Style(modern.AccentIcon), ui.Icon(original).Size(size), ui.Icon(draw).Size(size)).Spacing(30),
						ui.HBox(ui.Icon(gradient).Size(size).Style(modern.AccentIcon), ui.Icon(gradientOriginal).Size(size), ui.Icon(probe).Size(size).Style(modern.AccentIcon)).Spacing(30),
						ui.HBox(ui.Icon(bitmapSource).Size(size).Style(modern.AccentIcon), ui.Icon(missing).Size(size).Style(modern.AccentIcon), ui.Icon(nil).Size(size)).Spacing(30),
					).Padding(16).Spacing(16)),
				),
			),
		}
		if extra {
			windows = append(windows, ui.Window("svg-shared").Title("Shared Source / independent images").Size(360, 200).Chrome(ui.WindowChromeIntegrated).Content(
				ui.VBox(ui.HeaderBar(ui.Label("Shared sources")), ui.HBox(ui.Icon(brain).Size(48), ui.Icon(people).Size(48), ui.Icon(devTool).Size(48)).Padding(16).Spacing(24)),
			).OnDestroy(func() { extra = false; app.RequestUpdate() }))
		}
		return ui.Root().StyleSheet(modern.Sheet(modern.Options{Dark: dark, AccentColor: accents[accentIndex]})).Windows(windows...)
	})
	if err != nil {
		panic(err)
	}
}

type imageFunc func(int, int, ui.Color) image.Image

func (f imageFunc) Image(w, h int, c ui.Color) image.Image { return f(w, h, c) }
