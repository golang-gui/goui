// Package androidruntime accesses the Android host's process bootstrap data.
package androidruntime

import (
	"fmt"
	"sync"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/android/libs/jni"
)

var host struct {
	sync.Once
	getVM, getLoader uintptr
	err              error
}

func bootstrap() (jni.VM, jni.Object, error) {
	host.Do(func() {
		getVM, err := cgo.Dlsym(cgo.RTLD_DEFAULT, "goui_android_java_vm")
		if err != nil {
			host.err = err
			return
		}
		getLoader, err := cgo.Dlsym(cgo.RTLD_DEFAULT, "goui_android_class_loader")
		if err != nil {
			host.err = err
			return
		}
		host.getVM = getVM
		host.getLoader = getLoader
	})
	if host.err != nil {
		return 0, 0, host.err
	}
	vm := jni.VM(cgo.CallRet[uintptr](host.getVM))
	loader := jni.Object(cgo.CallRet[uintptr](host.getLoader))
	if vm == 0 || loader == 0 {
		return 0, 0, fmt.Errorf("Android Java bootstrap unavailable")
	}
	return vm, loader, nil
}

// EnterThread attaches only the current fixed OS thread. The startup owner
// invokes the returned cleanup before unlocking that thread.
func EnterThread() func() {
	vm, _, err := bootstrap()
	if err != nil {
		panic(err)
	}
	_, status := vm.GetEnv()
	if status == 0 {
		return func() {}
	}
	if status != -2 {
		panic(fmt.Sprintf("JNI GetEnv: %d", status))
	}
	if _, status = vm.AttachCurrentThreadAsDaemon(); status != 0 {
		panic(fmt.Sprintf("JNI attach: %d", status))
	}
	return func() {
		if status := vm.DetachCurrentThread(); status != 0 {
			panic(fmt.Sprintf("JNI detach: %d", status))
		}
	}
}

func Environment() (jni.Env, error) {
	vm, _, err := bootstrap()
	if err != nil {
		return 0, err
	}
	env, status := vm.GetEnv()
	if status != 0 {
		return 0, fmt.Errorf("Android JNI thread is not attached: %d", status)
	}
	return env, nil
}

// LoadClass returns a local reference in the application's class loader.
func LoadClass(env jni.Env, name string) (jni.Object, error) {
	_, loader, err := bootstrap()
	if err != nil {
		return 0, err
	}
	class := env.ObjectClass(loader)
	defer env.DeleteLocalRef(class)
	method := env.Method(class, "loadClass", "(Ljava/lang/String;)Ljava/lang/Class;", false)
	if err := env.Exception(); err != nil {
		return 0, err
	}
	text := env.NewString(name)
	defer env.DeleteLocalRef(text)
	result := env.CallObject(loader, method, jni.Ref(text))
	if err := env.Exception(); err != nil {
		return 0, err
	}
	if result == 0 {
		return 0, fmt.Errorf("Android class %s unavailable", name)
	}
	return result, nil
}
