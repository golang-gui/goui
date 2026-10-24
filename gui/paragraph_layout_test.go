package gui

import (
	"math"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui/textedit"
	"github.com/golang-gui/goui/platform/typography"
)

type paragraphBoundContext struct {
	typography.Context
	t       *testing.T
	maximum int
	hook    func()
}

func (c *paragraphBoundContext) NewTextLayout(text string, f typography.TextFormat, w, h float32) (typography.TextLayout, error) {
	c.maximum = max(c.maximum, len(text))
	if len(text) > 8192 {
		c.t.Fatalf("原生送测超过上限：%d", len(text))
	}
	native, err := c.Context.NewTextLayout(text, f, w, h)
	if c.hook != nil {
		fn := c.hook
		c.hook = nil
		fn()
	}
	return native, err
}

// 用原生完整段落作独立参考，检查连续块的真实软折点及跨块复杂文本上下文。
func testParagraphLayoutNative(t *testing.T, factory func() (typography.Context, error)) {
	for _, tc := range []struct{ name, text string }{
		{"ASCII", strings.Repeat("a9+Z/Bh4q2AZbng7 latencyword ", 4000)},
		{"Unicode", strings.Repeat("中文 e\u0301 ffi 👩‍💻 word 123 \t", 2400)},
		{"RTL", "עברית " + strings.Repeat("עברית العربية 123 abc ", 1800)},
		{"Isolate", "start \u2067" + strings.Repeat("עברית abc 123 ", 1800) + "\u2069 end"},
		{"Override", "start \u202e" + strings.Repeat("abc 123 العربية עברית e\u0301 ffi \t ", 1800) + "END\u202c"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			context, err := factory()
			if err != nil {
				t.Fatal(err)
			}
			defer context.Destroy()
			model := textedit.NewModel(tc.text)
			format := typography.TextFormat{Font: typography.FontInfo{Family: "monospace", Size: 12}, WrapMode: typography.WrapWordChar, TabWidth: 32}
			reference, err := context.NewTextLayout(tc.text, format, 280, 1<<24)
			if err != nil {
				t.Fatal(err)
			}
			defer reference.Destroy()
			want, wantClusters := reference.MeasureMetrics()
			clustersByStart := map[int]typography.TextCluster{}
			for _, c := range wantClusters {
				clustersByStart[c.Start] = c
			}
			byStart := map[int]typography.TextLine{}
			for _, line := range want {
				byStart[line.Start] = line
			}
			observed := &paragraphBoundContext{Context: context, t: t}
			p := NewParagraphLayout(observed, format, 280, model.Len(), model.Slice)
			defer p.Destroy()
			for i := 0; i < 4; i++ {
				page := p.page(i, time.Now().Add(time.Second))
				if page == nil {
					t.Fatalf("第 %d 块未就绪：%v", i, p.Err())
				}
				_, originY := p.origin(page)
				for _, line := range page.lines {
					r, ok := byStart[line.Start]
					if !ok || r.Length != line.Length || math.Abs(float64(r.Width-line.Width)) > .1 || math.Abs(float64(float32(float64(r.Y)-originY)-line.Y)) > .1 {
						t.Fatalf("块 %d 行 %+v、Y=%g 与完整参考 %+v 不一致", i, line, originY, r)
					}
				}
				for _, c := range page.clusters {
					w, ok := clustersByStart[c.Start]
					if !ok || w.Length != c.Length || w.Direction != c.Direction || math.Abs(float64(w.X-c.X)) > .1 || math.Abs(float64(w.Width-c.Width)) > .1 {
						t.Fatalf("块 %d cluster=%+v 与完整参考 %+v 不一致", i, c, w)
					}
				}
			}
			if tc.name == "ASCII" {
				other := NewParagraphLayout(observed, format, 280, model.Len(), model.Slice)
				observed.hook = other.Destroy
				if other.GeometryAt(0) != nil || len(other.pages) != 0 {
					t.Fatal("销毁后发布了过期原生窗口")
				}
				other.Destroy()
			}
			if observed.maximum > 8192 {
				t.Fatal("原生输入越界")
			}
			view := geometry.Rect(0, 0, 280, 180)
			for i := 0; i < 8 && !p.Prepare(view, true); i++ {
			}
			if p.VisibleRange().End == 0 {
				t.Fatal("首屏没有可见文本")
			}
			if tc.name == "ASCII" {
				at := p.pages[0].range_.End + 5
				if p.GeometryAt(at) == nil {
					t.Fatal("接缝编辑位置未就绪")
				}
				for n := 0; n < 20; n++ {
					edit := textedit.Edit{Range: textedit.Range{Start: at, End: at}, Text: "Q"}
					if err := model.Replace(edit.Range, edit.Text); err != nil {
						t.Fatal(err)
					}
					p.Change(model.Len(), []textedit.Edit{edit})
					at++
					g := p.GeometryAt(at)
					if g == nil || g.Snap(at, false) != at {
						t.Fatalf("接缝连续输入 %d 后光标回退：%d", n, at)
					}
				}
			}
		})
	}
}

// 绘制与命中共用局部度量；不能先平移到数亿 DIP 后再做单精度相减。
type paragraphPaintPositions struct {
	Painter
	points []geometry.Point
}

func (p *paragraphPaintPositions) Save()                          {}
func (p *paragraphPaintPositions) Restore()                       {}
func (p *paragraphPaintPositions) SetClipRect(geometry.Rectangle) {}
func (p *paragraphPaintPositions) DrawTextLayout(at geometry.Point, _ typography.TextLayout) {
	p.points = append(p.points, at)
}

func testParagraphCoordinatesNative(t *testing.T, factory func() (typography.Context, error)) {
	for _, family := range []string{"monospace", "sans"} {
		t.Run(family, func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			context, err := factory()
			if err != nil {
				t.Fatal(err)
			}
			defer context.Destroy()
			const index = 16384
			const base = index * paragraphBlockBytes
			read := func(r textedit.Range) (string, error) {
				const pattern = "i W l M "
				var out strings.Builder
				for at := r.Start; at < r.End; {
					n := min(r.End-at, len(pattern)-at%len(pattern))
					out.WriteString(pattern[at%len(pattern) : at%len(pattern)+n])
					at += n
				}
				return out.String(), nil
			}
			format := typography.TextFormat{Font: typography.FontInfo{Family: family, Size: 12}, WrapMode: typography.WrapNone}
			p := NewParagraphLayout(&paragraphBoundContext{Context: context, t: t}, format, 1<<20, base+2*paragraphBlockBytes, read)
			defer p.Destroy()
			if p.page(0, time.Now().Add(time.Second)) == nil {
				t.Fatal("首窗口失败")
			}
			// 虚拟 ASCII 前缀的状态由样本定义独立确认，无需分配或排版 64 MiB 文本。
			state := paragraphContext{at: base, runes: base, base: '\u200e', strong: '\u200e'}
			p.scanned, p.state = base, state
			p.checkpoints = append(p.checkpoints, state)
			a := p.page(index, time.Now().Add(time.Second))
			b := p.page(index+1, time.Now().Add(time.Second))
			if a == nil || b == nil {
				t.Fatal("远端片段失败", p.Err())
			}
			frame := p.snapshot([]int{index, index + 1})
			if frame.X < 1<<28 {
				t.Fatal("样本未覆盖巨大坐标", frame.X)
			}
			if frame.LineCount() != 1 {
				t.Fatal("水平片段被错误地当成显示行", frame.LineCount())
			}
			text, _ := read(textedit.Range{Start: base, End: base + 2*paragraphBlockBytes})
			reference, err := context.NewTextLayout(text, format, 1<<20, 1<<20)
			if err != nil {
				t.Fatal(err)
			}
			defer reference.Destroy()
			lines, clusters := reference.MeasureMetrics()
			want := textedit.NewGeometry(lines, clusters, len(text), 20)
			for at := paragraphBlockBytes - 24; at < paragraphBlockBytes+24; at++ {
				cell := want.Caret(textedit.Position{Offset: at})
				point := geometry.Point{X: cell.X + .25, Y: cell.Y + cell.Height/2}
				got := frame.Hit(point)
				if got.Offset != base+want.Hit(point).Offset {
					t.Fatalf("接缝命中 at=%d got=%v want=%v", at, got, want.Hit(point))
				}
				caret := frame.Caret(textedit.Position{Offset: base + at})
				if math.Abs(float64(caret.X-cell.X)) > .05 {
					t.Fatalf("光标错位 at=%d got=%v want=%v", at, caret, cell)
				}
			}
			r := textedit.Range{Start: paragraphBlockBytes - 20, End: paragraphBlockBytes + 20}
			got, wantRects := frame.Selection(textedit.Range{Start: base + r.Start, End: base + r.End}), want.Selection(r)
			if len(got) != 1 || len(wantRects) != 1 || math.Abs(float64(got[0].X-wantRects[0].X)) > .05 || math.Abs(float64(got[0].Width-wantRects[0].Width)) > .05 {
				t.Fatalf("连续选区不一致：%v want %v", got, wantRects)
			}
			p.geometry = frame
			p.active = []int{index, index + 1}
			draw := &paragraphPaintPositions{}
			origin := frame.RelativeTo(frame.X-123.25, -37.75)
			p.Paint(draw, origin)
			if len(draw.points) != 2 || draw.points[0] != origin || math.Abs(float64(draw.points[1].X-origin.X-a.width)) > .05 {
				t.Fatal("原生绘制与局部几何未衔接", draw.points, origin, a.width)
			}
		})
	}
}
