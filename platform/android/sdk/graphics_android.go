package sdk

import "github.com/golang-gui/goui/platform/android/libs/jni"

// android.graphics.Paint

type Paint jni.Object

var paintClass jni.Object
var paintMethods struct{ setTypeface, setTextSize, setColor jni.Method }

func initPaint(env jni.Env) (jni.Object, error) {
	var err error
	paintClass, err = bindClass(env, "android/graphics/Paint", []methodBinding{
		{"setTypeface", "(Landroid/graphics/Typeface;)Landroid/graphics/Typeface;", &paintMethods.setTypeface},
		{"setTextSize", "(F)V", &paintMethods.setTextSize},
		{"setColor", "(I)V", &paintMethods.setColor},
	})
	return paintClass, err
}

// android.graphics.Bitmap

type Bitmap jni.Object

var bitmapClass jni.Object
var bitmapMethods struct{ recycle, eraseColor, setDensity, createBitmap jni.Method }

func initBitmap(env jni.Env) (jni.Object, error) {
	var err error
	bitmapClass, err = bindClass(env, "android/graphics/Bitmap", []methodBinding{
		{"recycle", "()V", &bitmapMethods.recycle},
		{"eraseColor", "(I)V", &bitmapMethods.eraseColor},
		{"setDensity", "(I)V", &bitmapMethods.setDensity},
	}, []methodBinding{
		{"createBitmap", "(IILandroid/graphics/Bitmap$Config;)Landroid/graphics/Bitmap;", &bitmapMethods.createBitmap},
	})
	return bitmapClass, err
}

// android.graphics.Bitmap$Config

type BitmapConfig jni.Object

var bitmapConfigClass jni.Object

func initBitmapConfig(env jni.Env) (jni.Object, error) {
	var err error
	bitmapConfigClass, err = bindClass(env, "android/graphics/Bitmap$Config", []methodBinding{})
	return bitmapConfigClass, err
}

// android.graphics.Canvas

type Canvas jni.Object

var canvasClass jni.Object
var canvasMethods struct{ constructor, setBitmap, save, restore, clipRect, translate, scale jni.Method }

func initCanvas(env jni.Env) (jni.Object, error) {
	var err error
	canvasClass, err = bindClass(env, "android/graphics/Canvas", []methodBinding{
		{"<init>", "()V", &canvasMethods.constructor},
		{"setBitmap", "(Landroid/graphics/Bitmap;)V", &canvasMethods.setBitmap},
		{"save", "()I", &canvasMethods.save},
		{"restore", "()V", &canvasMethods.restore},
		{"clipRect", "(FFFF)Z", &canvasMethods.clipRect},
		{"translate", "(FF)V", &canvasMethods.translate},
		{"scale", "(FF)V", &canvasMethods.scale},
	})
	return canvasClass, err
}

func (p Paint) SetTypeface(env jni.Env, face Typeface) (Typeface, error) {
	result := env.CallObjectMethodA(jni.Object(p), paintMethods.setTypeface, jni.Ref(jni.Object(face)))
	return Typeface(result), env.Exception()
}
func (p Paint) SetTextSize(env jni.Env, size float32) error {
	env.CallVoidMethodA(jni.Object(p), paintMethods.setTextSize, jni.Float(size))
	return env.Exception()
}
func (p Paint) SetColor(env jni.Env, color int32) error {
	env.CallVoidMethodA(jni.Object(p), paintMethods.setColor, jni.Int(color))
	return env.Exception()
}
func CreateBitmap(env jni.Env, width, height int32, config BitmapConfig) (Bitmap, error) {
	result := env.CallStaticObjectMethodA(bitmapClass, bitmapMethods.createBitmap, jni.Int(width), jni.Int(height), jni.Ref(jni.Object(config)))
	return Bitmap(result), env.Exception()
}
func (b Bitmap) Recycle(env jni.Env) error {
	env.CallVoidMethodA(jni.Object(b), bitmapMethods.recycle)
	return env.Exception()
}
func (b Bitmap) EraseColor(env jni.Env, color int32) error {
	env.CallVoidMethodA(jni.Object(b), bitmapMethods.eraseColor, jni.Int(color))
	return env.Exception()
}
func (b Bitmap) SetDensity(env jni.Env, density int32) error {
	env.CallVoidMethodA(jni.Object(b), bitmapMethods.setDensity, jni.Int(density))
	return env.Exception()
}
func NewCanvas(env jni.Env) (Canvas, error) {
	result := env.NewObjectA(canvasClass, canvasMethods.constructor)
	return Canvas(result), env.Exception()
}
func (c Canvas) SetBitmap(env jni.Env, bitmap Bitmap) error {
	env.CallVoidMethodA(jni.Object(c), canvasMethods.setBitmap, jni.Ref(jni.Object(bitmap)))
	return env.Exception()
}
func (c Canvas) Save(env jni.Env) (int32, error) {
	result := env.CallIntMethodA(jni.Object(c), canvasMethods.save)
	return int32(result), env.Exception()
}
func (c Canvas) Restore(env jni.Env) error {
	env.CallVoidMethodA(jni.Object(c), canvasMethods.restore)
	return env.Exception()
}
func (c Canvas) ClipRect(env jni.Env, left, top, right, bottom float32) (bool, error) {
	result := env.CallBooleanMethodA(jni.Object(c), canvasMethods.clipRect, jni.Float(left), jni.Float(top), jni.Float(right), jni.Float(bottom))
	return bool(result), env.Exception()
}
func (c Canvas) Translate(env jni.Env, x, y float32) error {
	env.CallVoidMethodA(jni.Object(c), canvasMethods.translate, jni.Float(x), jni.Float(y))
	return env.Exception()
}
func (c Canvas) Scale(env jni.Env, x, y float32) error {
	env.CallVoidMethodA(jni.Object(c), canvasMethods.scale, jni.Float(x), jni.Float(y))
	return env.Exception()
}

const (
	PaintAntiAliasFlag    int32 = 1
	PaintLinearTextFlag   int32 = 64
	PaintSubpixelTextFlag int32 = 128
	BitmapDensityNone     int32 = 0
)

func BitmapARGB8888(env jni.Env) (BitmapConfig, error) {
	v, e := staticObject(env, bitmapConfigClass, "ARGB_8888", "Landroid/graphics/Bitmap$Config;")
	return BitmapConfig(v), e
}
