package sdk

import "github.com/golang-gui/goui/platform/android/libs/jni"

type Window jni.Object
type View jni.Object

const (
	ViewVisible   int32 = 0
	ViewInvisible int32 = 4
	ViewGone      int32 = 8
)

var windowMethods struct{ getDecorView jni.Method }
var viewMethods struct{ setVisibility, getVisibility jni.Method }

func initWindow(env jni.Env) (jni.Object, error) {
	return bindClass(env, "android/view/Window", []methodBinding{
		{"getDecorView", "()Landroid/view/View;", &windowMethods.getDecorView},
	})
}
func initView(env jni.Env) (jni.Object, error) {
	return bindClass(env, "android/view/View", []methodBinding{
		{"setVisibility", "(I)V", &viewMethods.setVisibility},
		{"getVisibility", "()I", &viewMethods.getVisibility},
	})
}

func (w Window) GetDecorView(env jni.Env) (View, error) {
	value := env.CallObjectMethodA(jni.Object(w), windowMethods.getDecorView)
	return View(value), env.Exception()
}
func (v View) SetVisibility(env jni.Env, visibility int32) error {
	env.CallVoidMethodA(jni.Object(v), viewMethods.setVisibility, jni.Int(visibility))
	return env.Exception()
}
func (v View) GetVisibility(env jni.Env) (int32, error) {
	value := env.CallIntMethodA(jni.Object(v), viewMethods.getVisibility)
	return value, env.Exception()
}
