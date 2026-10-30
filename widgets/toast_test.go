package widgets

import (
	"image/color"
	"testing"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
)

func TestToastPresentationAndContentOwnership(t *testing.T) {
	toast := NewToast()
	child := gui.NewLabel("first")
	closed := 0
	toast.ConnectClosed(func() { closed++ })
	toast.SetChild(child)
	toast.SetVisible(true)
	if toast.Visible() {
		t.Fatal("未 Show 的 Toast 不应被 Visible(true) 启动")
	}
	toast.Show(child, 0)
	if !toast.Visible() || toast.Padding() != geometry.UniformInsets(12) {
		t.Fatal("首次展示未生效")
	}
	other := gui.NewLabel("second")
	toast.Show(other, -time.Second)
	if closed != 0 || child.Parent() != nil || child.Destroyed() || toast.Child() != other {
		t.Fatal("替换不应触发 Closed 或销毁旧内容")
	}
	toast.Close()
	toast.Close()
	toast.SetVisible(true)
	if toast.Visible() || closed != 1 || toast.Child() != other || other.Destroyed() {
		t.Fatal("Close 应仅结束一次展示，不被 Visible 重放复活")
	}
	if info := toast.Snapshot(); info.Role != RoleToast || len(info.Children) != 1 || len(info.Actions) != 0 {
		t.Fatal("Toast 语义应保留内容且不伪造点击动作")
	}
}

func TestToastDeadlinePauseAndStaleTimeout(t *testing.T) {
	toast := NewToast()
	toast.Show(gui.NewLabel("first"), time.Second)
	now := time.Unix(100, 0)
	toast.started = now
	revision, schedule := toast.revision, toast.schedule
	toast.pause(now.Add(250 * time.Millisecond))
	if toast.remaining != 750*time.Millisecond || !toast.started.IsZero() {
		t.Fatal("暂停未保留剩余时长")
	}
	toast.started = now.Add(time.Second)
	toast.expire(revision, schedule)
	if !toast.Visible() {
		t.Fatal("暂停前的旧超时关闭了恢复后的提示")
	}
	revision, schedule = toast.revision, toast.schedule
	toast.Show(gui.NewLabel("new request"), time.Second)
	toast.started = now.Add(2 * time.Second)
	toast.expire(revision, schedule)
	if !toast.Visible() {
		t.Fatal("上一展示的旧超时关闭了新请求")
	}
	toast.expire(toast.revision, toast.schedule)
	if toast.Visible() {
		t.Fatal("当前超时没有结束展示")
	}
}

func TestToastHoverFocusAndResourcePause(t *testing.T) {
	app := &progressTestApplication{}
	useProgressApplication(t, app)
	toast := NewToast()
	toast.Show(gui.NewLabel("timed"), time.Second)
	host := gui.NewPopover(nil, nil)
	defer host.Destroy()
	host.SetWidget(toast)
	if len(app.timers) != 1 {
		t.Fatal("挂载后应惰性创建单个 GUI Timer")
	}
	toast.Arrange(geometry.Rect(0, 0, 160, 80))
	toast.started = time.Now()
	before := toast.remaining
	testWidgetHover(t, toast, true)
	if !toast.started.IsZero() || toast.remaining > before {
		t.Fatal("悬停未暂停计时")
	}
	toast.SetFocusable(true)
	host.(gui.EventTarget).SetFocusedWidget(toast)
	if !toast.started.IsZero() {
		t.Fatal("内部焦点未暂停")
	}
	toast.SetVisible(false)
	if toast.Visible() || !toast.showing {
		t.Fatal("隐藏应该保留展示")
	}
	host.SetWidget(nil)
	if toast.timer != nil || toast.timeout != nil || toast.mounted {
		t.Fatal("卸载未释放等待资源")
	}
	toast.Close()
}

func TestToastSurfaceReservesShadowAndClamps(t *testing.T) {
	useProgressApplication(t, &progressTestApplication{sheet: style.Sheet(style.Name("toast").Shadow(style.Shadow{Color: color.Black, BlurRadius: 10, Offset: geometry.Point{Y: 4}}))})
	toast := NewToast()
	toast.Show(splitTestChild(80, 20), 0)
	m := toast.Measure(layout.Unbounded())
	// 三 sigma 支持为 15 DIP，外加 1 DIP AA；Y 偏移 4 保守预留两侧 20。
	if m.Width != 136 || m.Height != 84 {
		t.Fatalf("阴影与 Padding 未计入测量：%+v", m)
	}
	toast.Arrange(geometry.Rect(0, 0, m.Width, m.Height))
	if toast.Child().Rect() != geometry.Rect(28, 32, 80, 20) {
		t.Fatalf("内容未避让阴影范围：%+v", toast.Child().Rect())
	}
	toast.Arrange(geometry.Rect(0, 0, 10, 10))
	if r := toast.Child().Rect(); r.Width < 0 || r.Height < 0 {
		t.Fatal("小尺寸产生负数内容区域")
	}
}

func TestToastClosedCallbackReplacementWins(t *testing.T) {
	toast := NewToast()
	toast.Show(gui.NewLabel("first"), 0)
	called := 0
	toast.ConnectClosed(func() { toast.Show(gui.NewLabel("replacement"), 0) })
	toast.ConnectClosed(func() { called++ })
	toast.Close()
	if !toast.Visible() || called != 0 {
		t.Fatal("关闭回调替换后仍执行旧请求回调")
	}
}
