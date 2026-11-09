package sdk

import "github.com/golang-gui/goui/platform/android/libs/jni"

// android.text.TextPaint

type TextPaint jni.Object

var textPaintClass jni.Object
var textPaintMethods struct{ constructor jni.Method }

func initTextPaint(env jni.Env) (jni.Object, error) {
	var err error
	textPaintClass, err = bindClass(env, "android/text/TextPaint", []methodBinding{
		{"<init>", "(I)V", &textPaintMethods.constructor},
	})
	return textPaintClass, err
}

// android.text.SpannableString

type SpannableString jni.Object

var spannableStringClass jni.Object
var spannableStringMethods struct{ constructor jni.Method }

func initSpannableString(env jni.Env) (jni.Object, error) {
	var err error
	spannableStringClass, err = bindClass(env, "android/text/SpannableString", []methodBinding{
		{"<init>", "(Ljava/lang/CharSequence;)V", &spannableStringMethods.constructor},
	})
	return spannableStringClass, err
}

// android.text.StaticLayout

type StaticLayout jni.Object

var staticLayoutClass jni.Object
var staticLayoutMethods struct{ getLineCount, getLineWidth, getLineBaseline, isRtlCharAt jni.Method }

func initStaticLayout(env jni.Env) (jni.Object, error) {
	var err error
	staticLayoutClass, err = bindClass(env, "android/text/StaticLayout", []methodBinding{
		{"getLineCount", "()I", &staticLayoutMethods.getLineCount},
		{"getLineWidth", "(I)F", &staticLayoutMethods.getLineWidth},
		{"getLineBaseline", "(I)I", &staticLayoutMethods.getLineBaseline},
		{"isRtlCharAt", "(I)Z", &staticLayoutMethods.isRtlCharAt},
	})
	return staticLayoutClass, err
}

// android.text.StaticLayout$Builder

type StaticLayoutBuilder jni.Object

var staticLayoutBuilderClass jni.Object
var staticLayoutBuilderMethods struct{ setIncludePad, setAlignment, build, obtain jni.Method }

func initStaticLayoutBuilder(env jni.Env) (jni.Object, error) {
	var err error
	staticLayoutBuilderClass, err = bindClass(env, "android/text/StaticLayout$Builder", []methodBinding{
		{"setIncludePad", "(Z)Landroid/text/StaticLayout$Builder;", &staticLayoutBuilderMethods.setIncludePad},
		{"setAlignment", "(Landroid/text/Layout$Alignment;)Landroid/text/StaticLayout$Builder;", &staticLayoutBuilderMethods.setAlignment},
		{"build", "()Landroid/text/StaticLayout;", &staticLayoutBuilderMethods.build},
	}, []methodBinding{
		{"obtain", "(Ljava/lang/CharSequence;IILandroid/text/TextPaint;I)Landroid/text/StaticLayout$Builder;", &staticLayoutBuilderMethods.obtain},
	})
	return staticLayoutBuilderClass, err
}

// android.text.Layout$Alignment

type LayoutAlignment jni.Object

var layoutAlignmentClass jni.Object

func initLayoutAlignment(env jni.Env) (jni.Object, error) {
	var err error
	layoutAlignmentClass, err = bindClass(env, "android/text/Layout$Alignment", []methodBinding{})
	return layoutAlignmentClass, err
}

func NewTextPaint(env jni.Env, flags int32) (TextPaint, error) {
	result := env.NewObjectA(textPaintClass, textPaintMethods.constructor, jni.Int(flags))
	return TextPaint(result), env.Exception()
}
func NewSpannableString(env jni.Env, text jni.Object) (SpannableString, error) {
	result := env.NewObjectA(spannableStringClass, spannableStringMethods.constructor, jni.Ref(text))
	return SpannableString(result), env.Exception()
}
func (l StaticLayout) GetLineCount(env jni.Env) (int32, error) {
	result := env.CallIntMethodA(jni.Object(l), staticLayoutMethods.getLineCount)
	return int32(result), env.Exception()
}
func (l StaticLayout) GetLineWidth(env jni.Env, line int32) (float32, error) {
	result := env.CallFloatMethodA(jni.Object(l), staticLayoutMethods.getLineWidth, jni.Int(line))
	return float32(result), env.Exception()
}
func (l StaticLayout) GetLineBaseline(env jni.Env, line int32) (int32, error) {
	result := env.CallIntMethodA(jni.Object(l), staticLayoutMethods.getLineBaseline, jni.Int(line))
	return int32(result), env.Exception()
}
func (l StaticLayout) IsRtlCharAt(env jni.Env, offset int32) (bool, error) {
	result := env.CallBooleanMethodA(jni.Object(l), staticLayoutMethods.isRtlCharAt, jni.Int(offset))
	return bool(result), env.Exception()
}
func StaticLayoutBuilderObtain(env jni.Env, source jni.Object, start, end int32, paint TextPaint, width int32) (StaticLayoutBuilder, error) {
	result := env.CallStaticObjectMethodA(staticLayoutBuilderClass, staticLayoutBuilderMethods.obtain, jni.Ref(source), jni.Int(start), jni.Int(end), jni.Ref(jni.Object(paint)), jni.Int(width))
	return StaticLayoutBuilder(result), env.Exception()
}
func (b StaticLayoutBuilder) SetIncludePad(env jni.Env, value bool) (StaticLayoutBuilder, error) {
	result := env.CallObjectMethodA(jni.Object(b), staticLayoutBuilderMethods.setIncludePad, jni.Bool(value))
	return StaticLayoutBuilder(result), env.Exception()
}
func (b StaticLayoutBuilder) SetAlignment(env jni.Env, value LayoutAlignment) (StaticLayoutBuilder, error) {
	result := env.CallObjectMethodA(jni.Object(b), staticLayoutBuilderMethods.setAlignment, jni.Ref(jni.Object(value)))
	return StaticLayoutBuilder(result), env.Exception()
}
func (b StaticLayoutBuilder) Build(env jni.Env) (StaticLayout, error) {
	result := env.CallObjectMethodA(jni.Object(b), staticLayoutBuilderMethods.build)
	return StaticLayout(result), env.Exception()
}
func LayoutAlignNormal(env jni.Env) (LayoutAlignment, error) {
	v, e := staticObject(env, layoutAlignmentClass, "ALIGN_NORMAL", "Landroid/text/Layout$Alignment;")
	return LayoutAlignment(v), e
}
