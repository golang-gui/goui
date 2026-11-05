// Package androidruntime accesses the Android host's process bootstrap data.
package androidruntime

import (
	"fmt"
	"sync"

	"github.com/golang-gui/goui/platform/android/libs/jni"
)

var host struct {
	sync.RWMutex
	vm     jni.VM
	loader jni.Object
}

// Initialize adopts the process VM and a retained application class loader.
// The platform calls it on the native thread before starting application main.
func Initialize(vm jni.VM, loader jni.Object) {
	host.Lock()
	defer host.Unlock()
	if host.vm != 0 {
		return
	}
	host.vm, host.loader = vm, loader
}
func bootstrap() (jni.VM, jni.Object, error) {
	host.RLock()
	defer host.RUnlock()
	if host.vm == 0 || host.loader == 0 {
		return 0, 0, fmt.Errorf("Android Java bootstrap unavailable")
	}
	return host.vm, host.loader, nil
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
