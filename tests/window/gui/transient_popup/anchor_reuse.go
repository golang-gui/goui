package main

import (
	"log"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/events"
)

// 在真实事件循环中检查普通跨窗重建和共享客户端选择目标缓存；不使用 DevServer。
func startAnchorVerification(app gui.Application, first gui.Window, a, b gui.Widget) {
	second, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 360, Height: 240}})
	must(err)
	must(second.SetTitle("GOUI Popover Anchor Destination"))
	otherAnchor := gui.NewButton()
	otherAnchor.SetChild(gui.NewLabel("Destination anchor"))
	second.SetWidget(otherAnchor)
	must(second.Show())
	ordinary := gui.NewPopover(a, nil)
	body := gui.NewLabel("Reusable ordinary popover")
	ordinary.SetWidget(body)
	ordinary.SetPlacement(gui.PopoverPlacementBottom)
	shared := gui.NewSharedPopover(b)
	shared.SetWidget(gui.NewLabel("Shared source"))
	destination := gui.NewSharedPopover(otherAnchor)
	destination.SetWidget(gui.NewLabel("Shared destination"))
	sourceAgain := gui.NewSharedPopover(b)
	sourceAgain.SetWidget(gui.NewLabel("Shared source reused"))
	closed := 0
	ordinary.ConnectClosed(func() { closed++ })
	var oldTarget gui.PopoverTarget
	var sourceRoot, destinationRoot gui.Root
	stage := 0
	check := func(ok bool, message string) {
		if !ok {
			log.Fatalf("ANCHOR FAIL stage=%d %s", stage, message)
		}
	}
	timer := app.NewTimer()
	timer.ConnectTimeout(func() {
		switch stage {
		case 0:
			must(ordinary.Show())
			check(body.Root() == ordinary && ordinary.Visible(), "首次显示没有使用普通 Root")
			oldTarget = first.Popovers()[0]
		case 1:
			ordinary.SetAnchor(b)
			check(!ordinary.Visible() && closed == 1, "换锚点没有结束旧显示一次")
			must(ordinary.Show())
			check(body.Root() == ordinary && ordinary.Anchor() == b, "同窗换锚点改变了 Root")
			check(oldTarget.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: events.KeyEscape}) != nil, "旧输入目标仍有效")
		case 2:
			a.Parent().(interface{ RemoveChild(gui.Widget) }).RemoveChild(a)
			check(ordinary.Visible(), "旧锚点卸载关闭了新锚点显示")
		case 3:
			ordinary.Hide()
			must(shared.Show())
			sourceRoot = shared.Widget().Root()
			_, ok := sourceRoot.(gui.Popover)
			check(ok, "共享 Root 不是普通 Popover")
		case 4:
			must(destination.Show())
			destinationRoot = destination.Widget().Root()
			check(sourceRoot != destinationRoot, "不同 Surface 共用了缓存 Root")
		case 5:
			shared.SetAnchor(otherAnchor)
			check(!shared.Visible(), "共享客户端换锚点没有结束旧显示")
			must(shared.Show())
			check(!destination.Visible() && shared.Widget().Root() == destinationRoot, "共享客户端迁窗未选择目标缓存")
		case 6:
			must(sourceAgain.Show())
			check(sourceAgain.Widget().Root() == sourceRoot, "源缓存被迁移或销毁")
		case 7:
			shared.Destroy()
			sourceAgain.Hide()
			ordinary.SetAnchor(otherAnchor)
			must(ordinary.Show())
			check(body.Root() == ordinary && ordinary.Visible() && len(second.Popovers()) == 1, "普通跨窗迁移失败")
			first.Destroy()
		case 8:
			check(ordinary.Visible() && body.Root() == ordinary && len(second.Popovers()) == 1, "源 Surface 销毁影响了新显示")
			ordinary.Hide()
			check(closed == 3, "普通关闭通知次数错误")
			ordinary.Destroy()
			destination.Destroy()
			sourceAgain.Destroy()
			timer.Stop()
			log.Printf("ANCHOR PASS ordinary=%p source=%p destination=%p", ordinary, sourceRoot, destinationRoot)
			second.Destroy()
		}
		stage++
	})
	must(timer.Start(150 * time.Millisecond))
}
