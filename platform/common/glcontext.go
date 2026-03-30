package common

type GlContext interface {
	Name() string
	Destroy()
	MakeCurrent() error
	ClearCurrent() error
	SwapBuffers() error
	SwapInterval(int) error
}
