package widgets

import (
	"math"
	"testing"
)

// 真实表体组合不能因小数视口把末行底线裁掉，不能调整应用分配的高度。
func TestTableLastRowFullyVisibleAtFractionalViewport(t *testing.T) {
	for _, height := range []float32{100.1, 100.25, 100.5, 100.75, 120.9} {
		v, _, _, _ := tableFixture(3)
		tableArrange(v, 240, height)
		v.scroll.SetScrollY(math.MaxFloat32)
		tableArrange(v, 240, height)
		row := v.realized["r2"]
		if row == nil {
			t.Fatal("滚到底未实现末行")
		}
		end := row.Rect().Y + row.Rect().Height
		viewport := v.body.Rect().Height
		if end > viewport || viewport-end >= 1 || v.Rect().Height != height || v.Snapshot().ScrollY != v.Snapshot().MaxScrollY {
			t.Fatalf("末行底部被裁切或分配高度被改写：height=%g end=%g viewport=%g", height, end, viewport)
		}
		v.SetModel(nil)
	}
}
