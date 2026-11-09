package sdk

import "github.com/golang-gui/goui/platform/android/libs/jni"

// android.graphics.Typeface

type Typeface jni.Object

var typefaceClass jni.Object
var typefaceMethods struct{ create, createWeight jni.Method }

func initTypeface(env jni.Env) (jni.Object, error) {
	var err error
	typefaceClass, err = bindClass(env, "android/graphics/Typeface", []methodBinding{}, []methodBinding{
		{"create", "(Ljava/lang/String;I)Landroid/graphics/Typeface;", &typefaceMethods.create},
		{"create", "(Landroid/graphics/Typeface;IZ)Landroid/graphics/Typeface;", &typefaceMethods.createWeight},
	})
	return typefaceClass, err
}

// android.graphics.fonts.Font

type Font jni.Object

var fontClass jni.Object
var fontMethods struct{ getStyle jni.Method }

func initFont(env jni.Env) (jni.Object, error) {
	var err error
	fontClass, err = bindClass(env, "android/graphics/fonts/Font", []methodBinding{
		{"getStyle", "()Landroid/graphics/fonts/FontStyle;", &fontMethods.getStyle},
	})
	return fontClass, err
}

// android.graphics.fonts.FontStyle

type FontStyle jni.Object

var fontStyleClass jni.Object
var fontStyleMethods struct{ constructor, getWeight, getSlant jni.Method }

func initFontStyle(env jni.Env) (jni.Object, error) {
	var err error
	fontStyleClass, err = bindClass(env, "android/graphics/fonts/FontStyle", []methodBinding{
		{"<init>", "(II)V", &fontStyleMethods.constructor},
		{"getWeight", "()I", &fontStyleMethods.getWeight},
		{"getSlant", "()I", &fontStyleMethods.getSlant},
	})
	return fontStyleClass, err
}

// android.graphics.fonts.Font$Builder

type FontBuilder jni.Object

var fontBuilderClass jni.Object
var fontBuilderMethods struct{ constructor, setTtcIndex, build jni.Method }

func initFontBuilder(env jni.Env) (jni.Object, error) {
	var err error
	fontBuilderClass, err = bindClass(env, "android/graphics/fonts/Font$Builder", []methodBinding{
		{"<init>", "(Ljava/io/File;)V", &fontBuilderMethods.constructor},
		{"setTtcIndex", "(I)Landroid/graphics/fonts/Font$Builder;", &fontBuilderMethods.setTtcIndex},
		{"build", "()Landroid/graphics/fonts/Font;", &fontBuilderMethods.build},
	})
	return fontBuilderClass, err
}

// android.graphics.fonts.FontFamily

type FontFamily jni.Object

var fontFamilyClass jni.Object

func initFontFamily(env jni.Env) (jni.Object, error) {
	var err error
	fontFamilyClass, err = bindClass(env, "android/graphics/fonts/FontFamily", []methodBinding{})
	return fontFamilyClass, err
}

// android.graphics.fonts.FontFamily$Builder

type FontFamilyBuilder jni.Object

var fontFamilyBuilderClass jni.Object
var fontFamilyBuilderMethods struct{ constructor, addFont, build jni.Method }

func initFontFamilyBuilder(env jni.Env) (jni.Object, error) {
	var err error
	fontFamilyBuilderClass, err = bindClass(env, "android/graphics/fonts/FontFamily$Builder", []methodBinding{
		{"<init>", "(Landroid/graphics/fonts/Font;)V", &fontFamilyBuilderMethods.constructor},
		{"addFont", "(Landroid/graphics/fonts/Font;)Landroid/graphics/fonts/FontFamily$Builder;", &fontFamilyBuilderMethods.addFont},
		{"build", "()Landroid/graphics/fonts/FontFamily;", &fontFamilyBuilderMethods.build},
	})
	return fontFamilyBuilderClass, err
}

// android.graphics.Typeface$CustomFallbackBuilder

type TypefaceCustomFallbackBuilder jni.Object

var typefaceCustomFallbackBuilderClass jni.Object
var typefaceCustomFallbackBuilderMethods struct{ constructor, setSystemFallback, setStyle, build jni.Method }

func initTypefaceCustomFallbackBuilder(env jni.Env) (jni.Object, error) {
	var err error
	typefaceCustomFallbackBuilderClass, err = bindClass(env, "android/graphics/Typeface$CustomFallbackBuilder", []methodBinding{
		{"<init>", "(Landroid/graphics/fonts/FontFamily;)V", &typefaceCustomFallbackBuilderMethods.constructor},
		{"setSystemFallback", "(Ljava/lang/String;)Landroid/graphics/Typeface$CustomFallbackBuilder;", &typefaceCustomFallbackBuilderMethods.setSystemFallback},
		{"setStyle", "(Landroid/graphics/fonts/FontStyle;)Landroid/graphics/Typeface$CustomFallbackBuilder;", &typefaceCustomFallbackBuilderMethods.setStyle},
		{"build", "()Landroid/graphics/Typeface;", &typefaceCustomFallbackBuilderMethods.build},
	})
	return typefaceCustomFallbackBuilderClass, err
}

func NewFontBuilder(env jni.Env, file File) (FontBuilder, error) {
	result := env.NewObjectA(fontBuilderClass, fontBuilderMethods.constructor, jni.Ref(jni.Object(file)))
	return FontBuilder(result), env.Exception()
}
func (b FontBuilder) SetTtcIndex(env jni.Env, index int32) (FontBuilder, error) {
	result := env.CallObjectMethodA(jni.Object(b), fontBuilderMethods.setTtcIndex, jni.Int(index))
	return FontBuilder(result), env.Exception()
}
func (b FontBuilder) Build(env jni.Env) (Font, error) {
	result := env.CallObjectMethodA(jni.Object(b), fontBuilderMethods.build)
	return Font(result), env.Exception()
}
func (f Font) GetStyle(env jni.Env) (FontStyle, error) {
	result := env.CallObjectMethodA(jni.Object(f), fontMethods.getStyle)
	return FontStyle(result), env.Exception()
}
func NewFontStyle(env jni.Env, weight, slant int32) (FontStyle, error) {
	result := env.NewObjectA(fontStyleClass, fontStyleMethods.constructor, jni.Int(weight), jni.Int(slant))
	return FontStyle(result), env.Exception()
}
func (f FontStyle) GetWeight(env jni.Env) (int32, error) {
	result := env.CallIntMethodA(jni.Object(f), fontStyleMethods.getWeight)
	return int32(result), env.Exception()
}
func (f FontStyle) GetSlant(env jni.Env) (int32, error) {
	result := env.CallIntMethodA(jni.Object(f), fontStyleMethods.getSlant)
	return int32(result), env.Exception()
}
func NewFontFamilyBuilder(env jni.Env, font Font) (FontFamilyBuilder, error) {
	result := env.NewObjectA(fontFamilyBuilderClass, fontFamilyBuilderMethods.constructor, jni.Ref(jni.Object(font)))
	return FontFamilyBuilder(result), env.Exception()
}
func (b FontFamilyBuilder) AddFont(env jni.Env, font Font) (FontFamilyBuilder, error) {
	result := env.CallObjectMethodA(jni.Object(b), fontFamilyBuilderMethods.addFont, jni.Ref(jni.Object(font)))
	return FontFamilyBuilder(result), env.Exception()
}
func (b FontFamilyBuilder) Build(env jni.Env) (FontFamily, error) {
	result := env.CallObjectMethodA(jni.Object(b), fontFamilyBuilderMethods.build)
	return FontFamily(result), env.Exception()
}
func NewTypefaceCustomFallbackBuilder(env jni.Env, family FontFamily) (TypefaceCustomFallbackBuilder, error) {
	result := env.NewObjectA(typefaceCustomFallbackBuilderClass, typefaceCustomFallbackBuilderMethods.constructor, jni.Ref(jni.Object(family)))
	return TypefaceCustomFallbackBuilder(result), env.Exception()
}
func (b TypefaceCustomFallbackBuilder) SetStyle(env jni.Env, style FontStyle) (TypefaceCustomFallbackBuilder, error) {
	result := env.CallObjectMethodA(jni.Object(b), typefaceCustomFallbackBuilderMethods.setStyle, jni.Ref(jni.Object(style)))
	return TypefaceCustomFallbackBuilder(result), env.Exception()
}
func (b TypefaceCustomFallbackBuilder) Build(env jni.Env) (Typeface, error) {
	result := env.CallObjectMethodA(jni.Object(b), typefaceCustomFallbackBuilderMethods.build)
	return Typeface(result), env.Exception()
}

const (
	FontSlantUpright int32 = 0
	FontSlantItalic  int32 = 1
)

func CreateTypeface(env jni.Env, family string, style int32) (Typeface, error) {
	text := env.NewString(family)
	if err := env.Exception(); err != nil {
		return 0, err
	}
	defer env.DeleteLocalRef(text)
	v := env.CallStaticObjectMethodA(typefaceClass, typefaceMethods.create, jni.Ref(text), jni.Int(style))
	return Typeface(v), env.Exception()
}
func CreateTypefaceWithWeight(env jni.Env, base Typeface, weight int32, italic bool) (Typeface, error) {
	v := env.CallStaticObjectMethodA(typefaceClass, typefaceMethods.createWeight, jni.Ref(jni.Object(base)), jni.Int(weight), jni.Bool(italic))
	return Typeface(v), env.Exception()
}
func (b TypefaceCustomFallbackBuilder) SetSystemFallback(env jni.Env, family string) (TypefaceCustomFallbackBuilder, error) {
	text := env.NewString(family)
	if err := env.Exception(); err != nil {
		return 0, err
	}
	defer env.DeleteLocalRef(text)
	v := env.CallObjectMethodA(jni.Object(b), typefaceCustomFallbackBuilderMethods.setSystemFallback, jni.Ref(text))
	return TypefaceCustomFallbackBuilder(v), env.Exception()
}
