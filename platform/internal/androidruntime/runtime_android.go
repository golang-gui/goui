// Package androidruntime accesses the Android host's process bootstrap data.
package androidruntime

import (
	"fmt"
	"sync"

	"github.com/golang-gui/goui/platform/android/libs/jni"
	"github.com/golang-gui/goui/platform/android/sdk"
)

var host struct {
	sync.RWMutex
	vm     jni.VM
	loader sdk.ClassLoader
}

// Initialize adopts the process VM and a retained application class loader.
// The platform calls it on the native thread before starting application main.
func Initialize(vm jni.VM, loader sdk.ClassLoader) error {
	if vm == 0 || loader == 0 {
		return fmt.Errorf("Android Java bootstrap requires a VM and global class loader")
	}
	host.Lock()
	defer host.Unlock()
	if host.vm != 0 {
		return fmt.Errorf("Android Java bootstrap is already initialized")
	}
	host.vm, host.loader = vm, loader
	return nil
}
func bootstrap() (jni.VM, sdk.ClassLoader, error) {
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
	_, status := vm.GetEnv(jni.Version16)
	if status == jni.OK {
		return func() {}
	}
	if status != jni.Detached {
		panic(fmt.Sprintf("JNI GetEnv: %d", status))
	}
	if _, status = vm.AttachCurrentThreadAsDaemon(nil); status != jni.OK {
		panic(fmt.Sprintf("JNI attach: %d", status))
	}
	return func() {
		if status := vm.DetachCurrentThread(); status != jni.OK {
			panic(fmt.Sprintf("JNI detach: %d", status))
		}
	}
}

func Environment() (jni.Env, error) {
	vm, _, err := bootstrap()
	if err != nil {
		return 0, err
	}
	env, status := vm.GetEnv(jni.Version16)
	if status != jni.OK {
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
	result, err := loader.LoadClass(env, name)
	if err != nil {
		return 0, err
	}
	if result == 0 {
		return 0, fmt.Errorf("Android class %s unavailable", name)
	}
	return result, nil
}
