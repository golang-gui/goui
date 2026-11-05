package gui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"runtime/trace"
	"slices"
	"strings"
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

func TestDiagnosticsReadRootKeepsCompletedFrameAndIdentity(t *testing.T) {
	d, a := startTestDiagnostics(t, true)
	w := &window{rootBase: rootBase{app: a}}
	other := &window{rootBase: rootBase{app: a}}
	if _, ok := d.ReadRoot(w); ok || d.Source() != nil {
		t.Fatal("单根查询提前创建了采集器")
	}
	r := diagnosticRoot(&w.rootBase)
	r.Backend = "test-backend"
	f := d.BeginFrame(r)
	f.Paint = time.Millisecond
	f.Completed, f.EndReturned = true, true
	f.Finish()
	completed := r.LastFrame()
	// 其它窗口的完成帧以及当前窗口的未完成帧，均不能覆盖已完成样本。
	f = d.BeginFrame(diagnosticRoot(&other.rootBase))
	f.Paint, f.Completed, f.EndReturned = time.Second, true, true
	f.Finish()
	f = d.BeginFrame(r)
	f.Paint = 2 * time.Second
	sample, ok := d.ReadRoot(w)
	if !ok || sample.Root.Ref != r.Ref || sample.Root.Backend != "test-backend" || sample.Frame != completed {
		t.Fatal("单根样本混入当前帧或其它窗口", sample)
	}
	f.Finish() // 中断帧也不得取代最近完成帧。
	sample, _ = d.ReadRoot(w)
	if sample.Frame != completed {
		t.Fatal("中断帧覆盖了已完成样本")
	}
	collector := d.Source().(*diagnosticCollector)
	roots := len(collector.roots)
	for _, target := range []any{nil, struct{}{}, &window{}, &window{rootBase: rootBase{app: a}}} {
		if _, ok := d.ReadRoot(target); ok {
			t.Fatal("读取了错误归属或没有记录的根")
		}
	}
	if len(collector.roots) != roots {
		t.Fatal("单根查询创建了额外记录")
	}
	if allocations := testing.AllocsPerRun(100, func() { _, _ = d.ReadRoot(w) }); allocations != 0 {
		t.Fatal("单根查询产生堆分配", allocations)
	}
	d.Stop()
	if _, ok := d.ReadRoot(w); ok {
		t.Fatal("停止后仍可访问采集器")
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

// 仅比较 trace 关闭时的相同帧工作，不把录制、文件写入或真实绘制后端计入基准。
func BenchmarkDiagnosticsFrame(b *testing.B) {
	b.Run("Lifecycle", func(b *testing.B) {
		root := &rootBase{}
		b.ReportAllocs()
		for range b.N {
			if !root.beginFrame() {
				b.Fatal("帧被意外拒绝")
			}
			observation := root.observeLayout()
			observation.end()
			root.endFrame()
		}
	})
	b.Run("Paint", func(b *testing.B) {
		root := &rootBase{painter: &testGraphicsPainter{}, width: 20, height: 20}
		widget := &mockWidget{size: geometry.Size{Width: 20, Height: 20}}
		widget.Arrange(geometry.Rect(0, 0, 20, 20))
		root.paintFrame(widget, style.Style{})
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			root.paintFrame(widget, style.Style{})
		}
	})
}

type traceTestPainter struct {
	testGraphicsPainter
	begin, end int
	panicAt    string
}

func (p *traceTestPainter) Begin(float32, float32, float32) {
	p.begin++
	if p.panicAt == "begin" {
		panic("trace test begin")
	}
}

func (p *traceTestPainter) End() {
	p.end++
	if p.panicAt == "end" {
		panic("trace test end")
	}
}

type traceLayoutPanicWidget struct{ mockWidget }

func (*traceLayoutPanicWidget) Arrange(geometry.Rectangle) { panic("trace test layout") }

// 子进程独占标准库的全局 trace 开关，兼容父测试本身使用 go test -trace。
// 使用标准工具解析文件，按 Task 校验真实 Region 顺序及闭合，不检查原始字节字符串。
func TestDiagnosticsRuntimeTrace(t *testing.T) {
	const child = "GOUI_TEST_RUNTIME_TRACE"
	if os.Getenv(child) != "1" {
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(executable, "-test.run=^TestDiagnosticsRuntimeTrace$", "-test.v")
		cmd.Env = append(os.Environ(), child+"=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("标准 trace 子进程失败：%v\n%s", err, output)
		}
		return
	}
	if trace.IsEnabled() || diagnostic.Current() != nil {
		t.Fatal("子进程未隔离全局采集状态")
	}
	checkDisabled := func() {
		root := &rootBase{}
		if n := testing.AllocsPerRun(100, func() {
			if root.beginFrame() {
				observation := root.observeLayout()
				observation.end()
				root.endFrame()
			}
		}); n != 0 || root.frameTrace != nil {
			t.Fatalf("trace 关闭时创建了状态或分配：%g", n)
		}
	}
	checkDisabled()
	// 先准备 Popover，避免把其初次定位所需的其它帧混进下面的独立预期。
	popup, _, _, _ := placementFixture(t, geometry.Rect(0, 0, 1000, 1000))
	if err := popup.Show(); err != nil {
		t.Fatal(err)
	}
	settlePlacement(t, popup)
	path := filepath.Join(t.TempDir(), "frames.trace")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	t.Cleanup(trace.Stop)
	// 开启发生在一帧回调内部：这一帧不产生半套标记，后续帧才完整记录。
	starting := &rootBase{}
	starting.ConnectFrame(func(time.Time) {
		if err := trace.Start(file); err != nil {
			t.Fatal(err)
		}
	})
	if !starting.beginFrame() {
		t.Fatal("未能启动测试帧")
	}
	starting.endFrame()
	if starting.frameTrace != nil {
		t.Fatal("追踪在帧中途开启时创建了半帧状态")
	}
	full := []string{"goui.callbacks", "goui.styles", "goui.layout", "goui.begin", "goui.paint", "goui.end"}
	expected := make(map[string][][]string)
	var keep []*rootBase
	run := func(stage string) *rootBase {
		backend := &traceTestPainter{panicAt: stage}
		root := &rootBase{painter: backend, width: 20, height: 20, layoutDirty: true}
		widget := &diagnosticCallbackWidget{paint: func() {
			if stage == "paint" {
				panic("trace test paint")
			}
		}}
		widget.size = geometry.Size{Width: 20, Height: 20}
		var content Widget = widget
		phases := full
		if stage == "callbacks" || stage == "cancel" {
			root.ConnectFrame(func(time.Time) {
				if stage == "callbacks" {
					panic("trace test callback")
				}
				root.destroyFrames()
			})
			phases = full[:1]
			if stage == "cancel" {
				phases = full[:2]
			}
		} else if stage == "begin" {
			phases = full[:4]
		} else if stage == "layout" {
			content = &traceLayoutPanicWidget{}
			phases = full[:3]
		}
		var recovered any
		func() {
			defer func() { recovered = recover() }()
			root.paintFrame(content, style.Style{})
		}()
		wantPanic := stage != "normal" && stage != "cancel"
		wantBegin, wantEnd := 1, 1
		if stage == "callbacks" || stage == "cancel" || stage == "layout" {
			wantBegin, wantEnd = 0, 0
		} else if stage == "begin" {
			wantEnd = 0
		}
		if (recovered != nil) != wantPanic || backend.begin != wantBegin || backend.end != wantEnd || root.frameTrace != nil {
			t.Fatalf("%s：观测改变正常／异常路径：panic=%v begin=%d end=%d", stage, recovered, backend.begin, backend.end)
		}
		expected[fmt.Sprintf("%p", root)] = [][]string{phases}
		keep = append(keep, root)
		return root
	}
	for _, stage := range []string{"normal", "callbacks", "cancel", "layout", "begin", "paint", "end"} {
		run(stage)
	}
	// Capture 开启与停止均不控制标准 trace；原有诊断记录仍正常产生。
	session, app := startTestDiagnostics(t, false)
	root := run("normal")
	root.app, root.layoutDirty = app, true
	root.paintFrame(nil, style.Style{})
	if len(session.Snapshot().Frames) != 1 {
		t.Fatal("标准 trace 干扰 Capture")
	}
	session.Stop()
	// 同一根会有两次记录，分别按根内的帧顺序校验。
	expected[fmt.Sprintf("%p", root)] = append(expected[fmt.Sprintf("%p", root)], append(slices.Clone(full[:2]), full[3:]...))
	popup.layoutDirty = true
	popup.paint()
	expected[fmt.Sprintf("%p", &popup.rootBase)] = [][]string{full}
	// 在回调中同步绘制另一个根，标准工具必须仍能解析合法的区域嵌套。
	outer := &rootBase{painter: &traceTestPainter{}, width: 20, height: 20}
	outer.ConnectFrame(func(time.Time) { run("normal") })
	outer.paintFrame(nil, style.Style{})
	expected[fmt.Sprintf("%p", outer)] = [][]string{append(slices.Clone(full[:2]), full[3:]...)}
	keep = append(keep, outer)
	trace.Stop()
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	checkDisabled()
	checkTraceFrames(t, dumpRuntimeTrace(t, path), expected)
	// 回调中停止采集可截断当前 Region，但不能阻止绘制、End 或私有状态清理。
	partial := filepath.Join(t.TempDir(), "partial.trace")
	partialFile, err := os.Create(partial)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = partialFile.Close() })
	if err := trace.Start(partialFile); err != nil {
		t.Fatal(err)
	}
	backend := &traceTestPainter{}
	stopping := &rootBase{painter: backend, width: 20, height: 20}
	stopping.ConnectFrame(func(time.Time) { trace.Stop() })
	stopping.paintFrame(nil, style.Style{})
	if trace.IsEnabled() || stopping.frameTrace != nil || stopping.framePainting || backend.begin != 1 || backend.end != 1 {
		t.Fatal("帧中停止追踪改变了正常绘制或残留了上下文")
	}
	if err := partialFile.Close(); err != nil {
		t.Fatal(err)
	}
	partialOutput := dumpRuntimeTrace(t, partial)
	if !strings.Contains(partialOutput, `Type="goui.frame"`) || !strings.Contains(partialOutput, `Type="goui.callbacks"`) || strings.Contains(partialOutput, " TaskEnd ") {
		t.Fatal("中途停止的追踪未保留合法的截断记录")
	}
	checkDisabled()
	runtime.KeepAlive(keep)
}

func dumpRuntimeTrace(t *testing.T, path string) string {
	t.Helper()
	help, _ := exec.Command("go", "tool", "trace", "-help").CombinedOutput()
	mode := "-d=1" // Go 1.24 使用数值，较新版本使用命名模式。
	if strings.Contains(string(help), "-d=mode") {
		mode = "-d=parsed"
	}
	output, err := exec.Command("go", "tool", "trace", mode, path).CombinedOutput()
	if err != nil {
		t.Fatalf("标准工具无法解析追踪：%v\n%s", err, output)
	}
	return string(output)
}

func checkTraceFrames(t *testing.T, output string, expected map[string][][]string) {
	t.Helper()
	ids := regexp.MustCompile(` (?:ID|Task)=([0-9]+)`)
	types := regexp.MustCompile(` Type="([^"]*)"`)
	logs := regexp.MustCompile(` Message="([^"]*)"`)
	tasks := make(map[string][]string)
	roots := make(map[string]string)
	ended := make(map[string]bool)
	var order []string
	for _, line := range strings.Split(output, "\n") {
		id := ids.FindStringSubmatch(line)
		if len(id) == 0 {
			continue
		}
		kind := types.FindStringSubmatch(line)
		switch {
		case strings.Contains(line, " TaskBegin ") && len(kind) > 0 && kind[1] == "goui.frame":
			tasks[id[1]] = nil
			order = append(order, id[1])
		case strings.Contains(line, " Category=\"goui.root\""):
			if message := logs.FindStringSubmatch(line); len(message) > 0 {
				roots[id[1]] = message[1]
			}
		case strings.Contains(line, " RegionBegin ") && len(kind) > 0:
			tasks[id[1]] = append(tasks[id[1]], "+"+kind[1])
		case strings.Contains(line, " RegionEnd ") && len(kind) > 0:
			tasks[id[1]] = append(tasks[id[1]], "-"+kind[1])
		case strings.Contains(line, " TaskEnd "):
			ended[id[1]] = true
		}
	}
	for _, id := range order {
		root := roots[id]
		if len(expected[root]) == 0 || !ended[id] {
			t.Fatalf("未预期或未闭合的帧：task=%s root=%s ended=%v", id, root, ended[id])
		}
		var want []string
		for _, phase := range expected[root][0] {
			want = append(want, "+"+phase, "-"+phase)
		}
		if !slices.Equal(tasks[id], want) {
			t.Fatalf("帧阶段不符：task=%s events=%v want=%v", id, tasks[id], want)
		}
		expected[root] = expected[root][1:]
	}
	for root, remaining := range expected {
		if len(remaining) != 0 {
			t.Fatalf("根 %s 缺少追踪阶段：%v", root, remaining)
		}
	}
}
