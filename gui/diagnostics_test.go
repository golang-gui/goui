package gui

import (
	"reflect"
	"runtime"
	"testing"
	"time"
	"weak"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/internal/diagnostic"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/style"
)

func startTestDiagnostics(t *testing.T, widgets bool) (*diagnostic.Session, *application) {
	t.Helper()
	a := &application{}
	useTestApplication(t, a)
	d, err := diagnostic.StartCapture(a, diagnostic.Options{Widgets: widgets})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.Stop)
	return d, a
}

func TestDiagnosticsCountsActualWorkAndFreezesCopies(t *testing.T) {
	d, a := startTestDiagnostics(t, true)
	w := &window{rootBase: rootBase{app: a}}
	widget := &mockWidget{size: geometry.Size{Width: 200, Height: 40}}
	w.SetWidget(widget)
	t.Cleanup(func() { w.SetWidget(nil) })
	c := layout.Tight(widget.size)
	measureWidget(widget, c)
	measureWidget(widget, c)
	widget.Arrange(geometry.Rect(0, 0, 200, 40))
	p := newPainter(&testGraphicsPainter{}, widget.Rect(), 1)
	paintWidget(widget, p)
	p.offscreen = true
	paintWidget(widget, p)
	widget.RequestPaint()
	widget.RequestLayout()
	s := d.Snapshot()
	if len(s.Widgets) != 1 {
		t.Fatal("控件计数丢失", s)
	}
	v := s.Widgets[0]
	if v.MeasureQueries != 2 || v.MeasureHits != 1 || v.MeasureCalls != 1 || v.PaintCalls != 1 || v.OffscreenPaintCalls != 1 || v.StyleChangedCalls != 1 || v.PaintRequests != 1 || v.LayoutRequests != 1 {
		t.Fatalf("缓存查询与实际工作混淆：%+v", v)
	}
	if v.Type != "*gui.mockWidget" || v.Bounds != widget.Rect() || v.RootRef == 0 {
		t.Fatal("诊断身份或坐标错误", v)
	}
	s.Widgets[0].PaintCalls = 999
	if d.Snapshot().Widgets[0].PaintCalls != 1 {
		t.Fatal("查询借出了内部状态")
	}
	d.Stop()
	frozen := d.Snapshot()
	measureWidget(widget, c)
	widget.RequestPaint()
	if !reflect.DeepEqual(frozen, d.Snapshot()) || diagnostic.Current() != nil || d.Source() != nil || d.Owner() != nil {
		t.Fatal("停止未冻结或未释放记录")
	}
	frozen.Widgets[0].PaintCalls = 888
	if d.Snapshot().Widgets[0].PaintCalls != 1 {
		t.Fatal("冻结结果不是独立副本")
	}
}

func TestDiagnosticsStartStopNeverSchedulesFrames(t *testing.T) {
	a := &application{}
	requester := &frameRequesterTest{}
	w := &window{rootBase: rootBase{app: a, frameRequester: requester}}
	a.windows = []*window{w}
	b := &w.rootBase
	d, err := diagnostic.StartCapture(a, diagnostic.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Stop()
	if _, err := diagnostic.StartCapture(a, diagnostic.Options{}); err == nil {
		t.Fatal("重复采集被接受")
	}
	for range 10 {
		d.Snapshot()
	}
	d.Stop()
	if requester.requests != 0 || b.frameTimer != nil || b.framePending || b.paintDirty || b.layoutDirty || len(d.Snapshot().Frames) != 0 || d.Snapshot().Options.Widgets {
		t.Fatal("诊断制造帧或擅自开启控件统计")
	}
}

type diagnosticPanicWidget struct{ mockWidget }

func (*diagnosticPanicWidget) Paint(Painter) { panic("diagnostic paint regression") }

type diagnosticCallbackWidget struct {
	mockWidget
	paint func()
}

func (w *diagnosticCallbackWidget) Paint(Painter) { w.paint() }

func TestDiagnosticsKeepsEndReceiverAcrossCallback(t *testing.T) {
	d, a := startTestDiagnostics(t, false)
	ends := 0
	backend := &diagnosticsEndPainter{inspect: func() { ends++ }}
	b := &rootBase{app: a, painter: backend, width: 20, height: 20}
	w := &diagnosticCallbackWidget{paint: func() { b.painter = nil }}
	w.Arrange(geometry.Rect(0, 0, 20, 20))
	b.paintFrame(w, style.Style{})
	if ends != 1 || !d.Snapshot().Frames[0].EndReturned {
		t.Fatal("统计改变了延迟 End 的接收者或调用次数")
	}
}

func TestDiagnosticsPaintPanicIsNotCompleted(t *testing.T) {
	d, a := startTestDiagnostics(t, false)
	b := &rootBase{app: a, painter: &testGraphicsPainter{}, width: 20, height: 20}
	w := &diagnosticPanicWidget{}
	w.Arrange(geometry.Rect(0, 0, 20, 20))
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("诊断吞掉绘制 panic")
			}
		}()
		b.paintFrame(w, style.Style{})
	}()
	s := d.Snapshot()
	if len(s.Frames) != 1 || !s.Frames[0].EndReturned || s.Frames[0].Completed || s.Roots[0].Completed != 0 || s.Roots[0].Aborted != 1 {
		t.Fatal("清理调用 End 被误判为成功绘制", s)
	}
}

func TestDiagnosticsRootBoundAndClosedMetadata(t *testing.T) {
	session, a := startTestDiagnostics(t, false)
	d := diagnosticCollectorFor(session)
	for range diagnosticRoots {
		w := &window{rootBase: rootBase{app: a}, id: "root", title: "diagnostic"}
		a.windows = append(a.windows, w)
		d.root(&w.rootBase)
	}
	if d.root(&rootBase{app: a}) != nil || d.droppedRoots != 1 {
		t.Fatal("Root 记录未遵守上界")
	}
	session.Snapshot() // 保存原生对象消失前的纯值元数据。
	a.windows[0].frameDestroyed = true
	if d.root(&rootBase{app: a}) != nil || len(d.roots) != diagnosticRoots {
		t.Fatal("记录路径在达到 Root 上限时扫描并清理了表")
	}
	s := session.Snapshot()
	if len(s.Roots) != diagnosticRoots || !s.Roots[0].Closed || !s.Roots[0].Idle || s.Roots[0].ID != "root" {
		t.Fatal("关闭 Root 的元数据丢失", s.Roots)
	}
	if d.root(&rootBase{app: a}) == nil || len(d.roots) != diagnosticRoots {
		t.Fatal("没有复用已关闭 Root 的统计容量")
	}
}

type diagnosticGetterWidget struct{ WidgetBase }

func (*diagnosticGetterWidget) Parent() Widget           { panic("query called user Parent") }
func (*diagnosticGetterWidget) Root() Root               { panic("query called user Root") }
func (*diagnosticGetterWidget) Rect() geometry.Rectangle { panic("query called user Rect") }

func TestDiagnosticsMetadataUsesOnlyCachedFields(t *testing.T) {
	s, app := startTestDiagnostics(t, true)
	d := diagnosticCollectorFor(s)
	root := &window{rootBase: rootBase{app: app}}
	parent, child := &diagnosticGetterWidget{}, &diagnosticGetterWidget{}
	parent.parentRoot, parent.self, parent.rect = root, parent, geometry.Rect(20, 30, 100, 100)
	child.parentWidget, child.self, child.rect = parent, child, geometry.Rect(3, 4, 10, 12)
	d.root(&root.rootBase)
	d.widget(&child.WidgetBase)
	snapshot := s.Snapshot()
	if len(snapshot.Widgets) != 1 || snapshot.Widgets[0].Bounds != geometry.Rect(23, 34, 10, 12) || snapshot.Widgets[0].RootRef == 0 {
		t.Fatal("只读缓存的几何或宿主归属不正确", snapshot)
	}
}

func TestDiagnosticsArrangeDoesNotCreateRecords(t *testing.T) {
	button, overlay := NewButton(), NewOverlay()
	s, _ := startTestDiagnostics(t, true)
	for range 3 {
		button.Arrange(geometry.Rect(1, 2, 30, 40))
		overlay.Arrange(geometry.Rect(2, 3, 40, 50))
	}
	if s.Source() != nil || button.Rect() != geometry.Rect(1, 2, 30, 40) || overlay.Rect() != geometry.Rect(2, 3, 40, 50) {
		t.Fatal("Arrange 创建了诊断记录或分配行为改变")
	}
}

func TestDiagnosticsPopoverLayoutPhaseEndsBeforePainting(t *testing.T) {
	p, native, _, app := placementFixture(t, geometry.Rect(0, 0, 1000, 1000))
	app.windows[0].app, p.app = app, app // 补齐真实窗口具备的应用归属。
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	settlePlacement(t, p)
	s, err := diagnostic.StartCapture(app, diagnostic.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	// 查询加入已知耗时，Windows 的较粗时钟粒度也能区分零工作与漏记。
	native.onQuery = func() { time.Sleep(2 * time.Millisecond) }
	var duringPaint time.Duration
	content := &diagnosticCallbackWidget{paint: func() {
		duringPaint = p.frameCapture.Layout
		time.Sleep(time.Millisecond)
	}}
	content.size = geometry.Size{Width: 30, Height: 20}
	p.SetWidget(content)
	p.paint()
	frames := s.Snapshot().Frames
	if len(frames) != 1 || !frames[0].Completed || frames[0].Layout <= 0 || frames[0].Layout != duringPaint || frames[0].Paint < time.Millisecond {
		t.Fatal("Popover 布局时段覆盖了后续绘制", frames)
	}
	native.err = platform.ErrUnavailable
	p.requestLayout()
	p.paint()
	frames = s.Snapshot().Frames
	if len(frames) != 2 || frames[1].Completed || frames[1].Layout <= 0 {
		t.Fatal("Popover 布局失败未保留中断阶段", frames)
	}
}

func recordUnownedDiagnostics(d *diagnosticCollector, a *application) (weak.Pointer[rootBase], weak.Pointer[WidgetBase]) {
	b, w := &rootBase{app: a}, &WidgetBase{}
	d.root(b)
	d.widget(w)
	return weak.Make(b), weak.Make(w)
}

func TestDiagnosticsDoesNotKeepGUIObjectsAlive(t *testing.T) {
	d, a := startTestDiagnostics(t, true)
	r, w := recordUnownedDiagnostics(diagnosticCollectorFor(d), a)
	for range 10 {
		runtime.GC()
		if r.Value() == nil && w.Value() == nil {
			break
		}
	}
	if r.Value() != nil || w.Value() != nil {
		t.Fatal("诊断记录保活了无主的 GUI 对象")
	}
	s := d.Snapshot()
	if len(s.Widgets) != 0 || len(s.Roots) != 1 || !s.Roots[0].Closed {
		t.Fatal("失效弱引用仍被发布为活动对象", s)
	}
}

type diagnosticsEndPainter struct {
	testGraphicsPainter
	inspect func()
}

func (p *diagnosticsEndPainter) End() { p.inspect(); time.Sleep(time.Millisecond) }

func TestDiagnosticsFrameIncludesEndAndAborts(t *testing.T) {
	d, a := startTestDiagnostics(t, false)
	backend := &diagnosticsEndPainter{inspect: func() {
		if len(d.Snapshot().Frames) != 0 {
			t.Fatal("在 End 返回前发布了完成帧")
		}
	}}
	b := &rootBase{app: a, painter: backend, width: 200, height: 40}
	w := &mockWidget{size: geometry.Size{Width: 200, Height: 40}}
	w.Arrange(geometry.Rect(0, 0, 200, 40))
	b.paintFrame(w, style.Style{})
	s := d.Snapshot()
	if len(s.Frames) != 1 || !s.Frames[0].EndReturned || !s.Frames[0].Completed || s.Frames[0].End < time.Millisecond || s.Frames[0].Total < s.Frames[0].End || s.Roots[0].Completed != 1 {
		t.Fatal("帧完成口径错误", s)
	}
	if !b.beginFrame() {
		t.Fatal("不能开始取消帧")
	}
	b.endFrame()
	s = d.Snapshot()
	if len(s.Frames) != 2 || s.Frames[1].EndReturned || s.Roots[0].Aborted != 1 || s.Roots[0].Completed != 1 {
		t.Fatal("取消帧计入完成帧率", s)
	}
}

func TestDiagnosticsHistoryIdleAndMidFrameStop(t *testing.T) {
	d, a := startTestDiagnostics(t, false)
	b := &rootBase{app: a, painter: &testGraphicsPainter{}, width: 20, height: 20}
	w := &mockWidget{size: geometry.Size{Width: 20, Height: 20}}
	w.Arrange(geometry.Rect(0, 0, 20, 20))
	for range diagnostic.HistoryLimit + 7 {
		b.paintFrame(w, style.Style{})
	}
	s := d.Snapshot()
	if len(s.Frames) != diagnostic.HistoryLimit || s.Roots[0].Frames != diagnostic.HistoryLimit+7 || len(s.Widgets) != 0 {
		t.Fatal("历史无界或默认逐控件采集", s)
	}
	idle := diagnosticCollectorFor(d).Snapshot(time.Now().Add(2 * time.Second))
	if !idle.Roots[0].Idle || idle.Roots[0].FPS != 0 {
		t.Fatal("空闲被误报为低帧率", idle.Roots)
	}
	b.frames.Connect(func(time.Time) { d.Stop() })
	b.paintFrame(w, style.Style{})
	if len(d.Snapshot().Frames) != diagnostic.HistoryLimit || d.Snapshot().Roots[0].Frames != diagnostic.HistoryLimit+7 || b.frameCapture != nil {
		t.Fatal("停止后发表了半帧或残留采集状态")
	}
}

func TestDiagnosticsWidgetBoundAndDestroyedMetadata(t *testing.T) {
	session, _ := startTestDiagnostics(t, true)
	d := diagnosticCollectorFor(session)
	keep := make([]*WidgetBase, diagnosticWidgets+1)
	for i := range keep {
		keep[i] = new(WidgetBase)
		d.widget(keep[i])
	}
	if len(d.widgets) != diagnosticWidgets || d.droppedWidgets != 1 {
		t.Fatal("逐控件记录无界")
	}
	keep[0].destroyed = true
	if d.widget(keep[len(keep)-1]) != nil || len(d.widgets) != diagnosticWidgets {
		t.Fatal("记录路径在达到上限时扫描并清理了表")
	}
	session.Snapshot()
	if len(d.widgets) != diagnosticWidgets-1 {
		t.Fatal("销毁对象仍作为活动控件发布")
	}
	d.widget(keep[len(keep)-1])
	if len(d.widgets) != diagnosticWidgets {
		t.Fatal("清理后未恢复记录容量")
	}
}

func TestDiagnosticsDisabledHotPathDoesNotAllocate(t *testing.T) {
	w := &mockWidget{size: geometry.Size{Width: 10, Height: 10}}
	c := layout.Tight(w.size)
	measureWidget(w, c)
	if n := testing.AllocsPerRun(1000, func() { measureWidget(w, c); w.Arrange(geometry.Rect(0, 0, 10, 10)) }); n != 0 {
		t.Fatal("关闭状态引入分配", n)
	}
}

func BenchmarkDiagnosticsMeasure(b *testing.B) {
	for _, mode := range []string{"disabled", "frames", "widgets"} {
		b.Run(mode, func(b *testing.B) {
			w := &mockWidget{size: geometry.Size{Width: 200, Height: 40}}
			c := layout.Tight(w.size)
			measureWidget(w, c)
			if mode != "disabled" {
				d, err := diagnostic.StartCapture(&application{}, diagnostic.Options{Widgets: mode == "widgets"})
				if err != nil {
					b.Fatal(err)
				}
				defer d.Stop()
				measureWidget(w, c) // 预热记录，不计入热路径分配。
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				measureWidget(w, c)
			}
			b.StopTimer()
		})
	}
}
