package pango

import (
	"runtime"
	"strings"
	"testing"

	"github.com/golang-gui/goui/platform/typography"
)

func TestRasterNativeFailureCanRetry(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ctx, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Destroy()
	layout, err := ctx.NewTextLayout(strings.Repeat("a", 5000), typography.TextFormat{Font: typography.FontInfo{Family: "monospace", Size: 16}}, 40000, 40)
	if err != nil {
		t.Fatal(err)
	}
	defer layout.Destroy()
	native := layout.(*TextLayout)
	// Cairo image surfaces reject this width (including Cairo 1.16.0).
	// Repeat both before and after a successful rasterization: the old code
	// published failed capacity and dereferenced a null context on the retry.
	if width, _ := layout.MeasureSize(); width < 40000 {
		t.Fatalf("test text is not wide enough to exceed Cairo surface limits: %g", width)
	}
	for pass := 0; pass < 2; pass++ {
		layout.SetSize(40000, 40)
		before := native.painter.context
		for i := 0; i < 2; i++ {
			if _, err := layout.Rasterize(1, nil); err == nil {
				t.Fatal("Cairo accepted oversized surface")
			}
			if native.painter.context != before {
				t.Fatal("failed allocation changed the valid painter")
			}
			if before == 0 && (native.painter.bitmap.Width != 0 || native.painter.bitmap.Pixels != nil) {
				t.Fatal("failed initialization retained capacity")
			}
		}
		layout.SetSize(100, 40)
		if _, err := layout.Rasterize(1, nil); err != nil {
			t.Fatal(err)
		}
	}
	// Sticky drawing errors must also discard the context and recover.
	native.painter.context.Scale(0, 0)
	if _, err := layout.Rasterize(1, nil); err == nil {
		t.Fatal("invalid Cairo matrix was ignored")
	}
	if native.painter.context != 0 {
		t.Fatal("sticky error context retained")
	}
	if _, err := layout.Rasterize(1, nil); err != nil {
		t.Fatal(err)
	}
}
