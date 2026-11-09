// Package sdk binds the Java framework classes used by the Android adapters.
// Init and InitText retain their class references for the process. Text, font
// and drawing bindings require InitText. Methods take the caller's
// current JNIEnv explicitly; returned objects are local references unless noted.
package sdk

import (
	"fmt"
	"sync"

	"github.com/golang-gui/goui/platform/android/libs/jni"
)

type classBindings struct {
	sync.Once
	err     error
	classes []jni.Object
}

var initialization classBindings
var textInitialization classBindings

func Init(env jni.Env) error {
	return initialization.init(env, initActivity, initWindow, initView, initFile, initClassLoader)
}

// InitText loads the public text, font and software drawing classes on demand.
// It retains class references for the process, independently of Context lifetime.
func InitText(env jni.Env) error {
	if err := Init(env); err != nil {
		return err
	}
	return textInitialization.init(env, initPaint, initTextPaint, initSpannableString,
		initStaticLayout, initStaticLayoutBuilder, initLayoutAlignment,
		initTypeface, initFont, initFontStyle, initFontBuilder,
		initFontFamily, initFontFamilyBuilder, initTypefaceCustomFallbackBuilder,
		initBitmap, initBitmapConfig, initCanvas)
}

func (b *classBindings) init(env jni.Env, classes ...func(jni.Env) (jni.Object, error)) error {
	b.Do(func() {
		for _, initClass := range classes {
			class, err := initClass(env)
			if err != nil {
				for _, class := range b.classes {
					env.DeleteGlobalRef(class)
				}
				b.classes = nil
				b.err = err
				return
			}
			b.classes = append(b.classes, class)
		}
	})
	return b.err
}

type methodBinding struct {
	name, signature string
	method          *jni.Method
}

func bindClass(env jni.Env, name string, methods []methodBinding, staticMethods ...[]methodBinding) (jni.Object, error) {
	class := env.FindClass(name)
	if err := env.Exception(); err != nil {
		return 0, fmt.Errorf("android sdk: %s: %w", name, err)
	}
	if class == 0 {
		return 0, fmt.Errorf("android sdk: class %s unavailable", name)
	}
	defer env.DeleteLocalRef(class)
	groups := append([][]methodBinding{methods}, staticMethods...)
	for i, group := range groups {
		for _, binding := range group {
			var method jni.Method
			if i == 0 {
				method = env.GetMethodID(class, binding.name, binding.signature)
			} else {
				method = env.GetStaticMethodID(class, binding.name, binding.signature)
			}
			if err := env.Exception(); err != nil {
				return 0, fmt.Errorf("android sdk: %s.%s: %w", name, binding.name, err)
			}
			if method == 0 {
				return 0, fmt.Errorf("android sdk: method %s.%s unavailable", name, binding.name)
			}
			*binding.method = method
		}
	}
	global := env.NewGlobalRef(class)
	if err := env.Exception(); err != nil {
		return 0, err
	}
	if global == 0 {
		return 0, fmt.Errorf("android sdk: retain class %s failed", name)
	}
	return global, nil
}

// staticObject returns a local reference to a named public constant.
func staticObject(env jni.Env, class jni.Object, name, signature string) (jni.Object, error) {
	field := env.GetStaticFieldID(class, name, signature)
	if err := env.Exception(); err != nil {
		return 0, err
	}
	if field == 0 {
		return 0, fmt.Errorf("android sdk: field %s unavailable", name)
	}
	value := env.GetStaticObjectField(class, field)
	return value, env.Exception()
}
