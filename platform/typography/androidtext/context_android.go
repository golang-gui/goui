// Package androidtext implements Typography using Android's public text APIs.
package androidtext

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"path/filepath"
	"runtime"
	"slices"
	"unicode/utf8"
	"unsafe"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/internal/diagnostic"
	"github.com/golang-gui/goui/platform/android/libs/bitmap"
	"github.com/golang-gui/goui/platform/android/libs/jni"
	"github.com/golang-gui/goui/platform/internal/androidruntime"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/internal/lifecycle"
	"github.com/golang-gui/goui/platform/typography/utils"
)

type Context struct {
	peer          jni.Object
	methods       map[string]jni.Method
	layoutMethods map[string]jni.Method
	files         map[string]bool
}

// NewContext creates a thread-affine Android typography context. The fixed
// GOUI thread must be attached by the Android host. API 29 or later and the
// matching typography Dex component are required. WrapChar is not yet supported.
func NewContext() (typography.Context, error) {
	e, err := androidruntime.Environment()
	if err != nil {
		return nil, err
	}
	if e.PushLocalFrame(32) < 0 {
		return nil, fmt.Errorf("androidtext: JNI local frame: %w", e.Exception())
	}
	defer e.PopLocalFrame()
	class, err := androidruntime.LoadClass(e, "org.golanggui.typography.TextContext")
	if err != nil {
		return nil, err
	}
	constructor := e.Method(class, "<init>", "(I)V", false)
	if err = e.Exception(); err != nil {
		return nil, err
	}
	peer := e.NewObject(class, constructor, jni.Int(1))
	if err = e.Exception(); err != nil {
		return nil, err
	}
	if peer == 0 {
		return nil, fmt.Errorf("androidtext: create Context failed")
	}
	c := &Context{methods: make(map[string]jni.Method), layoutMethods: make(map[string]jni.Method), files: make(map[string]bool)}
	for name, signature := range map[string]string{"close": "()V", "addFont": "(Ljava/lang/String;[Ljava/lang/String;[I)V", "create": "(Ljava/lang/String;Ljava/lang/String;FIIIIIFFFI)Lorg/golanggui/typography/TextLayout;"} {
		c.methods[name] = e.Method(class, name, signature, false)
		if err = e.Exception(); err != nil {
			return nil, err
		}
	}
	layoutClass, err := androidruntime.LoadClass(e, "org.golanggui.typography.TextLayout")
	if err != nil {
		return nil, err
	}
	for name, signature := range layoutSignatures {
		c.layoutMethods[name] = e.Method(layoutClass, name, signature, false)
		if err = e.Exception(); err != nil {
			return nil, err
		}
	}
	c.peer = e.NewGlobalRef(peer)
	if err = e.Exception(); err != nil {
		return nil, err
	}
	if c.peer == 0 {
		return nil, fmt.Errorf("androidtext: retain Context failed")
	}
	return c, nil
}
func (c *Context) Name() string { return "Android Layout" }
func (c *Context) Destroy() {
	if c.peer == 0 {
		return
	}
	e := environment()
	e.CallVoid(c.peer, c.methods["close"])
	check(e)
	e.DeleteGlobalRef(c.peer)
	c.peer = 0
	c.files = nil
	c.methods = nil
}
func environment() jni.Env {
	e, err := androidruntime.Environment()
	if err != nil {
		panic(err)
	}
	return e
}
func check(e jni.Env) {
	if err := e.Exception(); err != nil {
		panic(err)
	}
}
func frame() jni.Env {
	e := environment()
	if e.PushLocalFrame(32) < 0 {
		check(e)
		panic("androidtext: JNI local frame failed")
	}
	return e
}

func (c *Context) AddFont(path string) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if c.files[path] {
		return nil
	}
	faces, err := fontFaces(path)
	if err != nil {
		return err
	}
	names := make([]string, len(faces))
	metadata := make([]int32, len(faces)*2)
	for i, face := range faces {
		names[i] = face.family
		metadata[i*2] = face.index
		metadata[i*2+1] = face.width
	}
	e := frame()
	defer e.PopLocalFrame()
	p := e.NewString(path)
	if err = e.Exception(); err != nil {
		return err
	}
	n := e.Strings(names)
	if err = e.Exception(); err != nil {
		return err
	}
	m := e.IntArray(metadata)
	if err = e.Exception(); err != nil {
		return err
	}
	e.CallVoid(c.peer, c.methods["addFont"], jni.Ref(p), jni.Ref(n), jni.Ref(m))
	if err = e.Exception(); err != nil {
		return err
	}
	c.files[path] = true
	return nil
}

func dimensions(width, height float32) error {
	for _, v := range []float32{width, height} {
		if v < 0 || v > 1<<24 || math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return fmt.Errorf("androidtext: invalid layout dimensions %g x %g", width, height)
		}
	}
	return nil
}
func wrapping(wrap typography.WrapMode) error {
	if wrap == typography.WrapChar {
		return fmt.Errorf("androidtext: WrapChar has no public native strategy")
	}
	if wrap != typography.WrapNone && wrap != typography.WrapWordChar {
		return fmt.Errorf("androidtext: invalid wrap mode %d", wrap)
	}
	return nil
}
func resolveFont(request typography.FontInfo) (typography.FontInfo, error) {
	font, err := utils.ResolveFont(request)
	if err != nil {
		return font, err
	}
	if float64(font.Size)*96/72*64 > math.MaxInt32 {
		return font, fmt.Errorf("androidtext: font size exceeds native coordinate range")
	}
	return font, nil
}
func alignment(value typography.TextAlignment) error {
	if value < typography.TextAlignBegin || value > typography.TextAlignFill {
		return fmt.Errorf("androidtext: invalid alignment %d", value)
	}
	return nil
}
func argb(value color.Color) int32 {
	if value == nil {
		value = typography.DefaultTextColor()
	}
	r, g, b, a := value.RGBA()
	if a == 0 {
		return 0
	}
	r = min(65535, (r*65535+a/2)/a)
	g = min(65535, (g*65535+a/2)/a)
	b = min(65535, (b*65535+a/2)/a)
	return int32((a>>8)<<24 | (r>>8)<<16 | (g>>8)<<8 | b>>8)
}

func (c *Context) NewTextLayout(text string, format typography.TextFormat, width, height float32) (typography.TextLayout, error) {
	if !utf8.ValidString(text) {
		return nil, fmt.Errorf("androidtext: text is not UTF-8")
	}
	font, err := resolveFont(format.Font)
	if err != nil {
		return nil, err
	}
	for _, err := range []error{dimensions(width, height), wrapping(format.WrapMode), alignment(format.TextAlign), utils.ValidateTabWidth(format.TabWidth)} {
		if err != nil {
			return nil, err
		}
	}
	e := frame()
	defer e.PopLocalFrame()
	content := e.NewString(text)
	if err = e.Exception(); err != nil {
		return nil, err
	}
	family := e.NewString(font.Family)
	if err = e.Exception(); err != nil {
		return nil, err
	}
	peer := e.CallObject(c.peer, c.methods["create"], jni.Ref(content), jni.Ref(family), jni.Float(font.Size), jni.Int(int32(font.Weight)), jni.Int(int32(font.Width)), jni.Int(int32(font.Style)), jni.Int(int32(format.WrapMode)), jni.Int(int32(format.TextAlign)), jni.Float(width), jni.Float(height), jni.Float(format.TabWidth), jni.Int(argb(format.TextColor)))
	if err = e.Exception(); err != nil {
		return nil, err
	}
	if peer == 0 {
		return nil, fmt.Errorf("androidtext: create TextLayout failed")
	}
	t := &TextLayout{text: text, format: format, width: width, height: height, positions: utils.CalcStringPosition(text), methods: c.layoutMethods}
	t.peer = e.NewGlobalRef(peer)
	if err = e.Exception(); err != nil {
		return nil, err
	}
	if t.peer == 0 {
		return nil, fmt.Errorf("androidtext: retain TextLayout failed")
	}
	diagnostic.Record(diagnostic.LayoutCreate, 1)
	return t, nil
}

var layoutSignatures = map[string]string{
	"close": "()V", "setSize": "(FF)V", "setAlignment": "(I)V", "setWrap": "(I)V", "setTabWidth": "(F)V",
	"setFont": "(IILjava/lang/String;FIII)V", "setColor": "(III)V", "setUnderline": "(IIZ)V", "setStrike": "(IIZ)V",
	"setPlaceholder": "(IIFFF)V", "metrics": "()[Ljava/lang/Object;", "raster": "(FIIII)Landroid/graphics/Bitmap;",
}

type TextLayout struct {
	peer          jni.Object
	methods       map[string]jni.Method
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
	defer e.PopLocalFrame()
	e.CallVoid(t.peer, t.methods["close"])
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
func (t *TextLayout) invoke(name string, args ...jni.Value) {
	e := frame()
	defer e.PopLocalFrame()
	e.CallVoid(t.peer, t.methods[name], args...)
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
	t.invoke("setSize", jni.Float(width), jni.Float(height))
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
	t.invoke("setAlignment", jni.Int(int32(value)))
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
	t.invoke("setWrap", jni.Int(int32(value)))
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
	t.invoke("setTabWidth", jni.Float(value))
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
	defer e.PopLocalFrame()
	family := e.NewString(font.Family)
	check(e)
	e.CallVoid(t.peer, t.methods["setFont"], a, b, jni.Ref(family), jni.Float(font.Size), jni.Int(int32(font.Weight)), jni.Int(int32(font.Width)), jni.Int(int32(font.Style)))
	check(e)
	t.changed(true)
}
func (t *TextLayout) SetTextColor(start, length int, c color.Color) {
	a, b := t.textRange(start, length)
	t.invoke("setColor", a, b, jni.Int(argb(c)))
	t.changed(false)
}
func (t *TextLayout) SetUnderline(start, length int, enabled bool) {
	a, b := t.textRange(start, length)
	var value int32
	if enabled {
		value = 1
	}
	t.invoke("setUnderline", a, b, jni.Int(value))
	t.changed(false)
}
func (t *TextLayout) SetStrikethrough(start, length int, enabled bool) {
	a, b := t.textRange(start, length)
	var value int32
	if enabled {
		value = 1
	}
	t.invoke("setStrike", a, b, jni.Int(value))
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
	t.invoke("setPlaceholder", jni.Int(int32(t.positions.ToUtf16(start))), jni.Int(present), jni.Float(value.Width), jni.Float(value.Height), jni.Float(value.Baseline))
	t.changed(true)
}
func (t *TextLayout) prepare() {
	if t.valid {
		return
	}
	e := frame()
	defer e.PopLocalFrame()
	data := e.CallObject(t.peer, t.methods["metrics"])
	check(e)
	li := e.Ints(e.ArrayElement(data, 0))
	lf := e.Floats(e.ArrayElement(data, 1))
	ci := e.Ints(e.ArrayElement(data, 2))
	cf := e.Floats(e.ArrayElement(data, 3))
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
	defer e.PopLocalFrame()
	obj := e.CallObject(t.peer, t.methods["raster"], jni.Float(scale), jni.Int(int32(region.Min.X)), jni.Int(int32(region.Min.Y)), jni.Int(int32(w)), jni.Int(int32(h)))
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

var _ typography.Context = (*Context)(nil)
var _ typography.TextLayout = (*TextLayout)(nil)
