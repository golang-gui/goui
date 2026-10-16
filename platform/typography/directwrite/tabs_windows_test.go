package directwrite

import (
	"runtime"
	"testing"

	"github.com/golang-gui/goui/platform/typography/internal/typographytest"
)

func TestTabWidth(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ctx, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Destroy()
	typographytest.Tabs(t, ctx)
}
