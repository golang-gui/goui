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
	"github.com/golang-gui/goui/platform/android/sdk"
	"github.com/golang-gui/goui/platform/internal/androidruntime"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/utils"
)

type Context struct {
	fonts *fontCollection
	files map[string]bool
}

// NewContext creates a thread-affine Android typography context. The current
// OS thread must be attached to the VM. API 29 and component ABI 3 are required;
// public Android Layout APIs do not supply a WrapChar strategy.
func NewContext() (typography.Context, error) {
	e, err := androidruntime.Environment()
	if err != nil {
		return nil, err
	}
	if e.PushLocalFrame(32) < 0 {
		return nil, fmt.Errorf("androidtext: JNI local frame: %w", e.Exception())
	}
	defer e.PopLocalFrame(0)
	if err = sdk.InitText(e); err != nil {
		return nil, err
	}
	if err = initBridge(e); err != nil {
		return nil, err
	}
	return &Context{fonts: newFontCollection(), files: make(map[string]bool)}, nil
}
func (c *Context) Name() string { return "Android Layout" }
func (c *Context) Destroy() {
	if c.fonts == nil {
		return
	}
	e := environment()
	c.fonts.release(e)
	c.fonts = nil
	c.files = nil
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
	e := frame()
	defer e.PopLocalFrame(0)
	if err = c.fonts.add(e, path, faces); err != nil {
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
	if c.fonts == nil {
		return nil, fmt.Errorf("androidtext: Context is destroyed")
	}
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
	face, err := c.fonts.resolve(e, font)
	if err != nil {
		return nil, err
	}
	paint, err := sdk.NewTextPaint(e, sdk.PaintAntiAliasFlag|sdk.PaintSubpixelTextFlag|sdk.PaintLinearTextFlag)
	if err != nil {
		return nil, err
	}
	if _, err = sdk.Paint(paint).SetTypeface(e, face); err != nil {
		return nil, err
	}
	if err = sdk.Paint(paint).SetTextSize(e, font.Size*96/72*nativeUnits); err != nil {
		return nil, err
	}
	if err = sdk.Paint(paint).SetColor(e, argb(format.TextColor)); err != nil {
		return nil, err
	}
	spans, err := sdk.NewSpannableString(e, content)
	if err != nil {
		return nil, err
	}
	peer := e.NewObjectA(javaLayout.class, javaLayout.constructor, jni.Int(componentABI), jni.Ref(jni.Object(spans)), jni.Ref(jni.Object(paint)), jni.Int(int32(format.WrapMode)), jni.Int(int32(format.TextAlign)), jni.Float(width*nativeUnits), jni.Float(height*nativeUnits), jni.Float(nativeTabWidth(format.TabWidth)))
	if err = e.Exception(); err != nil {
		return nil, err
	}
	global, err := retain(e, peer)
	if err != nil {
		return nil, err
	}
	c.fonts.owners++
	t := &TextLayout{native: layoutPeer(global), fonts: c.fonts, text: text, format: format, width: width, height: height, positions: utils.CalcStringPosition(text)}
	diagnostic.Record(diagnostic.LayoutCreate, 1)
	return t, nil
}

var _ typography.Context = (*Context)(nil)
