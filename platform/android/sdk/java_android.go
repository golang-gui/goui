package sdk

import (
	"fmt"

	"github.com/golang-gui/goui/platform/android/libs/jni"
)

type File jni.Object
type ClassLoader jni.Object

var fileClass jni.Object
var fileMethods struct{ constructor, getAbsolutePath jni.Method }
var classLoaderMethods struct{ loadClass jni.Method }

func initFile(env jni.Env) (jni.Object, error) {
	var err error
	fileClass, err = bindClass(env, "java/io/File", []methodBinding{
		{"<init>", "(Ljava/lang/String;)V", &fileMethods.constructor},
		{"getAbsolutePath", "()Ljava/lang/String;", &fileMethods.getAbsolutePath},
	})
	return fileClass, err
}
func initClassLoader(env jni.Env) (jni.Object, error) {
	return bindClass(env, "java/lang/ClassLoader", []methodBinding{
		{"loadClass", "(Ljava/lang/String;)Ljava/lang/Class;", &classLoaderMethods.loadClass},
	})
}

func (f File) GetAbsolutePath(env jni.Env) (string, error) {
	text := env.CallObjectMethodA(jni.Object(f), fileMethods.getAbsolutePath)
	if err := env.Exception(); err != nil {
		return "", err
	}
	if text == 0 {
		return "", fmt.Errorf("android sdk: File.getAbsolutePath returned null")
	}
	defer env.DeleteLocalRef(text)
	value := env.String(text)
	return value, env.Exception()
}

// LoadClass takes a Java binary name and returns a local class reference.
func (l ClassLoader) LoadClass(env jni.Env, name string) (jni.Object, error) {
	text := env.NewString(name)
	if err := env.Exception(); err != nil {
		return 0, err
	}
	if text == 0 {
		return 0, fmt.Errorf("android sdk: create class name failed")
	}
	defer env.DeleteLocalRef(text)
	class := env.CallObjectMethodA(jni.Object(l), classLoaderMethods.loadClass, jni.Ref(text))
	return class, env.Exception()
}

func NewFile(env jni.Env, path string) (File, error) {
	text := env.NewString(path)
	if err := env.Exception(); err != nil {
		return 0, err
	}
	defer env.DeleteLocalRef(text)
	value := env.NewObjectA(fileClass, fileMethods.constructor, jni.Ref(text))
	return File(value), env.Exception()
}
