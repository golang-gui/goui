package sdk

import "github.com/golang-gui/goui/platform/android/libs/jni"

type Activity jni.Object

var activityMethods struct {
	getClassLoader, getCacheDir, getWindow jni.Method
}

func initActivity(env jni.Env) (jni.Object, error) {
	return bindClass(env, "android/app/Activity", []methodBinding{
		{"getClassLoader", "()Ljava/lang/ClassLoader;", &activityMethods.getClassLoader},
		{"getCacheDir", "()Ljava/io/File;", &activityMethods.getCacheDir},
		{"getWindow", "()Landroid/view/Window;", &activityMethods.getWindow},
	})
}

func (a Activity) GetClassLoader(env jni.Env) (ClassLoader, error) {
	value := env.CallObjectMethodA(jni.Object(a), activityMethods.getClassLoader)
	return ClassLoader(value), env.Exception()
}
func (a Activity) GetCacheDir(env jni.Env) (File, error) {
	value := env.CallObjectMethodA(jni.Object(a), activityMethods.getCacheDir)
	return File(value), env.Exception()
}
func (a Activity) GetWindow(env jni.Env) (Window, error) {
	value := env.CallObjectMethodA(jni.Object(a), activityMethods.getWindow)
	return Window(value), env.Exception()
}
