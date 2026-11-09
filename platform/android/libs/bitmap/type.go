// Package bitmap binds the public Android software bitmap NDK interface.
package bitmap

// Info mirrors AndroidBitmapInfo. Stride is in bytes, unlike ANativeWindow_Buffer.
type Info struct {
	Width, Height, Stride uint32
	Format                int32
	Flags                 uint32
}

const RGBA8888 int32 = 1
