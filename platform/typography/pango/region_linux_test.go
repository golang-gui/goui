package pango

import (
	"image"
	"runtime"
	"testing"

	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/internal/typographytest"
)

func TestRasterRegions(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ctx, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Destroy()
	typographytest.RasterRegions(t, ctx)
}

func TestRasterNativeFailureCanRetry(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ctx, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Destroy()
	layout, err := ctx.NewTextLayout("retry", typography.TextFormat{}, 100, 40)
	if err != nil {
		t.Fatal(err)
	}
	defer layout.Destroy()
	native := layout.(*TextLayout)
	// Cairo image surfaces reject this width (including Cairo 1.16.0).
	// Repeat both before and after a successful rasterization: the old code
	// published failed capacity and dereferenced a null context on the retry.
	bad, good := image.Rect(0, 0, 40000, 1), image.Rect(0, 0, 100, 40)
	for pass := 0; pass < 2; pass++ {
		before := native.painter.context
		for i := 0; i < 2; i++ {
			if _, err := layout.Rasterize(1, bad, nil); err == nil {
				t.Fatal("Cairo accepted oversized surface")
			}
			if native.painter.context != before {
				t.Fatal("failed allocation changed the valid painter")
			}
			if before == 0 && (native.painter.bitmap.Width != 0 || native.painter.bitmap.Pixels != nil) {
				t.Fatal("failed initialization retained capacity")
			}
		}
		if _, err := layout.Rasterize(1, good, nil); err != nil {
			t.Fatal(err)
		}
	}
	// Sticky drawing errors must also discard the context and recover.
	native.painter.context.Scale(0, 0)
	if _, err := layout.Rasterize(1, good, nil); err == nil {
		t.Fatal("invalid Cairo matrix was ignored")
	}
	if native.painter.context != 0 {
		t.Fatal("sticky error context retained")
	}
	if _, err := layout.Rasterize(1, good, nil); err != nil {
		t.Fatal(err)
	}
}
