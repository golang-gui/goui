//go:build linux || windows || darwin

package typography_test

import (
	"math"
	"strings"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/gui/textedit"
	"github.com/golang-gui/goui/platform/typography"
)

type boundedContext struct {
	typography.Context
	t       *testing.T
	maximum int
}

func (c *boundedContext) NewTextLayout(text string, format typography.TextFormat, width, height float32) (typography.TextLayout, error) {
	c.maximum = max(c.maximum, len(text))
	if len(text) > 8192 {
		c.t.Fatalf("有界段落向原生后端提交了 %d 字节", len(text))
	}
	return c.Context.NewTextLayout(text, format, width, height)
}

// 完整原生段落是独立参考；通过公开几何检查分页保留软折点和双向上下文。
// 私有页缓存、巨大坐标及销毁重入的断言继续留在 gui 的确定性契约测试。
func TestParagraphLayoutNativeReference(t *testing.T) {
	for _, tc := range []struct{ name, text string }{
		{"ASCII", strings.Repeat("a9+Z/Bh4q2AZbng7 latencyword ", 4000)},
		{"Unicode", strings.Repeat("中文 e\u0301 ffi 👩‍💻 word 123 \t", 2400)},
		{"RTL", "עברית " + strings.Repeat("עברית العربية 123 abc ", 1800)},
		{"Isolate", "start \u2067" + strings.Repeat("עברית abc 123 ", 1800) + "\u2069 end"},
		{"Override", "start \u202e" + strings.Repeat("abc 123 العربية עברית e\u0301 ffi \t ", 1800) + "END\u202c"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			context := nativeContext(t)
			format := typography.TextFormat{Font: typography.FontInfo{Family: "monospace", Size: 12}, WrapMode: typography.WrapWordChar, TabWidth: 32}
			reference, err := context.NewTextLayout(tc.text, format, 280, 1<<24)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(reference.Destroy)
			lines, clusters := reference.MeasureMetrics()
			byStart := map[int]typography.TextLine{}
			for _, line := range lines {
				byStart[line.Start] = line
			}
			want := textedit.NewGeometry(lines, clusters, len(tc.text), 20)
			observed := &boundedContext{Context: context, t: t}
			model := textedit.NewModel(tc.text)
			p := gui.NewParagraphLayout(observed, format, 280, model.Len(), model.Slice)
			t.Cleanup(p.Destroy)
			for block := 0; block < 4; block++ {
				g := paragraphAt(t, p, block*4096)
				for i := 0; i < g.LineCount(); i++ {
					line := g.LineMetrics(i)
					r, ok := byStart[line.Start]
					if !ok || r.Length != line.Length || math.Abs(float64(r.Width-line.Width)) > .1 || math.Abs(float64(r.Y)-(g.Y+float64(line.Y))) > .1 {
						t.Fatalf("块 %d 的行度量与完整参考不同：%+v，原点 %g，参考 %+v", block, line, g.Y, r)
					}
					pos := textedit.Position{Offset: line.Start}
					got, expected := g.Caret(pos), want.Caret(pos)
					if math.Abs(g.X+float64(got.X)-float64(expected.X)) > .1 || math.Abs(g.Y+float64(got.Y)-float64(expected.Y)) > .1 {
						t.Fatalf("块 %d 光标与原生参考不同：%v，参考 %v", block, got, expected)
					}
				}
			}
			for i := 0; i < 8 && !p.Prepare(geometry.Rect(0, 0, 280, 180), true); i++ {
			}
			if p.VisibleRange().End == 0 || observed.maximum > 8192 {
				t.Fatal("首屏或有界输入错误", p.VisibleRange(), observed.maximum)
			}
			if tc.name == "ASCII" {
				at := 4096 + 5
				for n := 0; n < 20; n++ {
					edit := textedit.Edit{Range: textedit.Range{Start: at, End: at}, Text: "Q"}
					if err := model.Replace(edit.Range, edit.Text); err != nil {
						t.Fatal(err)
					}
					p.Change(model.Len(), []textedit.Edit{edit})
					at++
					if g := paragraphAt(t, p, at); g.Snap(at, false) != at {
						t.Fatal("接缝连续输入后光标回退", n, at)
					}
				}
			}
		})
	}
}

func paragraphAt(t *testing.T, p *gui.ParagraphLayout, offset int) *gui.ParagraphGeometry {
	t.Helper()
	for attempt := 0; attempt < 100; attempt++ {
		if g := p.GeometryAt(offset); g != nil {
			return g
		}
		if err := p.Err(); err != nil {
			t.Fatal(err)
		}
	}
	t.Fatalf("字符 %d 的原生几何未在有界次数内就绪", offset)
	return nil
}

type paragraphDraw struct {
	gui.Painter
	points  []geometry.Point
	layouts []typography.TextLayout
}

func (*paragraphDraw) Save()                          {}
func (*paragraphDraw) Restore()                       {}
func (*paragraphDraw) SetClipRect(geometry.Rectangle) {}
func (p *paragraphDraw) DrawTextLayout(at geometry.Point, l typography.TextLayout) {
	p.points = append(p.points, at)
	p.layouts = append(p.layouts, l)
}

func TestParagraphNativeSeamCoordinates(t *testing.T) {
	for _, family := range []string{"monospace", "sans"} {
		t.Run(family, func(t *testing.T) {
			context := nativeContext(t)
			text := strings.Repeat("i W l M ", 1024)
			format := typography.TextFormat{Font: typography.FontInfo{Family: family, Size: 12}, WrapMode: typography.WrapNone}
			reference, err := context.NewTextLayout(text, format, 1<<20, 1<<20)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(reference.Destroy)
			lines, clusters := reference.MeasureMetrics()
			want := textedit.NewGeometry(lines, clusters, len(text), 20)
			p := gui.NewParagraphLayout(&boundedContext{Context: context, t: t}, format, 1<<20, len(text), func(r textedit.Range) (string, error) { return text[r.Start:r.End], nil })
			t.Cleanup(p.Destroy)
			g := paragraphAt(t, p, 4096)
			for at := 4096 - 24; at < 4096+24; at++ {
				cell := want.Caret(textedit.Position{Offset: at})
				point := geometry.Point{X: cell.X + .25, Y: cell.Y + cell.Height/2}
				local := geometry.Point{X: point.X - float32(g.X), Y: point.Y - float32(g.Y)}
				if g.Hit(local) != want.Hit(point) {
					t.Fatalf("接缝命中 at=%d：%v，参考 %v", at, g.Hit(local), want.Hit(point))
				}
				caret := g.Caret(textedit.Position{Offset: at})
				if math.Abs(g.X+float64(caret.X)-float64(cell.X)) > .05 {
					t.Fatal("接缝光标位置错误", at, caret, cell)
				}
			}
			r := textedit.Range{Start: 4096 - 20, End: 4096 + 20}
			got, expected := g.Selection(r), want.Selection(r)
			if len(got) != 1 || len(expected) != 1 || math.Abs(g.X+float64(got[0].X)-float64(expected[0].X)) > .05 || math.Abs(float64(got[0].Width-expected[0].Width)) > .05 {
				t.Fatal("接缝选区与完整参考不符", got, expected)
			}
			for attempt := 0; attempt < 8 && !p.Prepare(geometry.Rect(0, 0, p.Size().Width, 30), true); attempt++ {
			}
			frame := p.Geometry()
			draw := new(paragraphDraw)
			origin := frame.RelativeTo(frame.X-123.25, -37.75)
			p.Paint(draw, origin)
			advance := want.Caret(textedit.Position{Offset: 4096}).X
			if len(draw.points) != 2 || draw.points[0] != origin || math.Abs(float64(draw.points[1].X-origin.X-advance)) > .05 {
				t.Fatal("绘制与局部几何没有衔接", draw.points, origin, advance)
			}
		})
	}
}
