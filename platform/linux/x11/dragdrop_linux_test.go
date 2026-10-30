package x11

import (
	"image"
	"image/color"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/dragdrop"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"
	"github.com/golang-gui/goui/platform/linux/libs/xshape"
)

func TestDragMessageErrorScope(t *testing.T) {
	base := xlib.ErrorEvent{Display: 1, ResourceID: 42, Serial: 100,
		ErrorCode: xlib.BadWindow, RequestCode: xlib.RequestSendEvent}
	for _, tc := range []struct {
		name   string
		change func(*xlib.ErrorEvent)
	}{
		{"display", func(e *xlib.ErrorEvent) { e.Display = 2 }},
		{"serial", func(e *xlib.ErrorEvent) { e.Serial++ }},
		{"resource", func(e *xlib.ErrorEvent) { e.ResourceID++ }},
		{"error", func(e *xlib.ErrorEvent) { e.ErrorCode++ }},
		{"opcode", func(e *xlib.ErrorEvent) { e.RequestCode++ }},
		{"minor-opcode", func(e *xlib.ErrorEvent) { e.MinorCode++ }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &Platform{display: 1, dragMessages: map[uintptr]xlib.Window{100: 42}}
			e := base
			tc.change(&e)
			if p.consumeDragMessageError(&e) || len(p.dragMessages) != 1 {
				t.Fatal("unrelated error consumed or removed the pending request")
			}
			if !p.consumeDragMessageError(&base) || len(p.dragMessages) != 0 {
				t.Fatal("matching late peer error not consumed exactly once")
			}
			if p.consumeDragMessageError(&base) {
				t.Fatal("duplicate error consumed without a pending request")
			}
		})
	}
}

func TestDragMessageErrorsForwardToPreviousHandler(t *testing.T) {
	// 验证原生回调 ABI，不安装全局 Xlib handler、不连接显示服务器。
	// 错误回调不发起原生请求。
	var calls int
	var gotDisplay xlib.Display
	var gotEvent xlib.ErrorEvent
	handler := xlib.ErrorHandler(cgo.NewCallback(func(d xlib.Display, e *xlib.ErrorEvent) int32 {
		calls++
		gotDisplay, gotEvent = d, *e
		return 17
	}))
	p := &Platform{display: 1, previousXErrorHandler: handler,
		dragMessages: map[uintptr]xlib.Window{100: 42}}
	e := xlib.ErrorEvent{Display: 1, ResourceID: 42, Serial: 100,
		ErrorCode: xlib.BadWindow, RequestCode: xlib.RequestSendEvent}
	if ret := p.onXError(2, &e); ret != 17 || calls != 1 || gotDisplay != 2 || gotEvent != e {
		t.Fatal("foreign display did not reach the previous handler unchanged")
	}
	if ret := p.onXError(1, &e); ret != 0 || calls != 1 {
		t.Fatal("tracked BadWindow reached the previous handler")
	}
	if ret := p.onXError(1, &e); ret != 17 || calls != 2 || gotDisplay != 1 || gotEvent != e {
		t.Fatal("untracked request did not reach the previous handler unchanged")
	}
}

func TestRetireDragMessages(t *testing.T) {
	p := &Platform{dragMessages: map[uintptr]xlib.Window{100: 42, 101: 42, 105: 50}}
	p.retireDragMessages(101)
	if !reflect.DeepEqual(p.dragMessages, map[uintptr]xlib.Window{105: 50}) {
		t.Fatalf("pending requests after server progress: %v", p.dragMessages)
	}
	p.retireDragMessages(105)
	if len(p.dragMessages) != 0 {
		t.Fatal("acknowledged requests retained")
	}
}

func TestXDNDSourceDoesNotTimeoutHeldDrag(t *testing.T) {
	for _, awaiting := range []bool{false, true} {
		t.Run(map[bool]string{false: "idle", true: "awaiting-status"}[awaiting], func(t *testing.T) {
			// 无显示连接或事件循环；按住拖动时，即使目标无应答，也不能启动到期计时。
			loop := &EventLoop{pipe: [2]int32{-1, -1}}
			p := &Platform{eventLoop: loop}
			s := &xdndSource{service: &dragService{platform: p}, awaiting: awaiting}
			p.dragSource = s
			defer func() {
				if s.timer != nil {
					s.timer.Stop()
				}
				loop.state.Destroy()
			}()
			s.scheduleTimeout()
			if s.timer != nil {
				t.Fatal("held drag acquired an expiry timer")
			}
		})
	}
}

func TestXDNDSourceReplyDeadline(t *testing.T) {
	released := time.Unix(1000, 0)
	for _, tc := range []struct {
		name                        string
		released, awaiting, dropped bool
		progress, want              time.Duration
	}{
		{"held-idle", false, false, false, 0, 0},
		{"held-awaiting-status", false, true, false, 0, 0},
		{"released-without-wait", true, false, false, 0, 0},
		{"final-status", true, true, false, 0, 10 * time.Second},
		{"finished", true, false, true, 0, 10 * time.Second},
		{"transfer-progress", true, false, true, 8 * time.Second, 18 * time.Second},
		{"completion-cap", true, false, true, 25 * time.Second, 30 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &xdndSource{released: tc.released, awaiting: tc.awaiting, dropped: tc.dropped,
				releasedAt: released, progress: released.Add(tc.progress)}
			want := time.Time{}
			if tc.want != 0 {
				want = released.Add(tc.want)
			}
			if got := s.replyDeadline(); !got.Equal(want) {
				t.Fatalf("deadline=%v want=%v", got, want)
			}
		})
	}
}

func TestXDNDSourceClearsReplyTimer(t *testing.T) {
	loop := &EventLoop{pipe: [2]int32{-1, -1}}
	p := &Platform{eventLoop: loop}
	s := &xdndSource{service: &dragService{platform: p}, released: true, awaiting: true,
		releasedAt: time.Now(), progress: time.Now()}
	p.dragSource = s
	defer func() {
		if s.timer != nil {
			s.timer.Stop()
		}
		loop.state.Destroy()
	}()
	s.scheduleTimeout()
	if s.timer == nil {
		t.Fatal("released source did not bound its pending Status reply")
	}
	s.awaiting = false
	s.scheduleTimeout()
	if s.timer != nil {
		t.Fatal("completed status wait retained its timer")
	}
	s.dropped = true
	s.scheduleTimeout()
	if s.timer == nil {
		t.Fatal("dropped source did not bound its Finished reply")
	}
	p.dragSource = nil
	s.scheduleTimeout()
	if s.timer != nil {
		t.Fatal("ended source retained a timer")
	}
}

func TestDecodeDragData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "中文 space%.txt")
	uri, err := dragdrop.FileURLFromPath(path)
	if err != nil {
		t.Fatal(err)
	}
	data, err := decodeDragData(dragdrop.FormatFiles, []byte("# comment\r\n"+uri+"\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	files, ok := data.Files()
	if !ok || !reflect.DeepEqual(files, []string{path}) {
		t.Fatalf("files %v %t", files, ok)
	}
	urls, err := decodeDragData(dragdrop.FormatURLs, []byte(uri+"\r\nhttps://example.test/a\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	gotURLs, ok := urls.URLs()
	if !ok || !reflect.DeepEqual(gotURLs, []string{uri, "https://example.test/a"}) {
		t.Fatalf("urls %v %t", gotURLs, ok)
	}
	for _, raw := range [][]byte{
		[]byte("file://otherhost/path\r\n"),
		[]byte("https://example.test/path\r\n"),
		[]byte("# no file\r\n"),
	} {
		if _, err := decodeDragData(dragdrop.FormatFiles, raw); err == nil {
			t.Fatalf("accepted invalid file list %q", raw)
		}
	}
	text, err := decodeDragData(dragdrop.FormatText, []byte("hello 世界"))
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := text.Text(); !ok || got != "hello 世界" {
		t.Fatalf("text %q %t", got, ok)
	}
	if _, err := decodeDragData(dragdrop.FormatText, []byte{0xff}); err == nil {
		t.Fatal("accepted invalid UTF-8")
	}
}

func TestPreviewRasterScaleAndHotspot(t *testing.T) {
	// 像素为预乘颜色；非零图片原点不能影响 DIP 热点。
	// 拖放预览与光标不同，允许热点位于图片外部。
	src := image.NewRGBA(image.Rect(10, 20, 14, 22))
	for y := 20; y < 22; y++ {
		for x := 10; x < 14; x++ {
			src.SetRGBA(x, y, color.RGBA{R: 64, A: 128})
		}
	}
	for _, scale := range []float32{1, 2} {
		raster, x, y, err := previewRaster(dragdrop.Preview{Image: src, Scale: 2, Hotspot: geometry.Point{X: -1, Y: 3}}, scale)
		if err != nil {
			t.Fatal(err)
		}
		if raster.Rect != image.Rect(0, 0, int(2*scale), int(scale)) || x != -int(scale) || y != int(3*scale) {
			t.Fatalf("scale %g: bounds=%v hotspot=%d,%d", scale, raster.Rect, x, y)
		}
		if got := raster.RGBAAt(0, 0); got != (color.RGBA{R: 64, A: 128}) {
			t.Fatalf("premultiplied color changed: %v", got)
		}
	}
}

func TestOpaquePreviewCoverage(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 3, 1))
	src.SetRGBA(0, 0, color.RGBA{R: 64, A: 128})
	src.SetRGBA(1, 0, color.RGBA{R: 63, A: 127})
	src.SetRGBA(2, 0, color.RGBA{B: 255, A: 255})
	bitmap := graphics.CopyToBitmap(src, graphics.PixelFormatBGRA, nil)
	rects := opaquePreview(&bitmap)
	want := []xshape.Rectangle{{X: 0, Y: 0, Width: 1, Height: 1}, {X: 2, Y: 0, Width: 1, Height: 1}}
	if !reflect.DeepEqual(rects, want) {
		t.Fatalf("coverage=%v want=%v", rects, want)
	}
	if bitmap.Pixels[2] != 127 || bitmap.Pixels[3] != 255 {
		t.Fatalf("opaque BGRA=%v", bitmap.Pixels[:4])
	}
}

func TestXDNDLocalOnlyOfferStaysInProcess(t *testing.T) {
	previous := windowMap
	defer func() { windowMap = previous }()
	windowMap = map[xlib.Window]*Window{42: {}}
	p := new(Platform)
	p.atoms.GOUI_LOCAL = 7
	source := &xdndSource{service: &dragService{platform: p}}
	for _, test := range []struct {
		types  []xlib.Atom
		target xlib.Window
		want   bool
	}{
		{[]xlib.Atom{7}, 42, true},
		{[]xlib.Atom{7}, 43, false},
		{[]xlib.Atom{7, 8}, 43, true},
		{[]xlib.Atom{8}, 43, true},
	} {
		source.types = test.types
		if got := source.canOfferTo(test.target); got != test.want {
			t.Fatalf("types=%v target=%v: canOffer=%v, want %v", test.types, test.target, got, test.want)
		}
	}
}
