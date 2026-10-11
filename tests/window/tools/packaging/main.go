// 构建资源与应用打包的真实桌面验收，不依赖 DevServer。
// 环境：Windows x64、macOS arm64、Linux X11 x64/arm64；记录 OS/WM、后端、缩放。
// 启动：仓库根目录 go run ./cmd/goui build
// -config tests/window/tools/packaging/goui.toml -target <OS/ARCH> -o <仓库外二进制路径>。
// 图标为测试用红／半透明绿矩形，透明上下留白；不是 GOUI 项目 logo。
// 初始：Native 520×240 DIP，标题“GOUI 打包验证”，文本和关闭按钮。
//
// 操作与逐项预期：
//  1. Windows：构建目标 windows/amd64 的 EXE，Explorer 大／小图标均显示测试资源，
//     属性→详细信息为版本 0.1.0.1、产品／描述“GOUI 打包验证”。双击后不弹控制台，
//     原生标题栏及任务栏使用资源图标；首次缓存刷新可能需要新 EXE 文件名。
//  2. macOS：先 build darwin/arm64，再 package -format app -binary <二进制路径>
//     -o <仓库外新目录>/Packaging.app。plutil -lint <app>/Contents/Info.plist 成功；
//     Finder 和 Dock 显示测试图标，open <app> 能启动且无 Bundle ID 不一致错误。
//     裸 Mach-O 不承诺 Dock 图标；本用例不检验签名、公证或 Gatekeeper 分发。
//  3. Linux：先 build，再 package -format dir -binary <二进制路径> -o <仓库外新目录>。
//     desktop-file-validate <目录>/usr/share/applications/org.golang-gui.PackagingTest.desktop
//     成功；直接运行 usr/bin/goui-packaging-test 应正常。xprop WM_CLASS 后点击窗口，
//     res_name 为 goui-packaging-test、res_class 为 org.golang-gui.PackagingTest。
//     需要应用启动器验证时，在明确同意安装后手工把目录内容安装到对应 usr 路径，
//     刷新图标缓存，启动器名称／图标正确；未安装的目录不自动注册。
//     X11 运行时没有主动设置 _NET_WM_ICON，不承诺所有 WM 的标题栏图标一致。
//  4. 缩放窗口、点击关闭按钮，窗口正常绘制／响应并退出；不以启动成功代替图标验收。
//
// 平台差异：装饰、图标缓存、桌面关联规则由系统决定；Linux 不承诺 ELF 文件图标。
// 副作用：build/generate 在 main 包旁生成 Windows syso 及摘要，默认输出 dist；
// 请用仓库外 -o 放产物。package 不安装、不提权、不签名、不自动构建。
// 复位：重启应用；如手工安装过 Linux 文件，记录精确路径并移除这次安装的文件。
// 退出：关闭按钮或系统关闭窗口；-auto-close=2s 可做启动探针，但不代替桌面验收。
package main

import (
	"flag"
	"fmt"
	"runtime"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
)

func main() {
	runtime.LockOSThread()
	autoClose := flag.Duration("auto-close", 0, "自动关闭延时；0 表示手工验收")
	flag.Parse()
	app, err := gui.NewApplication("org.golang-gui.PackagingTest")
	check(err)
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 520, Height: 240}, Chrome: gui.WindowChromeNative})
	check(err)
	check(win.SetTitle("GOUI 打包验证"))
	box := gui.NewLinearBox(layout.DirectionVertical)
	box.SetSpacing(12)
	box.AddChild(gui.NewLabel("应用 ID: org.golang-gui.PackagingTest"))
	box.AddChild(gui.NewLabel("请检查 Explorer / Finder / 启动器的名称和图标。"))
	closeButton := gui.NewButton()
	closeButton.SetChild(gui.NewLabel("关闭"))
	closeButton.ConnectClicked(func() { check(win.RequestClose()) })
	box.AddChild(closeButton)
	win.SetWidget(box)
	check(win.Show())
	if *autoClose > 0 {
		timer := app.NewTimer()
		timer.ConnectTimeout(func() { win.Destroy() })
		check(timer.StartOnce(*autoClose))
	}
	fmt.Println("START: verify native icons separately; auto-close:", *autoClose)
	app.Run()
	fmt.Println("EXIT")
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
