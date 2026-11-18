package widgets

import (
	"runtime"
	"strings"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/typography/pango"
	"github.com/golang-gui/goui/theme/modern"
)

// 原生换行与字号失效不依赖桌面；原生创建、测量与释放在同一锁定线程完成。
func TestListRowGroupUniformRowsNativeWrappingAndStyle(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	context, err := pango.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer context.Destroy()
	previous := gui.App
	gui.App = &tabTypographyApp{context: context}
	defer func() { gui.App = previous }()
	group := NewListRowGroup()
	group.SetUniformRows(true)
	short, long := NewListRow("Updates"), NewListRow("Details")
	long.SetSubtitle(strings.Repeat("下载更新后保留设置，preserve user choices. ", 4))
	group.AddChild(short)
	group.AddChild(long)
	scope := gui.NewStyleScope()
	scope.SetStyleSheet(modern.Sheet(modern.Options{FontSize: 14}))
	scope.SetChild(group)
	owner := gui.NewPopover(nil, nil)
	owner.SetWidget(scope)
	defer owner.Destroy()
	measure := func(width float32) float32 {
		t.Helper()
		m := scope.Measure(layout.Loose(geometry.Size{Width: width, Height: layout.Inf}))
		group.Arrange(geometry.Rect(0, 0, width, m.Height))
		if short.Visible() && (short.Rect().Height != long.Rect().Height || short.Rect().Height*2 != m.Height) {
			t.Fatal("真实文字换行后两行未统一", short.Rect(), long.Rect(), m)
		}
		return long.Rect().Height
	}
	wide := measure(560)
	narrow := measure(260)
	if narrow <= wide || wide <= 48 {
		t.Fatal("真实换行没有增加统一行高", wide, narrow)
	}
	scope.SetStyleSheet(modern.Sheet(modern.Options{FontSize: 26}))
	large := measure(260)
	if large <= narrow {
		t.Fatal("相同宽度下样式字号改变没有使缓存失效", narrow, large)
	}
	long.SetSubtitle("")
	updated := measure(260)
	if updated >= large {
		t.Fatal("内容缩短后统一高度没有恢复", updated, large)
	}
	long.SetSubtitle(strings.Repeat("Long description. ", 5))
	measure(260)
	long.SetVisible(false)
	m := scope.Measure(layout.Loose(geometry.Size{Width: 260, Height: layout.Inf}))
	group.Arrange(geometry.Rect(0, 0, 260, m.Height))
	if short.Rect().Height != updated || m.Height != updated {
		t.Fatal("隐藏最高行后仍保留旧高度", short.Rect(), m, updated)
	}
	long.SetVisible(true)
	group.SetUniformRows(false)
	m = scope.Measure(layout.Loose(geometry.Size{Width: 260, Height: layout.Inf}))
	group.Arrange(geometry.Rect(0, 0, 260, m.Height))
	if short.Rect().Height != updated || long.Rect().Height <= updated {
		t.Fatal("关闭后没有恢复独立自然行高", short.Rect(), long.Rect())
	}
}
