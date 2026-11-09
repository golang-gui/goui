// Package androidtext implements Typography using Android public text APIs.
package androidtext

import (
	"fmt"
	"image/color"
	"math"
	"path/filepath"
	"unicode/utf8"

	"github.com/golang-gui/goui/internal/diagnostic"
	"github.com/golang-gui/goui/platform/android/libs/jni"
	"github.com/golang-gui/goui/platform/internal/androidruntime"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/utils"
)

type Context struct {
	peer          jni.Object
	methods       contextMethods
	layoutMethods *layoutMethods
	files         map[string]bool
}

// NewContext creates a thread-affine Android typography context. Its owning
// OS thread must be attached to the Java VM. API 29 or later and the
// matching typography Dex component are required. WrapChar is not yet supported.
func NewContext() (typography.Context, error) {
	e, err := androidruntime.Environment()
	if err != nil {
		return nil, err
	}
	if e.PushLocalFrame(32) < 0 {
		return nil, fmt.Errorf("androidtext: JNI local frame: %w", e.Exception())
	}
	defer e.PopLocalFrame(0)
	class, err := androidruntime.LoadClass(e, "org.golanggui.typography.TextContext")
	if err != nil {
		return nil, err
	}
	constructor := e.GetMethodID(class, "<init>", "(I)V")
	if err = e.Exception(); err != nil {
		return nil, err
	}
	peer := e.NewObjectA(class, constructor, jni.Int(1))
	if err = e.Exception(); err != nil {
		return nil, err
	}
	if peer == 0 {
		return nil, fmt.Errorf("androidtext: create Context failed")
	}
	c := &Context{files: make(map[string]bool), layoutMethods: new(layoutMethods)}
	if err := bindMethods(e, class, []methodBinding{
		{"close", "()V", &c.methods.close},
		{"addFont", "(Ljava/lang/String;[Ljava/lang/String;[I)V", &c.methods.addFont},
		{"create", "(Ljava/lang/String;Ljava/lang/String;FIIIIIFFFI)Lorg/golanggui/typography/TextLayout;", &c.methods.create},
	}); err != nil {
		return nil, err
	}
	layoutClass, err := androidruntime.LoadClass(e, "org.golanggui.typography.TextLayout")
	if err != nil {
		return nil, err
	}
	if err := bindMethods(e, layoutClass, []methodBinding{
		{"close", "()V", &c.layoutMethods.close},
		{"setSize", "(FF)V", &c.layoutMethods.setSize},
		{"setAlignment", "(I)V", &c.layoutMethods.setAlignment},
		{"setWrap", "(I)V", &c.layoutMethods.setWrap},
		{"setTabWidth", "(F)V", &c.layoutMethods.setTabWidth},
		{"setFont", "(IILjava/lang/String;FIII)V", &c.layoutMethods.setFont},
		{"setColor", "(III)V", &c.layoutMethods.setColor},
		{"setUnderline", "(IIZ)V", &c.layoutMethods.setUnderline},
		{"setStrike", "(IIZ)V", &c.layoutMethods.setStrike},
		{"setPlaceholder", "(IIFFF)V", &c.layoutMethods.setPlaceholder},
		{"metrics", "()[Ljava/lang/Object;", &c.layoutMethods.metrics},
		{"raster", "(FIIII)Landroid/graphics/Bitmap;", &c.layoutMethods.raster},
	}); err != nil {
		return nil, err
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
	e.CallVoidMethodA(c.peer, c.methods.close)
	check(e)
	e.DeleteGlobalRef(c.peer)
	c.peer = 0
	c.files = nil
	c.methods = contextMethods{}
	c.layoutMethods = nil
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
	defer e.PopLocalFrame(0)
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
	e.CallVoidMethodA(c.peer, c.methods.addFont, jni.Ref(p), jni.Ref(n), jni.Ref(m))
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
	defer e.PopLocalFrame(0)
	content := e.NewString(text)
	if err = e.Exception(); err != nil {
		return nil, err
	}
	family := e.NewString(font.Family)
	if err = e.Exception(); err != nil {
		return nil, err
	}
	peer := e.CallObjectMethodA(c.peer, c.methods.create, jni.Ref(content), jni.Ref(family), jni.Float(font.Size), jni.Int(int32(font.Weight)), jni.Int(int32(font.Width)), jni.Int(int32(font.Style)), jni.Int(int32(format.WrapMode)), jni.Int(int32(format.TextAlign)), jni.Float(width), jni.Float(height), jni.Float(format.TabWidth), jni.Int(argb(format.TextColor)))
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

// Method IDs are immutable after construction. Layouts share this table;
// Java TextPaint, bitmap and mutable layout data remain owned by each layout.
type contextMethods struct{ close, addFont, create jni.Method }
type layoutMethods struct {
	close                                                      jni.Method
	setSize, setAlignment, setWrap, setTabWidth                jni.Method
	setFont, setColor, setUnderline, setStrike, setPlaceholder jni.Method
	metrics, raster                                            jni.Method
}
type methodBinding struct {
	name, signature string
	method          *jni.Method
}

func bindMethods(e jni.Env, class jni.Object, bindings []methodBinding) error {
	for _, b := range bindings {
		method := e.GetMethodID(class, b.name, b.signature)
		if err := e.Exception(); err != nil {
			return fmt.Errorf("androidtext: %s: %w", b.name, err)
		}
		if method == 0 {
			return fmt.Errorf("androidtext: method %s unavailable", b.name)
		}
		*b.method = method
	}
	return nil
}

var _ typography.Context = (*Context)(nil)
