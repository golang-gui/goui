package win32

import (
	"image"
	"maps"
	"runtime"
	"slices"
	"testing"
	"unsafe"

	"github.com/golang-gui/goui/platform/dragdrop"
	"github.com/golang-gui/goui/platform/windows/sdk/winapi"
)

// 无窗口验证：原生尺寸转换超限属于预览不可用，不丢失原始载荷或留下COM锚点。
// inject.py的bitmap/helper/partial覆盖同一路径的原生创建失败和SetData后失败。
func TestOLESourcePreviewFallbackKeepsPortableData(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	for _, scale := range []float32{0.00001, 1} {
		wire := map[uint16][]byte{winapi.CF_UNICODETEXT: {65, 0, 0, 0}}
		before := maps.Clone(wire)
		count := func() int {
			n := 0
			oleSourceRoots.Range(func(_, _ any) bool { n++; return true })
			return n
		}
		roots := count()
		// scale=1使用无图路径；注入构建使用独立的有图测试覆盖原生故障。
		preview := dragdrop.Preview{}
		if scale < 1 {
			preview = dragdrop.Preview{Image: image.NewRGBA(image.Rect(0, 0, 2, 2)), Scale: scale}
		}
		if err := preview.Validate(); err != nil {
			t.Fatal("fixture不是有效预览", err)
		}
		data := newOLESourceData(wire, preview, 1)
		if !maps.EqualFunc(wire, before, slices.Equal[[]byte]) || !maps.EqualFunc(data.wire, before, slices.Equal[[]byte]) || len(data.streams) != 0 || count() != roots+1 {
			t.Fatal("预览失败污染载荷／泄漏旧对象", wire, data.wire, count()-roots)
		}
		if n := data.releaseRef(); n != 0 || count() != roots {
			t.Fatal("源对象没有被释放", n, count()-roots)
		}
		if _, exists := oleSourceRoots.Load(uintptr(unsafe.Pointer(data))); exists {
			t.Fatal("已释放对象仍被锚定")
		}
	}
}
