package coretext

import (
	"github.com/golang-gui/goui/platform/typography/internal/typographytest"
	"runtime"
	"testing"
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
