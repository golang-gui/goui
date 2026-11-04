package diagnostics

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/internal/diagnostic"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
)

// 只替换 HUD 真正使用的能力，不创建原生窗口或字体资源。
type hudApplication struct {
	gui.Application
	windows []gui.Window
	typo    hudTypography
}

func (a *hudApplication) Windows() []gui.Window          { return a.windows }
func (a *hudApplication) Typography() typography.Context { return &a.typo }

type hudTypography struct {
	typography.Context
	created, destroyed int
	fail               bool
}

func (c *hudTypography) NewTextLayout(text string, format typography.TextFormat, _, _ float32) (typography.TextLayout, error) {
	if c.fail {
		return nil, errors.New("font unavailable")
	}
	c.created++
	return &hudLayout{owner: c, text: text}, nil
}

type hudLayout struct {
	typography.TextLayout
	owner *hudTypography
	text  string
}

func (l *hudLayout) Text() string                    { return l.text }
func (l *hudLayout) MeasureSize() (float32, float32) { return 200, 90 }
func (l *hudLayout) Destroy()                        { l.owner.destroyed++ }

type hudWindow struct {
	gui.Window
	paints     int
	requestErr error
	after      signal.Signal2[gui.Painter, geometry.Rectangle]
	destroy    signal.Signal0
}

func (w *hudWindow) RequestPaint() error {
	w.paints++
	return w.requestErr
}
func (w *hudWindow) ConnectAfterPaint(fn func(gui.Painter, geometry.Rectangle)) signal.Handle {
	return w.after.Connect(fn)
}
func (w *hudWindow) ConnectDestroy(fn func()) signal.Handle { return w.destroy.Connect(fn) }

type hudSource struct {
	diagnostic.Source
	queries int
	sample  diagnostic.RootSample
}

func (s *hudSource) Snapshot(time.Time) diagnostic.GUIData { return diagnostic.GUIData{} }
func (s *hudSource) ReadRoot(any, time.Time) (diagnostic.RootSample, bool) {
	s.queries++
	return s.sample, true
}

type hudPainter struct {
	gui.Painter
	draws, saves int
}

func (p *hudPainter) Save()                                                     { p.saves++ }
func (p *hudPainter) Restore()                                                  { p.saves-- }
func (p *hudPainter) SetClipRect(geometry.Rectangle)                            {}
func (p *hudPainter) FillRoundRect(geometry.Rectangle, float32, graphics.Brush) {}
func (p *hudPainter) DrawTextLayout(geometry.Point, typography.TextLayout)      { p.draws++ }

func startHUDTest(t *testing.T) (*Capture, *hudApplication, *hudWindow) {
	t.Helper()
	w := &hudWindow{}
	a := &hudApplication{windows: []gui.Window{w}}
	previous := gui.App
	gui.App = a
	t.Cleanup(func() { gui.App = previous })
	c, err := Start(a, Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Stop)
	return c, a, w
}

func TestHUDOwnershipAndCleanup(t *testing.T) {
	c, a, w := startHUDTest(t)
	h, err := c.ShowHUD(w)
	if err != nil {
		t.Fatal(err)
	}
	again, err := c.ShowHUD(w)
	if err != nil || again != h || w.paints != 1 || a.typo.created != 1 {
		t.Fatal("重复 ShowHUD 创建了资源或重绘", err)
	}
	h.SetPosition(geometry.Point{X: 30, Y: 50})
	h.SetPosition(geometry.Point{X: 30, Y: 50})
	if w.paints != 2 {
		t.Fatal("相同位置产生了多余重绘")
	}
	p := &hudPainter{}
	w.after.Emit(p, geometry.Rect(0, 0, 400, 300))
	if p.draws != 1 || p.saves != 0 || w.paints != 2 {
		t.Fatal("HUD 绘制自行请求帧或未恢复状态")
	}
	h.Destroy()
	h.Destroy()
	h.SetPosition(geometry.Point{})
	w.after.Emit(p, geometry.Rect(0, 0, 400, 300))
	if w.paints != 3 || p.draws != 1 || a.typo.destroyed != 1 || len(c.huds) != 0 || c.stopped != nil {
		t.Fatal("销毁未幂等释放资源和连接")
	}
	// 重新显示后，由应用归属退出清理；旧句柄不得影响新采集。
	if _, err := c.ShowHUD(w); err != nil {
		t.Fatal(err)
	}
	diagnostic.StopOwner(a)
	if len(c.huds) != 0 || a.typo.created != a.typo.destroyed {
		t.Fatal("应用退出没有释放 HUD")
	}
}

func TestHUDWindowDestructionAndShowFailure(t *testing.T) {
	c, a, w := startHUDTest(t)
	if _, err := c.ShowHUD(nil); err == nil {
		t.Fatal("接受了空窗口")
	}
	if _, err := c.ShowHUD(&hudWindow{}); err == nil {
		t.Fatal("接受了其它应用的窗口")
	}
	a.typo.fail = true
	if _, err := c.ShowHUD(w); err == nil || w.paints != 0 || len(c.huds) != 0 {
		t.Fatal("字体失败时没有报告错误或留下连接")
	}
	a.typo.fail = false
	w.requestErr = errors.New("surface unavailable")
	if _, err := c.ShowHUD(w); !errors.Is(err, w.requestErr) || len(c.huds) != 0 || a.typo.created != a.typo.destroyed {
		t.Fatal("初次重绘失败未释放 HUD", err)
	}
	w.requestErr = nil
	h, err := c.ShowHUD(w)
	if err != nil {
		t.Fatal(err)
	}
	prior := w.paints
	w.destroy.Emit()
	h.Destroy()
	if h.window != nil || w.paints != prior || a.typo.created != a.typo.destroyed {
		t.Fatal("窗口销毁时仍请求绘制或保留文字资源")
	}
	c.Stop()
	if _, err := c.ShowHUD(w); err == nil {
		t.Fatal("已停止采集仍可显示 HUD")
	}
	var empty *Capture
	if _, err := empty.ShowHUD(w); err == nil {
		t.Fatal("空采集仍可显示 HUD")
	}
}

func TestHUDReusesTextAndThrottlesQueries(t *testing.T) {
	c, a, w := startHUDTest(t)
	source := &hudSource{sample: diagnostic.RootSample{
		Root:  diagnostic.Root{FPS: 60, Backend: "Software"},
		Frame: diagnostic.Frame{Completed: true, Total: 2 * time.Millisecond, Paint: time.Millisecond},
	}}
	c.session.SetSource(source)
	h, err := c.ShowHUD(w)
	if err != nil {
		t.Fatal(err)
	}
	p := &hudPainter{}
	h.draw(p, geometry.Rect(0, 0, 400, 300))
	if source.queries != 1 || a.typo.created != 2 || a.typo.destroyed != 1 {
		t.Fatal("未读取完成帧或更换文字资源")
	}
	// 将下一次更新放到未来，保证该断言不依赖测试机器的速度。
	h.updated = time.Now().Add(time.Hour)
	for range 100 {
		h.draw(p, geometry.Rect(0, 0, 400, 300))
	}
	if source.queries != 1 || a.typo.created != 2 || w.paints != 1 {
		t.Fatal("普通帧重复查询、重建文字或请求重绘")
	}
	h.updated = time.Time{}
	h.draw(p, geometry.Rect(0, 0, 400, 300))
	if source.queries != 2 || a.typo.created != 2 {
		t.Fatal("相同文本未复用排版")
	}
	a.typo.fail = true
	source.sample.Root.FPS = 30
	h.updated = time.Time{}
	h.draw(p, geometry.Rect(0, 0, 400, 300))
	if !strings.Contains(h.layout.Text(), "60.0") || a.typo.destroyed != 1 {
		t.Fatal("暂时字体失败未保留上一份资源")
	}
}

func TestHUDTextReportsCompletedCPUFrames(t *testing.T) {
	text := hudText(diagnostic.RootSample{}, false)
	if !strings.Contains(text, "Draw FPS   --") || !strings.Contains(text, "Frame   --") {
		t.Fatal("缺少样本时虚构帧率或耗时", text)
	}
	sample := diagnostic.RootSample{
		Root:  diagnostic.Root{FPS: 120, Backend: "OpenGL"},
		Frame: diagnostic.Frame{Completed: true, Total: 3 * time.Millisecond, End: time.Millisecond},
	}
	text = hudText(sample, true)
	for _, part := range []string{"120.0", "3.00 ms", "1.00 ms", "OpenGL", "recent CPU frames"} {
		if !strings.Contains(text, part) {
			t.Fatal("HUD 统计口径或单位错误", text)
		}
	}
	sample.Root.Idle = true
	if strings.Contains(hudText(sample, true), "120.0") {
		t.Fatal("闲置时仍报告有效帧率")
	}
}
