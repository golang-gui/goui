package androidtext

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"runtime"
	"slices"
	"unsafe"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/internal/diagnostic"
	"github.com/golang-gui/goui/platform/android/libs/bitmap"
	"github.com/golang-gui/goui/platform/android/libs/jni"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/internal/lifecycle"
	"github.com/golang-gui/goui/platform/typography/utils"
)

type TextLayout struct {
	peer          jni.Object
	methods       *layoutMethods
	text          string
	format        typography.TextFormat
	width, height float32
	positions     utils.StringPosition
	life          lifecycle.Layout
	valid         bool
	lines         []typography.TextLine
	clusters      []typography.TextCluster
}

func (t *TextLayout) Destroy() {
	if !t.life.BeginDestroy() {
		return
	}
	e := frame()
	defer e.PopLocalFrame(0)
	e.CallVoidMethodA(t.peer, t.methods.close)
	check(e)
	e.DeleteGlobalRef(t.peer)
	t.peer = 0
	t.lines = nil
	t.clusters = nil
	diagnostic.Record(diagnostic.LayoutDestroy, 1)
}
func (t *TextLayout) Text() string                           { return t.text }
func (t *TextLayout) Format() typography.TextFormat          { return t.format }
func (t *TextLayout) Size() (float32, float32)               { return t.width, t.height }
func (t *TextLayout) ConnectChanged(fn func()) signal.Handle { return t.life.ConnectChanged(fn) }
func (t *TextLayout) ConnectDestroy(fn func()) signal.Handle { return t.life.ConnectDestroy(fn) }
func (t *TextLayout) invoke(method jni.Method, args ...jni.Value) {
	e := frame()
	defer e.PopLocalFrame(0)
	e.CallVoidMethodA(t.peer, method, args...)
	check(e)
}
func (t *TextLayout) changed(metrics bool) {
	if metrics {
		t.valid = false
		t.lines = nil
		t.clusters = nil
	}
	t.life.Changed()
}
func (t *TextLayout) SetSize(width, height float32) {
	if t.width == width && t.height == height {
		return
	}
	if err := dimensions(width, height); err != nil {
		panic(err)
	}
	t.invoke(t.methods.setSize, jni.Float(width), jni.Float(height))
	t.width = width
	t.height = height
	diagnostic.Record(diagnostic.LayoutResize, 1)
	t.changed(true)
}
func (t *TextLayout) SetTextAlignment(value typography.TextAlignment) {
	if t.format.TextAlign == value {
		return
	}
	if err := alignment(value); err != nil {
		panic(err)
	}
	t.invoke(t.methods.setAlignment, jni.Int(int32(value)))
	t.format.TextAlign = value
	t.changed(true)
}
func (t *TextLayout) SetWrapMode(value typography.WrapMode) {
	if t.format.WrapMode == value {
		return
	}
	if err := wrapping(value); err != nil {
		panic(err)
	}
	t.invoke(t.methods.setWrap, jni.Int(int32(value)))
	t.format.WrapMode = value
	t.changed(true)
}
func (t *TextLayout) SetTabWidth(value float32) {
	if t.format.TabWidth == value {
		return
	}
	if err := utils.ValidateTabWidth(value); err != nil {
		panic(err)
	}
	t.invoke(t.methods.setTabWidth, jni.Float(value))
	t.format.TabWidth = value
	t.changed(true)
}
func (t *TextLayout) textRange(start, length int) (jni.Value, jni.Value) {
	utils.CheckTextRange(t.text, start, length)
	return jni.Int(int32(t.positions.ToUtf16(start))), jni.Int(int32(t.positions.ToUtf16(start + length)))
}
func (t *TextLayout) SetTextFont(start, length int, font typography.FontInfo) {
	a, b := t.textRange(start, length)
	font, err := resolveFont(font)
	if err != nil {
		panic(err)
	}
	e := frame()
	defer e.PopLocalFrame(0)
	family := e.NewString(font.Family)
	check(e)
	e.CallVoidMethodA(t.peer, t.methods.setFont, a, b, jni.Ref(family), jni.Float(font.Size), jni.Int(int32(font.Weight)), jni.Int(int32(font.Width)), jni.Int(int32(font.Style)))
	check(e)
	t.changed(true)
}
func (t *TextLayout) SetTextColor(start, length int, c color.Color) {
	a, b := t.textRange(start, length)
	t.invoke(t.methods.setColor, a, b, jni.Int(argb(c)))
	t.changed(false)
}
func (t *TextLayout) SetUnderline(start, length int, enabled bool) {
	a, b := t.textRange(start, length)
	var value int32
	if enabled {
		value = 1
	}
	t.invoke(t.methods.setUnderline, a, b, jni.Int(value))
	t.changed(false)
}
func (t *TextLayout) SetStrikethrough(start, length int, enabled bool) {
	a, b := t.textRange(start, length)
	var value int32
	if enabled {
		value = 1
	}
	t.invoke(t.methods.setStrike, a, b, jni.Int(value))
	t.changed(false)
}
func (t *TextLayout) SetPlaceholder(start int, metrics *typography.PlaceholderMetrics) {
	utils.CheckPlaceholder(t.text, start, metrics)
	var value typography.PlaceholderMetrics
	var present int32
	if metrics != nil {
		value = *metrics
		if float64(value.Width)*64 > math.MaxInt32 || float64(value.Height)*64 > math.MaxInt32 {
			panic("androidtext: placeholder exceeds native coordinate range")
		}
		present = 1
	}
	t.invoke(t.methods.setPlaceholder, jni.Int(int32(t.positions.ToUtf16(start))), jni.Int(present), jni.Float(value.Width), jni.Float(value.Height), jni.Float(value.Baseline))
	t.changed(true)
}
func (t *TextLayout) prepare() {
	if t.valid {
		return
	}
	e := frame()
	defer e.PopLocalFrame(0)
	data := e.CallObjectMethodA(t.peer, t.methods.metrics)
	check(e)
	if data == 0 {
		panic("androidtext: null metrics packet")
	}
	count := e.GetArrayLength(data)
	check(e)
	if count != 4 {
		panic("androidtext: invalid metrics packet")
	}
	li := e.Ints(e.GetObjectArrayElement(data, 0))
	check(e)
	lf := e.Floats(e.GetObjectArrayElement(data, 1))
	check(e)
	ci := e.Ints(e.GetObjectArrayElement(data, 2))
	check(e)
	cf := e.Floats(e.GetObjectArrayElement(data, 3))
	check(e)
	if len(li)%2 != 0 || len(lf) != len(li)/2*5 || len(ci)%4 != 0 || len(ci) != len(cf) {
		panic("androidtext: invalid metrics packet")
	}
	lines := make([]typography.TextLine, len(li)/2)
	clusters := make([]typography.TextCluster, len(ci)/4)
	for i := range lines {
		a := t.positions.ToUtf8(int(li[i*2]))
		b := t.positions.ToUtf8(int(li[i*2] + li[i*2+1]))
		f := lf[i*5:]
		lines[i] = typography.TextLine{Start: a, Length: b - a, X: f[0], Y: f[1], Width: f[2], Height: f[3], Baseline: f[4]}
	}
	for i := range clusters {
		ints := ci[i*4:]
		f := cf[i*4:]
		a := t.positions.ToUtf8(int(ints[0]))
		b := t.positions.ToUtf8(int(ints[0] + ints[1]))
		line := int(ints[2])
		if a < 0 || b < a || line < 0 || line >= len(lines) {
			panic("androidtext: invalid cluster range")
		}
		clusters[i] = typography.TextCluster{Start: a, Length: b - a, X: f[0], Y: f[1], Width: f[2], Height: f[3], LineIndex: line, Direction: typography.TextDirection(ints[3])}
		lines[line].Clusters = append(lines[line].Clusters, clusters[i])
	}
	t.lines = lines
	t.clusters = clusters
	t.valid = true
}
func (t *TextLayout) MeasureMetrics() ([]typography.TextLine, []typography.TextCluster) {
	t.prepare()
	lines, clusters := slices.Clone(t.lines), slices.Clone(t.clusters)
	offset := 0
	for i := range lines {
		count := len(lines[i].Clusters)
		if count > 0 {
			lines[i].Clusters = clusters[offset : offset+count]
		}
		offset += count
	}
	return lines, clusters
}
func (t *TextLayout) MeasureSize() (width, height float32) {
	t.prepare()
	for _, line := range t.lines {
		width = max(width, line.Width)
		height = max(height, line.Y+line.Height)
	}
	return
}

func (t *TextLayout) Rasterize(scale float32, region image.Rectangle, buf []byte) (typography.TextBitmap, error) {
	if scale <= 0 || math.IsInf(float64(scale), 0) || math.IsNaN(float64(scale)) {
		return typography.TextBitmap{}, fmt.Errorf("androidtext: invalid raster scale %g", scale)
	}
	w, h, err := utils.RasterDimensions(region)
	if err != nil || w == 0 || h == 0 {
		return typography.TextBitmap{}, err
	}
	for _, v := range []int{region.Min.X, region.Min.Y} {
		if int64(v) < math.MinInt32 || int64(v) > math.MaxInt32 {
			return typography.TextBitmap{}, fmt.Errorf("androidtext: raster origin is outside native integer range")
		}
	}
	e := frame()
	defer e.PopLocalFrame(0)
	obj := e.CallObjectMethodA(t.peer, t.methods.raster, jni.Float(scale), jni.Int(int32(region.Min.X)), jni.Int(int32(region.Min.Y)), jni.Int(int32(w)), jni.Int(int32(h)))
	if err = e.Exception(); err != nil {
		return typography.TextBitmap{}, err
	}
	info, status := bitmap.GetInfo(e, obj)
	if err = e.Exception(); err != nil {
		return typography.TextBitmap{}, err
	}
	if status != 0 {
		return typography.TextBitmap{}, fmt.Errorf("androidtext: bitmap info %d", status)
	}
	if info.Format != bitmap.RGBA8888 || info.Flags&3 != 0 || int(info.Width) < w || int(info.Height) < h || uint64(info.Stride) < uint64(w)*4 {
		return typography.TextBitmap{}, fmt.Errorf("androidtext: unexpected bitmap format %+v", info)
	}
	pixels, status := bitmap.LockPixels(e, obj)
	if err = e.Exception(); err != nil {
		return typography.TextBitmap{}, err
	}
	if status != 0 {
		return typography.TextBitmap{}, fmt.Errorf("androidtext: lock bitmap %d", status)
	}
	locked := true
	defer func() {
		if locked {
			bitmap.UnlockPixels(e, obj)
		}
	}()
	if cap(buf) < w*h*4 {
		buf = make([]byte, w*h*4)
	} else {
		buf = buf[:w*h*4]
	}
	for y := 0; y < h; y++ {
		copy(buf[y*w*4:(y+1)*w*4], unsafe.Slice((*byte)(cgo.Pointer(pixels+uintptr(y)*uintptr(info.Stride))), w*4))
	}
	status = bitmap.UnlockPixels(e, obj)
	locked = false
	runtime.KeepAlive(obj)
	if err = e.Exception(); err != nil {
		return typography.TextBitmap{}, err
	}
	if status != 0 {
		return typography.TextBitmap{}, fmt.Errorf("androidtext: unlock bitmap %d", status)
	}
	diagnostic.Record(diagnostic.TextRasterize, 1)
	return typography.TextBitmap{Width: w, Height: h, Stride: w * 4, Pixels: buf}, nil
}

var _ typography.TextLayout = (*TextLayout)(nil)
