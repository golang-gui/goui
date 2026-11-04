package graphics

import (
	"slices"
	"testing"
)

func Test_Path(t *testing.T) {
	path := MoveTo(1, 2).LineTo(3, 4).LineTo(5, 6).Close()
	path.Range(func(op PathOperation, args []float32) (stop bool) {
		t.Log(op, args)
		return false
	})
}

func TestPathContoursAndCurrentPos(t *testing.T) {
	check := func(p Path, x, y float32) {
		t.Helper()
		if px, py := p.CurrentPos(); px != x || py != y {
			t.Fatalf("当前点=(%g,%g)，预期(%g,%g)", px, py, x, y)
		}
	}
	check(Path{}, 0, 0)
	p := MoveTo(3, 4).LineTo(9, 10).Close()
	check(p, 3, 4)
	p = p.MoveTo(20, 30).Hole()
	check(p, 20, 30)
	p = p.LineTo(21, 32).ArcTo(4, 5, 0, 0, 1, 23, 38)
	check(p, 23, 38)
	p = p.BezierTo(24, 38, 25, 39, 25, 40)
	check(p, 25, 40)
	p = p.Close()
	check(p, 20, 30)
	var ops []PathOperation
	p.Range(func(op PathOperation, _ []float32) bool {
		ops = append(ops, op)
		return false
	})
	want := []PathOperation{PathMoveTo, PathLineTo, PathClose, PathMoveTo, PathHole, PathLineTo, PathArcTo, PathBezierTo, PathClose}
	if !slices.Equal(ops, want) {
		t.Fatalf("多轮廓指令丢失或重排：%v，预期%v", ops, want)
	}
}
