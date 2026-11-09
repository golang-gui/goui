// Package egl binds the public EGL API; handles are owned according to EGL.
package egl

type Display uintptr
type Config uintptr
type Context uintptr
type Surface uintptr
type Boolean uint32

func attributes(values []int32) *int32 {
	if len(values) == 0 {
		return nil
	}
	return &values[0]
}
func configPointer(values []Config) *Config {
	if len(values) == 0 {
		return nil
	}
	return &values[0]
}
