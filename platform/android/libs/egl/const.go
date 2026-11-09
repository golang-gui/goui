package egl

const (
	False                Boolean = 0
	True                 Boolean = 1
	Success              int32   = 0x3000
	None                 int32   = 0x3038
	SurfaceType          int32   = 0x3033
	WindowBit            int32   = 0x0004
	PbufferBit           int32   = 0x0001
	RenderableType       int32   = 0x3040
	OpenGLES3Bit         int32   = 0x0040
	RedSize              int32   = 0x3024
	GreenSize            int32   = 0x3023
	BlueSize             int32   = 0x3022
	AlphaSize            int32   = 0x3021
	DepthSize            int32   = 0x3025
	StencilSize          int32   = 0x3026
	NativeVisualID       int32   = 0x302e
	ContextClientVersion int32   = 0x3098
	Width                int32   = 0x3057
	Height               int32   = 0x3056
)
