package graphics

import "testing"

func Test_Path(t *testing.T) {
	path := MoveTo(1, 2).LineTo(3, 4).LineTo(5, 6).Close()
	path.Range(func(op PathOperation, args []float32) (stop bool) {
		t.Log(op, args)
		return false
	})
}
