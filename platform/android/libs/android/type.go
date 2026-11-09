// Package android binds public libandroid APIs. Handles and callback arguments
// are borrowed native values unless a native acquire operation says otherwise.
package android

// Activity mirrors ANativeActivity for the supported ARM64 ABI.
type Activity struct {
	Callbacks                          *ActivityCallbacks
	VM, Env, Object                    uintptr
	InternalDataPath, ExternalDataPath uintptr
	SDKVersion                         int32
	_                                  int32
	Instance, AssetManager, OBBPath    uintptr
}

// ActivityCallbacks mirrors ANativeActivityCallbacks. Registered callbacks must
// remain valid until the native Activity has finished using the table.
type ActivityCallbacks struct {
	OnStart, OnResume, OnSaveInstanceState, OnPause, OnStop, OnDestroy uintptr
	OnWindowFocusChanged, OnNativeWindowCreated, OnNativeWindowResized uintptr
	OnNativeWindowRedrawNeeded, OnNativeWindowDestroyed                uintptr
	OnInputQueueCreated, OnInputQueueDestroyed, OnContentRectChanged   uintptr
	OnConfigurationChanged, OnLowMemory                                uintptr
}

// Rect mirrors ARect in native pixels.
type Rect struct{ Left, Top, Right, Bottom int32 }

// WindowBuffer mirrors ANativeWindow_Buffer. Stride is in pixels; Bits is
// borrowed only while the corresponding native buffer remains locked.
type WindowBuffer struct {
	Width, Height, Stride, Format int32
	Bits                          uintptr
	Reserved                      [6]uint32
}
