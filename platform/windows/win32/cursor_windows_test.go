package win32

import (
	"testing"

	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/windows/sdk/winapi"
)

func TestResizeCursorIDs(t *testing.T) {
	for shape, want := range map[common.CursorShape]winapi.LPWSTR{
		common.CursorResizeHorizontal:  winapi.IDC_SIZEWE,
		common.CursorResizeVertical:    winapi.IDC_SIZENS,
		common.CursorResizeNWSE:        winapi.IDC_SIZENWSE,
		common.CursorResizeNESW:        winapi.IDC_SIZENESW,
		common.CursorResizeLeft:        winapi.IDC_SIZEWE,
		common.CursorResizeRight:       winapi.IDC_SIZEWE,
		common.CursorResizeTop:         winapi.IDC_SIZENS,
		common.CursorResizeBottom:      winapi.IDC_SIZENS,
		common.CursorResizeTopLeft:     winapi.IDC_SIZENWSE,
		common.CursorResizeTopRight:    winapi.IDC_SIZENESW,
		common.CursorResizeBottomLeft:  winapi.IDC_SIZENESW,
		common.CursorResizeBottomRight: winapi.IDC_SIZENWSE,
	} {
		if got := cursorID(shape); got != want {
			t.Fatalf("shape %v: got %v want %v", shape, got, want)
		}
	}
}
