//go:build linux || windows || darwin

// 真实排版与共享 GUI 控件的无桌面集成测试。
// 运行：go test ./tests/integration/typography；需要对应系统的原生字体库，无需窗口或事件循环。
// 原生创建与释放在各叶子测试的同一锁定线程；测试不修改系统设置或写入源码目录。
package typography_test

import (
	"runtime"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/style"
)

func nativeContext(t *testing.T) typography.Context {
	t.Helper()
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)
	c, err := newContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Destroy)
	return c
}

type typographyApplication struct {
	gui.Application
	context typography.Context
	sheet   style.StyleSheet
}

func (a *typographyApplication) Typography() typography.Context { return a.context }
func (a *typographyApplication) StyleSheet() style.StyleSheet   { return a.sheet }

// 无桌面集成没有事件循环，计时器启动按契约失败，编辑器使用既有同步排版回退。
// 定时调度与延迟重排由 gui 包的可推进计时队列另行验证。
func (*typographyApplication) NewTimer() *gui.Timer { return new(gui.Timer) }

func useTypography(t *testing.T, context typography.Context, sheet style.StyleSheet) {
	t.Helper()
	previous := gui.App
	gui.App = &typographyApplication{context: context, sheet: sheet}
	t.Cleanup(func() { gui.App = previous })
}

func ownWidget(t *testing.T, widget gui.Widget) gui.Popover {
	t.Helper()
	owner := gui.NewPopover(nil, nil)
	owner.SetWidget(widget)
	t.Cleanup(owner.Destroy)
	return owner
}

func descendants(widget gui.Widget, match func(gui.Widget) bool) []gui.Widget {
	var found []gui.Widget
	for _, child := range widget.Children() {
		if match(child) {
			found = append(found, child)
		}
		found = append(found, descendants(child, match)...)
	}
	return found
}

type fixedLayout struct{ size geometry.Size }

func (l fixedLayout) Measure(_ []layout.Child, c layout.Constraint) layout.Measurement {
	return layout.Measured(c.Clamp(l.size))
}
func (fixedLayout) Arrange([]layout.Child, geometry.Rectangle) {}

func fixedWidget(width, height float32) gui.Widget {
	w := new(gui.WidgetBase)
	w.SetLayoutManager(fixedLayout{geometry.Size{Width: width, Height: height}})
	return w
}
