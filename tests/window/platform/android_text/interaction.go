package main

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui/textedit"
	"github.com/golang-gui/goui/platform/typography"
)

// 这是测试程序的数据交换模型，不是 platform 对 gui 的生产依赖。
type interactionCase struct {
	Name       string
	Text       string
	Boundaries []int
	Lines      []typography.TextLine
	Clusters   []typography.TextCluster
	Carets     []nativeCaret
	Hits       []nativeHit
	Selections []nativeSelection
}

type nativeCaret struct {
	Offset, Line       int
	Primary, Secondary float32
}

type nativeHit struct {
	nativeCaret
	X, Y           float32
	NativeLine     int
	ExpectedOffset int
	ExpectedX      float32
	Reason         string
}

type nativeSelection struct {
	Start, End int
	Rects      []geometry.Rectangle
}

type interactionResult struct {
	Cases, Checks, Failures, CoincidentHits, BoundaryPixels int
	RawHitDifferences, GraphemeSnaps, WrapAffinities        int
	Errors                                                  []string
}

func (r *interactionResult) check(pass bool, name string, detail any) {
	r.Checks++
	if !pass {
		r.Failures++
		if len(r.Errors) < 40 {
			r.Errors = append(r.Errors, fmt.Sprintf("%s: %v", name, detail))
		}
	}
}

func closeX(a, b float32) bool { return math.Abs(float64(a-b)) <= 0.001 }

func contains(rects []geometry.Rectangle, x, y float32) bool {
	for _, rect := range rects {
		if x >= rect.X && x < rect.X+rect.Width && y >= rect.Y && y < rect.Y+rect.Height {
			return true
		}
	}
	return false
}

func horizontalEdge(rects []geometry.Rectangle, x, y float32) bool {
	for _, rect := range rects {
		if y >= rect.Y && y < rect.Y+rect.Height &&
			(math.Abs(float64(x-rect.X)) <= 1 || math.Abs(float64(x-rect.X-rect.Width)) <= 1) {
			return true
		}
	}
	return false
}

func verifyInteractions(cases []interactionCase) interactionResult {
	result := interactionResult{Cases: len(cases)}
	for _, c := range cases {
		result.check(len(c.Lines) > 0 && len(c.Boundaries) > 0, c.Name+":nonempty-metrics", len(c.Lines))
		g := textedit.NewGeometry(c.Lines, c.Clusters, len(c.Text), 1)
		legal := make(map[int]bool)
		for _, offset := range c.Boundaries {
			valid := offset >= 0 && offset <= len(c.Text) && utf8.ValidString(c.Text[:offset])
			result.check(valid, c.Name+":utf8-boundary", offset)
			legal[offset] = true
			result.check(g.Snap(offset, false) == offset && g.Snap(offset, true) == offset, c.Name+":retained-boundary", offset)
		}
		cursor := 0
		for _, cluster := range c.Clusters {
			// 不能丢弃零宽范围；硬换行保留在线范围，不伪造可见单元。
			gap := c.Text[cursor:cluster.Start]
			result.check(strings.Trim(gap, "\r\n") == "" && legal[cluster.Start] && legal[cluster.Start+cluster.Length], c.Name+":coverage", cluster)
			cursor = cluster.Start + cluster.Length
			for offset := cluster.Start + 1; offset < cursor; offset++ {
				result.check(g.Snap(offset, false) == cluster.Start && g.Snap(offset, true) == cursor,
					c.Name+":atomic-snap", offset)
			}
		}
		result.check(strings.Trim(c.Text[cursor:], "\r\n") == "", c.Name+":trailing-coverage", cursor)
		for _, caret := range c.Carets {
			matched := false
			for _, upstream := range []bool{false, true} {
				pos := textedit.Position{Offset: caret.Offset, Upstream: upstream}
				box := g.Caret(pos)
				matched = matched || (g.LineIndex(pos) == caret.Line && closeX(box.X, caret.Primary))
			}
			result.check(matched, c.Name+":native-primary-caret", caret)
		}
		for _, hit := range c.Hits {
			pos := g.Hit(geometry.Point{X: hit.X, Y: hit.Y})
			result.check(legal[pos.Offset], c.Name+":hit-boundary", pos)
			if pos.Offset != hit.Offset {
				result.RawHitDifferences++
			}
			switch hit.Reason {
			case "grapheme-snap":
				result.GraphemeSnaps++
			case "wrap-upstream":
				result.WrapAffinities++
			}
			result.check(legal[hit.ExpectedOffset], c.Name+":oracle-boundary", hit.ExpectedOffset)
			matched := pos.Offset == hit.ExpectedOffset
			if matched && hit.Reason != "native" {
				matched = g.LineIndex(pos) == hit.Line && closeX(g.Caret(pos).X, hit.ExpectedX)
			}
			// 零宽字符和 bidi 交界可有多个合法偏移共享同一视觉光标。
			// 仅允许原生报告的同一坐标，不使用任意字节或字符容差。
			if !matched && hit.Reason == "native" && legal[hit.Offset] && hit.NativeLine == hit.Line {
				x := g.Caret(pos).X
				matched = closeX(x, hit.Primary) || closeX(x, hit.Secondary)
				if matched {
					result.CoincidentHits++
				}
			}
			result.check(matched, c.Name+":native-hit", fmt.Sprintf("x=%g line=%d native=%d go=%+v", hit.X, hit.Line, hit.Offset, pos))
		}
		for _, selected := range c.Selections {
			actual := g.Selection(textedit.Range{Start: selected.Start, End: selected.End})
			bad := 0
			for _, line := range c.Lines {
				// 三条扫描线验证行高及不连续区间；水平边缘允许 Region 的整数栅格化误差 1 px。
				for _, y := range []float32{line.Y + 0.5, line.Y + line.Height/2, line.Y + line.Height - 0.5} {
					for x := float32(math.Floor(float64(line.X))) - 1.5; x < line.X+line.Width+2; x++ {
						if contains(actual, x, y) == contains(selected.Rects, x, y) {
							continue
						}
						if horizontalEdge(selected.Rects, x, y) || horizontalEdge(actual, x, y) {
							result.BoundaryPixels++
							continue
						}
						bad++
					}
				}
			}
			result.check(bad == 0, c.Name+":native-selection", fmt.Sprintf("%d:%d badPixels=%d native=%v go=%v", selected.Start, selected.End, bad, selected.Rects, actual))
		}
		// 视觉导航必须前进且终止，不能停留在零宽范围或组合字符内部。
		pos := g.LineEdge(0, false)
		for i := 0; ; i++ {
			next, ok := g.Visual(pos, true)
			if !ok {
				break
			}
			result.check(legal[next.Offset], c.Name+":visual-boundary", next)
			line, nextLine := g.LineIndex(pos), g.LineIndex(next)
			result.check(nextLine > line || (nextLine == line && g.Caret(next).X > g.Caret(pos).X), c.Name+":visual-progress", next)
			if i > len(c.Clusters)*2+len(c.Lines) {
				result.check(false, c.Name+":visual-termination", i)
				break
			}
			pos = next
		}
	}
	return result
}
