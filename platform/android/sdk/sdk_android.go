// Package sdk binds the Java framework classes used by the Android adapters.
// Init retains class references for the process. Methods take the caller's
// current JNIEnv explicitly; returned objects are local references unless noted.
package sdk

import (
	"fmt"
	"sync"

	"github.com/golang-gui/goui/platform/android/libs/jni"
)

var initialization struct {
	sync.Once
	err     error
	classes []jni.Object
}

func Init(env jni.Env) error {
	initialization.Do(func() {
		for _, initClass := range []func(jni.Env) (jni.Object, error){initActivity, initWindow, initView, initFile, initClassLoader} {
			class, err := initClass(env)
			if err != nil {
				for _, class := range initialization.classes {
					env.DeleteGlobalRef(class)
				}
				initialization.classes = nil
				initialization.err = err
				return
			}
			initialization.classes = append(initialization.classes, class)
		}
	})
	return initialization.err
}

type methodBinding struct {
	name, signature string
	method          *jni.Method
}

func bindClass(env jni.Env, name string, methods []methodBinding) (jni.Object, error) {
	class := env.FindClass(name)
	if err := env.Exception(); err != nil {
		return 0, fmt.Errorf("android sdk: %s: %w", name, err)
	}
	if class == 0 {
		return 0, fmt.Errorf("android sdk: class %s unavailable", name)
	}
	defer env.DeleteLocalRef(class)
	for _, binding := range methods {
		method := env.GetMethodID(class, binding.name, binding.signature)
		if err := env.Exception(); err != nil {
			return 0, fmt.Errorf("android sdk: %s.%s: %w", name, binding.name, err)
		}
		if method == 0 {
			return 0, fmt.Errorf("android sdk: method %s.%s unavailable", name, binding.name)
		}
		*binding.method = method
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
