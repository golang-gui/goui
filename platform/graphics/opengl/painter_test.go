package opengl

import (
	"errors"
	"image"
	"testing"

	"github.com/golang-gui/nanovgo"
)

type lifetimeWindow struct {
	NativeWindow
	handle uintptr
}

func (w *lifetimeWindow) NativeHandle() uintptr { return w.handle }

type lifetimeContext struct {
	Context
	binds int
}

func (c *lifetimeContext) MakeCurrent() error { c.binds++; return ErrSurfaceUnavailable }

func TestReleaseCachedImageAfterHostTermination(t *testing.T) {
	// 不创建原生窗口/GL 对象。Context 替身只允许 MakeCurrent，任何 GL 删除都会失败。
	// handle=1 覆盖 DestroyNotify 尚未派发、GLX 已观察到 drawable 失效的情形。
	for _, handle := range []uintptr{0, 1} {
		c := &lifetimeContext{}
		p := &Painter{window: &lifetimeWindow{handle: handle}, ctx: c, vg: new(nanovgo.Context), images: make(map[*imageResource]struct{})}
		img := &imageResource{owner: p, handle: 7}
		p.images[img] = struct{}{}
		img.Destroy()
		img.Destroy()
		if !img.destroyed || img.owner != nil || img.handle != 0 || img.pendingDestroy || len(p.images) != 0 || p.pendingImages != 0 {
			t.Fatal("已终止宿主的缓存图片没有解除引用")
		}
		if handle == 0 && c.binds != 0 || handle != 0 && c.binds != 1 {
			t.Fatalf("handle=%d binds=%d", handle, c.binds)
		}
	}
}

type resourceLifetimeContext struct {
	lifetimeContext
	resources int
}

func (c *resourceLifetimeContext) makeResourceCurrent() error {
	c.resources++
	return ErrSurfaceUnavailable
}
func (c *resourceLifetimeContext) Destroy() {}

func TestOffFrameOperationsUseResourceTarget(t *testing.T) {
	for _, operation := range []string{"create", "update", "destroy", "offscreen", "painter-destroy"} {
		t.Run(operation, func(t *testing.T) {
			c := &resourceLifetimeContext{}
			p := &Painter{window: &lifetimeWindow{handle: 1}, ctx: c, vg: new(nanovgo.Context), images: make(map[*imageResource]struct{})}
			src := image.NewRGBA(image.Rect(0, 0, 1, 1))
			img := &imageResource{owner: p, handle: 7, width: 1, height: 1}
			p.images[img] = struct{}{}
			var err error
			switch operation {
			case "create":
				_, err = p.NewImage(src)
			case "update":
				err = p.updateImage(img, src)
			case "destroy":
				img.Destroy()
			case "offscreen":
				_, err = p.RenderImage(1, 1, 1, func() { t.Fatal("无效宿主执行了离屏绘制") })
			case "painter-destroy":
				p.Destroy()
			}
			if err != nil && !errors.Is(err, ErrSurfaceUnavailable) {
				t.Fatal(err)
			}
			if c.binds != 0 || c.resources != 1 {
				t.Fatalf("帧外操作绑定了窗口或跳过资源目标：window=%d resource=%d", c.binds, c.resources)
			}
		})
	}
}
