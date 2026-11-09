// DropDown 弹窗自然宽度与独立约束验收。
//
// 环境：Linux/X11、Windows、macOS 桌面，记录后端、缩放和窗口管理器。
// 启动：GOUI_PLAT_PAINTER=opengl GOUI_PLAT_SCALE=1 go run ./tests/window/widgets/drop_down_width
// 另验 Software、2x；Windows 另可用 Direct2D。默认自动验收，-auto=false 手工操作。
// 初始：Integrated 窗口内有固定 110 DIP 的方法下拉框，选项包含一个长方法名。
//
// 操作与逐项预期：
//  1. 点击下拉框：圆角弹窗向下、主体左缘对齐控件，空间不足时允许原生退避。
//     弹窗按内容扩宽，短列表不出现多余水平/垂直滚动条，控件仍为 110 DIP。
//  2. Min 300：弹窗主体至少 300 DIP；Max 180：上限优先，宽内容允许水平滚动，
//     有足够高度时不能因水平条额外出现垂直条。Reset 清除两个宽度约束。
//  3. Font：切换 12/24pt，重新打开或保持展开时应重新适配实测文本宽度。
//  4. 多次关闭重开、改变窗口大小、悬停/滚动：宽度不得持续增长或跳变，内容可访问。
//  5. Exit 或关闭主窗口退出；重启复位。小于最小宽度的工作区可以限制实际尺寸。
//
// 自动模式按 API 依次打开、设最小/最大宽度和改变字体，通过真实 Popover 的公开
// Widget 树、ScrollView 范围与尺寸断言，无 DevServer；本例不代替键鼠输入验收。
// 输出 PASS 后退出，提前关闭或 panic 均不算通过。只创建窗口/Popup、GUI Timer 和
// 自有样式，不读取用户文件，不改系统设置，无持久化副作用；退出清理本例资源。
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/widgets"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func popupState(win gui.Window) (float32, *gui.ScrollView) {
	popups := win.Popovers()
	if len(popups) != 1 || popups[0].Placement() != gui.PopoverPlacementBottom {
		panic("缺少 Bottom Popover")
	}
	content := popups[0].Widget()
	var find func(gui.Widget) *gui.ScrollView
	find = func(w gui.Widget) *gui.ScrollView {
		if scroll, ok := w.(*gui.ScrollView); ok {
			return scroll
		}
		for _, child := range w.Children() {
			if scroll := find(child); scroll != nil {
				return scroll
			}
		}
		return nil
	}
	scroll := find(content)
	if scroll == nil {
		panic("缺少 ScrollView")
	}
	return content.Rect().Width, scroll
}

func main() {
	auto := flag.Bool("auto", true, "自动验收并退出")
	flag.Parse()
	app, err := gui.NewApplication("org.golanggui.DropDownWidth")
	if err != nil {
		log.Fatal(err)
	}
	var win gui.Window
	var choice *widgets.DropDown
	passed := false
	app.ConnectActivate(func() {
		font := float32(12)
		applyFont := func() { app.SetStyleSheet(modern.Sheet(modern.Options{FontSize: font})) }
		applyFont()
		win, err = app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 640, Height: 400}, Chrome: gui.WindowChromeIntegrated})
		must(err)
		must(win.SetTitle("GOUI DropDown popup width"))
		root := gui.NewLinearBox(layout.DirectionVertical)
		root.SetPadding(geometry.UniformInsets(24))
		root.SetSpacing(12)
		root.SetCrossAlign(layout.CrossStart)
		root.AddChild(gui.NewLabel("Fixed 110 DIP control; independent popup width"))
		choice = widgets.NewDropDown()
		choice.SetMinSize(geometry.Size{Width: 110})
		choice.SetMaxSize(geometry.Size{Width: 110})
		choice.SetModel(gui.NewSliceListModel([]widgets.DropDownItem{{Text: "GET"}, {Text: "POST"}, {Text: "DELETE"}, {Text: "CONNECT"}, {Text: "CUSTOM_LONG_METHOD"}}))
		choice.SetSelected(0)
		choice.ConnectSelected(func(index int) { fmt.Println("SELECTED", index) })
		root.AddChild(choice)
		row := gui.NewLinearBox(layout.DirectionHorizontal)
		row.SetSpacing(8)
		add := func(text string, fn func()) {
			button := gui.NewButton()
			button.SetChild(gui.NewLabel(text))
			button.ConnectClicked(fn)
			row.AddChild(button)
		}
		add("Open", func() { must(choice.Open()) })
		add("Min 300", func() { choice.SetPopupMinWidth(300); must(choice.Open()) })
		add("Max 180", func() { choice.SetPopupMaxWidth(180); must(choice.Open()) })
		add("Reset", func() { choice.SetPopupMinWidth(0); choice.SetPopupMaxWidth(0) })
		add("Font", func() {
			if font == 12 {
				font = 24
			} else {
				font = 12
			}
			applyFont()
		})
		add("Exit", app.Quit)
		root.AddChild(row)
		win.SetWidget(root)
		must(win.Show())
		if !*auto {
			return
		}
		phase := 0
		lastState := ""
		deadline := time.Now().Add(10 * time.Second)
		timer := app.NewTimer()
		timer.ConnectTimeout(func() {
			if time.Now().After(deadline) {
				panic("弹窗尺寸验收超时：" + lastState)
			}
			if choice.Rect().Width == 0 {
				return
			}
			if choice.Rect().Width != 110 {
				panic("收起控件宽度被弹窗改变")
			}
			if phase == 0 {
				must(choice.Open())
				phase++
				return
			}
			width, scroll := popupState(win)
			info := scroll.Snapshot()
			state := fmt.Sprintf("phase=%d width=%g height=%g scroll=%g,%g", phase, width, scroll.Parent().Rect().Height, info.MaxScrollX, info.MaxScrollY)
			if state != lastState {
				fmt.Println("STATE", state)
				lastState = state
			}
			switch phase {
			case 1:
				if width <= 110 || info.MaxScrollX > 0 || info.MaxScrollY > 0 {
					panic(fmt.Sprintf("自然尺寸错误：width=%g scroll=%g,%g", width, info.MaxScrollX, info.MaxScrollY))
				}
				choice.SetPopupMinWidth(300)
				phase++
			case 2:
				if width < 300 {
					return
				}
				choice.SetPopupMaxWidth(180)
				phase++
			case 3:
				if math.Abs(float64(width-180)) > 1 {
					return
				}
				if info.MaxScrollX <= 0 || info.MaxScrollY > 0 {
					panic("限宽后的水平条制造了额外垂直条")
				}
				choice.SetPopupMinWidth(0)
				choice.SetPopupMaxWidth(0)
				font = 24
				applyFont()
				phase++
			case 4:
				if width <= 180 || info.MaxScrollX > 0 || info.MaxScrollY > 0 {
					return
				}
				passed = true
				timer.Stop()
				fmt.Printf("PASS drop_down_width control=%g popup=%g font=%g\n", choice.Rect().Width, width, font)
				app.Quit()
			}
		})
		must(timer.Start(180 * time.Millisecond))
	})
	app.ConnectShutdown(func() {
		if choice != nil {
			choice.Close()
		}
		if win != nil {
			win.Destroy()
		}
	})
	must(app.Run())
	if *auto && !passed {
		log.Fatal("自动验收未完成")
	}
}
